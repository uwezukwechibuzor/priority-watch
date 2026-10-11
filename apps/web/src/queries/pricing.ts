import { apiFetch } from "@/api/client";
import type { BacktestRunRow, ModelEvalRow, Recommendation, ReplayResult } from "@/api/types";
import { queryOptions } from "@tanstack/react-query";

function buildQS(obj: Record<string, string | number | undefined>): string {
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(obj)) {
    if (v !== undefined && v !== "") p.set(k, String(v));
  }
  return p.toString();
}

export function pricingRecommendQuery(
  block: number,
  n: number,
  confidence: number,
  window: number,
  model?: string,
) {
  return queryOptions({
    queryKey: ["pricing", "recommend", block, n, confidence, window, model],
    queryFn: () =>
      apiFetch<{ data: Recommendation[] | null }>(
        `/api/v1/pricing/recommend?${buildQS({ block, n, confidence, window, model })}`,
      ).then((r) => r.data ?? []),
    staleTime: 30_000,
  });
}

export const modelEvalQuery = queryOptions({
  queryKey: ["pricing", "models", "eval"],
  queryFn: () =>
    apiFetch<{ data: ModelEvalRow[] | null }>("/api/v1/pricing/models/eval").then(
      (r) => r.data ?? [],
    ),
  staleTime: 60_000,
});

export async function createBacktest(body: {
  strategy: string;
  from_block: number;
  to_block: number;
  config: Record<string, unknown>;
}): Promise<{ run_id: string }> {
  return apiFetch<{ run_id: string }>("/api/v1/backtest", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export async function getBacktestRun(runId: string): Promise<BacktestRunRow> {
  return apiFetch<BacktestRunRow>(`/api/v1/backtest/${runId}`);
}

export async function getBacktestResults(runId: string): Promise<ReplayResult> {
  return apiFetch<ReplayResult>(`/api/v1/backtest/${runId}/results`);
}
