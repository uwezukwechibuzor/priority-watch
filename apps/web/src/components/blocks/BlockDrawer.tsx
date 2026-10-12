import type { TxRow } from "@/api/types";
import { Drawer } from "@/components/ui";
import {
  arbiscanBlock,
  arbiscanTx,
  entityColor,
  formatGwei,
  formatTime,
  shortAddress,
} from "@/lib/formatters";
import { useECharts } from "@/lib/use-echarts";
import { blockDetailQuery, blockTxsQuery } from "@/queries/blocks";
import { useQuery } from "@tanstack/react-query";
import type { EChartsOption } from "echarts";

interface BlockDrawerProps {
  blockNumber: number | undefined;
  onClose: () => void;
}

function FeeLadderChart({ txs }: { txs: TxRow[] }) {
  const tipped = txs
    .filter((t) => t.priority_fee_per_gas_gwei > 0)
    .sort((a, b) => a.tip_rank - b.tip_rank);
  if (tipped.length < 2) return null;

  const option: EChartsOption = {
    grid: { left: 8, right: 16, top: 16, bottom: 24, containLabel: true },
    xAxis: {
      type: "category",
      data: tipped.map((t) => `#${t.tip_rank}`),
      axisLabel: { fontSize: 9, color: "#6b7280" },
      axisLine: { lineStyle: { color: "#1f2329" } },
    },
    yAxis: {
      type: "value",
      name: "gwei",
      nameTextStyle: { fontSize: 9, color: "#6b7280" },
      axisLabel: { fontSize: 9, color: "#6b7280" },
      splitLine: { lineStyle: { color: "#1f2329" } },
    },
    tooltip: {
      trigger: "axis",
      formatter: (params: unknown) => {
        const p = (params as { dataIndex: number; value: number }[])[0];
        const tx = tipped[p.dataIndex];
        const pct =
          p.dataIndex === 0
            ? "—"
            : `${((tipped[p.dataIndex].priority_fee_per_gas_gwei / tipped[0].priority_fee_per_gas_gwei) * 100).toFixed(1)}% of rank 1`;
        return [
          `<b>Rank #${tx.tip_rank}</b>`,
          tx.entity_name || shortAddress(tx.from_address),
          `Fee: ${p.value.toFixed(6)} gwei`,
          pct !== "—" ? `2nd/1st: ${pct}` : "",
        ]
          .filter(Boolean)
          .join("<br/>");
      },
    },
    series: [
      {
        type: "line",
        step: "end",
        data: tipped.map((t, i) => ({
          value: t.priority_fee_per_gas_gwei,
          itemStyle: { color: i === 0 ? "#f59e0b" : "#3b82f6" },
        })),
        lineStyle: { color: "#3b82f6", width: 2 },
        symbol: "circle",
        symbolSize: 6,
        areaStyle: { color: "rgba(59,130,246,0.08)" },
        markLine: {
          silent: true,
          lineStyle: { color: "#f59e0b", type: "dashed", width: 1 },
          data: [{ yAxis: tipped[0].priority_fee_per_gas_gwei }],
          label: {
            formatter: "rank 1",
            fontSize: 9,
            color: "#f59e0b",
          },
        },
      },
    ],
  };
  const ref = useECharts(option, [txs]);
  return (
    <div style={{ marginTop: 16 }}>
      <div className="label" style={{ marginBottom: 6 }}>
        FEE LADDER (2ND/1ST:{" "}
        <span style={{ color: "#f59e0b" }}>
          {((tipped[1].priority_fee_per_gas_gwei / tipped[0].priority_fee_per_gas_gwei) * 100).toFixed(1)}%
        </span>
        )
      </div>
      <div ref={ref} style={{ height: 140 }} />
    </div>
  );
}

