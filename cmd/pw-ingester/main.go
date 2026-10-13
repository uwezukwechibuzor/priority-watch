// pw-ingester is the Priority Watch ingestion binary.
// Subcommands: migrate, backfill, follow, cex, import-timeboost, eval-models.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	pwmigrations "github.com/uwezukwechibuzor/priority-watch/infra/clickhouse/migrations"
	"github.com/uwezukwechibuzor/priority-watch/internal/cex"
	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
	"github.com/uwezukwechibuzor/priority-watch/internal/chain/hypersync"
	"github.com/uwezukwechibuzor/priority-watch/internal/chain/sqd"
	"github.com/uwezukwechibuzor/priority-watch/internal/config"
	"github.com/uwezukwechibuzor/priority-watch/internal/ingest"
	"github.com/uwezukwechibuzor/priority-watch/internal/models"
	"github.com/uwezukwechibuzor/priority-watch/internal/obs"
	"github.com/uwezukwechibuzor/priority-watch/internal/store"
	"github.com/uwezukwechibuzor/priority-watch/internal/timeboost"
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
		newEvalModelsCmd(ctx),
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
		sourceFlag  string
	)

	cmd := &cobra.Command{
		Use:   "backfill",
		Short: "Backfill historical blocks into ClickHouse",
		Long: `Resumable, idempotent backfill of Arbitrum One blocks.

--from and --to accept a block number or a date in YYYY-MM-DD format.
--to also accepts "head" to mean the current latest block.
--source selects the data provider: sqd | hypersync | rpc | sqd+hypersync.

Examples:
  pw-ingester backfill --from 100000 --to 110000
  pw-ingester backfill --from 2026-01-01 --to head --source sqd+hypersync
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := newLogger()
			return runBackfill(ctx, log, fromFlag, toFlag, chunkFlag, concFlag, logsOnly, addressFlag, sourceFlag)
		},
	}

	cmd.Flags().StringVar(&fromFlag, "from", "", "start block or YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&toFlag, "to", "head", "end block, YYYY-MM-DD, or \"head\"")
	cmd.Flags().Uint64Var(&chunkFlag, "chunk", ingest.DefaultChunkSize, "blocks per chunk")
	cmd.Flags().IntVar(&concFlag, "concurrency", 4, "parallel block workers (rpc source only)")
	cmd.Flags().BoolVar(&logsOnly, "logs-only", false, "ingest logs only (skip blocks/txs/receipts)")
	cmd.Flags().StringVar(&addressFlag, "address", "", "extra log filter address")
	cmd.Flags().StringVar(&sourceFlag, "source", "", "data source: sqd|hypersync|rpc|sqd+hypersync (default from BLOCK_SOURCE)")

	_ = cmd.MarkFlagRequired("from")
	return cmd
}

func runBackfill(
	ctx context.Context,
	log *slog.Logger,
	fromStr, toStr string,
	chunkSize uint64,
	concurrency int,
	logsOnly bool,
	extraAddr string,
	sourceFlag string,
) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	sourceName := sourceFlag
	if sourceName == "" {
		sourceName = cfg.BlockSource
	}

	// ── chain client (always created: used for date/head resolution and, when
	// source=rpc, for block data) ──────────────────────────────────────────────
	chainReg := prometheus.NewRegistry()
	chainClient, err := chain.New(ctx, chain.ClientConfig{
		PrimaryURL:   rpcURLOrPlaceholder(cfg.ArbitrumRPCURL),
		FallbackURLs: cfg.ArbitrumRPCFallbackURLs,
		RPS:          cfg.ArbitrumRPCRPS,
		Logger:       log,
		Registry:     chainReg,
	})
	if err != nil {
		return fmt.Errorf("dial rpc: %w", err)
	}
	defer chainClient.Close()

	// ── block source ───────────────────────────────────────────────────────────
	source, err := buildBlockSource(sourceName, cfg, chainClient, log)
	if err != nil {
		return fmt.Errorf("build block source: %w", err)
	}

	// ── resolve from/to ───────────────────────────────────────────────────────
	resolver := ingest.NewDateResolver(ingest.NewChainClientQuerier(chainClient))
	from, err := resolveBlock(ctx, fromStr, resolver)
	if err != nil {
		return fmt.Errorf("resolve --from %q: %w", fromStr, err)
	}
	to, err := resolveBlockToSource(ctx, toStr, resolver, source)
	if err != nil {
		return fmt.Errorf("resolve --to %q: %w", toStr, err)
	}
	if from > to {
		return fmt.Errorf("--from %d > --to %d", from, to)
	}
	log.InfoContext(ctx, "backfill range", "source", source.Name(), "from", from, "to", to,
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
	runner := ingest.NewSourceBackfillRunner(source, cursors, writers, metrics, log)
	backfillCfg := ingest.BackfillConfig{
		From:         from,
		To:           to,
		ChunkSize:    chunkSize,
		Concurrency:  concurrency,
		LogAddresses: logAddrs,
		LogsOnly:     logsOnly,
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

// resolveBlockToSource resolves --to using a chain.BlockSource for "head".
func resolveBlockToSource(ctx context.Context, s string, r *ingest.DateResolver, src chain.BlockSource) (uint64, error) {
	if s == "head" {
		return src.Head(ctx)
	}
	return resolveBlock(ctx, s, r)
}

// buildBlockSource constructs a chain.BlockSource from the named source.
// Supported names: rpc, sqd, hypersync, sqd+hypersync (failover).
func buildBlockSource(name string, cfg *config.Config, c *chain.Client, log *slog.Logger) (chain.BlockSource, error) {
	switch name {
	case "rpc":
		if cfg.ArbitrumRPCURL == "" {
			return nil, fmt.Errorf("ARBITRUM_RPC_URL is required for --source rpc")
		}
		return chain.NewRPCBlockSource(c, log), nil
	case "sqd", "":
		return sqd.New(cfg.SQDPortalURL, log), nil
	case "hypersync":
		return hypersync.New(cfg.EnvioHyperSyncURL, cfg.EnvioAPIToken, log), nil
	case "sqd+hypersync":
		primary := sqd.New(cfg.SQDPortalURL, log)
		secondary := hypersync.New(cfg.EnvioHyperSyncURL, cfg.EnvioAPIToken, log)
		return chain.NewFailoverBlockSource(primary, secondary, log), nil
	default:
		return nil, fmt.Errorf("unknown source %q (want rpc|sqd|hypersync|sqd+hypersync)", name)
	}
}

// rpcURLOrPlaceholder returns url, or a loopback placeholder when empty so the
// RPC client can still be constructed for date resolution in SQD/HyperSync mode.
func rpcURLOrPlaceholder(url string) string {
	if url == "" {
		return "http://127.0.0.1:8545"
	}
	return url
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

// ── follow ────────────────────────────────────────────────────────────────────

func newFollowCmd(ctx context.Context) *cobra.Command {
	var sourceFlag string
	cmd := &cobra.Command{
		Use:   "follow",
		Short: "Live block follower — streams new blocks from the configured source",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := newLogger()
			return runFollow(ctx, log, sourceFlag)
		},
	}
	cmd.Flags().StringVar(&sourceFlag, "source", "", "data source: sqd|hypersync|rpc|sqd+hypersync (default from BLOCK_SOURCE)")
	return cmd
}

func runFollow(ctx context.Context, log *slog.Logger, sourceFlag string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	sourceName := sourceFlag
	if sourceName == "" {
		sourceName = cfg.BlockSource
	}

	// ── obs server (created first so its registry is shared with all metrics) ──
	obsServer := obs.New(cfg.MetricsAddr, cfg.LogLevel)
	reg := obsServer.Registry()

	// ── chain HTTP client (balance snapshots + rpc source) ─────────────────────
	chainClient, err := chain.New(ctx, chain.ClientConfig{
		PrimaryURL:   rpcURLOrPlaceholder(cfg.ArbitrumRPCURL),
		FallbackURLs: cfg.ArbitrumRPCFallbackURLs,
		RPS:          cfg.ArbitrumRPCRPS,
		Logger:       log,
		Registry:     reg,
	})
	if err != nil {
		return fmt.Errorf("dial rpc: %w", err)
	}
	defer chainClient.Close()

	// ── optional WS client for newHeads subscription (rpc source only) ─────────
	var wsClient *chain.Client
	if sourceName == "rpc" && cfg.ArbitrumWSURL != "" {
		wsClient, err = chain.New(ctx, chain.ClientConfig{
			PrimaryURL: cfg.ArbitrumWSURL,
			RPS:        cfg.ArbitrumRPCRPS,
			Logger:     log,
			Registry:   prometheus.NewRegistry(),
		})
		if err != nil {
			log.WarnContext(ctx, "dial WS failed, will poll", "err", err)
			wsClient = nil
		} else {
			defer wsClient.Close()
		}
	}

	// ── ClickHouse ────────────────────────────────────────────────────────────
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

	// ── batch writers ─────────────────────────────────────────────────────────
	fw := &ingest.FollowerWriters{
		Blocks: store.NewBatchWriter(conn, ingest.InsertBlockSQL,
			cfg.BatchSize, cfg.BatchFlushInterval, store.NewBatchMetrics(reg, "raw_blocks")),
		Txs: store.NewBatchWriter(conn, ingest.InsertTxSQL,
			cfg.BatchSize, cfg.BatchFlushInterval, store.NewBatchMetrics(reg, "raw_transactions")),
		Receipts: store.NewBatchWriter(conn, ingest.InsertReceiptSQL,
			cfg.BatchSize, cfg.BatchFlushInterval, store.NewBatchMetrics(reg, "raw_receipts")),
		Logs: store.NewBatchWriter(conn, ingest.InsertLogSQL,
			cfg.BatchSize, cfg.BatchFlushInterval, store.NewBatchMetrics(reg, "raw_logs")),
		ReorgLog: store.NewBatchWriter(conn, ingest.InsertReorgLogSQL,
			cfg.BatchSize, cfg.BatchFlushInterval, store.NewBatchMetrics(reg, "raw_reorg_log")),
		Balances: store.NewBatchWriter(conn, ingest.InsertBalanceSnapshotSQL,
			cfg.BatchSize, cfg.BatchFlushInterval, store.NewBatchMetrics(reg, "raw_balance_snapshots")),
		StatusUpsert: store.NewSourceStatusUpsert(conn),
	}

	cursors := ingest.NewCursorStore(conn, "follower")
	metrics := ingest.NewFollowerMetrics(reg)

	followerCfg := ingest.FollowerConfig{
		LogAddresses: logAddrs,
		GapThreshold: cfg.GapThreshold,
	}

	// follower is either the RPC-native Follower or the BlockSource-backed
	// SourceFollower, selected by --source / BLOCK_SOURCE.
	var follower interface {
		Run(ctx context.Context) error
		HealthFunc() func() error
	}
	if sourceName == "rpc" {
		follower = ingest.NewFollower(chainClient, wsClient, cursors, fw, metrics, followerCfg, log)
	} else {
		source, serr := buildBlockSource(sourceName, cfg, chainClient, log)
		if serr != nil {
			return fmt.Errorf("build block source: %w", serr)
		}
		follower = ingest.NewSourceFollower(source, cursors, fw, metrics, followerCfg, log)
	}

	// ── obs server (created earlier; wire health check now that follower exists)
	obsServer.SetHealthCheck(follower.HealthFunc())

	// ── run all services ──────────────────────────────────────────────────────
	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error { return obsServer.Serve(egCtx) })
	eg.Go(func() error { return fw.Blocks.Run(egCtx) })
	eg.Go(func() error { return fw.Txs.Run(egCtx) })
	eg.Go(func() error { return fw.Receipts.Run(egCtx) })
	eg.Go(func() error { return fw.Logs.Run(egCtx) })
	eg.Go(func() error { return fw.ReorgLog.Run(egCtx) })
	eg.Go(func() error { return fw.Balances.Run(egCtx) })
	eg.Go(func() error { return follower.Run(egCtx) })

	if err := eg.Wait(); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

func newCEXCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cex",
		Short: "Binance 1s klines ingestion",
	}
	cmd.AddCommand(
		newCEXBackfillCmd(ctx),
		newCEXFollowCmd(ctx),
	)
	return cmd
}

func newCEXBackfillCmd(ctx context.Context) *cobra.Command {
	var (
		symbolsFlag string
		fromFlag    string
		toFlag      string
	)
	cmd := &cobra.Command{
		Use:   "backfill",
		Short: "Download historical 1s klines from data.binance.vision",
		Long: `Downloads Binance 1s klines for each symbol for each calendar day in [from, to].
Verifies the SHA-256 checksum, parses the CSV, and loads rows into raw.cex_klines_1s.

Examples:
  pw-ingester cex backfill --from 2026-09-01 --to 2026-09-30
  pw-ingester cex backfill --symbols ETHUSDT,BTCUSDT --from 2026-01-01 --to 2026-01-07
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := newLogger()
			return runCEXBackfill(ctx, log, symbolsFlag, fromFlag, toFlag)
		},
	}
	cmd.Flags().StringVar(&symbolsFlag, "symbols", "ETHUSDT,ARBUSDT,BTCUSDT", "comma-separated Binance symbols")
	cmd.Flags().StringVar(&fromFlag, "from", "", "start date YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&toFlag, "to", "", "end date YYYY-MM-DD (required)")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

func runCEXBackfill(ctx context.Context, log *slog.Logger, symbolsFlag, fromStr, toStr string) error {
	symbols := strings.Split(symbolsFlag, ",")

	from, err := time.ParseInLocation("2006-01-02", fromStr, time.UTC)
	if err != nil {
		return fmt.Errorf("parse --from %q: %w", fromStr, err)
	}
	to, err := time.ParseInLocation("2006-01-02", toStr, time.UTC)
	if err != nil {
		return fmt.Errorf("parse --to %q: %w", toStr, err)
	}
	if from.After(to) {
		return fmt.Errorf("--from %s is after --to %s", fromStr, toStr)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr:     []string{cfg.ClickHouseAddr},
		Auth:     clickhouse.Auth{Username: cfg.ClickHouseUser, Password: cfg.ClickHousePassword},
		Settings: clickhouse.Settings{"max_execution_time": 120},
	})
	if err != nil {
		return fmt.Errorf("open clickhouse: %w", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("ping clickhouse: %w", err)
	}

	reg := prometheus.NewRegistry()
	w := store.NewBatchWriter(conn, cex.InsertKlineSQL,
		cfg.BatchSize, cfg.BatchFlushInterval, store.NewBatchMetrics(reg, "raw_cex_klines_1s"))
	status := store.NewSourceStatusUpsert(conn)

	src := cex.NewBinanceSource(nil, log)

	var totalRows uint64
	for _, sym := range symbols {
		sym = strings.TrimSpace(sym)
		if sym == "" {
			continue
		}
		log.InfoContext(ctx, "backfilling symbol", "symbol", sym, "from", fromStr, "to", toStr)
		if err := src.Backfill(ctx, sym, from, to, func(klines []cex.Kline) error {
			now := time.Now().UTC()
			for _, k := range klines {
				if err := cex.AppendKline(ctx, w, k, now); err != nil {
					return err
				}
				totalRows++
			}
			return nil
		}); err != nil {
			return fmt.Errorf("backfill %s: %w", sym, err)
		}
	}

	if err := w.Flush(context.Background()); err != nil {
		log.WarnContext(ctx, "final flush error", "err", err)
	}

	if err := status.Upsert(ctx, store.SourceStatusRow{
		Source:          "CEX price feed (Binance 1s OHLCV)",
		Rows:            totalRows,
		LatestPosition:  toStr,
		LatestTimestamp: to,
	}); err != nil {
		log.WarnContext(ctx, "source_status upsert failed", "err", err)
	}

	log.InfoContext(ctx, "cex backfill complete", "symbols", symbolsFlag, "rows", totalRows)
	return nil
}

func newCEXFollowCmd(ctx context.Context) *cobra.Command {
	var symbolsFlag string
	cmd := &cobra.Command{
		Use:   "follow",
		Short: "Live Binance 1s klines via WebSocket",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := newLogger()
			return runCEXFollow(ctx, log, symbolsFlag)
		},
	}
	cmd.Flags().StringVar(&symbolsFlag, "symbols", "ETHUSDT,ARBUSDT,BTCUSDT", "comma-separated Binance symbols")
	return cmd
}

