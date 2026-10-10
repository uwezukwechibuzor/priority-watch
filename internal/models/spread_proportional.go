package models

import (
	"context"
	"math"
)

// SpreadProportional bids alpha × estimated arb profit (tip = α × E[profit]).
// When profit is unknown or zero, it falls back to the recent p50 top fee.
type SpreadProportional struct {
	alpha float64 // fraction of expected profit to bid as tip
}

func NewSpreadProportional(alpha float64) *SpreadProportional {
	return &SpreadProportional{alpha: alpha}
}

func (s *SpreadProportional) Name() string { return "spread-proportional" }

func (s *SpreadProportional) Tip(_ context.Context, opp Opportunity, recent []BlockFeeRecord) (float64, error) {
	bid := s.alpha * opp.EstProfitGwei
	if bid > 0 {
		return bid, nil
	}
	// Fallback: P50 of recent top fees.
	if len(recent) > 0 {
		tops := make([]float64, len(recent))
		for i, r := range recent {
			tops[i] = r.TopFeeGwei
		}
		v, err := quantile(tops, 0.50)
		if err == nil {
			return math.Max(v, 0), nil
		}
	}
	return 0, nil
}
