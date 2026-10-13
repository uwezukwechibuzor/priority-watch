package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// BlocksParams collects query parameters shared across block-list calls.
// Optional filters use zero-value sentinels: empty string or 0.
type BlocksParams struct {
	Range         string  `query:"range"          default:"24h" enum:"1h,24h,7d,30d,all" doc:"Time range"`
	Day           string  `query:"day"            doc:"YYYY-MM-DD; overrides range (empty = not set)"`
	Search        string  `query:"search"         doc:"Search block number, address, or entity name (empty = no filter)"`
	Entity        string  `query:"entity"         doc:"Filter by top-payer entity name (empty = no filter)"`
	ContestedOnly bool    `query:"contested_only" doc:"Only return contested blocks"`
	MinFeeGwei    float64 `query:"min_fee_gwei"   doc:"Minimum top fee in gwei (0 = no filter)"`
	FromBlock     uint64  `query:"from_block"     doc:"Inclusive start block number (0 = no filter)"`
	ToBlock       uint64  `query:"to_block"       doc:"Inclusive end block number (0 = no filter)"`
	Cursor        string  `query:"cursor"         doc:"Opaque keyset pagination cursor (empty = first page)"`
	Limit         int     `query:"limit"          default:"20" minimum:"1" maximum:"100" doc:"Page size"`
	CursorBlock   *uint64 // decoded from Cursor, not a query param
}

type listBlocksInput struct {
	BlocksParams
}

type listBlocksOutput struct {
	Body struct {
		Data       []BlockRow `json:"data"`
		Pagination Pagination `json:"pagination"`
	}
}

