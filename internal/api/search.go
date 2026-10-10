package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

func registerSearchHandlers(api huma.API, st *Store) {
	// GET /api/v1/search?q=
	type searchInput struct {
		Q string `query:"q" minLength:"2" maxLength:"100" doc:"Search query (block number, tx hash, address, entity name)"`
	}
	type searchOutput struct {
		Body struct {
			Data []SearchResult `json:"data"`
		}
	}

	huma.Register(api, huma.Operation{
		OperationID: "search",
		Method:      http.MethodGet,
		Path:        "/api/v1/search",
		Tags:        []string{"Search"},
		Summary:     "Search blocks, transactions, addresses and entities",
	}, func(ctx context.Context, input *searchInput) (*searchOutput, error) {
		q := strings.TrimSpace(input.Q)
		if len(q) < 2 {
			return nil, huma.Error400BadRequest("q must be at least 2 characters", nil)
		}
		results, err := st.Search(ctx, q)
		if err != nil {
			return nil, huma.Error500InternalServerError("search failed", err)
		}
		out := &searchOutput{}
		out.Body.Data = results
		return out, nil
	})
}
