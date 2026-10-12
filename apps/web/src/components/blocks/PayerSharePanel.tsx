import type { PayerShareRow } from "@/api/types";
import { Panel, Skeleton } from "@/components/ui";
import { entityColor, formatPct } from "@/lib/formatters";

interface PayerSharePanelProps {
  data: PayerShareRow[] | null;
  loading?: boolean;
  onPayerClick?: (name: string) => void;
}

export function PayerSharePanel({ data, loading = false, onPayerClick }: PayerSharePanelProps) {
  const rows = data?.slice(0, 5) ?? [];

  return (
    <Panel title="TOP PAYERS">
      {loading ? (
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          {Array.from({ length: 5 }, (_, i) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: skeleton placeholders have no stable id
            <Skeleton key={i} height={28} />
          ))}
        </div>
      ) : rows.length === 0 ? (
        <p style={{ color: "var(--color-text-muted)", fontSize: 12 }}>No data</p>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {rows.map((row) => (
            <button
              key={row.entity_name}
              type="button"
              onClick={() => onPayerClick?.(row.entity_name)}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 6,
                cursor: "pointer",
                background: "none",
                border: "none",
                padding: 0,
                width: "100%",
                textAlign: "left",
              }}
            >
              <span
                aria-hidden="true"
                style={{
                  width: 8,
                  height: 8,
                  borderRadius: 1,
                  background: entityColor(row.color_index),
                  flexShrink: 0,
                }}
              />
              <span
                style={{
                  fontSize: 12,
                  color: "var(--color-text-primary)",
                  minWidth: 0,
                  flex: "0 0 90px",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {row.entity_name}
              </span>
              <span
                style={{
                  flex: 1,
                  position: "relative",
                  height: 6,
                  background: "var(--color-border)",
                  borderRadius: 3,
                  overflow: "hidden",
                }}
                aria-label={`${row.entity_name} share: ${formatPct(row.share_pct)}`}
              >
                <span
                  style={{
                    position: "absolute",
                    left: 0,
                    top: 0,
                    bottom: 0,
                    width: `${row.share_pct * 100}%`,
                    background: entityColor(row.color_index),
                    borderRadius: 3,
                  }}
                />
              </span>
              <span
                className="tabular"
                style={{
                  fontSize: 11,
                  color: "var(--color-text-muted)",
                  width: 40,
                  textAlign: "right",
                  flexShrink: 0,
                }}
              >
                {(row.share_pct * 100).toFixed(1)}%
              </span>
            </button>
          ))}
        </div>
      )}
    </Panel>
  );
}