function TipRevenueChart({ txs }: { txs: TxRow[] }) {
  const tipped = txs
    .filter((t) => t.tip_paid_eth > 0)
    .sort((a, b) => b.tip_paid_eth - a.tip_paid_eth);
  if (tipped.length === 0) return null;

  const totalEth = tipped.reduce((s, t) => s + t.tip_paid_eth, 0);
  const labels = tipped.map((t) => (t.entity_name ? t.entity_name : shortAddress(t.from_address)));
  const chartHeight = Math.max(100, tipped.length * 26 + 40);

  const option: EChartsOption = {
    grid: { left: 8, right: 72, top: 8, bottom: 8, containLabel: true },
    xAxis: {
      type: "value",
      axisLabel: { fontSize: 9, color: "#6b7280", formatter: (v: number) => v.toFixed(6) },
      splitLine: { lineStyle: { color: "#1f2329" } },
      name: "ETH",
      nameTextStyle: { fontSize: 9, color: "#6b7280" },
      nameLocation: "end",
    },
    yAxis: {
      type: "category",
      data: labels,
      inverse: true,
      axisLabel: {
        fontSize: 10,
        color: "#d1d5db",
        width: 110,
        overflow: "truncate",
        fontFamily: "var(--font-mono)",
      },
      axisLine: { show: false },
      axisTick: { show: false },
    },
    tooltip: {
      trigger: "axis",
      axisPointer: { type: "none" },
      formatter: (params: unknown) => {
        const p = (params as { name: string; value: number }[])[0];
        const tx = tipped[labels.indexOf(p.name)];
        const share = totalEth > 0 ? ((p.value / totalEth) * 100).toFixed(1) : "0";
        return [
          `<b>${p.name}</b>`,
          `Tip: ${p.value.toFixed(8)} ETH`,
          `Share: ${share}% of block tips`,
          `Gas: ${tx.gas_used.toLocaleString()}`,
        ].join("<br/>");
      },
    },
    series: [
      {
        type: "bar",
        data: tipped.map((t) => ({
          value: t.tip_paid_eth,
          itemStyle: { color: entityColor(t.color_index) },
        })),
        barMaxWidth: 18,
        label: {
          show: true,
          position: "right",
          fontSize: 9,
          color: "#9ca3af",
          formatter: (p: { value: number }) =>
            totalEth > 0 ? `${((p.value / totalEth) * 100).toFixed(1)}%` : "",
        },
      },
    ],
  };
  const ref = useECharts(option, [txs]);
  return (
    <div style={{ marginTop: 16 }}>
      <div className="label" style={{ marginBottom: 6 }}>
        TIP REVENUE BY PARTICIPANT
      </div>
      <div ref={ref} style={{ height: chartHeight }} />
    </div>
  );
}

function TxClassChart({ txs }: { txs: TxRow[] }) {
  const counts: Record<string, number> = {};
  for (const tx of txs) {
    const cls = tx.tx_class || "unknown";
    counts[cls] = (counts[cls] ?? 0) + 1;
  }
  const entries = Object.entries(counts).sort((a, b) => b[1] - a[1]);
  if (entries.length === 0) return null;

  const classColor: Record<string, string> = {
    arbitrage: "#f59e0b",
    mev: "#ef4444",
    swap: "#3b82f6",
    transfer: "#6b7280",
    liquidation: "#a855f7",
    unknown: "#374151",
  };

  const option: EChartsOption = {
    grid: { left: 8, right: 40, top: 8, bottom: 8, containLabel: true },
    xAxis: {
      type: "value",
      axisLabel: { fontSize: 9, color: "#6b7280" },
      splitLine: { lineStyle: { color: "#1f2329" } },
      minInterval: 1,
    },
    yAxis: {
      type: "category",
      data: entries.map(([cls]) => cls),
      inverse: true,
      axisLabel: { fontSize: 10, color: "#d1d5db", textTransform: "capitalize" },
      axisLine: { show: false },
      axisTick: { show: false },
    },
    tooltip: {
      trigger: "axis",
      formatter: (params: unknown) => {
        const p = (params as { name: string; value: number }[])[0];
        return `${p.name}: ${p.value} tx${p.value !== 1 ? "s" : ""}`;
      },
    },
    series: [
      {
        type: "bar",
        data: entries.map(([cls, count]) => ({
          value: count,
          itemStyle: { color: classColor[cls] ?? "#374151" },
        })),
        barMaxWidth: 18,
        label: {
          show: true,
          position: "right",
          fontSize: 10,
          color: "#9ca3af",
          formatter: (p: { value: number }) => String(p.value),
        },
      },
    ],
  };
  const ref = useECharts(option, [txs]);
  return (
    <div style={{ marginTop: 16 }}>
      <div className="label" style={{ marginBottom: 6 }}>
        TX CLASSIFICATION
      </div>
      <div ref={ref} style={{ height: Math.max(80, entries.length * 28 + 24) }} />
    </div>
  );
}

