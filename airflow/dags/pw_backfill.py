"""pw_backfill — manual block-range backfill with Airflow dynamic task mapping.

Splits the requested range into Airflow-level chunks so each chunk is a
separate task instance with its own logs, retries and pool slot.  The
pw-ingester binary handles its own internal chunking and resumability.

Pool: create the "rpc_pool" Airflow pool before first run and set its
slot count to match your RPC plan's concurrency budget.
"""
from __future__ import annotations

import os
import subprocess
from datetime import datetime

from airflow.datasets import Dataset
from airflow.decorators import dag, task
from airflow.models.param import Param
from airflow.operators.python import get_current_context

from _shared import DEFAULT_ARGS, RAW_BLOCKS_DATASET_URI

RAW_BLOCKS_DATASET = Dataset(RAW_BLOCKS_DATASET_URI)


@dag(
    dag_id="pw_backfill",
    schedule=None,
    start_date=datetime(2026, 1, 1),
    catchup=False,
    tags=["pga", "ingester", "backfill"],
    default_args=DEFAULT_ARGS,
    params={
        "from_block": Param(
            description="Start block number (integer) or date YYYY-MM-DD",
            type="string",
        ),
        "to_block": Param(
            default="head",
            description="End block number, date YYYY-MM-DD, or 'head'",
            type="string",
        ),
        "chunk_size": Param(
            default=10000,
            description="Blocks per Airflow task (pw-ingester handles internal chunking)",
            type="integer",
            minimum=100,
        ),
        "concurrency": Param(
            default=4,
            description="Parallel block workers inside each pw-ingester call",
            type="integer",
            minimum=1,
            maximum=16,
        ),
        "logs_only": Param(
            default=False,
            description="Ingest event logs only (skip blocks/txs/receipts)",
            type="boolean",
        ),
        "extra_address": Param(
            default="",
            description="Additional contract address to include in log filter",
            type="string",
        ),
    },
    doc_md=__doc__,
)
def pw_backfill() -> None:
    @task
    def plan_chunks() -> list[dict]:
        context = get_current_context()
        params = context["params"]
        from_val: str = str(params["from_block"]).strip()
        to_val: str = str(params["to_block"]).strip()
        chunk_size: int = int(params["chunk_size"])

        # Only split when both ends are explicit integers; otherwise pw-ingester
        # resolves dates/head internally, so we produce one chunk.
        try:
            from_block = int(from_val)
            to_block = int(to_val)
        except ValueError:
            return [{"from": from_val, "to": to_val}]

        chunks = []
        cur = from_block
        while cur <= to_block:
            end = min(cur + chunk_size - 1, to_block)
            chunks.append({"from": str(cur), "to": str(end)})
            cur = end + 1
        return chunks

    @task(
        pool="rpc_pool",
        outlets=[RAW_BLOCKS_DATASET],
    )
    def backfill_chunk(chunk: dict) -> None:
        context = get_current_context()
        params = context["params"]
        concurrency = str(int(params["concurrency"]))
        logs_only: bool = bool(params["logs_only"])
        extra_addr: str = str(params.get("extra_address", "")).strip()

        cmd = [
            "pw-ingester", "backfill",
            "--from", chunk["from"],
            "--to", chunk["to"],
            "--concurrency", concurrency,
        ]
        if logs_only:
            cmd.append("--logs-only")
        if extra_addr:
            cmd.extend(["--address", extra_addr])

        result = subprocess.run(cmd, capture_output=True, text=True, env=os.environ.copy())
        if result.stdout:
            print(result.stdout)
        if result.stderr:
            print(result.stderr)
        if result.returncode != 0:
            raise RuntimeError(f"pw-ingester backfill failed (exit {result.returncode})")

    backfill_chunk.expand(chunk=plan_chunks())


pw_backfill()
