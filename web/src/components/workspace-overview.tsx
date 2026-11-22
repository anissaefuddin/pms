import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { formatDateTime, formatNumber, useProfile } from "@/lib/profile";
import { attentionSpaces, useOverviewPages, useOverviewSummary, type OverviewSort, type OverviewSpace } from "@/lib/overview";
import { Button } from "@/components/ui/button";
import { KpiCard } from "@/components/ui/kpi-card";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { WorkspaceTimeline } from "@/components/workspace-timeline";
import { WorkspaceAgenda } from "@/components/workspace-agenda";

const SORTS: OverviewSort[] = ["overdue", "name", "progress", "last_activity"];
const HEALTH = ["attention", "on_track", "empty"] as const;
const VIEWS = ["overview", "spaces", "timeline", "agenda"] as const;
type View = (typeof VIEWS)[number];
const select = "mt-1 block h-9 rounded-md border border-input bg-background px-2";

/** Read-only cross-Space summary. Everything here is what the server returns for the signed-in user. */
export function WorkspaceOverview() {
  const { t } = useTranslation();
  const [view, setView] = useState<View>("overview");
  return <section className="space-y-5" aria-label={t("dashboards.overview.title")}>
    <div className="flex flex-wrap items-center justify-between gap-3">
      <SegmentedControl aria-label={t("dashboards.overview.tabs.label")} value={view} onChange={(v) => setView(v as View)}
        items={VIEWS.map((key) => ({ value: key, label: t(`dashboards.overview.tabs.${key}`) }))} />
    </div>
    <p className="text-sm text-muted-foreground">{t("dashboards.overview.hint")}</p>
    {view === "overview" && <Summary />}
    {view === "spaces" && <SpaceTable />}
    {view === "timeline" && <WorkspaceTimeline />}
    {view === "agenda" && <WorkspaceAgenda />}
  </section>;
}

function Summary() {
  const { t } = useTranslation();
  const profile = useProfile();
  const q = useOverviewSummary();
  if (q.isPending) return <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.loading")}</p>;
  if (q.isError) return <div><p role="alert">{t("dashboards.overview.error")}</p>
    <Button variant="outline" className="mt-3" onClick={() => q.refetch()}>{t("dashboards.overview.retry")}</Button></div>;
  const { kpis, spaces, total, generated_at } = q.data;
  const health: Record<string, number> = {};
  for (const space of spaces) health[space.health] = (health[space.health] ?? 0) + 1;
  const attention = attentionSpaces(spaces);
  const n = (value: number) => formatNumber(value, profile);
  const total_spaces = spaces.length || 1;
  const tone: Record<string, string> = { attention: "bg-amber-500", on_track: "bg-emerald-500", empty: "bg-muted-foreground/30" };
  return <div className="space-y-6">
    <div className="flex flex-wrap items-center justify-between gap-2">
      <p className="text-xs text-muted-foreground">{t("dashboards.overview.updated", { time: formatDateTime(generated_at, profile) })}</p>
    </div>
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
      <KpiCard label={t("dashboards.overview.kpi.spaces")} value={n(kpis.accessible_spaces)} />
      <KpiCard label={t("dashboards.overview.kpi.projects")} value={n(kpis.active_projects)} />
      <KpiCard label={t("dashboards.overview.kpi.overdue")} value={n(kpis.overdue_tasks)} className={kpis.overdue_tasks > 0 ? "border-amber-400/60" : undefined} />
      <KpiCard label={t("dashboards.overview.kpi.completed")} value={n(kpis.completed_this_week)} className="border-emerald-400/60" />
      <KpiCard label={t("dashboards.overview.kpi.atRisk")} value={t("dashboards.overview.none")} hint={t("dashboards.overview.kpi.atRiskHint")} />
    </div>
    <div className="grid gap-6 lg:grid-cols-2">
      <section aria-labelledby="overview-health" className="space-y-3 rounded-xl border bg-card p-4">
        <h2 id="overview-health" className="text-sm font-semibold">{t("dashboards.overview.health.heading")}</h2>
        <div role="img" aria-label={HEALTH.map((key) => `${t(`dashboards.overview.health.${key}`)}: ${n(health[key] ?? 0)}`).join(", ")}
          className="flex h-2.5 overflow-hidden rounded-full bg-muted">
          {HEALTH.map((key) => (health[key] ?? 0) > 0 && <div key={key} className={tone[key]}
            style={{ width: `${((health[key] ?? 0) / total_spaces) * 100}%` }} />)}
        </div>
        <ul className="grid grid-cols-3 gap-2 text-sm">
          {HEALTH.map((key) => <li key={key} className="rounded-lg bg-muted/50 px-3 py-2">
            <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <span aria-hidden className={`size-2 rounded-full ${tone[key]}`} />{t(`dashboards.overview.health.${key}`)}</span>
            <span className="mt-0.5 block text-lg font-semibold tabular-nums">{n(health[key] ?? 0)}</span></li>)}
        </ul>
      </section>
      <section aria-labelledby="overview-attention" className="space-y-3 rounded-xl border bg-card p-4">
        <h2 id="overview-attention" className="text-sm font-semibold">{t("dashboards.overview.attention.heading")}</h2>
        {attention.length === 0
          ? <p className="text-sm text-muted-foreground">{t("dashboards.overview.attention.none")}</p>
          : <ul className="space-y-2">{attention.map((space) => <li key={space.id}>
              <Link to="/spaces/$spaceId" params={{ spaceId: space.id }}
                className="flex items-center justify-between gap-3 rounded-lg border px-3 py-2 text-sm transition-colors hover:bg-muted/60 focus-visible:ring-3 focus-visible:ring-ring/50 outline-none">
                <span className="truncate font-medium">{space.name}</span>
                <span className="shrink-0 rounded-full bg-amber-100 px-2 py-0.5 text-xs font-semibold tabular-nums text-amber-900 dark:bg-amber-500/20 dark:text-amber-200">
                  {t("dashboards.overview.attention.overdue", { value: n(space.overdue_count) })}</span>
              </Link></li>)}</ul>}
        {total > spaces.length && <p className="text-xs text-muted-foreground">{t("dashboards.overview.partial", { shown: spaces.length, total })}</p>}
      </section>
    </div>
  </div>;
}

