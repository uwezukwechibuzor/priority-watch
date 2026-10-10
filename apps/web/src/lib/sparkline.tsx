import { useId } from "react";

interface SparklineProps {
  data: number[];
  color?: string;
  height?: number;
}

export function Sparkline({ data, color = "var(--color-accent)", height = 32 }: SparklineProps) {
  const id = useId();
  if (data.length < 2) return <div style={{ height }} />;

  const min = Math.min(...data);
  const max = Math.max(...data);
  const range = max - min || 1;
  const W = 120;
  const H = height;
  const PAD = 2;

  const pts = data.map((v, i) => {
    const x = (i / (data.length - 1)) * (W - PAD * 2) + PAD;
    const y = H - PAD - ((v - min) / range) * (H - PAD * 2);
    return [x, y] as [number, number];
  });

  const firstPt = pts[0];
  const lastPt = pts[pts.length - 1];
  const linePts = pts.map(([x, y]) => `${x.toFixed(1)},${y.toFixed(1)}`).join(" ");

  // firstPt is always defined since data.length >= 2
  const areaD = `M ${firstPt[0].toFixed(1)},${firstPt[1].toFixed(1)} L ${linePts.slice(linePts.indexOf(" ") + 1)} L ${lastPt[0].toFixed(1)},${(H - PAD).toFixed(1)} L ${firstPt[0].toFixed(1)},${(H - PAD).toFixed(1)} Z`;
  const lineD = `M ${linePts}`;

  return (
    <svg
      aria-hidden="true"
      viewBox={`0 0 ${W} ${H}`}
      style={{ width: "100%", height, display: "block" }}
      preserveAspectRatio="none"
    >
      <defs>
        <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity="0.25" />
          <stop offset="100%" stopColor={color} stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={areaD} fill={`url(#${id})`} />
      <path
        d={lineD}
        stroke={color}
        strokeWidth="1.5"
        fill="none"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
