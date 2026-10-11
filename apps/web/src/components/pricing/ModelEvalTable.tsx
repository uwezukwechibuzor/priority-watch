import type { ModelEvalRow } from "@/api/types";
import { formatDate, formatGwei, formatPct } from "@/lib/formatters";
import { useState } from "react";

interface ModelEvalTableProps {
  rows: ModelEvalRow[];
  loading?: boolean;
}

type SortKey = keyof ModelEvalRow;

export function ModelEvalTable({ rows, loading }: ModelEvalTableProps) {
  const [sortKey, setSortKey] = useState<SortKey>("hit_rate");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");

  function handleSort(key: SortKey) {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("desc");
    }
  }

  const sorted = [...rows].sort((a, b) => {
    const av = a[sortKey];
    const bv = b[sortKey];
    if (typeof av === "number" && typeof bv === "number") {
      return sortDir === "asc" ? av - bv : bv - av;
    }
    return sortDir === "asc"
      ? String(av).localeCompare(String(bv))
      : String(bv).localeCompare(String(av));
  });

  const COLS: { key: SortKey; label: string; width: number }[] = [
    { key: "model_name", label: "MODEL", width: 180 },
    { key: "n", label: "N", width: 50 },
    { key: "confidence", label: "CONFIDENCE", width: 100 },
    { key: "window_blocks", label: "WINDOW", width: 80 },
    { key: "hit_rate", label: "HIT RATE", width: 90 },
    { key: "avg_cost_gwei", label: "AVG COST (gwei)", width: 130 },
    { key: "eval_blocks", label: "EVAL BLOCKS", width: 100 },
    { key: "eval_date", label: "DATE", width: 110 },
  ];

  function renderCell(row: ModelEvalRow, key: SortKey): React.ReactNode {
    switch (key) {
      case "hit_rate":
        return (
          <span className="tabular" style={{ color: "var(--color-green)" }}>
            {formatPct(row.hit_rate)}
          </span>
        );
      case "avg_cost_gwei":
        return <span className="tabular">{formatGwei(row.avg_cost_gwei)}</span>;
      case "confidence":
        return <span className="tabular">{formatPct(row.confidence)}</span>;
      case "eval_date":
        return (
          <span style={{ color: "var(--color-text-muted)" }}>{formatDate(row.eval_date)}</span>
        );
      default:
        return <span className="tabular">{String(row[key])}</span>;
    }
  }

  return (
    <div
      style={{
        background: "var(--color-bg-panel)",
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-panel)",
        overflow: "hidden",
      }}
    >
      <div
        style={{
          padding: "10px var(--spacing-panel)",
          borderBottom: "1px solid var(--color-border)",
        }}
      >
        <span className="label" style={{ fontSize: 11 }}>
          MODEL EVALUATION
        </span>
      </div>

      <div style={{ overflowX: "auto" }}>
        <div style={{ display: "flex", borderBottom: "1px solid var(--color-border)" }}>
          {COLS.map((col) => (
            <button
              type="button"
              key={col.key}
              onClick={() => handleSort(col.key)}
              style={{
                width: col.width,
                minWidth: col.width,
                padding: "6px 8px",
                fontSize: 10,
                fontWeight: 500,
                textTransform: "uppercase",
                letterSpacing: "0.06em",
                color:
                  sortKey === col.key ? "var(--color-text-primary)" : "var(--color-text-muted)",
                background: "transparent",
                border: "none",
                cursor: "pointer",
                textAlign: "left",
                flexShrink: 0,
              }}
            >
              {col.label}
              {sortKey === col.key && (sortDir === "asc" ? " ↑" : " ↓")}
            </button>
          ))}
        </div>

        {loading ? (
          <div style={{ padding: 16 }}>
            {Array.from({ length: 4 }, (_, i) => (
              <div
                key={i}
                className="skeleton"
                style={{ height: 28, marginBottom: 4, borderRadius: 3 }}
              />
            ))}
          </div>
        ) : sorted.length === 0 ? (
          <div
            style={{
              padding: "32px 0",
              textAlign: "center",
              color: "var(--color-text-muted)",
              fontSize: 12,
            }}
          >
            No evaluation data
          </div>
        ) : (
          sorted.map((row, idx) => (
            <div
              key={idx}
              style={{
                display: "flex",
                alignItems: "center",
                borderBottom: "1px solid var(--color-border)",
                height: 36,
              }}
            >
              {COLS.map((col) => (
                <div
                  key={col.key}
                  style={{
                    width: col.width,
                    minWidth: col.width,
                    padding: "0 8px",
                    fontSize: 12,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                    flexShrink: 0,
                  }}
                >
                  {renderCell(row, col.key)}
                </div>
              ))}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
