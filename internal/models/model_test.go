package models

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"
)

// makeHist returns n identical BlockFeeRecord values with the given top fee.
func makeHist(n int, topFee, ratio, median float64) []BlockFeeRecord {
	recs := make([]BlockFeeRecord, n)
	for i := range recs {
		recs[i] = BlockFeeRecord{
			BlockNumber:        uint64(i + 1),
			TippedCount:        100,
			TopFeeGwei:         topFee,
			SecondToFirstRatio: ratio,
			MedianFeeGwei:      median,
		}
	}
	return recs
}

// ---- quantile ----------------------------------------------------------------

func TestQuantile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		xs      []float64
		p       float64
		want    float64
		wantErr error
	}{
		{name: "empty", xs: nil, p: 0.5, wantErr: errAny},
		{name: "single", xs: []float64{5}, p: 0.5, want: 5},
		{name: "two min", xs: []float64{1, 2}, p: 0.0, want: 1},
		{name: "two max", xs: []float64{1, 2}, p: 1.0, want: 2},
		{name: "two mid", xs: []float64{1, 2}, p: 0.5, want: 1.5},
		{name: "p50 uniform10", xs: seq(1, 10), p: 0.5, want: 5.5},
		{name: "p0 uniform10", xs: seq(1, 10), p: 0.0, want: 1},
		{name: "p1 uniform10", xs: seq(1, 10), p: 1.0, want: 10},
		{name: "p out of range", xs: []float64{1}, p: 1.1, wantErr: errAny},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := quantile(tc.xs, tc.p)
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(got-tc.want) > 1e-9 {
				t.Errorf("quantile(%v, %g) = %g, want %g", tc.xs, tc.p, got, tc.want)
			}
		})
	}
}

// errAny is a sentinel meaning "any non-nil error".
var errAny = errors.New("any error")

func seq(lo, hi int) []float64 {
	out := make([]float64, hi-lo+1)
	for i := range out {
		out[i] = float64(lo + i)
	}
	return out
}

// ---- feeAtRank ---------------------------------------------------------------

func TestFeeAtRank(t *testing.T) {
	t.Parallel()
	rec := BlockFeeRecord{
		TippedCount:        100,
		TopFeeGwei:         10.0,
		SecondToFirstRatio: 0.9,
		MedianFeeGwei:      5.0,
	}
	tests := []struct {
		name      string
		n         int
		wantRange [2]float64 // [lo, hi] inclusive
	}{
		{name: "rank1 = top", n: 1, wantRange: [2]float64{10.0, 10.0}},
		{name: "rank2 = top×ratio", n: 2, wantRange: [2]float64{9.0, 9.0}},
		{name: "rank50 ≈ median", n: 50, wantRange: [2]float64{4.0, 6.0}},
		{name: "rank100 < median", n: 100, wantRange: [2]float64{0.0, 5.0}},
		{name: "rank0 clamped to 1", n: 0, wantRange: [2]float64{10.0, 10.0}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := feeAtRank(rec, tc.n)
			if got < tc.wantRange[0] || got > tc.wantRange[1] {
				t.Errorf("feeAtRank(n=%d) = %g, want in [%g, %g]",
					tc.n, got, tc.wantRange[0], tc.wantRange[1])
			}
		})
	}
}

// ---- RecentPerformance -------------------------------------------------------

func TestRecentPerformanceModel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := NewRecentPerformance()

	// All blocks have rank-1 fee = 10 gwei, so P80 of rank-1 fees ≈ 10 gwei.
	hist := makeHist(50, 10.0, 0.9, 5.0)

	tests := []struct {
		name    string
		hist    []BlockFeeRecord
		req     Request
		wantFee float64
		wantErr bool
	}{
		{
			name:    "empty history",
			hist:    nil,
			req:     Request{N: 1, Confidence: 0.8, WindowBlocks: 50},
			wantErr: true,
		},
		{
			name:    "uniform top fees",
			hist:    hist,
			req:     Request{N: 1, Confidence: 0.8, WindowBlocks: 50},
			wantFee: 10.0,
		},
		{
			name:    "rank-2 confidence 0.5",
			hist:    hist,
			req:     Request{N: 2, Confidence: 0.5, WindowBlocks: 50},
			wantFee: 9.0, // rank-2 = 10 × 0.9
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec, err := m.Recommend(ctx, tc.req, tc.hist)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(rec.FeeGwei-tc.wantFee) > 0.5 {
				t.Errorf("FeeGwei = %g, want %g", rec.FeeGwei, tc.wantFee)
			}
			if diff := cmp.Diff(m.Name(), rec.ModelName); diff != "" {
				t.Errorf("ModelName mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// ---- CompetitorBehavior ------------------------------------------------------

func TestCompetitorBehaviorModel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := NewCompetitorBehavior()

	hist := func(ef float64) []BlockFeeRecord {
		recs := makeHist(20, 10.0, 0.9, 5.0)
		for i := range recs {
			recs[i].TopEntityFees = []float64{ef}
		}
		return recs
	}

	tests := []struct {
		name    string
		hist    []BlockFeeRecord
		req     Request
		wantMin float64
		wantErr bool
	}{
		// no entity fees: model falls back to TopFeeGwei (≥ 0), not an error
		{name: "no entity fees fallback", hist: makeHist(10, 10, 0.9, 5), req: Request{N: 1, Confidence: 0.8}, wantMin: 9.0},
		{name: "entity fees 8 gwei", hist: hist(8.0), req: Request{N: 1, Confidence: 0.8}, wantMin: 8.0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec, err := m.Recommend(ctx, tc.req, tc.hist)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.FeeGwei < tc.wantMin {
				t.Errorf("FeeGwei = %g, want >= %g", rec.FeeGwei, tc.wantMin)
			}
		})
	}
}

// ---- MarketCondition ---------------------------------------------------------

func TestMarketConditionModel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := NewMarketCondition()

	tests := []struct {
		name    string
		hist    []BlockFeeRecord
		req     Request
		wantGe  float64 // fee must be >= this (vol/spread pushes it up)
		wantErr bool
	}{
		{name: "empty", hist: nil, req: Request{N: 1, Confidence: 0.8}, wantErr: true},
		{
			name:   "zero vol/spread == recent-perf",
			hist:   makeHist(20, 10, 0.9, 5),
			req:    Request{N: 1, Confidence: 0.8},
			wantGe: 9.5, // should be close to 10
		},
		{
			name: "high vol pushes fee up",
			hist: func() []BlockFeeRecord {
				recs := makeHist(20, 10, 0.9, 5)
				for i := range recs {
					recs[i].Volatility = 1.0
				}
				return recs
			}(),
			req:    Request{N: 1, Confidence: 0.8},
			wantGe: 10.5, // base(~10) × (1 + 0.3×1) = 13
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec, err := m.Recommend(ctx, tc.req, tc.hist)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.FeeGwei < tc.wantGe {
				t.Errorf("FeeGwei = %g, want >= %g", rec.FeeGwei, tc.wantGe)
			}
		})
	}
}

