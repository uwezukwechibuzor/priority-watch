package ingest

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// BackfillMetrics holds Prometheus counters and histograms for the backfill runner.
type BackfillMetrics struct {
	BlocksIngested  prometheus.Counter
	ChunkDuration   prometheus.Histogram
	BlocksPerSecond prometheus.Gauge
}

// NewBackfillMetrics registers and returns backfill metrics in reg.
func NewBackfillMetrics(reg prometheus.Registerer) *BackfillMetrics {
	f := promauto.With(reg)
	return &BackfillMetrics{
		BlocksIngested: f.NewCounter(prometheus.CounterOpts{
			Namespace: "pw",
			Subsystem: "backfill",
			Name:      "blocks_ingested_total",
			Help:      "Total number of blocks written to raw.blocks during backfill.",
		}),
		ChunkDuration: f.NewHistogram(prometheus.HistogramOpts{
			Namespace: "pw",
			Subsystem: "backfill",
			Name:      "chunk_duration_seconds",
			Help:      "Wall-clock time to fully ingest one chunk of blocks.",
			Buckets:   prometheus.DefBuckets,
		}),
		BlocksPerSecond: f.NewGauge(prometheus.GaugeOpts{
			Namespace: "pw",
			Subsystem: "backfill",
			Name:      "blocks_per_second",
			Help:      "Rolling ingestion rate in blocks per second for the current chunk.",
		}),
	}
}
