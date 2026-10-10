package models

import (
	"context"
	"fmt"
)

// FixedTip always bids a constant priority fee regardless of market conditions.
type FixedTip struct {
	feeGwei float64
}

func NewFixedTip(feeGwei float64) *FixedTip { return &FixedTip{feeGwei: feeGwei} }

func (s *FixedTip) Name() string { return fmt.Sprintf("fixed-tip(%.4g)", s.feeGwei) }

func (s *FixedTip) Tip(_ context.Context, _ Opportunity, _ []BlockFeeRecord) (float64, error) {
	return s.feeGwei, nil
}
