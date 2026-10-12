package sqd

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
)

// DefaultPortalURL is the public SQD Portal dataset URL for Arbitrum One.
const DefaultPortalURL = "https://portal.sqd.dev/datasets/arbitrum-one"

const (
	maxBackoff    = 60 * time.Second
	initBackoff   = time.Second
	followPollGap = time.Second
)

// SQDMetrics holds Prometheus counters for the SQD source.
type SQDMetrics struct {
	BatchesReceived prometheus.Counter
	BlocksEmitted   prometheus.Counter
	Retries         prometheus.Counter
	Reorgs          prometheus.Counter
}

// NewSQDMetrics registers and returns SQD metrics in reg.
func NewSQDMetrics(reg prometheus.Registerer) *SQDMetrics {
	f := promauto.With(reg)
	return &SQDMetrics{
		BatchesReceived: f.NewCounter(prometheus.CounterOpts{
			Namespace: "pw", Subsystem: "sqd", Name: "batches_received_total",
			Help: "SQD Portal batch lines received.",
		}),
		BlocksEmitted: f.NewCounter(prometheus.CounterOpts{
			Namespace: "pw", Subsystem: "sqd", Name: "blocks_emitted_total",
			Help: "Blocks emitted by the SQD source.",
		}),
		Retries: f.NewCounter(prometheus.CounterOpts{
			Namespace: "pw", Subsystem: "sqd", Name: "retries_total",
			Help: "HTTP retries (429/503) against the SQD Portal.",
		}),
		Reorgs: f.NewCounter(prometheus.CounterOpts{
			Namespace: "pw", Subsystem: "sqd", Name: "reorgs_total",
			Help: "Chain reorganisations detected via 409 fork responses.",
		}),
	}
}

// SQDBlockSource streams Arbitrum One blocks from the SQD Portal.
type SQDBlockSource struct {
	baseURL    string
	httpClient *http.Client
	log        *slog.Logger
	metrics    *SQDMetrics
}

// New creates an SQDBlockSource. baseURL defaults to DefaultPortalURL when empty.
func New(baseURL string, log *slog.Logger) *SQDBlockSource {
	if baseURL == "" {
		baseURL = DefaultPortalURL
	}
	if log == nil {
		log = slog.Default()
	}
	return &SQDBlockSource{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 0}, // streaming; rely on ctx for cancellation
		log:        log,
		metrics:    NewSQDMetrics(prometheus.NewRegistry()),
	}
}

// WithMetrics overrides the metrics registry (used by callers that export them).
func (s *SQDBlockSource) WithMetrics(m *SQDMetrics) *SQDBlockSource {
	if m != nil {
		s.metrics = m
	}
	return s
}

// Name returns "sqd".
func (s *SQDBlockSource) Name() string { return "sqd" }

// streamRequest is the JSON body posted to the /stream endpoint.
type streamRequest struct {
	Type             string           `json:"type"`
	FromBlock        uint64           `json:"fromBlock"`
	ToBlock          *uint64          `json:"toBlock,omitempty"`
	IncludeAllBlocks bool             `json:"includeAllBlocks"`
	Fields           map[string]any   `json:"fields"`
	Logs             []map[string]any `json:"logs,omitempty"`
	Transactions     []map[string]any `json:"transactions,omitempty"`
}

func fieldSelection() map[string]any {
	return map[string]any{
		"block": map[string]bool{
			"number": true, "hash": true, "parentHash": true, "timestamp": true,
			"gasUsed": true, "gasLimit": true, "baseFeePerGas": true,
			"l1BlockNumber": true, "size": true,
		},
		// SQD v2: receipt fields are merged into transaction.
		// l1GasUsed is the Arbitrum-specific gasUsedForL1 field.
		"transaction": map[string]bool{
			"hash": true, "transactionIndex": true, "from": true, "to": true,
			"value": true, "gas": true, "gasPrice": true, "maxFeePerGas": true,
			"maxPriorityFeePerGas": true, "type": true, "nonce": true, "input": true,
			"status": true, "gasUsed": true, "l1GasUsed": true,
			"effectiveGasPrice": true, "cumulativeGasUsed": true, "contractAddress": true,
		},
		"log": map[string]bool{
			"address": true, "topics": true, "data": true, "logIndex": true,
			"transactionIndex": true, "transactionHash": true,
		},
	}
}

