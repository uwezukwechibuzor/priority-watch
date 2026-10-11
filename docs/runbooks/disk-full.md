# Runbook: ClickHouse Disk Full

## Detection
- Grafana alert: ClickHouse disk usage > 85%
- Ingester log: `level=error msg="clickhouse: no space left on device"`

## Immediate response

### 1. Check disk usage
```bash
docker exec clickhouse df -h /var/lib/clickhouse
```

### 2. Check table sizes
```sql
SELECT
    database,
    table,
    formatReadableSize(sum(bytes_on_disk)) AS size
FROM system.parts
WHERE active
GROUP BY database, table
ORDER BY sum(bytes_on_disk) DESC
LIMIT 20;
```

### 3. Free space immediately
```sql
-- Trigger merge to compact old parts (frees deduplicated rows)
OPTIMIZE TABLE raw.transactions FINAL;
OPTIMIZE TABLE raw.logs FINAL;
```

### 4. Delete old backups
```bash
docker exec clickhouse ls -lh /var/lib/clickhouse/backups/
# Delete backups older than 30 days
docker exec clickhouse find /var/lib/clickhouse/backups/ -mtime +30 -delete
```

### 5. Expand volume (if needed)
Resize the Docker volume or attach additional storage following your cloud
provider's instructions. Update `docker-compose.yml` to mount the new path.

## Prevention
- Set a Grafana alert at 70% disk usage for early warning.
- Run `OPTIMIZE TABLE` weekly in the nightly DAG.
- Store backups on a separate disk (configure `CLICKHOUSE_BACKUP_DISK` in `.env`).
