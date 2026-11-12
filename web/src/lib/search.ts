import { useQuery } from "@tanstack/react-query";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";

export type SearchResults = components["schemas"]["SearchResults"];
export type SearchScope = "everywhere" | "space" | "project";
export type SearchContext = { spaceId?: string; containerId?: string };

export function effectiveScope(scope: SearchScope, context: SearchContext): SearchScope {
  if (scope === "space" && context.spaceId || scope === "project" && context.containerId) return scope;
  return "everywhere";
}

export function useSearchResults(query: string, scope: SearchScope, context: SearchContext, limit: number, enabled = true) {
  const params = new URLSearchParams({ q: query, limit: String(limit) });
  if (scope === "space" && context.spaceId) params.set("space_id", context.spaceId);
  if (scope === "project" && context.containerId) {
    params.set("container_id", context.containerId);
    if (context.spaceId) params.set("space_id", context.spaceId);
  }
  return useQuery({
    queryKey: ["search", params.toString()],
    queryFn: ({ signal }) => api<SearchResults>(`/api/v1/search?${params}`, { signal }),
    enabled: enabled && query.length >= 2,
    retry: false,
  });
}
