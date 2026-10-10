//go:build integration

package ingest_test

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
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

// reorgScenario describes a sequence of block-head notifications to feed into
// the follower, and what the expected ClickHouse state should be afterwards.
type reorgScenario struct {
	// blocks is the sequence of (number, hash, parentHash) triples sent to the follower.
	// Use makeTestBlock / makeReorgBlock helpers below.
	blocks []*chain.RPCBlock

	wantCanonicalBlocks uint64 // expected count in raw.blocks FINAL
	wantReorgRows       uint64 // expected count in raw.reorg_log
}

func TestFollower_Integration(t *testing.T) {
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

	for _, ddl := range followerIntegrationSchema() {
		if err := conn.Exec(ctx, ddl); err != nil {
			t.Fatalf("exec ddl: %v\n%s", err, ddl)
		}
	}

	tests := []struct {
		name     string
		scenario reorgScenario
	}{
		{
			name: "no_reorg_depth_0",
			scenario: reorgScenario{
				blocks: []*chain.RPCBlock{
					makeTestBlock(1, "h1", "h0"),
					makeTestBlock(2, "h2", "h1"),
					makeTestBlock(3, "h3", "h2"),
					makeTestBlock(4, "h4", "h3"),
					makeTestBlock(5, "h5", "h4"),
				},
				wantCanonicalBlocks: 5,
				wantReorgRows:       0,
			},
		},
		{
			name: "depth_1_reorg",
			scenario: reorgScenario{
				// Blocks 1-4 normal; block 5a is stale; block 5b is canonical.
				blocks: []*chain.RPCBlock{
					makeTestBlock(1, "h1", "h0"),
					makeTestBlock(2, "h2", "h1"),
					makeTestBlock(3, "h3", "h2"),
					makeTestBlock(4, "h4", "h3"),
					makeTestBlock(5, "h5a", "h4"), // will be reorged out
					makeTestBlock(5, "h5b", "h4"), // reorg: same parent, different hash
				},
				wantCanonicalBlocks: 5,
				wantReorgRows:       1,
			},
		},
		{
			name: "depth_3_reorg",
			scenario: reorgScenario{
				// Blocks 1-4 normal; blocks 5-7 stale; then reorg at block 5.
				blocks: []*chain.RPCBlock{
					makeTestBlock(1, "h1", "h0"),
					makeTestBlock(2, "h2", "h1"),
					makeTestBlock(3, "h3", "h2"),
					makeTestBlock(4, "h4", "h3"),
					makeTestBlock(5, "h5a", "h4"),
					makeTestBlock(6, "h6a", "h5a"),
					makeTestBlock(7, "h7a", "h6a"),
					// Reorg back to block 4; new branch starts with h5b.
					makeTestBlock(5, "h5b", "h4"),
				},
				wantCanonicalBlocks: 5,
				wantReorgRows:       1,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Truncate tables between sub-tests for isolation.
			for _, tbl := range []string{"raw.blocks", "raw.transactions", "raw.receipts", "raw.logs", "raw.reorg_log"} {
				if err := conn.Exec(ctx, "TRUNCATE TABLE "+tbl); err != nil {
					t.Fatalf("truncate %s: %v", tbl, err)
				}
			}

			reg := prometheus.NewRegistry()
			writers := &ingest.FollowerWriters{
				Blocks: pwstore.NewBatchWriter(conn, ingest.InsertBlockSQL, 100, 5*time.Second,
					pwstore.NewBatchMetrics(reg, "blocks")),
				Txs: pwstore.NewBatchWriter(conn, ingest.InsertTxSQL, 100, 5*time.Second,
					pwstore.NewBatchMetrics(reg, "txs")),
				Receipts: pwstore.NewBatchWriter(conn, ingest.InsertReceiptSQL, 100, 5*time.Second,
					pwstore.NewBatchMetrics(reg, "receipts")),
				Logs: pwstore.NewBatchWriter(conn, ingest.InsertLogSQL, 100, 5*time.Second,
					pwstore.NewBatchMetrics(reg, "logs")),
				ReorgLog: pwstore.NewBatchWriter(conn, ingest.InsertReorgLogSQL, 100, 5*time.Second,
					pwstore.NewBatchMetrics(reg, "reorg_log")),
			}

			chainClient := newStubFollowerClient(tc.scenario.blocks)
			wsSub := &fakeWSSubscriber{blocks: tc.scenario.blocks}

			cursors := ingest.NewCursorStore(conn, "follower-test")
			metrics := ingest.NewFollowerMetrics(reg)

			cfg := ingest.FollowerConfig{
				WindowSize:      10,
				PollInterval:    10 * time.Millisecond,
				BalanceInterval: 24 * time.Hour,
				SafeInterval:    24 * time.Hour,
				GapThreshold:    1000,
			}

			log := slog.New(slog.NewTextHandler(&testWriter{t}, nil))
			follower := ingest.NewFollower(chainClient, wsSub, cursors, writers, metrics, cfg, log)

			// Run with a context that cancels once all blocks are consumed.
			runCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			// wsSub closes its done channel after sending all blocks.
			go func() {
				<-wsSub.done
				cancel()
			}()

			_ = follower.Run(runCtx)

			// Flush all writers.
			for _, w := range []*pwstore.BatchWriter{
				writers.Blocks, writers.Txs, writers.Receipts,
				writers.Logs, writers.ReorgLog,
			} {
				if err := w.Flush(ctx); err != nil {
					t.Errorf("flush: %v", err)
				}
			}

			// Give ClickHouse a moment to merge.
			time.Sleep(200 * time.Millisecond)

			var gotBlocks uint64
			if err := conn.QueryRow(ctx, `SELECT count() FROM raw.blocks FINAL`).Scan(&gotBlocks); err != nil {
				t.Fatalf("count blocks: %v", err)
			}
			if gotBlocks != tc.scenario.wantCanonicalBlocks {
				t.Errorf("raw.blocks FINAL: want %d rows, got %d",
					tc.scenario.wantCanonicalBlocks, gotBlocks)
			}

			var gotReorgs uint64
			if err := conn.QueryRow(ctx, `SELECT count() FROM raw.reorg_log`).Scan(&gotReorgs); err != nil {
				t.Fatalf("count reorg_log: %v", err)
			}
			if gotReorgs != tc.scenario.wantReorgRows {
				t.Errorf("raw.reorg_log: want %d rows, got %d",
					tc.scenario.wantReorgRows, gotReorgs)
			}
		})
	}
}

