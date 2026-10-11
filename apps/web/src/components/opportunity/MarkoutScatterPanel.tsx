import type { MarkoutRow } from "@/api/types";
import { Panel } from "@/components/ui";
import { useECharts } from "@/lib/use-echarts";

interface MarkoutScatterPanelProps {
  data: MarkoutRow[];
  loading?: boolean;
}

export function MarkoutScatterPanel({ data, loading }: MarkoutScatterPanelProps) {
  const series1s = data
    .filter((d) => d.markout_usd_1s != null)
    .map((d) => [d.exec_price_usd, d.markout_usd_1s as number]);

  const series300s = data
    .filter((d) => d.markout_usd_300s != null)
    .map((d) => [d.exec_price_usd, d.markout_usd_300s as number]);

  const option = {
    backgroundColor: "transparent",
    grid: { top: 32, right: 16, bottom: 40, left: 60 },
    legend: {
      top: 4,
      right: 16,
      textStyle: { fontSize: 11, color: "var(--color-text-muted)" },
    },
    xAxis: {
      type: "value" as const,
      name: "Exec Price (USD)",
      nameTextStyle: { fontSize: 10, color: "var(--color-text-muted)" },
      axisLabel: {
        fontSize: 10,
        color: "var(--color-text-muted)",
        formatter: (v: number) => `$${v.toFixed(0)}`,
      },
      axisLine: { lineStyle: { color: "var(--color-border)" } },
      splitLine: { lineStyle: { color: "var(--color-border)", type: "dashed" as const } },
    },
    yAxis: {
      type: "value" as const,
      name: "Markout P&L (USD)",
      nameTextStyle: { fontSize: 10, color: "var(--color-text-muted)" },
      axisLabel: {
        fontSize: 10,
        color: "var(--color-text-muted)",
        formatter: (v: number) => `$${v.toFixed(2)}`,
      },
      axisLine: { lineStyle: { color: "var(--color-border)" } },
      splitLine: { lineStyle: { color: "var(--color-border)", type: "dashed" as const } },
    },
    series: [
      {
        name: "1s",
        type: "scatter" as const,
        data: series1s,
        symbolSize: 4,
        itemStyle: { color: "var(--color-accent)", opacity: 0.6 },
      },
      {
        name: "300s",
        type: "scatter" as const,
        data: series300s,
        symbolSize: 4,
        itemStyle: { color: "#a855f7", opacity: 0.6 },
      },
      {
        // zero line
        type: "line" as const,
        markLine: {
          silent: true,
          data: [{ yAxis: 0 }],
          lineStyle: { color: "var(--color-text-muted)", type: "dashed" as const },
          label: { show: false },
        },
        data: [],
      },
    ],
    tooltip: {
      trigger: "item" as const,
      formatter: (params: unknown) => {
        const p = params as { seriesName: string; value: number[] };
        return `${p.seriesName}<br/>Price: $${(p.value[0] ?? 0).toFixed(2)}<br/>Markout: $${(p.value[1] ?? 0).toFixed(4)}`;
      },
    },
  };

  const containerRef = useECharts(option, [data]);

  return (
    <Panel title="MARKOUT SCATTER (1s vs 300s)">
      {loading ? (
        <div className="skeleton" style={{ height: 220, borderRadius: 4 }} />
      ) : (
        <div ref={containerRef} style={{ height: 220 }} />
      )}
    </Panel>
  );
}