func buildRequest(from uint64, to *uint64, logAddrs []string) streamRequest {
	req := streamRequest{
		Type:             "evm",
		FromBlock:        from,
		ToBlock:          to,
		IncludeAllBlocks: true,
		Fields:           fieldSelection(),
		Transactions:     []map[string]any{{}},
	}
	if len(logAddrs) > 0 {
		req.Logs = []map[string]any{{"address": logAddrs}}
	}
	return req
}

// Head returns the current finalized head from the SQD Portal /height endpoint.
func (s *SQDBlockSource) Head(ctx context.Context) (uint64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/height", nil)
	if err != nil {
		return 0, fmt.Errorf("sqd: head request: %w", err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("sqd: head request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("sqd: head unexpected status %d", resp.StatusCode)
	}
	var n uint64
	if err := json.NewDecoder(resp.Body).Decode(&n); err != nil {
		return 0, fmt.Errorf("sqd: head decode: %w", err)
	}
	if n == 0 {
		return 0, errors.New("sqd: head not reported")
	}
	return n, nil
}

func uint64p(v uint64) *uint64 { return &v }

// Range streams blocks in [from, to] inclusive, in order.
func (s *SQDBlockSource) Range(
	ctx context.Context,
	from, to uint64,
	logAddrs []string,
) (<-chan *chain.FetchedBlock, <-chan error) {
	out := make(chan *chain.FetchedBlock)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		last := from
		for {
			err := s.stream(ctx, last, uint64p(to), logAddrs,
				func(fb *chain.FetchedBlock) error {
					if fb.Block != nil {
						last = uint64(fb.Block.Number) + 1
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					case out <- fb:
						return nil
					}
				}, nil)
			if err == nil {
				return
			}
			var fork *forkError
			if errors.As(err, &fork) {
				last = fork.resumeFrom
				s.metrics.Reorgs.Inc()
				s.log.WarnContext(ctx, "sqd reorg, resuming", "from", last, "ancestor", fork.info.CommonAncestor)
				continue
			}
			errc <- err
			return
		}
	}()

	return out, errc
}

// Follow streams new blocks from startBlock until ctx is cancelled.
func (s *SQDBlockSource) Follow(
	ctx context.Context,
	startBlock uint64,
	logAddrs []string,
) (<-chan *chain.FetchedBlock, <-chan error) {
	out := make(chan *chain.FetchedBlock)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		last := startBlock
		for {
			if ctx.Err() != nil {
				errc <- ctx.Err()
				return
			}
			err := s.stream(ctx, last, nil, logAddrs,
				func(fb *chain.FetchedBlock) error {
					if fb.Block != nil {
						last = uint64(fb.Block.Number) + 1
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					case out <- fb:
						return nil
					}
				}, nil)
			if err == nil {
				// Stream closed cleanly (caught up); re-request after a short gap.
				select {
				case <-ctx.Done():
					errc <- ctx.Err()
					return
				case <-time.After(followPollGap):
				}
				continue
			}
			var fork *forkError
			if errors.As(err, &fork) {
				last = fork.resumeFrom
				s.metrics.Reorgs.Inc()
				s.log.WarnContext(ctx, "sqd reorg in follow, resuming", "from", last)
				continue
			}
			if errors.Is(err, context.Canceled) {
				errc <- err
				return
			}
			errc <- err
			return
		}
	}()

	return out, errc
}

// forkError signals a 409 reorg; resumeFrom is the block to restart from.
type forkError struct {
	info       chain.ReorgInfo
	resumeFrom uint64
}

func (e *forkError) Error() string {
	return fmt.Sprintf("sqd fork: ancestor=%d depth=%d", e.info.CommonAncestor, e.info.Depth)
}

