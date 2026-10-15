"""Shared helpers used across all Priority Watch Airflow DAGs."""
from __future__ import annotations

import json
import os
import urllib.request
from datetime import timedelta
from typing import Any


def slack_failure_callback(context: dict[str, Any]) -> None:
    webhook = os.environ.get("SLACK_WEBHOOK_URL", "")
    if not webhook:
        return
    dag_id = context["dag"].dag_id
    ti = context["task_instance"]
    payload = json.dumps(
        {"text": f":red_circle: *{dag_id}* › `{ti.task_id}` failed\n<{ti.log_url}|View logs>"}
    ).encode()
    req = urllib.request.Request(
        webhook, data=payload, headers={"Content-Type": "application/json"}
    )
    try:
        urllib.request.urlopen(req, timeout=10)
    except Exception:
        pass  # never let alert failure mask the original task failure


DEFAULT_ARGS: dict[str, Any] = {
    "retries": 2,
    "retry_delay": timedelta(seconds=30),
    "retry_exponential_backoff": True,
    "max_retry_delay": timedelta(minutes=10),
    "on_failure_callback": slack_failure_callback,
}

DBT_PROJECT_DIR = "/opt/airflow/dbt"
DBT_PROFILES_DIR = "/opt/airflow/dbt"
DBT_EXECUTABLE = "/home/airflow/.local/bin/dbt"
DBT_MANIFEST_PATH = "/opt/airflow/dbt/target/manifest.json"
DBT_TARGET = "dev"
