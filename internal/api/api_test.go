package api

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/uwezukwechibuzor/priority-watch/internal/config"
)

var update = flag.Bool("update", false, "update golden files")

// newTestServer creates a Server with a nil ClickHouse connection.
// Store methods return errors when conn is nil; routing and validation tests pass.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{
		APIAddr:            ":0",
		APICORSOrigin:      "*",
		ClickHouseHTTPAddr: "http://localhost:8123",
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return New(cfg, nil, log)
}

// ── cursor tests ──────────────────────────────────────────────────────────────

func TestCursorRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		block   uint64
		txIndex *uint32
	}{
		{name: "block_only", block: 1234567, txIndex: nil},
		{name: "block_and_tx", block: 9999999, txIndex: ptr32(42)},
		{name: "zero_block", block: 0, txIndex: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			enc := encodeCursor(tc.block, tc.txIndex)
			gotBlock, gotTx, err := decodeCursor(enc)
			if err != nil {
				t.Fatalf("decodeCursor(%q) error: %v", enc, err)
			}
			if diff := cmp.Diff(tc.block, gotBlock); diff != "" {
				t.Errorf("block mismatch (-want +got):\n%s", diff)
			}
			if tc.txIndex == nil && gotTx != nil {
				t.Errorf("want nil txIndex, got %v", *gotTx)
			}
			if tc.txIndex != nil {
				if gotTx == nil {
					t.Fatalf("want txIndex %d, got nil", *tc.txIndex)
				}
				if diff := cmp.Diff(*tc.txIndex, *gotTx); diff != "" {
					t.Errorf("txIndex mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestDecodeCursorInvalid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		cursor string
	}{
		{name: "empty", cursor: ""},
		{name: "not_base64", cursor: "not!base64!!!"},
		{name: "bad_json", cursor: "bm90anNvbg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := decodeCursor(tc.cursor)
			if err == nil {
				t.Errorf("decodeCursor(%q) want error, got nil", tc.cursor)
			}
		})
	}
}

// ── rangeCutoff tests ─────────────────────────────────────────────────────────

func TestRangeCutoff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		minAge  time.Duration // cutoff must be at least this old
	}{
		{name: "1h", input: "1h", minAge: 59 * time.Minute},
		{name: "24h", input: "24h", minAge: 23 * time.Hour},
		{name: "7d", input: "7d", minAge: 6 * 24 * time.Hour},
		{name: "30d", input: "30d", minAge: 29 * 24 * time.Hour},
		{name: "all", input: "all", minAge: 365 * 24 * time.Hour},
		{name: "invalid_5m", input: "5m", wantErr: true},
		{name: "invalid_empty", input: "", wantErr: true},
		{name: "invalid_1d", input: "1d", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := rangeCutoff(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("rangeCutoff(%q) error=%v, wantErr=%v", tc.input, err, tc.wantErr)
				return
			}
			if !tc.wantErr {
				age := time.Since(got)
				if age < tc.minAge {
					t.Errorf("cutoff for %q: age=%v, want >= %v", tc.input, age, tc.minAge)
				}
			}
		})
	}
}

// ── cache middleware tests ─────────────────────────────────────────────────────

