package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerTreasuryHandlers(api huma.API, st *Store) {
	type rangeInput struct {
		Range string `query:"range" default:"30d" enum:"1h,24h,7d,30d,all"`
	}

	// GET /api/v1/treasury/daily
	type dailyOutput struct {
		Body struct {
			Data []TreasuryDailyRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-treasury-daily",
		Method:      http.MethodGet,
		Path:        "/api/v1/treasury/daily",
		Tags:        []string{"Treasury"},
		Summary:     "Daily tip revenue split between Treasury (97%) and Developer Guild (3%)",
	}, func(ctx context.Context, input *rangeInput) (*dailyOutput, error) {
		rows, err := st.GetTreasuryDaily(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &dailyOutput{}
		out.Body.Data = rows
		return out, nil
	})

	// GET /api/v1/treasury/summary
	type summaryOutput struct{ Body TreasurySummary }
	huma.Register(api, huma.Operation{
		OperationID: "get-treasury-summary",
		Method:      http.MethodGet,
		Path:        "/api/v1/treasury/summary",
		Tags:        []string{"Treasury"},
		Summary:     "Cumulative treasury totals and projected annual run rate",
	}, func(ctx context.Context, _ *struct{}) (*summaryOutput, error) {
		s, err := st.GetTreasurySummary(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		return &summaryOutput{Body: s}, nil
	})

	// GET /api/v1/treasury/reconciliation
	// Returns days where status != 'ok' — a stub until balance snapshots are available.
	type reconOutput struct {
		Body struct {
			Data []TreasuryDailyRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-treasury-reconciliation",
		Method:      http.MethodGet,
		Path:        "/api/v1/treasury/reconciliation",
		Tags:        []string{"Treasury"},
		Summary:     "Days where the on-chain reconciliation status is warn or fail",
		Description: "Pending balance snapshot integration (task 8 part C). Returns all rows until then.",
	}, func(ctx context.Context, input *rangeInput) (*reconOutput, error) {
		rows, err := st.GetTreasuryDaily(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		// Filter to non-ok rows for the reconciliation view.
		var filtered []TreasuryDailyRow
		for _, r := range rows {
			if r.Status != "ok" {
				filtered = append(filtered, r)
			}
		}
		out := &reconOutput{}
		out.Body.Data = filtered
		return out, nil
	})
}
