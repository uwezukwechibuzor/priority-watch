package models

import (
	"context"
	"fmt"
)

// CompetitorBehavior recommends a fee based on recent fee levels of the
// top entities by tip share.  For each block, it uses the maximum fee
// paid by any observed entity; the recommendation is the confidence-
// quantile of that distribution, raised slightly to beat competitors.
type CompetitorBehavior struct{}

func NewCompetitorBehavior() *CompetitorBehavior { return &CompetitorBehavior{} }

func (*CompetitorBehavior) Name() string { return "competitor-behavior" }

func (*CompetitorBehavior) Recommend(_ context.Context, req Request, hist []BlockFeeRecord) (Recommendation, error) {
	if len(hist) == 0 {
		return Recommendation{}, fmt.Errorf("competitor-behavior: no history")
	}

	// Collect the max entity fee per block (proxy for dominant competitor's bid).
	competitorFees := make([]float64, 0, len(hist))
	entityCount := 0
	for _, h := range hist {
		if len(h.TopEntityFees) == 0 {
			// Fall back to rank-1 fee when entity data is absent.
			competitorFees = append(competitorFees, h.TopFeeGwei)
			continue
		}
		maxFee := 0.0
		for _, f := range h.TopEntityFees {
			if f > maxFee {
				maxFee = f
			}
		}
		entityCount += len(h.TopEntityFees)
		competitorFees = append(competitorFees, maxFee)
	}

	base, err := quantile(competitorFees, req.Confidence)
	if err != nil {
		return Recommendation{}, fmt.Errorf("competitor-behavior: %w", err)
	}
	// Add a small premium (1 %) to beat, not merely match, the competitor.
	rec := base * 1.01

	low, _ := quantile(competitorFees, 0.10)  //nolint:errcheck
	high, _ := quantile(competitorFees, 0.90) //nolint:errcheck
	if high < rec {
		high = rec // interval must contain the recommendation
	}

	hits := 0
	rankFees := collectRankFees(hist, req.N)
	for _, f := range rankFees {
		if rec >= f {
			hits++
		}
	}
	hitRate := float64(hits) / float64(len(rankFees))

	return Recommendation{
		FeeGwei:   rec,
		LowGwei:   low,
		HighGwei:  high,
		HitRate:   hitRate,
		AvgCost:   rec,
		ModelName: "competitor-behavior",
		Reasoning: []TraceStep{
			{Label: "blocks sampled", Value: float64(len(hist))},
			{Label: "entity observations", Value: float64(entityCount)},
			{Label: "competitor fee p10", Value: low, Unit: "gwei"},
			{Label: "competitor fee p90", Value: high, Unit: "gwei"},
			{Label: "base (competitor quantile)", Value: base, Unit: "gwei"},
			{Label: "beat-competitor premium", Value: 0.01, Unit: "fraction"},
			{Label: "recommended fee", Value: rec, Unit: "gwei"},
		},
	}, nil
}
