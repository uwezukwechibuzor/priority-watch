import * as echarts from "echarts/core";

const PRIORITY_WATCH_THEME = {
  color: ["#3b82f6", "#f59e0b", "#a855f7", "#22c55e", "#ef4444", "#ec4899", "#14b8a6", "#f97316"],
  backgroundColor: "transparent",
  textStyle: {
    color: "#6b7280",
    fontFamily: "ui-sans-serif, system-ui, -apple-system, sans-serif",
    fontSize: 11,
  },
  title: {
    textStyle: { color: "#f0f2f5", fontSize: 12, fontWeight: 500 },
  },
  grid: {
    left: 48,
    right: 16,
    top: 32,
    bottom: 32,
    containLabel: false,
  },
  axisLine: {
    lineStyle: { color: "#1f2329" },
  },
  axisTick: {
    lineStyle: { color: "#1f2329" },
  },
  axisLabel: {
    color: "#6b7280",
    fontSize: 10,
  },
  splitLine: {
    lineStyle: { color: "#1f2329", type: "solid" as const },
  },
  tooltip: {
    backgroundColor: "#111318",
    borderColor: "#1f2329",
    borderWidth: 1,
    textStyle: { color: "#f0f2f5", fontSize: 12 },
    extraCssText: "box-shadow: 0 4px 12px rgba(0,0,0,0.5);",
  },
  legend: {
    textStyle: { color: "#6b7280", fontSize: 11 },
    itemWidth: 12,
    itemHeight: 12,
  },
  line: {
    itemStyle: { borderWidth: 0 },
    lineStyle: { width: 1.5 },
    symbol: "none",
    smooth: false,
  },
  bar: {
    barMaxWidth: 40,
    itemStyle: { borderRadius: [2, 2, 0, 0] },
  },
  scatter: {
    itemStyle: { opacity: 0.7 },
    symbolSize: 5,
  },
  markLine: {
    lineStyle: { type: "dashed" as const, width: 1 },
    label: { fontSize: 10, color: "#6b7280" },
  },
};

echarts.registerTheme("priority-watch", PRIORITY_WATCH_THEME);

export { PRIORITY_WATCH_THEME };
