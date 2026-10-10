# UI Reference — Priority Watch

> Binding UI specification. Every frontend task reads this before writing code.
> Translated from [timeboost.app](https://timeboost.app) screens to PGA.
> A **block** replaces a **round** everywhere.

---

## Visual Design Language

### Colour tokens
| Token | Hex | Usage |
|-------|-----|-------|
| `bg-base` | `#0b0c0e` | Page background |
| `bg-panel` | `#111318` | Card/panel background |
| `border` | `#1f2329` | Panel borders (1 px) |
| `text-muted` | `#6b7280` | Labels, secondary text |
| `text-primary` | `#f0f2f5` | Values, headings |
| `accent` | `#3b82f6` | Active tab, selected state, links |
| `green` | `#22c55e` | Positive delta, single-payer badge |
| `amber` | `#f59e0b` | Mid-ratio bar, warning |
| `red` | `#ef4444` | High ratio bar, error, revert |
| `purple` | `#a855f7` | Contested badge |

### Categorical entity palette (stable across sessions)
Assign colours from this ordered list by entity rank (most tips first).
`#3b82f6` `#f59e0b` `#a855f7` `#22c55e` `#ef4444` `#ec4899` `#14b8a6` `#f97316`

A coloured 8×8 px square precedes every entity name.

### Typography
- Labels: uppercase, letter-spacing 0.08em, 10–11 px, `text-muted`
- Values: tabular nums (`font-variant-numeric: tabular-nums`), 13–14 px, `text-primary`
- Table cells: 12–13 px, monospace for addresses and numbers
- KPI values: 24–28 px, semi-bold

### Spacing and density
- Panel padding: 16 px
- Table row height: 36 px
- Grid gap: 8 px

---

## Top Bar (persistent, fixed)

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│ priority watch  PGA   Blocks  Health  Pricing  Opportunity  Simulator          │
│                                        [Search blocks, txs, addresses…  ⌘K] ⚙ ?│
└─────────────────────────────────────────────────────────────────────────────────┘
```

- **Wordmark**: "priority watch" in `text-primary`, small `PGA` badge in `bg-panel` border `border`
- **Tabs**: `Blocks · Health · Pricing · Opportunity · Simulator`; active tab underlined in `accent`
- **Live pill**: green pulsing dot + head block number + relative time ("3s ago"), updates every 250 ms
- **Search**: full-width overlay on ⌘K / Ctrl-K; searches blocks (by number), txs (by hash),
  addresses (by 0x…), entities (by name); results grouped by type
- **Icons**: settings ⚙, help ?; both open right-side drawers

---

## Context Bar (below top bar, persistent)

```
< 10/10/2026 >   Source api.priority.watch   Network Arbitrum One
Live data 2s ago   Block time 250ms   PGA round 125ms
```

On `/health` only, add to the right:
```
Window [7d ▾]   Since 2026-09-24
```

| Element | Values / behaviour |
|---------|-------------------|
| Day picker | `< MM/DD/YYYY >` arrows; clicking date opens a calendar; defaults to today |
| Source | `api.<domain>` from `API_CORS_ORIGIN` env |
| Network | Always "Arbitrum One" |
| Live data | Time since last block ingested; red if > 2 s |
| Block time | Always "250ms" |
| PGA round | Always "125ms" |
| Window (health) | `7d / 30d / 90d / since activation`; default `30d` |
| Since | Activation date (from `PGA_ACTIVATION_BLOCK` env + chain timestamp) |

---

## `/blocks` Page

Clone of timeboost.app `/auctions`. A block replaces a round.

### KPI tiles (6, top row)

Each tile: label (uppercase muted), large value + unit, sparkline (last 24 h), subtext.

| Tile | Label | Value | Subtext |
|------|-------|-------|---------|
| 1 | BLOCKS | count | "of ~345,600 expected" (per day) |
| 2 | TIP REVENUE | X.XXX ETH | $X,XXX USD |
| 3 | AVG TIP / BLOCK | X.XXXX ETH | $X.XX USD |
| 4 | CONTESTED | XX.X% | "blocks where 2+ entities paid tips" |
| 5 | TOP PAYER | XX.X% | entity name with colour square |
| 6 | HHI | X,XXX | "Competitive / Moderate / High" |

HHI bands: < 1,500 Competitive (green), 1,500–2,500 Moderate (amber), > 2,500 High (red).

### Toolbar

```
[🔍 Search block, address, entity…]  [1h][24h][7d][30d][all]  [Contested only ☐]
[Filter by payer…]  [↻ Refresh]  [↓ Export CSV]
```

- Range buttons: mutually exclusive; active = `accent` background
- "Contested only": toggle; when on, table shows only contested blocks
- "Filter by payer…": text input; narrows table to rows matching entity name/address
- Refresh: re-fetches without full page reload
- Export: downloads the current filtered view as CSV

### Panels (3, below toolbar)

#### Panel 1: TIP REVENUE / TOP FEE time series

Toggle button: `TIP REVENUE` | `TOP FEE`

- **TIP REVENUE**: area chart; x = block time, y = total tip revenue per block (ETH)
- **TOP FEE**: area chart; x = block time, y = highest priority fee in block (gwei)
- Both: event markers from `config/events.yaml` as vertical dashed lines with a label tooltip
- Hover: crosshair tooltip showing block #, time, value

#### Panel 2: PAYER SHARE (horizontal bars)

Top 5 entities by tip revenue in the selected range.

```
[■] Entity A  ███████████████████░░░░░░  42.3%
[■] Entity B  ████████████░░░░░░░░░░░░░  28.1%
[■] Entity C  ███████░░░░░░░░░░░░░░░░░░  17.4%
[■] Entity D  ████░░░░░░░░░░░░░░░░░░░░░   8.9%
[■] Others    ██░░░░░░░░░░░░░░░░░░░░░░░   3.3%
```

Each bar: coloured square, entity name, filled bar (entity colour), percentage.
Click entity → filters table to that payer.

#### Panel 3: TIPPED TXS PER BLOCK histogram

X-axis: tipped tx count bins (0, 1–5, 6–10, 11–20, 21–50, 51+).
Y-axis: block count.
Bars coloured by type (blue = contested, green = single, grey = untipped).

### Block table

Columns (dense, 12 px):

| Column | Type | Notes |
|--------|------|-------|
| BLOCK | number link | Click → block explorer |
| TIME | hh:mm:ss · relative | "14:23:01 · 5m ago" |
| TXS | integer | Total tx count |
| TOP PAYER | coloured square + name/address | Truncated address if unlabelled |
| TOP FEE | X.XX gwei + $X.XX | Highest priority fee in block |
| TIP REVENUE | X.XXXX ETH + $X.XX | Sum of all priority fees |
| 2ND/1ST | coloured bar + % | second-highest ÷ highest; green < 50%, amber 50–85%, red > 85% |
| TIPPED | integer | Count of txs with priority fee > 0 |
| TYPE | badge | `contested` (purple) / `single` (blue) / `untipped` (grey) |

- Default sort: block DESC (newest first)
- Sticky header; virtual scrolling for large ranges
- Row click: opens right-side **Block Drawer**

### Block Drawer (right side, 480 px wide)

Slides in from the right. Shows:
- Block number, timestamp, base fee, block gas used / limit
- Transactions table: rank, hash (truncated), from (entity name or address), priority fee (gwei), gas used, status (✓/✗)
- "View on explorer" link
- Close button (×) or Escape key

---

## `/health` Page

Clone of timeboost.app `/health`.

### KPI tiles (6)

| Tile | Label | Value |
|------|-------|-------|
| 1 | REVENUE 30D | X.XX ETH |
| 2 | BLOCKS 30D | XX,XXX (XX.X% of expected) |
| 3 | HHI 30D | X,XXX |
| 4 | CONTESTED 30D | XX.X% |
| 5 | TOP PAYER | entity name + share % |
| 6 | REVERT RATE | XX.X% (tipped txs only) |

### Tab panel (right side of page)

Tabs: `Overview · Trends · Payers · Treasury · Compare · Sources · Pipeline`

#### Overview tab

Four panels stacked:

**1. REVENUE + THROUGHPUT** (dual-axis area + line)
- Left y-axis (area): daily tip revenue in ETH
- Right y-axis (line): tipped tx count per day
- X-axis: date
- Event markers from `config/events.yaml`

**2. CONCENTRATION (HHI)**
- Line chart: daily HHI over selected window
- Horizontal shaded bands: green < 1,500, amber 1,500–2,500, red > 2,500
- Tooltip: HHI value + interpretation

**3. COMPETITION + FEE BAND**
- Left y-axis (bars): distinct entities with tipped txs per block (daily avg)
- Right y-axis (area band): p10–p90 priority fee range (gwei)
- Shows widening/narrowing of fee distribution over time

**4. PAYERS LEADERBOARD** (table)

| Column | Notes |
|--------|-------|
| ENTITY | colour square + name |
| TIPS | ETH + sparkline (last 30 d) |
| SHARE | % of total tip revenue |
| REVERT RATE | % of tipped txs reverted |
| AVG POSITION | average tx rank in block (lower = earlier) |

#### Trends tab

Time series for every KPI metric over the selected window: revenue, contested %, HHI, avg tip, revert rate. Each as a small multiples grid.

#### Payers tab

Full payer leaderboard (all entities, paginated). Same columns as Overview leaderboard plus: BLOCKS APPEARED, TOP FEE MED, CONTESTED RATE.

#### Treasury tab

- Cumulative tip revenue area chart (ETH, since PGA activation)
- 97% Treasury / 3% Developer Guild split always shown
- Projected annualised run rate
- Link to on-chain DAO vote records

#### Compare tab

Side-by-side metric comparison: Timeboost era vs. PGA era.

Metrics: avg daily revenue, contested %, HHI, revert rate, capture rate.

Shows Entropy benchmark numbers (1.74% capture, 28.83% ETH, 7.34% Base) with source link.

#### Sources tab

Table of data sources: chain RPC, Binance klines, timeboost.app Parquet.

| Column | Values |
|--------|--------|
| SOURCE | name |
| STATUS | 🟢 live / 🟡 lagging / 🔴 down |
| LAG | seconds behind head |
| LAST BLOCK | block number |
| LAST SEEN | relative time |

#### Pipeline tab

Airflow DAG status table. Last run time, duration, status per DAG.

---

## `/pricing` Page

Clone of timeboost.app `/pricing`.

### Controls (left sidebar)

- **Block selector**: jump to specific block number
- **Model selector**: dropdown `Naive Percentile · EMA · Quantile Regression · ARIMA`
- **Parameter sliders**: per-model (e.g. percentile, window, alpha)
- **"Run"** button

### Main area

- **Recommendation**: large display — "Suggested priority fee: X.XX gwei" with reasoning trace
  (e.g. "p75 of last 100 blocks = 2.3 gwei; +20% safety margin = 2.76 gwei")
- **4-model side-by-side**: four small cards, each showing the model name, its recommendation,
  and a mini time-series of its historical suggestions vs. actual top fees

### Fee distribution panel

Histogram of priority fees in the selected block range with the recommendation overlaid.

---

## `/opportunity` Page

Clone of timeboost.app `/opportunity`.

### Panels

**1. Spread heatmap**
- X-axis: hour of day, Y-axis: day of week
- Colour: median spread (price impact) intensity
- Tooltip: median spread + block count

**2. Spread histogram**
- X-axis: spread bins (gwei), Y-axis: block count
- Overlaid line: cumulative %

**3. 6-horizon markout scatter**
- One scatter per horizon (1, 5, 10, 30, 60, 120 blocks)
- X: tip paid (gwei), Y: markout PnL (ETH)
- Coloured by entity; trend line

**4. Entity position leaderboard**
Table: entity, avg tip paid (gwei), avg tx position in block, position rank, tip rank, delta (tip rank − position rank).
Sorted by delta DESC (who over/under-pays relative to their position).

**5. Execution table**
Block, time, entity, tip paid, position, spread at execution, 1-block markout.
Filters: entity, date range, min spread.

---

## `/simulator` Page

Clone of timeboost.app `/simulator`.

### Controls

- **Strategy picker**: dropdown or radio (Flat fee, Percentile-based, Adaptive EMA)
- **Block / date range**: start–end block or date picker
- **Add run** button (up to 4 parallel runs)
- Each run has a colour + label (editable)

### Charts

**Cumulative P&L curve**
- X-axis: block number or date
- Y-axis: cumulative simulated tip spent vs. priority positions won
- One line per run, in run colour
- Toggle: "net P&L" (positions won value − tips spent) vs. "tips spent" vs. "positions won"

**Per-run summary cards** (below chart)
- Total tips spent, positions won, win rate, net P&L, Sharpe (if enough data)

---

## Event Markers (all charts)

Every time-series chart renders vertical dashed lines from `config/events.yaml`:

| Event | Colour | Label |
|-------|--------|-------|
| Timeboost launch | amber | "Timeboost →" |
| ArbOS 60 Elara | grey | "ArbOS 60" |
| PGA activation | green | "PGA →" |
| Security Council pause | red | "SC Pause" |

Clicking a marker opens a tooltip with: event name, date, description, source link.

---

## Shared Behaviours

### Deep-linking
Every view encodes its full state in the URL:
- `/blocks?range=24h&day=2026-10-10&payer=0xabc&contested=true&block=12345678`
- `/health?window=30d&tab=payers`
- `/pricing?model=ema&block=12345678`

### Loading states
- Skeleton shimmer (same shape as content) on first load
- Spinner overlay on re-fetch (content stays visible)

### Empty states
- "No data for this range" with a block range suggestion

### Error states
- Red banner: "Failed to load — [Retry]"
- API errors show the error code and message from the JSON response

### Responsive behaviour
- Min width 1280 px (desktop analytics tool; no mobile layout)
- Sidebar collapses to icon rail below 1440 px

### Accessibility
- All colour-coded information also encoded in text (badge labels, tooltip text)
- `aria-label` on all icon buttons
- Table `<caption>` elements
