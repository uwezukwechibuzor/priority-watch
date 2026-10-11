package hypersync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
)

// DefaultURL is the public Envio HyperSync endpoint for Arbitrum One.
const DefaultURL = "https://arbitrum.hypersync.xyz"

const (
	maxBackoff    = 60 * time.Second
	initBackoff   = time.Second
	followPollGap = time.Second
)

// HyperSyncSource streams Arbitrum One blocks from the Envio HyperSync API.
type HyperSyncSource struct {
	baseURL    string
	token      string
	httpClient *http.Client
	log        *slog.Logger
}

// New creates a HyperSyncSource. baseURL defaults to DefaultURL when empty.
// token is optional; when set it is sent as a Bearer Authorization header.
func New(baseURL, token string, log *slog.Logger) *HyperSyncSource {
	if baseURL == "" {
		baseURL = DefaultURL
	}
	if log == nil {
		log = slog.Default()
	}
	return &HyperSyncSource{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: 2 * time.Minute},
		log:        log,
	}
}

// Name returns "hypersync".
func (s *HyperSyncSource) Name() string { return "hypersync" }

// Head returns the current archive height from GET /height.
func (s *HyperSyncSource) Head(ctx context.Context) (uint64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/height", nil)
	if err != nil {
		return 0, fmt.Errorf("hypersync: new height request: %w", err)
	}
	s.auth(req)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("hypersync: height request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("hypersync: height status %d", resp.StatusCode)
	}
	var out struct {
		Height uint64 `json:"height"`
	}
	if derr := json.NewDecoder(resp.Body).Decode(&out); derr != nil {
		return 0, fmt.Errorf("hypersync: decode height: %w", derr)
	}
	return out.Height, nil
}

func (s *HyperSyncSource) auth(req *http.Request) {
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
}

// Range streams blocks in [from, to] inclusive, in order, paginating via next_block.
func (s *HyperSyncSource) Range(
	ctx context.Context,
	from, to uint64,
	logAddrs []string,
) (<-chan *chain.FetchedBlock, <-chan error) {
	out := make(chan *chain.FetchedBlock)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		next := from
		for next <= to {
			resp, err := s.query(ctx, next, uint64p(to+1), logAddrs)
			if err != nil {
				errc <- err
				return
			}
			blocks, perr := s.pageBlocks(resp)
			if perr != nil {
				errc <- perr
				return
			}
			for _, fb := range blocks {
				if uint64(fb.Block.Number) > to {
					continue
				}
				select {
				case <-ctx.Done():
					errc <- ctx.Err()
					return
				case out <- fb:
				}
			}
			if resp.NextBlock <= next {
				// No progress; avoid an infinite loop.
				return
			}
			next = resp.NextBlock
		}
	}()

	return out, errc
}

// Follow polls the head and requests [last+1, head] repeatedly.
func (s *HyperSyncSource) Follow(
	ctx context.Context,
	startBlock uint64,
	logAddrs []string,
) (<-chan *chain.FetchedBlock, <-chan error) {
	out := make(chan *chain.FetchedBlock)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		next := startBlock
		for {
			if ctx.Err() != nil {
				errc <- ctx.Err()
				return
			}
			head, err := s.Head(ctx)
			if err != nil {
				errc <- err
				return
			}
			for next <= head {
				resp, qerr := s.query(ctx, next, uint64p(head+1), logAddrs)
				if qerr != nil {
					errc <- qerr
					return
				}
				blocks, perr := s.pageBlocks(resp)
				if perr != nil {
					errc <- perr
					return
				}
				for _, fb := range blocks {
					select {
					case <-ctx.Done():
						errc <- ctx.Err()
						return
					case out <- fb:
					}
				}
				if resp.NextBlock <= next {
					break
				}
				next = resp.NextBlock
			}
			select {
			case <-ctx.Done():
				errc <- ctx.Err()
				return
			case <-time.After(followPollGap):
			}
		}
	}()

	return out, errc
}

func uint64p(v uint64) *uint64 { return &v }