func runCEXFollow(ctx context.Context, log *slog.Logger, symbolsFlag string) error {
	symbols := strings.Split(symbolsFlag, ",")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr:     []string{cfg.ClickHouseAddr},
		Auth:     clickhouse.Auth{Username: cfg.ClickHouseUser, Password: cfg.ClickHousePassword},
		Settings: clickhouse.Settings{"max_execution_time": 120},
	})
	if err != nil {
		return fmt.Errorf("open clickhouse: %w", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("ping clickhouse: %w", err)
	}

	reg := prometheus.NewRegistry()
	w := store.NewBatchWriter(conn, cex.InsertKlineSQL,
		cfg.BatchSize, cfg.BatchFlushInterval, store.NewBatchMetrics(reg, "raw_cex_klines_1s"))
	statusUpd := store.NewSourceStatusUpsert(conn)

	src := cex.NewBinanceSource(nil, log)

	var totalRows atomic.Uint64

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error { return w.Run(egCtx) })
	eg.Go(func() error {
		return src.Follow(egCtx, symbols, func(k cex.Kline) error {
			if err := cex.AppendKline(egCtx, w, k, time.Now().UTC()); err != nil {
				return err
			}
			n := totalRows.Add(1)
			if n%1000 == 0 {
				if err := statusUpd.Upsert(egCtx, store.SourceStatusRow{
					Source:          "CEX price feed (Binance 1s OHLCV)",
					Rows:            n,
					LatestPosition:  k.Symbol,
					LatestTimestamp: k.OpenTime,
				}); err != nil {
					log.WarnContext(egCtx, "source_status upsert failed", "err", err)
				}
			}
			return nil
		})
	})

	if err := eg.Wait(); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

