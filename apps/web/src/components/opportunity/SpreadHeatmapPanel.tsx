import type { HeatmapCell } from "@/api/types";
import { Panel } from "@/components/ui";
import { shortAddress } from "@/lib/formatters";
import { useECharts } from "@/lib/use-echarts";

interface SpreadHeatmapPanelProps {
  data: HeatmapCell[];
  loading?: boolean;
}

export function SpreadHeatmapPanel({ data, loading }: SpreadHeatmapPanelProps) {
  const pools = [...new Set(data.map((d) => d.pool_address))];
  const poolLabels = pools.map((p) => shortAddress(p));

  const seriesData = data.map((d) => [
    d.hour_of_day,
    pools.indexOf(d.pool_address),
    d.median_spread_bps,
  ]);

  const option = {
    backgroundColor: "transparent",
    grid: { top: 16, right: 80, bottom: 40, left: 90 },
    xAxis: {
      type: "category" as const,
      data: Array.from({ length: 24 }, (_, i) => String(i).padStart(2, "0")),
      axisLabel: { fontSize: 10, color: "var(--color-text-muted)" },
      axisLine: { lineStyle: { color: "var(--color-border)" } },
    },
    yAxis: {
      type: "category" as const,
      data: poolLabels,
      axisLabel: { fontSize: 10, color: "var(--color-text-muted)", fontFamily: "monospace" },
      axisLine: { lineStyle: { color: "var(--color-border)" } },
    },
    visualMap: {
      min: 0,
      max: 500,
      calculable: true,
      orient: "vertical" as const,
      right: 0,
      top: "center",
      textStyle: { fontSize: 10, color: "var(--color-text-muted)" },
      inRange: { color: ["#1a1a2e", "#a855f7"] },
    },
    series: [
      {
        type: "heatmap" as const,
        data: seriesData,
        emphasis: { itemStyle: { borderColor: "var(--color-accent)", borderWidth: 1 } },
      },
    ],
    tooltip: {
      formatter: (params: unknown) => {
        const p = params as { value: number[] };
        const [hour, poolIdx, val] = p.value;
        return `${poolLabels[poolIdx as number] ?? poolIdx}<br/>Hour: ${hour}:00 UTC<br/>Spread: ${(val as number).toFixed(1)} bps`;
      },
    },
  };

  const containerRef = useECharts(option, [data]);

  return (
    <Panel title="SPREAD HEATMAP">
      {loading ? (
        <div className="skeleton" style={{ height: 240, borderRadius: 4 }} />
      ) : (
        <div ref={containerRef} style={{ height: 240 }} />
      )}
    </Panel>
  );
}
