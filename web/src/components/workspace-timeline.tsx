import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { holidayMap, loadWorkSchedule, workScheduleKey } from "@/lib/work-schedule";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronRight } from "lucide-react";
import { addDays, TIMELINE_DAYS, useOverviewTimeline, type TimelineNode, type TimelineResponse, type TimelineSpace } from "@/lib/overview";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { formatDate, useProfile } from "@/lib/profile";
import type { CurrentUser } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { PeriodNav } from "@/components/overview-period-nav";

const DAY_PX = 32;
const LABEL_PX = 260;
const INDENT_PX = 16;
const DAY_MS = 86_400_000;
const utc = (date: string) => Date.parse(`${date}T00:00:00Z`);
const todayKey = () => {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};
const isWeekend = (date: string) => [0, 6].includes(new Date(utc(date)).getUTCDay());
const dayNumber = (date: string) => new Date(utc(date)).getUTCDate();
const isContainer = (node: TimelineNode) => node.type === "folder" || node.type === "project" || node.children.length > 0;

const DAY_OPTIONS = [30, 60, 90] as const;

/** Every due date below a node (or Space), in any order. Used for the summary dots on a collapsed row. */
function dueDatesBelow(nodes: TimelineNode[]): string[] {
  const out: string[] = [];
  const visit = (list: TimelineNode[]) => list.forEach((node) => {
    if (node.due_date) out.push(node.due_date);
    visit(node.children);
  });
  visit(nodes);
  return out;
}

/** How many items in the Space are due in the range. Context-only ancestors are not counted. */
function countInRange(nodes: TimelineNode[]): number {
  return nodes.reduce((sum, node) => sum + (node.in_range ? 1 : 0) + countInRange(node.children), 0);
}

/** Column offset of the earliest item due in the range, or null when nothing is scheduled. */
function firstScheduledIndex(data: TimelineResponse): number | null {
  let best: number | null = null;
  const visit = (nodes: TimelineNode[]) => nodes.forEach((node) => {
    if (node.in_range && node.due_date) {
      const index = Math.round((utc(node.due_date) - utc(data.from)) / DAY_MS);
      if (index >= 0 && (best === null || index < best)) best = index;
    }
    visit(node.children);
  });
  data.spaces.forEach((space) => visit(space.nodes));
  return best;
}

/** Read-only Gantt-style grid: one column per day. Rows follow the real hierarchy, Space > Folder or Project > Task > Subtask. */
export function WorkspaceTimeline() {
  const { t } = useTranslation();
  const profile = useProfile();
  const [from, setFrom] = useState<string | null>(null);
  const [days, setDays] = useState<number>(TIMELINE_DAYS);
  const [openIds, setOpenIds] = useState<string[]>([]);
  const q = useOverviewTimeline(from, days);
  const schedule = useQuery({ queryKey: workScheduleKey, queryFn: loadWorkSchedule });
  const holidays = useMemo(() => holidayMap(schedule.data?.data), [schedule.data]);
  const regionRef = useRef<HTMLDivElement>(null);
  const jumpToFirst = () => {
    const index = q.data ? firstScheduledIndex(q.data) : null;
    if (index === null || !regionRef.current) return;
    // Keep two days of context to the left of the first item; the label column is part of the scrolled width.
    regionRef.current.scrollLeft = Math.max(0, LABEL_PX + index * DAY_PX - 2 * DAY_PX);
  };
  // Opening a range should show where its work is, not the first days of an empty stretch.
  useEffect(() => { jumpToFirst(); }, [q.data]);
  if (q.isPending) return <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.loading")}</p>;
  if (q.isError) return <div><p role="alert">{t("dashboards.overview.error")}</p>
    <Button variant="outline" className="mt-3" onClick={() => q.refetch()}>{t("dashboards.overview.retry")}</Button></div>;
  const data = q.data;
  const dates = Array.from({ length: days }, (_, i) => addDays(data.from, i));
  const toggle = (id: string) => setOpenIds((ids) => ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id]);
  const allOpen = data.spaces.length > 0 && data.spaces.every((space) => openIds.includes(space.id));
  const scheduled = data.spaces.reduce((sum, space) => sum + countInRange(space.nodes), 0);
  const ctx: Ctx = { from: data.from, dates, days, profile, holidays, isOpen: (id) => openIds.includes(id), toggle };
  return <div className="space-y-4">
    <div className="flex flex-wrap items-end justify-between gap-3">
      <PeriodNav start={data.from} end={data.to} days={days} onMove={setFrom} />
      <div className="flex flex-wrap items-center gap-2">
        <SegmentedControl aria-label={t("dashboards.overview.timeline.range")} value={String(days)} onChange={(v) => setDays(Number(v))}
          items={DAY_OPTIONS.map((n) => ({ value: String(n), label: t("dashboards.overview.timeline.days", { count: n }) }))} />
        {data.spaces.length > 0 && <Button variant="outline" size="sm"
          onClick={() => setOpenIds(allOpen ? [] : data.spaces.map((space) => space.id))}>
          {allOpen ? t("dashboards.overview.timeline.collapseAll") : t("dashboards.overview.timeline.expandAll")}</Button>}
      </div>
    </div>
    <div className="flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
      <span className="flex items-center gap-1.5"><span className="h-3 w-5 rounded-md bg-sky-500" />{t("dashboards.overview.timeline.legend.project")}</span>
      <span className="flex items-center gap-1.5"><span className="h-3 w-5 rounded-md bg-muted-foreground/50" />{t("dashboards.overview.timeline.legend.deferred")}</span>
      <span className="flex items-center gap-1.5"><span className="h-3 w-5 rounded-sm bg-primary" />{t("dashboards.overview.timeline.legend.task")}</span>
      {scheduled > 0 && <Button variant="outline" size="sm" onClick={jumpToFirst}>{t("dashboards.overview.timeline.jumpFirst")}</Button>}
      <span className="ml-auto">{t("dashboards.overview.timeline.scheduled", { count: scheduled })}</span>
    </div>
    {data.truncated && <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.timeline.truncated")}</p>}
    {data.spaces.length === 0 && <p className="text-sm text-muted-foreground">{t("dashboards.overview.timeline.empty")}</p>}
    {data.spaces.length > 0 && <div ref={regionRef} role="region" tabIndex={0} aria-label={t("dashboards.overview.timeline.region")}
      className="overflow-x-auto rounded-xl border bg-card outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
      <div style={{ minWidth: LABEL_PX + days * DAY_PX }}>
        <Header ctx={ctx} />
        {data.spaces.map((space) => <SpaceRows key={space.id} space={space} ctx={ctx} />)}
      </div>
    </div>}
  </div>;
}

