package chain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	gethRPC "github.com/ethereum/go-ethereum/rpc"

	"github.com/cenkalti/backoff/v4"
	"github.com/ethereum/go-ethereum/common"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
)

// errTooManyResults is the sentinel returned (or wrapped) by Arbitrum nodes
// when an eth_getLogs call would return too many results.
var errTooManyResults = errors.New("too many results")

// Client is a rate-limited, retry-safe, failover Arbitrum One JSON-RPC client.
// Create with New; all methods are safe for concurrent use.
type Client struct {
	endpoints []string // failover list; endpoints[0] is primary
	active    int      // index of the currently active endpoint
	conn      *gethRPC.Client
	limiter   *rate.Limiter
	metrics   *ClientMetrics
	log       *slog.Logger
}

// ClientConfig carries options for New.
type ClientConfig struct {
	// PrimaryURL is required.
	PrimaryURL string
	// FallbackURLs are tried in order when the primary is unavailable.
	FallbackURLs []string
	// RPS controls the token-bucket rate limiter (requests per second).
	RPS float64
	// Logger is used for retry/failover events.  Defaults to discarding logger.
	Logger *slog.Logger
	// Registry is used to register Prometheus metrics.  Defaults to the default registry.
	Registry prometheus.Registerer
}

// New dials the primary RPC endpoint and returns a ready-to-use Client.
func New(ctx context.Context, cfg ClientConfig) (*Client, error) {
	if cfg.PrimaryURL == "" {
		return nil, errors.New("chain: PrimaryURL is required")
	}
	if cfg.RPS <= 0 {
		cfg.RPS = 25
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Registry == nil {
		cfg.Registry = prometheus.DefaultRegisterer
	}

	endpoints := append([]string{cfg.PrimaryURL}, cfg.FallbackURLs...)

	conn, err := gethRPC.DialContext(ctx, endpoints[0])
	if err != nil {
		return nil, fmt.Errorf("chain: dial %s: %w", endpoints[0], err)
	}

	return &Client{
		endpoints: endpoints,
		active:    0,
		conn:      conn,
		limiter:   rate.NewLimiter(rate.Limit(cfg.RPS), int(cfg.RPS)+1),
		metrics:   NewClientMetrics(cfg.Registry),
		log:       cfg.Logger,
	}, nil
}

// Close releases the underlying connection.
func (c *Client) Close() { c.conn.Close() }

// BlockWithTxs fetches a block by number with full transaction objects.
// Passing 0 as number returns the latest block.
func (c *Client) BlockWithTxs(ctx context.Context, number uint64) (*RPCBlock, error) {
	var tag any
	if number == 0 {
		tag = "latest"
	} else {
		tag = fmt.Sprintf("0x%x", number)
	}

	var block RPCBlock
	err := c.call(ctx, "eth_getBlockByNumber", &block, tag, true)
	if err != nil {
		return nil, err
	}
	return &block, nil
}

// BlockHeader fetches a block header (no transactions) by number.
// Passing 0 returns the latest block.
func (c *Client) BlockHeader(ctx context.Context, number uint64) (*RPCBlock, error) {
	var tag any
	if number == 0 {
		tag = "latest"
	} else {
		tag = fmt.Sprintf("0x%x", number)
	}
	var block RPCBlock
	if err := c.call(ctx, "eth_getBlockByNumber", &block, tag, false); err != nil {
		return nil, err
	}
	return &block, nil
}

// Head returns the latest canonical block (no transactions).
func (c *Client) Head(ctx context.Context) (*RPCBlock, error) {
	var block RPCBlock
	if err := c.call(ctx, "eth_getBlockByNumber", &block, "latest", false); err != nil {
		return nil, err
	}
	return &block, nil
}

// Safe returns the latest safe block (no transactions).
func (c *Client) Safe(ctx context.Context) (*RPCBlock, error) {
	var block RPCBlock
	if err := c.call(ctx, "eth_getBlockByNumber", &block, "safe", false); err != nil {
		return nil, err
	}
	return &block, nil
}

// Finalized returns the latest finalized block (no transactions).
func (c *Client) Finalized(ctx context.Context) (*RPCBlock, error) {
	var block RPCBlock
	if err := c.call(ctx, "eth_getBlockByNumber", &block, "finalized", false); err != nil {
		return nil, err
	}
	return &block, nil
}

// BlockReceipts fetches all receipts for blockHash.
// It tries eth_getBlockReceipts first; if the node does not support it, it
// falls back to fetching receipts for each transaction individually.
func (c *Client) BlockReceipts(ctx context.Context, blockHash common.Hash) ([]RPCReceipt, error) {
	var receipts []RPCReceipt
	err := c.call(ctx, "eth_getBlockReceipts", &receipts, blockHash)
	if err == nil {
		return receipts, nil
	}

	// Fallback: fetch the block to get tx hashes, then batch per-tx receipts.
	if !isUnsupportedMethod(err) {
		return nil, err
	}
	c.log.InfoContext(ctx, "eth_getBlockReceipts unsupported, falling back to per-tx receipts",
		"block", blockHash)

	var block RPCBlock
	if err := c.call(ctx, "eth_getBlockByHash", &block, blockHash, false); err != nil {
		return nil, fmt.Errorf("block receipts fallback get block: %w", err)
	}

	if len(block.Transactions) == 0 {
		return nil, nil
	}

	// Build batch of eth_getTransactionReceipt calls.
	calls := make([]gethRPC.BatchElem, len(block.Transactions))
	results := make([]RPCReceipt, len(block.Transactions))
	for i, tx := range block.Transactions {
		calls[i] = gethRPC.BatchElem{
			Method: "eth_getTransactionReceipt",
			Args:   []any{tx.Hash},
			Result: &results[i],
		}
	}

	if err := c.callBatch(ctx, calls); err != nil {
		return nil, fmt.Errorf("block receipts fallback batch: %w", err)
	}
	for i, elem := range calls {
		if elem.Error != nil {
			return nil, fmt.Errorf("receipt for tx %s: %w", block.Transactions[i].Hash, elem.Error)
		}
	}
	return results, nil
}

// Logs fetches event logs matching filter.  If the node returns a
// "too-many-results" error the range is halved and retried automatically.
func (c *Client) Logs(ctx context.Context, filter LogFilter) ([]RPCLog, error) {
	return c.logsRecursive(ctx, filter)
}

// logsRecursive handles range splitting on too-many-results errors.
func (c *Client) logsRecursive(ctx context.Context, filter LogFilter) ([]RPCLog, error) {
	var logs []RPCLog
	err := c.call(ctx, "eth_getLogs", &logs, filter)
	if err == nil {
		return logs, nil
	}

	if !isTooManyResults(err) {
		return nil, err
	}

	// Split the range in half and recurse.
	from, to, ok := parseBlockRange(filter)
	if !ok || to-from < 1 {
		return nil, fmt.Errorf("logs: cannot split range further: %w", err)
	}
	mid := from + (to-from)/2

	lo := filter
	lo.FromBlock = fmt.Sprintf("0x%x", from)
	lo.ToBlock = fmt.Sprintf("0x%x", mid)

	hi := filter
	hi.FromBlock = fmt.Sprintf("0x%x", mid+1)
	hi.ToBlock = fmt.Sprintf("0x%x", to)

	loLogs, err := c.logsRecursive(ctx, lo)
	if err != nil {
		return nil, err
	}
	hiLogs, err := c.logsRecursive(ctx, hi)
	if err != nil {
		return nil, err
	}
	return append(loLogs, hiLogs...), nil
}

// Balance returns the ETH balance of addr at blockNumber (hex-encoded).
// Pass "latest" for the current balance.
func (c *Client) Balance(ctx context.Context, addr common.Address, blockTag string) (*big.Int, error) {
	if blockTag == "" {
		blockTag = "latest"
	}
	var result hexBig
	if err := c.call(ctx, "eth_getBalance", &result, addr, blockTag); err != nil {
		return nil, err
	}
	if result.Int == nil {
		return new(big.Int), nil
	}
	return new(big.Int).Set(result.Int), nil
}

// hexBig wraps *big.Int with hex JSON unmarshalling.
type hexBig struct{ *big.Int }

func (h *hexBig) UnmarshalJSON(b []byte) error {
	var s string
	if err := unmarshalJSONString(b, &s); err != nil {
		return err
	}
	if s == "" || s == "0x" {
		h.Int = new(big.Int)
		return nil
	}
	i, ok := new(big.Int).SetString(s[2:], 16)
	if !ok {
		return fmt.Errorf("hexBig: invalid hex %q", s)
	}
	h.Int = i
	return nil
}

func unmarshalJSONString(b []byte, s *string) error {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("expected JSON string, got %s", b)
	}
	*s = string(b[1 : len(b)-1])
	return nil
}

