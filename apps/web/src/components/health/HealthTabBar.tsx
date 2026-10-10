import type { HealthSearch } from "@/lib/search-params";
import { useNavigate } from "@tanstack/react-router";

type Tab = NonNullable<HealthSearch["tab"]>;

const TABS: { key: Tab; label: string }[] = [
  { key: "overview", label: "Overview" },
  { key: "trends", label: "Trends" },
  { key: "payers", label: "Payers" },
  { key: "treasury", label: "Treasury" },
  { key: "compare", label: "Compare" },
  { key: "sources", label: "Sources" },
  { key: "pipeline", label: "Pipeline" },
];

interface HealthTabBarProps {
  activeTab: Tab;
}

export function HealthTabBar({ activeTab }: HealthTabBarProps) {
  const navigate = useNavigate({ from: "/health" });

  function setTab(tab: Tab) {
    void navigate({ search: (prev) => ({ ...prev, tab }) });
  }

  return (
    <div
      style={{
        display: "flex",
        gap: 0,
        borderBottom: "1px solid var(--color-border)",
        marginBottom: 12,
        overflowX: "auto",
      }}
    >
      {TABS.map((t) => {
        const isActive = t.key === activeTab;
        return (
          <button
            key={t.key}
            type="button"
            onClick={() => setTab(t.key)}
            style={{
              padding: "8px 16px",
              fontSize: 12,
              fontWeight: isActive ? 600 : 400,
              background: "transparent",
              border: "none",
              borderBottom: isActive ? "2px solid var(--color-accent)" : "2px solid transparent",
              color: isActive ? "var(--color-text-primary)" : "var(--color-text-muted)",
              cursor: "pointer",
              whiteSpace: "nowrap",
              transition: "color 0.1s, border-color 0.1s",
              marginBottom: -1,
            }}
          >
            {t.label}
          </button>
        );
      })}
    </div>
  );
}
