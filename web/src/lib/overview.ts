import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import type { paths } from "@/api/schema";
import { api } from "@/lib/api";

// Response types come from api/openapi.yaml. Nothing here recomputes access: the server decides which Spaces,
// items and counts the actor may see.
export type OverviewResponse = paths["/overview"]["get"]["responses"][200]["content"]["application/json"];
export type OverviewSpace = OverviewResponse["spaces"][number];
export type OverviewSort = "name" | "overdue" | "progress" | "last_activity";

export type TimelineResponse = paths["/overview/timeline"]["get"]["responses"][200]["content"]["application/json"];
export type TimelineSpace = TimelineResponse["spaces"][number];
export type TimelineProject = TimelineSpace["projects"][number];
export type TimelineTask = TimelineSpace["unassigned_tasks"][number];

export type AgendaResponse = paths["/overview/agenda"]["get"]["responses"][200]["content"]["application/json"];
export type AgendaItem = AgendaResponse["items"][number];
export type AgendaKind = "" | "project" | "task";

/** Client cache window for aggregate numbers. The server still decides access on every request. */
export const OVERVIEW_STALE_MS = 2 * 60_000;
export const OVERVIEW_PAGE_SIZE = 25;
export const TIMELINE_DAYS = 60;
export const AGENDA_DAYS = 14;

/** Adds calendar days to a YYYY-MM-DD date. Pure date arithmetic, so no time zone is involved. */
export function addDays(date: string, n: number): string {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

function query(params: Record<string, string | number | null | undefined>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== null && value !== undefined && value !== "") search.set(key, String(value));
  }
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

export function overviewPath(params: { sort?: OverviewSort; limit?: number; cursor?: string | null }): string {
  return `/api/v1/overview${query({ sort: params.sort, limit: params.limit, cursor: params.cursor })}`;
}

/** The first page for the summary: KPIs plus the Spaces that need attention. */
export function useOverviewSummary() {
  return useQuery({
    queryKey: ["overview", "summary"],
    queryFn: () => api<OverviewResponse>(overviewPath({ sort: "overdue", limit: 100 })),
    staleTime: OVERVIEW_STALE_MS,
  });
}

/** Every Space in the table, one page at a time with the server's cursor. */
export function useOverviewPages(sort: OverviewSort) {
  return useInfiniteQuery({
    queryKey: ["overview", "spaces", sort],
    initialPageParam: null as string | null,
    queryFn: ({ pageParam }) => api<OverviewResponse>(overviewPath({ sort, limit: OVERVIEW_PAGE_SIZE, cursor: pageParam })),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
    staleTime: OVERVIEW_STALE_MS,
  });
}

/** Projects and Tasks due in a range. from null means today in the server's view of the user's zone. */
export function useOverviewTimeline(from: string | null, days: number = TIMELINE_DAYS) {
  return useQuery({
    queryKey: ["overview", "timeline", from, days],
    queryFn: () => api<TimelineResponse>(`/api/v1/overview/timeline${query({ from, days })}`),
    staleTime: OVERVIEW_STALE_MS,
  });
}

/** Due Projects and Tasks ordered by date, optionally narrowed to one Space or one type. */
export function useOverviewAgenda(from: string | null, spaceId: string, kind: AgendaKind, days: number = AGENDA_DAYS) {
  return useQuery({
    queryKey: ["overview", "agenda", from, spaceId, kind, days],
    queryFn: () => api<AgendaResponse>(`/api/v1/overview/agenda${query({ from, days, space_id: spaceId, type: kind })}`),
    staleTime: OVERVIEW_STALE_MS,
  });
}

/** Spaces that need attention, in the order the server returned them (sorted by overdue). */
export function attentionSpaces(spaces: OverviewSpace[], limit = 5): OverviewSpace[] {
  return spaces.filter((space) => space.health === "attention").slice(0, limit);
}
