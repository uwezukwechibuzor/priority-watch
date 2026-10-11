import type { ReplayResult } from "@/api/types";
import { Panel } from "@/components/ui";
import { useECharts } from "@/lib/use-echarts";

const RUN_COLORS = ["#3b82f6", "#f59e0b", "#22c55e", "#ef4444"];

export interface SimRun {
  id: string;
  label: string;
  colorIndex: number;
  status: "pending" | "running" | "done" | "error";
  result?: ReplayResult;
  errorMsg?: string;
}

interface PnlChartProps {
  runs: SimRun[];
}

export function PnlChart({ runs }: PnlChartProps) {
  const completedRuns = runs.filter((r) => r.status === "done" && r.result?.cumulative_pnl_gwei);

  const maxLen = completedRuns.reduce(
    (m, r) => Math.max(m, r.result?.cumulative_pnl_gwei.length ?? 0),
    0,
  );

  const xData = Array.from({ length: maxLen }, (_, i) => i);

  const series = completedRuns.map((run) => ({
    name: run.label,
    type: "line" as const,
    data: run.result?.cumulative_pnl_gwei ?? [],
    smooth: false,
    symbol: "none",
    lineStyle: { color: RUN_COLORS[run.colorIndex % RUN_COLORS.length], width: 2 },
    itemStyle: { color: RUN_COLORS[run.colorIndex % RUN_COLORS.length] },
  }));

  const option = {
    backgroundColor: "transparent",
    legend: {
      top: 4,
      right: 16,
      textStyle: { fontSize: 11, color: "var(--color-text-muted)" },
    },
    grid: { top: 36, right: 16, bottom: 36, left: 64 },
    xAxis: {
      type: "category" as const,
      data: xData,
      axisLabel: { fontSize: 10, color: "var(--color-text-muted)" },
      axisLine: { lineStyle: { color: "var(--color-border)" } },
      name: "Block Index",
      nameTextStyle: { fontSize: 10, color: "var(--color-text-muted)" },
      nameLocation: "end" as const,
    },
    yAxis: {
      type: "value" as const,
      name: "Cum. P&L (gwei)",
      nameTextStyle: { fontSize: 10, color: "var(--color-text-muted)" },
      axisLabel: { fontSize: 10, color: "var(--color-text-muted)" },
      splitLine: { lineStyle: { color: "var(--color-border)", type: "dashed" as const } },
    },
    series:
      series.length > 0
        ? [
            ...series,
            {
              type: "line" as const,
              markLine: {
                silent: true,
                data: [{ yAxis: 0 }],
                lineStyle: {
                  color: "var(--color-text-muted)",
                  type: "dashed" as const,
                  opacity: 0.5,
                },
                label: { show: false },
              },
              data: [],
            },
          ]
        : [],
    tooltip: { trigger: "axis" as const },
  };

  const containerRef = useECharts(option, [runs]);

  if (completedRuns.length === 0) {
    return (
      <Panel title="CUMULATIVE P&L">
        <div
          style={{
            height: 280,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            color: "var(--color-text-muted)",
            fontSize: 12,
          }}
        >
          Run a backtest to see P&L curve
        </div>
      </Panel>
    );
  }

  return (
    <Panel title="CUMULATIVE P&L">
      <div ref={containerRef} style={{ height: 280 }} />
    </Panel>
  );
}