func newImportTimeboostCmd(ctx context.Context) *cobra.Command {
	var fromFlag, toFlag string
	cmd := &cobra.Command{
		Use:   "import-timeboost",
		Short: "Download timeboost.app Parquet baseline into raw.timeboost_bids_ext",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImportTimeboost(ctx, newLogger(), fromFlag, toFlag)
		},
	}
	cmd.Flags().StringVar(&fromFlag, "from", "", "start date YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&toFlag, "to", "", "end date YYYY-MM-DD (required)")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

func runImportTimeboost(ctx context.Context, log *slog.Logger, fromStr, toStr string) error {
	from, err := time.ParseInLocation("2006-01-02", fromStr, time.UTC)
	if err != nil {
		return fmt.Errorf("parse --from %q: %w", fromStr, err)
	}
	to, err := time.ParseInLocation("2006-01-02", toStr, time.UTC)
	if err != nil {
		return fmt.Errorf("parse --to %q: %w", toStr, err)
	}
	if from.After(to) {
		return fmt.Errorf("--from %s is after --to %s", fromStr, toStr)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr:     []string{cfg.ClickHouseAddr},
		Auth:     clickhouse.Auth{Username: cfg.ClickHouseUser, Password: cfg.ClickHousePassword},
		Settings: clickhouse.Settings{"max_execution_time": 120},
	})
	if err != nil {
		return fmt.Errorf("open clickhouse: %w", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("ping clickhouse: %w", err)
	}

	reg := prometheus.NewRegistry()
	bidsWriter := store.NewBatchWriter(conn, timeboost.InsertBidSQL,
		cfg.BatchSize, cfg.BatchFlushInterval, store.NewBatchMetrics(reg, "raw_timeboost_bids_ext"))
	resultsWriter := store.NewBatchWriter(conn, timeboost.InsertResultSQL,
		cfg.BatchSize, cfg.BatchFlushInterval, store.NewBatchMetrics(reg, "raw_timeboost_auction_results_ext"))

	importCfg := timeboost.Config{
		BidsURL:    envOr("TIMEBOOST_BIDS_URL", "https://timeboost.app/api/bids/"),
		ResultsURL: envOr("TIMEBOOST_RESULTS_URL", "https://timeboost.app/api/results/"),
		RateLimit:  60,
		From:       from,
		To:         to,
	}

	imp := timeboost.NewImporter(importCfg, bidsWriter, resultsWriter, nil, log)
	if err := imp.Run(ctx); err != nil {
		return fmt.Errorf("import-timeboost run: %w", err)
	}

	log.InfoContext(ctx, "import-timeboost complete", "from", fromStr, "to", toStr)
	return nil
}

// ── eval-models ───────────────────────────────────────────────────────────────

func newEvalModelsCmd(ctx context.Context) *cobra.Command {
	var trainBlocks, testBlocks, n int
	var confidence float64

	cmd := &cobra.Command{
		Use:   "eval-models",
		Short: "Evaluate pricing models and write results to marts.model_eval",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := newLogger()
			return runEvalModels(ctx, log, trainBlocks, testBlocks, n, confidence)
		},
	}
	cmd.Flags().IntVar(&trainBlocks, "train", 500, "Training window in blocks")
	cmd.Flags().IntVar(&testBlocks, "test", 100, "Held-out test blocks")
	cmd.Flags().IntVar(&n, "n", 1, "Target rank position")
	cmd.Flags().Float64Var(&confidence, "confidence", 0.80, "Target confidence")
	return cmd
}

