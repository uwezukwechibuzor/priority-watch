import type { ExecQualityRow } from "@/api/types";
import { EntityChip } from "@/components/ui";
import { Panel } from "@/components/ui";
import { formatDate, formatPct } from "@/lib/formatters";

interface ExecutionQualityTableProps {
  data: ExecQualityRow[];
  loading?: boolean;
}

function fmtMarkout(v: number | null | undefined): string {
  if (v == null) return "—";
  return `$${v.toFixed(4)}`;
}

export function ExecutionQualityTable({ data, loading }: ExecutionQualityTableProps) {
  const COLS = [
    { key: "entity_name", label: "ENTITY", width: 160 },
    { key: "revert_rate", label: "REVERT RATE", width: 110 },
    { key: "avg_markout_usd_1s", label: "MARKOUT 1s", width: 110 },
    { key: "avg_markout_usd_300s", label: "MARKOUT 300s", width: 120 },
    { key: "date", label: "DATE", width: 110 },
  ];

  return (
    <Panel title="EXECUTION QUALITY">
      <div style={{ overflowX: "auto" }}>
        <div style={{ display: "flex", borderBottom: "1px solid var(--color-border)" }}>
          {COLS.map((col) => (
            <div
              key={col.key}
              style={{
                width: col.width,
                minWidth: col.width,
                padding: "6px 8px",
                fontSize: 10,
                fontWeight: 500,
                textTransform: "uppercase",
                letterSpacing: "0.06em",
                color: "var(--color-text-muted)",
                flexShrink: 0,
              }}
            >
              {col.label}
            </div>
          ))}
        </div>

        {loading ? (
          <div style={{ padding: 16 }}>
            {Array.from({ length: 5 }, (_, i) => (
              <div
                key={i}
                className="skeleton"
                style={{ height: 28, marginBottom: 4, borderRadius: 3 }}
              />
            ))}
          </div>
        ) : data.length === 0 ? (
          <div
            style={{
              padding: "32px 0",
              textAlign: "center",
              color: "var(--color-text-muted)",
              fontSize: 12,
            }}
          >
            No data for this range
          </div>
        ) : (
          data.slice(0, 50).map((row, idx) => (
            <div
              key={idx}
              style={{
                display: "flex",
                alignItems: "center",
                borderBottom: "1px solid var(--color-border)",
                height: 36,
              }}
            >
              <div
                style={{
                  width: 160,
                  minWidth: 160,
                  padding: "0 8px",
                  overflow: "hidden",
                  flexShrink: 0,
                }}
              >
                <EntityChip name={row.entity_name} />
              </div>
              <div
                className="tabular"
                style={{
                  width: 110,
                  minWidth: 110,
                  padding: "0 8px",
                  fontSize: 12,
                  color: row.revert_rate > 0.1 ? "var(--color-red)" : "var(--color-text-primary)",
                  flexShrink: 0,
                }}
              >
                {formatPct(row.revert_rate)}
              </div>
              <div
                className="tabular"
                style={{ width: 110, minWidth: 110, padding: "0 8px", fontSize: 12, flexShrink: 0 }}
              >
                {fmtMarkout(row.avg_markout_usd_1s)}
              </div>
              <div
                className="tabular"
                style={{ width: 120, minWidth: 120, padding: "0 8px", fontSize: 12, flexShrink: 0 }}
              >
                {fmtMarkout(row.avg_markout_usd_300s)}
              </div>
              <div
                style={{
                  width: 110,
                  minWidth: 110,
                  padding: "0 8px",
                  fontSize: 12,
                  color: "var(--color-text-muted)",
                  flexShrink: 0,
                }}
              >
                {formatDate(row.date)}
              </div>
            </div>
          ))
        )}
      </div>
    </Panel>
  );
}