function BasePriorityStackChart({ txs, baseFeeGwei }: { txs: TxRow[]; baseFeeGwei: number }) {
  const sorted = txs
    .filter((t) => t.gas_used > 0)
    .sort((a, b) => a.tip_rank - b.tip_rank);
  if (sorted.length === 0) return null;

  const labels = sorted.map((t) =>
    t.entity_name ? t.entity_name : shortAddress(t.from_address),
  );
  const baseFees = sorted.map((t) => +(baseFeeGwei * (t.gas_used / 1e9)).toFixed(8));
  const tipFees = sorted.map((t) => +(t.tip_paid_eth));
  const chartHeight = Math.max(100, sorted.length * 26 + 40);

  const option: EChartsOption = {
    grid: { left: 8, right: 16, top: 8, bottom: 8, containLabel: true },
    xAxis: {
      type: "value",
      axisLabel: { fontSize: 9, color: "#6b7280", formatter: (v: number) => v.toFixed(6) },
      splitLine: { lineStyle: { color: "#1f2329" } },
      name: "ETH",
      nameTextStyle: { fontSize: 9, color: "#6b7280" },
      nameLocation: "end",
    },
    yAxis: {
      type: "category",
      data: labels,
      inverse: true,
      axisLabel: {
        fontSize: 10,
        color: "#d1d5db",
        width: 110,
        overflow: "truncate",
        fontFamily: "var(--font-mono)",
      },
      axisLine: { show: false },
      axisTick: { show: false },
    },
    legend: {
      bottom: 0,
      textStyle: { color: "#6b7280", fontSize: 9 },
      itemWidth: 10,
      itemHeight: 8,
    },
    tooltip: {
      trigger: "axis",
      axisPointer: { type: "none" },
      formatter: (params: unknown) => {
        const ps = params as { seriesName: string; value: number }[];
        const base = ps.find((p) => p.seriesName === "Base Fee")?.value ?? 0;
        const tip = ps.find((p) => p.seriesName === "Priority Fee")?.value ?? 0;
        const total = base + tip;
        const pct = total > 0 ? ((tip / total) * 100).toFixed(1) : "0";
        return [
          `Base Fee: ${base.toFixed(8)} ETH`,
          `Priority Fee: ${tip.toFixed(8)} ETH`,
          `PGA share: ${pct}%`,
        ].join("<br/>");
      },
    },
    series: [
      {
        name: "Base Fee",
        type: "bar",
        stack: "total",
        data: baseFees,
        barMaxWidth: 18,
        itemStyle: { color: "#374151" },
      },
      {
        name: "Priority Fee",
        type: "bar",
        stack: "total",
        data: tipFees,
        itemStyle: { color: "#f59e0b" },
      },
    ],
  };
  const ref = useECharts(option, [txs, baseFeeGwei]);
  return (
    <div style={{ marginTop: 16 }}>
      <div className="label" style={{ marginBottom: 6 }}>
        BASE FEE vs PRIORITY FEE
      </div>
      <div ref={ref} style={{ height: chartHeight }} />
    </div>
  );
}

function CumulativeTipChart({ txs }: { txs: TxRow[] }) {
  const sorted = txs
    .filter((t) => t.tip_paid_eth > 0)
    .sort((a, b) => a.tip_rank - b.tip_rank);
  if (sorted.length < 2) return null;

  const total = sorted.reduce((s, t) => s + t.tip_paid_eth, 0);
  let running = 0;
  const cumulative = sorted.map((t) => {
    running += t.tip_paid_eth;
    return +((running / total) * 100).toFixed(2);
  });

  const option: EChartsOption = {
    grid: { left: 8, right: 16, top: 16, bottom: 24, containLabel: true },
    xAxis: {
      type: "category",
      data: sorted.map((t) => `#${t.tip_rank}`),
      axisLabel: { fontSize: 9, color: "#6b7280" },
      axisLine: { lineStyle: { color: "#1f2329" } },
      name: "PGA rank",
      nameTextStyle: { fontSize: 9, color: "#6b7280" },
      nameLocation: "end",
    },
    yAxis: {
      type: "value",
      min: 0,
      max: 100,
      axisLabel: { fontSize: 9, color: "#6b7280", formatter: (v: number) => `${v}%` },
      splitLine: { lineStyle: { color: "#1f2329" } },
    },
    tooltip: {
      trigger: "axis",
      formatter: (params: unknown) => {
        const p = (params as { dataIndex: number; value: number }[])[0];
        const tx = sorted[p.dataIndex];
        return [
          `<b>Rank #${tx.tip_rank}</b>`,
          tx.entity_name || shortAddress(tx.from_address),
          `Cumulative: ${p.value}% of block tips`,
          `This tx: ${tx.tip_paid_eth.toFixed(8)} ETH`,
        ].join("<br/>");
      },
    },
    series: [
      {
        type: "line",
        data: cumulative.map((v, i) => ({
          value: v,
          itemStyle: { color: i === 0 ? "#f59e0b" : "#3b82f6" },
        })),
        lineStyle: { color: "#3b82f6", width: 2 },
        symbol: "circle",
        symbolSize: 6,
        areaStyle: { color: "rgba(59,130,246,0.08)" },
        markLine: {
          silent: true,
          lineStyle: { color: "#6b7280", type: "dashed", width: 1 },
          data: [{ yAxis: 80 }],
          label: { formatter: "80%", fontSize: 9, color: "#6b7280" },
        },
      },
    ],
  };
  const ref = useECharts(option, [txs]);
  return (
    <div style={{ marginTop: 16 }}>
      <div className="label" style={{ marginBottom: 6 }}>
        CUMULATIVE TIP REVENUE{" "}
        <span style={{ color: "#6b7280", textTransform: "none", fontWeight: 400 }}>
          (rank 1 = {cumulative[0]}% of block)
        </span>
      </div>
      <div ref={ref} style={{ height: 140 }} />
    </div>
  );
}

