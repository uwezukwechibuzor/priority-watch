"""pw_timeboost_baseline — manual Timeboost-era data import.

Two steps, both optional via params:
  1. Auction-log backfill — uses pw-ingester backfill with --logs-only
     and --address set to the ExpressLaneAuction contract to pull all
     AuctionResolved events into raw.logs.
  2. timeboost.app import — uses pw-ingester import-timeboost to download
     and load the Parquet baseline from timeboost.app into
     raw.timeboost_bids_ext / raw.timeboost_auction_results_ext.

Run this once after PGA activation to populate the Timeboost-era baseline
that feeds agg_era_comparison and int_timeboost_rounds.
"""
from __future__ import annotations

import os
import subprocess
from datetime import datetime

from airflow.decorators import dag, task
from airflow.models.param import Param
from airflow.operators.python import get_current_context

from _shared import DEFAULT_ARGS

# ExpressLaneAuction address (verified in config/contracts.yaml).
EXPRESS_LANE_AUCTION = "0x65f07c7d521164a4d5dac6eb8fac8da067a3b78f"


@dag(
    dag_id="pw_timeboost_baseline",
    schedule=None,
    start_date=datetime(2026, 1, 1),
    catchup=False,
    tags=["pga", "ingester", "timeboost", "baseline"],
    default_args=DEFAULT_ARGS,
    params={
        "from_date": Param(
            default="2025-04-17",
            description="Start date YYYY-MM-DD (Timeboost launch)",
            type="string",
        ),
        "to_date": Param(
            default="2026-09-24",
            description="End date YYYY-MM-DD (PGA activation)",
            type="string",
        ),
        "run_log_backfill": Param(
            default=True,
            description="Backfill auction logs from the chain",
            type="boolean",
        ),
        "run_timeboost_import": Param(
            default=True,
            description="Import Parquet baseline from timeboost.app",
            type="boolean",
        ),
        "extra_auction_address": Param(
            default="",
            description="Override ExpressLaneAuction address (leave empty to use default)",
            type="string",
        ),
    },
    doc_md=__doc__,
)
def pw_timeboost_baseline() -> None:
    @task
    def backfill_auction_logs() -> None:
        context = get_current_context()
        params = context["params"]
        if not params["run_log_backfill"]:
            print("run_log_backfill=false — skipping")
            return

        auction_addr = str(params.get("extra_auction_address", "")).strip() or EXPRESS_LANE_AUCTION
        cmd = [
            "pw-ingester", "backfill",
            "--from", str(params["from_date"]),
            "--to", str(params["to_date"]),
            "--logs-only",
            "--address", auction_addr,
        ]
        result = subprocess.run(cmd, capture_output=True, text=True, env=os.environ.copy())
        if result.stdout:
            print(result.stdout)
        if result.stderr:
            print(result.stderr)
        if result.returncode != 0:
            raise RuntimeError(f"auction log backfill failed (exit {result.returncode})")

    @task
    def import_timeboost_parquet() -> None:
        context = get_current_context()
        params = context["params"]
        if not params["run_timeboost_import"]:
            print("run_timeboost_import=false — skipping")
            return

        cmd = [
            "pw-ingester", "import-timeboost",
            "--from", str(params["from_date"]),
            "--to", str(params["to_date"]),
        ]
        result = subprocess.run(cmd, capture_output=True, text=True, env=os.environ.copy())
        if result.stdout:
            print(result.stdout)
        if result.stderr:
            print(result.stderr)
        if result.returncode != 0:
            raise RuntimeError(f"import-timeboost failed (exit {result.returncode})")

    backfill_auction_logs() >> import_timeboost_parquet()


pw_timeboost_baseline()
