package ingest

import "github.com/prometheus/client_golang/prometheus"

// FollowerMetrics holds Prometheus gauges and counters for the live follower.
type FollowerMetrics struct {
	HeadBlock          prometheus.Gauge
	IngestLagSeconds   prometheus.Gauge
	ReorgsTotal        prometheus.Counter
	SafeLagBlocks      prometheus.Gauge
	FinalizedLagBlocks prometheus.Gauge
}

// NewFollowerMetrics registers and returns FollowerMetrics using reg.
func NewFollowerMetrics(reg prometheus.Registerer) *FollowerMetrics {
	headBlock := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "pw",
		Subsystem: "follower",
		Name:      "head_block",
		Help:      "Latest block number ingested by the follower.",
	})
	ingestLag := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "pw",
		Subsystem: "follower",
		Name:      "ingest_lag_seconds",
		Help:      "Seconds between the chain head timestamp and now.",
	})
	reorgsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "pw",
		Subsystem: "follower",
		Name:      "reorgs_total",
		Help:      "Total reorg events detected and handled.",
	})
	safeLag := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "pw",
		Subsystem: "follower",
		Name:      "safe_lag_blocks",
		Help:      "Number of blocks the head is ahead of the safe block.",
	})
	finalizedLag := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "pw",
		Subsystem: "follower",
		Name:      "finalized_lag_blocks",
		Help:      "Number of blocks the head is ahead of the finalized block.",
	})

	reg.MustRegister(headBlock, ingestLag, reorgsTotal, safeLag, finalizedLag)

	return &FollowerMetrics{
		HeadBlock:          headBlock,
		IngestLagSeconds:   ingestLag,
		ReorgsTotal:        reorgsTotal,
		SafeLagBlocks:      safeLag,
		FinalizedLagBlocks: finalizedLag,
	}
}