type Ctx = { from: string; dates: string[]; days: number; profile?: CurrentUser; holidays: Map<string, string>; isOpen: (id: string) => boolean; toggle: (id: string) => void };

function Header({ ctx }: { ctx: Ctx }) {
  const { t } = useTranslation();
  const today = todayKey();
  const months: { label: string; span: number }[] = [];
  for (const date of ctx.dates) {
    const label = new Intl.DateTimeFormat(ctx.profile?.locale ?? "en", { month: "short", year: "numeric", timeZone: "UTC" }).format(new Date(utc(date)));
    const last = months[months.length - 1];
    if (last?.label === label) last.span++; else months.push({ label, span: 1 });
  }
  return <div className="border-b text-xs text-muted-foreground">
    <div className="flex">
      <div className="sticky left-0 z-10 shrink-0 border-r bg-card" style={{ width: LABEL_PX }} />
      {months.map((m, i) => <div key={i} className="shrink-0 border-l px-2 py-1 font-medium text-foreground" style={{ width: m.span * DAY_PX }}>{m.label}</div>)}
    </div>
    <div className="flex">
      <div className="sticky left-0 z-10 flex shrink-0 items-center border-r bg-card px-4 py-1.5 font-medium text-foreground" style={{ width: LABEL_PX }}>
        {t("dashboards.overview.timeline.item")}</div>
      {ctx.dates.map((date) => <div key={date} style={{ width: DAY_PX }} title={ctx.holidays.get(date)}
        className={cn("shrink-0 border-l py-1.5 text-center tabular-nums", isWeekend(date) && "bg-muted/40",
          ctx.holidays.has(date) && "bg-rose-100 font-semibold text-rose-700 dark:bg-rose-500/20 dark:text-rose-300", date === today && "bg-primary/10 font-semibold text-foreground")}>
        {dayNumber(date)}</div>)}
    </div>
  </div>;
}

/** One line of the grid: a sticky label on the left, the day track on the right. */
function Row({ ctx, label, tone, children }: { ctx: Ctx; label: ReactNode; tone?: string; children?: ReactNode }) {
  return <div className={cn("flex border-b last:border-b-0", tone)}>
    <div className="sticky left-0 z-10 flex shrink-0 items-center gap-2 border-r bg-card px-4 py-1.5" style={{ width: LABEL_PX }}>{label}</div>
    <div className="relative shrink-0" style={{
      width: ctx.days * DAY_PX, backgroundImage: "linear-gradient(to right, var(--border) 1px, transparent 1px)", backgroundSize: `${DAY_PX}px 100%`,
    }}>{children}</div>
  </div>;
}

function Marker({ ctx, date, className, style, title }: { ctx: Ctx; date: string; className: string; style: (left: number) => CSSProperties; title: string }) {
  const index = Math.round((utc(date) - utc(ctx.from)) / DAY_MS);
  if (index < 0 || index >= ctx.days) return null;
  return <div title={title} className={cn("absolute", className)} style={style(index * DAY_PX)} />;
}

