import type { SearchResult } from "@/api/types";
import { arbiscanAddress, arbiscanBlock } from "@/lib/formatters";
import { searchQuery } from "@/queries/search";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";

interface CommandPaletteProps {
  open: boolean;
  onClose: () => void;
}

const TYPE_LABEL: Record<string, string> = {
  block: "Blocks",
  tx: "Transactions",
  address: "Addresses",
};

export function CommandPalette({ open, onClose }: CommandPaletteProps) {
  const [q, setQ] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);
  const navigate = useNavigate();

  const { data: results, isFetching } = useQuery(searchQuery(q));

  useEffect(() => {
    if (open) {
      setQ("");
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, onClose]);

  function handleSelect(r: SearchResult) {
    onClose();
    if (r.type === "block") {
      void navigate({ to: "/blocks", search: { block: Number(r.value), range: "24h" } });
    } else if (r.type === "address") {
      window.open(arbiscanAddress(r.value), "_blank");
    } else if (r.type === "tx") {
      window.open(arbiscanBlock(r.sub_text ?? ""), "_blank");
    }
  }

  // Group results by type
  const grouped = groupBy(results ?? [], (r) => r.type);

  return (
    <>
      {/* Backdrop */}
      {/* biome-ignore lint/a11y/useKeyWithClickEvents: backdrop is aria-hidden, keyboard handled by Escape listener */}
      <div
        aria-hidden="true"
        onClick={onClose}
        style={{
          position: "fixed",
          inset: 0,
          background: "rgba(0,0,0,0.6)",
          zIndex: 60,
          display: open ? "block" : "none",
        }}
      />
      {/* Modal */}
      {/* biome-ignore lint/a11y/useSemanticElements: <dialog> requires imperative .showModal() which conflicts with display:none toggle */}
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Command palette"
        style={{
          position: "fixed",
          top: "20%",
          left: "50%",
          transform: "translateX(-50%)",
          width: "min(600px, 90vw)",
          background: "var(--color-bg-panel)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius-panel)",
          boxShadow: "0 8px 40px rgba(0,0,0,0.6)",
          zIndex: 70,
          display: open ? "flex" : "none",
          flexDirection: "column",
          overflow: "hidden",
          maxHeight: "60vh",
        }}
      >
        {/* Search input */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 10,
            padding: "10px 14px",
            borderBottom: "1px solid var(--color-border)",
          }}
        >
          <svg aria-hidden="true" width="16" height="16" viewBox="0 0 16 16" fill="none">
            <circle cx="7" cy="7" r="5" stroke="var(--color-text-muted)" strokeWidth="1.5" />
            <path
              d="M11 11l3 3"
              stroke="var(--color-text-muted)"
              strokeWidth="1.5"
              strokeLinecap="round"
            />
          </svg>
          <input
            ref={inputRef}
            type="text"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Search blocks, txs, addresses…"
            style={{
              flex: 1,
              background: "transparent",
              border: "none",
              outline: "none",
              color: "var(--color-text-primary)",
              fontSize: 14,
            }}
          />
          {isFetching && <span style={{ fontSize: 10, color: "var(--color-text-muted)" }}>…</span>}
          <kbd
            style={{
              padding: "1px 5px",
              background: "var(--color-bg-base)",
              border: "1px solid var(--color-border)",
              borderRadius: 3,
              fontSize: 10,
              color: "var(--color-text-muted)",
            }}
          >
            ESC
          </kbd>
        </div>

        {/* Results */}
        <div style={{ overflow: "auto", flex: 1 }}>
          {q.length >= 2 && results?.length === 0 && !isFetching && (
            <div
              style={{
                padding: 20,
                textAlign: "center",
                color: "var(--color-text-muted)",
                fontSize: 12,
              }}
            >
              No results for "{q}"
            </div>
          )}
          {Object.entries(grouped).map(([type, items]) => (
            <div key={type}>
              <div className="label" style={{ padding: "6px 14px 3px", fontSize: 10 }}>
                {TYPE_LABEL[type] ?? type}
              </div>
              {items.map((r) => (
                <button
                  key={r.value}
                  type="button"
                  onClick={() => handleSelect(r)}
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 10,
                    width: "100%",
                    padding: "7px 14px",
                    background: "transparent",
                    border: "none",
                    cursor: "pointer",
                    textAlign: "left",
                    color: "var(--color-text-primary)",
                    fontSize: 13,
                    transition: "background 0.1s",
                  }}
                  onMouseEnter={(e) => {
                    (e.currentTarget as HTMLElement).style.background = "rgba(59,130,246,0.08)";
                  }}
                  onMouseLeave={(e) => {
                    (e.currentTarget as HTMLElement).style.background = "transparent";
                  }}
                >
                  <TypeIcon type={type} />
                  <span
                    style={{
                      flex: 1,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}
                  >
                    {r.label}
                  </span>
                  {r.sub_text && (
                    <span
                      className="mono"
                      style={{ fontSize: 11, color: "var(--color-text-muted)" }}
                    >
                      {r.sub_text}
                    </span>
                  )}
                </button>
              ))}
            </div>
          ))}
          {q.length < 2 && (
            <div style={{ padding: "16px 14px" }}>
              <div className="label" style={{ fontSize: 10, marginBottom: 8 }}>
                Quick navigation
              </div>
              {[
                { label: "Blocks", path: "/blocks" },
                { label: "Health", path: "/health" },
                { label: "Pricing", path: "/pricing" },
                { label: "Opportunity", path: "/opportunity" },
                { label: "Simulator", path: "/simulator" },
              ].map((item) => (
                <button
                  key={item.path}
                  type="button"
                  onClick={() => {
                    onClose();
                    void navigate({ to: item.path as "/" });
                  }}
                  style={{
                    display: "block",
                    width: "100%",
                    padding: "5px 0",
                    background: "transparent",
                    border: "none",
                    cursor: "pointer",
                    textAlign: "left",
                    color: "var(--color-text-primary)",
                    fontSize: 13,
                  }}
                >
                  → {item.label}
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
    </>
  );
}

function TypeIcon({ type }: { type: string }) {
  const icons: Record<string, string> = {
    block: "□",
    tx: "⟶",
    address: "◎",
  };
  return (
    <span
      style={{ color: "var(--color-text-muted)", width: 16, textAlign: "center", fontSize: 12 }}
    >
      {icons[type] ?? "·"}
    </span>
  );
}

function groupBy<T>(arr: T[], fn: (item: T) => string): Record<string, T[]> {
  return arr.reduce<Record<string, T[]>>((acc, item) => {
    const key = fn(item);
    if (!acc[key]) acc[key] = [];
    acc[key]?.push(item);
    return acc;
  }, {});
}
