# Storage Capacity and Backfill Performance

## Methodology

Estimates are based on the Arbitrum One row profile measured during a 10,000-block
backfill test of blocks around the PGA activation epoch (~Sep 2026).
Arbitrum One produces one block every ~250 ms, yielding ~345,600 blocks/day.

## Row counts per block (typical)

| Table             | Rows/block | Notes                                    |
|-------------------|-----------|------------------------------------------|
| raw.blocks        | 1         | always                                   |
| raw.transactions  | ~70       | average over 10,000-block sample         |
| raw.receipts      | ~70       | 1:1 with transactions                    |
| raw.logs          | ~30       | DEX + protocol events (filtered)         |

## Compressed storage per block (ZSTD L1, typical)

| Table             | bytes/block | GB / 1M blocks |
|-------------------|-------------|----------------|
| raw.blocks        | ~200        | 0.20           |
| raw.transactions  | ~4,200      | 4.20           |
| raw.receipts      | ~2,100      | 2.10           |
| raw.logs          | ~900        | 0.90           |
| **Total raw**     | **~7,400**  | **7.40**       |

## Full-history estimates (Timeboost launch Apr 2025 → present)

Assuming ~18 months of history (~16 M blocks):

| Layer   | Estimated size |
|---------|---------------|
| raw.*   | ~118 GB       |
| dbt mart aggregates | ~5–10 GB |
| **Total** | **~125–130 GB** |

A standard 500 GB SSD is sufficient for the full history plus growth headroom.

## Backfill performance (measured)

All measurements from a single-node development setup
(M-series Mac, localhost ClickHouse, Arbitrum public RPC):

| Parameter        | Value                                      |
|------------------|--------------------------------------------|
| Concurrency      | 4 workers                                  |
| Chunk size       | 2,000 blocks                               |
| RPC              | Arbitrum One public endpoint               |
| Rate limit       | 25 RPS                                     |
| Blocks/second    | ~8–12 (RPC-bound)                          |
| Time for 10k blocks | ~15–20 min                            |
| Time for 16M blocks (full history) | ~14–19 days (RPC-bound) |

**Bottleneck**: `eth_getBlockByNumber` + `eth_getBlockReceipts` calls per block.
A dedicated RPC node at 100+ RPS would reduce full-history backfill to ~3–4 days.

## Resumability

Each backfill chunk (default 2,000 blocks) is tracked in `meta.ingest_cursors`.
Interrupting and restarting a backfill resumes from the last incomplete chunk;
individual blocks within a chunk are re-fetched idempotently via
`ReplacingMergeTree(_version)`.

## Recommendations

- Use `--concurrency 8` and a dedicated RPC at 50+ RPS for production backfills.
- Run backfills on the Airflow `pga_backfill` DAG (task 13) which retries failed chunks automatically.
- Monitor `pw_backfill_blocks_per_second` gauge in Grafana for live throughput.
