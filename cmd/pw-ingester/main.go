// pw-ingester is the Priority Watch ingestion binary.
// Subcommands: migrate, backfill, follow, cex, import-timeboost.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"

	pwmigrations "github.com/uwezukwechibuzor/priority-watch/infra/clickhouse/migrations"
	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
	"github.com/uwezukwechibuzor/priority-watch/internal/config"
	"github.com/uwezukwechibuzor/priority-watch/internal/ingest"
	"github.com/uwezukwechibuzor/priority-watch/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := newRootCmd(ctx).Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd(ctx context.Context) *cobra.Command {
	root := &cobra.Command{
		Use:   "pw-ingester",
		Short: "Priority Watch ingestion binary",
	}
	root.AddCommand(
		newMigrateCmd(ctx),
		newBackfillCmd(ctx),
		newFollowCmd(ctx),
		newCEXCmd(ctx),
		newImportTimeboostCmd(ctx),
	)
	return root
}

// ── migrate ──────────────────────────────────────────────────────────────────

func newMigrateCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Apply ClickHouse DDL migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := newLogger()
			return runMigrate(ctx, log)
		},
	}
}

func runMigrate(ctx context.Context, log *slog.Logger) error {
	addr := envOr("CLICKHOUSE_ADDR", "localhost:9000")
	user := envOr("CLICKHOUSE_USER", "default")
	pass := envOr("CLICKHOUSE_PASSWORD", "")

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{Username: user, Password: pass},
		Settings: clickhouse.Settings{
			"max_execution_time": 120,
		},
	})
	if err != nil {
		return fmt.Errorf("open clickhouse: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("ping clickhouse at %s: %w", addr, err)
	}
	log.InfoContext(ctx, "connected to clickhouse", "addr", addr)

	m := store.NewMigrator(conn, pwmigrations.FS)
	if err := m.Run(ctx); err != nil {
		return err
	}
	log.InfoContext(ctx, "migrations complete")
	return nil
}

// ── backfill ─────────────────────────────────────────────────────────────────

