import type { Recommendation } from "@/api/types";
import { Panel } from "@/components/ui";
import { formatEth, formatGwei, formatPct } from "@/lib/formatters";

interface RecommendationPanelProps {
  rec?: Recommendation;
  loading?: boolean;
}

export function RecommendationPanel({ rec, loading }: RecommendationPanelProps) {
  if (loading) {
    return (
      <Panel title="RECOMMENDATION">
        <div className="skeleton" style={{ height: 80, borderRadius: 4 }} />
      </Panel>
    );
  }

  if (!rec) {
    return (
      <Panel title="RECOMMENDATION">
        <p style={{ color: "var(--color-text-muted)", fontSize: 12 }}>
          Set parameters and click Get Recommendation.
        </p>
      </Panel>
    );
  }

  const costEth = rec.fee_gwei * 1e-9 * 1_000_000;

  return (
    <Panel title="RECOMMENDATION">
      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        <div style={{ display: "flex", alignItems: "baseline", gap: 12 }}>
          <span
            className="tabular"
            style={{ fontSize: 36, fontWeight: 700, color: "var(--color-text-primary)" }}
          >
            {formatGwei(rec.fee_gwei)}
          </span>
          <span
            style={{
              fontSize: 12,
              color: "var(--color-text-muted)",
            }}
          >
            [{formatGwei(rec.low_gwei)} – {formatGwei(rec.high_gwei)}]
          </span>
          <span
            style={{
              display: "inline-flex",
              alignItems: "center",
              padding: "2px 8px",
              background: "rgba(168,85,247,0.15)",
              border: "1px solid #a855f7",
              borderRadius: 3,
              fontSize: 10,
              color: "#a855f7",
              textTransform: "uppercase",
              letterSpacing: "0.06em",
            }}
          >
            {rec.model_name}
          </span>
        </div>

        <div style={{ display: "flex", gap: 24 }}>
          <div>
            <span
              style={{
                fontSize: 10,
                textTransform: "uppercase",
                letterSpacing: "0.06em",
                color: "var(--color-text-muted)",
              }}
            >
              Est. Cost @ 1M gas
            </span>
            <div className="tabular" style={{ fontSize: 16, fontWeight: 600, marginTop: 2 }}>
              {formatEth(costEth)}
            </div>
          </div>
          <div>
            <span
              style={{
                fontSize: 10,
                textTransform: "uppercase",
                letterSpacing: "0.06em",
                color: "var(--color-text-muted)",
              }}
            >
              Hit Rate
            </span>
            <div className="tabular" style={{ fontSize: 16, fontWeight: 600, marginTop: 2 }}>
              {formatPct(rec.hit_rate)}
            </div>
          </div>
          <div>
            <span
              style={{
                fontSize: 10,
                textTransform: "uppercase",
                letterSpacing: "0.06em",
                color: "var(--color-text-muted)",
              }}
            >
              Avg Cost
            </span>
            <div className="tabular" style={{ fontSize: 16, fontWeight: 600, marginTop: 2 }}>
              {formatGwei(rec.avg_cost_gwei)}
            </div>
          </div>
        </div>

        {rec.reasoning.length > 0 && (
          <div
            style={{
              borderTop: "1px solid var(--color-border)",
              paddingTop: 10,
            }}
          >
            <span
              style={{
                fontSize: 10,
                textTransform: "uppercase",
                letterSpacing: "0.06em",
                color: "var(--color-text-muted)",
                display: "block",
                marginBottom: 6,
              }}
            >
              Reasoning
            </span>
            <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
              {rec.reasoning.map((step, i) => (
                <div
                  key={i}
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    fontSize: 12,
                  }}
                >
                  <span style={{ color: "var(--color-text-muted)" }}>{step.label}</span>
                  <span className="tabular" style={{ fontWeight: 500 }}>
                    {step.value.toFixed(4)}
                    {step.unit ? ` ${step.unit}` : ""}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </Panel>
  );
}
