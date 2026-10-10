//go:build integration

package ingest_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
	"github.com/uwezukwechibuzor/priority-watch/internal/ingest"
	pwstore "github.com/uwezukwechibuzor/priority-watch/internal/store"
)

// stubChainClient is a fake ChainClient that returns deterministic empty blocks.
type stubChainClient struct {
	mu    sync.Mutex
	calls int
}

func (s *stubChainClient) Head(ctx context.Context) (*chain.RPCBlock, error) {
	return makeStubBlock(100), nil
}

func (s *stubChainClient) BlockHeader(_ context.Context, n uint64) (*chain.RPCBlock, error) {
	return makeStubBlock(n), nil
}

func (s *stubChainClient) BlockWithTxs(_ context.Context, n uint64) (*chain.RPCBlock, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	return makeStubBlock(n), nil
}

func (s *stubChainClient) BlockReceipts(_ context.Context, _ common.Hash) ([]chain.RPCReceipt, error) {
	return nil, nil
}

func (s *stubChainClient) Logs(_ context.Context, _ chain.LogFilter) ([]chain.RPCLog, error) {
	return nil, nil
}

func makeStubBlock(n uint64) *chain.RPCBlock {
	return &chain.RPCBlock{
		Number:        hexutil.Uint64(n),
		Hash:          common.BigToHash(new(hexutil.Big).ToInt().SetUint64(n)),
		ParentHash:    common.BigToHash(new(hexutil.Big).ToInt().SetUint64(n - 1)),
		Timestamp:     hexutil.Uint64(1_700_000_000 + n),
		L1BlockNumber: hexutil.Uint64(n / 4),
		GasUsed:       hexutil.Uint64(0),
		GasLimit:      hexutil.Uint64(30_000_000),
		Size:          hexutil.Uint64(512),
	}
}