// call is the central method that applies rate limiting, per-request timeout,
// exponential backoff with jitter, and endpoint failover.
func (c *Client) call(ctx context.Context, method string, result any, args ...any) error {
	start := time.Now()

	bo := backoff.WithContext(newBackoff(), ctx)

	var attempt int
	err := backoff.Retry(func() error {
		if attempt > 0 {
			c.metrics.incRetry(method)
		}
		attempt++

		if err := c.limiter.Wait(ctx); err != nil {
			return backoff.Permanent(err)
		}

		callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		err := c.conn.CallContext(callCtx, result, method, args...)
		if err == nil {
			return nil
		}

		if isPermanentRPCError(err) {
			return backoff.Permanent(err)
		}

		// On retryable error, try the next endpoint.
		c.failover(ctx)
		return err
	}, bo)

	status := "ok"
	if err != nil {
		status = "error"
	}
	c.metrics.incRequest(method, status)
	c.metrics.observeDuration(method, time.Since(start).Seconds())
	return err
}

// callBatch sends a JSON-RPC batch request with rate limiting and backoff.
func (c *Client) callBatch(ctx context.Context, elems []gethRPC.BatchElem) error {
	method := "batch"
	start := time.Now()

	bo := backoff.WithContext(newBackoff(), ctx)

	var attempt int
	err := backoff.Retry(func() error {
		if attempt > 0 {
			c.metrics.incRetry(method)
		}
		attempt++

		if err := c.limiter.Wait(ctx); err != nil {
			return backoff.Permanent(err)
		}

		callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		err := c.conn.BatchCallContext(callCtx, elems)
		if err == nil {
			return nil
		}
		if isPermanentRPCError(err) {
			return backoff.Permanent(err)
		}
		c.failover(ctx)
		return err
	}, bo)

	status := "ok"
	if err != nil {
		status = "error"
	}
	c.metrics.incRequest(method, status)
	c.metrics.observeDuration(method, time.Since(start).Seconds())
	return err
}

