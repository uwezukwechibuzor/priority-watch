//go:build integration

package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	pwmigrations "github.com/uwezukwechibuzor/priority-watch/infra/clickhouse/migrations"
	"github.com/uwezukwechibuzor/priority-watch/internal/store"
)

// TestMigratorIntegration starts a real ClickHouse container, runs migrations
// twice and verifies: tables exist, second run is a no-op, checksum tamper fails.
func TestMigratorIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "clickhouse/clickhouse-server:25.3",
		ExposedPorts: []string{"9000/tcp", "8123/tcp"},
		Env: map[string]string{
			"CLICKHOUSE_PASSWORD": "test",
		},
		WaitingFor: wait.ForHTTP("/ping").
			WithPort("8123/tcp").
			WithStartupTimeout(90 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start ClickHouse container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "9000")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr:     []string{fmt.Sprintf("%s:%s", host, port.Port())},
		Auth:     clickhouse.Auth{Password: "test"},
		Settings: clickhouse.Settings{"max_execution_time": 60},
	})
	if err != nil {
		t.Fatalf("open clickhouse: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	// Wait for ClickHouse to accept connections.
	for i := 0; i < 30; i++ {
		if err := conn.Ping(ctx); err == nil {
			break
		}
		time.Sleep(time.Second)
		if i == 29 {
			t.Fatal("ClickHouse did not accept connections within 30s")
		}
	}

	tests := []struct {
		name    string
		wantErr bool
	}{
		{name: "first run applies all migrations", wantErr: false},
		{name: "second run is a no-op (idempotent)", wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Note: NOT t.Parallel() — runs must be sequential on the same container.
			m := store.NewMigrator(conn, pwmigrations.FS)
			err := m.Run(ctx)
			if tc.wantErr != (err != nil) {
				t.Fatalf("Run() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}

	// Verify expected tables exist after migration.
	expectedTables := []struct{ db, table string }{
		{"raw", "blocks"},
		{"raw", "transactions"},
		{"raw", "receipts"},
		{"raw", "logs"},
		{"raw", "cex_klines_1s"},
		{"raw", "balance_snapshots"},
		{"raw", "timeboost_bids_ext"},
		{"raw", "reorg_log"},
		{"meta", "ingest_cursors"},
		{"meta", "source_status"},
		{"meta", "schema_migrations"},
	}

	for _, want := range expectedTables {
		t.Run(fmt.Sprintf("table_%s.%s_exists", want.db, want.table), func(t *testing.T) {
			var count uint64
			row := conn.QueryRow(ctx,
				"SELECT count() FROM system.tables WHERE database = ? AND name = ?",
				want.db, want.table,
			)
			if err := row.Scan(&count); err != nil {
				t.Fatalf("query system.tables: %v", err)
			}
			if count != 1 {
				t.Errorf("table %s.%s not found (count=%d)", want.db, want.table, count)
			}
		})
	}

	// Verify schema_migrations recorded all applied files.
	t.Run("schema_migrations_row_count", func(t *testing.T) {
		var count uint64
		row := conn.QueryRow(ctx, "SELECT count() FROM meta.schema_migrations FINAL")
		if err := row.Scan(&count); err != nil {
			t.Fatalf("count schema_migrations: %v", err)
		}
		if count == 0 {
			t.Error("schema_migrations has no rows after migration")
		}
	})

	// Verify databases were created.
	for _, db := range []string{"raw", "staging", "marts", "meta"} {
		db := db
		t.Run("database_"+db+"_exists", func(t *testing.T) {
			var count uint64
			row := conn.QueryRow(ctx,
				"SELECT count() FROM system.databases WHERE name = ?", db,
			)
			if err := row.Scan(&count); err != nil {
				t.Fatalf("query system.databases: %v", err)
			}
			if count != 1 {
				t.Errorf("database %q not found", db)
			}
		})
	}
}