// query issues a single POST /query with retry on 429/503.
func (s *HyperSyncSource) query(
	ctx context.Context,
	from uint64,
	to *uint64,
	logAddrs []string,
) (*HyperSyncResponse, error) {
	q := HyperSyncQuery{
		FromBlock:        from,
		ToBlock:          to,
		IncludeAllBlocks: true,
		Transactions:     []HSEmptyFilter{{}},
		FieldSelection:   fieldSelection(),
	}
	if len(logAddrs) > 0 {
		q.Logs = []HSLogFilter{{Address: logAddrs}}
	}

	body, err := json.Marshal(q)
	if err != nil {
		return nil, fmt.Errorf("hypersync: marshal query: %w", err)
	}

	backoffDur := initBackoff
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		resp, retry, qerr := s.queryOnce(ctx, body)
		if qerr == nil {
			return resp, nil
		}
		if !retry {
			return nil, qerr
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoffDur):
		}
		backoffDur *= 2
		if backoffDur > maxBackoff {
			backoffDur = maxBackoff
		}
	}
}

// queryOnce performs one HTTP request. retry=true signals a backoff-and-retry.
func (s *HyperSyncSource) queryOnce(ctx context.Context, body []byte) (*HyperSyncResponse, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/query", bytes.NewReader(body))
	if err != nil {
		return nil, false, fmt.Errorf("hypersync: new query request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	s.auth(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, false, ctx.Err()
		}
		return nil, true, fmt.Errorf("hypersync: do query: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		var out HyperSyncResponse
		if derr := json.NewDecoder(resp.Body).Decode(&out); derr != nil {
			return nil, false, fmt.Errorf("hypersync: decode response: %w", derr)
		}
		return &out, false, nil
	case http.StatusTooManyRequests:
		s.applyRetryAfter(ctx, resp)
		return nil, true, fmt.Errorf("hypersync: 429 too many requests")
	case http.StatusServiceUnavailable:
		return nil, true, fmt.Errorf("hypersync: 503 service unavailable")
	default:
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, false, fmt.Errorf("hypersync: unexpected status %d: %s", resp.StatusCode, string(msg))
	}
}

func (s *HyperSyncSource) applyRetryAfter(ctx context.Context, resp *http.Response) {
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

// pageBlocks assembles blocks from all data pages, sorted by block number.
// Transactions, receipts and logs are grouped by their block number.
func (s *HyperSyncSource) pageBlocks(resp *HyperSyncResponse) ([]*chain.FetchedBlock, error) {
	type agg struct {
		block    HSBlock
		haveBlk  bool
		txs      []HSTransaction
		logs     []HSLog
		receipts []HSReceipt
	}
	byNum := make(map[uint64]*agg)
	get := func(n uint64) *agg {
		a, ok := byNum[n]
		if !ok {
			a = &agg{}
			byNum[n] = a
		}
		return a
	}

	for _, page := range resp.Data {
		for _, b := range page.Blocks {
			a := get(b.Number)
			a.block = b
			a.haveBlk = true
		}
		for _, tx := range page.Transactions {
			a := get(tx.BlockNumber)
			a.txs = append(a.txs, tx)
		}
		for _, l := range page.Logs {
			a := get(l.BlockNumber)
			a.logs = append(a.logs, l)
		}
		for _, r := range page.Receipts {
			a := get(r.BlockNumber)
			a.receipts = append(a.receipts, r)
		}
	}

	nums := make([]uint64, 0, len(byNum))
	for n, a := range byNum {
		if a.haveBlk {
			nums = append(nums, n)
		}
	}
	sort.Slice(nums, func(i, j int) bool { return nums[i] < nums[j] })

	out := make([]*chain.FetchedBlock, 0, len(nums))
	for _, n := range nums {
		a := byNum[n]
		sort.Slice(a.txs, func(i, j int) bool { return a.txs[i].TransactionIndex < a.txs[j].TransactionIndex })
		sort.Slice(a.receipts, func(i, j int) bool {
			return a.receipts[i].TransactionIndex < a.receipts[j].TransactionIndex
		})

		block, err := toRPCBlock(a.block, a.txs)
		if err != nil {
			return nil, err
		}
		receipts := make([]chain.RPCReceipt, 0, len(a.receipts))
		for _, r := range a.receipts {
			rr, rerr := toRPCReceipt(r, a.block.Hash)
			if rerr != nil {
				return nil, rerr
			}
			receipts = append(receipts, rr)
		}
		logs := make([]chain.RPCLog, 0, len(a.logs))
		for _, l := range a.logs {
			ll, lerr := toRPCLog(l)
			if lerr != nil {
				return nil, lerr
			}
			logs = append(logs, ll)
		}
		out = append(out, &chain.FetchedBlock{
			Block:         block,
			Receipts:      receipts,
			Logs:          logs,
			FinalizedHead: resp.ArchiveHeight,
		})
	}
	return out, nil
}
