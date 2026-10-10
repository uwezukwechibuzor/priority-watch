"""pw_dbt_incremental — run dbt incremental models every 10 minutes.

Uses Astronomer Cosmos to render dbt models as native Airflow tasks.
Runs in three sequential groups: staging → intermediate → marts, with
dbt test executed after each model (TestBehavior.AFTER_EACH).

Also triggered whenever the pw_backfill DAG updates the raw.blocks Dataset,
so a manual backfill immediately kicks off a dbt refresh without waiting for
the next scheduled run.
"""
from __future__ import annotations

from datetime import datetime
from pathlib import Path

from airflow.datasets import Dataset
from airflow.decorators import dag
from airflow.timetables.datasets import DatasetOrTimeSchedule
from airflow.timetables.trigger import CronTriggerTimetable
from cosmos import DbtTaskGroup, ExecutionConfig, ProfileConfig, ProjectConfig, RenderConfig
from cosmos.constants import ExecutionMode, TestBehavior

from _shared import (
    DBT_EXECUTABLE,
    DBT_PROFILES_DIR,
    DBT_PROJECT_DIR,
    DEFAULT_ARGS,
    RAW_BLOCKS_DATASET_URI,
)

RAW_BLOCKS_DATASET = Dataset(RAW_BLOCKS_DATASET_URI)

_project_config = ProjectConfig(dbt_project_path=Path(DBT_PROJECT_DIR))

_profile_config = ProfileConfig(
    profile_name="priority_watch",
    target_name="prod",
    profiles_yml_filepath=Path(DBT_PROFILES_DIR) / "profiles.yml",
)

_execution_config = ExecutionConfig(
    execution_mode=ExecutionMode.LOCAL,
    dbt_executable_path=DBT_EXECUTABLE,
)


def _render(tag: str) -> RenderConfig:
    return RenderConfig(
        select=[f"tag:{tag}"],
        test_behavior=TestBehavior.AFTER_EACH,
    )


@dag(
    dag_id="pw_dbt_incremental",
    schedule=DatasetOrTimeSchedule(
        timetable=CronTriggerTimetable("*/10 * * * *", timezone="UTC"),
        datasets=[RAW_BLOCKS_DATASET],
    ),
    start_date=datetime(2026, 1, 1),
    catchup=False,
    tags=["pga", "dbt", "incremental"],
    default_args=DEFAULT_ARGS,
    max_active_runs=1,
    doc_md=__doc__,
)
def pw_dbt_incremental() -> None:
    staging = DbtTaskGroup(
        group_id="staging",
        project_config=_project_config,
        profile_config=_profile_config,
        execution_config=_execution_config,
        render_config=_render("staging"),
    )

    intermediate = DbtTaskGroup(
        group_id="intermediate",
        project_config=_project_config,
        profile_config=_profile_config,
        execution_config=_execution_config,
        render_config=_render("intermediate"),
    )

    marts = DbtTaskGroup(
        group_id="marts",
        project_config=_project_config,
        profile_config=_profile_config,
        execution_config=_execution_config,
        render_config=_render("marts"),
    )

    staging >> intermediate >> marts


pw_dbt_incremental()
