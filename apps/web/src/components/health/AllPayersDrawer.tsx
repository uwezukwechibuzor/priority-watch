import type { AllPayerRow } from "@/api/types";
import { Drawer } from "@/components/ui/Drawer";
import { formatEth } from "@/lib/formatters";
import { useECharts } from "@/lib/use-echarts";
import { allPayersQuery } from "@/queries/health";
import { useQuery } from "@tanstack/react-query";
import type { EChartsOption } from "echarts";
import { useMemo, useState } from "react";

interface AllPayersDrawerProps {
  open: boolean;
  onClose: () => void;
}

function shortAddr(addr: string): string {
  if (!addr || addr.length < 10) return addr;
  return `${addr.slice(0, 6)}…${addr.slice(-4)}`;
}

function displayName(r: AllPayerRow): string {
  return r.entity_name !== r.from_address ? r.entity_name : shortAddr(r.from_address);
}

function PayersBarChart({ rows }: { rows: AllPayerRow[] }) {
  const top = rows.slice(0, 20);
  const labels = top.map((r) => displayName(r));
  const values = top.map((r) => +r.tip_eth.toFixed(6));

  const option = useMemo<EChartsOption>(
    () => ({
      grid: { left: 120, right: 60, top: 8, bottom: 8, containLabel: false },
      xAxis: {
        type: "value",
        axisLabel: { color: "#6b7280", fontSize: 10, formatter: (v: number) => `${v.toFixed(3)}` },
        splitLine: { lineStyle: { color: "#2a2a2a" } },
      },
      yAxis: {
        type: "category",
        data: [...labels].reverse(),
        axisLabel: {
          color: "#9ca3af",
          fontSize: 10,
          width: 110,
          overflow: "truncate",
          fontFamily: "var(--font-mono, monospace)",
        },
        axisTick: { show: false },
        axisLine: { show: false },
      },
      series: [
        {
          type: "bar",
          data: [...values].reverse(),
          barMaxWidth: 16,
          itemStyle: {
            color: {
              type: "linear",
              x: 0,
              y: 0,
              x2: 1,
              y2: 0,
              colorStops: [
                { offset: 0, color: "#2563eb" },
                { offset: 1, color: "#38bdf8" },
              ],
            },
            borderRadius: [0, 3, 3, 0],
          },
          label: {
            show: true,
            position: "right",
            fontSize: 10,
            color: "#9ca3af",
            formatter: (p: { value: number }) => `${p.value.toFixed(4)} ETH`,
          },
        },
      ],
      tooltip: {
        trigger: "axis",
        axisPointer: { type: "shadow" },
        formatter: (params: { name: string; value: number }[]) => {
          const p = params[0];
          return `${p.name}<br/>${p.value.toFixed(6)} ETH`;
        },
      },
    }),
    [labels, values],
  );

  const ref = useECharts(option, [option]);
  return <div ref={ref} style={{ width: "100%", height: top.length * 28 + 20 }} />;
}

function PayersTreemap({ rows }: { rows: AllPayerRow[] }) {
  const totalEth = rows.reduce((s, r) => s + r.tip_eth, 0);

  const option = useMemo<EChartsOption>(
    () => ({
      tooltip: {
        formatter: (p: { name: string; value: number; data: { txCount: number } }) =>
          [
            `<span style="font-family:monospace;font-size:11px">${p.name}</span>`,
            `<b>${p.value.toFixed(6)} ETH</b>`,
            `${((p.value / totalEth) * 100).toFixed(2)}% of total`,
            `${p.data.txCount.toLocaleString()} txs`,
          ].join("<br/>"),
      },
      series: [
        {
          type: "treemap",
          roam: false,
          nodeClick: false,
          breadcrumb: { show: false },
          width: "100%",
          height: "100%",
          data: rows.map((r, i) => ({
            name: displayName(r),
            value: +r.tip_eth.toFixed(8),
            txCount: r.tx_count,
            itemStyle: {
              color: `hsl(${210 + ((i * 7) % 60)}, ${70 - (i % 3) * 10}%, ${38 - Math.min(i, 20) * 0.6}%)`,
              borderColor: "#111",
              borderWidth: 1,
              gapWidth: 1,
            },
            label: {
              show: true,
              color: "#e5e7eb",
              fontSize: Math.max(9, 13 - Math.floor(i / 20)),
              fontFamily: "var(--font-mono, monospace)",
              formatter: ({ name, value }: { name: string; value: number }) =>
                i < 40 ? `${name}\n${(value as number).toFixed(4)} ETH` : name,
            },
          })),
          emphasis: {
            itemStyle: { borderColor: "#38bdf8", borderWidth: 2 },
          },
        },
      ],
    }),
    [rows, totalEth],
  );

  const ref = useECharts(option, [option]);
  return <div ref={ref} style={{ width: "100%", height: 420 }} />;
}

