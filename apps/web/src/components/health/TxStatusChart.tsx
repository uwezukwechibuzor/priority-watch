import { Panel } from "@/components/ui";
import { useECharts } from "@/lib/use-echarts";
import { txStatusQuery } from "@/queries/health";
import { useQuery } from "@tanstack/react-query";
import type { EChartsOption } from "echarts";
import { useMemo, useState } from "react";

const RANGES = ["1h", "24h", "7d", "30d", "all"] as const;
type Range = (typeof RANGES)[number];

function formatTs(ts: string, range: Range): string {
  const d = new Date(ts);
  if (range === "1h") {
    return `${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}`;
  }
  if (range === "24h") {
    return `${d.getHours().toString().padStart(2, "0")}:00`;
  }
  if (range === "7d") {
    return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, "0")}h`;
  }
  return `${d.getMonth() + 1}/${d.getDate()}`;
}

function fmtCount(v: number): string {
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
  if (v >= 1_000) return `${(v / 1_000).toFixed(0)}K`;
  return String(v);
}

interface ChartInnerProps {
  timestamps: string[];
  successData: number[];
  failData: number[];
}

function ChartInner({ timestamps, successData, failData }: ChartInnerProps) {
  const option = useMemo<EChartsOption>(
    () => ({
      grid: { left: 52, right: 16, top: 32, bottom: 32, containLabel: false },
      xAxis: {
        type: "category",
        data: timestamps,
        boundaryGap: false,
        axisLabel: {
          fontSize: 9,
          color: "#6b7280",
          interval: Math.max(0, Math.floor(timestamps.length / 8) - 1),
        },
        axisLine: { lineStyle: { color: "#1f2329" } },
        axisTick: { show: false },
      },
      yAxis: {
        type: "value",
        axisLabel: {
          fontSize: 9,
          color: "#6b7280",
          formatter: fmtCount,
        },
        splitLine: { lineStyle: { color: "#1f2329" } },
      },
      legend: {
        data: ["Success", "Failed"],
        right: 8,
        top: 4,
        itemWidth: 14,
        itemHeight: 2,
        textStyle: { color: "#9ca3af", fontSize: 10 },
      },
      tooltip: {
        trigger: "axis",
        backgroundColor: "#1a1f2e",
        borderColor: "#374151",
        textStyle: { color: "#e5e7eb", fontSize: 11 },
        formatter: (raw: unknown) => {
          const params = raw as Array<{ seriesName: string; value: number; name: string }>;
          if (!params[0]) return "";
          const lines = params.map(
            (p) =>
              `<span style="color:${p.seriesName === "Success" ? "#22d3ee" : "#f97316"}">${p.seriesName}</span>: ${p.value.toLocaleString()}`,
          );
          return `<b>${params[0].name}</b><br/>${lines.join("<br/>")}`;
        },
      },
      series: [
        {
          name: "Success",
          type: "line",
          data: successData,
          smooth: false,
          symbol: "none",
          lineStyle: { color: "#22d3ee", width: 1.5 },
          areaStyle: {
            color: {
              type: "linear",
              x: 0,
              y: 0,
              x2: 0,
              y2: 1,
              colorStops: [
                { offset: 0, color: "rgba(34,211,238,0.18)" },
                { offset: 1, color: "rgba(34,211,238,0)" },
              ],
            },
          },
        },
        {
          name: "Failed",
          type: "line",
          data: failData,
          smooth: false,
          symbol: "none",
          lineStyle: { color: "#f97316", width: 1.5 },
          areaStyle: {
            color: {
              type: "linear",
              x: 0,
              y: 0,
              x2: 0,
              y2: 1,
              colorStops: [
                { offset: 0, color: "rgba(249,115,22,0.18)" },
                { offset: 1, color: "rgba(249,115,22,0)" },
              ],
            },
          },
        },
      ],
    }),
    [timestamps, successData, failData],
  );

  const ref = useECharts(option, [option]);
  return <div ref={ref} style={{ height: 200 }} />;
}

export function TxStatusChart() {
  const [range, setRange] = useState<Range>("24h");
  const { data: rows = [], isLoading } = useQuery(txStatusQuery(range));

  const timestamps = rows.map((r) => formatTs(r.ts, range));
  const successData = rows.map((r) => r.success_count);
  const failData = rows.map((r) => r.fail_count);

  return (
    <Panel
      title="TX STATUS — SUCCESS vs FAILED"
      action={
        <div style={{ display: "flex", gap: 2 }}>
          {RANGES.map((r) => (
            <button
              key={r}
              type="button"
              onClick={() => setRange(r)}
              style={{
                padding: "2px 7px",
                fontSize: 10,
                fontWeight: 500,
                background: range === r ? "var(--color-accent)" : "transparent",
                color: range === r ? "#fff" : "var(--color-text-muted)",
                border: "1px solid",
                borderColor: range === r ? "var(--color-accent)" : "var(--color-border)",
                borderRadius: "var(--radius-badge)",
                cursor: "pointer",
                textTransform: "uppercase",
                letterSpacing: "0.04em",
              }}
            >
              {r}
            </button>
          ))}
        </div>
      }
    >
      {isLoading ? (
        <div
          style={{
            height: 200,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            color: "var(--color-text-muted)",
            fontSize: 12,
          }}
        >
          Loading…
        </div>
      ) : (
        <ChartInner timestamps={timestamps} successData={successData} failData={failData} />
      )}
    </Panel>
  );
}
