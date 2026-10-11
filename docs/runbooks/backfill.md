# Runbook: Block Backfill

## When to use
- Initial historical backfill after first deploy
- After an RPC outage causes blocks to be missed
- Re-ingesting a range after a schema migration

## Steps

### 1. Identify the gap
```sql
-- Find missing block numbers
SELECT arrayJoin(range(min_block, max_block + 1)) AS n
FROM (SELECT min(block_number) AS min_block, max(block_number) AS max_block FROM raw.blocks FINAL)
WHERE n NOT IN (SELECT block_number FROM raw.blocks FINAL)
LIMIT 100;
```

### 2. Run the backfill
```bash
# Backfill a specific range
make backfill FROM=<start_block> TO=<end_block>

# Backfill from PGA activation to head
make backfill FROM=$PGA_ACTIVATION_BLOCK TO=head

# Tune concurrency for large ranges
make backfill FROM=<start> TO=<end> CHUNK=5000 CONCURRENCY=8
```

### 3. Verify
```bash
make smoke
```

### 4. Re-run dbt
```bash
make dbt-build
```

## Notes
- Backfill is idempotent — safe to re-run over any range.
- On RPC rate-limit errors, reduce `CONCURRENCY`.
- On ClickHouse write errors, check disk space: `df -h` inside the container.
