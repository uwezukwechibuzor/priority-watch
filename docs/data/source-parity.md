# Source parity report (template)

Priority Watch can ingest Arbitrum One block data from three independent
providers. This document is the template for the parity report produced by
`scripts/source_parity`. The numbers below are placeholders — fill them in by
running the tool against a live range once ingestion credentials are available.

## Sources compared

| Source | Transport | Auth | Notes |
|--------|-----------|------|-------|
| SQD Portal | `POST /stream` NDJSON | none | Primary. Free, no key required. |
| Envio HyperSync | `POST /query` JSON | optional Bearer | Fallback. Guarantees `gas_used_for_l1`. |
| Arbitrum JSON-RPC | `eth_*` | provider key | Reference. Balance snapshots and task 8 samples. |

## How to generate

```bash
export SQD_PORTAL_URL=https://portal.sqd.dev/datasets/arbitrum-one
export ENVIO_HYPERSYNC_URL=https://arbitrum.hypersync.xyz
# optional: export ENVIO_API_TOKEN=...
# optional: export ARBITRUM_RPC_URL=...   (RPC comparison is a documented extension point)

go run ./scripts/source_parity --from 350000000 --count 2000 > docs/data/source-parity.md
```

The tool fetches the same `[from, from+count-1]` range from every configured
source, runs each block through `ingest.TransformBlock`, and compares the
resulting `raw.blocks` columns. It exits non-zero if any column disagrees.

## Columns compared

`block_hash`, `parent_hash`, `gas_used`, `gas_limit`, `base_fee_per_gas`,
`tx_count`, `l1_block_number`.

## Result (placeholder)

| Column | Mismatches |
|--------|-----------|
| block_hash | 0 |
| parent_hash | 0 |
| gas_used | 0 |
| gas_limit | 0 |
| base_fee_per_gas | 0 |
| tx_count | 0 |
| l1_block_number | 0 |

> Replace this table with live output. A non-empty "First mismatches" section
> indicates a provider discrepancy that must be investigated before trusting
> the differing source for that column.

## Known provider notes

- **`gas_used_for_l1`** — HyperSync returns this Arbitrum-specific receipt field
  natively. SQD returns it when present in the dataset; if a future SQD dataset
  revision drops it, HyperSync is the authoritative fallback.
- **`base_fee_per_gas`** — SQD encodes block numeric fields as hex strings;
  HyperSync may encode them as decimal strings. Both are normalised to
  `*big.Int` before comparison, so the encoding difference is not a mismatch.
