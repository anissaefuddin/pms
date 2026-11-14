import type { TFunction } from "i18next";
import { api } from "@/lib/api";
import type { Dashboard } from "@/lib/dashboards";

/** The ready-made analytics board of a Space (DASH-08): five widgets that need no setup. */
export async function createAnalyticsBoard(spaceId: string, t: TFunction) {
  const q = (extra: object) => ({ reporting_on: "tasks", metric: "count", filters: {}, ...extra });
  const widgets = [
    { name: t("dashboards.analytics.completion"), type: "completion", config: { query: q({ metric: "completion_percent" }) } },
    { name: t("dashboards.analytics.byStatus"), type: "pie", config: { show_legend: true, query: q({ group_by: "status_group" }) } },
    { name: t("dashboards.analytics.burndown"), type: "burndown", config: { query: q({}) } },
    { name: t("dashboards.analytics.pivot"), type: "pivot", config: { query: q({ group_by: "importance", break_by: "status_group" }) } },
    { name: t("dashboards.analytics.byAssignee"), type: "bar", config: { query: q({ group_by: "assignee", filters: { status_groups: ["active"] } }) } },
  ];
  let board = await api<Dashboard>("/api/v1/dashboards", { method: "POST", body: JSON.stringify({ name: t("dashboards.analytics.title"), space_id: spaceId }) });
  for (const [position, widget] of widgets.entries()) {
    board = await api<Dashboard>(`/api/v1/dashboards/${board.id}/widgets`, { method: "POST", headers: { "If-Match": `"${board.version}"` }, body: JSON.stringify({ ...widget, position }) });
  }
  return board;
}
