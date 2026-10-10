#!/usr/bin/env python3
"""Generate dbt seed CSVs from config/*.yaml registry files.

Usage:
    python scripts/yaml_to_seeds.py [--config-dir config] [--seeds-dir dbt/seeds]

Run this whenever config/*.yaml files change and re-run `dbt seed` afterwards.
"""

import argparse
import csv
import os
import sys

try:
    import yaml
except ImportError:
    print("PyYAML not installed. Run: pip install pyyaml", file=sys.stderr)
    sys.exit(1)


def load_yaml(path: str) -> dict:
    with open(path) as f:
        return yaml.safe_load(f)


def write_csv(path: str, fieldnames: list, rows: list) -> None:
    with open(path, "w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames, extrasaction="ignore")
        writer.writeheader()
        writer.writerows(rows)
    print(f"Wrote {len(rows)} rows → {path}")


def pools_to_rows(pools: list) -> list:
    rows = []
    for p in pools:
        rows.append(
            {
                "address": p.get("address", ""),
                "dex": p.get("dex", ""),
                "event_type": p.get("event_type", ""),
                "token0_address": (p.get("token0") or {}).get("address", ""),
                "token0_symbol": (p.get("token0") or {}).get("symbol", ""),
                "token0_decimals": (p.get("token0") or {}).get("decimals", 18),
                "token1_address": (p.get("token1") or {}).get("address", ""),
                "token1_symbol": (p.get("token1") or {}).get("symbol", ""),
                "token1_decimals": (p.get("token1") or {}).get("decimals", 18),
                "fee": p.get("fee", 0),
                "tick_spacing": p.get("tick_spacing", 0),
                "cex_symbol": p.get("cex_symbol", ""),
                "verified": 1 if p.get("verified") else 0,
                "source_url": p.get("source_url", ""),
            }
        )
    return rows


def entities_to_rows(entities: list) -> list:
    """Normalise entities: one row per (entity, address) pair.
    Entities with no verified addresses get a single row with empty address fields.
    """
    rows = []
    for e in entities:
        addresses = e.get("addresses") or []
        if not addresses:
            rows.append(
                {
                    "entity_name": e["name"],
                    "entity_type": e.get("type", ""),
                    "color_index": e.get("color_index", 0),
                    "address": "",
                    "address_label": "",
                    "verified": 0,
                    "source_url": e.get("source_url", ""),
                }
            )
        else:
            for a in addresses:
                rows.append(
                    {
                        "entity_name": e["name"],
                        "entity_type": e.get("type", ""),
                        "color_index": e.get("color_index", 0),
                        "address": a.get("address", ""),
                        "address_label": a.get("label", ""),
                        "verified": 1 if a.get("verified") else 0,
                        "source_url": a.get(
                            "source_url", e.get("source_url", "")
                        ),
                    }
                )
    return rows


def contracts_to_rows(contracts: list) -> list:
    rows = []
    for c in contracts:
        rows.append(
            {
                "name": c.get("name", ""),
                "address": c.get("address", ""),
                "deployed_block": c.get("deployed_block", 0) or 0,
                "verified": 1 if c.get("verified") else 0,
                "source_url": c.get("source_url", ""),
                "note": c.get("note", ""),
            }
        )
    return rows


def events_to_rows(events: list) -> list:
    rows = []
    for e in events:
        rows.append(
            {
                "id": e.get("id", ""),
                "label": e.get("label", ""),
                "event_date": e.get("date", "") or "",
                "block": e.get("block", "") if e.get("block") is not None else "",
                "description": e.get("description", "") or "",
                "source_url": e.get("source_url", ""),
            }
        )
    return rows


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config-dir", default="config")
    parser.add_argument("--seeds-dir", default="dbt/seeds")
    args = parser.parse_args()

    pools = load_yaml(os.path.join(args.config_dir, "pools.yaml"))["pools"]
    entities = load_yaml(os.path.join(args.config_dir, "entities.yaml"))["entities"]
    contracts = load_yaml(os.path.join(args.config_dir, "contracts.yaml"))["contracts"]
    events = load_yaml(os.path.join(args.config_dir, "events.yaml"))["events"]

    write_csv(
        os.path.join(args.seeds_dir, "pools.csv"),
        [
            "address",
            "dex",
            "event_type",
            "token0_address",
            "token0_symbol",
            "token0_decimals",
            "token1_address",
            "token1_symbol",
            "token1_decimals",
            "fee",
            "tick_spacing",
            "cex_symbol",
            "verified",
            "source_url",
        ],
        pools_to_rows(pools),
    )
    write_csv(
        os.path.join(args.seeds_dir, "entities.csv"),
        [
            "entity_name",
            "entity_type",
            "color_index",
            "address",
            "address_label",
            "verified",
            "source_url",
        ],
        entities_to_rows(entities),
    )
    write_csv(
        os.path.join(args.seeds_dir, "contracts.csv"),
        ["name", "address", "deployed_block", "verified", "source_url", "note"],
        contracts_to_rows(contracts),
    )
    write_csv(
        os.path.join(args.seeds_dir, "events.csv"),
        ["id", "label", "event_date", "block", "description", "source_url"],
        events_to_rows(events),
    )


if __name__ == "__main__":
    main()