function PGAParticipantsChart({ txs }: { txs: TxRow[] }) {
  const tipped = txs
    .filter((t) => t.priority_fee_per_gas_gwei > 0)
    .sort((a, b) => a.tip_rank - b.tip_rank);

  const labels = tipped.map((t) =>
    t.entity_name ? t.entity_name : shortAddress(t.from_address),
  );
  const fees = tipped.map((t) => t.priority_fee_per_gas_gwei);
  const colors = tipped.map((t) => entityColor(t.color_index));
  const chartHeight = Math.max(120, tipped.length * 28 + 40);

  const option: EChartsOption = {
    grid: { left: 8, right: 56, top: 8, bottom: 8, containLabel: true },
    xAxis: {
      type: "value",
      axisLabel: { fontSize: 9, color: "#6b7280", formatter: (v: number) => `${v.toFixed(4)}` },
      splitLine: { lineStyle: { color: "#1f2329" } },
      name: "gwei",
      nameTextStyle: { fontSize: 9, color: "#6b7280" },
      nameLocation: "end",
    },
    yAxis: {
      type: "category",
      data: labels,
      inverse: true,
      axisLabel: {
        fontSize: 10,
        color: "#d1d5db",
        width: 110,
        overflow: "truncate",
        fontFamily: "var(--font-mono)",
      },
      axisLine: { show: false },
      axisTick: { show: false },
    },
    tooltip: {
      trigger: "axis",
      axisPointer: { type: "none" },
      formatter: (params: unknown) => {
        const p = (params as { name: string; value: number }[])[0];
        const tx = tipped[labels.indexOf(p.name)];
        return [
          `<b>Rank #${tx.tip_rank}</b>`,
          tx.entity_name ? `Entity: ${tx.entity_name}` : `From: ${tx.from_address}`,
          `Fee: ${p.value.toFixed(6)} gwei`,
          `Gas: ${tx.gas_used.toLocaleString()}`,
          `Status: ${tx.status === 1 ? "✓" : "✗"}`,
        ].join("<br/>");
      },
    },
    series: [
      {
        type: "bar",
        data: fees.map((v, i) => ({ value: v, itemStyle: { color: colors[i] } })),
        barMaxWidth: 18,
        label: {
          show: true,
          position: "right",
          fontSize: 9,
          color: "#9ca3af",
          formatter: (p: { value: number }) => p.value.toFixed(4),
        },
      },
    ],
  };
  const ref = useECharts(option, [txs]);
  if (tipped.length === 0) return null;
  return (
    <div style={{ marginTop: 16 }}>
      <div className="label" style={{ marginBottom: 6 }}>
        PGA PARTICIPANTS ({tipped.length})
      </div>
      <div ref={ref} style={{ height: chartHeight }} />
    </div>
  );
}

