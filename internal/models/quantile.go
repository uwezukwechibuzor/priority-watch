package models

import (
	"fmt"
	"math"
	"sort"
)

// quantile returns the p-th quantile (0 ≤ p ≤ 1) of xs using linear interpolation.
// xs need not be sorted; a copy is sorted internally.
func quantile(xs []float64, p float64) (float64, error) {
	if len(xs) == 0 {
		return 0, fmt.Errorf("quantile: empty slice")
	}
	if p < 0 || p > 1 {
		return 0, fmt.Errorf("quantile: p must be in [0,1], got %g", p)
	}
	cp := make([]float64, len(xs))
	copy(cp, xs)
	sort.Float64s(cp)

	if len(cp) == 1 {
		return cp[0], nil
	}
	// Linear interpolation (R type 7)
	h := p * float64(len(cp)-1)
	lo := int(math.Floor(h))
	hi := int(math.Ceil(h))
	if lo == hi {
		return cp[lo], nil
	}
	return cp[lo] + (h-float64(lo))*(cp[hi]-cp[lo]), nil
}

// feeAtRank estimates the fee required for rank N in a block using
// log-linear interpolation between known anchor points:
//   - rank 1  →  TopFeeGwei
//   - rank 2  →  TopFeeGwei × SecondToFirstRatio
//   - rank T/2 →  MedianFeeGwei  (T = TippedCount)
func feeAtRank(rec BlockFeeRecord, n int) float64 {
	if n <= 0 {
		n = 1
	}
	T := int(rec.TippedCount) //nolint:gosec
	if T == 0 {
		return 0
	}
	if n == 1 {
		return rec.TopFeeGwei
	}
	// Clamp to bottom of block
	if n >= T {
		return math.Max(rec.MedianFeeGwei*0.3, 0)
	}

	rank2 := rec.TopFeeGwei * rec.SecondToFirstRatio
	if rank2 <= 0 {
		rank2 = rec.TopFeeGwei * 0.9 // fallback if ratio missing
	}
	if n == 2 {
		return rank2
	}

	medianRank := T / 2
	if medianRank < 2 {
		medianRank = 2
	}

	if n <= medianRank {
		// log-linear interpolation between rank-2 and median
		t := float64(n-2) / math.Max(1, float64(medianRank-2))
		l2 := math.Log(math.Max(rank2, 1e-9))
		lm := math.Log(math.Max(rec.MedianFeeGwei, 1e-9))
		return math.Exp(l2 + t*(lm-l2))
	}
	// log-linear interpolation from median down to ~10 % of median
	t := float64(n-medianRank) / math.Max(1, float64(T-medianRank))
	lm := math.Log(math.Max(rec.MedianFeeGwei, 1e-9))
	lf := math.Log(math.Max(rec.MedianFeeGwei*0.1, 1e-9))
	return math.Exp(lm + t*(lf-lm))
}

// collectRankFees returns, for each block in hist, the estimated fee
// required for rank n.
func collectRankFees(hist []BlockFeeRecord, n int) []float64 {
	out := make([]float64, 0, len(hist))
	for _, h := range hist {
		out = append(out, feeAtRank(h, n))
	}
	return out
}
