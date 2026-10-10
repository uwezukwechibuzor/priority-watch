package timeboost

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// InsertBidSQL is the INSERT statement for raw.timeboost_bids_ext.
const InsertBidSQL = `INSERT INTO raw.timeboost_bids_ext
(round, bidder, express_lane_controller, amount_wei, bid_timestamp, source, _version)`

// InsertResultSQL is the INSERT statement for raw.timeboost_auction_results_ext.
const InsertResultSQL = `INSERT INTO raw.timeboost_auction_results_ext
(round, winner, express_lane_controller, first_price_wei, second_price_wei, round_start, round_end, source, _version)`

// BatchAppender is a minimal interface satisfied by store.BatchWriter.
type BatchAppender interface {
	Append(ctx context.Context, cols ...any) error
	Flush(ctx context.Context) error
}

// Querier is a minimal interface for running raw ClickHouse queries.
type Querier interface {
	QueryRow(ctx context.Context, query string, args ...any) Row
}

// Row is a minimal interface for scanning a single ClickHouse row.
type Row interface {
	Scan(dest ...any) error
}

// Config holds configuration for the Timeboost importer.
type Config struct {
	// BidsURL is the base URL template for bids NDJSON (date formatted as YYYY-MM-DD appended).
	BidsURL string
	// ResultsURL is the base URL template for results NDJSON (date formatted as YYYY-MM-DD appended).
	ResultsURL string
	// RateLimit is the maximum number of HTTP requests per minute. Defaults to 60.
	RateLimit int
	// From is the start of the date range to import (inclusive).
	From time.Time
	// To is the end of the date range to import (inclusive).
	To time.Time
}

// rawBid is the JSON shape of a single bid record from timeboost.app.
type rawBid struct {
	Round                 uint64 `json:"round"`
	Bidder                string `json:"bidder"`
	ExpressLaneController string `json:"express_lane_controller"`
	AmountWei             string `json:"amount_wei"`
	BidTimestamp          string `json:"bid_timestamp"`
}

// rawResult is the JSON shape of a single auction result record from timeboost.app.
type rawResult struct {
	Round                 uint64 `json:"round"`
	Winner                string `json:"winner"`
	ExpressLaneController string `json:"express_lane_controller"`
	FirstPriceWei         string `json:"first_price_wei"`
	SecondPriceWei        string `json:"second_price_wei"`
	RoundStart            string `json:"round_start"`
	RoundEnd              string `json:"round_end"`
}

// Bid is a parsed timeboost auction bid.
type Bid struct {
	Round                 uint64
	Bidder                string
	ExpressLaneController string
	AmountWei             *big.Int
	BidTimestamp          time.Time
}

// Result is a parsed timeboost auction result.
type Result struct {
	Round                 uint64
	Winner                string
	ExpressLaneController string
	FirstPriceWei         *big.Int
	SecondPriceWei        *big.Int
	RoundStart            time.Time
	RoundEnd              time.Time
}

// Importer downloads Timeboost baseline data from timeboost.app and inserts it
// into raw.timeboost_bids_ext and raw.timeboost_auction_results_ext.
type Importer struct {
	cfg        Config
	bids       BatchAppender
	results    BatchAppender
	querier    Querier
	log        *slog.Logger
	limiter    *rate.Limiter
	httpClient *http.Client
}

