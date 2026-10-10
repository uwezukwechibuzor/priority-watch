//go:build integration

package store_test

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/uwezukwechibuzor/priority-watch/internal/store"
)

// TestBatchWriterUInt256RoundTrip verifies that *big.Int values survive a
// round-trip through ClickHouse UInt256 columns via the batch writer.
func TestBatchWriterUInt256RoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "clickhouse/clickhouse-server:25.3",
		ExposedPorts: []string{"9000/tcp", "8123/tcp"},
		Env:          map[string]string{"CLICKHOUSE_PASSWORD": "test"},
		WaitingFor: wait.ForHTTP("/ping").
			WithPort("8123/tcp").
			WithStartupTimeout(90 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "9000")

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
			t.Fatal("ClickHouse did not become ready")
		}
	}

	// Create a minimal table with a UInt256 column.
	if err := conn.Exec(ctx, `
		CREATE DATABASE IF NOT EXISTS test_batch
	`); err != nil {
		t.Fatalf("create db: %v", err)
	}
	if err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_batch.uint256_test (
			id     UInt64,
			value  UInt256
		) ENGINE = MergeTree()
		ORDER BY id
	`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	tests := []struct {
		name  string
		id    uint64
		value *big.Int
	}{
		{
			name:  "zero value",
			id:    1,
			value: big.NewInt(0),
		},
		{
			name:  "one ETH in wei",
			id:    2,
			value: new(big.Int).Mul(big.NewInt(1e18), big.NewInt(1)),
		},
		{
			name:  "max uint256",
			id:    3,
			value: new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)),
		},
		{
			name:  "large tip",
			id:    4,
			value: new(big.Int).SetUint64(999_999_999_999_999_999),
		},
	}

	// Write all rows via BatchWriter.
	w := store.NewBatchWriter(conn, "INSERT INTO test_batch.uint256_test", 100, 0, nil)
	for _, tc := range tests {
		if err := w.Append(ctx, tc.id, tc.value); err != nil {
			t.Fatalf("Append %s: %v", tc.name, err)
		}
	}
	if err := w.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// Read back and verify.
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Note: NOT t.Parallel() — shares the same container.
			var id uint64
			var value big.Int

			row := conn.QueryRow(ctx,
				"SELECT id, value FROM test_batch.uint256_test WHERE id = ?",
				tc.id,
			)
			if err := row.Scan(&id, &value); err != nil {
				t.Fatalf("scan: %v", err)
			}
			if value.Cmp(tc.value) != 0 {
				t.Errorf("value = %s, want %s", &value, tc.value)
			}
		})
	}
}
