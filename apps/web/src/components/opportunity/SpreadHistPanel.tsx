import type { HistogramBin } from "@/api/types";
import { Panel } from "@/components/ui";
import { useECharts } from "@/lib/use-echarts";

interface SpreadHistPanelProps {
  data: HistogramBin[];
  loading?: boolean;
}

export function SpreadHistPanel({ data, loading }: SpreadHistPanelProps) {
  // Aggregate across all pools
  const binMap = new Map<string, { count: number; pct: number }>();
  for (const d of data) {
    const existing = binMap.get(d.bin) ?? { count: 0, pct: 0 };
    binMap.set(d.bin, { count: existing.count + d.block_count, pct: existing.pct + d.block_pct });
  }

  const bins = [...binMap.keys()].sort();
  const counts = bins.map((b) => binMap.get(b)?.count ?? 0);

  // cumulative percentage
  const cumPcts: number[] = [];
  let cum = 0;
  for (const b of bins) {
    cum += (binMap.get(b)?.pct ?? 0) * 100;
    cumPcts.push(cum);
  }

  const option = {
    backgroundColor: "transparent",
    grid: { top: 16, right: 60, bottom: 36, left: 50 },
    xAxis: {
      type: "category" as const,
      data: bins,
      axisLabel: { fontSize: 10, color: "var(--color-text-muted)", rotate: 30 },
      axisLine: { lineStyle: { color: "var(--color-border)" } },
    },
    yAxis: [
      {
        type: "value" as const,
        name: "Blocks",
        nameTextStyle: { fontSize: 10, color: "var(--color-text-muted)" },
        axisLabel: { fontSize: 10, color: "var(--color-text-muted)" },
        axisLine: { lineStyle: { color: "var(--color-border)" } },
        splitLine: { lineStyle: { color: "var(--color-border)", type: "dashed" as const } },
      },
      {
        type: "value" as const,
        name: "Cumulative %",
        nameTextStyle: { fontSize: 10, color: "var(--color-text-muted)" },
        axisLabel: {
          fontSize: 10,
          color: "var(--color-text-muted)",
          formatter: (v: number) => `${v.toFixed(0)}%`,
        },
        axisLine: { lineStyle: { color: "var(--color-border)" } },
        splitLine: { show: false },
        max: 100,
      },
    ],
    series: [
      {
        type: "bar" as const,
        data: counts,
        itemStyle: { color: "var(--color-accent)" },
        yAxisIndex: 0,
      },
      {
        type: "line" as const,
        data: cumPcts,
        smooth: true,
        lineStyle: { color: "#a855f7", width: 2 },
        itemStyle: { color: "#a855f7" },
        symbol: "none",
        yAxisIndex: 1,
      },
    ],
    tooltip: { trigger: "axis" as const },
  };

  const containerRef = useECharts(option, [data]);

  return (
    <Panel title="SPREAD HISTOGRAM">
      {loading ? (
        <div className="skeleton" style={{ height: 200, borderRadius: 4 }} />
      ) : (
        <div ref={containerRef} style={{ height: 200 }} />
      )}
    </Panel>
  );
}
