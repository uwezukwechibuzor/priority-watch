"""pw_monitoring — every-5-minute health and lag monitor.

Checks in order:
  1. Follower /healthz — HTTP GET, expects 200.
  2. Ingest lag — latest block timestamp vs now(); fails if > LAG_THRESHOLD_S.
  3. Missing blocks — counts gaps in block sequence in last 24 h; fails if any.
  4. CEX gaps — max gap in cex_klines_1s last 1 h; warns if > 60 s.
  5. meta.source_status staleness — warns if any source not updated in 30 min.
  6. dbt freshness — checks marts.fct_blocks max block_timestamp; warns if stale.

All failures trigger a Slack alert when SLACK_WEBHOOK_URL is set.
Individual checks are independent tasks so one failure does not skip others.
"""
from __future__ import annotations

import os
import urllib.request
from datetime import datetime

from airflow.decorators import dag, task
from airflow.utils.trigger_rule import TriggerRule

from _shared import DEFAULT_ARGS, slack_failure_callback

_LAG_THRESHOLD_S = 120        # fail if ingest is more than 2 minutes behind
_CEX_GAP_WARN_S = 60          # warn (not fail) if CEX gap exceeds 60 s
_SOURCE_STATUS_STALE_MIN = 30  # warn if any source_status row not updated in 30 min
_DBT_STALE_MIN = 20            # warn if marts.fct_blocks max timestamp older than 20 min


def _ch_client():
    import clickhouse_connect  # noqa: PLC0415
    return clickhouse_connect.get_client(
        host=os.environ.get("CLICKHOUSE_HOST", "clickhouse"),
        port=int(os.environ.get("CLICKHOUSE_PORT", "8123")),
        username=os.environ.get("CLICKHOUSE_USER", "default"),
        password=os.environ.get("CLICKHOUSE_PASSWORD", ""),
    )


@dag(
    dag_id="pw_monitoring",
    schedule="*/5 * * * *",
    start_date=datetime(2026, 1, 1),
    catchup=False,
    tags=["pga", "monitoring", "ops"],
    default_args=DEFAULT_ARGS,
    max_active_runs=1,
    is_paused_upon_creation=False,
    doc_md=__doc__,
)
def pw_monitoring() -> None:
    @task(on_failure_callback=slack_failure_callback)
    def check_follower_healthz() -> None:
        follower_url = os.environ.get("FOLLOWER_HEALTHZ_URL", "http://pw-follower:9091/healthz")
        try:
            with urllib.request.urlopen(follower_url, timeout=10) as resp:
                if resp.status != 200:
                    raise RuntimeError(f"follower /healthz returned HTTP {resp.status}")
        except Exception as exc:
            raise RuntimeError(f"follower /healthz unreachable: {exc}") from exc
        print("follower /healthz OK")

    @task(on_failure_callback=slack_failure_callback)
    def check_ingest_lag() -> None:
        client = _ch_client()
        result = client.query(
            "SELECT dateDiff('second', max(block_timestamp), now()) AS lag_s "
            "FROM raw.blocks FINAL"
        )
        lag_s = result.first_row[0] if result.result_rows else None
        if lag_s is None:
            raise RuntimeError("raw.blocks is empty — follower may not have started")
        print(f"ingest lag: {lag_s}s (threshold: {_LAG_THRESHOLD_S}s)")
        if lag_s > _LAG_THRESHOLD_S:
            raise RuntimeError(
                f"ingest lag {lag_s}s exceeds threshold {_LAG_THRESHOLD_S}s"
            )

    @task(on_failure_callback=slack_failure_callback)
    def check_missing_blocks() -> None:
        client = _ch_client()
        # Detect gaps: any block number in [min_block, max_block] that is absent.
        result = client.query(
            """
            WITH
                range_min AS (
                    SELECT min(block_number) AS v
                    FROM raw.blocks FINAL
                    WHERE block_timestamp >= now() - INTERVAL 24 HOUR
                ),
                range_max AS (
                    SELECT max(block_number) AS v
                    FROM raw.blocks FINAL
                    WHERE block_timestamp >= now() - INTERVAL 24 HOUR
                ),
                present AS (
                    SELECT block_number
                    FROM raw.blocks FINAL
                    WHERE block_timestamp >= now() - INTERVAL 24 HOUR
                )
            SELECT countIf(n NOT IN (SELECT block_number FROM present)) AS missing_count
            FROM (
                SELECT arrayJoin(range(
                    (SELECT v FROM range_min),
                    (SELECT v FROM range_max) + 1
                )) AS n
            )
            """
        )
        missing = result.first_row[0] if result.result_rows else 0
        print(f"missing blocks in last 24 h: {missing}")
        if missing > 0:
            raise RuntimeError(
                f"{missing} missing block(s) detected in last 24 h — "
                "check follower logs or trigger a backfill"
            )

    @task  # warn only — does not fail the DAG
    def check_cex_gaps() -> None:
        client = _ch_client()
        result = client.query(
            """
            WITH ordered AS (
                SELECT open_time,
                       neighbor(open_time, 1) AS next_time
                FROM raw.cex_klines_1s FINAL
                WHERE symbol = 'ETHUSDT'
                  AND venue = 'binance'
                  AND open_time >= now() - INTERVAL 1 HOUR
                ORDER BY open_time
            )
            SELECT max(dateDiff('second', open_time, next_time)) AS max_gap
            FROM ordered
            WHERE next_time > open_time
            """
        )
        max_gap = result.first_row[0] if result.result_rows else 0
        print(f"max CEX gap in last 1 h: {max_gap}s")
        if max_gap > _CEX_GAP_WARN_S:
            print(
                f"WARNING: CEX kline gap {max_gap}s > {_CEX_GAP_WARN_S}s — "
                "WebSocket follower may have missed data"
            )

    @task  # warn only
    def check_source_status() -> None:
        client = _ch_client()
        result = client.query(
            f"""
            SELECT source, last_updated_at,
                   dateDiff('minute', last_updated_at, now()) AS age_min
            FROM meta.source_status FINAL
            WHERE dateDiff('minute', last_updated_at, now()) > {_SOURCE_STATUS_STALE_MIN}
            ORDER BY age_min DESC
            """
        )
        stale = result.result_rows
        if stale:
            for row in stale:
                print(f"WARNING: source '{row[0]}' last updated {row[2]} min ago")

    @task  # warn only
    def check_dbt_freshness() -> None:
        client = _ch_client()
        result = client.query(
            "SELECT dateDiff('minute', max(block_timestamp), now()) AS age_min "
            "FROM marts.fct_blocks FINAL"
        )
        age_min = result.first_row[0] if result.result_rows else None
        if age_min is None:
            print("WARNING: marts.fct_blocks is empty — dbt has not run yet")
            return
        print(f"marts.fct_blocks freshness: {age_min} min old")
        if age_min > _DBT_STALE_MIN:
            print(
                f"WARNING: marts.fct_blocks is {age_min} min old "
                f"(threshold: {_DBT_STALE_MIN} min) — check pw_dbt_incremental"
            )

    @task(trigger_rule=TriggerRule.ALL_DONE)
    def summarise() -> None:
        print("monitoring sweep complete")

    checks = [
        check_follower_healthz(),
        check_ingest_lag(),
        check_missing_blocks(),
        check_cex_gaps(),
        check_source_status(),
        check_dbt_freshness(),
    ]
    checks >> summarise()


pw_monitoring()
