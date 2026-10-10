import type { HealthDailyRow } from "@/api/types";
import { Panel } from "@/components/ui";
import { useECharts } from "@/lib/use-echarts";
import type { EChartsOption } from "echarts";

interface SmallChartProps {
  title: string;
  dates: string[];
  data: number[];
  color: string;
  type?: "line" | "bar";
  formatter?: (v: number) => string;
  bands?: boolean;
}

function SmallChart({
  title,
  dates,
  data,
  color,
  type = "line",
  formatter,
  bands,
}: SmallChartProps) {
  const option: EChartsOption = {
    grid: { left: 40, right: 8, top: 16, bottom: 24, containLabel: false },
    xAxis: {
      type: "category",
      data: dates,
      axisLabel: {
        fontSize: 9,
        color: "#6b7280",
        formatter: (v: string) => {
          const d = new Date(v);
          return `${d.getMonth() + 1}/${d.getDate()}`;
        },
        interval: Math.floor(dates.length / 4),
      },
      axisLine: { lineStyle: { color: "#1f2329" } },
      axisTick: { show: false },
    },
    yAxis: {
      type: "value",
      axisLabel: {
        fontSize: 9,
        color: "#6b7280",
        formatter: formatter ?? ((v: number) => v.toFixed(1)),
      },
      splitLine: { lineStyle: { color: "#1f2329" } },
    },
    tooltip: { trigger: "axis" },
    series: [
      {
        type,
        data,
        smooth: false,
        symbol: "none",
        lineStyle: type === "line" ? { color, width: 1.5 } : undefined,
        itemStyle: type === "bar" ? { color: `${color}99` } : undefined,
        areaStyle:
          type === "line"
            ? {
                color: {
                  type: "linear",
                  x: 0,
                  y: 0,
                  x2: 0,
                  y2: 1,
                  colorStops: [
                    { offset: 0, color: `${color}40` },
                    { offset: 1, color: `${color}00` },
                  ],
                },
              }
            : undefined,
        markArea: bands
          ? {
              silent: true,
              data: [
                [{ yAxis: 0, itemStyle: { color: "rgba(34,197,94,0.06)" } }, { yAxis: 1500 }],
                [{ yAxis: 1500, itemStyle: { color: "rgba(245,158,11,0.06)" } }, { yAxis: 2500 }],
                [{ yAxis: 2500, itemStyle: { color: "rgba(239,68,68,0.06)" } }, { yAxis: 10000 }],
              ],
            }
          : undefined,
      },
    ],
  };

  const containerRef = useECharts(option, [data, dates]);

  return (
    <Panel title={title}>
      <div ref={containerRef} style={{ height: 140 }} />
    </Panel>
  );
}

interface TrendsTabProps {
  daily: HealthDailyRow[];
}

export function TrendsTab({ daily }: TrendsTabProps) {
  const dates = daily.map((d) => d.date.slice(0, 10));
  const revData = daily.map((d) => d.tip_revenue_eth);
  const contestedData = daily.map((d) => d.contested_share * 100);
  const hhiData = daily.map((d) => d.hhi);
  const p50Data = daily.map((d) => d.p50_fee_gwei);
  const revertData = daily.map((d) => d.revert_rate_tipped * 100);
  const tippedPerBlock = daily.map((d) =>
    d.block_count > 0 ? d.tipped_tx_count / d.block_count : 0,
  );

  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: "1fr 1fr",
        gap: 8,
      }}
    >
      <SmallChart
        title="DAILY REVENUE (ETH)"
        dates={dates}
        data={revData}
        color="#3b82f6"
        formatter={(v) => v.toFixed(3)}
      />
      <SmallChart
        title="CONTESTED %"
        dates={dates}
        data={contestedData}
        color="#a855f7"
        formatter={(v) => `${v.toFixed(1)}%`}
      />
      <SmallChart
        title="HHI"
        dates={dates}
        data={hhiData}
        color="#f59e0b"
        bands
        formatter={(v) => Math.round(v).toLocaleString()}
      />
      <SmallChart
        title="P50 FEE (GWEI)"
        dates={dates}
        data={p50Data}
        color="#14b8a6"
        formatter={(v) => v.toFixed(2)}
      />
      <SmallChart
        title="REVERT RATE"
        dates={dates}
        data={revertData}
        color="#ef4444"
        formatter={(v) => `${v.toFixed(1)}%`}
      />
      <SmallChart
        title="TIPPED TXS/BLOCK"
        dates={dates}
        data={tippedPerBlock}
        color="#22c55e"
        type="bar"
        formatter={(v) => v.toFixed(1)}
      />
    </div>
  );
}
