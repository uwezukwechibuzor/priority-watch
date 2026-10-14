import type { EventRow, HealthDailyRow, PayerShareRow } from "@/api/types";
import { TxStatusChart } from "@/components/health/TxStatusChart";
import { type Column, DataTable, Panel } from "@/components/ui";
import { entityColor, formatEth, formatPct } from "@/lib/formatters";
import { useECharts } from "@/lib/use-echarts";
import type { EChartsOption } from "echarts";

const EVENT_COLORS: Record<string, string> = {
  timeboost_launch: "#f59e0b",
  pga_activation: "#22c55e",
  sc_pause: "#ef4444",
};

function eventColor(id: string): string {
  return EVENT_COLORS[id] ?? "#6b7280";
}

interface OverviewTabProps {
  daily: HealthDailyRow[];
  concentration: HealthDailyRow[];
  competition: HealthDailyRow[];
  leaderboard: PayerShareRow[];
  events: EventRow[] | null;
}

function RevenueChart({
  daily,
  events,
}: {
  daily: HealthDailyRow[];
  events: EventRow[] | null;
}) {
  const dates = daily.map((d) => d.date.slice(0, 10));
  const revData = daily.map((d) => d.tip_revenue_eth);
  const txData = daily.map((d) => d.tipped_tx_count);

  const markLines =
    events?.map((ev) => ({
      xAxis: ev.event_date,
      lineStyle: { color: eventColor(ev.id), type: "dashed" as const, width: 1 },
      label: {
        show: true,
        formatter: ev.label,
        fontSize: 10,
        color: eventColor(ev.id),
        position: "insideEndTop" as const,
      },
    })) ?? [];

  const option: EChartsOption = {
    grid: { left: 56, right: 56, top: 24, bottom: 32, containLabel: false },
    xAxis: {
      type: "category",
      data: dates,
      axisLabel: {
        fontSize: 10,
        color: "#6b7280",
        formatter: (v: string) => {
          const d = new Date(v);
          return `${d.getMonth() + 1}/${d.getDate()}`;
        },
      },
      axisLine: { lineStyle: { color: "#1f2329" } },
      axisTick: { show: false },
    },
    yAxis: [
      {
        type: "value",
        name: "ETH",
        nameTextStyle: { fontSize: 10, color: "#6b7280" },
        axisLabel: { fontSize: 10, color: "#6b7280", formatter: (v: number) => v.toFixed(2) },
        splitLine: { lineStyle: { color: "#1f2329" } },
      },
      {
        type: "value",
        name: "Tipped Txs",
        nameTextStyle: { fontSize: 10, color: "#6b7280" },
        axisLabel: {
          fontSize: 10,
          color: "#6b7280",
          formatter: (v: number) => (v >= 1000 ? `${(v / 1000).toFixed(0)}K` : String(v)),
        },
        splitLine: { show: false },
      },
    ],
    tooltip: { trigger: "axis" },
    legend: { show: false },
    series: [
      {
        name: "Revenue (ETH)",
        type: "line",
        data: revData,
        smooth: false,
        symbol: "none",
        lineStyle: { color: "#3b82f6", width: 1.5 },
        areaStyle: {
          color: {
            type: "linear",
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: "rgba(59,130,246,0.25)" },
              { offset: 1, color: "rgba(59,130,246,0)" },
            ],
          },
        },
        markLine:
          markLines.length > 0 ? { silent: true, data: markLines, symbol: "none" } : undefined,
        yAxisIndex: 0,
      },
      {
        name: "Tipped Txs",
        type: "line",
        data: txData,
        smooth: false,
        symbol: "none",
        lineStyle: { color: "#a855f7", width: 1.5 },
        yAxisIndex: 1,
      },
    ],
  };

  const containerRef = useECharts(option, [daily, events]);

  return (
    <Panel title="REVENUE + THROUGHPUT">
      <div ref={containerRef} style={{ height: 200 }} />
    </Panel>
  );
}

function HhiChart({ concentration }: { concentration: HealthDailyRow[] }) {
  const dates = concentration.map((d) => d.date.slice(0, 10));
  const hhiData = concentration.map((d) => d.hhi);

  const option: EChartsOption = {
    grid: { left: 56, right: 16, top: 24, bottom: 32, containLabel: false },
    xAxis: {
      type: "category",
      data: dates,
      axisLabel: {
        fontSize: 10,
        color: "#6b7280",
        formatter: (v: string) => {
          const d = new Date(v);
          return `${d.getMonth() + 1}/${d.getDate()}`;
        },
      },
      axisLine: { lineStyle: { color: "#1f2329" } },
      axisTick: { show: false },
    },
    yAxis: {
      type: "value",
      min: 0,
      max: 10000,
      axisLabel: {
        fontSize: 10,
        color: "#6b7280",
        formatter: (v: number) => v.toLocaleString(),
      },
      splitLine: { lineStyle: { color: "#1f2329" } },
    },
    tooltip: {
      trigger: "axis",
      formatter: (params: unknown) => {
        const p = params as Array<{ name: string; value: number }>;
        if (!p[0]) return "";
        return `${p[0].name}<br/>HHI: ${Math.round(p[0].value).toLocaleString()}`;
      },
    },
    series: [
      {
        name: "HHI",
        type: "line",
        data: hhiData,
        smooth: false,
        symbol: "none",
        lineStyle: { color: "#f59e0b", width: 1.5 },
        markArea: {
          silent: true,
          data: [
            [{ yAxis: 0, itemStyle: { color: "rgba(34,197,94,0.06)" } }, { yAxis: 1500 }],
            [{ yAxis: 1500, itemStyle: { color: "rgba(245,158,11,0.06)" } }, { yAxis: 2500 }],
            [{ yAxis: 2500, itemStyle: { color: "rgba(239,68,68,0.06)" } }, { yAxis: 10000 }],
          ],
        },
      },
    ],
  };

  const containerRef = useECharts(option, [concentration]);

  return (
    <Panel title="CONCENTRATION (HHI)">
      <div ref={containerRef} style={{ height: 200 }} />
    </Panel>
  );
}

