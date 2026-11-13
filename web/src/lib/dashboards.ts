import type { components } from "@/api/schema";

export type Dashboard = components["schemas"]["Dashboard"];
export type DashboardSummary = components["schemas"]["DashboardSummary"];
export type Widget = components["schemas"]["DashboardWidget"];
export type WidgetInput = components["schemas"]["DashboardWidgetInput"];
export type WidgetData = components["schemas"]["WidgetData"];
export type WidgetType = WidgetInput["type"];

export const CHART_TYPES = ["pie", "column", "bar", "line"] as const;
export const WIDGET_TYPES: WidgetType[] = ["task_list", "indicator", "pie", "column", "bar", "line", "table"];
export const LISTS = ["todo_this_week", "to_review", "completed", "assigned_to_me"] as const;
export const TEMPLATES = ["my_tasks", "overdue", "followed", "backlog", "starred"] as const;
export const isChart = (type: string) => (CHART_TYPES as readonly string[]).includes(type);