// stream performs one POST /stream call and dispatches each parsed block to
// emit. finStatus, if non-nil, is called when a finalizedHeadNumber status line
// is seen. A 409 reorg is reported as *forkError.
func (s *SQDBlockSource) stream(
	ctx context.Context,
	from uint64,
	to *uint64,
	logAddrs []string,
	emit func(*chain.FetchedBlock) error,
	finStatus func(uint64),
) error {
	backoffDur := initBackoff
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		retry, err := s.streamOnce(ctx, from, to, logAddrs, emit, finStatus, &from)
		if err == nil {
			return nil
		}
		var fork *forkError
		if errors.As(err, &fork) {
			return err
		}
		if !retry {
			return err
		}
		s.metrics.Retries.Inc()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoffDur):
		}
		backoffDur *= 2
		if backoffDur > maxBackoff {
			backoffDur = maxBackoff
		}
	}
}

// streamOnce issues a single HTTP request. It returns retry=true when the
// caller should back off and retry (429/503). On success it returns (false, nil).
// cursor is advanced to the next unseen block so a retry resumes correctly.
func (s *SQDBlockSource) streamOnce(
	ctx context.Context,
	from uint64,
	to *uint64,
	logAddrs []string,
	emit func(*chain.FetchedBlock) error,
	finStatus func(uint64),
	cursor *uint64,
) (bool, error) {
	body, err := json.Marshal(buildRequest(from, to, logAddrs))
	if err != nil {
		return false, fmt.Errorf("sqd: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/stream", bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("sqd: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/x-ndjson")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false, ctx.Err()
		}
		return true, fmt.Errorf("sqd: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		if err := s.parseStream(ctx, resp, to, emit, finStatus, cursor); err != nil {
			// Server dropped the connection mid-stream (truncated gzip or partial
			// NDJSON line). cursor was already advanced to the last complete block,
			// so retrying from there is safe and idempotent.
			if errors.Is(err, io.ErrUnexpectedEOF) {
				s.log.WarnContext(ctx, "sqd stream truncated, retrying", "cursor", *cursor, "err", err)
				return true, fmt.Errorf("sqd: stream truncated: %w", err)
			}
			var syntaxErr *json.SyntaxError
			if errors.As(err, &syntaxErr) {
				s.log.WarnContext(ctx, "sqd stream truncated (json), retrying", "cursor", *cursor, "err", err)
				return true, fmt.Errorf("sqd: stream truncated: %w", err)
			}
			return false, err
		}
		return false, nil
	case http.StatusTooManyRequests:
		s.applyRetryAfter(ctx, resp)
		return true, fmt.Errorf("sqd: 429 too many requests")
	case http.StatusServiceUnavailable:
		return true, fmt.Errorf("sqd: 503 service unavailable")
	case 529: // overloaded — treat same as 503, back off and retry
		return true, fmt.Errorf("sqd: 529 overloaded")
	case http.StatusConflict:
		return false, s.parseFork(resp)
	default:
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return false, fmt.Errorf("sqd: unexpected status %d: %s", resp.StatusCode, string(msg))
	}
}

// applyRetryAfter sleeps for the Retry-After header duration, if present.
func (s *SQDBlockSource) applyRetryAfter(ctx context.Context, resp *http.Response) {
	ra := resp.Header.Get("Retry-After")
	if ra == "" {
		return
	}
	secs, err := strconv.Atoi(ra)
	if err != nil || secs <= 0 {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Duration(secs) * time.Second):
	}
}

// parseFork decodes a 409 body into a *forkError, locating the common ancestor.
func (s *SQDBlockSource) parseFork(resp *http.Response) error {
	r, err := bodyReader(resp)
	if err != nil {
		return err
	}
	var fork SQDForkResponse
	if derr := json.NewDecoder(r).Decode(&fork); derr != nil {
		return fmt.Errorf("sqd: decode fork body: %w", derr)
	}
	if len(fork.PreviousBlocks) == 0 {
		return errors.New("sqd: 409 fork with no previousBlocks")
	}
	ancestor := fork.PreviousBlocks[len(fork.PreviousBlocks)-1]
	info := chain.ReorgInfo{
		CommonAncestor: ancestor.Number,
		OldHash:        fork.PreviousBlocks[0].Hash,
		NewHash:        ancestor.Hash,
		Depth:          uint32(len(fork.PreviousBlocks)), //nolint:gosec
	}
	return &forkError{info: info, resumeFrom: ancestor.Number}
}

