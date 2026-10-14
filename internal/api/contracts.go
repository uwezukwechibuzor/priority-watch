package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func registerContractHandlers(api huma.API, st *Store) {
	type contractsOutput struct {
		Body struct {
			Data []ContractRow `json:"data"`
		}
	}

	huma.Register(api, huma.Operation{
		OperationID: "list-contracts",
		Method:      http.MethodGet,
		Path:        "/api/v1/contracts",
		Tags:        []string{"Reference"},
		Summary:     "Known contract addresses and names",
		Description: "Returns all known contract addresses with human-readable names. " +
			"Use as a client-side lookup map: address (lowercase) → name. " +
			"This list is stable between deploys; cache aggressively (ETag / 1 h TTL).",
	}, func(ctx context.Context, _ *struct{}) (*contractsOutput, error) {
		rows, err := st.GetContracts(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &contractsOutput{}
		out.Body.Data = rows
		return out, nil
	})
}
