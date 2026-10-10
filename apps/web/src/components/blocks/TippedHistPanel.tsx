import type { TippedHistRow } from "@/api/types";
import { Panel } from "@/components/ui";
import { useECharts } from "@/lib/use-echarts";
import type { EChartsOption } from "echarts";

interface TippedHistPanelProps {
  data: TippedHistRow[] | null;
  loading?: boolean;
}

export function TippedHistPanel({ data, loading = false }: TippedHistPanelProps) {
  const rows = data ?? [];

  const option: EChartsOption = {
    grid: { left: 40, right: 8, top: 16, bottom: 32, containLabel: false },
    xAxis: {
      type: "category",
      data: rows.map((r) => r.bin),
      axisLabel: { fontSize: 10, color: "#6b7280", interval: 0, rotate: 30 },
      axisLine: { lineStyle: { color: "#1f2329" } },
      axisTick: { show: false },
    },
    yAxis: {
      type: "value",
      axisLabel: { fontSize: 10, color: "#6b7280" },
      splitLine: { lineStyle: { color: "#1f2329" } },
    },
    tooltip: {
      trigger: "axis",
      formatter: (params: unknown) => {
        const p = params as Array<{ name: string; value: number }>;
        if (!p[0]) return "";
        return `${p[0].name}<br/>Blocks: ${p[0].value}`;
      },
    },
    series: [
      {
        type: "bar",
        data: rows.map((r) => r.block_count),
        itemStyle: { color: "#3b82f6", borderRadius: [2, 2, 0, 0] },
        barMaxWidth: 40,
      },
    ],
  };

  const containerRef = useECharts(option, [data]);

  return (
    <Panel title="TIPPED TXS DIST.">
      {loading ? (
        <div className="skeleton" style={{ height: 180, borderRadius: "var(--radius-panel)" }} />
      ) : (
        <div ref={containerRef} style={{ height: 180 }} />
      )}
    </Panel>
  );
}
