import { Suspense, lazy } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ApiError } from "@/lib/api";
import { OTHERS_KEY, foldOthers, toGrid } from "@/lib/charts";
import { isChart, type WidgetData, type WidgetType } from "@/lib/dashboards";
import type { AnalyticsQuery } from "@/lib/reports";
import { ResultTable, useResultLabels } from "@/components/result-table";

const WidgetChart = lazy(() => import("@/components/widget-chart"));

export type WidgetConfig = { show_legend?: boolean; show_labels?: boolean; pie_style?: "classic" | "donut" | "ring"; list?: string; template?: string; query?: AnalyticsQuery };

/** Draws one widget from its data: a task list, an indicator, a table or an accessible chart with a data table behind it. */
export function WidgetBody({ type, config, data, pending, error }: { type: WidgetType; config: WidgetConfig; data?: WidgetData; pending: boolean; error: Error | null }) {
  const { t } = useTranslation();
  if (pending) return <p role="status" className="text-sm text-muted-foreground">{t("dashboards.widget.loading")}</p>;
  if (error) {
    const status = (error as ApiError).status;
    return <p role="alert" className="text-sm text-destructive">{status === 422 ? (error.message || t("dashboards.widget.notReady")) : t("dashboards.widget.error")}</p>;
  }
  if (!data) return null;
  if (data.kind === "tasks") return <TaskList items={data.items ?? []} />;
  if (!config.query || !data.result) return <p className="text-sm text-muted-foreground">{t("dashboards.widget.notReady")}</p>;
  return <Series type={type} config={config} query={config.query} result={data.result} />;
}

function TaskList({ items }: { items: NonNullable<WidgetData["items"]> }) {
  const { t } = useTranslation();
  if (items.length === 0) return <p role="status" className="text-sm text-muted-foreground">{t("dashboards.widget.noTasks")}</p>;
  return <ul className="divide-y text-sm">{items.map((task) => <li key={task.id} className="flex items-baseline justify-between gap-3 py-1.5">
    <Link to="/tasks/$itemId" params={{ itemId: task.id }} className="min-w-0 truncate underline">{task.title}</Link>
    <span className="shrink-0 text-muted-foreground">{task.due_date || t("dashboards.widget.noDate")}</span></li>)}</ul>;
}

function Series({ type, config, query, result }: { type: WidgetType; config: WidgetConfig; query: AnalyticsQuery; result: NonNullable<WidgetData["result"]> }) {
  const { t } = useTranslation();
  const labels = useResultLabels(query);
  if (type === "indicator") return <p className="text-4xl font-semibold tabular-nums" aria-label={t("dashboards.widget.indicator", { metric: labels.metric, count: result.total })}>
    {result.total}<span className="ml-2 block text-sm font-normal text-muted-foreground">{labels.metric}</span></p>;
  if (result.rows.length === 0) return <p role="status" className="text-sm text-muted-foreground">{t("reports.result.empty")}</p>;
  if (!isChart(type)) return <ResultTable result={result} query={query} />;
  const label = (key: string, text: string, field: "group" | "break") => labels.value(field === "group" ? query.group_by : query.break_by, key, text);
  const summary = t("reports.result.chart", { metric: labels.metric, group: labels.group });
  const slices = foldOthers(result.rows.map((row) => ({ key: row.group.key, label: label(row.group.key, row.group.label, "group"), value: row.value })), 8, t("dashboards.widget.others"));
  const grid = toGrid(result.rows, label, labels.metric);
  return <div className="space-y-2">
    <Suspense fallback={<p role="status" className="text-sm text-muted-foreground">{t("dashboards.widget.loading")}</p>}>
      <WidgetChart type={type as "pie"} grid={grid} slices={slices.map((s) => ({ name: s.key === OTHERS_KEY ? t("dashboards.widget.others") : s.label, value: s.value }))}
        showLegend={config.show_legend ?? true} showLabels={config.show_labels ?? false} pieStyle={config.pie_style ?? "classic"} metric={labels.metric} summary={summary} />
    </Suspense>
    <details className="text-sm"><summary className="cursor-pointer underline">{t("dashboards.widget.showTable")}</summary>
      <ResultTable result={result} query={query} caption={summary} /></details>
  </div>;
}
