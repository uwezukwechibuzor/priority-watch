"""pytest configuration for Airflow DAG unit tests."""
from __future__ import annotations

import os
import sys
from pathlib import Path

import pytest

# Add airflow/dags to sys.path so DAG files can import _shared.
DAGS_DIR = Path(__file__).parent.parent / "dags"
if str(DAGS_DIR) not in sys.path:
    sys.path.insert(0, str(DAGS_DIR))


def pytest_configure(config):
    # Minimal Airflow env so DagBag.collect_dags works without a running DB.
    os.environ.setdefault("AIRFLOW__CORE__UNIT_TEST_MODE", "True")
    os.environ.setdefault("AIRFLOW__CORE__DAGS_FOLDER", str(DAGS_DIR))
    os.environ.setdefault("AIRFLOW__DATABASE__SQL_ALCHEMY_CONN", "sqlite:///:memory:")
    os.environ.setdefault("AIRFLOW__CORE__EXECUTOR", "SequentialExecutor")
    os.environ.setdefault("AIRFLOW__CORE__LOAD_EXAMPLES", "False")
    # Stub env vars consumed by DAGs so they don't raise on import.
    os.environ.setdefault("CLICKHOUSE_HOST", "clickhouse")
    os.environ.setdefault("CLICKHOUSE_PORT", "8123")
    os.environ.setdefault("CLICKHOUSE_USER", "default")
    os.environ.setdefault("CLICKHOUSE_PASSWORD", "")
    os.environ.setdefault("ARBITRUM_RPC_URL", "http://localhost:8545")
    os.environ.setdefault("PGA_ACTIVATION_BLOCK", "0")
