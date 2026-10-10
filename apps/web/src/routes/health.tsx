import { HealthKpiStrip } from "@/components/health/HealthKpiStrip";
import { HealthTabBar } from "@/components/health/HealthTabBar";
import { CompareTab } from "@/components/health/tabs/CompareTab";
import { OverviewTab } from "@/components/health/tabs/OverviewTab";
import { PayersTab } from "@/components/health/tabs/PayersTab";
import { PipelineTab } from "@/components/health/tabs/PipelineTab";
import { SourcesTab } from "@/components/health/tabs/SourcesTab";
import { TreasuryTab } from "@/components/health/tabs/TreasuryTab";
import { TrendsTab } from "@/components/health/tabs/TrendsTab";
import { useContextBarExtra } from "@/lib/context-bar-extra";
import type { HealthSearch } from "@/lib/search-params";
import { healthSearchSchema } from "@/lib/search-params";
import { eventsQuery } from "@/queries/blocks";
import {
  compareErasQuery,
  healthCompetitionQuery,
  healthConcentrationQuery,
  healthLeaderboardQuery,
  healthRevenueQuery,
  ingesterStatusQuery,
  treasuryDailyQuery,
  treasurySummaryQuery,
} from "@/queries/health";
import { rootRoute } from "@/router";
import { useQuery } from "@tanstack/react-query";
import { createRoute, useNavigate } from "@tanstack/react-router";
import { useCallback, useEffect } from "react";

export const healthRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/health",
  validateSearch: (s) => healthSearchSchema.parse(s),
  component: HealthPage,
});

const WINDOW_OPTIONS: { value: NonNullable<HealthSearch["window"]>; label: string }[] = [
  { value: "7d", label: "7d" },
  { value: "30d", label: "30d" },
  { value: "90d", label: "90d" },
  { value: "all", label: "All" },
];

function windowToRange(window: NonNullable<HealthSearch["window"]>): string {
  if (window === "90d") return "all";
  return window;
}

function windowToCompareDays(window: NonNullable<HealthSearch["window"]>): number {
  if (window === "7d") return 7;
  if (window === "30d") return 30;
  if (window === "90d") return 90;
  return 365;
}

function WindowSelector({
  window: win,
  onWindowChange,
}: {
  window: NonNullable<HealthSearch["window"]>;
  onWindowChange: (w: NonNullable<HealthSearch["window"]>) => void;
}) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 6, whiteSpace: "nowrap" }}>
      <span
        style={{
          fontSize: 10,
          color: "var(--color-text-muted)",
          textTransform: "uppercase",
          letterSpacing: "0.06em",
        }}
      >
        Window
      </span>
      <div style={{ display: "flex", gap: 2 }}>
        {WINDOW_OPTIONS.map((opt) => {
          const isActive = win === opt.value;
          return (
            <button
              key={opt.value}
              type="button"
              onClick={() => onWindowChange(opt.value)}
              style={{
                padding: "2px 7px",
                fontSize: 11,
                fontWeight: 500,
                background: isActive ? "var(--color-accent)" : "transparent",
                color: isActive ? "#fff" : "var(--color-text-muted)",
                border: "1px solid",
                borderColor: isActive ? "var(--color-accent)" : "var(--color-border)",
                borderRadius: "var(--radius-badge)",
                cursor: "pointer",
                textTransform: "uppercase",
                letterSpacing: "0.04em",
              }}
            >
              {opt.label}
            </button>
          );
        })}
      </div>
      <span style={{ fontSize: 10, color: "var(--color-text-muted)" }}>Since 2026-09-24</span>
    </div>
  );
}

function HealthPage() {
  const search = healthRoute.useSearch();
  const navigate = useNavigate({ from: "/health" });
  const { setExtraRight } = useContextBarExtra();

  const win = search.window ?? "30d";
  const activeTab = search.tab ?? "overview";
  const range = windowToRange(win);

  const setWindow = useCallback(
    (w: NonNullable<HealthSearch["window"]>) => {
      void navigate({ search: (prev) => ({ ...prev, window: w }) });
    },
    [navigate],
  );

  // Inject the window selector into the ContextBar extra slot
  useEffect(() => {
    setExtraRight(<WindowSelector window={win} onWindowChange={setWindow} />);
    return () => setExtraRight(null);
  }, [win, setWindow, setExtraRight]);

  const { data: daily = [], isLoading: dailyLoading } = useQuery(healthRevenueQuery(range));
  const { data: concentration = [] } = useQuery(healthConcentrationQuery(range));
  const { data: competition = [] } = useQuery(healthCompetitionQuery(range));
  const { data: leaderboard = [], isLoading: leaderboardLoading } = useQuery(
    healthLeaderboardQuery(range),
  );
  const { data: eventsData } = useQuery(eventsQuery());
  const { data: sources = [] } = useQuery(ingesterStatusQuery);
  const { data: treasuryDaily = [], isLoading: treasuryLoading } = useQuery(
    treasuryDailyQuery(range),
  );
  const { data: treasurySummary } = useQuery(treasurySummaryQuery);
  const { data: compareRows = [], isLoading: compareLoading } = useQuery(
    compareErasQuery(windowToCompareDays(win)),
  );

  return (
    <div style={{ padding: 16 }}>
      <HealthKpiStrip daily={daily} leaderboard={leaderboard} loading={dailyLoading} />

      <HealthTabBar activeTab={activeTab} />

      {activeTab === "overview" && (
        <OverviewTab
          daily={daily}
          concentration={concentration}
          competition={competition}
          leaderboard={leaderboard}
          events={eventsData?.data ?? null}
        />
      )}
      {activeTab === "trends" && <TrendsTab daily={daily} />}
      {activeTab === "payers" && <PayersTab rows={leaderboard} loading={leaderboardLoading} />}
      {activeTab === "treasury" && (
        <TreasuryTab daily={treasuryDaily} summary={treasurySummary} loading={treasuryLoading} />
      )}
      {activeTab === "compare" && <CompareTab rows={compareRows} loading={compareLoading} />}
      {activeTab === "sources" && <SourcesTab sources={sources} loading={sources.length === 0} />}
      {activeTab === "pipeline" && <PipelineTab sources={sources} />}
    </div>
  );
}
