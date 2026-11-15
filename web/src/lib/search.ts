import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";

export type SearchResults = components["schemas"]["SearchResults"];
export type SearchScope = "everywhere" | "space" | "project";
export type SearchContext = { spaceId?: string; containerId?: string };

export function effectiveScope(scope: SearchScope, context: SearchContext): SearchScope {
  if (scope === "space" && context.spaceId || scope === "project" && context.containerId) return scope;
  return "everywhere";
}

export type SearchExtras = { assigned?: boolean; status?: string; sort?: string };

export function useSearchResults(query: string, scope: SearchScope, context: SearchContext, limit: number, enabled = true, extras: SearchExtras = {}) {
  const params = new URLSearchParams({ q: query, limit: String(limit) });
  if (extras.assigned) params.set("assigned_to_me", "true");
  if (extras.status) params.set("status_group", extras.status);
  if (extras.sort) params.set("sort", extras.sort);
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

export const SEARCH_PAGE = 50;

/** Server-side hierarchical search for a Table filter: pages of hits, each with its ancestor `path` (RT-37). */
export function useSearchPages(query: string, context: SearchContext, enabled: boolean) {
  const base = new URLSearchParams({ q: query, limit: String(SEARCH_PAGE) });
  if (context.spaceId) base.set("space_id", context.spaceId);
  if (context.containerId) base.set("container_id", context.containerId);
  return useInfiniteQuery({
    queryKey: ["search-pages", base.toString()],
    initialPageParam: 0,
    queryFn: ({ pageParam, signal }) => api<SearchResults>(`/api/v1/search?${base}&offset=${pageParam}`, { signal }),
    getNextPageParam: (last, pages) => (last.items.length >= SEARCH_PAGE ? pages.length * SEARCH_PAGE : undefined),
    enabled: enabled && query.length >= 2,
    retry: false,
  });
}
