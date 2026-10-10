package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/uwezukwechibuzor/priority-watch/internal/models"
)

// ── Pricing recommend ─────────────────────────────────────────────────────────

type PricingRecommendParams struct {
	Block      uint64  `query:"block"      doc:"Reference block number; 0 = latest"`
	N          int     `query:"n"          doc:"Target rank position (1 = top of block)"`
	Confidence float64 `query:"confidence" doc:"Desired hit-rate fraction, e.g. 0.80"`
	Model      string  `query:"model"      doc:"Model name; empty = all models"`
	Window     int     `query:"window"     doc:"History window in blocks (default 200)"`
}

type PricingRecommendOutput struct {
	Body []models.Recommendation
}

func registerPricingHandlers(api huma.API, store *Store) {
	huma.Register(api, huma.Operation{
		OperationID: "getPricingRecommend",
		Method:      http.MethodGet,
		Path:        "/api/v1/pricing/recommend",
		Summary:     "Get fee recommendations from pricing models",
		Tags:        []string{"Pricing"},
	}, func(ctx context.Context, input *struct {
		PricingRecommendParams
	},
	) (*PricingRecommendOutput, error) {
		p := input.PricingRecommendParams
		if p.N <= 0 {
			p.N = 1
		}
		if p.Confidence <= 0 {
			p.Confidence = 0.80
		}
		if p.Window <= 0 {
			p.Window = 200
		}

		hist, err := store.GetPricingHistory(ctx, p.Block, p.Window)
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("pricing history unavailable: " + err.Error())
		}
		if len(hist) == 0 {
			return nil, huma.Error404NotFound("no block fee history available")
		}

		req := models.Request{
			N:            p.N,
			Confidence:   p.Confidence,
			WindowBlocks: p.Window,
		}

		var recs []models.Recommendation
		for _, m := range models.AllModels() {
			if p.Model != "" && m.Name() != p.Model {
				continue
			}
			rec, err := m.Recommend(ctx, req, hist)
			if err != nil {
				continue // skip models that can't produce a recommendation
			}
			recs = append(recs, rec)
		}
		if len(recs) == 0 {
			return nil, huma.Error404NotFound(fmt.Sprintf("no model named %q", p.Model))
		}
		return &PricingRecommendOutput{Body: recs}, nil
	})
}

// ── Model eval ────────────────────────────────────────────────────────────────

type ModelEvalParams struct {
	N          int     `query:"n"          doc:"Filter by rank position; 0 = all"`
	Confidence float64 `query:"confidence" doc:"Filter by confidence; 0 = all"`
}

type ModelEvalOutput struct {
	Body []ModelEvalRow
}

func registerModelEvalHandlers(api huma.API, store *Store) {
	huma.Register(api, huma.Operation{
		OperationID: "getModelEval",
		Method:      http.MethodGet,
		Path:        "/api/v1/pricing/models/eval",
		Summary:     "Get the latest model evaluation results",
		Tags:        []string{"Pricing"},
	}, func(ctx context.Context, input *struct {
		ModelEvalParams
	},
	) (*ModelEvalOutput, error) {
		rows, err := store.GetModelEval(ctx, input.N, input.Confidence)
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("model eval unavailable: " + err.Error())
		}
		return &ModelEvalOutput{Body: rows}, nil
	})
}

// ── Backtest ──────────────────────────────────────────────────────────────────

// BacktestRunner manages async backtest jobs with bounded concurrency.
type BacktestRunner struct {
	store *Store
	sem   chan struct{}
	log   *slog.Logger
}

func newBacktestRunner(store *Store, maxJobs int, log *slog.Logger) *BacktestRunner {
	return &BacktestRunner{
		store: store,
		sem:   make(chan struct{}, maxJobs),
		log:   log,
	}
}

type CreateBacktestInput struct {
	Body struct {
		Strategy  string          `json:"strategy"   doc:"Strategy name: fixed-tip, percentile-of-recent, spread-proportional"`
		FromBlock uint64          `json:"from_block" doc:"Start of replay range"`
		ToBlock   uint64          `json:"to_block"   doc:"End of replay range"`
		Config    json.RawMessage `json:"config"     doc:"Strategy-specific parameters (JSON object)"`
	}
}

type CreateBacktestOutput struct {
	Body struct {
		RunID  string `json:"run_id"`
		Status string `json:"status"`
	}
}

type GetBacktestOutput struct {
	Body BacktestRunRow
}

type GetBacktestResultsOutput struct {
	Body struct {
		RunID  string          `json:"run_id"`
		Status string          `json:"status"`
		Result json.RawMessage `json:"result"`
	}
}

