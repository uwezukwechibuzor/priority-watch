// pw-api is the Priority Watch REST API binary.
// Serves /api/v1/* and OpenAPI docs at /docs.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ClickHouse/clickhouse-go/v2"

	"github.com/uwezukwechibuzor/priority-watch/internal/api"
	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
	"github.com/uwezukwechibuzor/priority-watch/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log := newLogger()

	cfg, err := config.LoadAPI()
	if err != nil {
		log.ErrorContext(ctx, "config error", "err", err)
		os.Exit(1)
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.ClickHouseAddr},
		Auth: clickhouse.Auth{
			Username: cfg.ClickHouseROUser,
			Password: cfg.ClickHouseROPassword,
		},
		// readonly=2 and max_execution_time are set via the pw_reader profile in users.d/pw-profiles.xml
	})
	if err != nil {
		log.ErrorContext(ctx, "clickhouse open", "err", err)
		os.Exit(1)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.Ping(ctx); err != nil {
		log.ErrorContext(ctx, "clickhouse ping failed", "err", err)
		// Don't exit — the DB may become available shortly; the /healthz
		// check will reflect the state on first request.
		log.WarnContext(ctx, "starting without clickhouse connectivity — retrying on requests")
	}

	srv := api.New(cfg, conn, log)

	if cfg.ArbitrumRPCURL != "" {
		rpcClient, cerr := chain.New(ctx, chain.ClientConfig{
			PrimaryURL:   cfg.ArbitrumRPCURL,
			FallbackURLs: cfg.ArbitrumRPCFallbackURLs,
			RPS:          cfg.ArbitrumRPCRPS,
			Logger:       log,
		})
		if cerr != nil {
			log.WarnContext(ctx, "chain client init failed — chain head unavailable", "err", cerr)
		} else {
			srv.WithHeadFunc(func(ctx context.Context) (uint64, error) {
				blk, err := rpcClient.Head(ctx)
				if err != nil {
					return 0, err
				}
				return uint64(blk.Number), nil
			})
		}
	}

	if err := srv.Run(ctx); err != nil {
		log.ErrorContext(ctx, "server error", "err", err)
		os.Exit(1)
	}
}

func newLogger() *slog.Logger {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func init() {
	// Ensure the binary can be found — this is a no-op but satisfies the
	// linker when the main package imports are correct.
	_ = fmt.Sprintf
}
