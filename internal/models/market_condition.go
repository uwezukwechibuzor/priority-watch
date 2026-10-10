package models

import (
	"context"
	"fmt"
	"math"
)

// MarketCondition adjusts the recent-performance base estimate by
// 1-minute realised CEX volatility and DEX/CEX spread.
// High vol or wide spread raises the premium; quiet markets lower it.
type MarketCondition struct {
	// VolCoef is the multiplier applied to normalised volatility.
	// A value of 0.5 means 1 unit of vol adds 50 % to the base estimate.
	VolCoef float64
	// SpreadCoef is the multiplier applied to the normalised spread.
	SpreadCoef float64
}

const (
	defaultVolCoef    = 0.30
	defaultSpreadCoef = 0.20
)

func NewMarketCondition() *MarketCondition {
	return &MarketCondition{VolCoef: defaultVolCoef, SpreadCoef: defaultSpreadCoef}
}

func (*MarketCondition) Name() string { return "market-condition" }

func (m *MarketCondition) Recommend(_ context.Context, req Request, hist []BlockFeeRecord) (Recommendation, error) {
	if len(hist) == 0 {
		return Recommendation{}, fmt.Errorf("market-condition: no history")
	}

	fees := collectRankFees(hist, req.N)
	base, err := quantile(fees, req.Confidence)
	if err != nil {
		return Recommendation{}, fmt.Errorf("market-condition: %w", err)
	}
	low, _ := quantile(fees, 0.10)  //nolint:errcheck
	high, _ := quantile(fees, 0.90) //nolint:errcheck

	// Compute average vol and spread over the history window.
	avgVol, avgSpread := 0.0, 0.0
	for _, h := range hist {
		avgVol += h.Volatility
		avgSpread += h.DexSpread
	}
	avgVol /= float64(len(hist))
	avgSpread /= float64(len(hist))

	// Adjustment factor: 1 + vol_term + spread_term, minimum 1.
	volCoef := m.VolCoef
	spreadCoef := m.SpreadCoef
	adjustment := 1.0 + volCoef*avgVol + spreadCoef*avgSpread
	adjustment = math.Max(adjustment, 1.0)

	rec := base * adjustment

	hits := 0
	for _, f := range fees {
		if rec >= f {
			hits++
		}
	}
	hitRate := float64(hits) / float64(len(fees))

	return Recommendation{
		FeeGwei:   rec,
		LowGwei:   low * adjustment,
		HighGwei:  high * adjustment,
		HitRate:   hitRate,
		AvgCost:   rec,
		ModelName: "market-condition",
		Reasoning: []TraceStep{
			{Label: "blocks sampled", Value: float64(len(hist))},
			{Label: "base (recent-performance)", Value: base, Unit: "gwei"},
			{Label: "avg 1-min vol", Value: avgVol},
			{Label: "avg dex spread", Value: avgSpread},
			{Label: "vol coefficient", Value: volCoef},
			{Label: "spread coefficient", Value: spreadCoef},
			{Label: "adjustment factor", Value: adjustment},
			{Label: "recommended fee", Value: rec, Unit: "gwei"},
		},
	}, nil
}
