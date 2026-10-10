package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerFeeHandlers(api huma.API, st *Store) {
	// GET /api/v1/fees/current
	type currentOutput struct{ Body BaseFeeRow }
	huma.Register(api, huma.Operation{
		OperationID: "get-current-fee",
		Method:      http.MethodGet,
		Path:        "/api/v1/fees/current",
		Tags:        []string{"Fees"},
		Summary:     "Current base fee (most recent block)",
	}, func(ctx context.Context, _ *struct{}) (*currentOutput, error) {
		row, err := st.GetCurrentFee(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		return &currentOutput{Body: row}, nil
	})

	// GET /api/v1/fees/history
	type histInput struct {
		Range string `query:"range" default:"24h" enum:"1h,24h,7d,30d,all"`
		Limit int    `query:"limit" default:"200" minimum:"1" maximum:"1000"`
	}
	type histOutput struct {
		Body struct {
			Data []BaseFeeRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-fee-history",
		Method:      http.MethodGet,
		Path:        "/api/v1/fees/history",
		Tags:        []string{"Fees"},
		Summary:     "Per-block base fee history",
	}, func(ctx context.Context, input *histInput) (*histOutput, error) {
		rows, err := st.GetFeeHistory(ctx, input.Range, input.Limit)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &histOutput{}
		out.Body.Data = rows
		return out, nil
	})

	// GET /api/v1/fees/changes — hourly aggregated fee series
	type changesInput struct {
		Range string `query:"range" default:"24h" enum:"1h,24h,7d,30d,all"`
	}
	type changesOutput struct {
		Body struct {
			Data []BaseFeeHourlyRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-fee-changes",
		Method:      http.MethodGet,
		Path:        "/api/v1/fees/changes",
		Tags:        []string{"Fees"},
		Summary:     "Hourly base fee aggregates (min/max/avg/median, change %)",
		Description: "Analogue of timeboost.app's Reserve group; shows base fee evolution over time.",
	}, func(ctx context.Context, input *changesInput) (*changesOutput, error) {
		rows, err := st.GetFeeHourly(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &changesOutput{}
		out.Body.Data = rows
		return out, nil
	})
}
