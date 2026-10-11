import { formatPct } from "@/lib/formatters";
import type { SimRun } from "./PnlChart";

const RUN_COLORS = ["#3b82f6", "#f59e0b", "#22c55e", "#ef4444"];

interface RunSummaryCardsProps {
  runs: SimRun[];
  onCancel: (id: string) => void;
}

export function RunSummaryCards({ runs, onCancel }: RunSummaryCardsProps) {
  if (runs.length === 0) return null;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      {runs.map((run) => {
        const color = RUN_COLORS[run.colorIndex % RUN_COLORS.length];
        return (
          <div
            key={run.id}
            style={{
              background: "var(--color-bg-panel)",
              border: `1px solid ${color}`,
              borderRadius: "var(--radius-panel)",
              padding: 12,
              position: "relative",
            }}
          >
            <button
              type="button"
              onClick={() => onCancel(run.id)}
              aria-label="Cancel run"
              style={{
                position: "absolute",
                top: 8,
                right: 8,
                background: "none",
                border: "none",
                color: "var(--color-text-muted)",
                cursor: "pointer",
                fontSize: 16,
                lineHeight: 1,
                padding: 2,
              }}
            >
              ×
            </button>

            <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
              <span
                style={{
                  width: 10,
                  height: 10,
                  borderRadius: "50%",
                  background: color,
                  flexShrink: 0,
                }}
              />
              <span
                style={{
                  fontSize: 11,
                  fontWeight: 600,
                  color: "var(--color-text-primary)",
                  fontFamily: "var(--font-mono)",
                }}
              >
                {run.label}
              </span>
              <StatusBadge status={run.status} />
            </div>

            {run.status === "done" && run.result && (
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "1fr 1fr 1fr 1fr",
                  gap: 8,
                }}
              >
                <Stat label="Win Rate" value={formatPct(run.result.win_rate)} />
                <Stat label="Wins / Total" value={`${run.result.wins} / ${run.result.total}`} />
                <Stat label="Avg Cost" value={`${run.result.avg_cost_gwei.toFixed(2)} gwei`} />
                <Stat
                  label="Total P&L"
                  value={`${run.result.total_pnl_gwei >= 0 ? "+" : ""}${run.result.total_pnl_gwei.toFixed(2)} gwei`}
                  color={run.result.total_pnl_gwei >= 0 ? "var(--color-green)" : "var(--color-red)"}
                />
              </div>
            )}

            {run.status === "error" && run.errorMsg && (
              <p style={{ fontSize: 11, color: "var(--color-red)", margin: 0 }}>
                Error: {run.errorMsg}
              </p>
            )}

            {(run.status === "pending" || run.status === "running") && (
              <p style={{ fontSize: 11, color: "var(--color-text-muted)", margin: 0 }}>
                {run.status === "pending" ? "Queued…" : "Running…"}
              </p>
            )}
          </div>
        );
      })}
    </div>
  );
}

function Stat({
  label,
  value,
  color,
}: {
  label: string;
  value: string;
  color?: string;
}) {
  return (
    <div>
      <div
        style={{
          fontSize: 10,
          textTransform: "uppercase",
          letterSpacing: "0.06em",
          color: "var(--color-text-muted)",
          marginBottom: 2,
        }}
      >
        {label}
      </div>
      <div
        className="tabular"
        style={{
          fontSize: 13,
          fontWeight: 600,
          color: color ?? "var(--color-text-primary)",
        }}
      >
        {value}
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: SimRun["status"] }) {
  const styles: Record<SimRun["status"], { bg: string; color: string; label: string }> = {
    pending: { bg: "rgba(107,114,128,0.15)", color: "#6b7280", label: "Pending" },
    running: { bg: "rgba(245,158,11,0.15)", color: "#f59e0b", label: "Running" },
    done: { bg: "rgba(34,197,94,0.15)", color: "#22c55e", label: "Done" },
    error: { bg: "rgba(239,68,68,0.15)", color: "#ef4444", label: "Error" },
  };
  const s = styles[status];
  return (
    <span
      style={{
        fontSize: 10,
        padding: "1px 6px",
        background: s.bg,
        border: `1px solid ${s.color}`,
        borderRadius: 3,
        color: s.color,
        textTransform: "uppercase",
        letterSpacing: "0.06em",
      }}
    >
      {s.label}
    </span>
  );
}
