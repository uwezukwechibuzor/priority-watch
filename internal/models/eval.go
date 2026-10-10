package models

import (
	"context"
	"fmt"
	"time"
)

// EvalConfig controls a model evaluation run.
type EvalConfig struct {
	TrainBlocks int     // blocks used as history for each test block
	TestBlocks  int     // number of held-out test blocks to evaluate
	N           int     // target rank
	Confidence  float64 // desired hit rate
}

// EvalRecord is one row written to marts.model_eval.
type EvalRecord struct {
	EvalDate     time.Time
	ModelName    string
	N            int
	Confidence   float64
	WindowBlocks int
	HitRate      float64
	AvgCostGwei  float64
	EvalBlocks   int
}

// Eval evaluates all models on the held-out test window of hist.
// hist must be ordered oldest-first (ascending block number).
// The test window is hist[len(hist)-cfg.TestBlocks:].
func Eval(ctx context.Context, cfg EvalConfig, hist []BlockFeeRecord) ([]EvalRecord, error) {
	if len(hist) < cfg.TrainBlocks+cfg.TestBlocks {
		return nil, fmt.Errorf("eval: need at least %d blocks, got %d",
			cfg.TrainBlocks+cfg.TestBlocks, len(hist))
	}

	testStart := len(hist) - cfg.TestBlocks
	testBlocks := hist[testStart:]
	evalDate := time.Now().UTC().Truncate(24 * time.Hour)

	var out []EvalRecord
	for _, m := range AllModels() {
		hits := 0
		totalCost := 0.0

		for i, tb := range testBlocks {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			// Training window: the TrainBlocks blocks immediately before this test block.
			trainEnd := testStart + i
			trainStart := trainEnd - cfg.TrainBlocks
			if trainStart < 0 {
				trainStart = 0
			}
			train := hist[trainStart:trainEnd]
			if len(train) == 0 {
				continue
			}

			rec, err := m.Recommend(ctx, Request{
				N:            cfg.N,
				Confidence:   cfg.Confidence,
				WindowBlocks: cfg.TrainBlocks,
			}, train)
			if err != nil {
				continue
			}

			// Hit: recommended fee ≥ actual rank-N fee in the test block.
			actual := feeAtRank(tb, cfg.N)
			if rec.FeeGwei >= actual {
				hits++
			}
			totalCost += rec.FeeGwei
		}

		n := len(testBlocks)
		if n == 0 {
			continue
		}
		out = append(out, EvalRecord{
			EvalDate:     evalDate,
			ModelName:    m.Name(),
			N:            cfg.N,
			Confidence:   cfg.Confidence,
			WindowBlocks: cfg.TrainBlocks,
			HitRate:      float64(hits) / float64(n),
			AvgCostGwei:  totalCost / float64(n),
			EvalBlocks:   n,
		})
	}
	return out, nil
}
