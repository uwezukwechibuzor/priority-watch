import * as echarts from "echarts";
import { useEffect, useRef } from "react";

export function useECharts(option: echarts.EChartsOption, deps: unknown[] = []) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<echarts.ECharts | null>(null);

  // biome-ignore lint/correctness/useExhaustiveDependencies: init runs once; option is applied in second effect
  useEffect(() => {
    if (!containerRef.current) return;
    chartRef.current = echarts.init(containerRef.current, "priority-watch");
    chartRef.current.setOption(option);

    const ro = new ResizeObserver(() => chartRef.current?.resize());
    ro.observe(containerRef.current);
    return () => {
      ro.disconnect();
      chartRef.current?.dispose();
      chartRef.current = null;
    };
  }, []);

  // biome-ignore lint/correctness/useExhaustiveDependencies: caller passes deps explicitly
  useEffect(() => {
    chartRef.current?.setOption(option, { notMerge: false });
  }, deps);

  return containerRef;
}