function SpaceTable() {
  const { t } = useTranslation();
  const profile = useProfile();
  const [sort, setSort] = useState<OverviewSort>("overdue");
  const q = useOverviewPages(sort);
  if (q.isPending) return <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.loading")}</p>;
  if (q.isError) return <div><p role="alert">{t("dashboards.overview.error")}</p>
    <Button variant="outline" className="mt-3" onClick={() => q.refetch()}>{t("dashboards.overview.retry")}</Button></div>;
  const rows: OverviewSpace[] = q.data.pages.flatMap((page) => page.spaces);
  const n = (value: number) => formatNumber(value, profile);
  return <div className="space-y-4">
    <label className="block text-sm">{t("dashboards.overview.sort.label")}
      <select className={select} value={sort} onChange={(e) => setSort(e.target.value as OverviewSort)}>
        {SORTS.map((key) => <option key={key} value={key}>{t(`dashboards.overview.sort.${key}`)}</option>)}
      </select></label>
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead><tr className="border-b">
          <th scope="col" className="py-2 pr-4">{t("dashboards.overview.col.space")}</th>
          <th scope="col" className="py-2 pr-4 text-right">{t("dashboards.overview.col.projects")}</th>
          <th scope="col" className="py-2 pr-4 text-right">{t("dashboards.overview.col.tasks")}</th>
          <th scope="col" className="py-2 pr-4 text-right">{t("dashboards.overview.col.overdue")}</th>
          <th scope="col" className="py-2 pr-4 text-right">{t("dashboards.overview.col.progress")}</th>
          <th scope="col" className="py-2 pr-4">{t("dashboards.overview.col.lastActivity")}</th>
          <th scope="col" className="py-2">{t("dashboards.overview.col.access")}</th>
        </tr></thead>
        <tbody>{rows.map((space) => <tr key={space.id} className="border-b">
          <th scope="row" className="py-2 pr-4 text-left font-medium">
            <Link to="/spaces/$spaceId" params={{ spaceId: space.id }} className="underline">{space.name}</Link>
            {space.archived && <span className="ml-2 text-xs text-muted-foreground">{t("dashboards.overview.archived")}</span>}
          </th>
          <td className="py-2 pr-4 text-right tabular-nums">{n(space.project_count)}</td>
          <td className="py-2 pr-4 text-right tabular-nums">{n(space.task_count)}</td>
          <td className="py-2 pr-4 text-right tabular-nums">{n(space.overdue_count)}</td>
          <td className="py-2 pr-4 text-right tabular-nums">{space.progress_percent == null ? t("dashboards.overview.none") : t("dashboards.overview.percent", { value: space.progress_percent })}</td>
          <td className="py-2 pr-4 whitespace-nowrap">{space.last_activity ? formatDateTime(space.last_activity, profile) : t("dashboards.overview.none")}</td>
          <td className="py-2">{t(`dashboards.overview.access.${space.effective_access}`)}</td>
        </tr>)}</tbody>
      </table>
    </div>
    {q.hasNextPage && <Button variant="outline" onClick={() => void q.fetchNextPage()} disabled={q.isFetchingNextPage}>{t("dashboards.overview.loadMore")}</Button>}
  </div>;
}
