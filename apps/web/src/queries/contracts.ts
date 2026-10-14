import { apiFetch } from "@/api/client";
import type { ContractRow } from "@/api/types";
import { queryOptions, useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

export const contractsQuery = queryOptions({
  queryKey: ["contracts"],
  queryFn: () => apiFetch<{ data: ContractRow[] }>("/api/v1/contracts").then((r) => r.data ?? []),
  staleTime: 60 * 60 * 1000, // 1 h — the list almost never changes
  gcTime: 24 * 60 * 60 * 1000, // keep in cache for 24 h
});

/** Returns a Map<lowercase_address, human_name> built once and memoized. */
export function useContractNames(): Map<string, string> {
  const { data } = useQuery(contractsQuery);
  return useMemo(() => {
    const m = new Map<string, string>();
    for (const c of data ?? []) {
      m.set(c.address.toLowerCase(), c.name);
    }
    return m;
  }, [data]);
}

/** Resolves a raw address to a contract name, or returns null if unknown. */
export function useContractName(address: string | undefined): string | null {
  const names = useContractNames();
  if (!address) return null;
  return names.get(address.toLowerCase()) ?? null;
}

/** Resolves a "calls 0x..." label produced by the heuristic-bot dbt model.
 *  Falls back to the original label when the address is not in the registry. */
export function resolveContractLabel(label: string, names: Map<string, string>): string {
  const CALLS_PREFIX = "calls 0x";
  if (!label.startsWith(CALLS_PREFIX)) return label;
  const addr = label.slice("calls ".length).toLowerCase();
  const name = names.get(addr);
  return name ? `calls ${name}` : label;
}
