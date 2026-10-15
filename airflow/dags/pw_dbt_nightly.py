"""pw_dbt_nightly — nightly full dbt test, source freshness and docs build.

Runs at 02:00 UTC daily:
  1. dbt test (all models) — fails the DAG if any model test fails.
  2. dbt source freshness — warns but does not fail the DAG.
  3. dbt docs generate — writes manifest + catalog to /opt/airflow/docs.

The generated docs directory is expected to be mounted or copied to a
static web server by the host.  See infra/caddy/Caddyfile for the
/dbt-docs route if serving via Caddy.
"""
from __future__ import annotations

from datetime import datetime

from airflow.decorators import dag
from airflow.operators.bash import BashOperator

from _shared import DBT_PROFILES_DIR, DBT_PROJECT_DIR, DEFAULT_ARGS

_DBT = f"/home/airflow/.local/bin/dbt --profiles-dir {DBT_PROFILES_DIR} --project-dir {DBT_PROJECT_DIR} --target dev"


@dag(
    dag_id="pw_dbt_nightly",
    schedule="0 2 * * *",
    start_date=datetime(2026, 1, 1),
    catchup=False,
    tags=["pga", "dbt", "nightly"],
    default_args=DEFAULT_ARGS,
    max_active_runs=1,
    is_paused_upon_creation=False,
    doc_md=__doc__,
)
def pw_dbt_nightly() -> None:
    full_test = BashOperator(
        task_id="dbt_test_all",
        bash_command=f"{_DBT} test",
    )

    source_freshness = BashOperator(
        task_id="dbt_source_freshness",
        bash_command=f"{_DBT} source freshness || true",  # warn only
    )

    docs_generate = BashOperator(
        task_id="dbt_docs_generate",
        # Writes manifest.json to target/ (used by pw_dbt_incremental LoadMode.DBT_MANIFEST)
        # and catalog + index.html to /opt/airflow/docs for the Caddy /dbt-docs route.
        bash_command=f"{_DBT} docs generate && cp /opt/airflow/dbt/target/index.html /opt/airflow/docs/ 2>/dev/null || true",
    )

    full_test >> source_freshness >> docs_generate


pw_dbt_nightly()
