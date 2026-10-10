package models

import (
	"context"
	"fmt"
	"sync"
)

// ReplayConfig controls one backtest run.
type ReplayConfig struct {
	Strategy      Strategy
	Opportunities []Opportunity
	// History is the full block-fee history available to strategies.
	// Each opportunity's "recent" window is History[:opp_index].
	History    []BlockFeeRecord
	MaxWorkers int
}

// opportunityResult is the per-event output from a worker.
type opportunityResult struct {
	idx  int
	won  bool
	tipG float64
	pnlG float64
	err  error
}

// Replay replays a sequence of arb opportunities using the configured strategy.
// Workers run in a bounded pool; the run is cancelled if ctx is cancelled.
func Replay(ctx context.Context, cfg ReplayConfig) (ReplayResult, error) {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 4
	}
	n := len(cfg.Opportunities)
	if n == 0 {
		return ReplayResult{
			Strategy:   cfg.Strategy.Name(),
			Disclaimer: replayDisclaimer,
		}, nil
	}

	results := make([]opportunityResult, n)

	// Bounded worker pool via buffered semaphore channel.
	sem := make(chan struct{}, cfg.MaxWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i, opp := range cfg.Opportunities {
		select {
		case <-ctx.Done():
			return ReplayResult{}, ctx.Err()
		default:
		}

		wg.Add(1)
		sem <- struct{}{} // acquire
		go func(idx int, o Opportunity) {
			defer wg.Done()
			defer func() { <-sem }() // release

			// recent = history up to (but not including) this block
			var recent []BlockFeeRecord
			if idx < len(cfg.History) {
				recent = cfg.History[:idx]
			} else {
				recent = cfg.History
			}

			tip, err := cfg.Strategy.Tip(ctx, o, recent)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("strategy error at idx %d: %w", idx, err)
				}
				mu.Unlock()
				results[idx] = opportunityResult{idx: idx, err: err}
				return
			}

			won := tip >= o.ActualTopFee
			pnl := 0.0
			if won {
				// P&L = gross profit – tip – base fee (all in gwei)
				pnl = o.EstProfitGwei - tip - o.BaseFeeGwei
			}
			results[idx] = opportunityResult{idx: idx, won: won, tipG: tip, pnlG: pnl}
		}(i, opp)
	}
	wg.Wait()

	if firstErr != nil {
		return ReplayResult{}, firstErr
	}

	// Aggregate
	wins := 0
	totalTip := 0.0
	cumPnL := make([]float64, n)
	running := 0.0
	for i, r := range results {
		if r.won {
			wins++
		}
		totalTip += r.tipG
		running += r.pnlG
		cumPnL[i] = running
	}

	avgCost := 0.0
	if n > 0 {
		avgCost = totalTip / float64(n)
	}
	winRate := 0.0
	if n > 0 {
		winRate = float64(wins) / float64(n)
	}

	return ReplayResult{
		Strategy:   cfg.Strategy.Name(),
		TotalPnL:   running,
		WinRate:    winRate,
		Wins:       wins,
		Total:      n,
		AvgCost:    avgCost,
		CumPnL:     cumPnL,
		Disclaimer: replayDisclaimer,
	}, nil
}