// NewImporter creates a new Importer.
// bids is a BatchWriter for raw.timeboost_bids_ext.
// results is a BatchWriter for raw.timeboost_auction_results_ext.
// querier is used for cross-check queries against raw.logs.
func NewImporter(cfg Config, bids, results BatchAppender, querier Querier, log *slog.Logger) *Importer {
	rateLimit := cfg.RateLimit
	if rateLimit <= 0 {
		rateLimit = 60
	}
	// Convert req/min to req/s as a rate.Limit.
	limiter := rate.NewLimiter(rate.Limit(float64(rateLimit)/60.0), 1)
	return &Importer{
		cfg:        cfg,
		bids:       bids,
		results:    results,
		querier:    querier,
		log:        log,
		limiter:    limiter,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Run downloads and inserts all bids and results for each date in [From, To],
// then performs a cross-check against raw.logs.
func (imp *Importer) Run(ctx context.Context) error {
	for d := imp.cfg.From; !d.After(imp.cfg.To); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")

		// Fetch and insert bids.
		bidsURL := imp.cfg.BidsURL + dateStr
		bids, err := imp.fetchBids(ctx, bidsURL)
		if err != nil {
			imp.log.WarnContext(ctx, "fetch bids failed", "date", dateStr, "err", err)
		} else {
			if err := imp.insertBids(ctx, bids, dateStr); err != nil {
				return fmt.Errorf("insert bids %s: %w", dateStr, err)
			}
			imp.log.InfoContext(ctx, "inserted bids", "date", dateStr, "count", len(bids))
		}

		// Fetch and insert results.
		resultsURL := imp.cfg.ResultsURL + dateStr
		results, err := imp.fetchResults(ctx, resultsURL)
		if err != nil {
			imp.log.WarnContext(ctx, "fetch results failed", "date", dateStr, "err", err)
		} else {
			if err := imp.insertResults(ctx, results, dateStr); err != nil {
				return fmt.Errorf("insert results %s: %w", dateStr, err)
			}
			imp.log.InfoContext(ctx, "inserted results", "date", dateStr, "count", len(results))

			// Cross-check this day's results against raw.logs.
			if imp.querier != nil {
				imp.crossCheck(ctx, results)
			}
		}
	}

	if err := imp.bids.Flush(ctx); err != nil {
		imp.log.WarnContext(ctx, "final bids flush error", "err", err)
	}
	if err := imp.results.Flush(ctx); err != nil {
		imp.log.WarnContext(ctx, "final results flush error", "err", err)
	}
	return nil
}

// fetchBids downloads NDJSON bids from url and parses them.
func (imp *Importer) fetchBids(ctx context.Context, url string) ([]Bid, error) {
	body, contentType, err := imp.get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer body.Close() //nolint:errcheck

	if isParquet(contentType) {
		// Parquet support not yet implemented; log and skip.
		imp.log.WarnContext(ctx, "parquet response not yet supported, skipping", "url", url)
		return nil, nil
	}
	return parseNDJSONBids(body)
}

// fetchResults downloads NDJSON results from url and parses them.
func (imp *Importer) fetchResults(ctx context.Context, url string) ([]Result, error) {
	body, contentType, err := imp.get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer body.Close() //nolint:errcheck

	if isParquet(contentType) {
		imp.log.WarnContext(ctx, "parquet response not yet supported, skipping", "url", url)
		return nil, nil
	}
	return parseNDJSONResults(body)
}

// get performs a rate-limited HTTP GET and returns the response body.
func (imp *Importer) get(ctx context.Context, url string) (io.ReadCloser, string, error) {
	if err := imp.limiter.Wait(ctx); err != nil {
		return nil, "", fmt.Errorf("rate limiter: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build request: %w", err)
	}
	resp, err := imp.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("http get %s: %w", url, err)
	}
	if resp.StatusCode == http.StatusNotFound {
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("not found: %s", url)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("http %d: %s", resp.StatusCode, url)
	}
	return resp.Body, resp.Header.Get("Content-Type"), nil
}

// isParquet returns true if the Content-Type indicates a Parquet file.
func isParquet(ct string) bool {
	return strings.Contains(ct, "parquet") || strings.Contains(ct, "octet-stream")
}

// parseNDJSONBids parses newline-delimited JSON bids from r.
func parseNDJSONBids(r io.Reader) ([]Bid, error) {
	var out []Bid
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rb rawBid
		if err := json.Unmarshal([]byte(line), &rb); err != nil {
			return nil, fmt.Errorf("parse bid JSON: %w", err)
		}
		b, err := rawBidToBid(rb)
		if err != nil {
			return nil, fmt.Errorf("convert bid: %w", err)
		}
		out = append(out, b)
	}
	return out, scanner.Err()
}

// parseNDJSONResults parses newline-delimited JSON results from r.
func parseNDJSONResults(r io.Reader) ([]Result, error) {
	var out []Result
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rr rawResult
		if err := json.Unmarshal([]byte(line), &rr); err != nil {
			return nil, fmt.Errorf("parse result JSON: %w", err)
		}
		res, err := rawResultToResult(rr)
		if err != nil {
			return nil, fmt.Errorf("convert result: %w", err)
		}
		out = append(out, res)
	}
	return out, scanner.Err()
}

func rawBidToBid(rb rawBid) (Bid, error) {
	amount, ok := new(big.Int).SetString(rb.AmountWei, 10)
	if !ok {
		return Bid{}, fmt.Errorf("invalid amount_wei %q", rb.AmountWei)
	}
	ts, err := time.Parse(time.RFC3339, rb.BidTimestamp)
	if err != nil {
		// Try without nanoseconds.
		ts, err = time.Parse("2006-01-02T15:04:05Z", rb.BidTimestamp)
		if err != nil {
			return Bid{}, fmt.Errorf("parse bid_timestamp %q: %w", rb.BidTimestamp, err)
		}
	}
	return Bid{
		Round:                 rb.Round,
		Bidder:                strings.ToLower(rb.Bidder),
		ExpressLaneController: strings.ToLower(rb.ExpressLaneController),
		AmountWei:             amount,
		BidTimestamp:          ts.UTC(),
	}, nil
}

