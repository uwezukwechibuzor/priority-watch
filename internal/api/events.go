package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerEventHandlers(api huma.API, st *Store) {
	// GET /api/v1/events
	type eventsOutput struct {
		Body struct {
			Data []EventRow `json:"data"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-events",
		Method:      http.MethodGet,
		Path:        "/api/v1/events",
		Tags:        []string{"Events"},
		Summary:     "Chart marker events (Timeboost launch, ArbOS upgrades, PGA activation)",
		Description: "Returns all events from config/events.yaml loaded via dbt seed into marts.dim_events.",
	}, func(ctx context.Context, _ *struct{}) (*eventsOutput, error) {
		rows, err := st.GetEvents(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &eventsOutput{}
		out.Body.Data = rows
		return out, nil
	})
}
