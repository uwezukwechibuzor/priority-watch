import type { IngesterStatusRow } from "@/api/types";
import { Panel } from "@/components/ui";
import { relativeTime } from "@/lib/formatters";

interface PipelineTabProps {
  sources: IngesterStatusRow[];
}

const KNOWN_DAGS = [
  { dag: "pw_backfill", schedule: "Manual" },
  { dag: "pw_dbt_incremental", schedule: "*/10 * * * *" },
  { dag: "pw_dbt_nightly", schedule: "0 2 * * *" },
  { dag: "pw_cex_daily", schedule: "0 1 * * *" },
  { dag: "pw_monitoring", schedule: "*/5 * * * *" },
  { dag: "pw_timeboost_baseline", schedule: "Manual" },
  { dag: "pw_clickhouse_backup", schedule: "0 3 * * *" },
];

export function PipelineTab({ sources }: PipelineTabProps) {
  const chainSource = sources.find((s) => s.source === "chain");

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      {/* Ingester metrics */}
      <Panel title="INGESTER METRICS">
        {sources.length === 0 ? (
          <div style={{ color: "var(--color-text-muted)", fontSize: 12 }}>
            No ingester data available.
          </div>
        ) : (
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fill, minmax(200px, 1fr))",
              gap: 12,
            }}
          >
            {chainSource && (
              <>
                <div
                  style={{
                    background: "var(--color-bg-base)",
                    border: "1px solid var(--color-border)",
                    borderRadius: "var(--radius-panel)",
                    padding: "12px 14px",
                  }}
                >
                  <div className="label" style={{ marginBottom: 6 }}>
                    LATEST BLOCK
                  </div>
                  <div className="tabular" style={{ fontSize: 16, fontWeight: 600 }}>
                    {chainSource.latest_position}
                  </div>
                </div>
                <div
                  style={{
                    background: "var(--color-bg-base)",
                    border: "1px solid var(--color-border)",
                    borderRadius: "var(--radius-panel)",
                    padding: "12px 14px",
                  }}
                >
                  <div className="label" style={{ marginBottom: 6 }}>
                    CHAIN LAG
                  </div>
                  <div
                    className="tabular"
                    style={{
                      fontSize: 16,
                      fontWeight: 600,
                      color:
                        chainSource.lag_seconds > 2 ? "var(--color-red)" : "var(--color-green)",
                    }}
                  >
                    {chainSource.lag_seconds.toFixed(1)}s
                  </div>
                </div>
                <div
                  style={{
                    background: "var(--color-bg-base)",
                    border: "1px solid var(--color-border)",
                    borderRadius: "var(--radius-panel)",
                    padding: "12px 14px",
                  }}
                >
                  <div className="label" style={{ marginBottom: 6 }}>
                    LAST SEEN
                  </div>
                  <div className="tabular" style={{ fontSize: 16, fontWeight: 600 }}>
                    {chainSource.latest_timestamp
                      ? relativeTime(chainSource.latest_timestamp)
                      : "—"}
                  </div>
                </div>
              </>
            )}
          </div>
        )}
      </Panel>

      {/* DAG list */}
      <Panel title="AIRFLOW DAG STATUS">
        <div style={{ marginBottom: 8, fontSize: 11, color: "var(--color-text-muted)" }}>
          Airflow DAG status — connect Airflow REST API to populate
        </div>
        <table
          style={{
            width: "100%",
            borderCollapse: "collapse",
            fontSize: 12,
          }}
        >
          <thead>
            <tr>
              {(["DAG", "SCHEDULE", "STATUS"] as const).map((h) => (
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
            {KNOWN_DAGS.map((d) => (
              <tr key={d.dag} style={{ borderBottom: "1px solid var(--color-border)" }}>
                <td style={{ padding: "8px 8px", color: "var(--color-text-primary)" }}>{d.dag}</td>
                <td
                  style={{ padding: "8px 8px", color: "var(--color-text-muted)" }}
                  className="tabular"
                >
                  {d.schedule}
                </td>
                <td style={{ padding: "8px 8px", color: "var(--color-text-muted)" }}>—</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Panel>
    </div>
  );
}
