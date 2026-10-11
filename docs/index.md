# Priority Watch

Priority Watch is an independent, verifiable, open-source analytics platform for
**Arbitrum One's Priority Gas Auction (PGA)** ordering policy, which replaced
Timeboost around 24–25 September 2026.

## What it measures

| Metric group | What we track |
|---|---|
| **Revenue** | Priority-fee revenue by block, entity and era; 97/3 Treasury split |
| **Ordering** | Contested blocks, 2nd/1st fee ratio, HHI concentration |
| **User cost** | Base-fee evolution, revert rates, tip distribution across transaction types |
| **Opportunity** | CEX-DEX spread, markout P&L, atomic-arbitrage capture rate |
| **Pricing** | Four fee-recommendation models with held-out evaluation and a strategy backtester |

## Why it exists

The Entropy research (Frontier Research, Mar 2026) found that Timeboost captured
only **1.74%** of atomic-arbitrage revenue on Arbitrum One, vs. 28.83% on Ethereum
and 7.34% on Base. Three entities won ~97% of Timeboost auctions. PGA replaces the
auction with a per-transaction priority-fee market. Priority Watch provides the data
infrastructure to measure whether PGA improves on those benchmarks.

## Quick start

```bash
git clone https://github.com/uwezukwechibuzor/priority-watch.git
cd priority-watch
cp .env.example .env   # add your ARBITRUM_RPC_URL
make up && make migrate && make backfill FROM=PGA_ACTIVATION_BLOCK TO=head
```

See [Run locally in the README](https://github.com/uwezukwechibuzor/priority-watch#run-locally)
for the full walkthrough.

## Data sources

| Source | Update cadence | Notes |
|--------|---------------|-------|
| Arbitrum One RPC | 250 ms per block | go-ethereum JSON-RPC + WebSocket |
| Binance 1s klines | 1s WebSocket | ETH/USDT pair |
| Balance snapshots | Hourly | ArbOwnerPublic precompile calls |

## Privacy and limitations

Priority Watch only observes **confirmed on-chain data**. Arbitrum One's private
mempool means arrival times and PGA round boundaries are **not observable**. See
[Limitations](limitations.md) for a full list of caveats.
