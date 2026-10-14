import { apiFetch } from "@/api/client";
import type {
  AllPayerRow,
  BaseFeeHourlyRow,
  EraComparisonRow,
  HealthDailyRow,
  IngesterStatusRow,
  PayerShareRow,
  TreasuryDailyRow,
  TreasurySummary,
} from "@/api/types";
import { queryOptions } from "@tanstack/react-query";

export const ingesterStatusQuery = queryOptions({
  queryKey: ["ingester-status"],
  queryFn: () =>
    apiFetch<{ data: IngesterStatusRow[] }>("/api/v1/health/ingester-status").then((r) => r.data),
  refetchInterval: 5_000,
  staleTime: 4_000,
});

function buildQS(obj: Record<string, string | undefined>): string {
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(obj)) {
    if (v !== undefined && v !== "") p.set(k, v);
  }
  return p.toString();
}

export function healthRevenueQuery(range: string) {
  return queryOptions({
    queryKey: ["health", "revenue", range],
    queryFn: () =>
      apiFetch<{ data: HealthDailyRow[] | null }>(
        `/api/v1/health/revenue?${buildQS({ range })}`,
      ).then((r) => r.data ?? []),
    staleTime: 60_000,
  });
}

export function healthConcentrationQuery(range: string) {
  return queryOptions({
    queryKey: ["health", "concentration", range],
    queryFn: () =>
      apiFetch<{ data: HealthDailyRow[] | null }>(
        `/api/v1/health/concentration?${buildQS({ range })}`,
      ).then((r) => r.data ?? []),
    staleTime: 60_000,
  });
}

export function healthCompetitionQuery(range: string) {
  return queryOptions({
    queryKey: ["health", "competition", range],
    queryFn: () =>
      apiFetch<{ data: HealthDailyRow[] | null }>(
        `/api/v1/health/competition?${buildQS({ range })}`,
      ).then((r) => r.data ?? []),
    staleTime: 60_000,
  });
}

export function healthLeaderboardQuery(range: string) {
  return queryOptions({
    queryKey: ["health", "leaderboard", range],
    queryFn: () =>
      apiFetch<{ data: PayerShareRow[] | null }>(
        `/api/v1/health/leaderboard?${buildQS({ range })}`,
      ).then((r) => r.data ?? []),
    staleTime: 60_000,
  });
}

export function healthBaseFeeQuery(range: string) {
  return queryOptions({
    queryKey: ["health", "base-fee", range],
    queryFn: () =>
      apiFetch<{ data: BaseFeeHourlyRow[] | null }>(
        `/api/v1/health/base-fee?${buildQS({ range })}`,
      ).then((r) => r.data ?? []),
    staleTime: 60_000,
  });
}

export function healthRevertRatesQuery(range: string) {
  return queryOptions({
    queryKey: ["health", "revert-rates", range],
    queryFn: () =>
      apiFetch<{ data: HealthDailyRow[] | null }>(
        `/api/v1/health/revert-rates?${buildQS({ range })}`,
      ).then((r) => r.data ?? []),
    staleTime: 60_000,
  });
}

export function treasuryDailyQuery(range: string) {
  return queryOptions({
    queryKey: ["treasury", "daily", range],
    queryFn: () =>
      apiFetch<{ data: TreasuryDailyRow[] | null }>(
        `/api/v1/treasury/daily?${buildQS({ range })}`,
      ).then((r) => r.data ?? []),
    staleTime: 60_000,
  });
}

export const treasurySummaryQuery = queryOptions({
  queryKey: ["treasury", "summary"],
  queryFn: () => apiFetch<TreasurySummary>("/api/v1/treasury/summary"),
  staleTime: 60_000,
});

export function treasuryReconQuery(range: string) {
  return queryOptions({
    queryKey: ["treasury", "reconciliation", range],
    queryFn: () =>
      apiFetch<{ data: TreasuryDailyRow[] | null }>(
        `/api/v1/treasury/reconciliation?${buildQS({ range })}`,
      ).then((r) => r.data ?? []),
    staleTime: 60_000,
  });
}

export const allPayersQuery = queryOptions({
  queryKey: ["health", "payers"],
  queryFn: () =>
    apiFetch<{ data: AllPayerRow[] | null }>("/api/v1/health/payers?limit=200").then(
      (r) => r.data ?? [],
    ),
  staleTime: 300_000,
});

export function compareErasQuery(windowDays: number) {
  return queryOptions({
    queryKey: ["compare", "eras", windowDays],
    queryFn: () =>
      apiFetch<{ data: EraComparisonRow[] | null }>(
        `/api/v1/compare/eras?window_days=${windowDays}`,
      ).then((r) => r.data ?? []),
    staleTime: 120_000,
  });
}
