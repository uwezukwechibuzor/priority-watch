"""Unit tests for Priority Watch Airflow DAGs.

Checks:
  - DagBag loads all DAGs with no import errors.
  - No DAG has a cycle.
  - Every non-sensor task has retries >= 1.
  - Every DAG has at least one tag.
  - Manual-only DAGs have schedule=None.
  - Scheduled DAGs have the expected schedule string.
"""
from __future__ import annotations

import os
from pathlib import Path

import pytest
from airflow.models import DagBag

DAGS_DIR = str(Path(__file__).parent.parent / "dags")

# DAGs that must have schedule=None (manual-only).
MANUAL_DAGS = {"pw_backfill", "pw_timeboost_baseline"}

# DAGs and their expected schedule expressions (non-None).
EXPECTED_SCHEDULES: dict[str, str] = {
    "pw_dbt_nightly": "0 2 * * *",
    "pw_cex_daily": "0 1 * * *",
    "pw_monitoring": "*/5 * * * *",
    "pw_clickhouse_backup": "0 3 * * *",
}


@pytest.fixture(scope="module")
def dagbag() -> DagBag:
    bag = DagBag(dag_folder=DAGS_DIR, include_examples=False)
    return bag


# ── import health ─────────────────────────────────────────────────────────────

def test_no_import_errors(dagbag: DagBag) -> None:
    errors = dagbag.import_errors
    assert errors == {}, f"DAG import errors:\n" + "\n".join(
        f"  {path}: {err}" for path, err in errors.items()
    )


def test_expected_dag_ids_present(dagbag: DagBag) -> None:
    expected = {
        "pw_backfill",
        "pw_dbt_incremental",
        "pw_dbt_nightly",
        "pw_cex_daily",
        "pw_monitoring",
        "pw_timeboost_baseline",
        "pw_clickhouse_backup",
    }
    loaded = set(dagbag.dag_ids)
    missing = expected - loaded
    assert not missing, f"DAGs not loaded: {missing}"


# ── structural correctness ────────────────────────────────────────────────────

@pytest.mark.parametrize("dag_id", [
    "pw_backfill",
    "pw_dbt_incremental",
    "pw_dbt_nightly",
    "pw_cex_daily",
    "pw_monitoring",
    "pw_timeboost_baseline",
    "pw_clickhouse_backup",
])
def test_dag_has_no_cycle(dagbag: DagBag, dag_id: str) -> None:
    dag = dagbag.get_dag(dag_id)
    assert dag is not None, f"{dag_id} not found in DagBag"
    # DagBag.collect_dags() raises CycleError during import if a cycle exists;
    # calling test_cycle() here makes the assertion explicit.
    dag.test_cycle()  # raises if cycle detected


@pytest.mark.parametrize("dag_id", [
    "pw_backfill",
    "pw_dbt_incremental",
    "pw_dbt_nightly",
    "pw_cex_daily",
    "pw_monitoring",
    "pw_timeboost_baseline",
    "pw_clickhouse_backup",
])
def test_dag_has_tags(dagbag: DagBag, dag_id: str) -> None:
    dag = dagbag.get_dag(dag_id)
    assert dag is not None
    assert dag.tags, f"{dag_id} has no tags"


@pytest.mark.parametrize("dag_id", [
    "pw_backfill",
    "pw_dbt_incremental",
    "pw_dbt_nightly",
    "pw_cex_daily",
    "pw_monitoring",
    "pw_timeboost_baseline",
    "pw_clickhouse_backup",
])
def test_tasks_have_retries(dagbag: DagBag, dag_id: str) -> None:
    dag = dagbag.get_dag(dag_id)
    assert dag is not None
    for task in dag.tasks:
        # Sensors and trigger rules may legitimately have 0 retries; skip them.
        if "sensor" in type(task).__name__.lower():
            continue
        assert task.retries >= 1, (
            f"{dag_id}.{task.task_id}: retries={task.retries} — must be >= 1"
        )


# ── schedule assertions ───────────────────────────────────────────────────────

@pytest.mark.parametrize("dag_id", sorted(MANUAL_DAGS))
def test_manual_dag_has_no_schedule(dagbag: DagBag, dag_id: str) -> None:
    dag = dagbag.get_dag(dag_id)
    assert dag is not None
    # schedule_interval is None for manual DAGs.
    assert dag.schedule_interval is None, (
        f"{dag_id} should have schedule=None but has {dag.schedule_interval!r}"
    )


@pytest.mark.parametrize("dag_id,expected", sorted(EXPECTED_SCHEDULES.items()))
def test_scheduled_dag_cron(dagbag: DagBag, dag_id: str, expected: str) -> None:
    dag = dagbag.get_dag(dag_id)
    assert dag is not None
    assert dag.schedule_interval == expected, (
        f"{dag_id}: expected schedule {expected!r}, got {dag.schedule_interval!r}"
    )
