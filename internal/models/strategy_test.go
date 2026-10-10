package models

import (
	"context"
	"errors"
	"math"
	"testing"

	"pgregory.net/rapid"
)

// ---- FixedTip ----------------------------------------------------------------

func TestFixedTipStrategy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		fee     float64
		wantTip float64
	}{
		{name: "zero fee", fee: 0, wantTip: 0},
		{name: "positive fee", fee: 5.5, wantTip: 5.5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := NewFixedTip(tc.fee)
			got, err := s.Tip(context.Background(), Opportunity{}, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(got-tc.wantTip) > 1e-9 {
				t.Errorf("Tip = %g, want %g", got, tc.wantTip)
			}
		})
	}
}

// ---- PercentileOfRecent ------------------------------------------------------

func TestPercentileOfRecentStrategy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		p       float64
		recent  []BlockFeeRecord
		wantTip float64
		wantErr bool
	}{
		{
			name:    "no recent",
			p:       0.5,
			recent:  nil,
			wantErr: true,
		},
		{
			name:    "p50 uniform",
			p:       0.5,
			recent:  makeHist(10, 10, 0.9, 5),
			wantTip: 10.0, // all tops = 10
		},
		{
			name: "p90 of varying tops",
			p:    0.9,
			recent: func() []BlockFeeRecord {
				recs := make([]BlockFeeRecord, 10)
				for i := range recs {
					recs[i].TippedCount = 100
					recs[i].TopFeeGwei = float64(i + 1)
				}
				return recs
			}(),
			// tops=[1..10], p90 = 9.1
			wantTip: 9.1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := NewPercentileOfRecent(tc.p)
			got, err := s.Tip(context.Background(), Opportunity{}, tc.recent)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(got-tc.wantTip) > 0.01 {
				t.Errorf("Tip = %g, want %g", got, tc.wantTip)
			}
		})
	}
}

// ---- SpreadProportional ------------------------------------------------------

func TestSpreadProportionalStrategy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		alpha   float64
		opp     Opportunity
		recent  []BlockFeeRecord
		wantTip float64
	}{
		{
			name:    "alpha × profit",
			alpha:   0.5,
			opp:     Opportunity{EstProfitGwei: 10},
			recent:  nil,
			wantTip: 5.0,
		},
		{
			name:    "zero profit fallback to p50",
			alpha:   0.5,
			opp:     Opportunity{EstProfitGwei: 0},
			recent:  makeHist(10, 8, 0.9, 4),
			wantTip: 8.0, // p50 of all-8 tops = 8
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := NewSpreadProportional(tc.alpha)
			got, err := s.Tip(context.Background(), tc.opp, tc.recent)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(got-tc.wantTip) > 0.01 {
				t.Errorf("Tip = %g, want %g", got, tc.wantTip)
			}
		})
	}
}

// ---- Replay ------------------------------------------------------------------

func TestReplay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hist := makeHist(20, 10, 0.9, 5)

	opps := []Opportunity{
		{EstProfitGwei: 10, ActualTopFee: 5, BaseFeeGwei: 0.01},
		{EstProfitGwei: 10, ActualTopFee: 20, BaseFeeGwei: 0.01}, // won't win with fixed-5
		{EstProfitGwei: 10, ActualTopFee: 3, BaseFeeGwei: 0.01},
	}

	tests := []struct {
		name     string
		strategy Strategy
		opps     []Opportunity
		wantWins int
		wantPnL  float64
	}{
		{
			name:     "fixed-5 wins 2 of 3",
			strategy: NewFixedTip(5.0),
			opps:     opps,
			wantWins: 2,
			// opp0: 10 - 5 - 0.01 = 4.99; opp2: 10 - 5 - 0.01 = 4.99
			wantPnL: 9.98,
		},
		{
			name:     "fixed-100 wins all",
			strategy: NewFixedTip(100.0),
			opps:     opps,
			wantWins: 3,
		},
		{
			name:     "fixed-0 wins none",
			strategy: NewFixedTip(0),
			opps:     opps,
			wantWins: 0,
			wantPnL:  0,
		},
		{
			name:     "empty opportunities",
			strategy: NewFixedTip(5),
			opps:     nil,
			wantWins: 0,
			wantPnL:  0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := Replay(ctx, ReplayConfig{
				Strategy:      tc.strategy,
				Opportunities: tc.opps,
				History:       hist,
				MaxWorkers:    2,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Wins != tc.wantWins {
				t.Errorf("Wins = %d, want %d", result.Wins, tc.wantWins)
			}
			if tc.wantPnL != 0 && math.Abs(result.TotalPnL-tc.wantPnL) > 0.01 {
				t.Errorf("TotalPnL = %g, want %g", result.TotalPnL, tc.wantPnL)
			}
			if result.Disclaimer == "" {
				t.Error("Disclaimer must be non-empty")
			}
		})
	}
}

// ---- Replay cancellation -----------------------------------------------------

func TestReplayCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	opps := make([]Opportunity, 100)
	_, err := Replay(ctx, ReplayConfig{
		Strategy:      NewFixedTip(1),
		Opportunities: opps,
		History:       makeHist(100, 10, 0.9, 5),
		MaxWorkers:    1,
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}
}

// ---- Replay concurrency limit ------------------------------------------------

func TestReplayConcurrencyLimit(t *testing.T) {
	t.Parallel()
	opps := make([]Opportunity, 50)
	for i := range opps {
		opps[i] = Opportunity{EstProfitGwei: 5, ActualTopFee: 1, BaseFeeGwei: 0.01}
	}
	result, err := Replay(context.Background(), ReplayConfig{
		Strategy:      NewFixedTip(2),
		Opportunities: opps,
		History:       makeHist(50, 10, 0.9, 5),
		MaxWorkers:    3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Wins != len(opps) {
		t.Errorf("want all wins with fixed-2 vs actualTop-1, got %d/%d", result.Wins, len(opps))
	}
}

// ---- Property: win rate in [0, 1] -------------------------------------------

func TestReplayWinRateProperty(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		fee := rapid.Float64Range(0, 100).Draw(rt, "fee")
		n := rapid.IntRange(0, 20).Draw(rt, "n")
		opps := make([]Opportunity, n)
		for i := range opps {
			opps[i].ActualTopFee = rapid.Float64Range(0, 50).Draw(rt, "actualTop")
			opps[i].EstProfitGwei = rapid.Float64Range(0, 100).Draw(rt, "profit")
			opps[i].BaseFeeGwei = rapid.Float64Range(0, 5).Draw(rt, "base")
		}
		result, err := Replay(context.Background(), ReplayConfig{
			Strategy:      NewFixedTip(fee),
			Opportunities: opps,
			History:       makeHist(5, 10, 0.9, 5),
			MaxWorkers:    2,
		})
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if result.WinRate < 0 || result.WinRate > 1+1e-9 {
			rt.Fatalf("WinRate = %g outside [0,1]", result.WinRate)
		}
		if result.Total != n {
			rt.Fatalf("Total = %d, want %d", result.Total, n)
		}
	})
}
