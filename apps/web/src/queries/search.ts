import { apiFetch } from "@/api/client";
import type { SearchResult } from "@/api/types";
import { queryOptions } from "@tanstack/react-query";

export const searchQuery = (q: string) =>
  queryOptions({
    queryKey: ["search", q],
    queryFn: () =>
      apiFetch<{ data: SearchResult[] }>(`/api/v1/search?q=${encodeURIComponent(q)}`).then(
        (r) => r.data,
      ),
    enabled: q.trim().length >= 2,
    staleTime: 30_000,
  });
