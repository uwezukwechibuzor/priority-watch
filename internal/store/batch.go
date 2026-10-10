package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/prometheus/client_golang/prometheus"
)

// BatchConn is the subset of clickhouse.Conn used by BatchWriter.
type BatchConn interface {
	PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error)
}

// BatchMetrics holds Prometheus metrics for a BatchWriter.
type BatchMetrics struct {
	rowsWritten  prometheus.Counter
	flushLatency prometheus.Observer
	flushErrors  prometheus.Counter
}

// NewBatchMetrics registers and returns metrics in reg under namespace "pw_store".
func NewBatchMetrics(reg prometheus.Registerer, table string) *BatchMetrics {
	rowsWritten := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "pw",
		Subsystem:   "store",
		Name:        "rows_written_total",
		Help:        "Total rows written to ClickHouse.",
		ConstLabels: prometheus.Labels{"table": table},
	})
	flushLatency := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace:   "pw",
		Subsystem:   "store",
		Name:        "flush_duration_seconds",
		Help:        "Batch flush latency in seconds.",
		Buckets:     prometheus.DefBuckets,
		ConstLabels: prometheus.Labels{"table": table},
	})
	flushErrors := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "pw",
		Subsystem:   "store",
		Name:        "flush_errors_total",
		Help:        "Total batch flush errors.",
		ConstLabels: prometheus.Labels{"table": table},
	})
	reg.MustRegister(rowsWritten, flushLatency, flushErrors)
	return &BatchMetrics{
		rowsWritten:  rowsWritten,
		flushLatency: flushLatency,
		flushErrors:  flushErrors,
	}
}

// BatchWriter accumulates rows and flushes them to ClickHouse as a single
// batch INSERT when either maxRows is reached or maxAge elapses.
//
// Each row is a []any slice containing one value per column, in the order
// that matches the INSERT statement passed to NewBatchWriter.
//
// Call Run in a goroutine to drive timer-based flushing; it returns when
// ctx is cancelled.  Call Flush to flush synchronously at any time.
// Always call Flush (or rely on Run's final flush) before exiting.
type BatchWriter struct {
	conn      BatchConn
	insertSQL string
	maxRows   int
	maxAge    time.Duration

	mu      sync.Mutex
	pending [][]any
	metrics *BatchMetrics
}

// NewBatchWriter creates a BatchWriter.
// insertSQL is the full INSERT statement (e.g. "INSERT INTO raw.blocks").
// maxRows triggers a flush when the buffer reaches this size.
// maxAge triggers a periodic flush; pass 0 to disable timer flushing.
// metrics may be nil (no-op in that case).
func NewBatchWriter(conn BatchConn, insertSQL string, maxRows int, maxAge time.Duration, metrics *BatchMetrics) *BatchWriter {
	return &BatchWriter{
		conn:      conn,
		insertSQL: insertSQL,
		maxRows:   maxRows,
		maxAge:    maxAge,
		metrics:   metrics,
	}
}

// Append adds a row to the pending buffer.
// If the buffer has reached maxRows, it calls Flush synchronously.
// cols must match the column count of the INSERT statement.
func (b *BatchWriter) Append(ctx context.Context, cols ...any) error {
	b.mu.Lock()
	row := make([]any, len(cols))
	copy(row, cols)
	b.pending = append(b.pending, row)
	shouldFlush := len(b.pending) >= b.maxRows
	b.mu.Unlock()

	if shouldFlush {
		return b.Flush(ctx)
	}
	return nil
}

// Flush sends all pending rows to ClickHouse in a single batch INSERT.
// It is safe to call concurrently: a single flush runs at a time.
func (b *BatchWriter) Flush(ctx context.Context) error {
	b.mu.Lock()
	if len(b.pending) == 0 {
		b.mu.Unlock()
		return nil
	}
	rows := b.pending
	b.pending = nil
	b.mu.Unlock()

	start := time.Now()
	err := b.sendBatch(ctx, rows)
	elapsed := time.Since(start).Seconds()

	if b.metrics != nil {
		b.metrics.flushLatency.Observe(elapsed)
		if err != nil {
			b.metrics.flushErrors.Add(1)
		} else {
			b.metrics.rowsWritten.Add(float64(len(rows)))
		}
	}
	return err
}

// sendBatch sends rows to ClickHouse using PrepareBatch.
func (b *BatchWriter) sendBatch(ctx context.Context, rows [][]any) error {
	batch, err := b.conn.PrepareBatch(ctx, b.insertSQL)
	if err != nil {
		return fmt.Errorf("batch prepare %s: %w", b.insertSQL, err)
	}

	for _, row := range rows {
		if err := batch.Append(row...); err != nil {
			_ = batch.Abort()
			return fmt.Errorf("batch append: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("batch send: %w", err)
	}
	return nil
}

// Run drives periodic flushing until ctx is cancelled.
// It performs a final Flush before returning.
func (b *BatchWriter) Run(ctx context.Context) error {
	if b.maxAge <= 0 {
		<-ctx.Done()
		return b.Flush(context.Background())
	}

	ticker := time.NewTicker(b.maxAge)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := b.Flush(ctx); err != nil {
				// Log but don't abort the run loop on flush errors.
				_ = err
			}
		case <-ctx.Done():
			return b.Flush(context.Background())
		}
	}
}

// SourceStatusRow is a row for meta.source_status.
type SourceStatusRow struct {
	Source      string
	LastBlockAt time.Time
	LastBlock   uint64
	Lag         float64
	IsHealthy   bool
	UpdatedAt   time.Time
}

// SourceStatusUpsert writes a single SourceStatusRow to meta.source_status.
// It uses a simple Exec (not batch) because source_status is low-frequency.
type SourceStatusUpsert struct {
	conn interface {
		Exec(ctx context.Context, query string, args ...any) error
	}
}

// NewSourceStatusUpsert creates a SourceStatusUpsert.
func NewSourceStatusUpsert(conn interface {
	Exec(ctx context.Context, query string, args ...any) error
}) *SourceStatusUpsert {
	return &SourceStatusUpsert{conn: conn}
}

// Upsert inserts or replaces a source status row.
func (u *SourceStatusUpsert) Upsert(ctx context.Context, row SourceStatusRow) error {
	if row.UpdatedAt.IsZero() {
		row.UpdatedAt = time.Now().UTC()
	}
	return u.conn.Exec(ctx,
		`INSERT INTO meta.source_status
		 (source, last_block_at, last_block, lag_seconds, is_healthy, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		row.Source,
		row.LastBlockAt.UTC(),
		row.LastBlock,
		row.Lag,
		row.IsHealthy,
		row.UpdatedAt,
	)
}