// ---- FloorEstimate -----------------------------------------------------------

func TestFloorEstimateModel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := NewFloorEstimate()

	tests := []struct {
		name    string
		hist    []BlockFeeRecord
		req     Request
		wantMax float64
		wantErr bool
	}{
		{name: "empty", hist: nil, req: Request{N: 1, Confidence: 0.8}, wantErr: true},
		{
			name: "uniform fees — floor = rank1 fee",
			hist: makeHist(50, 10, 0.9, 5),
			req:  Request{N: 1, Confidence: 0.8},
			// floor ≤ top fee
			wantMax: 11.0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec, err := m.Recommend(ctx, tc.req, tc.hist)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.FeeGwei > tc.wantMax {
				t.Errorf("FeeGwei = %g, want <= %g", rec.FeeGwei, tc.wantMax)
			}
		})
	}
}

// ---- Monotonicity: higher confidence → higher fee ----------------------------

func TestMonotonicity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hist := makeHist(100, 10, 0.9, 5)

	confidences := []float64{0.5, 0.7, 0.9}
	for _, m := range AllModels() {
		m := m
		t.Run(m.Name(), func(t *testing.T) {
			t.Parallel()
			prev := -1.0
			for _, c := range confidences {
				rec, err := m.Recommend(ctx, Request{N: 1, Confidence: c, WindowBlocks: 100}, hist)
				if err != nil {
					t.Skipf("model returned error: %v", err)
				}
				if rec.FeeGwei < prev-1e-9 {
					t.Errorf("%s: fee(%g) = %g < fee(%g) = %g, want monotone",
						m.Name(), c, rec.FeeGwei, confidences[0], prev)
				}
				prev = rec.FeeGwei
			}
		})
	}
}

// ---- Low/High interval contains the recommended fee -------------------------

func TestIntervalContainsRecommendation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hist := makeHist(50, 10, 0.9, 5)
	req := Request{N: 1, Confidence: 0.8, WindowBlocks: 50}

	for _, m := range AllModels() {
		m := m
		t.Run(m.Name(), func(t *testing.T) {
			t.Parallel()
			rec, err := m.Recommend(ctx, req, hist)
			if err != nil {
				t.Skipf("model returned error: %v", err)
			}
			if rec.LowGwei > rec.FeeGwei+1e-9 {
				t.Errorf("LowGwei(%g) > FeeGwei(%g)", rec.LowGwei, rec.FeeGwei)
			}
			if rec.HighGwei < rec.FeeGwei-1e-9 {
				t.Errorf("HighGwei(%g) < FeeGwei(%g)", rec.HighGwei, rec.FeeGwei)
			}
		})
	}
}

// ---- Property: quantile is bounded by min/max of input ----------------------

func TestQuantileProperty(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		xs := rapid.SliceOfN(rapid.Float64Range(0.001, 1000.0), 1, 100).Draw(rt, "xs")
		p := rapid.Float64Range(0, 1).Draw(rt, "p")

		v, err := quantile(xs, p)
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		lo, hi := xs[0], xs[0]
		for _, x := range xs {
			if x < lo {
				lo = x
			}
			if x > hi {
				hi = x
			}
		}
		if v < lo-1e-9 || v > hi+1e-9 {
			rt.Fatalf("quantile(%g) = %g outside [%g, %g]", p, v, lo, hi)
		}
	})
}
