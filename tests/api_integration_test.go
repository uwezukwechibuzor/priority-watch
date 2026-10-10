//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/uwezukwechibuzor/priority-watch/internal/api"
	"github.com/uwezukwechibuzor/priority-watch/internal/config"
)

// TestAPIIntegration starts a ClickHouse container, seeds mart tables, and
// verifies end-to-end HTTP responses from the API.
func TestAPIIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in short mode")
	}
	ctx := context.Background()

	// ── ClickHouse container ───────────────────────────────────────────────────
	req := testcontainers.ContainerRequest{
		Image:        "clickhouse/clickhouse-server:25.4",
		ExposedPorts: []string{"9000/tcp", "8123/tcp"},
		Env: map[string]string{
			"CLICKHOUSE_DB":       "default",
			"CLICKHOUSE_USER":     "default",
			"CLICKHOUSE_PASSWORD": "",
		},
		WaitingFor: wait.ForHTTP("/ping").WithPort("8123/tcp").WithStartupTimeout(90 * time.Second),
	}
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start clickhouse container: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(context.Background()) })

	nativePort, err := ctr.MappedPort(ctx, "9000/tcp")
	if err != nil {
		t.Fatalf("mapped native port: %v", err)
	}
	httpPort, err := ctr.MappedPort(ctx, "8123/tcp")
	if err != nil {
		t.Fatalf("mapped http port: %v", err)
	}

	chAddr := fmt.Sprintf("localhost:%s", nativePort.Port())
	chHTTPAddr := fmt.Sprintf("http://localhost:%s", httpPort.Port())

	// ── Open connection + seed data ────────────────────────────────────────────
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{chAddr},
		Auth: clickhouse.Auth{Username: "default"},
	})
	if err != nil {
		t.Fatalf("open clickhouse: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if err := seedMartTables(ctx, conn); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// ── Build API server ───────────────────────────────────────────────────────
	apiCfg := &config.Config{
		APIAddr:            ":0",
		APICORSOrigin:      "*",
		ClickHouseHTTPAddr: chHTTPAddr,
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	srv := api.New(apiCfg, conn, log)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	// ── Test cases ─────────────────────────────────────────────────────────────
	tests := []struct {
		name       string
		path       string
		wantStatus int
		checkJSON  func(t *testing.T, body map[string]any)
	}{
		{
			name:       "healthz",
			path:       "/healthz",
			wantStatus: 200,
			checkJSON: func(t *testing.T, body map[string]any) {
				t.Helper()
				if diff := cmp.Diff("ok", body["status"]); diff != "" {
					t.Errorf("healthz status (-want +got):\n%s", diff)
				}
			},
		},
		{
			name:       "openapi_json",
			path:       "/openapi.json",
			wantStatus: 200,
		},
		{
			name:       "blocks_list",
			path:       "/api/v1/blocks?limit=5",
			wantStatus: 200,
			checkJSON: func(t *testing.T, body map[string]any) {
				t.Helper()
				if _, ok := body["data"]; !ok {
					t.Error("response missing 'data' field")
				}
			},
		},
		{
			name:       "blocks_kpis",
			path:       "/api/v1/blocks/kpis?range=24h",
			wantStatus: 200,
			checkJSON: func(t *testing.T, body map[string]any) {
				t.Helper()
				for _, field := range []string{"block_count", "tip_revenue_wei", "hhi"} {
					if _, ok := body[field]; !ok {
						t.Errorf("KPI response missing field %q", field)
					}
				}
			},
		},
		{
			name:       "blocks_series",
			path:       "/api/v1/blocks/series?range=24h",
			wantStatus: 200,
		},
		{
			name:       "invalid_range",
			path:       "/api/v1/blocks/kpis?range=bad",
			wantStatus: 422,
		},
		{
			name:       "limit_too_large",
			path:       "/api/v1/blocks?limit=500",
			wantStatus: 422,
		},
		{
			name:       "dataset_unknown",
			path:       "/api/v1/datasets/parquet/nonexistent?from=2026-01-01&to=2026-01-02",
			wantStatus: 404,
		},
		{
			name:       "dataset_missing_dates",
			path:       "/api/v1/datasets/parquet/block_tips",
			wantStatus: 400,
		},
		{
			name:       "dataset_window_too_wide",
			path:       "/api/v1/datasets/parquet/block_tips?from=2026-01-01&to=2026-04-01",
			wantStatus: 400,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp, err := http.Get(ts.URL + tc.path)
			if err != nil {
				t.Fatalf("GET %s: %v", tc.path, err)
			}
			defer resp.Body.Close()

			if diff := cmp.Diff(tc.wantStatus, resp.StatusCode); diff != "" {
				t.Errorf("status for %s (-want +got):\n%s", tc.path, diff)
			}

			if tc.checkJSON != nil {
				var body map[string]any
				if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatalf("decode JSON for %s: %v", tc.path, err)
				}
				tc.checkJSON(t, body)
			}
		})
	}
}