// stubFollowerClient is a fake FollowerChainClient.
type stubFollowerClient struct {
	blocks map[uint64]*chain.RPCBlock
	head   *chain.RPCBlock
}

func newStubFollowerClient(seq []*chain.RPCBlock) *stubFollowerClient {
	m := make(map[uint64]*chain.RPCBlock)
	var head *chain.RPCBlock
	for _, b := range seq {
		n := uint64(b.Number)
		m[n] = b // last one for each number wins (canonical)
		if head == nil || n >= uint64(head.Number) {
			head = b
		}
	}
	return &stubFollowerClient{blocks: m, head: head}
}

func (s *stubFollowerClient) Head(_ context.Context) (*chain.RPCBlock, error) {
	return s.head, nil
}

func (s *stubFollowerClient) BlockHeader(_ context.Context, n uint64) (*chain.RPCBlock, error) {
	if b, ok := s.blocks[n]; ok {
		return b, nil
	}
	return makeTestBlock(n, fmt.Sprintf("h%d", n), fmt.Sprintf("h%d", n-1)), nil
}

func (s *stubFollowerClient) BlockWithTxs(_ context.Context, n uint64) (*chain.RPCBlock, error) {
	if b, ok := s.blocks[n]; ok {
		return b, nil
	}
	return makeTestBlock(n, fmt.Sprintf("h%d", n), fmt.Sprintf("h%d", n-1)), nil
}

