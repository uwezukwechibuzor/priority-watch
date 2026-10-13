package ingest

import "github.com/prometheus/client_golang/prometheus"

// FollowerMetrics holds Prometheus gauges and counters for the live follower.
type FollowerMetrics struct {
	HeadBlock          prometheus.Gauge
	IngestLagSeconds   prometheus.Gauge
	LagBlocks          prometheus.Gauge
	BlocksTotal        prometheus.Counter
	RpcErrorsTotal     prometheus.Counter
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
	lagBlocks := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "pw",
		Subsystem: "follower",
		Name:      "lag_blocks",
		Help:      "Number of blocks the follower is behind the chain tip.",
	})
	blocksTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "pw",
		Subsystem: "follower",
		Name:      "blocks_total",
		Help:      "Total blocks ingested by the follower.",
	})
	rpcErrorsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "pw",
		Subsystem: "follower",
		Name:      "rpc_errors_total",
		Help:      "Total errors returned by the block source (SQD, HyperSync, or RPC).",
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

	reg.MustRegister(headBlock, ingestLag, lagBlocks, blocksTotal, rpcErrorsTotal, reorgsTotal, safeLag, finalizedLag)

	return &FollowerMetrics{
		HeadBlock:          headBlock,
		IngestLagSeconds:   ingestLag,
		LagBlocks:          lagBlocks,
		BlocksTotal:        blocksTotal,
		RpcErrorsTotal:     rpcErrorsTotal,
		ReorgsTotal:        reorgsTotal,
		SafeLagBlocks:      safeLag,
		FinalizedLagBlocks: finalizedLag,
	}
}
