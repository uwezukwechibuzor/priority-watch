import type { PayerShareRow } from "@/api/types";
import { type Column, DataTable, FilterInput, Panel } from "@/components/ui";
import { compact, entityColor, formatEth, formatPct } from "@/lib/formatters";
import { useState } from "react";

interface PayersTabProps {
  rows: PayerShareRow[];
  loading: boolean;
}

export function PayersTab({ rows, loading }: PayersTabProps) {
  const [filter, setFilter] = useState("");

  const filtered = filter
    ? rows.filter((r) => r.entity_name.toLowerCase().includes(filter.toLowerCase()))
    : rows;

  const columns: Column<PayerShareRow>[] = [
    {
      key: "entity",
      header: "Entity",
      minWidth: 140,
      render: (row) => (
        <span style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <span
            style={{
              width: 8,
              height: 8,
              borderRadius: "50%",
              background: entityColor(row.color_index),
              flexShrink: 0,
            }}
          />
          <span style={{ fontSize: 12 }}>{row.entity_name}</span>
        </span>
      ),
    },
    {
      key: "tips_eth",
      header: "Tips ETH",
      width: 110,
      align: "right",
      render: (row) => (
        <span className="tabular" style={{ fontSize: 12 }}>
          {formatEth(row.tip_eth)}
        </span>
      ),
    },
    {
      key: "share",
      header: "Share %",
      width: 80,
      align: "right",
      render: (row) => (
        <span className="tabular" style={{ fontSize: 12 }}>
          {formatPct(row.share_pct / 100)}
        </span>
      ),
    },
    {
      key: "revert",
      header: "Revert Rate",
      width: 90,
      align: "right",
      render: (row) => (
        <span
          className="tabular"
          style={{
            fontSize: 12,
            color: row.revert_rate > 0.1 ? "var(--color-red)" : undefined,
          }}
        >
          {formatPct(row.revert_rate)}
        </span>
      ),
    },
    {
      key: "avg_rank",
      header: "Avg Position",
      width: 90,
      align: "right",
      render: (row) => (
        <span className="tabular" style={{ fontSize: 12 }}>
          {row.avg_tip_rank.toFixed(1)}
        </span>
      ),
    },
    {
      key: "tx_count",
      header: "Tx Count",
      width: 80,
      align: "right",
      render: (row) => (
        <span className="tabular" style={{ fontSize: 12 }}>
          {compact(row.tx_count)}
        </span>
      ),
    },
    {
      key: "type",
      header: "Type",
      width: 80,
      render: (row) => (
        <span style={{ fontSize: 12, color: "var(--color-text-muted)" }}>{row.entity_type}</span>
      ),
    },
  ];

  return (
    <Panel
      title="PAYERS LEADERBOARD"
      action={
        <FilterInput value={filter} onChange={setFilter} placeholder="Filter entity…" width={200} />
      }
    >
      <DataTable
        columns={columns}
        rows={filtered}
        rowKey={(r) => r.entity_name}
        maxHeight={600}
        loading={loading}
      />
    </Panel>
  );
}
