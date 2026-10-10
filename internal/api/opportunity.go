package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerOpportunityHandlers(api huma.API, st *Store) {
	type rangeInput struct {
		Range string `query:"range" default:"24h" enum:"1h,24h,7d,30d,all"`
		Limit int    `query:"limit" default:"500" minimum:"1" maximum:"5000"`
	}

	// GET /api/v1/opportunity/spreads
	type spreadsOutput struct {
		Body struct {
			Data []SpreadRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-spreads",
		Method:      http.MethodGet,
		Path:        "/api/v1/opportunity/spreads",
		Tags:        []string{"Opportunity"},
		Summary:     "DEX–CEX spread per block and pool",
	}, func(ctx context.Context, input *rangeInput) (*spreadsOutput, error) {
		rows, err := st.GetSpreads(ctx, input.Range, input.Limit)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &spreadsOutput{}
		out.Body.Data = rows
		return out, nil
	})

	// GET /api/v1/opportunity/heatmap
	type heatmapOutput struct {
		Body struct {
			Data []HeatmapCell `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-spread-heatmap",
		Method:      http.MethodGet,
		Path:        "/api/v1/opportunity/heatmap",
		Tags:        []string{"Opportunity"},
		Summary:     "Spread heatmap — hour of day × day of week",
	}, func(ctx context.Context, _ *struct{}) (*heatmapOutput, error) {
		cells, err := st.GetSpreadHeatmap(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &heatmapOutput{}
		out.Body.Data = cells
		return out, nil
	})

	// GET /api/v1/opportunity/histogram
	type histogramOutput struct {
		Body struct {
			Data []HistogramBin `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-spread-histogram",
		Method:      http.MethodGet,
		Path:        "/api/v1/opportunity/histogram",
		Tags:        []string{"Opportunity"},
		Summary:     "Spread histogram (bps bins with block count and cumulative %)",
	}, func(ctx context.Context, _ *struct{}) (*histogramOutput, error) {
		bins, err := st.GetSpreadHistogram(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &histogramOutput{}
		out.Body.Data = bins
		return out, nil
	})

	// GET /api/v1/opportunity/markouts
	type markoutsOutput struct {
		Body struct {
			Data []MarkoutRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-markouts",
		Method:      http.MethodGet,
		Path:        "/api/v1/opportunity/markouts",
		Tags:        []string{"Opportunity"},
		Summary:     "Swap markouts at 1 s and 300 s horizons",
	}, func(ctx context.Context, input *rangeInput) (*markoutsOutput, error) {
		rows, err := st.GetMarkouts(ctx, input.Range, input.Limit)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &markoutsOutput{}
		out.Body.Data = rows
		return out, nil
	})

	// GET /api/v1/opportunity/position-sensitivity
	type posInput struct {
		Range string `query:"range" default:"30d" enum:"1h,24h,7d,30d,all"`
	}
	type posOutput struct {
		Body struct {
			Data []PositionRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-position-sensitivity",
		Method:      http.MethodGet,
		Path:        "/api/v1/opportunity/position-sensitivity",
		Tags:        []string{"Opportunity"},
		Summary:     "Entity position vs. fee rank (over/under-payment delta)",
	}, func(ctx context.Context, input *posInput) (*posOutput, error) {
		rows, err := st.GetPositionSensitivity(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &posOutput{}
		out.Body.Data = rows
		return out, nil
	})
}

func registerExecutionHandlers(api huma.API, st *Store) {
	type rangeInput struct {
		Range string `query:"range" default:"30d" enum:"1h,24h,7d,30d,all"`
	}

	// GET /api/v1/execution/quality
	type qualityOutput struct {
		Body struct {
			Data []ExecQualityRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-execution-quality",
		Method:      http.MethodGet,
		Path:        "/api/v1/execution/quality",
		Tags:        []string{"Execution"},
		Summary:     "Daily execution quality per entity (revert rate + markouts)",
	}, func(ctx context.Context, input *rangeInput) (*qualityOutput, error) {
		rows, err := st.GetExecutionQuality(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &qualityOutput{}
		out.Body.Data = rows
		return out, nil
	})

	// GET /api/v1/execution/by-entity/{id}
	type byEntityInput struct {
		ID    string `path:"id"    doc:"Entity name"`
		Range string `query:"range" default:"30d" enum:"1h,24h,7d,30d,all"`
	}
	type byEntityOutput struct {
		Body struct {
			Data []ExecQualityRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-execution-by-entity",
		Method:      http.MethodGet,
		Path:        "/api/v1/execution/by-entity/{id}",
		Tags:        []string{"Execution"},
		Summary:     "Execution quality for a specific entity",
	}, func(ctx context.Context, input *byEntityInput) (*byEntityOutput, error) {
		rows, err := st.GetExecutionQuality(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		var filtered []ExecQualityRow
		for _, r := range rows {
			if r.EntityName == input.ID {
				filtered = append(filtered, r)
			}
		}
		out := &byEntityOutput{}
		out.Body.Data = filtered
		return out, nil
	})
}