func (s *stubFollowerClient) BlockReceipts(_ context.Context, _ common.Hash) ([]chain.RPCReceipt, error) {
	return nil, nil
}

func (s *stubFollowerClient) Logs(_ context.Context, _ chain.LogFilter) ([]chain.RPCLog, error) {
	return nil, nil
}

func (s *stubFollowerClient) Safe(_ context.Context) (*chain.RPCBlock, error) {
	return s.head, nil
}

func (s *stubFollowerClient) Finalized(_ context.Context) (*chain.RPCBlock, error) {
	return s.head, nil
}

func (s *stubFollowerClient) Balance(_ context.Context, _ common.Address, _ string) (*big.Int, error) {
	return new(big.Int), nil
}

// fakeWSSubscriber implements wsSubscriber by sending a preset sequence of
// blocks into the follower's head channel one at a time.
type fakeWSSubscriber struct {
	blocks []*chain.RPCBlock
	done   chan struct{}
}

func (f *fakeWSSubscriber) SubscribeNewHeads(_ context.Context, ch chan<- *chain.RPCBlock) (chain.HeadSubscription, error) {
	f.done = make(chan struct{})
	errCh := make(chan error, 1)
	sub := &fakeHeadSub{errCh: errCh}

	go func() {
		defer close(f.done)
		for _, b := range f.blocks {
			ch <- b
			time.Sleep(5 * time.Millisecond)
		}
	}()

	return sub, nil
}

// fakeHeadSub satisfies chain.HeadSubscription.
type fakeHeadSub struct {
	errCh chan error
}

func (s *fakeHeadSub) Err() <-chan error { return s.errCh }
func (s *fakeHeadSub) Unsubscribe()      {}

// makeTestBlock creates a block with the given number, hash and parent hash
// using deterministic values so validators can check chain continuity.
func makeTestBlock(n uint64, hash, parentHash string) *chain.RPCBlock {
	return &chain.RPCBlock{
		Number:        hexutil.Uint64(n),
		Hash:          common.HexToHash(fmt.Sprintf("0x%064s", hash)),
		ParentHash:    common.HexToHash(fmt.Sprintf("0x%064s", parentHash)),
		Timestamp:     hexutil.Uint64(1_750_000_000 + n),
		L1BlockNumber: hexutil.Uint64(n / 4),
		GasUsed:       hexutil.Uint64(0),
		GasLimit:      hexutil.Uint64(32_000_000),
		Size:          hexutil.Uint64(512),
	}
}

func followerIntegrationSchema() []string {
	base := integrationSchema()
	return append(base,
		`CREATE TABLE IF NOT EXISTS raw.reorg_log (
			detected_at DateTime64(3, 'UTC'),
			fork_block  UInt64,
			old_hash    FixedString(66),
			new_hash    FixedString(66),
			depth       UInt32
		) ENGINE = MergeTree()
		ORDER BY detected_at`,
		`CREATE TABLE IF NOT EXISTS raw.balance_snapshots (
			address        FixedString(42),
			block_number   UInt64,
			block_timestamp DateTime64(3, 'UTC'),
			balance_wei    UInt256,
			_ingested_at   DateTime64(3, 'UTC') DEFAULT now64(3),
			_version       UInt64
		) ENGINE = ReplacingMergeTree(_version)
		ORDER BY (address, block_number)`,
		`CREATE TABLE IF NOT EXISTS meta.source_status (
			source           LowCardinality(String),
			rows             UInt64,
			latest_position  String,
			latest_timestamp DateTime64(3, 'UTC'),
			updated_at       DateTime64(3, 'UTC')
		) ENGINE = ReplacingMergeTree(updated_at)
		ORDER BY source`,
	)
}