export function AllPayersDrawer({ open, onClose }: AllPayersDrawerProps) {
  const { data: rows = [], isLoading } = useQuery({
    ...allPayersQuery,
    enabled: open,
  });
  const [filter, setFilter] = useState("");

  const filtered = filter
    ? rows.filter(
        (r) =>
          r.entity_name.toLowerCase().includes(filter.toLowerCase()) ||
          r.from_address.toLowerCase().includes(filter.toLowerCase()) ||
          r.contract_label.toLowerCase().includes(filter.toLowerCase()),
      )
    : rows;

  return (
    <Drawer open={open} onClose={onClose} title="ALL PAYERS — HISTORICAL" width={780}>
      {/* Treemap — all payers by tip */}
      {!isLoading && rows.length > 0 && (
        <div
          style={{
            marginBottom: 20,
            paddingBottom: 16,
            borderBottom: "1px solid var(--color-border)",
          }}
        >
          <div
            style={{
              fontSize: 10,
              fontWeight: 500,
              textTransform: "uppercase",
              letterSpacing: "0.06em",
              color: "var(--color-text-muted)",
              marginBottom: 8,
            }}
          >
            All Payers — Total Tip (ETH) · hover for details
          </div>
          <PayersTreemap rows={rows} />
        </div>
      )}

      {/* Top-20 bar chart */}
      {!isLoading && rows.length > 0 && (
        <div
          style={{
            marginBottom: 20,
            paddingBottom: 16,
            borderBottom: "1px solid var(--color-border)",
          }}
        >
          <div
            style={{
              fontSize: 10,
              fontWeight: 500,
              textTransform: "uppercase",
              letterSpacing: "0.06em",
              color: "var(--color-text-muted)",
              marginBottom: 8,
            }}
          >
            Top 20 Payers — Total Tip (ETH)
          </div>
          <PayersBarChart rows={rows} />
        </div>
      )}

      {/* Search */}
      <div style={{ marginBottom: 12 }}>
        <input
          type="search"
          placeholder="Filter by address, entity or contract…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          style={{
            width: "100%",
            padding: "6px 10px",
            background: "var(--color-bg-base)",
            border: "1px solid var(--color-border)",
            borderRadius: 4,
            color: "var(--color-text-primary)",
            fontSize: 12,
            outline: "none",
            boxSizing: "border-box",
          }}
        />
      </div>

      {/* Table */}
      <div style={{ overflowX: "auto" }}>
        <table
          style={{
            width: "100%",
            borderCollapse: "collapse",
            fontSize: 12,
          }}
        >
          <thead>
            <tr
              style={{
                borderBottom: "1px solid var(--color-border)",
                color: "var(--color-text-muted)",
                fontSize: 10,
                textTransform: "uppercase",
                letterSpacing: "0.06em",
              }}
            >
              <th style={{ padding: "6px 8px", textAlign: "left", fontWeight: 500 }}>#</th>
              <th style={{ padding: "6px 8px", textAlign: "left", fontWeight: 500 }}>From</th>
              <th style={{ padding: "6px 8px", textAlign: "left", fontWeight: 500 }}>Contract</th>
              <th style={{ padding: "6px 8px", textAlign: "left", fontWeight: 500 }}>Name</th>
              <th style={{ padding: "6px 8px", textAlign: "right", fontWeight: 500 }}>Tip (ETH)</th>
              <th style={{ padding: "6px 8px", textAlign: "right", fontWeight: 500 }}>Txs</th>
            </tr>
          </thead>
          <tbody>
            {isLoading
              ? Array.from({ length: 12 }, (_, i) => (
                  // biome-ignore lint/suspicious/noArrayIndexKey: skeleton rows have no stable id
                  <tr key={i}>
                    {Array.from({ length: 6 }, (_, j) => (
                      // biome-ignore lint/suspicious/noArrayIndexKey: skeleton cells
                      <td key={j} style={{ padding: "6px 8px" }}>
                        <div
                          className="skeleton"
                          style={{ height: 14, borderRadius: 2, width: "80%" }}
                        />
                      </td>
                    ))}
                  </tr>
                ))
              : filtered.map((row, idx) => (
                  <PayerRow key={row.from_address} rank={idx + 1} row={row} />
                ))}
          </tbody>
        </table>

        {!isLoading && filtered.length === 0 && (
          <div
            style={{
              padding: "32px 0",
              textAlign: "center",
              color: "var(--color-text-muted)",
              fontSize: 12,
            }}
          >
            No results
          </div>
        )}
      </div>

      <div
        style={{
          marginTop: 12,
          fontSize: 11,
          color: "var(--color-text-muted)",
        }}
      >
        {filtered.length} payers{filter ? ` (filtered from ${rows.length})` : ""} · all-time ·
        contract = most-tipped interaction
      </div>
    </Drawer>
  );
}

