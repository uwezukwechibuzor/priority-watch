// Package models implements priority-fee pricing models and the backtest engine
// used by the /pricing and /simulator API endpoints.
package models

import (
	"context"
	"time"
)

// BlockFeeRecord is a precomputed summary of one block's fee landscape,
// derived from marts.fct_blocks and associated CEX/DEX data.
type BlockFeeRecord struct {
	BlockNumber        uint64
	Timestamp          time.Time
	TippedCount        uint64
	TopFeeGwei         float64 // rank-1 fee
	SecondToFirstRatio float64 // rank-2 fee / rank-1 fee
	MedianFeeGwei      float64 // median tipped fee
	Volatility         float64 // 1-min normalised CEX realised vol [0, +∞)
	DexSpread          float64 // DEX/CEX spread fraction [0, +∞)
	TopEntityFees      []float64
}

// TraceStep is one step in a model's reasoning chain.
type TraceStep struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
}

// Recommendation is the full output of a pricing model.
type Recommendation struct {
	FeeGwei   float64     `json:"fee_gwei"`
	LowGwei   float64     `json:"low_gwei"`  // 10th-pct of modelled distribution
	HighGwei  float64     `json:"high_gwei"` // 90th-pct (together: 80 % interval)
	HitRate   float64     `json:"hit_rate"`  // expected fraction of blocks won
	AvgCost   float64     `json:"avg_cost_gwei"`
	ModelName string      `json:"model_name"`
	Reasoning []TraceStep `json:"reasoning"`
}

// Request describes what the caller wants from a pricing model.
type Request struct {
	// N is the desired rank position (1 = top of block, 2 = second, …).
	N int
	// Confidence is the desired fraction of blocks where the recommendation
	// achieves rank ≤ N, e.g. 0.80 for 80 %.
	Confidence float64
	// WindowBlocks is how many recent blocks to use as history.
	WindowBlocks int
}

// Model is the interface every pricing model must satisfy.
type Model interface {
	Name() string
	Recommend(ctx context.Context, req Request, hist []BlockFeeRecord) (Recommendation, error)
}

// AllModels returns one instance of every built-in pricing model.
func AllModels() []Model {
	return []Model{
		NewRecentPerformance(),
		NewCompetitorBehavior(),
		NewMarketCondition(),
		NewFloorEstimate(),
	}
}
