import type { EventRow, SeriesPoint } from "@/api/types";
import { Panel } from "@/components/ui";
import { useECharts } from "@/lib/use-echarts";
import type { EChartsOption } from "echarts";
import { useState } from "react";

interface TipRevenueChartProps {
  series: SeriesPoint[] | null;
  events: EventRow[] | null;
  loading?: boolean;
}

type Mode = "revenue" | "fee";

const EVENT_COLORS: Record<string, string> = {
  timeboost_launch: "#f59e0b",
  pga_activation: "#22c55e",
  sc_pause: "#ef4444",
};

function eventColor(id: string): string {
  return EVENT_COLORS[id] ?? "#6b7280";
}

export function TipRevenueChart({ series, events, loading = false }: TipRevenueChartProps) {
  const [mode, setMode] = useState<Mode>("revenue");

  const data = series ?? [];
  const xData = data.map((p) => p.ts);
  const yData =
    mode === "revenue" ? data.map((p) => p.tip_revenue_eth) : data.map((p) => p.top_fee_gwei);

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
    grid: { left: 48, right: 16, top: 32, bottom: 32, containLabel: false },
    xAxis: {
      type: "category",
      data: xData,
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
        formatter: (v: number) => (mode === "revenue" ? `${v.toFixed(2)}` : `${v.toFixed(0)}`),
      },
      splitLine: { lineStyle: { color: "#1f2329" } },
    },
    tooltip: {
      trigger: "axis",
      formatter: (params: unknown) => {
        const p = params as Array<{ name: string; value: number }>;
        if (!p[0]) return "";
        const val =
          mode === "revenue" ? `${p[0].value.toFixed(4)} ETH` : `${p[0].value.toFixed(2)} gwei`;
        return `${p[0].name}<br/>${val}`;
      },
    },
    series: [
      {
        type: "line",
        data: yData,
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
          markLines.length > 0
            ? {
                silent: true,
                data: markLines,
                symbol: "none",
              }
            : undefined,
      },
    ],
  };

  const containerRef = useECharts(option, [mode, series, events]);

  const btnStyle = (active: boolean): React.CSSProperties => ({
    padding: "2px 8px",
    fontSize: 11,
    background: active ? "var(--color-accent)" : "transparent",
    color: active ? "#fff" : "var(--color-text-muted)",
    border: "1px solid",
    borderColor: active ? "var(--color-accent)" : "var(--color-border)",
    borderRadius: "var(--radius-badge)",
    cursor: "pointer",
  });

  return (
    <Panel
      title="TIP REVENUE"
      action={
        <div style={{ display: "flex", gap: 4 }}>
          <button
            type="button"
            style={btnStyle(mode === "revenue")}
            onClick={() => setMode("revenue")}
          >
            Revenue
          </button>
          <button type="button" style={btnStyle(mode === "fee")} onClick={() => setMode("fee")}>
            Top Fee
          </button>
        </div>
      }
    >
      {loading ? (
        <div className="skeleton" style={{ height: 180, borderRadius: "var(--radius-panel)" }} />
      ) : (
        <div ref={containerRef} style={{ height: 180 }} />
      )}
    </Panel>
  );
}