func runEvalModels(ctx context.Context, log *slog.Logger, trainBlocks, testBlocks, n int, confidence float64) error {
	addr := envOr("CLICKHOUSE_ADDR", "localhost:9000")
	user := envOr("CLICKHOUSE_USER", "default")
	pass := envOr("CLICKHOUSE_PASSWORD", "")

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr:     []string{addr},
		Auth:     clickhouse.Auth{Username: user, Password: pass},
		Settings: clickhouse.Settings{"max_execution_time": 120},
	})
	if err != nil {
		return fmt.Errorf("clickhouse open: %w", err)
	}
	defer conn.Close() //nolint:errcheck

	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("clickhouse ping: %w", err)
	}

	total := trainBlocks + testBlocks
	log.InfoContext(ctx, "fetching block fee history", "blocks", total)

	rows, err := conn.Query(ctx, `
		SELECT block_number, block_timestamp, tipped_tx_count,
			top_fee_gwei, second_to_first_ratio, median_fee_gwei
		FROM marts.fct_blocks FINAL
		WHERE tipped_tx_count > 0
		ORDER BY block_number DESC
		LIMIT @limit`,
		clickhouse.Named("limit", total))
	if err != nil {
		return fmt.Errorf("query fct_blocks: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var hist []models.BlockFeeRecord
	for rows.Next() {
		var r models.BlockFeeRecord
		if err := rows.Scan(
			&r.BlockNumber, &r.Timestamp, &r.TippedCount,
			&r.TopFeeGwei, &r.SecondToFirstRatio, &r.MedianFeeGwei,
		); err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		hist = append(hist, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// Reverse to oldest-first.
	for i, j := 0, len(hist)-1; i < j; i, j = i+1, j-1 {
		hist[i], hist[j] = hist[j], hist[i]
	}

	log.InfoContext(ctx, "running model evaluation", "history_blocks", len(hist))
	recs, err := models.Eval(ctx, models.EvalConfig{
		TrainBlocks: trainBlocks,
		TestBlocks:  testBlocks,
		N:           n,
		Confidence:  confidence,
	}, hist)
	if err != nil {
		return fmt.Errorf("eval: %w", err)
	}

	if len(recs) == 0 {
		log.InfoContext(ctx, "no eval records produced")
		return nil
	}

	batch, err := conn.PrepareBatch(ctx, `
		INSERT INTO marts.model_eval
			(eval_date, model_name, n, confidence, window_blocks,
			 hit_rate, avg_cost_gwei, eval_blocks, _version)`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, r := range recs {
		if err := batch.Append(
			r.EvalDate,
			r.ModelName,
			int32(r.N), //nolint:gosec
			r.Confidence,
			int32(r.WindowBlocks), //nolint:gosec
			r.HitRate,
			r.AvgCostGwei,
			int32(r.EvalBlocks), //nolint:gosec
			r.EvalDate.UnixMilli(),
		); err != nil {
			return fmt.Errorf("batch append: %w", err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("batch send: %w", err)
	}

	log.InfoContext(ctx, "eval-models complete", "records", len(recs))
	return nil
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
