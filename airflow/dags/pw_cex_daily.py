"""pw_cex_daily — daily Binance klines catch-up and gap check.

Runs at 01:00 UTC daily:
  1. Backfill yesterday's 1s klines from data.binance.vision (idempotent).
  2. Gap check — queries ClickHouse to confirm no second-level gaps remain
     in raw.cex_klines_1s for the last 48 hours.  Fails the task (and
     triggers the Slack alert) if gaps > allowed_gap_seconds.

The live WebSocket follower (pw-ingester cex follow) keeps the feed
current; this job is the safety net for any WebSocket downtime.
"""
from __future__ import annotations

import os
from datetime import datetime, timedelta, timezone

from airflow.decorators import dag, task
from airflow.operators.bash import BashOperator
from airflow.operators.python import get_current_context

from _shared import DEFAULT_ARGS

_SYMBOLS = "ETHUSDT,ARBUSDT,BTCUSDT"
_ALLOWED_GAP_SECONDS = 300  # tolerate up to 5 minutes of gaps before alerting


@dag(
    dag_id="pw_cex_daily",
    schedule="0 1 * * *",
    start_date=datetime(2026, 1, 1),
    catchup=False,
    tags=["pga", "cex", "binance"],
    default_args=DEFAULT_ARGS,
    doc_md=__doc__,
)
def pw_cex_daily() -> None:
    # Use Airflow's ds_nodash macro to pass yesterday's date.
    cex_backfill = BashOperator(
        task_id="cex_backfill_yesterday",
        bash_command=(
            "pw-ingester cex backfill"
            f" --symbols {_SYMBOLS}"
            " --from {{ macros.ds_add(ds, -1) }}"
            " --to {{ macros.ds_add(ds, -1) }}"
        ),
    )

    @task
    def gap_check() -> None:
        import clickhouse_connect

        client = clickhouse_connect.get_client(
            host=os.environ.get("CLICKHOUSE_HOST", "clickhouse"),
            port=int(os.environ.get("CLICKHOUSE_PORT", "8123")),
            username=os.environ.get("CLICKHOUSE_USER", "default"),
            password=os.environ.get("CLICKHOUSE_PASSWORD", ""),
        )

        # Find the largest consecutive gap (in seconds) over the last 48 hours
        # for the primary ETHUSDT feed.
        query = """
            WITH ordered AS (
                SELECT open_time,
                       neighbor(open_time, 1) AS next_open_time
                FROM raw.cex_klines_1s FINAL
                WHERE symbol = 'ETHUSDT'
                  AND venue = 'binance'
                  AND open_time >= now() - INTERVAL 48 HOUR
                ORDER BY open_time
            )
            SELECT max(dateDiff('second', open_time, next_open_time)) AS max_gap_seconds
            FROM ordered
            WHERE next_open_time > open_time
        """
        result = client.query(query)
        max_gap = result.first_row[0] if result.result_rows else 0
        print(f"max CEX gap in last 48 h: {max_gap}s (threshold: {_ALLOWED_GAP_SECONDS}s)")
        if max_gap > _ALLOWED_GAP_SECONDS:
            raise ValueError(
                f"CEX kline gap too large: {max_gap}s > {_ALLOWED_GAP_SECONDS}s — "
                "check WebSocket follower or trigger a manual cex backfill"
            )

    cex_backfill >> gap_check()


pw_cex_daily()
