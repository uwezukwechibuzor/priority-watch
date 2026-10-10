package models

import (
	"context"
	"fmt"
)

// PercentileOfRecent bids the p-th percentile of the top fees observed
// in the last W blocks (W = len(recent)).
type PercentileOfRecent struct {
	percentile float64 // 0–1
}

func NewPercentileOfRecent(p float64) *PercentileOfRecent {
	return &PercentileOfRecent{percentile: p}
}

func (s *PercentileOfRecent) Name() string {
	return fmt.Sprintf("percentile-of-recent(%.4g)", s.percentile)
}

func (s *PercentileOfRecent) Tip(_ context.Context, _ Opportunity, recent []BlockFeeRecord) (float64, error) {
	if len(recent) == 0 {
		return 0, fmt.Errorf("percentile-of-recent: no recent history")
	}
	tops := make([]float64, len(recent))
	for i, r := range recent {
		tops[i] = r.TopFeeGwei
	}
	v, err := quantile(tops, s.percentile)
	if err != nil {
		return 0, fmt.Errorf("percentile-of-recent: %w", err)
	}
	return v, nil
}