func registerBacktestHandlers(api huma.API, runner *BacktestRunner) {
	huma.Register(api, huma.Operation{
		OperationID: "createBacktest",
		Method:      http.MethodPost,
		Path:        "/api/v1/backtest",
		Summary:     "Submit an async backtest job",
		Tags:        []string{"Backtest"},
	}, func(ctx context.Context, input *CreateBacktestInput) (*CreateBacktestOutput, error) {
		runID := uuid.New().String()
		now := time.Now().UTC()

		cfgBytes := input.Body.Config
		if len(cfgBytes) == 0 {
			cfgBytes = []byte("{}")
		}

		run := BacktestRunRow{
			RunID:      runID,
			Status:     "pending",
			Strategy:   input.Body.Strategy,
			FromBlock:  input.Body.FromBlock,
			ToBlock:    input.Body.ToBlock,
			ConfigJSON: string(cfgBytes),
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := runner.store.SaveBacktestRun(ctx, run); err != nil {
			return nil, huma.Error503ServiceUnavailable("could not create run: " + err.Error())
		}

		go runner.executeJob(run) //nolint:gosec

		out := &CreateBacktestOutput{}
		out.Body.RunID = runID
		out.Body.Status = "pending"
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getBacktest",
		Method:      http.MethodGet,
		Path:        "/api/v1/backtest/{run_id}",
		Summary:     "Get backtest job status",
		Tags:        []string{"Backtest"},
	}, func(ctx context.Context, input *struct {
		RunID string `path:"run_id"`
	},
	) (*GetBacktestOutput, error) {
		run, err := runner.store.GetBacktestRun(ctx, input.RunID)
		if err != nil {
			return nil, huma.Error404NotFound("run not found: " + input.RunID)
		}
		return &GetBacktestOutput{Body: run}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getBacktestResults",
		Method:      http.MethodGet,
		Path:        "/api/v1/backtest/{run_id}/results",
		Summary:     "Get backtest results (available when status=done)",
		Tags:        []string{"Backtest"},
	}, func(ctx context.Context, input *struct {
		RunID string `path:"run_id"`
	},
	) (*GetBacktestResultsOutput, error) {
		run, err := runner.store.GetBacktestRun(ctx, input.RunID)
		if err != nil {
			return nil, huma.Error404NotFound("run not found: " + input.RunID)
		}
		if run.Status != "done" {
			return nil, huma.Error409Conflict("run is not done yet, status=" + run.Status)
		}
		resultJSON := json.RawMessage(run.ResultJSON)
		if !json.Valid(resultJSON) {
			resultJSON = []byte("null")
		}
		out := &GetBacktestResultsOutput{}
		out.Body.RunID = run.RunID
		out.Body.Status = run.Status
		out.Body.Result = resultJSON
		return out, nil
	})
}

// executeJob runs the replay in a worker-pool goroutine.
// context.Background is intentional: the HTTP request context may have been
// cancelled before the job finishes, but we must complete the job.
func (r *BacktestRunner) executeJob(run BacktestRunRow) {
	ctx := context.Background() //nolint:gosec

	// Acquire semaphore.
	r.sem <- struct{}{}
	defer func() { <-r.sem }()

	if err := r.store.UpdateBacktestRun(ctx, run.RunID, "running", "", ""); err != nil {
		r.log.ErrorContext(ctx, "backtest update running", "run_id", run.RunID, "err", err)
		return
	}

	result, execErr := r.runReplay(ctx, run)
	if execErr != nil {
		_ = r.store.UpdateBacktestRun(ctx, run.RunID, "error", "", execErr.Error())
		return
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		_ = r.store.UpdateBacktestRun(ctx, run.RunID, "error", "", err.Error())
		return
	}
	if err := r.store.UpdateBacktestRun(ctx, run.RunID, "done", string(resultBytes), ""); err != nil {
		r.log.ErrorContext(ctx, "backtest update done", "run_id", run.RunID, "err", err)
	}
}

// runReplay fetches history and builds opportunities for the replay range.
func (r *BacktestRunner) runReplay(ctx context.Context, run BacktestRunRow) (models.ReplayResult, error) {
	strategy, err := parseStrategy(run.Strategy, run.ConfigJSON)
	if err != nil {
		return models.ReplayResult{}, err
	}

	windowBlocks := 200
	hist, err := r.store.GetPricingHistory(ctx, run.FromBlock, windowBlocks)
	if err != nil {
		return models.ReplayResult{}, fmt.Errorf("fetch history: %w", err)
	}

	// Build an opportunity for each block in [FromBlock, ToBlock].
	replayHist, err := r.store.GetPricingHistory(ctx, run.ToBlock+1,
		int(run.ToBlock-run.FromBlock+1)) //nolint:gosec
	if err != nil {
		return models.ReplayResult{}, fmt.Errorf("fetch replay range: %w", err)
	}

	opps := make([]models.Opportunity, 0, len(replayHist))
	for _, b := range replayHist {
		if b.BlockNumber < run.FromBlock || b.BlockNumber > run.ToBlock {
			continue
		}
		opps = append(opps, models.Opportunity{
			BlockNumber:   b.BlockNumber,
			ActualTopFee:  b.TopFeeGwei,
			EstProfitGwei: b.TopFeeGwei * 1.5, // rough estimate: top fee × 1.5
			BaseFeeGwei:   0,                  // not tracked at this level
		})
	}

	return models.Replay(ctx, models.ReplayConfig{
		Strategy:      strategy,
		Opportunities: opps,
		History:       hist,
		MaxWorkers:    4,
	})
}

// parseStrategy constructs the named strategy from its config JSON.
func parseStrategy(name, cfgJSON string) (models.Strategy, error) {
	var cfg struct {
		TipGwei    float64 `json:"tip_gwei"`
		Percentile float64 `json:"percentile"`
		Alpha      float64 `json:"alpha"`
	}
	if cfgJSON != "" && cfgJSON != "{}" {
		if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
			return nil, fmt.Errorf("bad config JSON: %w", err)
		}
	}

	switch name {
	case "fixed-tip":
		return models.NewFixedTip(cfg.TipGwei), nil
	case "percentile-of-recent":
		p := cfg.Percentile
		if p <= 0 {
			p = 0.80
		}
		return models.NewPercentileOfRecent(p), nil
	case "spread-proportional":
		a := cfg.Alpha
		if a <= 0 {
			a = 0.50
		}
		return models.NewSpreadProportional(a), nil
	default:
		return nil, fmt.Errorf("unknown strategy %q; valid: fixed-tip, percentile-of-recent, spread-proportional", name)
	}
}
