package cex

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/websocket"
)

// ── CSV parsing ──────────────────────────────────────────────────────────────

func TestParseCSV(t *testing.T) {
	t.Parallel()

	ms0 := time.UnixMilli(1640000000000).UTC()
	ms1 := time.UnixMilli(1640000001000).UTC()

	tests := []struct {
		name    string
		input   string
		symbol  string
		want    []Kline
		wantErr error
	}{
		{
			name:   "ms_timestamps",
			symbol: "ETHUSDT",
			input: "1640000000000,1000.0,1001.0,999.0,1000.5,10.5,1640000000999,10500.0,100,5.0,5000.0,0\n" +
				"1640000001000,1000.5,1002.0,999.5,1001.0,11.0,1640000001999,11000.0,110,5.5,5500.0,0\n",
			want: []Kline{
				{Venue: "binance", Symbol: "ETHUSDT", OpenTime: ms0, Open: 1000.0, High: 1001.0, Low: 999.0, Close: 1000.5, Volume: 10.5, Trades: 100},
				{Venue: "binance", Symbol: "ETHUSDT", OpenTime: ms1, Open: 1000.5, High: 1002.0, Low: 999.5, Close: 1001.0, Volume: 11.0, Trades: 110},
			},
		},
		{
			name:   "us_timestamps",
			symbol: "BTCUSDT",
			// 16-digit open_time → μs → divide by 1000 → 1640000000000 ms
			input: "1640000000000000,50000.0,50100.0,49900.0,50050.0,0.5,1640000000999000,25025.0,50,0.25,12500.0,0\n",
			want: []Kline{
				{Venue: "binance", Symbol: "BTCUSDT", OpenTime: ms0, Open: 50000.0, High: 50100.0, Low: 49900.0, Close: 50050.0, Volume: 0.5, Trades: 50},
			},
		},
		{
			name:   "header_row_skipped",
			symbol: "ETHUSDT",
			input: "open_time,open,high,low,close,volume,close_time,quote_vol,trades,tb_base,tb_quote,ignore\n" +
				"1640000000000,1000.0,1001.0,999.0,1000.5,10.5,1640000000999,10500.0,100,5.0,5000.0,0\n",
			want: []Kline{
				{Venue: "binance", Symbol: "ETHUSDT", OpenTime: ms0, Open: 1000.0, High: 1001.0, Low: 999.0, Close: 1000.5, Volume: 10.5, Trades: 100},
			},
		},
		{
			name:   "too_few_columns_skipped",
			symbol: "ETHUSDT",
			input:  "1640000000000,1000.0,1001.0\n",
			want:   nil,
		},
		{
			name:   "malformed_float_skipped",
			symbol: "ETHUSDT",
			input:  "1640000000000,notanumber,1001.0,999.0,1000.5,10.5,1640000000999,10500.0,100,5.0,5000.0,0\n",
			want:   nil,
		},
		{
			name:   "empty_input",
			symbol: "ETHUSDT",
			input:  "",
			want:   nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseCSV(strings.NewReader(tc.input), tc.symbol)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("error: want %v, got %v", tc.wantErr, err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// ── Timestamp unit detection ─────────────────────────────────────────────────

func TestDetectTSUnit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want int64
	}{
		{name: "13_digits_ms", raw: "1640000000000", want: 1},
		{name: "16_digits_us", raw: "1640000000000000", want: 1000},
		{name: "short", raw: "123456", want: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := detectTSUnit(tc.raw)
			if got != tc.want {
				t.Errorf("detectTSUnit(%q) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}

// ── Checksum verification ─────────────────────────────────────────────────────

func TestVerifyChecksum(t *testing.T) {
	t.Parallel()

	data := []byte("hello world")
	goodHash := fmt.Sprintf("%x", sha256.Sum256(data))

	tests := []struct {
		name     string
		data     []byte
		expected string
		wantErr  bool
	}{
		{name: "match", data: data, expected: goodHash, wantErr: false},
		{name: "match_uppercase", data: data, expected: strings.ToUpper(goodHash), wantErr: false},
		{name: "mismatch", data: data, expected: "deadbeef", wantErr: true},
		{name: "empty_expected", data: data, expected: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := verifyChecksum(tc.data, tc.expected)
			if (err != nil) != tc.wantErr {
				t.Errorf("verifyChecksum() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// ── fetchDay via httptest ─────────────────────────────────────────────────────

func TestFetchDay(t *testing.T) {
	t.Parallel()

	csvContent := "1640000000000,1000.0,1001.0,999.0,1000.5,10.5,1640000000999,10500.0,100,5.0,5000.0,0\n"
	zipData := makeTestZip(t, "ETHUSDT-1s-2022-12-20.csv", csvContent)
	goodHash := fmt.Sprintf("%x  ETHUSDT-1s-2022-12-20.zip\n", sha256.Sum256(zipData))
	badHash := "deadbeef00000000000000000000000000000000000000000000000000000000  ETHUSDT-1s-2022-12-20.zip\n"

	tests := []struct {
		name      string
		checksum  string
		wantErr   bool
		wantCount int
	}{
		{name: "valid", checksum: goodHash, wantErr: false, wantCount: 1},
		{name: "checksum_mismatch", checksum: badHash, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, ".CHECKSUM") {
					_, _ = w.Write([]byte(tc.checksum))
					return
				}
				_, _ = w.Write(zipData)
			}))
			defer srv.Close()

			src := &BinanceSource{client: srv.Client(), baseURL: srv.URL, log: noopLogger()}
			day := time.Date(2022, 12, 20, 0, 0, 0, 0, time.UTC)

			klines, err := src.fetchDay(context.Background(), "ETHUSDT", day)
			if (err != nil) != tc.wantErr {
				t.Fatalf("fetchDay() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && len(klines) != tc.wantCount {
				t.Errorf("got %d klines, want %d", len(klines), tc.wantCount)
			}
		})
	}
}

// ── WebSocket message parsing ────────────────────────────────────────────────

func TestKlineFromWS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  Kline
	}{
		{
			name: "finalized_kline",
			input: `{
				"stream":"ethusdt@kline_1s",
				"data":{
					"e":"kline","s":"ETHUSDT",
					"k":{"t":1640000000000,"o":"1000.0","h":"1001.0","l":"999.0","c":"1000.5","v":"10.5","n":100,"x":true}
				}
			}`,
			want: Kline{
				Venue:    "binance",
				Symbol:   "ETHUSDT",
				OpenTime: time.UnixMilli(1640000000000).UTC(),
				Open:     1000.0,
				High:     1001.0,
				Low:      999.0,
				Close:    1000.5,
				Volume:   10.5,
				Trades:   100,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var km wsKlineMsg
			if err := json.Unmarshal([]byte(tc.input), &km); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			got := klineFromWS(km)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// ── WebSocket reconnect ───────────────────────────────────────────────────────

func TestWSReconnect(t *testing.T) {
	t.Parallel()

	var attempt atomic.Int32

	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		n := attempt.Add(1)
		if n == 1 {
			// First attempt: close immediately to trigger reconnect.
			return
		}
		// Second attempt: send one finalized kline then block until disconnected.
		msg := `{"stream":"ethusdt@kline_1s","data":{"e":"kline","s":"ETHUSDT","k":{"t":1640000000000,"o":"1000.0","h":"1001.0","l":"999.0","c":"1000.5","v":"10.5","n":42,"x":true}}}`
		_ = conn.WriteMessage(websocket.TextMessage, []byte(msg)) //nolint:errcheck
		// Keep the connection open until the client disconnects.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	src := &BinanceSource{
		log: noopLogger(),
		ws: func(ctx context.Context, _ string) (*websocket.Conn, error) {
			conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil) //nolint:bodyclose
			return conn, err
		},
		newBO: func() backoff.BackOff { return &backoff.ZeroBackOff{} },
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	received := make(chan Kline, 1)
	go func() {
		_ = src.Follow(ctx, []string{"ETHUSDT"}, func(k Kline) error {
			received <- k
			cancel() // cancel after first kline
			return nil
		})
	}()

	select {
	case k := <-received:
		if k.Symbol != "ETHUSDT" || k.Trades != 42 {
			t.Errorf("unexpected kline: %+v", k)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for kline via reconnect")
	}

	if n := attempt.Load(); n < 2 {
		t.Errorf("expected at least 2 connection attempts, got %d", n)
	}
}

// ── Fuzz ─────────────────────────────────────────────────────────────────────

func FuzzParseCSV(f *testing.F) {
	// Seed corpus
	f.Add([]byte("1640000000000,1000.0,1001.0,999.0,1000.5,10.5,1640000000999,10500.0,100,5.0,5000.0,0\n"))
	f.Add([]byte("1640000000000000,50000.0,50100.0,49900.0,50050.0,0.5,1640000000999000,25025.0,50,0.25,12500.0,0\n"))
	f.Add([]byte("bad,data\n"))
	f.Add([]byte(""))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic.
		_, _ = parseCSV(bytes.NewReader(data), "ETHUSDT")
	})
}

// ── helpers ───────────────────────────────────────────────────────────────────

// makeTestZip creates an in-memory zip archive containing one CSV file.
func makeTestZip(t *testing.T, filename, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, err := zw.Create(filename)
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
