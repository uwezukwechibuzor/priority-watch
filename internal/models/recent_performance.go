package models

import (
	"context"
	"fmt"
)

// RecentPerformance recommends the empirical quantile of the fee needed
// for rank ≤ N over the last W blocks.
type RecentPerformance struct{}

func NewRecentPerformance() *RecentPerformance { return &RecentPerformance{} }

func (*RecentPerformance) Name() string { return "recent-performance" }

func (*RecentPerformance) Recommend(_ context.Context, req Request, hist []BlockFeeRecord) (Recommendation, error) {
	if len(hist) == 0 {
		return Recommendation{}, fmt.Errorf("recent-performance: no history")
	}

	fees := collectRankFees(hist, req.N)

	rec, err := quantile(fees, req.Confidence)
	if err != nil {
		return Recommendation{}, fmt.Errorf("recent-performance: %w", err)
	}
	low, err := quantile(fees, 0.10)
	if err != nil {
		return Recommendation{}, err
	}
	high, err := quantile(fees, 0.90)
	if err != nil {
		return Recommendation{}, err
	}

	// Hit rate at recommended fee: fraction of blocks where fee >= actual rank-N fee
	hits := 0
	for _, f := range fees {
		if rec >= f {
			hits++
		}
	}
	hitRate := float64(hits) / float64(len(fees))

	return Recommendation{
		FeeGwei:   rec,
		LowGwei:   low,
		HighGwei:  high,
		HitRate:   hitRate,
		AvgCost:   rec,
		ModelName: "recent-performance",
		Reasoning: []TraceStep{
			{Label: "blocks sampled", Value: float64(len(hist))},
			{Label: "target rank", Value: float64(req.N)},
			{Label: "confidence", Value: req.Confidence},
			{Label: "rank-N fee p10", Value: low, Unit: "gwei"},
			{Label: "rank-N fee p90", Value: high, Unit: "gwei"},
			{Label: "recommended fee", Value: rec, Unit: "gwei"},
		},
	}, nil
}
