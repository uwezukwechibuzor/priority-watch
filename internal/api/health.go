package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerHealthHandlers(api huma.API, st *Store) {
	type rangeInput struct {
		Range string `query:"range" default:"30d" enum:"1h,24h,7d,30d,all"`
	}

	// GET /api/v1/health/revenue
	type revenueOutput struct {
		Body struct {
			Data []HealthDailyRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-health-revenue",
		Method:      http.MethodGet,
		Path:        "/api/v1/health/revenue",
		Tags:        []string{"Health"},
		Summary:     "Daily tip revenue and throughput",
	}, func(ctx context.Context, input *rangeInput) (*revenueOutput, error) {
		rows, err := st.GetHealthDaily(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &revenueOutput{}
		out.Body.Data = rows
		return out, nil
	})

	// GET /api/v1/health/concentration
	type concOutput struct {
		Body struct {
			Data []HealthDailyRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-health-concentration",
		Method:      http.MethodGet,
		Path:        "/api/v1/health/concentration",
		Tags:        []string{"Health"},
		Summary:     "Daily HHI and payer concentration",
	}, func(ctx context.Context, input *rangeInput) (*concOutput, error) {
		rows, err := st.GetHealthDaily(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &concOutput{}
		out.Body.Data = rows
		return out, nil
	})

	// GET /api/v1/health/competition
	type compOutput struct {
		Body struct {
			Data []HealthDailyRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-health-competition",
		Method:      http.MethodGet,
		Path:        "/api/v1/health/competition",
		Tags:        []string{"Health"},
		Summary:     "Daily competition metrics (contested share, fee band)",
	}, func(ctx context.Context, input *rangeInput) (*compOutput, error) {
		rows, err := st.GetHealthDaily(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &compOutput{}
		out.Body.Data = rows
		return out, nil
	})

	// GET /api/v1/health/revert-rates
	type revertOutput struct {
		Body struct {
			Data []HealthDailyRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-health-revert-rates",
		Method:      http.MethodGet,
		Path:        "/api/v1/health/revert-rates",
		Tags:        []string{"Health"},
		Summary:     "Daily revert rates (tipped vs. untipped txs)",
	}, func(ctx context.Context, input *rangeInput) (*revertOutput, error) {
		rows, err := st.GetHealthDaily(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &revertOutput{}
		out.Body.Data = rows
		return out, nil
	})

	// GET /api/v1/health/user-costs
	type userCostOutput struct {
		Body struct {
			Data []HealthDailyRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-health-user-costs",
		Method:      http.MethodGet,
		Path:        "/api/v1/health/user-costs",
		Tags:        []string{"Health"},
		Summary:     "Daily user cost metrics (median tx cost, fee percentiles)",
	}, func(ctx context.Context, input *rangeInput) (*userCostOutput, error) {
		rows, err := st.GetHealthDaily(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &userCostOutput{}
		out.Body.Data = rows
		return out, nil
	})

	// GET /api/v1/health/leaderboard
	type leaderInput struct {
		Range string `query:"range" default:"30d" enum:"1h,24h,7d,30d,all"`
		Limit int    `query:"limit" default:"20" minimum:"1" maximum:"100"`
	}
	type leaderOutput struct {
		Body struct {
			Data       []PayerShareRow `json:"data"`
			Pagination Pagination      `json:"pagination"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-health-leaderboard",
		Method:      http.MethodGet,
		Path:        "/api/v1/health/leaderboard",
		Tags:        []string{"Health"},
		Summary:     "Payer leaderboard sorted by tip revenue",
	}, func(ctx context.Context, input *leaderInput) (*leaderOutput, error) {
		rows, err := st.GetPayerShare(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &leaderOutput{}
		out.Body.Data = rows
		return out, nil
	})

	// GET /api/v1/health/base-fee
	type baseFeeInput struct {
		Range string `query:"range" default:"24h" enum:"1h,24h,7d,30d,all"`
	}
	type baseFeeOutput struct {
		Body struct {
			Data []BaseFeeHourlyRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-health-base-fee",
		Method:      http.MethodGet,
		Path:        "/api/v1/health/base-fee",
		Tags:        []string{"Health"},
		Summary:     "Hourly base fee history",
	}, func(ctx context.Context, input *baseFeeInput) (*baseFeeOutput, error) {
		rows, err := st.GetFeeHourly(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &baseFeeOutput{}
		out.Body.Data = rows
		return out, nil
	})

	// GET /api/v1/health/ingester-status
	type ingesterOutput struct {
		Body struct {
			Data []IngesterStatusRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-ingester-status",
		Method:      http.MethodGet,
		Path:        "/api/v1/health/ingester-status",
		Tags:        []string{"Health"},
		Summary:     "Per-source ingester status and lag",
		Description: "Returns one row per data source (chain RPC, Binance klines, timeboost.app). Mirrors timeboost.app's ingester-status endpoint.",
	}, func(ctx context.Context, _ *struct{}) (*ingesterOutput, error) {
		rows, err := st.GetIngesterStatus(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &ingesterOutput{}
		out.Body.Data = rows
		return out, nil
	})

	// GET /api/v1/health/payers
	type allPayersInput struct {
		Limit int `query:"limit" default:"200" minimum:"1" maximum:"1000"`
	}
	type allPayersOutput struct {
		Body struct {
			Data []AllPayerRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-health-payers",
		Method:      http.MethodGet,
		Path:        "/api/v1/health/payers",
		Tags:        []string{"Health"},
		Summary:     "All payers with contract and tip history",
		Description: "Returns every tipping address grouped with the contract most interacted with, ordered by total tip paid descending.",
	}, func(ctx context.Context, input *allPayersInput) (*allPayersOutput, error) {
		rows, err := st.GetAllPayers(ctx, input.Limit)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &allPayersOutput{}
		out.Body.Data = rows
		return out, nil
	})
}
