import type { components } from "@/api/schema";

export type Report = components["schemas"]["Report"];
export type ReportSummary = components["schemas"]["ReportSummary"];
export type AnalyticsQuery = components["schemas"]["AnalyticsQuery"];
export type AnalyticsFilters = components["schemas"]["AnalyticsFilters"];
export type AnalyticsResult = components["schemas"]["AnalyticsResult"];

export const SOURCES = ["tasks", "projects", "time_entries"] as const;
export type Source = (typeof SOURCES)[number];

const itemDimensions = ["assignee", "status", "status_group", "importance", "type", "space", "due_month", "created_month"];
export const METRICS: Record<Source, string[]> = {
  tasks: ["count", "effort_minutes", "completion_percent"], projects: ["count", "effort_minutes", "completion_percent"], time_entries: ["minutes", "billable_minutes", "utilization", "count"],
};
export const DIMENSIONS: Record<Source, string[]> = {
  tasks: itemDimensions, projects: itemDimensions, time_entries: ["user", "item", "category", "billing_type", "entry_month"],
};
export const STATUS_GROUPS = ["active", "completed", "deferred", "cancelled"] as const;
export const IMPORTANCE = ["high", "normal", "low"] as const;
/** Dimensions whose values are a fixed vocabulary that the UI translates. */
export const ENUM_DIMENSIONS = ["status_group", "importance", "type", "billing_type"];

export const defaultQuery = (source: Source): AnalyticsQuery => ({ reporting_on: source, metric: METRICS[source][0] as AnalyticsQuery["metric"], filters: {} });

/** Switching the source keeps the Spaces and drops every choice the new source does not know. */
export function changeSource(query: AnalyticsQuery, source: Source): AnalyticsQuery {
  return { ...defaultQuery(source), space_ids: query.space_ids, container_ids: query.container_ids };
}

/** Removes empty filter values so the stored definition stays small and comparable. */
export function cleanQuery(query: AnalyticsQuery): AnalyticsQuery {
  const filters = Object.fromEntries(Object.entries(query.filters ?? {}).filter(([, v]) =>
    v !== undefined && v !== "" && v !== false && !(Array.isArray(v) && v.length === 0)));
  return { ...query, filters, group_by: query.group_by || undefined, break_by: query.group_by ? query.break_by || undefined : undefined,
    space_ids: query.space_ids?.length ? query.space_ids : undefined, container_ids: query.container_ids?.length ? query.container_ids : undefined };
}

export const weekdays = [0, 1, 2, 3, 4, 5, 6];
