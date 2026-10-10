import { apiFetch } from "@/api/client";
import type {
  BlockKPIs,
  BlockRow,
  EventRow,
  Pagination,
  PayerShareRow,
  SeriesPoint,
  TippedHistRow,
  TxRow,
} from "@/api/types";
import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

interface ListBlocksPage {
  data: BlockRow[] | null;
  pagination: Pagination;
}

interface BlockTxsPage {
  data: TxRow[] | null;
  pagination: Pagination;
}

export interface ListBlocksParams {
  range?: string;
  day?: string;
  search?: string;
  entity?: string;
  contested_only?: boolean;
}

function buildQS(obj: Record<string, string | boolean | undefined>): string {
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(obj)) {
    if (v !== undefined && v !== false && v !== "") p.set(k, String(v));
  }
  return p.toString();
}

export function blocksKpisQuery(range: string | undefined, day: string | undefined) {
  return queryOptions({
    queryKey: ["blocks", "kpis", range, day],
    queryFn: () => apiFetch<BlockKPIs>(`/api/v1/blocks/kpis?${buildQS({ range, day })}`),
    staleTime: 30_000,
  });
}

export function blocksSeriesQuery(range: string | undefined) {
  return queryOptions({
    queryKey: ["blocks", "series", range],
    queryFn: () =>
      apiFetch<{ data: SeriesPoint[] | null }>(`/api/v1/blocks/series?${buildQS({ range })}`),
    staleTime: 30_000,
  });
}

export function payerShareQuery(range: string | undefined) {
  return queryOptions({
    queryKey: ["blocks", "payer-share", range],
    queryFn: () =>
      apiFetch<{ data: PayerShareRow[] | null }>(
        `/api/v1/blocks/payer-share?${buildQS({ range })}`,
      ),
    staleTime: 30_000,
  });
}

export function tippedHistQuery(range: string | undefined) {
  return queryOptions({
    queryKey: ["blocks", "tipped-hist", range],
    queryFn: () =>
      apiFetch<{ data: TippedHistRow[] | null }>(
        `/api/v1/blocks/tipped-histogram?${buildQS({ range })}`,
      ),
    staleTime: 30_000,
  });
}

export function blocksInfiniteQuery(params: ListBlocksParams) {
  return infiniteQueryOptions({
    queryKey: ["blocks", "list", params],
    queryFn: ({ pageParam }) =>
      apiFetch<ListBlocksPage>(
        `/api/v1/blocks?${buildQS({ ...params, cursor: pageParam ?? "", limit: "100" })}`,
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.pagination?.next_cursor ?? undefined,
    staleTime: 10_000,
  });
}

export function blockDetailQuery(number: number) {
  return queryOptions({
    queryKey: ["blocks", "detail", number],
    queryFn: () => apiFetch<BlockRow>(`/api/v1/blocks/${number}`),
    staleTime: Number.POSITIVE_INFINITY,
  });
}

export function blockTxsQuery(number: number) {
  return queryOptions({
    queryKey: ["blocks", "txs", number],
    queryFn: () => apiFetch<BlockTxsPage>(`/api/v1/blocks/${number}/txs?limit=200`),
    staleTime: Number.POSITIVE_INFINITY,
  });
}

export function eventsQuery() {
  return queryOptions({
    queryKey: ["events"],
    queryFn: () => apiFetch<{ data: EventRow[] | null }>("/api/v1/events"),
    staleTime: Number.POSITIVE_INFINITY,
  });
}
