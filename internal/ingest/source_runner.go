package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
	"github.com/uwezukwechibuzor/priority-watch/internal/store"
)

// SourceBackfillRunner performs a resumable, idempotent backfill using a
// chain.BlockSource (SQD, HyperSync, RPC or failover) instead of per-block RPC
// calls. It reads the streamed FetchedBlock channel sequentially and writes to
// the batch writers, tracking per-chunk progress in meta.ingest_cursors.
type SourceBackfillRunner struct {
	source  chain.BlockSource
	cursors *CursorStore
	writers *Writers
	metrics *BackfillMetrics
	log     *slog.Logger
}

// NewSourceBackfillRunner creates a SourceBackfillRunner.
func NewSourceBackfillRunner(
	source chain.BlockSource,
	cursors *CursorStore,
	writers *Writers,
	metrics *BackfillMetrics,
	log *slog.Logger,
) *SourceBackfillRunner {
	return &SourceBackfillRunner{
		source:  source,
		cursors: cursors,
		writers: writers,
		metrics: metrics,
		log:     log,
	}
}

// Run executes the backfill for the range in cfg using the configured source.
func (r *SourceBackfillRunner) Run(ctx context.Context, cfg BackfillConfig) error {
	if cfg.ChunkSize == 0 {
		cfg.ChunkSize = DefaultChunkSize
	}

	chunks := PlanChunks(cfg.From, cfg.To, cfg.ChunkSize)

	for _, c := range chunks {
		if err := r.cursors.Register(ctx, c); err != nil {
			return fmt.Errorf("register chunks: %w", err)
		}
	}

	done, err := r.cursors.LoadDone(ctx)
	if err != nil {
		return fmt.Errorf("load done chunks: %w", err)
	}

	for _, c := range chunks {
		if _, skip := done[cursorKey(r.cursors.jobPrefix, c)]; skip {
			r.log.InfoContext(ctx, "skipping done chunk", "from", c.From, "to", c.To)
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := r.runChunk(ctx, c, cfg); err != nil {
			return fmt.Errorf("chunk %d-%d: %w", c.From, c.To, err)
		}
	}
	return nil
}

// runChunk streams one chunk from the source and writes each block.
func (r *SourceBackfillRunner) runChunk(ctx context.Context, c Chunk, cfg BackfillConfig) error {
	start := time.Now()
	now := time.Now().UTC()

	if err := r.cursors.SetRunning(ctx, c, 1); err != nil {
		r.log.WarnContext(ctx, "set running failed", "err", err)
	}

	blocks, errc := r.source.Range(ctx, c.From, c.To, cfg.LogAddresses)

	var count uint64
	for fb := range blocks {
		if err := BlockFromSource(ctx, fb, r.writers, now, cfg.LogsOnly); err != nil {
			_ = r.cursors.SetFailed(ctx, c, 1, err.Error())
			return fmt.Errorf("write block %d: %w", uint64(fb.Block.Number), err)
		}
		n := uint64(fb.Block.Number)
		count++
		if r.metrics != nil {
			r.metrics.BlocksIngested.Add(1)
		}
		_ = r.cursors.SetProgress(ctx, c, n, 1)
	}
	if err := <-errc; err != nil {
		_ = r.cursors.SetFailed(ctx, c, 1, err.Error())
		return err
	}

	elapsed := time.Since(start).Seconds()
	if r.metrics != nil {
		r.metrics.ChunkDuration.Observe(elapsed)
		if elapsed > 0 {
			r.metrics.BlocksPerSecond.Set(float64(count) / elapsed)
		}
	}

	if err := r.cursors.SetDone(ctx, c); err != nil {
		r.log.WarnContext(ctx, "set done failed", "err", err)
	}

	r.log.InfoContext(ctx, "chunk done",
		"source", r.source.Name(), "from", c.From, "to", c.To,
		"blocks", count, "elapsed_s", fmt.Sprintf("%.1f", elapsed),
	)
	return nil
}

// SourceFollower follows the chain tip using a chain.BlockSource.
// It writes each block and records reorgs (reported via FetchedBlock.Reorg) to
// raw.reorg_log. It is the BlockSource counterpart of Follower.
type SourceFollower struct {
	source  chain.BlockSource
	cursors *CursorStore
	writers *FollowerWriters
	metrics *FollowerMetrics
	cfg     FollowerConfig
	log     *slog.Logger

	lastWrite time.Time
}

// NewSourceFollower creates a SourceFollower.
func NewSourceFollower(
	source chain.BlockSource,
	cursors *CursorStore,
	writers *FollowerWriters,
	metrics *FollowerMetrics,
	cfg FollowerConfig,
	log *slog.Logger,
) *SourceFollower {
	cfg.setDefaults()
	return &SourceFollower{
		source:  source,
		cursors: cursors,
		writers: writers,
		metrics: metrics,
		cfg:     cfg,
		log:     log,
	}
}

// HealthFunc returns a health-check function for obs.Server.SetHealthCheck.
func (f *SourceFollower) HealthFunc() func() error {
	return func() error {
		if !f.lastWrite.IsZero() && time.Since(f.lastWrite) > 30*time.Second {
			return fmt.Errorf("no write in %s", time.Since(f.lastWrite).Truncate(time.Second))
		}
		return nil
	}
}

// Run starts the follower loop. It blocks until ctx is cancelled.
func (f *SourceFollower) Run(ctx context.Context) error {
	start, err := f.resolveStart(ctx)
	if err != nil {
		return fmt.Errorf("source follower startup: %w", err)
	}
	f.log.InfoContext(ctx, "source follower started", "source", f.source.Name(), "from", start)

	blocks, errc := f.source.Follow(ctx, start, f.cfg.LogAddresses)
	for fb := range blocks {
		if err := f.handle(ctx, fb); err != nil {
			f.log.ErrorContext(ctx, "handle block error",
				"block", uint64(fb.Block.Number), "err", err)
		}
	}
	if err := <-errc; err != nil && ctx.Err() == nil {
		return err
	}
	return ctx.Err()
}

// resolveStart returns the block number the follower should begin at: one past
// the highest stored block, or the chain head when nothing is stored.
func (f *SourceFollower) resolveStart(ctx context.Context) (uint64, error) {
	lastStored, err := f.cursors.HighestDoneBlock(ctx)
	if err != nil {
		f.log.WarnContext(ctx, "highest done block query failed, assuming 0", "err", err)
		lastStored = 0
	}
	if lastStored > 0 {
		return lastStored + 1, nil
	}
	head, err := f.source.Head(ctx)
	if err != nil {
		return 0, fmt.Errorf("get head: %w", err)
	}
	return head, nil
}

// handle writes one FetchedBlock, logging any reported reorg.
func (f *SourceFollower) handle(ctx context.Context, fb *chain.FetchedBlock) error {
	if fb == nil || fb.Block == nil {
		return fmt.Errorf("nil block")
	}
	now := time.Now().UTC()
	n := uint64(fb.Block.Number)

	if fb.Reorg != nil && f.writers.ReorgLog != nil {
		if err := f.writers.ReorgLog.Append(ctx,
			now, fb.Reorg.CommonAncestor+1, fb.Reorg.OldHash, fb.Reorg.NewHash, fb.Reorg.Depth,
		); err != nil {
			f.log.WarnContext(ctx, "reorg log append failed", "err", err)
		}
		if f.metrics != nil {
			f.metrics.ReorgsTotal.Add(1)
		}
		f.log.WarnContext(ctx, "reorg reported by source",
			"source", f.source.Name(), "block", n,
			"ancestor", fb.Reorg.CommonAncestor, "depth", fb.Reorg.Depth)
	}

	w := &Writers{
		Blocks:   f.writers.Blocks,
		Txs:      f.writers.Txs,
		Receipts: f.writers.Receipts,
		Logs:     f.writers.Logs,
	}
	if err := BlockFromSource(ctx, fb, w, now, false); err != nil {
		return fmt.Errorf("write block %d: %w", n, err)
	}
	f.lastWrite = now

	if f.metrics != nil {
		f.metrics.HeadBlock.Set(float64(n))
		ts := time.Unix(int64(fb.Block.Timestamp), 0) //nolint:gosec
		f.metrics.IngestLagSeconds.Set(time.Since(ts).Seconds())
	}

	if f.writers.StatusUpsert != nil {
		_ = f.writers.StatusUpsert.Upsert(ctx, store.SourceStatusRow{
			Source:          "follower-" + f.source.Name(),
			Rows:            1,
			LatestPosition:  strconv.FormatUint(n, 10),
			LatestTimestamp: time.Unix(int64(fb.Block.Timestamp), 0), //nolint:gosec
			UpdatedAt:       now,
		})
	}
	return nil
}
