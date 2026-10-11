# Runbook: Reorg Recovery

Arbitrum One reorgs are very rare (Nitro + L1 finality) but can occur.

## Detection
The live follower detects reorgs by comparing the incoming block's `parentHash`
against the stored hash of the previous block. On mismatch, it logs:

```
level=warn msg="reorg detected" common_ancestor=<N> depth=<D>
```

Prometheus metric: `pw_follower_reorgs_total{depth="<D>"}`.

## Recovery

### Shallow reorgs (depth <= 5)
The follower handles these automatically. It rolls back the cursor to the common
ancestor and re-ingest the reorganised blocks. No manual action required.

### Deep reorgs (depth > 5)
1. Stop the follower: `docker compose stop pw-follower`
2. Identify common ancestor block N.
3. Delete orphaned data:
```sql
DELETE FROM raw.blocks WHERE block_number > N;
DELETE FROM raw.transactions WHERE block_number > N;
DELETE FROM raw.receipts WHERE block_number > N;
DELETE FROM raw.logs WHERE block_number > N;
```
4. Reset the cursor:
```sql
INSERT INTO meta.ingest_cursors (cursor_name, last_block, updated_at)
VALUES ('follower', N, now64(3));
```
5. Restart the follower: `docker compose start pw-follower`
6. Verify: `make smoke`

## Note
ClickHouse `ReplacingMergeTree` ensures idempotency — re-ingesting reorganised
blocks will overwrite orphaned rows with the correct canonical chain data.