/** Summary dots on a collapsed row: one per distinct due date underneath, so the row shows its work without expanding. */
function DueDots({ ctx, dates }: { ctx: Ctx; dates: string[] }) {
  return <>{[...new Set(dates)].map((date) => {
    const index = Math.round((utc(date) - utc(ctx.from)) / DAY_MS);
    if (index < 0 || index >= ctx.days) return null;
    return <div key={date} data-summary="" title={formatDate(date, ctx.profile)} className="absolute top-1/2 size-2 -translate-y-1/2 rounded-full bg-primary/60"
      style={{ left: index * DAY_PX + DAY_PX / 2 - 4 }} />;
  })}</>;
}

/** A Space is collapsed by default; its top-level items sit under one header, in their real hierarchy. */
function SpaceRows({ space, ctx }: { space: TimelineSpace; ctx: Ctx }) {
  const { t } = useTranslation();
  const open = ctx.isOpen(space.id);
  const Chevron = open ? ChevronDown : ChevronRight;
  const count = countInRange(space.nodes);
  return <section aria-label={space.name}>
    <Row ctx={ctx} tone="bg-muted/50" label={<>
      <button type="button" aria-expanded={open} onClick={() => ctx.toggle(space.id)} title={space.name}
        className="flex min-w-0 flex-1 items-center gap-1.5 rounded-sm text-left font-semibold outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
        <Chevron className="size-4 shrink-0" aria-hidden />
        <span className="min-w-0 truncate">{space.name}</span>
      </button>
      <span className="shrink-0 rounded-full bg-background px-2 py-0.5 text-xs tabular-nums text-muted-foreground">{count}</span>
      <Link to="/spaces/$spaceId" params={{ spaceId: space.id }} className="shrink-0 text-xs underline">{t("dashboards.overview.timeline.openSpace")}</Link></>}>
      {!open && <DueDots ctx={ctx} dates={dueDatesBelow(space.nodes)} />}
    </Row>
    {open && <>
      {space.nodes.length === 0 && <Row ctx={ctx} label={<span className="pl-6 text-xs text-muted-foreground">{t("dashboards.overview.timeline.nothingDue")}</span>} />}
      {space.nodes.map((node) => <NodeRows key={node.id} node={node} depth={0} ctx={ctx} />)}
    </>}
  </section>;
}

/** One item and, when expanded, everything below it. Context-only ancestors have no bar of their own. */
function NodeRows({ node, depth, ctx }: { node: TimelineNode; depth: number; ctx: Ctx }) {
  const { t } = useTranslation();
  const hasChildren = node.children.length > 0;
  const open = ctx.isOpen(node.id);
  const Chevron = open ? ChevronDown : ChevronRight;
  const container = isContainer(node);
  const label = <>
    {hasChildren
      ? <button type="button" aria-expanded={open} onClick={() => ctx.toggle(node.id)} title={node.title}
        className="flex min-w-0 flex-1 items-center gap-1.5 rounded-sm text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        style={{ paddingLeft: depth * INDENT_PX }}>
        <Chevron className="size-4 shrink-0" aria-hidden />
        <span className={cn("min-w-0 truncate", container ? "font-medium" : "text-sm", !node.in_range && "text-muted-foreground")}>{node.title}</span>
      </button>
      : <span title={node.title} className={cn("flex min-w-0 flex-1 items-center gap-1.5", container ? "font-medium" : "text-sm", !node.in_range && "text-muted-foreground")}
        style={{ paddingLeft: depth * INDENT_PX + (container ? 0 : 20) }}>
        <span className="min-w-0 truncate">{node.title}</span>
      </span>}
    {node.due_date && node.in_range && <span className="ml-auto shrink-0 text-xs text-muted-foreground">{formatDate(node.due_date, ctx.profile)}</span>}
    {!node.in_range && <span className="sr-only">{t("dashboards.overview.timeline.context")}</span>}
  </>;
  return <>
    <Row ctx={ctx} label={label}>
      {node.in_range && node.due_date && <Marker ctx={ctx} date={node.due_date}
        className={cn(container ? "top-1/2 h-4 -translate-y-1/2 rounded-md" : "top-1/2 h-3 -translate-y-1/2 rounded-sm",
          container ? (node.status_group === "deferred" ? "bg-muted-foreground/50" : "bg-sky-500") : "bg-primary")}
        title={node.title}
        style={(left) => ({ left: left + 4, width: DAY_PX - 8 })} />}
      {hasChildren && !open && <DueDots ctx={ctx} dates={dueDatesBelow(node.children)} />}
    </Row>
    {hasChildren && open && node.children.map((child) => <NodeRows key={child.id} node={child} depth={depth + 1} ctx={ctx} />)}
  </>;
}
