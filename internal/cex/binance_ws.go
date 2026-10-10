package cex

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gorilla/websocket"
)

const binanceWSBase = "wss://stream.binance.com:9443/stream"

// wsDial is the dial function signature; injectable for tests.
type wsDial func(ctx context.Context, url string) (*websocket.Conn, error)

// boFactory creates a new BackOff on each Follow attempt; injectable for tests.
type boFactory func() backoff.BackOff

type wsKlineMsg struct {
	Stream string `json:"stream"`
	Data   struct {
		EventType string `json:"e"`
		Symbol    string `json:"s"`
		K         struct {
			OpenTime int64  `json:"t"`
			Open     string `json:"o"`
			High     string `json:"h"`
			Low      string `json:"l"`
			Close    string `json:"c"`
			Volume   string `json:"v"`
			Trades   int    `json:"n"`
			IsFinal  bool   `json:"x"`
		} `json:"k"`
	} `json:"data"`
}

// Follow subscribes to Binance combined kline_1s streams for the given symbols.
// Only closed (is_final) klines are delivered to fn. Reconnects with exponential
// backoff until ctx is cancelled or fn returns a non-nil error.
func (b *BinanceSource) Follow(ctx context.Context, symbols []string, fn func(Kline) error) error {
	streams := make([]string, len(symbols))
	for i, s := range symbols {
		streams[i] = strings.ToLower(s) + "@kline_1s"
	}
	url := binanceWSBase + "?streams=" + strings.Join(streams, "/")

	newBO := b.newBO
	if newBO == nil {
		newBO = func() backoff.BackOff {
			bo := backoff.NewExponentialBackOff()
			bo.InitialInterval = 1 * time.Second
			bo.MaxInterval = 60 * time.Second
			bo.MaxElapsedTime = 0 // retry forever
			return bo
		}
	}

	return backoff.Retry(func() error {
		if err := ctx.Err(); err != nil {
			return backoff.Permanent(err)
		}
		err := b.runWS(ctx, url, fn)
		if err != nil {
			if ctx.Err() != nil {
				return backoff.Permanent(ctx.Err())
			}
			b.log.Warn("binance WS disconnected, retrying", "err", err)
			return err
		}
		return nil
	}, backoff.WithContext(newBO(), ctx))
}

func (b *BinanceSource) runWS(ctx context.Context, url string, fn func(Kline) error) error {
	var (
		conn *websocket.Conn
		err  error
	)
	if b.ws != nil {
		conn, err = b.ws(ctx, url)
	} else {
		conn, _, err = websocket.DefaultDialer.DialContext(ctx, url, nil) //nolint:bodyclose
	}
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Close the connection when ctx is cancelled so ReadMessage unblocks.
	go func() {
		<-ctx.Done()
		_ = conn.Close() //nolint:errcheck
	}()

	// Binance sends server-side pings roughly every 3 minutes; gorilla/websocket
	// automatically responds with a pong. We extend the deadline on each pong to
	// detect stale connections without a 5-minute hard timeout.
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Minute)); err != nil {
		return fmt.Errorf("set read deadline: %w", err)
	}
	conn.SetPongHandler(func(_ string) error {
		return conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	})

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read: %w", err)
		}

		var km wsKlineMsg
		if jerr := json.Unmarshal(msg, &km); jerr != nil {
			continue
		}
		if km.Data.EventType != "kline" || !km.Data.K.IsFinal {
			continue
		}

		k := klineFromWS(km)
		if err := fn(k); err != nil {
			return backoff.Permanent(err)
		}
	}
}

func klineFromWS(km wsKlineMsg) Kline {
	open, _ := strconv.ParseFloat(km.Data.K.Open, 64)
	high, _ := strconv.ParseFloat(km.Data.K.High, 64)
	low, _ := strconv.ParseFloat(km.Data.K.Low, 64)
	closeV, _ := strconv.ParseFloat(km.Data.K.Close, 64)
	vol, _ := strconv.ParseFloat(km.Data.K.Volume, 64)
	return Kline{
		Venue:    binanceName,
		Symbol:   km.Data.Symbol,
		OpenTime: time.UnixMilli(km.Data.K.OpenTime).UTC(),
		Open:     open,
		High:     high,
		Low:      low,
		Close:    closeV,
		Volume:   vol,
		Trades:   uint32(km.Data.K.Trades), //nolint:gosec
	}
}
