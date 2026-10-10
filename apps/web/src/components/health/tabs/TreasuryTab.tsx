import type { TreasuryDailyRow, TreasurySummary } from "@/api/types";
import { type Column, DataTable, KpiTile, Panel } from "@/components/ui";
import { formatDate, formatEth, formatUsd } from "@/lib/formatters";
import { useECharts } from "@/lib/use-echarts";
import type { EChartsOption } from "echarts";

interface TreasuryTabProps {
  daily: TreasuryDailyRow[];
  summary: TreasurySummary | undefined;
  loading: boolean;
}

function StatusBadge({ status }: { status: string }) {
  let color = "var(--color-green)";
  let bg = "rgba(34,197,94,0.1)";
  if (status === "warn") {
    color = "var(--color-amber)";
    bg = "rgba(245,158,11,0.1)";
  } else if (status === "fail") {
    color = "var(--color-red)";
    bg = "rgba(239,68,68,0.1)";
  }
  return (
    <span
      style={{
        padding: "2px 6px",
        borderRadius: 4,
        fontSize: 11,
        background: bg,
        color,
        fontWeight: 500,
      }}
    >
      {status}
    </span>
  );
}

function CumulativeChart({ daily }: { daily: TreasuryDailyRow[] }) {
  const dates = daily.map((d) => d.date.slice(0, 10));
  let cumTreasury = 0;
  let cumGuild = 0;
  const treasuryData: number[] = [];
  const guildData: number[] = [];

  for (const d of daily) {
    cumTreasury += d.total_tip_eth * 0.97;
    cumGuild += d.total_tip_eth * 0.03;
    treasuryData.push(Number(cumTreasury.toFixed(6)));
    guildData.push(Number(cumGuild.toFixed(6)));
  }

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
      axisLabel: {
        fontSize: 10,
        color: "#6b7280",
        formatter: (v: number) => v.toFixed(2),
      },
      splitLine: { lineStyle: { color: "#1f2329" } },
    },
    tooltip: { trigger: "axis" },
    series: [
      {
        name: "Treasury (97%)",
        type: "line",
        data: treasuryData,
        smooth: false,
        symbol: "none",
        lineStyle: { color: "#22c55e", width: 1.5 },
        areaStyle: {
          color: {
            type: "linear",
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: "rgba(34,197,94,0.25)" },
              { offset: 1, color: "rgba(34,197,94,0)" },
            ],
          },
        },
        stack: "cumulative",
      },
      {
        name: "Developer Guild (3%)",
        type: "line",
        data: guildData,
        smooth: false,
        symbol: "none",
        lineStyle: { color: "#a855f7", width: 1.5 },
        areaStyle: {
          color: {
            type: "linear",
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: "rgba(168,85,247,0.25)" },
              { offset: 1, color: "rgba(168,85,247,0)" },
            ],
          },
        },
        stack: "cumulative",
      },
    ],
  };

  const containerRef = useECharts(option, [daily]);

  return (
    <Panel title="CUMULATIVE TIP REVENUE">
      <div ref={containerRef} style={{ height: 200 }} />
    </Panel>
  );
}

function DailyTable({ daily, loading }: { daily: TreasuryDailyRow[]; loading: boolean }) {
  const columns: Column<TreasuryDailyRow>[] = [
    {
      key: "date",
      header: "Date",
      width: 100,
      render: (row) => (
        <span className="tabular" style={{ fontSize: 12 }}>
          {formatDate(row.date)}
        </span>
      ),
    },
    {
      key: "total",
      header: "Total Tips ETH",
      width: 120,
      align: "right",
      render: (row) => (
        <span className="tabular" style={{ fontSize: 12 }}>
          {formatEth(row.total_tip_eth)}
        </span>
      ),
    },
    {
      key: "treasury",
      header: "Treasury (97%)",
      width: 120,
      align: "right",
      render: (row) => (
        <span className="tabular" style={{ fontSize: 12 }}>
          {formatEth(row.total_tip_eth * 0.97)}
        </span>
      ),
    },
    {
      key: "guild",
      header: "Dev Guild (3%)",
      width: 120,
      align: "right",
      render: (row) => (
        <span className="tabular" style={{ fontSize: 12 }}>
          {formatEth(row.total_tip_eth * 0.03)}
        </span>
      ),
    },
    {
      key: "status",
      header: "Status",
      width: 70,
      render: (row) => <StatusBadge status={row.status} />,
    },
  ];

  return (
    <Panel title="DAILY BREAKDOWN">
      <DataTable
        columns={columns}
        rows={daily}
        rowKey={(r) => r.date}
        maxHeight={400}
        loading={loading}
      />
    </Panel>
  );
}

export function TreasuryTab({ daily, summary, loading }: TreasuryTabProps) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      {/* Summary cards */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 8 }}>
        <KpiTile
          label="TOTAL TIPS"
          value={summary ? formatEth(summary.total_tip_eth) : "—"}
          subtext={summary ? formatUsd(summary.total_tip_usd) : undefined}
          loading={loading}
        />
        <KpiTile
          label="TREASURY (97%)"
          value={summary ? formatEth(summary.treasury_eth) : "—"}
          subtext="DAO treasury"
          loading={loading}
        />
        <KpiTile
          label="DEVELOPER GUILD (3%)"
          value={summary ? formatEth(summary.developer_guild_eth) : "—"}
          subtext="Arbitrum Developer Guild"
          loading={loading}
        />
      </div>

      {/* Cumulative chart */}
      <CumulativeChart daily={daily} />

      {/* Daily table */}
      <DailyTable daily={daily} loading={loading} />
    </div>
  );
}
