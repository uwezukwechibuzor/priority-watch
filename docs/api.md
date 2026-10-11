# API Guide

Base URL: `https://api.priority.watch/api/v1`

Interactive documentation (Scalar UI): `https://api.priority.watch/docs`

OpenAPI 3.1 spec: `https://api.priority.watch/openapi.json`

## Authentication

The public API is unauthenticated. Rate limit: **60 requests per minute per IP**.

## Common parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `range` | string | Time range: `1h`, `24h`, `7d`, `30d`, `all` |
| `window` | string | Window size: `7d`, `30d`, `90d`, `all` |
| `cursor` | string | Pagination cursor (from `pagination.next_cursor` in previous response) |
| `limit` | int | Page size (default 50, max 200) |

## Response conventions

- **Wei** values are decimal strings (e.g. `"1500000000000000"`) — never floats.
- **Timestamps** are RFC 3339 UTC strings.
- **Addresses** are lowercase hex with `0x` prefix.
- Paginated responses wrap in `{ "data": [...], "pagination": { "next_cursor": "..." } }`.
- `null` cursor means no more pages.

## Endpoint groups

### Blocks

```
GET /api/v1/blocks?range=7d&limit=50&cursor=<cursor>
```
Returns a cursor-paginated list of blocks with PGA analytics fields.

```
GET /api/v1/blocks/kpi?range=7d
```
Returns KPI summary (tip revenue, contested %, HHI, revert rate).

```
GET /api/v1/blocks/health?window=30d
```
Returns daily health metrics for the selected window.

### Pricing

```
GET /api/v1/pricing/recommend?block=0&n=1&confidence=0.8&window=200&model=floor-estimate
```
Returns fee recommendations from one or all models.

| Field | Description |
|-------|-------------|
| `recommended_tip_gwei` | Recommended priority fee in Gwei |
| `interval_lo_gwei` / `interval_hi_gwei` | Confidence interval |
| `hit_rate` | Historical fraction of blocks where this tip would have ranked <= N |
| `cost_est_wei` | Estimated cost at 500,000 gas (typical arbitrage) |
| `reasoning` | Array of strings explaining the recommendation |

### Backtest

```
POST /api/v1/backtest
Content-Type: application/json

{ "strategy": "percentile-of-recent", "from_block": 0, "to_block": 0, "config": { "percentile": 0.8 } }
```
Starts an async backtest. Returns `{ "run_id": "..." }`.

Poll status: `GET /api/v1/backtest/{run_id}`

Get results: `GET /api/v1/backtest/{run_id}/results`

### Opportunity

```
GET /api/v1/opportunity/spread/heatmap
GET /api/v1/opportunity/spread/histogram
GET /api/v1/opportunity/markouts?range=7d
GET /api/v1/opportunity/position?range=7d
GET /api/v1/execution/quality?range=7d
```

### Treasury

```
GET /api/v1/treasury?window=30d
```
Returns daily 97/3 split breakdown and cumulative totals.

### Compare

```
GET /api/v1/compare?era=both
```
Returns side-by-side Timeboost vs PGA era comparison metrics.
