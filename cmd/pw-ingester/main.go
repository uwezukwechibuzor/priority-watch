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
		log.InfoContext(ctx, "migrate: stub – task 3 implements schema migrations")
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
