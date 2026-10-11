import type { Recommendation } from "@/api/types";
import { formatGwei, formatPct } from "@/lib/formatters";

interface ModelCardsProps {
  recommendations: Recommendation[];
  selectedModel: string;
}

export function ModelCards({ recommendations, selectedModel }: ModelCardsProps) {
  if (recommendations.length === 0) {
    return null;
  }

  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: "1fr 1fr",
        gap: 8,
      }}
    >
      {recommendations.map((rec) => {
        const isSelected = selectedModel !== "all" && rec.model_name === selectedModel;
        return (
          <div
            key={rec.model_name}
            style={{
              background: "var(--color-bg-panel)",
              border: `1px solid ${isSelected ? "var(--color-accent)" : "var(--color-border)"}`,
              borderRadius: "var(--radius-panel)",
              padding: 12,
              display: "flex",
              flexDirection: "column",
              gap: 6,
            }}
          >
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
              <span
                style={{
                  fontSize: 10,
                  textTransform: "uppercase",
                  letterSpacing: "0.06em",
                  color: "var(--color-text-muted)",
                }}
              >
                {rec.model_name}
              </span>
              <span
                style={{
                  fontSize: 10,
                  padding: "1px 6px",
                  background: "rgba(34,197,94,0.15)",
                  border: "1px solid var(--color-green)",
                  borderRadius: 3,
                  color: "var(--color-green)",
                }}
              >
                {formatPct(rec.hit_rate)} hit
              </span>
            </div>

            <div
              className="tabular"
              style={{ fontSize: 22, fontWeight: 700, color: "var(--color-text-primary)" }}
            >
              {formatGwei(rec.fee_gwei)}
            </div>

            <div style={{ fontSize: 11, color: "var(--color-text-muted)" }}>
              avg cost: {formatGwei(rec.avg_cost_gwei)}
            </div>

            {rec.reasoning.length > 0 && (
              <div
                style={{
                  borderTop: "1px solid var(--color-border)",
                  paddingTop: 6,
                  display: "flex",
                  flexDirection: "column",
                  gap: 2,
                }}
              >
                {rec.reasoning.slice(0, 4).map((step, i) => (
                  <div
                    key={i}
                    style={{
                      display: "flex",
                      justifyContent: "space-between",
                      fontSize: 11,
                    }}
                  >
                    <span style={{ color: "var(--color-text-muted)" }}>{step.label}</span>
                    <span className="tabular">
                      {step.value.toFixed(3)}
                      {step.unit ? ` ${step.unit}` : ""}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