function PayerRow({ rank, row }: { rank: number; row: AllPayerRow }) {
  const isEntity = row.entity_name !== row.from_address;

  return (
    <tr
      style={{
        borderBottom: "1px solid var(--color-border)",
      }}
    >
      <td
        style={{
          padding: "7px 8px",
          color: "var(--color-text-muted)",
          fontVariantNumeric: "tabular-nums",
          width: 32,
        }}
      >
        {rank}
      </td>
      <td style={{ padding: "7px 8px" }}>
        <div style={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {isEntity && (
            <span style={{ color: "var(--color-text-primary)", fontWeight: 500 }}>
              {row.entity_name}
            </span>
          )}
          <span
            title={row.from_address}
            style={{
              fontFamily: "var(--font-mono)",
              fontSize: 11,
              color: isEntity ? "var(--color-text-muted)" : "var(--color-text-primary)",
            }}
          >
            {shortAddr(row.from_address)}
          </span>
        </div>
      </td>
      <td style={{ padding: "7px 8px" }}>
        {row.to_address ? (
          <span
            title={row.to_address}
            style={{
              fontFamily: "var(--font-mono)",
              fontSize: 11,
              color: "var(--color-text-muted)",
            }}
          >
            {shortAddr(row.to_address)}
          </span>
        ) : (
          <span style={{ color: "var(--color-text-muted)" }}>—</span>
        )}
      </td>
      <td style={{ padding: "7px 8px" }}>
        {row.contract_label ? (
          <span
            style={{
              padding: "2px 6px",
              background: "var(--color-bg-base)",
              border: "1px solid var(--color-border)",
              borderRadius: 3,
              fontSize: 11,
              color: "var(--color-text-primary)",
            }}
          >
            {row.contract_label}
          </span>
        ) : (
          <span style={{ color: "var(--color-text-muted)" }}>—</span>
        )}
      </td>
      <td
        style={{
          padding: "7px 8px",
          textAlign: "right",
          fontVariantNumeric: "tabular-nums",
          color: "var(--color-text-primary)",
        }}
      >
        {formatEth(row.tip_eth)}
      </td>
      <td
        style={{
          padding: "7px 8px",
          textAlign: "right",
          fontVariantNumeric: "tabular-nums",
          color: "var(--color-text-muted)",
        }}
      >
        {row.tx_count.toLocaleString()}
      </td>
    </tr>
  );
}
