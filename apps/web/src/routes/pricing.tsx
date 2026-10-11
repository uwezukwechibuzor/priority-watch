import { ModelCards } from "@/components/pricing/ModelCards";
import { ModelEvalTable } from "@/components/pricing/ModelEvalTable";
import { PricingControls } from "@/components/pricing/PricingControls";
import type { PricingParams } from "@/components/pricing/PricingControls";
import { RecommendationPanel } from "@/components/pricing/RecommendationPanel";
import type { PricingSearch } from "@/lib/search-params";
import { pricingSearchSchema } from "@/lib/search-params";
import { modelEvalQuery, pricingRecommendQuery } from "@/queries/pricing";
import { rootRoute } from "@/router";
import { useQuery } from "@tanstack/react-query";
import { createRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

export const pricingRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/pricing",
  validateSearch: (s) => pricingSearchSchema.parse(s),
  component: PricingPage,
});

const DEFAULT_PARAMS: PricingParams = {
  block: 0,
  model: "all",
  n: 1,
  confidence: 0.8,
  window: 200,
};

function PricingPage() {
  const search = pricingRoute.useSearch();
  const navigate = useNavigate({ from: "/pricing" });

  const [params, setParams] = useState<PricingParams>({
    block: search.block ?? DEFAULT_PARAMS.block,
    model: search.model ?? DEFAULT_PARAMS.model,
    n: search.n ?? DEFAULT_PARAMS.n,
    confidence: search.confidence ?? DEFAULT_PARAMS.confidence,
    window: DEFAULT_PARAMS.window,
  });

  const [fetchKey, setFetchKey] = useState(0);

  const modelFilter = params.model === "all" ? undefined : params.model;

  const { data: recs, isLoading: recsLoading } = useQuery({
    ...pricingRecommendQuery(params.block, params.n, params.confidence, params.window, modelFilter),
    queryKey: [
      "pricing",
      "recommend",
      params.block,
      params.n,
      params.confidence,
      params.window,
      modelFilter,
      fetchKey,
    ],
  });

  const { data: evalRows, isLoading: evalLoading } = useQuery(modelEvalQuery);

  const bestRec =
    recs && recs.length > 0
      ? recs.reduce((best, r) => (r.hit_rate > best.hit_rate ? r : best), recs[0])
      : undefined;

  function handleParamsChange(p: PricingParams) {
    setParams(p);
    void navigate({
      search: (prev: PricingSearch) => ({
        ...prev,
        block: p.block || undefined,
        model: p.model !== "all" ? p.model : undefined,
        n: p.n !== 1 ? p.n : undefined,
        confidence: p.confidence !== 0.8 ? p.confidence : undefined,
      }),
    });
  }

  return (
    <div style={{ display: "flex", height: "calc(100vh - 84px)", overflow: "hidden" }}>
      <PricingControls
        params={params}
        onChange={handleParamsChange}
        onSubmit={() => setFetchKey((k) => k + 1)}
        loading={recsLoading}
      />

      <div
        style={{
          flex: 1,
          overflowY: "auto",
          padding: 16,
          display: "flex",
          flexDirection: "column",
          gap: 8,
        }}
      >
        <RecommendationPanel rec={bestRec} loading={recsLoading} />

        {recs && recs.length > 0 && (
          <ModelCards recommendations={recs} selectedModel={params.model} />
        )}

        <ModelEvalTable rows={evalRows ?? []} loading={evalLoading} />
      </div>
    </div>
  );
}
