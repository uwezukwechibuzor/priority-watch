import { RATIO_BAND_COLOR, ratioBand } from "@/lib/formatters";

interface RatioBarProps {
  ratio: number; // 0–1
  width?: number;
}

export function RatioBar({ ratio, width = 64 }: RatioBarProps) {
  const clamped = Math.max(0, Math.min(1, ratio));
  const band = ratioBand(clamped);
  const color = RATIO_BAND_COLOR[band];
  const pct = `${(clamped * 100).toFixed(0)}%`;

  return (
    <span
      style={{ display: "inline-flex", alignItems: "center", gap: 5 }}
      title={`2nd/1st ratio: ${pct}`}
    >
      <span
        style={{
          position: "relative",
          width,
          height: 6,
          borderRadius: 3,
          background: "var(--color-border)",
          overflow: "hidden",
        }}
        aria-label={`ratio ${pct}`}
      >
        <span
          style={{
            position: "absolute",
            left: 0,
            top: 0,
            bottom: 0,
            width: `${clamped * 100}%`,
            background: color,
            borderRadius: 3,
          }}
        />
      </span>
      <span className="tabular" style={{ fontSize: 11, color, width: 36, textAlign: "right" }}>
        {pct}
      </span>
    </span>
  );
}
