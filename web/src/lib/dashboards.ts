import type { components } from "@/api/schema";

export type Dashboard = components["schemas"]["Dashboard"];
export type DashboardSummary = components["schemas"]["DashboardSummary"];
export type Widget = components["schemas"]["DashboardWidget"];
export type WidgetInput = components["schemas"]["DashboardWidgetInput"];
export type WidgetData = components["schemas"]["WidgetData"];
export type WidgetType = WidgetInput["type"];

export const CHART_TYPES = ["pie", "column", "bar", "line", "area", "burndown"] as const;
export const WIDGET_TYPES: WidgetType[] = ["task_list", "indicator", "completion", "pie", "column", "bar", "line", "area", "burndown", "table", "pivot"];
/** The "Choose widget" modal groups the kinds by what they show. */
export const WIDGET_CATEGORIES: { key: string; types: WidgetType[] }[] = [
  { key: "lists", types: ["task_list"] },
  { key: "numbers", types: ["indicator", "completion"] },
  { key: "charts", types: ["pie", "column", "bar", "line", "area"] },
  { key: "progress", types: ["burndown"] },
  { key: "tables", types: ["table", "pivot"] },
];
export const LISTS = ["todo_this_week", "to_review", "completed", "assigned_to_me"] as const;
export const TEMPLATES = ["my_tasks", "overdue", "followed", "backlog", "starred"] as const;
export const isChart = (type: string) => (CHART_TYPES as readonly string[]).includes(type);
