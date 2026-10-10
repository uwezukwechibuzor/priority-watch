package cex

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	binanceName    = "binance"
	binanceDataURL = "https://data.binance.vision/data/spot/daily/klines"
)

// BinanceSource implements PriceSource for Binance.
// Use NewBinanceSource to construct; unexported fields are injectable for tests.
type BinanceSource struct {
	client  *http.Client
	log     *slog.Logger
	baseURL string    // defaults to binanceDataURL; overridden in tests
	ws      wsDial    // nil → websocket.DefaultDialer
	newBO   boFactory // nil → default exponential backoff
}

// NewBinanceSource creates a BinanceSource.
// client and log may be nil (http.DefaultClient / slog.Default() are used).
func NewBinanceSource(client *http.Client, log *slog.Logger) *BinanceSource {
	if client == nil {
		client = http.DefaultClient
	}
	if log == nil {
		log = slog.Default()
	}
	return &BinanceSource{client: client, log: log}
}

// Name returns "binance".
func (b *BinanceSource) Name() string { return binanceName }

// Backfill downloads historical 1s klines from data.binance.vision for each
// calendar day in [from, to] (UTC, inclusive), verifies the SHA-256 checksum,
// parses the CSV and calls fn once per day.
func (b *BinanceSource) Backfill(
	ctx context.Context,
	symbol string,
	from, to time.Time,
	fn func([]Kline) error,
) error {
	from = from.UTC().Truncate(24 * time.Hour)
	to = to.UTC().Truncate(24 * time.Hour)

	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		klines, err := b.fetchDay(ctx, symbol, d)
		if err != nil {
			return fmt.Errorf("binance backfill %s %s: %w", symbol, d.Format("2006-01-02"), err)
		}
		if len(klines) == 0 {
			continue
		}
		if err := fn(klines); err != nil {
			return err
		}
	}
	return nil
}

func (b *BinanceSource) fetchDay(ctx context.Context, symbol string, day time.Time) ([]Kline, error) {
	date := day.Format("2006-01-02")
	base := b.baseURL
	if base == "" {
		base = binanceDataURL
	}
	prefix := fmt.Sprintf("%s/%s/1s/%s-1s-%s", base, symbol, symbol, date)
	zipURL := prefix + ".zip"
	cksumURL := prefix + ".zip.CHECKSUM"

	expected, err := b.fetchChecksum(ctx, cksumURL)
	if err != nil {
		return nil, fmt.Errorf("fetch checksum: %w", err)
	}

	zipData, err := b.fetchBytes(ctx, zipURL)
	if err != nil {
		return nil, fmt.Errorf("fetch zip: %w", err)
	}

	if err := verifyChecksum(zipData, expected); err != nil {
		return nil, err
	}

	return parseZipCSV(zipData, symbol)
}

func (b *BinanceSource) fetchChecksum(ctx context.Context, url string) (string, error) {
	data, err := b.fetchBytes(ctx, url)
	if err != nil {
		return "", err
	}
	// Format: "<sha256hex>  <filename>\n"
	parts := strings.Fields(string(data))
	if len(parts) == 0 {
		return "", fmt.Errorf("empty checksum file at %s", url)
	}
	return strings.ToLower(parts[0]), nil
}

func (b *BinanceSource) fetchBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

// verifyChecksum returns an error if sha256(data) != expected (hex).
func verifyChecksum(data []byte, expected string) error {
	actual := fmt.Sprintf("%x", sha256.Sum256(data))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch: got %s want %s", actual, expected)
	}
	return nil
}

// parseZipCSV extracts the first .csv file from a zip archive and parses it.
func parseZipCSV(zipData []byte, symbol string) ([]Kline, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".csv") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open csv in zip: %w", err)
		}
		defer func() { _ = rc.Close() }()
		return parseCSV(rc, symbol)
	}
	return nil, fmt.Errorf("no CSV found in zip for symbol %s", symbol)
}

// parseCSV parses a Binance 1s kline CSV.
// Column order: open_time, open, high, low, close, volume, close_time,
// quote_asset_volume, number_of_trades, taker_buy_base, taker_buy_quote, ignore.
// Rows with non-numeric open_time (e.g. headers) or too few columns are skipped.
func parseCSV(r io.Reader, symbol string) ([]Kline, error) {
	cr := csv.NewReader(r)
	cr.ReuseRecord = true

	var klines []Kline
	var tsUnit int64 // 1 = already ms; 1000 = μs, divide to get ms

	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv read: %w", err)
		}
		if len(rec) < 9 {
			continue
		}

		rawTS := strings.TrimSpace(rec[0])
		tsVal, err := strconv.ParseInt(rawTS, 10, 64)
		if err != nil {
			continue // header row or malformed
		}

		if tsUnit == 0 {
			tsUnit = detectTSUnit(rawTS)
		}

		openTime := time.UnixMilli(tsVal / tsUnit).UTC()

		open, err := strconv.ParseFloat(rec[1], 64)
		if err != nil {
			continue
		}
		high, err := strconv.ParseFloat(rec[2], 64)
		if err != nil {
			continue
		}
		low, err := strconv.ParseFloat(rec[3], 64)
		if err != nil {
			continue
		}
		closeV, err := strconv.ParseFloat(rec[4], 64)
		if err != nil {
			continue
		}
		vol, err := strconv.ParseFloat(rec[5], 64)
		if err != nil {
			continue
		}
		trades, err := strconv.ParseUint(rec[8], 10, 32)
		if err != nil {
			continue
		}

		klines = append(klines, Kline{
			Venue:    binanceName,
			Symbol:   symbol,
			OpenTime: openTime,
			Open:     open,
			High:     high,
			Low:      low,
			Close:    closeV,
			Volume:   vol,
			Trades:   uint32(trades), //nolint:gosec
		})
	}
	return klines, nil
}

// detectTSUnit returns 1000 for μs (≥16-digit) timestamps, 1 for ms (13-digit).
// Dividing by the unit converts either format to milliseconds.
func detectTSUnit(raw string) int64 {
	if len(raw) >= 16 {
		return 1000
	}
	return 1
}