func TestBackfillRunner_Integration(t *testing.T) {
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "clickhouse/clickhouse-server:25.1",
			ExposedPorts: []string{"9000/tcp"},
			WaitingFor:   wait.ForLog("Ready for connections").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start clickhouse: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := container.MappedPort(ctx, "9000")
	if err != nil {
		t.Fatal(err)
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%s", host, port.Port())},
		Auth: clickhouse.Auth{Username: "default", Password: ""},
	})
	if err != nil {
		t.Fatalf("open clickhouse: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	for _, ddl := range integrationSchema() {
		if err := conn.Exec(ctx, ddl); err != nil {
			t.Fatalf("exec ddl: %v\n%s", err, ddl)
		}
	}

	reg := prometheus.NewRegistry()
	writers := &ingest.Writers{
		Blocks: pwstore.NewBatchWriter(conn, ingest.InsertBlockSQL, 100, 5*time.Second,
			pwstore.NewBatchMetrics(reg, "blocks")),
		Txs: pwstore.NewBatchWriter(conn, ingest.InsertTxSQL, 100, 5*time.Second,
			pwstore.NewBatchMetrics(reg, "txs")),
		Receipts: pwstore.NewBatchWriter(conn, ingest.InsertReceiptSQL, 100, 5*time.Second,
			pwstore.NewBatchMetrics(reg, "receipts")),
		Logs: pwstore.NewBatchWriter(conn, ingest.InsertLogSQL, 100, 5*time.Second,
			pwstore.NewBatchMetrics(reg, "logs")),
	}

	cursors := ingest.NewCursorStore(conn, "backfill")
	metrics := ingest.NewBackfillMetrics(reg)
	client := &stubChainClient{}
	log := slog.New(slog.NewTextHandler(&testWriter{t}, nil))

	runner := ingest.NewBackfillRunner(client, cursors, writers, metrics, log)

	const from, to = uint64(1), uint64(10)
	cfg := ingest.BackfillConfig{
		From:        from,
		To:          to,
		ChunkSize:   5,
		Concurrency: 2,
	}

	if err := runner.Run(ctx, cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, w := range []*pwstore.BatchWriter{writers.Blocks, writers.Txs, writers.Receipts, writers.Logs} {
		if err := w.Flush(ctx); err != nil {
			t.Errorf("flush: %v", err)
		}
	}

	var count uint64
	if err := conn.QueryRow(ctx, `SELECT count() FROM raw.blocks FINAL`).Scan(&count); err != nil {
		t.Fatalf("count blocks: %v", err)
	}
	if count != to-from+1 {
		t.Errorf("raw.blocks: want %d rows, got %d", to-from+1, count)
	}

	// Second run must skip all done chunks (no extra RPC calls).
	calls1 := client.calls
	if err := runner.Run(ctx, cfg); err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if client.calls != calls1 {
		t.Errorf("second run made %d extra RPC calls; want 0", client.calls-calls1)
	}
}

type testWriter struct{ t *testing.T }

func (w *testWriter) Write(p []byte) (int, error) {
	w.t.Log(string(p))
	return len(p), nil
}

func integrationSchema() []string {
	return []string{
		`CREATE DATABASE IF NOT EXISTS raw`,
		`CREATE DATABASE IF NOT EXISTS meta`,
		`CREATE TABLE IF NOT EXISTS raw.blocks (
			block_number     UInt64,
			block_hash       FixedString(66),
			parent_hash      FixedString(66),
			block_timestamp  DateTime64(3, 'UTC'),
			l1_block_number  UInt64,
			base_fee_per_gas UInt256,
			gas_used         UInt64,
			gas_limit        UInt64,
			tx_count         UInt32,
			size             UInt32,
			_ingested_at     DateTime64(3, 'UTC') DEFAULT now64(3),
			_version         UInt64
		) ENGINE = ReplacingMergeTree(_version)
		ORDER BY block_number`,
		`CREATE TABLE IF NOT EXISTS raw.transactions (
			block_number             UInt64,
			block_timestamp          DateTime64(3, 'UTC'),
			tx_index                 UInt32,
			tx_hash                  FixedString(66),
			tx_type                  UInt8,
			from_address             String,
			to_address               String,
			nonce                    UInt64,
			value                    UInt256,
			gas_limit                UInt64,
			gas_price                UInt256,
			max_fee_per_gas          Nullable(UInt256),
			max_priority_fee_per_gas Nullable(UInt256),
			input_selector           String,
			input_size               UInt32,
			_ingested_at             DateTime64(3, 'UTC') DEFAULT now64(3),
			_version                 UInt64
		) ENGINE = ReplacingMergeTree(_version)
		ORDER BY (block_number, tx_index)`,
		`CREATE TABLE IF NOT EXISTS raw.receipts (
			block_number        UInt64,
			block_timestamp     DateTime64(3, 'UTC'),
			tx_index            UInt32,
			tx_hash             FixedString(66),
			status              UInt8,
			gas_used            UInt64,
			gas_used_for_l1     UInt64,
			effective_gas_price UInt256,
			cumulative_gas_used UInt64,
			contract_address    String,
			log_count           UInt32,
			_ingested_at        DateTime64(3, 'UTC') DEFAULT now64(3),
			_version            UInt64
		) ENGINE = ReplacingMergeTree(_version)
		ORDER BY (block_number, tx_index)`,
		`CREATE TABLE IF NOT EXISTS raw.logs (
			block_number    UInt64,
			block_timestamp DateTime64(3, 'UTC'),
			tx_index        UInt32,
			tx_hash         FixedString(66),
			log_index       UInt32,
			address         FixedString(42),
			topic0          FixedString(66),
			topic1          String,
			topic2          String,
			topic3          String,
			data            String,
			_ingested_at    DateTime64(3, 'UTC') DEFAULT now64(3),
			_version        UInt64
		) ENGINE = ReplacingMergeTree(_version)
		ORDER BY (block_number, log_index)`,
		`CREATE TABLE IF NOT EXISTS meta.ingest_cursors (
			job         LowCardinality(String),
			range_start UInt64,
			range_end   UInt64,
			status      LowCardinality(String),
			last_block  UInt64,
			attempts    UInt32,
			error       String,
			updated_at  DateTime64(3, 'UTC')
		) ENGINE = ReplacingMergeTree(updated_at)
		ORDER BY job`,
	}
}
