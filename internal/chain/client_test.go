package chain_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
)

// jrpcResponse is a minimal JSON-RPC 2.0 response.
type jrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *jrpcError      `json:"error,omitempty"`
}

type jrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type jrpcRequest struct {
	Method string            `json:"method"`
	ID     json.RawMessage   `json:"id"`
	Params []json.RawMessage `json:"params"`
}

// makeHandler returns an http.HandlerFunc that serves a fixed result for a
// given method.  The handler increments callCount on each request.
func makeHandler(method string, result any, callCount *atomic.Int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req jrpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		callCount.Add(1)

		resp := jrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
		}
		if req.Method != method {
			resp.Error = &jrpcError{Code: -32601, Message: "method not found"}
		} else {
			resp.Result = result
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// makeErrorHandler returns a handler that always responds with a JSON-RPC error.
func makeErrorHandler(code int, msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req jrpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := jrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jrpcError{Code: code, Message: msg},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// newTestClient creates a chain.Client pointing at ts.
func newTestClient(t *testing.T, ts *httptest.Server, fallbacks ...string) *chain.Client {
	t.Helper()
	reg := prometheus.NewRegistry()
	c, err := chain.New(t.Context(), chain.ClientConfig{
		PrimaryURL:   ts.URL,
		FallbackURLs: fallbacks,
		RPS:          1000,
		Registry:     reg,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

// minimalBlock is a valid RPCBlock JSON fragment.
const minimalBlock = `{
  "number":"0x1",
  "hash":"0xaaaa000000000000000000000000000000000000000000000000000000000001",
  "parentHash":"0x0000000000000000000000000000000000000000000000000000000000000000",
  "timestamp":"0x67000000",
  "l1BlockNumber":"0x100",
  "baseFeePerGas":"0x1",
  "gasUsed":"0x0",
  "gasLimit":"0x1fca055",
  "size":"0x100",
  "transactions":[]
}`

func TestClientHead(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	var block map[string]any
	_ = json.Unmarshal([]byte(minimalBlock), &block)

	ts := httptest.NewServer(makeHandler("eth_getBlockByNumber", block, &calls))
	defer ts.Close()

	c := newTestClient(t, ts)
	head, err := c.Head(context.Background())
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if head == nil {
		t.Fatal("Head returned nil block")
	}
	if uint64(head.Number) != 1 {
		t.Errorf("head.Number = %d, want 1", uint64(head.Number))
	}
}

func TestClientMethodNotFound(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	ts := httptest.NewServer(makeErrorHandler(-32601, "method not found"))
	defer ts.Close()

	c := newTestClient(t, ts)
	_, err := c.BlockWithTxs(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error for -32601, got nil")
	}
	calls.Load() // ensure variable is used
}

func TestClientPermanentError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(makeErrorHandler(-32602, "invalid params"))
	defer ts.Close()

	c := newTestClient(t, ts)
	_, err := c.BlockWithTxs(context.Background(), 99)
	if err == nil {
		t.Fatal("expected permanent error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid params") {
		t.Errorf("error %q should mention 'invalid params'", err)
	}
}

func TestClientBlockReceiptsFallback(t *testing.T) {
	t.Parallel()

	// eth_getBlockReceipts → -32601 (unsupported)
	// eth_getBlockByHash → block with one tx
	// eth_getTransactionReceipt → receipt

	blockHash := "0xaaaa000000000000000000000000000000000000000000000000000000000001"
	txHash := "0xbbbb000000000000000000000000000000000000000000000000000000000001"

	receiptJSON := fmt.Sprintf(`{
		"transactionHash":"%s",
		"blockHash":"%s",
		"blockNumber":"0x1",
		"transactionIndex":"0x0",
		"from":"0x0000000000000000000000000000000000000001",
		"to":"0x0000000000000000000000000000000000000002",
		"contractAddress":null,
		"status":"0x1",
		"gasUsed":"0x5208",
		"cumulativeGasUsed":"0x5208",
		"effectiveGasPrice":"0x1",
		"gasUsedForL1":"0x0",
		"l1BlockNumber":"0x100",
		"logs":[]
	}`, txHash, blockHash)

	blockJSON := fmt.Sprintf(`{
		"number":"0x1",
		"hash":"%s",
		"parentHash":"0x0000000000000000000000000000000000000000000000000000000000000000",
		"timestamp":"0x67000000",
		"l1BlockNumber":"0x100",
		"baseFeePerGas":"0x1",
		"gasUsed":"0x5208",
		"gasLimit":"0x1fca055",
		"size":"0x200",
		"transactions":[{
			"hash":"%s",
			"blockHash":"%s",
			"blockNumber":"0x1",
			"transactionIndex":"0x0",
			"from":"0x0000000000000000000000000000000000000001",
			"to":"0x0000000000000000000000000000000000000002",
			"value":"0x0","gas":"0x5208","gasPrice":"0x1","type":"0x0","nonce":"0x0","input":"0x"
		}]
	}`, blockHash, txHash, blockHash)

	var calls atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		// Support both single and batch requests.
		body, _ := readAll(r.Body)

		// Determine if it's a batch (array) or single request.
		trimmed := strings.TrimSpace(string(body))
		isBatch := strings.HasPrefix(trimmed, "[")

		writeResp := func(req jrpcRequest) jrpcResponse {
			switch req.Method {
			case "eth_getBlockReceipts":
				return jrpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &jrpcError{Code: -32601, Message: "method not found"}}
			case "eth_getBlockByHash":
				var blk map[string]any
				_ = json.Unmarshal([]byte(blockJSON), &blk)
				return jrpcResponse{JSONRPC: "2.0", ID: req.ID, Result: blk}
			case "eth_getTransactionReceipt":
				var rec map[string]any
				_ = json.Unmarshal([]byte(receiptJSON), &rec)
				return jrpcResponse{JSONRPC: "2.0", ID: req.ID, Result: rec}
			default:
				return jrpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &jrpcError{Code: -32601, Message: "unknown"}}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if isBatch {
			var reqs []jrpcRequest
			_ = json.Unmarshal(body, &reqs)
			resps := make([]jrpcResponse, len(reqs))
			for i, req := range reqs {
				resps[i] = writeResp(req)
			}
			_ = json.NewEncoder(w).Encode(resps)
		} else {
			var req jrpcRequest
			_ = json.Unmarshal(body, &req)
			_ = json.NewEncoder(w).Encode(writeResp(req))
		}
	}))
	defer ts.Close()

	c := newTestClient(t, ts)
	receipts, err := c.BlockReceipts(context.Background(), common.HexToHash(blockHash))
	if err != nil {
		t.Fatalf("BlockReceipts: %v", err)
	}
	if len(receipts) != 1 {
		t.Errorf("got %d receipts, want 1", len(receipts))
	}
}

func TestClientLogsRangeSplit(t *testing.T) {
	t.Parallel()

	// First call returns too-many-results error; subsequent calls succeed.
	var callCount atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		var req jrpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		resp := jrpcResponse{JSONRPC: "2.0", ID: req.ID}
		if n == 1 {
			// First call: too many results
			resp.Error = &jrpcError{Code: -32602, Message: "query returned more than 10000 results"}
		} else {
			resp.Result = []any{} // empty logs
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(t, ts)
	filter := chain.LogFilter{
		FromBlock: "0x1",
		ToBlock:   "0x64",
	}
	logs, err := c.Logs(context.Background(), filter)
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("got %d logs, want 0", len(logs))
	}
	// Should have been called more than once due to splitting.
	if callCount.Load() < 2 {
		t.Errorf("callCount = %d, want ≥ 2 (range should have been split)", callCount.Load())
	}
}

// readAll reads all bytes from rc.
func readAll(rc interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var buf []byte
	tmp := make([]byte, 512)
	for {
		n, err := rc.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}