func registerBlockHandlers(api huma.API, st *Store, headFunc func(context.Context) (uint64, error)) {
	huma.Register(api, huma.Operation{
		OperationID: "list-blocks",
		Method:      http.MethodGet,
		Path:        "/api/v1/blocks",
		Tags:        []string{"Blocks"},
		Summary:     "List blocks",
		Description: "Returns blocks newest-first with optional filters and keyset pagination.",
	}, func(ctx context.Context, input *listBlocksInput) (*listBlocksOutput, error) {
		if input.Cursor != "" {
			blockNum, _, err := decodeCursor(input.Cursor)
			if err != nil {
				return nil, huma.Error400BadRequest("invalid cursor", err)
			}
			input.CursorBlock = &blockNum
		}

		blocks, err := st.GetBlocks(ctx, input.BlocksParams)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}

		out := &listBlocksOutput{}
		out.Body.Data = blocks
		if len(blocks) == int(input.Limit) {
			last := blocks[len(blocks)-1]
			c := encodeCursor(last.BlockNumber, nil)
			out.Body.Pagination.NextCursor = &c
		}
		return out, nil
	})

	// GET /api/v1/blocks/kpis
	type kpisInput struct {
		Range string `query:"range" default:"24h" enum:"1h,24h,7d,30d,all"`
		Day   string `query:"day"   doc:"YYYY-MM-DD; overrides range (empty = not set)"`
	}
	type kpisOutput struct{ Body BlockKPIs }

	huma.Register(api, huma.Operation{
		OperationID: "get-block-kpis",
		Method:      http.MethodGet,
		Path:        "/api/v1/blocks/kpis",
		Tags:        []string{"Blocks"},
		Summary:     "Block KPI summary",
	}, func(ctx context.Context, input *kpisInput) (*kpisOutput, error) {
		var day *string
		if input.Day != "" {
			day = &input.Day
		}
		kpis, err := st.GetBlockKPIs(ctx, input.Range, day)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		if headFunc != nil {
			if head, herr := headFunc(ctx); herr == nil && head > 0 {
				kpis.MaxBlockNumber = head
			}
		}
		return &kpisOutput{Body: kpis}, nil
	})

	// GET /api/v1/blocks/series
	type seriesInput struct {
		Range string `query:"range" default:"24h" enum:"1h,24h,7d,30d,all"`
	}
	type seriesOutput struct {
		Body struct {
			Data []SeriesPoint `json:"data"`
		}
	}

	huma.Register(api, huma.Operation{
		OperationID: "get-block-series",
		Method:      http.MethodGet,
		Path:        "/api/v1/blocks/series",
		Tags:        []string{"Blocks"},
		Summary:     "Block time-series (tip revenue + top fee per bucket)",
	}, func(ctx context.Context, input *seriesInput) (*seriesOutput, error) {
		pts, err := st.GetBlockSeries(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &seriesOutput{}
		out.Body.Data = pts
		return out, nil
	})

	// GET /api/v1/blocks/payer-share
	type payerShareInput struct {
		Range string `query:"range" default:"24h" enum:"1h,24h,7d,30d,all"`
	}
	type payerShareOutput struct {
		Body struct {
			Data []PayerShareRow `json:"data"`
		}
	}

	huma.Register(api, huma.Operation{
		OperationID: "get-payer-share",
		Method:      http.MethodGet,
		Path:        "/api/v1/blocks/payer-share",
		Tags:        []string{"Blocks"},
		Summary:     "Top payer share (horizontal bar chart data)",
	}, func(ctx context.Context, input *payerShareInput) (*payerShareOutput, error) {
		rows, err := st.GetPayerShare(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &payerShareOutput{}
		out.Body.Data = rows
		return out, nil
	})

	// GET /api/v1/blocks/tipped-histogram
	type histInput struct {
		Range string `query:"range" default:"24h" enum:"1h,24h,7d,30d,all"`
	}
	type histOutput struct {
		Body struct {
			Data []TippedHistRow `json:"data"`
		}
	}

	huma.Register(api, huma.Operation{
		OperationID: "get-tipped-histogram",
		Method:      http.MethodGet,
		Path:        "/api/v1/blocks/tipped-histogram",
		Tags:        []string{"Blocks"},
		Summary:     "Tipped-tx-count histogram per block",
	}, func(ctx context.Context, input *histInput) (*histOutput, error) {
		rows, err := st.GetTippedHistogram(ctx, input.Range)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &histOutput{}
		out.Body.Data = rows
		return out, nil
	})

	// GET /api/v1/blocks/{number}
	type blockByNumInput struct {
		Number uint64 `path:"number" doc:"Block number"`
	}
	type blockByNumOutput struct{ Body *BlockRow }

	huma.Register(api, huma.Operation{
		OperationID: "get-block",
		Method:      http.MethodGet,
		Path:        "/api/v1/blocks/{number}",
		Tags:        []string{"Blocks"},
		Summary:     "Get a single block by number",
	}, func(ctx context.Context, input *blockByNumInput) (*blockByNumOutput, error) {
		blk, err := st.GetBlock(ctx, input.Number)
		if err != nil {
			return nil, huma.Error404NotFound("block not found", err)
		}
		return &blockByNumOutput{Body: blk}, nil
	})

	// GET /api/v1/blocks/{number}/txs
	type blockTxsInput struct {
		Number uint64 `path:"number" doc:"Block number"`
		Cursor string `query:"cursor" doc:"Opaque pagination cursor (empty = first page)"`
		Limit  int    `query:"limit" default:"50" minimum:"1" maximum:"200"`
	}
	type blockTxsOutput struct {
		Body struct {
			Data       []TxRow    `json:"data"`
			Pagination Pagination `json:"pagination"`
		}
	}

	huma.Register(api, huma.Operation{
		OperationID: "list-block-txs",
		Method:      http.MethodGet,
		Path:        "/api/v1/blocks/{number}/txs",
		Tags:        []string{"Blocks"},
		Summary:     "Transactions for a block",
	}, func(ctx context.Context, input *blockTxsInput) (*blockTxsOutput, error) {
		var cursorIdx *uint32
		if input.Cursor != "" {
			_, idx, err := decodeCursor(input.Cursor)
			if err != nil {
				return nil, huma.Error400BadRequest("invalid cursor", err)
			}
			cursorIdx = idx
		}
		txs, err := st.GetBlockTxs(ctx, input.Number, cursorIdx, input.Limit)
		if err != nil {
			return nil, huma.Error500InternalServerError("query failed", err)
		}
		out := &blockTxsOutput{}
		out.Body.Data = txs
		if len(txs) == int(input.Limit) {
			last := txs[len(txs)-1]
			u32 := last.TxIndex
			c := encodeCursor(input.Number, &u32)
			out.Body.Pagination.NextCursor = &c
		}
		return out, nil
	})
}
