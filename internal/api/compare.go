package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerCompareHandlers(api huma.API, st *Store) {
	// GET /api/v1/compare/eras?window_days=14
	type erasInput struct {
		WindowDays int `query:"window_days" default:"14" minimum:"1" maximum:"365"`
	}
	type erasOutput struct {
		Body struct {
			Data []EraComparisonRow `json:"data"`
		}
	}

	huma.Register(api, huma.Operation{
		OperationID: "compare-eras",
		Method:      http.MethodGet,
		Path:        "/api/v1/compare/eras",
		Tags:        []string{"Compare"},
		Summary:     "Side-by-side Timeboost vs. PGA era metrics",
		Description: "Returns one row per era for the requested window. Includes Entropy benchmark capture rates.",
	}, func(ctx context.Context, input *erasInput) (*erasOutput, error) {
		rows, err := st.GetEraComparison(ctx, input.WindowDays)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &erasOutput{}
		out.Body.Data = rows
		return out, nil
	})
}