// bodyReader returns a reader for resp.Body, transparently gunzipping if needed.
func bodyReader(resp *http.Response) (io.Reader, error) {
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("sqd: gzip reader: %w", err)
		}
		return gz, nil
	}
	return resp.Body, nil
}

// parseStream reads NDJSON batch/status lines and emits assembled blocks.
func (s *SQDBlockSource) parseStream(
	ctx context.Context,
	resp *http.Response,
	to *uint64,
	emit func(*chain.FetchedBlock) error,
	finStatus func(uint64),
	cursor *uint64,
) error {
	r, err := bodyReader(resp)
	if err != nil {
		return err
	}

	var finalized uint64
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		// Status lines carry "status"; batch lines carry "header".
		if status, ok := tryStatusLine(line); ok {
			if status.Status == "finalizedHeadNumber" {
				finalized = status.Value
				if finStatus != nil {
					finStatus(status.Value)
				}
			}
			continue
		}

		var batch SQDBatch
		if derr := json.Unmarshal(line, &batch); derr != nil {
			return fmt.Errorf("sqd: decode batch line: %w", derr)
		}
		s.metrics.BatchesReceived.Inc()

		fb, berr := s.assemble(batch, finalized)
		if berr != nil {
			return berr
		}
		if cursor != nil {
			*cursor = uint64(fb.Block.Number) + 1
		}
		s.metrics.BlocksEmitted.Inc()
		if eerr := emit(fb); eerr != nil {
			return eerr
		}
		if to != nil && uint64(fb.Block.Number) >= *to {
			return nil
		}
	}
	if serr := scanner.Err(); serr != nil {
		if errors.Is(serr, context.Canceled) {
			return ctx.Err()
		}
		return fmt.Errorf("sqd: scan stream: %w", serr)
	}
	return nil
}

// tryStatusLine attempts to decode a line as an SQD status line.
// It returns ok=false when the line is a batch (contains "header").
func tryStatusLine(line []byte) (SQDStatusLine, bool) {
	var probe struct {
		Status string `json:"status"`
		Header *struct {
			Number uint64 `json:"number"`
		} `json:"header"`
	}
	if err := json.Unmarshal(line, &probe); err != nil {
		return SQDStatusLine{}, false
	}
	if probe.Header != nil || probe.Status == "" {
		return SQDStatusLine{}, false
	}
	var sl SQDStatusLine
	if err := json.Unmarshal(line, &sl); err != nil {
		return SQDStatusLine{}, false
	}
	return sl, true
}

// assemble converts an SQD batch into a chain.FetchedBlock.
// SQD v2 embeds receipt data in transactions; receipts are synthesised from them.
func (s *SQDBlockSource) assemble(batch SQDBatch, finalized uint64) (*chain.FetchedBlock, error) {
	block, err := toRPCBlock(batch.Header, batch.Transactions)
	if err != nil {
		return nil, err
	}

	receipts := make([]chain.RPCReceipt, 0, len(batch.Transactions))
	for _, tx := range batch.Transactions {
		rr, rerr := toRPCReceipt(tx, batch.Header.Number, batch.Header.Hash)
		if rerr != nil {
			return nil, rerr
		}
		receipts = append(receipts, rr)
	}

	logs := make([]chain.RPCLog, 0, len(batch.Logs))
	for _, l := range batch.Logs {
		ll, lerr := toRPCLog(l, batch.Header.Number)
		if lerr != nil {
			return nil, lerr
		}
		logs = append(logs, ll)
	}

	return &chain.FetchedBlock{
		Block:         block,
		Receipts:      receipts,
		Logs:          logs,
		FinalizedHead: finalized,
	}, nil
}