func rawResultToResult(rr rawResult) (Result, error) {
	fp, ok := new(big.Int).SetString(rr.FirstPriceWei, 10)
	if !ok {
		return Result{}, fmt.Errorf("invalid first_price_wei %q", rr.FirstPriceWei)
	}
	sp, ok := new(big.Int).SetString(rr.SecondPriceWei, 10)
	if !ok {
		return Result{}, fmt.Errorf("invalid second_price_wei %q", rr.SecondPriceWei)
	}
	start, err := time.Parse(time.RFC3339, rr.RoundStart)
	if err != nil {
		start, err = time.Parse("2006-01-02T15:04:05Z", rr.RoundStart)
		if err != nil {
			return Result{}, fmt.Errorf("parse round_start %q: %w", rr.RoundStart, err)
		}
	}
	end, err := time.Parse(time.RFC3339, rr.RoundEnd)
	if err != nil {
		end, err = time.Parse("2006-01-02T15:04:05Z", rr.RoundEnd)
		if err != nil {
			return Result{}, fmt.Errorf("parse round_end %q: %w", rr.RoundEnd, err)
		}
	}
	return Result{
		Round:                 rr.Round,
		Winner:                strings.ToLower(rr.Winner),
		ExpressLaneController: strings.ToLower(rr.ExpressLaneController),
		FirstPriceWei:         fp,
		SecondPriceWei:        sp,
		RoundStart:            start.UTC(),
		RoundEnd:              end.UTC(),
	}, nil
}

// insertBids appends bids to raw.timeboost_bids_ext via the BatchWriter.
func (imp *Importer) insertBids(ctx context.Context, bids []Bid, source string) error {
	now := time.Now().UTC()
	for _, b := range bids {
		if err := imp.bids.Append(ctx,
			b.Round,
			b.Bidder,
			b.ExpressLaneController,
			b.AmountWei.String(),
			b.BidTimestamp,
			source,
			now.UnixMilli(),
		); err != nil {
			return fmt.Errorf("append bid row: %w", err)
		}
	}
	return nil
}

// insertResults appends results to raw.timeboost_auction_results_ext via the BatchWriter.
func (imp *Importer) insertResults(ctx context.Context, results []Result, source string) error {
	now := time.Now().UTC()
	for _, r := range results {
		if err := imp.results.Append(ctx,
			r.Round,
			r.Winner,
			r.ExpressLaneController,
			r.FirstPriceWei.String(),
			r.SecondPriceWei.String(),
			r.RoundStart,
			r.RoundEnd,
			source,
			now.UnixMilli(),
		); err != nil {
			return fmt.Errorf("append result row: %w", err)
		}
	}
	return nil
}

// crossCheck queries raw.logs for AuctionResolved events and compares winner/price
// with the imported results, logging any mismatches.
// topic0 for AuctionResolved must be set once the contract ABI is known.
func (imp *Importer) crossCheck(ctx context.Context, results []Result) {
	if len(results) == 0 {
		return
	}
	// Placeholder topic0: replace with keccak256("AuctionResolved(uint64,address,address,uint256,uint256)")
	// once the ABI is confirmed on-chain.
	const topic0 = "0x0000000000000000000000000000000000000000000000000000000000000000"

	for _, res := range results {
		row := imp.querier.QueryRow(ctx,
			`SELECT topic1, data
			 FROM raw.logs FINAL
			 WHERE topic0 = ?
			   AND block_number BETWEEN ? AND ?
			 ORDER BY block_number ASC
			 LIMIT 1`,
			topic0,
			res.RoundStart.Unix()/12, // approximate block from timestamp
			res.RoundEnd.Unix()/12,
		)
		var topic1, data string
		if err := row.Scan(&topic1, &data); err != nil {
			// No matching log found — expected until the topic0 is populated.
			continue
		}
		// Compare winner address from log topic1 with imported winner.
		onChainWinner := strings.ToLower(topic1)
		if onChainWinner != res.Winner {
			imp.log.WarnContext(ctx, "cross-check mismatch: winner",
				"round", res.Round,
				"imported_winner", res.Winner,
				"onchain_winner", onChainWinner,
			)
		}
		_ = data // price cross-check requires ABI decode; deferred until ABI confirmed
	}
}
