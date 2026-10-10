// pw-ingester is the Priority Watch ingestion binary.
// Subcommands: migrate, backfill, follow, cex, import-timeboost.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ClickHouse/clickhouse-go/v2"

	pwmigrations "github.com/uwezukwechibuzor/priority-watch/infra/clickhouse/migrations"
	"github.com/uwezukwechibuzor/priority-watch/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: pw-ingester <subcommand> [flags]")
		fmt.Fprintln(os.Stderr, "subcommands: migrate | backfill | follow | cex | import-timeboost")
		os.Exit(1)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	sub := flag.Arg(0)
	log.InfoContext(ctx, "starting", "subcommand", sub)

	switch sub {
	case "migrate":
		if err := runMigrate(ctx, log); err != nil {
			log.ErrorContext(ctx, "migrate failed", "err", err)
			os.Exit(1)
		}
	case "backfill":
		log.InfoContext(ctx, "backfill: stub – task 5 implements block backfill")
	case "follow":
		log.InfoContext(ctx, "follow: stub – task 6 implements live block follower")
	case "cex":
		log.InfoContext(ctx, "cex: stub – task 7 implements Binance klines ingestion")
	case "import-timeboost":
		log.InfoContext(ctx, "import-timeboost: stub – task 8 implements Timeboost baseline import")
	default:
		log.ErrorContext(ctx, "unknown subcommand", "sub", sub)
		os.Exit(1)
	}
}

func runMigrate(ctx context.Context, log *slog.Logger) error {
	addr := envOr("CLICKHOUSE_ADDR", "localhost:9000")
	user := envOr("CLICKHOUSE_USER", "default")
	pass := envOr("CLICKHOUSE_PASSWORD", "")

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Username: user,
			Password: pass,
		},
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
