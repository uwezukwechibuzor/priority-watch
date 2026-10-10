package cex

import (
	"context"
	"time"
)

// Kline is a single 1-second OHLCV bar from a CEX venue.
type Kline struct {
	Venue    string
	Symbol   string
	OpenTime time.Time
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
	Trades   uint32
}

// PriceSource is implemented by any CEX that provides 1s kline data.
// Binance is the default implementation; additional venues can be added without
// changing callers.
type PriceSource interface {
	// Name returns the venue identifier stored in raw.cex_klines_1s (e.g. "binance").
	Name() string

	// Backfill downloads historical 1s klines for the given symbol and inclusive
	// UTC date range, calling fn once per day's worth of klines. fn must not
	// retain the slice after returning.
	Backfill(ctx context.Context, symbol string, from, to time.Time, fn func([]Kline) error) error

	// Follow streams live 1s klines for the given symbols. Only finalized
	// (is_final) klines are delivered. Reconnects with exponential backoff.
	// Blocks until ctx is cancelled or fn returns a non-nil error.
	Follow(ctx context.Context, symbols []string, fn func(Kline) error) error
}
