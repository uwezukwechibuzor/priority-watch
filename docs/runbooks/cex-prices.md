# Runbook: CEX Price Feed (Binance 1s klines)

## Overview

The CEX price feed downloads Binance 1s OHLCV klines into `raw.cex_klines_1s`.
It has two modes:

| Mode | Command | Schedule |
|------|---------|----------|
| Historical backfill | `pw-ingester cex backfill` | Manual / Airflow `pga_cex_catchup` |
| Live follow | `pw-ingester cex follow` | Continuous (follower service) |

Default symbols: **ETHUSDT, ARBUSDT, BTCUSDT**.

---

## Backfill

```bash
pw-ingester cex backfill \
  --symbols ETHUSDT,ARBUSDT,BTCUSDT \
  --from 2026-09-01 \
  --to   2026-09-30
```

For each symbol × calendar day, the ingester:
1. Downloads `<SYMBOL>-1s-<DATE>.zip` from `data.binance.vision`
2. Downloads the matching `.CHECKSUM` file and verifies SHA-256
3. Parses the CSV (handles both ms and μs `open_time` timestamps)
4. Loads rows into `raw.cex_klines_1s` via `BatchWriter`
5. Updates `meta.source_status` on completion

### Error: HTTP 404

```
binance backfill ETHUSDT 2026-09-01: fetch zip: HTTP 404
```

The daily file is not yet available (Binance publishes the previous day's file
around 08:00 UTC). Wait and retry, or omit that date from the range.

### Error: checksum mismatch

```
checksum mismatch: got <actual> want <expected>
```

The downloaded zip was corrupted in transit. Re-run the backfill for that date;
the command is idempotent (`ReplacingMergeTree(_version)` deduplicates).

### Error: unreachable data.binance.vision

```
fetch zip: Get "https://data.binance.vision/...": dial tcp: ...
```

Check outbound internet connectivity from the host. If running inside Docker,
ensure the container has external access. The backfill can be resumed from any
intermediate date; it does not re-download already-loaded days because
ClickHouse deduplicates by `(venue, symbol, open_time)`.

---

## Live Follow

```bash
pw-ingester cex follow --symbols ETHUSDT,ARBUSDT,BTCUSDT
```

Connects to `wss://stream.binance.com:9443/stream` combined stream. Only
**finalized** klines (`is_final = true`) are written; the final kline for each
second is published at the close of the 1s window.

Reconnection uses exponential backoff (1 s → 60 s, infinite retries) so
transient disconnections are handled automatically.

### Error: failed to connect

```
dial binance WS: dial tcp: ...
```

Binance rate-limits aggressive reconnectors. The exponential backoff prevents
this in normal operation. If the error persists, check:
- Outbound WSS (port 9443) is allowed
- Binance API status: https://www.binance.com/en/support

### Gap detection

If the follower is offline for more than `FOLLOWER_GAP_THRESHOLD` blocks, the
block follower will trigger a chain backfill. For CEX prices, the Airflow DAG
`pga_cex_catchup` runs daily and fills any gaps between the follower's last
seen position and yesterday.

---

## Manual gap repair

To fill a specific missing date range:

```bash
docker exec pw-follower pw-ingester cex backfill \
  --symbols ETHUSDT,ARBUSDT,BTCUSDT \
  --from 2026-10-01 \
  --to   2026-10-03
```

The command is safe to re-run; higher `_version` values overwrite stale rows.

---

## Symbol mapping

| Token | Binance pair |
|-------|-------------|
| WETH / ETH | ETHUSDT |
| WBTC / BTC | BTCUSDT |
| ARB | ARBUSDT |
| USDC, USDT, DAI, FRAX, LUSD, USDCE | 1.00 USD (no feed needed) |

To add a new symbol, update `internal/cex/symbol.go` and the `--symbols`
default in `cmd/pw-ingester/main.go`.

---

## Verification

```sql
-- Row count per symbol per day
SELECT symbol, toDate(open_time) AS day, count() AS rows
FROM raw.cex_klines_1s FINAL
GROUP BY symbol, day
ORDER BY symbol, day DESC
LIMIT 30;

-- Check for gaps (days with < 86400 rows indicate missing seconds)
SELECT symbol, toDate(open_time) AS day, count() AS rows
FROM raw.cex_klines_1s FINAL
GROUP BY symbol, day
HAVING rows < 86000
ORDER BY symbol, day;
```
