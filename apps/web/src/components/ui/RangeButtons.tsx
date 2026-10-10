import type { Range } from "@/lib/search-params";
import { RANGES } from "@/lib/search-params";

interface RangeButtonsProps {
  value: Range;
  onChange: (r: Range) => void;
}

export function RangeButtons({ value, onChange }: RangeButtonsProps) {
  return (
    <fieldset
      style={{ display: "flex", gap: 2, border: "none", padding: 0, margin: 0 }}
      aria-label="Time range"
    >
      {RANGES.map((r) => (
        <button
          key={r}
          type="button"
          onClick={() => onChange(r)}
          aria-pressed={value === r}
          style={{
            padding: "3px 8px",
            fontSize: 11,
            fontWeight: 500,
            background: value === r ? "var(--color-accent)" : "transparent",
            color: value === r ? "#fff" : "var(--color-text-muted)",
            border: "1px solid",
            borderColor: value === r ? "var(--color-accent)" : "var(--color-border)",
            borderRadius: "var(--radius-badge)",
            cursor: "pointer",
            transition: "all 0.1s",
            textTransform: "uppercase",
            letterSpacing: "0.04em",
          }}
        >
          {r}
        </button>
      ))}
    </fieldset>
  );
}
