import { Suspense, lazy } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ApiError } from "@/lib/api";
import { OTHERS_KEY, foldOthers, toGrid } from "@/lib/charts";
import { isChart, type WidgetData, type WidgetType } from "@/lib/dashboards";
import type { AnalyticsQuery } from "@/lib/reports";
import { StatusPill } from "@/components/todo-chips";
import { bucketOrder, dueBucket, groupBy } from "@/lib/todo-groups";
import { ResultTable, useResultLabels } from "@/components/result-table";

const WidgetChart = lazy(() => import("@/components/widget-chart"));

export type WidgetConfig = { description?: string; show_legend?: boolean; show_labels?: boolean; pie_style?: "classic" | "donut" | "ring"; list?: string; template?: string; query?: AnalyticsQuery };

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
  const series = <Series type={type} config={config} query={config.query} result={data.result} />;
  // A table says it itself; a chart or pivot needs the note beside it.
  return data.result.cost_status === "partial" && (type === "indicator" || (data.result.rows.length > 0 && (type === "pivot" || isChart(type))))
    ? <><p role="status" className="text-xs text-muted-foreground">{t("reports.result.partialCost")}</p>{series}</> : series;
}

/** Rows are the group, columns the break; every row and column carries its total (DASH-03B). */
function PivotTable({ result, query }: { result: NonNullable<WidgetData["result"]>; query: AnalyticsQuery }) {
  const { t } = useTranslation();
  const labels = useResultLabels(query);
  const columns = [...new Map(result.rows.filter((r) => r.break).map((r) => [r.break!.key, r.break!.label])).entries()];
  const rows = [...new Map(result.rows.map((r) => [r.group.key, r.group.label])).entries()];
  const cell = (g: string, b: string) => result.rows.find((r) => r.group.key === g && r.break?.key === b)?.value ?? 0;
  const rowTotal = (g: string) => columns.reduce((sum, [b]) => sum + cell(g, b), 0);
  return <div className="overflow-x-auto"><table className="w-full text-left text-sm">
    <caption className="sr-only">{t("reports.result.chart", { metric: labels.metric, group: labels.group })}</caption>
    <thead><tr className="border-b"><th scope="col" className="py-2 pr-4">{labels.group}</th>
      {columns.map(([key, label]) => <th key={key} scope="col" className="py-2 pr-4 text-right">{labels.value(query.break_by, key, label)}</th>)}
      <th scope="col" className="py-2 pr-4 text-right">{t("dashboards.widget.rowTotal")}</th></tr></thead>
    <tbody>{rows.map(([g, label]) => <tr key={g} className="border-b"><th scope="row" className="py-2 pr-4 font-normal">{labels.value(query.group_by, g, label)}</th>
      {columns.map(([b]) => <td key={b} className="py-2 pr-4 text-right tabular-nums">{cell(g, b)}</td>)}
      <td className="py-2 pr-4 text-right font-medium tabular-nums">{rowTotal(g)}</td></tr>)}</tbody>
    <tfoot><tr><th scope="row" className="py-2 pr-4 text-left">{t("dashboards.widget.columnTotal")}</th>
      {columns.map(([b]) => <td key={b} className="py-2 pr-4 text-right font-medium tabular-nums">{rows.reduce((sum, [g]) => sum + cell(g, b), 0)}</td>)}
      <td className="py-2 pr-4 text-right font-medium tabular-nums">{result.total}</td></tr></tfoot>
  </table></div>;
}

function TaskList({ items }: { items: NonNullable<WidgetData["items"]> }) {
  const { t } = useTranslation();
  if (items.length === 0) return <p role="status" className="text-sm text-muted-foreground">{t("dashboards.widget.noTasks")}</p>;
  const today = new Date().toLocaleDateString("sv");
  const groups = groupBy(items, (task) => dueBucket(task.due_date, today), bucketOrder);
  return <div className="space-y-3 text-sm">{groups.map((group) => <div role="group" key={group.key} aria-label={t(`dashboards.widget.group.${group.key}`)}>
    <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{t(`dashboards.widget.group.${group.key}`)} ({group.rows.length})</h3>
    <ul className="divide-y">{group.rows.map((task) => <li key={task.id} className="flex items-center justify-between gap-3 py-1.5">
      <Link to="/tasks/$itemId" params={{ itemId: task.id }} className="min-w-0 truncate underline">{task.title}</Link>
      <span className="flex shrink-0 items-center gap-2">
        <StatusPill name={task.status} group={task.status_group} />
        <span className="text-muted-foreground">{task.due_date || t("dashboards.widget.noDate")}</span></span></li>)}</ul></div>)}</div>;
}

function Series({ type, config, query, result }: { type: WidgetType; config: WidgetConfig; query: AnalyticsQuery; result: NonNullable<WidgetData["result"]> }) {
  const { t } = useTranslation();
  const labels = useResultLabels(query);
  const { i18n } = useTranslation();
  // Money comes in minor units; a partial cost with nothing known is not a zero.
  const total = query.metric !== "actual_cost_minor" ? result.total : result.cost_status === "partial" && result.total === 0 ? "—"
    : new Intl.NumberFormat(i18n.language, { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(result.total / 100);
  if (type === "indicator") return <p className="text-4xl font-semibold tabular-nums" aria-label={t("dashboards.widget.indicator", { metric: labels.metric, count: total as number })}>
    {total}<span className="ml-2 block text-sm font-normal text-muted-foreground">{labels.metric}</span></p>;
  if (type === "completion") return <div className="space-y-1">
    <p className="text-3xl font-semibold tabular-nums">{result.total}%</p>
    <div role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={result.total} aria-label={labels.metric} className="h-3 overflow-hidden rounded bg-muted">
      <div className="h-full bg-primary" style={{ width: `${Math.min(100, Math.max(0, result.total))}%` }} /></div>
    <p className="text-sm text-muted-foreground">{labels.metric}</p></div>;
  if (result.rows.length === 0) return <p role="status" className="text-sm text-muted-foreground">{t(result.cost_status === "partial" ? "reports.result.partialCost" : "reports.result.empty")}</p>;
  if (type === "pivot") return <PivotTable result={result} query={query} />;
  if (!isChart(type)) return <ResultTable result={result} query={query} />;
  const label = (key: string, text: string, field: "group" | "break") => labels.value(field === "group" ? query.group_by : query.break_by, key, text);
  const summary = t("reports.result.chart", { metric: labels.metric, group: labels.group });
  const slices = foldOthers(result.rows.map((row) => ({ key: row.group.key, label: label(row.group.key, row.group.label, "group"), value: row.value })), 8, t("dashboards.widget.others"));
  const grid = toGrid(result.rows, label, labels.metric);
  return <div className="space-y-2">
    <Suspense fallback={<p role="status" className="text-sm text-muted-foreground">{t("dashboards.widget.loading")}</p>}>
      <WidgetChart type={type as "pie" | "area" | "burndown"} grid={grid} slices={slices.map((s) => ({ name: s.key === OTHERS_KEY ? t("dashboards.widget.others") : s.label, value: s.value }))}
        showLegend={config.show_legend ?? true} showLabels={config.show_labels ?? false} pieStyle={config.pie_style ?? "classic"} metric={labels.metric} summary={summary} />
    </Suspense>
    <details className="text-sm"><summary className="cursor-pointer underline">{t("dashboards.widget.showTable")}</summary>
      <ResultTable result={result} query={query} caption={summary} /></details>
  </div>;
}
