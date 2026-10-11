import type { PositionRow } from "@/api/types";
import { EntityChip } from "@/components/ui";
import { Panel } from "@/components/ui";
import { formatGwei, formatPct } from "@/lib/formatters";

interface PositionLeaderboardProps {
  data: PositionRow[];
  loading?: boolean;
}

export function PositionLeaderboard({ data, loading }: PositionLeaderboardProps) {
  // Aggregate by entity (take latest / average)
  const byEntity = new Map<string, PositionRow>();
  for (const row of data) {
    const existing = byEntity.get(row.entity_name);
    if (!existing || row.date > existing.date) {
      byEntity.set(row.entity_name, row);
    }
  }
  const rows = [...byEntity.values()].sort(
    (a, b) => b.delta_rank_vs_position - a.delta_rank_vs_position,
  );

  const COLS = [
    { key: "entity", label: "ENTITY", width: 160 },
    { key: "avg_fee_gwei", label: "AVG FEE", width: 100 },
    { key: "avg_tip_rank", label: "AVG RANK", width: 90 },
    { key: "top3_position_share", label: "TOP-3 SHARE", width: 100 },
    { key: "fee_efficiency", label: "FEE EFF.", width: 90 },
    { key: "delta_rank_vs_position", label: "DELTA", width: 80 },
  ];

  return (
    <Panel title="ENTITY POSITION LEADERBOARD">
      <div style={{ overflowX: "auto" }}>
        <div
          style={{
            display: "flex",
            borderBottom: "1px solid var(--color-border)",
            background: "var(--color-bg-panel)",
          }}
        >
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
        ) : rows.length === 0 ? (
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
          rows.map((row, idx) => {
            const deltaColor =
              row.delta_rank_vs_position > 0
                ? "var(--color-red)"
                : row.delta_rank_vs_position < 0
                  ? "var(--color-green)"
                  : "var(--color-text-muted)";
            return (
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
                    width: 100,
                    minWidth: 100,
                    padding: "0 8px",
                    fontSize: 12,
                    flexShrink: 0,
                  }}
                >
                  {formatGwei(row.avg_fee_gwei)}
                </div>
                <div
                  className="tabular"
                  style={{ width: 90, minWidth: 90, padding: "0 8px", fontSize: 12, flexShrink: 0 }}
                >
                  {row.avg_tip_rank.toFixed(1)}
                </div>
                <div
                  className="tabular"
                  style={{
                    width: 100,
                    minWidth: 100,
                    padding: "0 8px",
                    fontSize: 12,
                    flexShrink: 0,
                  }}
                >
                  {formatPct(row.top3_position_share)}
                </div>
                <div
                  className="tabular"
                  style={{ width: 90, minWidth: 90, padding: "0 8px", fontSize: 12, flexShrink: 0 }}
                >
                  {row.fee_efficiency.toFixed(3)}
                </div>
                <div
                  className="tabular"
                  style={{
                    width: 80,
                    minWidth: 80,
                    padding: "0 8px",
                    fontSize: 12,
                    color: deltaColor,
                    fontWeight: 600,
                    flexShrink: 0,
                  }}
                >
                  {row.delta_rank_vs_position > 0 ? "+" : ""}
                  {row.delta_rank_vs_position.toFixed(1)}
                </div>
              </div>
            );
          })
        )}
      </div>
    </Panel>
  );
}