function CompetitionChart({ competition }: { competition: HealthDailyRow[] }) {
  const dates = competition.map((d) => d.date.slice(0, 10));
  const tippersProxy = competition.map((d) =>
    d.block_count > 0 ? d.tipped_tx_count / d.block_count : 0,
  );
  const p10Data = competition.map((d) => d.p10_fee_gwei);
  const p90Data = competition.map((d) => d.p90_fee_gwei - d.p10_fee_gwei);

  const option: EChartsOption = {
    grid: { left: 56, right: 56, top: 24, bottom: 32, containLabel: false },
    xAxis: {
      type: "category",
      data: dates,
      axisLabel: {
        fontSize: 10,
        color: "#6b7280",
        formatter: (v: string) => {
          const d = new Date(v);
          return `${d.getMonth() + 1}/${d.getDate()}`;
        },
      },
      axisLine: { lineStyle: { color: "#1f2329" } },
      axisTick: { show: false },
    },
    yAxis: [
      {
        type: "value",
        name: "Txs/Block",
        nameTextStyle: { fontSize: 10, color: "#6b7280" },
        axisLabel: { fontSize: 10, color: "#6b7280", formatter: (v: number) => v.toFixed(1) },
        splitLine: { lineStyle: { color: "#1f2329" } },
      },
      {
        type: "value",
        name: "gwei",
        nameTextStyle: { fontSize: 10, color: "#6b7280" },
        axisLabel: { fontSize: 10, color: "#6b7280", formatter: (v: number) => v.toFixed(0) },
        splitLine: { show: false },
      },
    ],
    tooltip: { trigger: "axis" },
    series: [
      {
        name: "Tipped Txs/Block",
        type: "bar",
        data: tippersProxy,
        itemStyle: { color: "rgba(59,130,246,0.6)" },
        yAxisIndex: 0,
      },
      {
        name: "P10 Fee",
        type: "line",
        data: p10Data,
        smooth: false,
        symbol: "none",
        lineStyle: { color: "transparent" },
        areaStyle: { color: "rgba(168,85,247,0.15)" },
        stack: "fee-band",
        yAxisIndex: 1,
      },
      {
        name: "P10–P90 Band",
        type: "line",
        data: p90Data,
        smooth: false,
        symbol: "none",
        lineStyle: { color: "#a855f7", width: 1 },
        areaStyle: { color: "rgba(168,85,247,0.2)" },
        stack: "fee-band",
        yAxisIndex: 1,
      },
    ],
  };

  const containerRef = useECharts(option, [competition]);

  return (
    <Panel title="COMPETITION + FEE BAND">
      <div ref={containerRef} style={{ height: 200 }} />
    </Panel>
  );
}

function LeaderboardTable({ rows }: { rows: PayerShareRow[] }) {
  const columns: Column<PayerShareRow>[] = [
    {
      key: "entity",
      header: "Entity",
      minWidth: 180,
      render: (row) => (
        <span style={{ display: "flex", alignItems: "center", gap: 6, minWidth: 0 }}>
          <span
            style={{
              width: 8,
              height: 8,
              borderRadius: "50%",
              background: entityColor(row.color_index),
              flexShrink: 0,
            }}
          />
          <span
            style={{
              fontSize: 12,
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
            }}
          >
            {row.entity_name}
          </span>
        </span>
      ),
    },
    {
      key: "tips",
      header: "Tips (ETH)",
      width: 130,
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
      width: 100,
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
      width: 100,
      align: "right",
      render: (row) => (
        <span className="tabular" style={{ fontSize: 12 }}>
          {row.avg_tip_rank.toFixed(1)}
        </span>
      ),
    },
  ];

  return (
    <Panel title="PAYERS LEADERBOARD">
      <DataTable columns={columns} rows={rows} rowKey={(r) => r.entity_name} maxHeight={320} />
    </Panel>
  );
}

export function OverviewTab({
  daily,
  concentration,
  competition,
  leaderboard,
  events,
}: OverviewTabProps) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      <RevenueChart daily={daily} events={events} />
      <TxStatusChart />
      <HhiChart concentration={concentration} />
      <CompetitionChart competition={competition} />
      <LeaderboardTable rows={leaderboard} />
    </div>
  );
}
