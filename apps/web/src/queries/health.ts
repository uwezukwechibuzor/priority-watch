import { apiFetch } from "@/api/client";
import type { IngesterStatusRow } from "@/api/types";
import { queryOptions } from "@tanstack/react-query";

export const ingesterStatusQuery = queryOptions({
  queryKey: ["ingester-status"],
  queryFn: () =>
    apiFetch<{ data: IngesterStatusRow[] }>("/api/v1/health/ingester-status").then((r) => r.data),
  refetchInterval: 5_000,
  staleTime: 4_000,
});
