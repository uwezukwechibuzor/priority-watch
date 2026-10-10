import type { EraComparisonRow } from "@/api/types";
import { Panel } from "@/components/ui";
import { formatEth, formatPct } from "@/lib/formatters";

interface CompareTabProps {
  rows: EraComparisonRow[];
  loading: boolean;
}

interface MetricRowDef {
  label: string;
  getValue: (row: EraComparisonRow) => number;
  format: (v: number) => string;
  higherIsBetter?: boolean;
}

const METRICS: MetricRowDef[] = [
  {
    label: "Avg Daily Revenue (ETH)",
    getValue: (r) => r.avg_daily_revenue_eth,
    format: (v) => formatEth(v),
    higherIsBetter: true,
  },
  {
    label: "Contested %",
    getValue: (r) => r.avg_contested_share,
    format: (v) => formatPct(v),
    higherIsBetter: true,
  },
  {
    label: "Avg HHI",
    getValue: (r) => r.avg_hhi,
    format: (v) => Math.round(v).toLocaleString(),
    higherIsBetter: false,
  },
  {
    label: "Top Payer Share",
    getValue: (r) => r.avg_top_payer_share,
    format: (v) => formatPct(v),
    higherIsBetter: false,
  },
  {
    label: "Revert Rate",
    getValue: (r) => r.avg_revert_rate,
    format: (v) => formatPct(v),
    higherIsBetter: false,
  },
  {
    label: "Capture Rate",
    getValue: (r) => r.capture_rate_pct / 100,
    format: (v) => formatPct(v),
    higherIsBetter: true,
  },
];

function deltaColor(delta: number, higherIsBetter: boolean): string {
  if (Math.abs(delta) < 0.0001) return "var(--color-text-muted)";
  const positive = delta > 0;
  const good = higherIsBetter ? positive : !positive;
  return good ? "var(--color-green)" : "var(--color-red)";
}

export function CompareTab({ rows, loading }: CompareTabProps) {
  const timeboost = rows.find((r) => r.era === "timeboost");
  const pga = rows.find((r) => r.era === "pga");

  if (loading) {
    return (
      <Panel title="ERA COMPARISON">
        <div
          style={{
            padding: 32,
            textAlign: "center",
            color: "var(--color-text-muted)",
            fontSize: 12,
          }}
        >
          Loading comparison data…
        </div>
      </Panel>
    );
  }

  if (!timeboost && !pga) {
    return (
      <Panel title="ERA COMPARISON">
        <div
          style={{
            padding: 32,
            textAlign: "center",
            color: "var(--color-text-muted)",
            fontSize: 12,
          }}
        >
          No comparison data available for this window.
        </div>
      </Panel>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      {/* Era stat cards */}
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
        <Panel title="TIMEBOOST ERA">
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <div style={{ fontSize: 11, color: "var(--color-text-muted)" }}>
              Apr 2025 – Sep 2026
            </div>
            <div
              className="tabular"
              style={{ fontSize: 20, fontWeight: 600, color: "var(--color-text-primary)" }}
            >
              {timeboost ? formatEth(timeboost.window_tip_eth) : "—"}
            </div>
            <div style={{ fontSize: 11, color: "var(--color-text-muted)" }}>Window tip revenue</div>
          </div>
        </Panel>
        <Panel title="PGA ERA">
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <div style={{ fontSize: 11, color: "var(--color-text-muted)" }}>Sep 2026 – present</div>
            <div
              className="tabular"
              style={{ fontSize: 20, fontWeight: 600, color: "var(--color-accent)" }}
            >
              {pga ? formatEth(pga.window_tip_eth) : "—"}
            </div>
            <div style={{ fontSize: 11, color: "var(--color-text-muted)" }}>Window tip revenue</div>
          </div>
        </Panel>
      </div>

      {/* Comparison table */}
      <Panel title="METRIC COMPARISON">
        <table
          style={{
            width: "100%",
            borderCollapse: "collapse",
            fontSize: 12,
          }}
        >
          <thead>
            <tr>
              {(["METRIC", "TIMEBOOST ERA", "PGA ERA", "DELTA"] as const).map((h) => (
                <th
                  key={h}
                  style={{
                    padding: "6px 8px",
                    textAlign: h === "METRIC" ? "left" : "right",
                    borderBottom: "1px solid var(--color-border)",
                    fontWeight: 500,
                    textTransform: "uppercase",
                    letterSpacing: "0.06em",
                    fontSize: 10,
                    color: "var(--color-text-muted)",
                  }}
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {METRICS.map((m) => {
              const tbVal = timeboost ? m.getValue(timeboost) : null;
              const pgaVal = pga ? m.getValue(pga) : null;
              const delta = tbVal != null && pgaVal != null ? pgaVal - tbVal : null;

              return (
                <tr key={m.label} style={{ borderBottom: "1px solid var(--color-border)" }}>
                  <td style={{ padding: "8px 8px", color: "var(--color-text-primary)" }}>
                    {m.label}
                  </td>
                  <td
                    style={{
                      padding: "8px 8px",
                      textAlign: "right",
                      color: "var(--color-text-muted)",
                    }}
                    className="tabular"
                  >
                    {tbVal != null ? m.format(tbVal) : "—"}
                  </td>
                  <td
                    style={{
                      padding: "8px 8px",
                      textAlign: "right",
                      color: "var(--color-text-primary)",
                    }}
                    className="tabular"
                  >
                    {pgaVal != null ? m.format(pgaVal) : "—"}
                  </td>
                  <td
                    style={{
                      padding: "8px 8px",
                      textAlign: "right",
                      color:
                        delta != null
                          ? deltaColor(delta, m.higherIsBetter ?? true)
                          : "var(--color-text-muted)",
                    }}
                    className="tabular"
                  >
                    {delta != null
                      ? `${delta > 0 ? "+" : ""}${m.format(Math.abs(delta))}${delta > 0 ? " ↑" : " ↓"}`
                      : "—"}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </Panel>

      {/* Entropy benchmark note */}
      <Panel title="ENTROPY BENCHMARKS">
        <p
          style={{
            fontSize: 12,
            color: "var(--color-text-muted)",
            margin: 0,
            lineHeight: 1.6,
          }}
        >
          Entropy benchmark: ETH 28.83% · Base 7.34% · Timeboost (old) 1.74%. Source:{" "}
          <a
            href="https://forum.arbitrum.foundation/t/constitutional-aip-transition-arbitrum-one-ordering-policy-to-priority-gas-auctions-pga/30942"
            target="_blank"
            rel="noopener noreferrer"
            style={{ color: "var(--color-accent)" }}
          >
            AIP thread
          </a>
          .
        </p>
      </Panel>
    </div>
  );
}
