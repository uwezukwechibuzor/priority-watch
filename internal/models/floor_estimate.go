package models

import (
	"context"
	"fmt"
	"sort"
)

// FloorEstimate recommends the lowest historical fee that achieved
// rank ≤ N with at least the requested confidence.
// It differs from recent-performance in that it searches for the minimum
// sufficient fee rather than the confidence quantile.
type FloorEstimate struct{}

func NewFloorEstimate() *FloorEstimate { return &FloorEstimate{} }

func (*FloorEstimate) Name() string { return "floor-estimate" }

func (*FloorEstimate) Recommend(_ context.Context, req Request, hist []BlockFeeRecord) (Recommendation, error) {
	if len(hist) == 0 {
		return Recommendation{}, fmt.Errorf("floor-estimate: no history")
	}

	fees := collectRankFees(hist, req.N)
	n := len(fees)

	// Sort a copy to walk candidate thresholds in ascending order.
	sorted := make([]float64, n)
	copy(sorted, fees)
	sort.Float64s(sorted)

	// Find the smallest fee F such that fraction(fees ≤ F) ≥ confidence.
	// Since sorted[k] is the (k+1)/n quantile, the first index where
	// (k+1)/n ≥ confidence gives us the floor.
	floor := sorted[n-1] // start at max
	for k, f := range sorted {
		if float64(k+1)/float64(n) >= req.Confidence {
			floor = f
			break
		}
	}

	low, _ := quantile(fees, 0.10)  //nolint:errcheck
	high, _ := quantile(fees, 0.90) //nolint:errcheck

	hits := 0
	for _, f := range fees {
		if floor >= f {
			hits++
		}
	}
	hitRate := float64(hits) / float64(len(fees))

	return Recommendation{
		FeeGwei:   floor,
		LowGwei:   low,
		HighGwei:  high,
		HitRate:   hitRate,
		AvgCost:   floor,
		ModelName: "floor-estimate",
		Reasoning: []TraceStep{
			{Label: "blocks sampled", Value: float64(n)},
			{Label: "target rank", Value: float64(req.N)},
			{Label: "confidence", Value: req.Confidence},
			{Label: "fee p10", Value: low, Unit: "gwei"},
			{Label: "fee p90", Value: high, Unit: "gwei"},
			{Label: "floor fee", Value: floor, Unit: "gwei"},
			{Label: "achieved hit rate", Value: hitRate},
		},
	}, nil
}