func newBackfillCmd(ctx context.Context) *cobra.Command {
	var (
		fromFlag    string
		toFlag      string
		chunkFlag   uint64
		concFlag    int
		logsOnly    bool
		addressFlag string
	)

	cmd := &cobra.Command{
		Use:   "backfill",
		Short: "Backfill historical blocks into ClickHouse",
		Long: `Resumable, idempotent backfill of Arbitrum One blocks.

--from and --to accept a block number or a date in YYYY-MM-DD format.
--to also accepts "head" to mean the current latest block.

Examples:
  pw-ingester backfill --from 100000 --to 110000
  pw-ingester backfill --from 2026-01-01 --to head
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := newLogger()
			return runBackfill(ctx, log, fromFlag, toFlag, chunkFlag, concFlag, logsOnly, addressFlag)
		},
	}

	cmd.Flags().StringVar(&fromFlag, "from", "", "start block or YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&toFlag, "to", "head", "end block, YYYY-MM-DD, or \"head\"")
	cmd.Flags().Uint64Var(&chunkFlag, "chunk", ingest.DefaultChunkSize, "blocks per chunk")
	cmd.Flags().IntVar(&concFlag, "concurrency", 4, "parallel block workers")
	cmd.Flags().BoolVar(&logsOnly, "logs-only", false, "ingest logs only (skip blocks/txs/receipts)")
	cmd.Flags().StringVar(&addressFlag, "address", "", "extra log filter address")

	_ = cmd.MarkFlagRequired("from")
	return cmd
}

func runBackfill(
	ctx context.Context,
	log *slog.Logger,
	fromStr, toStr string,
	chunkSize uint64,
	concurrency int,
	_ bool, // logsOnly — reserved
	extraAddr string,
) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// ── chain client ─────────────────────────────────────────────────────────
	chainReg := prometheus.NewRegistry()
	chainClient, err := chain.New(ctx, chain.ClientConfig{
		PrimaryURL:   cfg.ArbitrumRPCURL,
		FallbackURLs: cfg.ArbitrumRPCFallbackURLs,
		RPS:          cfg.ArbitrumRPCRPS,
		Logger:       log,
		Registry:     chainReg,
	})
	if err != nil {
		return fmt.Errorf("dial rpc: %w", err)
	}
	defer chainClient.Close()

	// ── resolve from/to ───────────────────────────────────────────────────────
	resolver := ingest.NewDateResolver(ingest.NewChainClientQuerier(chainClient))
	from, err := resolveBlock(ctx, fromStr, resolver)
	if err != nil {
		return fmt.Errorf("resolve --from %q: %w", fromStr, err)
	}
	to, err := resolveBlockTo(ctx, toStr, resolver, chainClient)
	if err != nil {
		return fmt.Errorf("resolve --to %q: %w", toStr, err)
	}
	if from > to {
		return fmt.Errorf("--from %d > --to %d", from, to)
	}
	log.InfoContext(ctx, "backfill range", "from", from, "to", to,
		"blocks", to-from+1, "chunk", chunkSize, "concurrency", concurrency)

	// ── clickhouse ───────────────────────────────────────────────────────────
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.ClickHouseAddr},
		Auth: clickhouse.Auth{
			Username: cfg.ClickHouseUser,
			Password: cfg.ClickHousePassword,
		},
		Settings: clickhouse.Settings{"max_execution_time": 120},
	})
	if err != nil {
		return fmt.Errorf("open clickhouse: %w", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("ping clickhouse: %w", err)
	}

	// ── log filter addresses ──────────────────────────────────────────────────
	logAddrs, err := ingest.LoadLogFilterAddresses(
		configPath("config/pools.yaml"),
		configPath("config/contracts.yaml"),
	)
	if err != nil {
		log.WarnContext(ctx, "could not load filter addresses; ingesting all logs", "err", err)
	}
	if extraAddr != "" {
		logAddrs = append(logAddrs, extraAddr)
	}

	// ── batch writers ─────────────────────────────────────────────────────────
	reg := prometheus.NewRegistry()
	writers := &ingest.Writers{
		Blocks: store.NewBatchWriter(conn, ingest.InsertBlockSQL,
			cfg.BatchSize, cfg.BatchFlushInterval, store.NewBatchMetrics(reg, "raw_blocks")),
		Txs: store.NewBatchWriter(conn, ingest.InsertTxSQL,
			cfg.BatchSize, cfg.BatchFlushInterval, store.NewBatchMetrics(reg, "raw_transactions")),
		Receipts: store.NewBatchWriter(conn, ingest.InsertReceiptSQL,
			cfg.BatchSize, cfg.BatchFlushInterval, store.NewBatchMetrics(reg, "raw_receipts")),
		Logs: store.NewBatchWriter(conn, ingest.InsertLogSQL,
			cfg.BatchSize, cfg.BatchFlushInterval, store.NewBatchMetrics(reg, "raw_logs")),
	}

	// ── cursor store ──────────────────────────────────────────────────────────
	cursors := ingest.NewCursorStore(conn, "backfill")

	// ── metrics ───────────────────────────────────────────────────────────────
	metrics := ingest.NewBackfillMetrics(reg)

	// ── run ───────────────────────────────────────────────────────────────────
	runner := ingest.NewBackfillRunner(chainClient, cursors, writers, metrics, log)
	backfillCfg := ingest.BackfillConfig{
		From:         from,
		To:           to,
		ChunkSize:    chunkSize,
		Concurrency:  concurrency,
		LogAddresses: logAddrs,
	}

	if err := runner.Run(ctx, backfillCfg); err != nil {
		return fmt.Errorf("backfill run: %w", err)
	}

	// Flush any remaining rows in the batch writers.
	for _, w := range []*store.BatchWriter{writers.Blocks, writers.Txs, writers.Receipts, writers.Logs} {
		if err := w.Flush(context.Background()); err != nil {
			log.WarnContext(ctx, "final flush error", "err", err)
		}
	}

	log.InfoContext(ctx, "backfill complete", "from", from, "to", to)
	return nil
}

// resolveBlock parses a block-number or YYYY-MM-DD string into a block number.
func resolveBlock(ctx context.Context, s string, r *ingest.DateResolver) (uint64, error) {
	if n, err := strconv.ParseUint(s, 10, 64); err == nil {
		return n, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return 0, fmt.Errorf("must be a block number or YYYY-MM-DD: %w", err)
	}
	return r.Resolve(ctx, t)
}

// resolveBlockTo resolves --to (block number, date, or "head").
func resolveBlockTo(
	ctx context.Context,
	s string,
	r *ingest.DateResolver,
	c interface{ Head(ctx context.Context) (*chain.RPCBlock, error) },
) (uint64, error) {
	if s == "head" {
		blk, err := c.Head(ctx)
		if err != nil {
			return 0, err
		}
		return uint64(blk.Number), nil
	}
	return resolveBlock(ctx, s, r)
}

// configPath returns a path relative to the current working directory.
// Falls back to a relative path if os.Getwd fails.
func configPath(rel string) string {
	wd, err := os.Getwd()
	if err != nil {
		return rel
	}
	return filepath.Join(wd, rel)
}

// ── stubs ─────────────────────────────────────────────────────────────────────

func newFollowCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "follow",
		Short: "Live block follower (task 6)",
		RunE: func(cmd *cobra.Command, args []string) error {
			newLogger().InfoContext(ctx, "follow: implemented in task 6")
			return nil
		},
	}
}

func newCEXCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "cex",
		Short: "Binance klines ingestion (task 7)",
		RunE: func(cmd *cobra.Command, args []string) error {
			newLogger().InfoContext(ctx, "cex: implemented in task 7")
			return nil
		},
	}
}

func newImportTimeboostCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "import-timeboost",
		Short: "Import Timeboost baseline (task 8)",
		RunE: func(cmd *cobra.Command, args []string) error {
			newLogger().InfoContext(ctx, "import-timeboost: implemented in task 8")
			return nil
		},
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newLogger() *slog.Logger {
	level := slog.LevelInfo
	if v := os.Getenv("LOG_LEVEL"); v == "debug" {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
