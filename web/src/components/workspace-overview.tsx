import { useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { formatDate, formatDateTime, formatNumber, useProfile } from "@/lib/profile";
import { AGENDA_DAYS, attentionSpaces, useOverviewAgenda, useOverviewPages, useOverviewSummary, useOverviewTimeline, TIMELINE_DAYS, type OverviewSort, type OverviewSpace } from "@/lib/overview";
import { Layers, Search } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { KpiCard } from "@/components/ui/kpi-card";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { WorkspaceTimeline } from "@/components/workspace-timeline";
import { WorkspaceAgenda } from "@/components/workspace-agenda";
import { WorkspaceTasks, type TasksSeed } from "@/components/workspace-tasks";

const SORTS: OverviewSort[] = ["overdue", "name", "progress", "last_activity"];
const HEALTH = ["attention", "on_track", "empty"] as const;
const VIEWS = ["overview", "spaces", "timeline", "agenda", "tasks"] as const;
type View = (typeof VIEWS)[number];
const select = "mt-1 block h-9 rounded-md border border-input bg-background px-2";
const HEALTH_TONE: Record<string, string> = {
  attention: "bg-amber-100 text-amber-900 dark:bg-amber-500/20 dark:text-amber-200",
  on_track: "bg-emerald-100 text-emerald-900 dark:bg-emerald-500/20 dark:text-emerald-200",
  empty: "bg-muted text-muted-foreground",
};
const BAR_TONE: Record<string, string> = { attention: "bg-amber-500", on_track: "bg-emerald-500", empty: "bg-muted-foreground/30" };

/** Read-only cross-Space summary. Everything here is what the server returns for the signed-in user. */
export function WorkspaceOverview() {
  const { t } = useTranslation();
  const [view, setView] = useState<View>("overview");
  // A figure on the Summary opens the Tasks tab already filtered; choosing a tab by hand starts it clean.
  const [tasksSeed, setTasksSeed] = useState<TasksSeed>({});
  const openTasks = (seed: TasksSeed) => { setTasksSeed(seed); setView("tasks"); };
  return <section className="space-y-5" aria-label={t("dashboards.overview.title")}>
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div className="max-w-full overflow-x-auto"><SegmentedControl aria-label={t("dashboards.overview.tabs.label")} value={view} onChange={(v) => { setView(v as View); setTasksSeed({}); }}
        items={VIEWS.map((key) => ({ value: key, label: t(`dashboards.overview.tabs.${key}`) }))} /></div>
    </div>
    <p className="text-sm text-muted-foreground">{t("dashboards.overview.hint")}</p>
    {view === "overview" && <Summary onOpenTasks={openTasks} />}
    {view === "spaces" && <SpaceTable onOpenTasks={openTasks} />}
    {view === "timeline" && <WorkspaceTimeline />}
    {view === "agenda" && <WorkspaceAgenda />}
    {view === "tasks" && <WorkspaceTasks seed={tasksSeed} />}
  </section>;
}

function Summary({ onOpenTasks }: { onOpenTasks: (seed: TasksSeed) => void }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const q = useOverviewSummary();
  const milestones = useOverviewAgenda(null, "", "project");
  const timeline = useOverviewTimeline(null, TIMELINE_DAYS);
  if (q.isPending) return <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.loading")}</p>;
  if (q.isError) return <div><p role="alert">{t("dashboards.overview.error")}</p>
    <Button variant="outline" className="mt-3" onClick={() => q.refetch()}>{t("dashboards.overview.retry")}</Button></div>;
  const { kpis, spaces, total, generated_at } = q.data;
  const health: Record<string, number> = {};
  for (const space of spaces) health[space.health] = (health[space.health] ?? 0) + 1;
  const attention = attentionSpaces(spaces);
  const n = (value: number) => formatNumber(value, profile);
  const total_spaces = spaces.length || 1;
  const upcoming = (milestones.data?.items ?? []).slice(0, 5);
  const loadMap = (timeline.data?.spaces ?? []).map((space) => ({
    id: space.id, name: space.name,
    count: space.projects.length + space.unassigned_tasks.length + space.projects.reduce((sum, p) => sum + p.tasks.length, 0),
  }));
  const peak = Math.max(1, ...loadMap.map((row) => row.count));
  return <div className="space-y-6">
    <div className="flex flex-wrap items-baseline justify-between gap-2">
      <h2 className="text-lg font-semibold">{t("dashboards.overview.title")}</h2>
      <p className="text-xs text-muted-foreground">{t("dashboards.overview.updated", { time: formatDateTime(generated_at, profile) })}</p>
    </div>
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
      <KpiCard label={t("dashboards.overview.kpi.spaces")} value={n(kpis.accessible_spaces)} />
      <KpiCard label={t("dashboards.overview.kpi.projects")} value={n(kpis.active_projects)} />
      <button type="button" onClick={() => onOpenTasks({ preset: "overdue" })}
        className="rounded-xl text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50 hover:opacity-90">
        <KpiCard label={t("dashboards.overview.kpi.overdue")} value={n(kpis.overdue_tasks)} className={kpis.overdue_tasks > 0 ? "border-amber-400/60" : undefined} />
      </button>
      <button type="button" onClick={() => onOpenTasks({ preset: "completed_week" })}
        className="rounded-xl text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50 hover:opacity-90">
        <KpiCard label={t("dashboards.overview.kpi.completed")} value={n(kpis.completed_this_week)} className="border-emerald-400/60" />
      </button>
      <KpiCard label={t("dashboards.overview.kpi.atRisk")} value={t("dashboards.overview.none")} hint={t("dashboards.overview.kpi.atRiskHint")} />
    </div>
    <div className="grid gap-6 lg:grid-cols-2">
      <section aria-labelledby="overview-health" className="space-y-3 rounded-xl border bg-card p-4">
        <h2 id="overview-health" className="text-sm font-semibold">{t("dashboards.overview.health.heading")}</h2>
        <div role="img" aria-label={HEALTH.map((key) => `${t(`dashboards.overview.health.${key}`)}: ${n(health[key] ?? 0)}`).join(", ")}
          className="flex h-2.5 overflow-hidden rounded-full bg-muted">
          {HEALTH.map((key) => (health[key] ?? 0) > 0 && <div key={key} className={BAR_TONE[key]}
            style={{ width: `${((health[key] ?? 0) / total_spaces) * 100}%` }} />)}
        </div>
        <ul className="grid grid-cols-3 gap-2 text-sm">
          {HEALTH.map((key) => <li key={key} className="rounded-lg bg-muted/50 px-3 py-2">
            <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <span aria-hidden className={`size-2 rounded-full ${BAR_TONE[key]}`} />{t(`dashboards.overview.health.${key}`)}</span>
            <span className="mt-0.5 block text-lg font-semibold tabular-nums">{n(health[key] ?? 0)}</span></li>)}
        </ul>
        <p className="text-xs text-muted-foreground">{t("dashboards.overview.health.rule")}</p>
      </section>
      <section aria-labelledby="overview-attention" className="space-y-3 rounded-xl border bg-card p-4">
        <h2 id="overview-attention" className="text-sm font-semibold">{t("dashboards.overview.attention.heading")}</h2>
        {attention.length === 0
          ? <p className="text-sm text-muted-foreground">{t("dashboards.overview.attention.none")}</p>
          : <ul className="space-y-2">{attention.map((space) => <li key={space.id} className="flex items-center justify-between gap-3 rounded-lg border px-3 py-2 text-sm">
              <Link to="/spaces/$spaceId" params={{ spaceId: space.id }} className="truncate font-medium underline-offset-2 hover:underline">{space.name}</Link>
              <button type="button" onClick={() => onOpenTasks({ preset: "overdue", spaceId: space.id })}
                className="shrink-0 rounded-full bg-amber-100 px-2 py-0.5 text-xs font-semibold tabular-nums text-amber-900 hover:bg-amber-200 focus-visible:ring-3 focus-visible:ring-ring/50 outline-none dark:bg-amber-500/20 dark:text-amber-200">
                {t("dashboards.overview.attention.overdue", { value: n(space.overdue_count) })}</button></li>)}</ul>}
        {total > spaces.length && <p className="text-xs text-muted-foreground">{t("dashboards.overview.partial", { shown: spaces.length, total })}</p>}
      </section>
    </div>
    <section aria-labelledby="overview-milestones" className="space-y-3 rounded-xl border bg-card p-4">
      <div className="flex items-baseline justify-between gap-2">
        <h2 id="overview-milestones" className="text-sm font-semibold">{t("dashboards.overview.milestones.heading")}</h2>
        <span className="text-xs text-muted-foreground">{t("dashboards.overview.milestones.range", { days: AGENDA_DAYS })}</span>
      </div>
      {upcoming.length === 0
        ? <p className="text-sm text-muted-foreground">{t("dashboards.overview.milestones.none")}</p>
        : <ul className="divide-y text-sm">{upcoming.map((item) => <li key={`${item.id}-${item.space_id}`} className="flex items-center justify-between gap-3 py-2">
            <span className="min-w-0 truncate"><span className="font-medium">{item.title}</span>
              <span className="text-muted-foreground"> · {item.space_name}</span></span>
            <span className={cn("shrink-0 tabular-nums text-xs", item.overdue ? "text-destructive" : "text-muted-foreground")}>{formatDate(item.due_date, profile)}</span>
          </li>)}</ul>}
    </section>
    <section aria-labelledby="overview-load" className="space-y-3 rounded-xl border bg-card p-4">
      <div className="flex items-baseline justify-between gap-2">
        <h2 id="overview-load" className="text-sm font-semibold">{t("dashboards.overview.load.heading")}</h2>
        <span className="text-xs text-muted-foreground">{t("dashboards.overview.load.range", { days: TIMELINE_DAYS })}</span>
      </div>
      {loadMap.length === 0
        ? <p className="text-sm text-muted-foreground">{t("dashboards.overview.timeline.empty")}</p>
        : <ul className="space-y-2">{loadMap.map((row) => <li key={row.id} className="grid grid-cols-[minmax(0,10rem)_1fr_auto] items-center gap-3 text-sm">
            <span className="truncate">{row.name}</span>
            <span className="h-2 overflow-hidden rounded-full bg-muted"><span className="block h-full rounded-full bg-primary" style={{ width: `${(row.count / peak) * 100}%` }} /></span>
            <span className="tabular-nums text-xs text-muted-foreground">{n(row.count)}</span>
          </li>)}</ul>}
    </section>
  </div>;
}

const STATUS_FILTERS = ["all", "attention", "on_track", "empty"] as const;

/** One row per Space: icon, name, counts, overdue, progress, last activity and health. Search and filters narrow the loaded rows. */
function SpaceTable({ onOpenTasks }: { onOpenTasks: (seed: TasksSeed) => void }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const [sort, setSort] = useState<OverviewSort>("overdue");
  const [text, setText] = useState("");
  const [health, setHealth] = useState<(typeof STATUS_FILTERS)[number]>("all");
  const [spaceId, setSpaceId] = useState("");
  const q = useOverviewPages(sort);
  const rows: OverviewSpace[] = useMemo(() => q.data?.pages.flatMap((page) => page.spaces) ?? [], [q.data]);
  const shown = useMemo(() => rows.filter((space) =>
    (!text || space.name.toLowerCase().includes(text.trim().toLowerCase())) &&
    (health === "all" || space.health === health) &&
    (!spaceId || space.id === spaceId)), [rows, text, health, spaceId]);
  if (q.isPending) return <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.loading")}</p>;
  if (q.isError) return <div><p role="alert">{t("dashboards.overview.error")}</p>
    <Button variant="outline" className="mt-3" onClick={() => q.refetch()}>{t("dashboards.overview.retry")}</Button></div>;
  const n = (value: number) => formatNumber(value, profile);
  return <div className="space-y-4">
    <div className="flex flex-wrap items-end gap-3">
      <label className="block text-sm">{t("dashboards.overview.search.label")}
        <span className="relative mt-1 block">
          <Search aria-hidden className="pointer-events-none absolute left-2 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <input type="search" value={text} onChange={(e) => setText(e.target.value)} placeholder={t("dashboards.overview.search.placeholder")}
            className="h-9 w-56 rounded-md border border-input bg-background pl-8 pr-2" /></span></label>
      <label className="block text-sm">{t("dashboards.overview.filter.health")}
        <select className={select} value={health} onChange={(e) => setHealth(e.target.value as (typeof STATUS_FILTERS)[number])}>
          {STATUS_FILTERS.map((key) => <option key={key} value={key}>{t(`dashboards.overview.filter.${key}`)}</option>)}
        </select></label>
      <label className="block text-sm">{t("dashboards.overview.filter.space")}
        <select className={select} value={spaceId} onChange={(e) => setSpaceId(e.target.value)}>
          <option value="">{t("dashboards.overview.agenda.allSpaces")}</option>
          {rows.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
        </select></label>
      <label className="block text-sm">{t("dashboards.overview.sort.label")}
        <select className={select} value={sort} onChange={(e) => setSort(e.target.value as OverviewSort)}>
          {SORTS.map((key) => <option key={key} value={key}>{t(`dashboards.overview.sort.${key}`)}</option>)}
        </select></label>
    </div>
    <div className="overflow-x-auto rounded-xl border bg-card">
      <table className="w-full text-left text-sm">
        <thead><tr className="border-b text-xs text-muted-foreground">
          <th scope="col" className="px-4 py-2 font-medium">{t("dashboards.overview.col.space")}</th>
          <th scope="col" className="px-4 py-2 text-right font-medium">{t("dashboards.overview.col.projects")}</th>
          <th scope="col" className="px-4 py-2 text-right font-medium">{t("dashboards.overview.col.tasks")}</th>
          <th scope="col" className="px-4 py-2 text-right font-medium">{t("dashboards.overview.col.overdue")}</th>
          <th scope="col" className="px-4 py-2 font-medium">{t("dashboards.overview.col.progress")}</th>
          <th scope="col" className="px-4 py-2 font-medium">{t("dashboards.overview.col.lastActivity")}</th>
          <th scope="col" className="px-4 py-2 font-medium">{t("dashboards.overview.col.health")}</th>
        </tr></thead>
        <tbody>{shown.map((space) => <tr key={space.id} className="border-b last:border-b-0 hover:bg-muted/40" title={t(`dashboards.overview.access.${space.effective_access}`)}>
          <th scope="row" className="px-4 py-3 text-left font-medium">
            <span className="flex items-center gap-2.5">
              <span aria-hidden className="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground"><Layers className="size-4" /></span>
              <Link to="/spaces/$spaceId" params={{ spaceId: space.id }} className="truncate underline-offset-2 hover:underline">{space.name}</Link>
              {space.archived && <span className="text-xs text-muted-foreground">{t("dashboards.overview.archived")}</span>}
            </span>
          </th>
          <td className="px-4 py-3 text-right tabular-nums">{n(space.project_count)}</td>
          <td className="px-4 py-3 text-right tabular-nums">{n(space.task_count)}</td>
          <td className="px-4 py-3 text-right">
            <span className={cn("inline-flex items-center gap-1.5 tabular-nums", space.overdue_count > 0 && "font-semibold text-amber-700 dark:text-amber-300")}>
              {space.overdue_count > 0 && <span aria-hidden className="size-1.5 rounded-full bg-amber-500" />}
              {space.overdue_count > 0
                ? <button type="button" onClick={() => onOpenTasks({ preset: "overdue", spaceId: space.id })} className="tabular-nums underline-offset-2 hover:underline">{n(space.overdue_count)}</button>
                : n(space.overdue_count)}</span></td>
          <td className="px-4 py-3">
            {space.progress_percent == null
              ? <span className="text-xs text-muted-foreground">{t("dashboards.overview.progress.none")}</span>
              : <div className="flex min-w-32 items-center gap-2">
                  <span className="h-1.5 w-24 overflow-hidden rounded-full bg-muted"><span className="block h-full rounded-full bg-primary" style={{ width: `${space.progress_percent}%` }} /></span>
                  <span className="tabular-nums text-xs">{t("dashboards.overview.percent", { value: space.progress_percent })}</span>
                  {space.progress_percent === 0 && space.overdue_count === 0 && <span className="text-xs text-muted-foreground">{t("dashboards.overview.progress.zeroNote")}</span>}
                </div>}
          </td>
          <td className="whitespace-nowrap px-4 py-3 text-muted-foreground">{space.last_activity ? formatDateTime(space.last_activity, profile) : t("dashboards.overview.none")}</td>
          <td className="px-4 py-3"><span className={cn("inline-flex rounded-full px-2 py-0.5 text-xs font-medium", HEALTH_TONE[space.health])}>{t(`dashboards.overview.health.${space.health}`)}</span></td>
        </tr>)}</tbody>
      </table>
      {shown.length === 0 && <p className="p-4 text-sm text-muted-foreground">{t("dashboards.overview.search.none")}</p>}
    </div>
    {q.hasNextPage && <Button variant="outline" onClick={() => void q.fetchNextPage()} disabled={q.isFetchingNextPage}>{t("dashboards.overview.loadMore")}</Button>}
  </div>;
}
