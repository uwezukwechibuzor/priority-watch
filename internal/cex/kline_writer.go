package cex

import (
	"context"
	"time"

	"github.com/uwezukwechibuzor/priority-watch/internal/store"
)

// InsertKlineSQL is the INSERT statement for raw.cex_klines_1s.
// Column order must match AppendKline.
const InsertKlineSQL = `INSERT INTO raw.cex_klines_1s
	(venue, symbol, open_time, open, high, low, close, volume, trades, _version)`

// AppendKline writes a single Kline to w using now.UnixMilli() as the
// ReplacingMergeTree version.
func AppendKline(ctx context.Context, w *store.BatchWriter, k Kline, now time.Time) error {
	return w.Append(ctx,
		k.Venue,
		k.Symbol,
		k.OpenTime,
		k.Open,
		k.High,
		k.Low,
		k.Close,
		k.Volume,
		k.Trades,
		uint64(now.UnixMilli()), //nolint:gosec
	)
}
