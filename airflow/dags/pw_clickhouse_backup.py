"""pw_clickhouse_backup — nightly ClickHouse database backup.

Runs at 03:00 UTC daily.  Backs up the raw and meta databases to the
ClickHouse backup disk configured in infra/clickhouse/config.d/pw-config.xml.

Requires env var CLICKHOUSE_BACKUP_DISK (default: "backups").  The disk
must be configured in ClickHouse as a LocalDisk or S3 backup disk; see
infra/clickhouse/config.d/pw-config.xml for the "backups" disk definition.

Backup names include the ISO date so each run produces a new snapshot.
Old snapshots are not automatically pruned; manage retention separately.
"""
from __future__ import annotations

import os
from datetime import datetime

from airflow.decorators import dag, task
from airflow.operators.python import get_current_context

from _shared import DEFAULT_ARGS


@dag(
    dag_id="pw_clickhouse_backup",
    schedule="0 3 * * *",
    start_date=datetime(2026, 1, 1),
    catchup=False,
    tags=["pga", "ops", "backup"],
    default_args=DEFAULT_ARGS,
    doc_md=__doc__,
)
def pw_clickhouse_backup() -> None:
    @task
    def backup_databases() -> None:
        import clickhouse_connect  # noqa: PLC0415

        context = get_current_context()
        ds: str = context["ds"]  # YYYY-MM-DD
        disk = os.environ.get("CLICKHOUSE_BACKUP_DISK", "backups")

        client = clickhouse_connect.get_client(
            host=os.environ.get("CLICKHOUSE_HOST", "clickhouse"),
            port=int(os.environ.get("CLICKHOUSE_PORT", "8123")),
            username=os.environ.get("CLICKHOUSE_USER", "default"),
            password=os.environ.get("CLICKHOUSE_PASSWORD", ""),
            query_limit=0,
            connect_timeout=30,
            send_receive_timeout=3600,  # backups can be slow
        )

        for db in ("raw", "meta"):
            backup_name = f"{db}_{ds.replace('-', '')}"
            sql = f"BACKUP DATABASE `{db}` TO Disk('{disk}', '{backup_name}')"
            print(f"running: {sql}")
            client.command(sql)
            print(f"backup complete: {backup_name}")

    backup_databases()


pw_clickhouse_backup()