func TestCacheMiddleware(t *testing.T) {
	// NOT Parallel: the sub-tests share a cache and must run sequentially
	// so the cache state from test N is visible to test N+1.
	srv := newTestServer(t)

	const body = `{"hello":"world"}`
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	handler := srv.cacheMiddleware(inner)
	etag := computeETag([]byte(body))

	tests := []struct {
		name        string
		ifNoneMatch string
		wantStatus  int
		wantXCache  string
		wantBody    string
	}{
		{name: "first_request_miss", wantStatus: 200, wantXCache: "MISS", wantBody: body},
		{name: "second_request_hit", wantStatus: 200, wantXCache: "HIT", wantBody: body},
		{name: "conditional_304", ifNoneMatch: etag, wantStatus: 304, wantBody: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test?q=cache", nil)
			if tc.ifNoneMatch != "" {
				req.Header.Set("If-None-Match", tc.ifNoneMatch)
			}
			rw := httptest.NewRecorder()
			handler.ServeHTTP(rw, req)

			if diff := cmp.Diff(tc.wantStatus, rw.Code); diff != "" {
				t.Errorf("status mismatch (-want +got):\n%s", diff)
			}
			if tc.wantXCache != "" {
				if diff := cmp.Diff(tc.wantXCache, rw.Header().Get("X-Cache")); diff != "" {
					t.Errorf("X-Cache mismatch (-want +got):\n%s", diff)
				}
			}
			if tc.wantBody != "" {
				if diff := cmp.Diff(tc.wantBody, rw.Body.String()); diff != "" {
					t.Errorf("body mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

// ── infra endpoint tests ──────────────────────────────────────────────────────

func TestInfraEndpoints(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{name: "healthz", method: http.MethodGet, path: "/healthz", wantStatus: 200, wantBody: `{"status":"ok"}`},
		{name: "docs", method: http.MethodGet, path: "/docs", wantStatus: 200},
		{name: "openapi_json", method: http.MethodGet, path: "/openapi.json", wantStatus: 200},
		{name: "metrics", method: http.MethodGet, path: "/metrics", wantStatus: 200},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(context.Background(), tc.method, tc.path, nil)
			rw := httptest.NewRecorder()
			srv.router.ServeHTTP(rw, req)

			if diff := cmp.Diff(tc.wantStatus, rw.Code); diff != "" {
				t.Errorf("status mismatch for %s (-want +got):\n%s", tc.path, diff)
			}
			if tc.wantBody != "" {
				got := strings.TrimSpace(rw.Body.String())
				if diff := cmp.Diff(tc.wantBody, got); diff != "" {
					t.Errorf("body mismatch for %s (-want +got):\n%s", tc.path, diff)
				}
			}
		})
	}
}

// ── OpenAPI golden snapshot ───────────────────────────────────────────────────

func TestOpenAPIGolden(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/openapi.json", nil)
	rw := httptest.NewRecorder()
	srv.router.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("GET /openapi.json status=%d", rw.Code)
	}

	var raw map[string]any
	if err := json.Unmarshal(rw.Body.Bytes(), &raw); err != nil {
		t.Fatalf("invalid OpenAPI JSON: %v", err)
	}
	pretty, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("marshal indent: %v", err)
	}

	golden := filepath.Join("testdata", "openapi.json")
	if *update {
		if err := os.MkdirAll("testdata", 0o750); err != nil { //nolint:gosec
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(golden, pretty, 0o600); err != nil { //nolint:gosec
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated %s", golden)
		return
	}

	want, err := os.ReadFile(golden) //nolint:gosec
	if err != nil {
		t.Skipf("golden file %s missing; run with -update to create it", golden)
	}
	if diff := cmp.Diff(string(want), string(pretty)); diff != "" {
		t.Errorf("openapi.json changed (-want +got):\n%s\nRun with -update to accept.", diff)
	}
}

// ── query-parameter validation ────────────────────────────────────────────────

func TestQueryParamValidation(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		// huma validates enum before handler runs → 422
		{name: "invalid_range_5m", path: "/api/v1/blocks/kpis?range=5m", wantStatus: 422},
		{name: "invalid_range_1d", path: "/api/v1/blocks/series?range=1d", wantStatus: 422},
		// huma validates minimum/maximum → 422
		{name: "limit_too_large", path: "/api/v1/blocks?limit=999", wantStatus: 422},
		{name: "limit_zero", path: "/api/v1/blocks?limit=0", wantStatus: 422},
		// search q too short → 400 from our code or 422 from huma minLength
		{name: "search_too_short", path: "/api/v1/search?q=a", wantStatus: 422},
		// compare eras window_days=0 → 422
		{name: "window_days_zero", path: "/api/v1/compare/eras?window_days=0", wantStatus: 422},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tc.path, nil)
			rw := httptest.NewRecorder()
			srv.router.ServeHTTP(rw, req)
			if diff := cmp.Diff(tc.wantStatus, rw.Code); diff != "" {
				t.Errorf("status for %s (-want +got):\n%s", tc.path, diff)
			}
		})
	}
}

func TestDatasetValidation(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "missing_from_to", path: "/api/v1/datasets/parquet/block_tips", wantStatus: 400},
		{name: "unknown_table", path: "/api/v1/datasets/parquet/raw_blocks?from=2026-01-01&to=2026-01-02", wantStatus: 404},
		{name: "window_too_wide", path: "/api/v1/datasets/parquet/block_tips?from=2026-01-01&to=2026-04-01", wantStatus: 400},
		{name: "to_before_from", path: "/api/v1/datasets/parquet/block_tips?from=2026-01-10&to=2026-01-01", wantStatus: 400},
		{name: "bad_from_format", path: "/api/v1/datasets/parquet/block_tips?from=01-01-2026&to=2026-01-02", wantStatus: 400},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tc.path, nil)
			rw := httptest.NewRecorder()
			srv.router.ServeHTTP(rw, req)
			if diff := cmp.Diff(tc.wantStatus, rw.Code); diff != "" {
				t.Errorf("status for %s (-want +got):\n%s", tc.path, diff)
			}
		})
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func ptr32(v uint32) *uint32 { return &v }
