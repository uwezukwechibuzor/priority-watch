import { ExecutionQualityTable } from "@/components/opportunity/ExecutionQualityTable";
import { MarkoutScatterPanel } from "@/components/opportunity/MarkoutScatterPanel";
import { PositionLeaderboard } from "@/components/opportunity/PositionLeaderboard";
import { SpreadHeatmapPanel } from "@/components/opportunity/SpreadHeatmapPanel";
import { SpreadHistPanel } from "@/components/opportunity/SpreadHistPanel";
import { RangeButtons } from "@/components/ui";
import type { OpportunitySearch, Range } from "@/lib/search-params";
import { opportunitySearchSchema } from "@/lib/search-params";
import {
  executionQualityQuery,
  markoutsQuery,
  positionSensitivityQuery,
  spreadHeatmapQuery,
  spreadHistogramQuery,
} from "@/queries/opportunity";
import { rootRoute } from "@/router";
import { useQuery } from "@tanstack/react-query";
import { createRoute, useNavigate } from "@tanstack/react-router";

export const opportunityRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/opportunity",
  validateSearch: (s) => opportunitySearchSchema.parse(s),
  component: OpportunityPage,
});

function OpportunityPage() {
  const search = opportunityRoute.useSearch();
  const navigate = useNavigate({ from: "/opportunity" });
  const range = search.range ?? "24h";

  function setRange(r: string) {
    void navigate({
      search: (prev: OpportunitySearch) => ({ ...prev, range: r as OpportunitySearch["range"] }),
    });
  }

  const { data: heatmap = [], isLoading: heatmapLoading } = useQuery(spreadHeatmapQuery);
  const { data: histogram = [], isLoading: histLoading } = useQuery(spreadHistogramQuery);
  const { data: markouts = [], isLoading: markoutsLoading } = useQuery(markoutsQuery(range));
  const { data: positions = [], isLoading: posLoading } = useQuery(positionSensitivityQuery(range));
  const { data: execQuality = [], isLoading: execLoading } = useQuery(executionQualityQuery(range));

  return (
    <div style={{ padding: 16, display: "flex", flexDirection: "column", gap: 8 }}>
      {/* Toolbar */}
      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
        <span
          style={{ fontSize: 10, textTransform: "uppercase", color: "var(--color-text-muted)" }}
        >
          Range
        </span>
        <RangeButtons value={range as Range} onChange={(r) => setRange(r)} />
      </div>

      <SpreadHeatmapPanel data={heatmap} loading={heatmapLoading} />
      <SpreadHistPanel data={histogram} loading={histLoading} />
      <MarkoutScatterPanel data={markouts} loading={markoutsLoading} />
      <PositionLeaderboard data={positions} loading={posLoading} />
      <ExecutionQualityTable data={execQuality} loading={execLoading} />

      {/* Entropy benchmark note */}
      <div
        style={{
          background: "rgba(245,158,11,0.08)",
          border: "1px solid rgba(245,158,11,0.3)",
          borderRadius: "var(--radius-panel)",
          padding: "10px 16px",
          fontSize: 11,
          color: "var(--color-text-muted)",
          lineHeight: 1.6,
        }}
      >
        <strong style={{ color: "var(--color-amber)" }}>
          Entropy research (Mar 2026 — different methodology):
        </strong>{" "}
        Ethereum 28.83% · Base 7.34% · Timeboost (Arb One) 1.74%. Source: Arbitrum Forum AIP thread.
      </div>
    </div>
  );
}