// failover dials the next endpoint in the list and replaces c.conn.
func (c *Client) failover(ctx context.Context) {
	if len(c.endpoints) <= 1 {
		return
	}
	next := (c.active + 1) % len(c.endpoints)
	c.log.WarnContext(ctx, "rpc failover",
		"from", c.endpoints[c.active],
		"to", c.endpoints[next],
	)
	conn, err := gethRPC.DialContext(ctx, c.endpoints[next])
	if err != nil {
		c.log.ErrorContext(ctx, "rpc failover dial failed", "url", c.endpoints[next], "err", err)
		return
	}
	c.conn.Close()
	c.conn = conn
	c.active = next
}

// newBackoff returns an exponential back-off with jitter suitable for RPC retries.
func newBackoff() backoff.BackOff {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 200 * time.Millisecond
	bo.Multiplier = 2
	bo.MaxInterval = 10 * time.Second
	bo.MaxElapsedTime = 60 * time.Second
	return bo
}

// isPermanentRPCError returns true for errors that should not be retried.
func isPermanentRPCError(err error) bool {
	if err == nil {
		return false
	}
	var rpcErr gethRPC.Error
	if errors.As(err, &rpcErr) {
		code := rpcErr.ErrorCode()
		// -32600 invalid request, -32601 method not found, -32602 invalid params,
		// -32700 parse error — all are permanent.
		return code <= -32600 || code == -32700
	}
	// Context cancellation / deadline exceeded — don't retry.
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// isUnsupportedMethod returns true when the node rejects the method as unknown.
func isUnsupportedMethod(err error) bool {
	var rpcErr gethRPC.Error
	if errors.As(err, &rpcErr) {
		return rpcErr.ErrorCode() == -32601
	}
	return false
}

// isTooManyResults returns true for Arbitrum's "logs response limit exceeded" error.
func isTooManyResults(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errTooManyResults) {
		return true
	}
	var rpcErr gethRPC.Error
	if errors.As(err, &rpcErr) {
		msg := rpcErr.Error()
		return containsAny(msg, "query returned more than", "too many results", "response size limit")
	}
	return containsAny(err.Error(), "query returned more than", "too many results", "response size limit")
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// parseBlockRange extracts from/to block numbers from a LogFilter.
// Returns (0, 0, false) if the range cannot be parsed as hex numbers.
func parseBlockRange(f LogFilter) (from, to uint64, ok bool) {
	if f.FromBlock == "" || f.ToBlock == "" {
		return 0, 0, false
	}
	var fromB, toB uint64
	if _, err := fmt.Sscanf(f.FromBlock, "0x%x", &fromB); err != nil {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(f.ToBlock, "0x%x", &toB); err != nil {
		return 0, 0, false
	}
	return fromB, toB, true
}