// seedMartTables creates the mart tables the API queries and inserts one row each.
func seedMartTables(ctx context.Context, conn clickhouse.Conn) error {
	ddl := []string{
		`CREATE DATABASE IF NOT EXISTS marts`,
		`CREATE DATABASE IF NOT EXISTS meta`,
		`CREATE TABLE IF NOT EXISTS meta.source_status (
			source    String,
			status    String,
			detail    String,
			checked_at DateTime64(3,'UTC'),
			_version  UInt64
		) ENGINE = ReplacingMergeTree(_version) ORDER BY source`,
		`CREATE TABLE IF NOT EXISTS marts.fct_blocks (
			block_number           UInt64,
			block_timestamp        DateTime64(3,'UTC'),
			era                    String,
			tx_count               UInt64,
			tipped_tx_count        UInt64,
			tip_revenue_wei        UInt256,
			tip_revenue_eth        Float64,
			tip_revenue_usd        Float64,
			eth_price_usd          Float64,
			top_fee_gwei           Float64,
			top_tip_usd            Float64,
			median_fee_gwei        Float64,
			top_payer_address      String,
			top_payer_name         String,
			top_payer_color_index  UInt8,
			revert_count           UInt64,
			second_to_first_ratio  Float64,
			block_type             String,
			ordering_score         Float64,
			distinct_tippers       UInt32,
			base_fee_per_gas       UInt256,
			base_fee_per_gas_gwei  Float64,
			gas_used               UInt64,
			gas_limit              UInt64,
			gas_used_pct           Float64,
			hhi                    Float64,
			_version               UInt64
		) ENGINE = ReplacingMergeTree(_version) ORDER BY block_number`,
		`CREATE TABLE IF NOT EXISTS marts.agg_blocks_hourly (
			block_timestamp  DateTime64(3,'UTC'),
			block_count      UInt64,
			tip_revenue_wei  UInt256,
			tip_revenue_eth  Float64,
			tip_revenue_usd  Float64,
			block_type       String,
			top_payer_name   String,
			tipped_tx_count  UInt64,
			hhi              Float64,
			top_fee_gwei     Float64,
			_version         UInt64
		) ENGINE = ReplacingMergeTree(_version) ORDER BY block_timestamp`,
		`CREATE TABLE IF NOT EXISTS marts.agg_blocks_minutely (
			bucket_time     DateTime64(3,'UTC'),
			tip_revenue_eth Float64,
			top_fee_gwei    Float64,
			_version        UInt64
		) ENGINE = ReplacingMergeTree(_version) ORDER BY bucket_time`,
		`CREATE TABLE IF NOT EXISTS marts.fct_treasury_daily (
			date              Date,
			tip_revenue_wei   UInt256,
			tip_revenue_eth   Float64,
			tip_revenue_usd   Float64,
			treasury_wei      UInt256,
			treasury_eth      Float64,
			treasury_usd      Float64,
			guild_wei         UInt256,
			guild_eth         Float64,
			guild_usd         Float64,
			_version          UInt64
		) ENGINE = ReplacingMergeTree(_version) ORDER BY date`,
		`CREATE TABLE IF NOT EXISTS marts.dim_events (
			event_date    Date,
			event_type    String,
			title         String,
			description   String,
			_version      UInt64
		) ENGINE = ReplacingMergeTree(_version) ORDER BY (event_date, event_type)`,
	}

	for _, q := range ddl {
		if err := conn.Exec(ctx, q); err != nil {
			return fmt.Errorf("DDL %q: %w", q[:min(50, len(q))], err)
		}
	}

	now := time.Now().UTC()
	if err := conn.Exec(ctx, `INSERT INTO marts.fct_blocks VALUES (
		1000000, ?, 'pga', 150, 50, 1000000000000000000, 0.001, 2.50, 2500.0,
		1.5, 3.75, 0.8, '0xabc', 'Searcher A', 0, 2, 0.9, 'contested', 0.95,
		5, 100000000, 0.0000001, 8000000, 10000000, 0.8, 2500.0, 1
	)`, now); err != nil {
		return fmt.Errorf("insert fct_blocks: %w", err)
	}

	if err := conn.Exec(ctx, `INSERT INTO marts.agg_blocks_hourly VALUES (
		?, 1, 1000000000000000000, 0.001, 2.50, 'contested', 'Searcher A', 50, 2500.0, 1.5, 1
	)`, now); err != nil {
		return fmt.Errorf("insert agg_blocks_hourly: %w", err)
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