export function BlockDrawer({ blockNumber, onClose }: BlockDrawerProps) {
  const open = blockNumber !== undefined;

  const { data: block } = useQuery({
    ...blockDetailQuery(blockNumber ?? 0),
    enabled: open,
  });
  const { data: txsPage } = useQuery({
    ...blockTxsQuery(blockNumber ?? 0),
    enabled: open,
  });

  const txs = txsPage?.data ?? [];

  return (
    <Drawer open={open} onClose={onClose} title={`BLOCK #${blockNumber ?? ""}`} width={480}>
      {block && (
        <div>
          {/* Header metrics */}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gap: 8,
              marginBottom: 12,
            }}
          >
            <div>
              <div className="label" style={{ marginBottom: 2 }}>
                Timestamp
              </div>
              <div style={{ fontSize: 12 }}>{formatTime(block.block_timestamp)}</div>
            </div>
            <div>
              <div className="label" style={{ marginBottom: 2 }}>
                Base Fee
              </div>
              <div className="tabular" style={{ fontSize: 12 }}>
                {formatGwei(block.base_fee_per_gas_gwei)}
              </div>
            </div>
            <div>
              <div className="label" style={{ marginBottom: 2 }}>
                Gas Used
              </div>
              <div className="tabular" style={{ fontSize: 12 }}>
                {block.gas_used.toLocaleString()} / {block.gas_limit.toLocaleString()}
              </div>
            </div>
            <div>
              <div className="label" style={{ marginBottom: 2 }}>
                Gas Used %
              </div>
              <div className="tabular" style={{ fontSize: 12 }}>
                {(block.gas_used_pct * 100).toFixed(1)}%
              </div>
            </div>
          </div>

          {/* Arbiscan link */}
          <a
            href={arbiscanBlock(block.block_number)}
            target="_blank"
            rel="noreferrer"
            style={{
              fontSize: 12,
              color: "var(--color-accent)",
              display: "inline-block",
              marginBottom: 16,
            }}
          >
            View on Arbiscan ↗
          </a>

          {/* PGA participants chart */}
          {txs.length > 0 && <PGAParticipantsChart txs={txs} />}
          {/* Fee ladder — step chart showing fee drop across ranks */}
          {txs.length > 0 && <FeeLadderChart txs={txs} />}
          {/* Tip revenue — who actually paid the DAO the most */}
          {txs.length > 0 && <TipRevenueChart txs={txs} />}
          {/* Tx classification — what types of activity competed */}
          {txs.length > 0 && <TxClassChart txs={txs} />}
          {/* Base fee vs priority fee stacked per tx */}
          {txs.length > 0 && block && <BasePriorityStackChart txs={txs} baseFeeGwei={block.base_fee_per_gas_gwei} />}
          {/* Cumulative tip revenue waterfall */}
          {txs.length > 0 && <CumulativeTipChart txs={txs} />}

          {/* Tx table */}
          <div style={{ marginTop: 16 }}>
            <div className="label" style={{ marginBottom: 6 }}>
              Transactions ({txs.length})
            </div>
            <div style={{ overflowX: "auto" }}>
              <table
                style={{
                  width: "100%",
                  borderCollapse: "collapse",
                  fontSize: 11,
                }}
              >
                <thead>
                  <tr>
                    {["#", "TX", "FROM", "FEE (GWEI)", "GAS", "ST"].map((h) => (
                      <th
                        key={h}
                        style={{
                          padding: "4px 6px",
                          textAlign: "left",
                          borderBottom: "1px solid var(--color-border)",
                          fontWeight: 500,
                          textTransform: "uppercase",
                          letterSpacing: "0.06em",
                          fontSize: 10,
                          color: "var(--color-text-muted)",
                          whiteSpace: "nowrap",
                        }}
                      >
                        {h}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {txs.map((tx) => (
                    <tr key={tx.tx_hash} style={{ borderBottom: "1px solid var(--color-border)" }}>
                      <td
                        className="tabular"
                        style={{ padding: "4px 6px", color: "var(--color-text-muted)" }}
                      >
                        {tx.tip_rank}
                      </td>
                      <td style={{ padding: "4px 6px" }}>
                        <a
                          href={arbiscanTx(tx.tx_hash)}
                          target="_blank"
                          rel="noreferrer"
                          style={{ color: "var(--color-accent)", fontFamily: "var(--font-mono)" }}
                        >
                          {tx.tx_hash.slice(0, 8)}…
                        </a>
                      </td>
                      <td style={{ padding: "4px 6px" }}>
                        <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
                          <span
                            aria-hidden="true"
                            style={{
                              width: 6,
                              height: 6,
                              borderRadius: 1,
                              background: entityColor(tx.color_index),
                              flexShrink: 0,
                            }}
                          />
                          <span
                            className="mono"
                            style={{
                              fontSize: 11,
                              overflow: "hidden",
                              textOverflow: "ellipsis",
                              whiteSpace: "nowrap",
                              maxWidth: 90,
                            }}
                          >
                            {tx.entity_name || shortAddress(tx.from_address)}
                          </span>
                        </span>
                      </td>
                      <td className="tabular" style={{ padding: "4px 6px", textAlign: "right" }}>
                        {tx.priority_fee_per_gas_gwei.toFixed(4)}
                      </td>
                      <td className="tabular" style={{ padding: "4px 6px", textAlign: "right" }}>
                        {tx.gas_used.toLocaleString()}
                      </td>
                      <td
                        style={{
                          padding: "4px 6px",
                          textAlign: "center",
                          color: tx.status === 1 ? "var(--color-green)" : "var(--color-red)",
                        }}
                      >
                        {tx.status === 1 ? "✓" : "✗"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}
    </Drawer>
  );
}
