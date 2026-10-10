package store_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/go-cmp/cmp"
	"github.com/uwezukwechibuzor/priority-watch/internal/store"
)

// ── fake Batch ───────────────────────────────────────────────────────────────

type fakeBatch struct {
	rows   [][]any
	mu     sync.Mutex
	sendFn func() error
}

func (b *fakeBatch) Append(v ...any) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	row := make([]any, len(v))
	copy(row, v)
	b.rows = append(b.rows, row)
	return nil
}

func (b *fakeBatch) AppendStruct(_ any) error              { return nil }
func (b *fakeBatch) Column(_ int) driver.BatchColumn       { return nil }
func (b *fakeBatch) Flush() error                          { return b.Send() }
func (b *fakeBatch) Abort() error                          { return nil }
func (b *fakeBatch) Close() error                          { return nil }
func (b *fakeBatch) IsSent() bool                          { return false }
func (b *fakeBatch) Rows() int                             { return len(b.rows) }
func (b *fakeBatch) Columns() []column.Interface           { return nil }

func (b *fakeBatch) Send() error {
	if b.sendFn != nil {
		return b.sendFn()
	}
	return nil
}

// ── fake BatchConn ───────────────────────────────────────────────────────────

type fakeBatchConn struct {
	batches []*fakeBatch
	mu      sync.Mutex
	sendErr error
}

func (c *fakeBatchConn) PrepareBatch(_ context.Context, _ string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	b := &fakeBatch{sendFn: func() error { return c.sendErr }}
	c.batches = append(c.batches, b)
	return b, nil
}

func (c *fakeBatchConn) totalRows() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, b := range c.batches {
		n += len(b.rows)
	}
	return n
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestBatchWriterFlush(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rows     int
		maxRows  int
		wantRows int
		wantErr  bool
	}{
		{
			name:     "appending below maxRows does not flush automatically",
			rows:     5,
			maxRows:  100,
			wantRows: 5,
			wantErr:  false,
		},
		{
			name:     "flush sends all pending rows",
			rows:     10,
			maxRows:  100,
			wantRows: 10,
			wantErr:  false,
		},
		{
			name:     "flush on empty buffer is no-op",
			rows:     0,
			maxRows:  100,
			wantRows: 0,
			wantErr:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			conn := &fakeBatchConn{}
			w := store.NewBatchWriter(conn, "INSERT INTO raw.blocks", tc.maxRows, 0, nil)

			ctx := context.Background()
			for i := 0; i < tc.rows; i++ {
				if err := w.Append(ctx, fmt.Sprintf("row%d", i)); err != nil {
					t.Fatalf("Append: %v", err)
				}
			}
			if err := w.Flush(ctx); err != nil {
				if tc.wantErr {
					return
				}
				t.Fatalf("Flush: %v", err)
			}
			if diff := cmp.Diff(tc.wantRows, conn.totalRows()); diff != "" {
				t.Errorf("row count mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBatchWriterAutoFlushOnMaxRows(t *testing.T) {
	t.Parallel()

	const maxRows = 5
	conn := &fakeBatchConn{}
	w := store.NewBatchWriter(conn, "INSERT INTO raw.blocks", maxRows, 0, nil)
	ctx := context.Background()

	// Append maxRows rows; the last append should trigger an auto-flush.
	for i := 0; i < maxRows; i++ {
		if err := w.Append(ctx, i); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	// The auto-flush sends all rows once maxRows is reached.
	if got := conn.totalRows(); got != maxRows {
		t.Errorf("totalRows after auto-flush = %d, want %d", got, maxRows)
	}
}

func TestBatchWriterFlushError(t *testing.T) {
	t.Parallel()

	conn := &fakeBatchConn{sendErr: errors.New("clickhouse unavailable")}
	w := store.NewBatchWriter(conn, "INSERT INTO raw.blocks", 100, 0, nil)
	ctx := context.Background()

	if err := w.Append(ctx, "row"); err != nil {
		t.Fatalf("Append: %v", err)
	}
	err := w.Flush(ctx)
	if err == nil {
		t.Fatal("expected flush error, got nil")
	}
	if !errors.Is(err, conn.sendErr) && !containsErr(err, "clickhouse unavailable") {
		t.Errorf("error %q should mention the send error", err)
	}
}

func TestBatchWriterRunPeriodic(t *testing.T) {
	t.Parallel()

	conn := &fakeBatchConn{}
	interval := 50 * time.Millisecond
	w := store.NewBatchWriter(conn, "INSERT INTO raw.blocks", 1000, interval, nil)
	ctx, cancel := context.WithCancel(context.Background())

	var rowsAppended atomic.Int64
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
				_ = w.Append(ctx, "row")
				rowsAppended.Add(1)
			}
		}
	}()

	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after cancel")
	}

	// All appended rows should be flushed by the final flush in Run.
	if got, want := conn.totalRows(), int(rowsAppended.Load()); got != want {
		t.Errorf("totalRows = %d, want %d", got, want)
	}
}

func containsErr(err error, substr string) bool {
	if err == nil {
		return false
	}
	return len(err.Error()) >= len(substr) && func() bool {
		s, sub := err.Error(), substr
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
