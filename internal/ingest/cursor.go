package ingest

import (
	"context"
	"fmt"
	"time"
)

// cursorConn is the minimal ClickHouse interface used by CursorStore.
type cursorConn interface {
	Exec(ctx context.Context, query string, args ...any) error
	Select(ctx context.Context, dest any, query string, args ...any) error
}

// CursorStore manages rows in meta.ingest_cursors for backfill job tracking.
// The schema uses ORDER BY job with ReplacingMergeTree, so each unique job
// string is one cursor row that gets replaced on upsert.
type CursorStore struct {
	conn      cursorConn
	jobPrefix string
}

// NewCursorStore creates a CursorStore for the given job prefix
// (e.g. "backfill").
func NewCursorStore(conn cursorConn, jobPrefix string) *CursorStore {
	return &CursorStore{conn: conn, jobPrefix: jobPrefix}
}

// cursorRow is scanned from meta.ingest_cursors.
type cursorRow struct {
	Job        string    `ch:"job"`
	RangeStart uint64    `ch:"range_start"`
	RangeEnd   uint64    `ch:"range_end"`
	Status     string    `ch:"status"`
	LastBlock  uint64    `ch:"last_block"`
	Attempts   uint32    `ch:"attempts"`
	Error      string    `ch:"error"`
	UpdatedAt  time.Time `ch:"updated_at"`
}

// Register inserts a pending cursor for chunk c if one does not already exist.
func (s *CursorStore) Register(ctx context.Context, c Chunk) error {
	key := cursorKey(s.jobPrefix, c)
	return s.conn.Exec(ctx,
		`INSERT INTO meta.ingest_cursors
		 (job, range_start, range_end, status, last_block, attempts, error, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		key, c.From, c.To, string(StatusPending), c.From, uint32(0), "", time.Now().UTC(),
	)
}

// HighestDoneBlock returns the highest range_end among all done cursors for this
// job prefix, or 0 if no done cursors exist. Used by the follower to detect gaps.
func (s *CursorStore) HighestDoneBlock(ctx context.Context) (uint64, error) {
	var result []struct {
		Max uint64 `ch:"max_end"`
	}
	if err := s.conn.Select(ctx, &result,
		`SELECT max(range_end) AS max_end
		 FROM meta.ingest_cursors FINAL
		 WHERE job LIKE ? AND status = ?`,
		s.jobPrefix+":%", string(StatusDone),
	); err != nil {
		return 0, fmt.Errorf("highest done block: %w", err)
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Max, nil
}

// MaxRawBlock returns the highest block_number stored in raw.blocks, or 0 if
// none. Used as a fallback when no cursor history exists.
func (s *CursorStore) MaxRawBlock(ctx context.Context) (uint64, error) {
	var result []struct {
		Max uint64 `ch:"max_block"`
	}
	if err := s.conn.Select(ctx, &result,
		`SELECT max(block_number) AS max_block FROM raw.blocks FINAL`); err != nil {
		return 0, fmt.Errorf("max raw block: %w", err)
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Max, nil
}

// LoadDone returns the set of cursor keys that have status=done.
func (s *CursorStore) LoadDone(ctx context.Context) (map[string]struct{}, error) {
	var rows []cursorRow
	if err := s.conn.Select(ctx, &rows,
		`SELECT job, range_start, range_end, status, last_block, attempts, error, updated_at
		 FROM meta.ingest_cursors FINAL
		 WHERE job LIKE ? AND status = ?`,
		s.jobPrefix+":%", string(StatusDone),
	); err != nil {
		return nil, fmt.Errorf("load done cursors: %w", err)
	}
	out := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		out[r.Job] = struct{}{}
	}
	return out, nil
}

// SetRunning marks the chunk as running and increments attempts.
func (s *CursorStore) SetRunning(ctx context.Context, c Chunk, attempts uint32) error {
	return s.upsert(ctx, c, StatusRunning, c.From, attempts, "")
}

// SetProgress updates the last_block pointer while the chunk is still running.
func (s *CursorStore) SetProgress(ctx context.Context, c Chunk, lastBlock uint64, attempts uint32) error {
	return s.upsert(ctx, c, StatusRunning, lastBlock, attempts, "")
}

// SetDone marks the chunk as done.
func (s *CursorStore) SetDone(ctx context.Context, c Chunk) error {
	return s.upsert(ctx, c, StatusDone, c.To, 0, "")
}

// SetFailed marks the chunk as failed with an error message.
func (s *CursorStore) SetFailed(ctx context.Context, c Chunk, attempts uint32, errMsg string) error {
	return s.upsert(ctx, c, StatusFailed, c.From, attempts, errMsg)
}

func (s *CursorStore) upsert(ctx context.Context, c Chunk, status ChunkStatus, lastBlock uint64, attempts uint32, errMsg string) error {
	key := cursorKey(s.jobPrefix, c)
	return s.conn.Exec(ctx,
		`INSERT INTO meta.ingest_cursors
		 (job, range_start, range_end, status, last_block, attempts, error, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		key, c.From, c.To, string(status), lastBlock, attempts, errMsg, time.Now().UTC(),
	)
}
