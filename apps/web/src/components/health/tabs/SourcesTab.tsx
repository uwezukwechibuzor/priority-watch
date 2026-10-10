import type { IngesterStatusRow } from "@/api/types";
import { Panel } from "@/components/ui";
import { relativeTime } from "@/lib/formatters";

interface SourcesTabProps {
  sources: IngesterStatusRow[];
  loading: boolean;
}

function StatusDot({ status }: { status: IngesterStatusRow["status"] }) {
  const colors: Record<IngesterStatusRow["status"], string> = {
    live: "#22c55e",
    lagging: "#f59e0b",
    down: "#ef4444",
  };
  const labels: Record<IngesterStatusRow["status"], string> = {
    live: "live",
    lagging: "lagging",
    down: "down",
  };

  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: 5 }}>
      <span
        style={{
          width: 8,
          height: 8,
          borderRadius: "50%",
          background: colors[status],
          display: "inline-block",
          flexShrink: 0,
        }}
      />
      <span style={{ color: colors[status], fontSize: 12, fontWeight: 500 }}>{labels[status]}</span>
    </span>
  );
}

export function SourcesTab({ sources, loading }: SourcesTabProps) {
  if (loading && sources.length === 0) {
    return (
      <Panel title="INGESTER SOURCES">
        <div
          style={{
            padding: 32,
            textAlign: "center",
            color: "var(--color-text-muted)",
            fontSize: 12,
          }}
        >
          Loading sources…
        </div>
      </Panel>
    );
  }

  return (
    <Panel title="INGESTER SOURCES">
      <table
        style={{
          width: "100%",
          borderCollapse: "collapse",
          fontSize: 12,
        }}
      >
        <thead>
          <tr>
            {(["SOURCE", "STATUS", "LAG", "LATEST POSITION", "LAST SEEN"] as const).map((h) => (
              <th
                key={h}
                style={{
                  padding: "6px 8px",
                  textAlign: "left",
                  borderBottom: "1px solid var(--color-border)",
                  fontWeight: 500,
                  textTransform: "uppercase",
                  letterSpacing: "0.06em",
                  fontSize: 10,
                  color: "var(--color-text-muted)",
                  whiteSpace: "nowrap",
                }}
              >
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {sources.length === 0 ? (
            <tr>
              <td
                colSpan={5}
                style={{
                  padding: "32px 8px",
                  textAlign: "center",
                  color: "var(--color-text-muted)",
                }}
              >
                No source data
              </td>
            </tr>
          ) : (
            sources.map((s) => (
              <tr key={s.source} style={{ borderBottom: "1px solid var(--color-border)" }}>
                <td style={{ padding: "8px 8px", color: "var(--color-text-primary)" }}>
                  {s.source}
                </td>
                <td style={{ padding: "8px 8px" }}>
                  <StatusDot status={s.status} />
                </td>
                <td
                  style={{
                    padding: "8px 8px",
                    color:
                      s.lag_seconds > 30
                        ? "var(--color-red)"
                        : s.lag_seconds > 5
                          ? "var(--color-amber)"
                          : "var(--color-text-muted)",
                  }}
                  className="tabular"
                >
                  {s.lag_seconds.toFixed(1)}s
                </td>
                <td
                  style={{ padding: "8px 8px", color: "var(--color-text-primary)" }}
                  className="tabular"
                >
                  {s.latest_position}
                </td>
                <td
                  style={{ padding: "8px 8px", color: "var(--color-text-muted)" }}
                  className="tabular"
                >
                  {s.latest_timestamp ? relativeTime(s.latest_timestamp) : "—"}
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </Panel>
  );
}
