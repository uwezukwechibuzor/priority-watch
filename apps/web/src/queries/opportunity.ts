import { apiFetch } from "@/api/client";
import type {
  ExecQualityRow,
  HeatmapCell,
  HistogramBin,
  MarkoutRow,
  PositionRow,
} from "@/api/types";
import { queryOptions } from "@tanstack/react-query";

function buildQS(obj: Record<string, string | undefined>): string {
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(obj)) {
    if (v !== undefined && v !== "") p.set(k, v);
  }
  return p.toString();
}

export const spreadHeatmapQuery = queryOptions({
  queryKey: ["opportunity", "heatmap"],
  queryFn: () =>
    apiFetch<{ data: HeatmapCell[] | null }>("/api/v1/opportunity/heatmap").then(
      (r) => r.data ?? [],
    ),
  staleTime: 60_000,
});

export const spreadHistogramQuery = queryOptions({
  queryKey: ["opportunity", "histogram"],
  queryFn: () =>
    apiFetch<{ data: HistogramBin[] | null }>("/api/v1/opportunity/histogram").then(
      (r) => r.data ?? [],
    ),
  staleTime: 60_000,
});

export function markoutsQuery(range: string) {
  return queryOptions({
    queryKey: ["opportunity", "markouts", range],
    queryFn: () =>
      apiFetch<{ data: MarkoutRow[] | null }>(
        `/api/v1/opportunity/markouts?${buildQS({ range, limit: "500" })}`,
      ).then((r) => r.data ?? []),
    staleTime: 60_000,
  });
}

export function positionSensitivityQuery(range: string) {
  return queryOptions({
    queryKey: ["opportunity", "position-sensitivity", range],
    queryFn: () =>
      apiFetch<{ data: PositionRow[] | null }>(
        `/api/v1/opportunity/position-sensitivity?${buildQS({ range })}`,
      ).then((r) => r.data ?? []),
    staleTime: 60_000,
  });
}

export function executionQualityQuery(range: string) {
  return queryOptions({
    queryKey: ["execution", "quality", range],
    queryFn: () =>
      apiFetch<{ data: ExecQualityRow[] | null }>(
        `/api/v1/execution/quality?${buildQS({ range })}`,
      ).then((r) => r.data ?? []),
    staleTime: 60_000,
  });
}
