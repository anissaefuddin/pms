import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { holidayMap, loadWorkSchedule, workScheduleKey } from "@/lib/work-schedule";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronRight } from "lucide-react";
import { addDays, useOverviewTimeline, type TimelineNode, type TimelineResponse, type TimelineSpace } from "@/lib/overview";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { formatDate, useProfile } from "@/lib/profile";
import type { CurrentUser } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PeriodNav } from "@/components/overview-period-nav";

const LABEL_PX = 260;
const INDENT_PX = 16;
const DAY_MS = 86_400_000;
const MAX_DAYS = 400;
const utc = (date: string) => Date.parse(`${date}T00:00:00Z`);
const iso = (ms: number) => new Date(ms).toISOString().slice(0, 10);
const todayKey = () => {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};
const isWeekend = (date: string) => [0, 6].includes(new Date(utc(date)).getUTCDay());

/** Zoom, like the Gantt scale: pixels per day for each unit. */
const SCALES = ["day", "week", "month", "quarter", "year"] as const;
type Scale = (typeof SCALES)[number];
const PX_PER_DAY: Record<Scale, number> = { day: 32, week: 10, month: 3, quarter: 1.2, year: 0.9 };

/** Range presets: a number of days, or a calendar period that contains the anchor date. */
const RANGES = ["30", "60", "90", "month", "quarter", "year", "custom"] as const;
type RangeKind = (typeof RANGES)[number];

function periodOf(kind: RangeKind, anchor: string, customFrom: string, customTo: string): { from: string; days: number } {
  if (kind === "custom") {
    const span = Math.round((utc(customTo) - utc(customFrom)) / DAY_MS) + 1;
    return { from: customFrom, days: Math.min(MAX_DAYS, Math.max(1, span)) };
  }
  if (kind === "month" || kind === "quarter" || kind === "year") {
    const d = new Date(utc(anchor));
    const y = d.getUTCFullYear();
    const m = d.getUTCMonth();
    const startMonth = kind === "quarter" ? Math.floor(m / 3) * 3 : kind === "year" ? 0 : m;
    const months = kind === "quarter" ? 3 : kind === "year" ? 12 : 1;
    const from = iso(Date.UTC(y, startMonth, 1));
    const end = Date.UTC(y, startMonth + months, 1);
    return { from, days: Math.round((end - utc(from)) / DAY_MS) };
  }
  return { from: anchor, days: Number(kind) };
}

/** Column offset of the earliest item due or starting in the range, or null when nothing is scheduled. */
function firstScheduledIndex(data: TimelineResponse): number | null {
  let best: number | null = null;
  const visit = (nodes: TimelineNode[]) => nodes.forEach((node) => {
    const first = node.in_range ? node.start_date ?? node.due_date : null;
    if (first) {
      const index = Math.round((utc(first) - utc(data.from)) / DAY_MS);
      if (index >= 0 && (best === null || index < best)) best = index;
    }
    visit(node.children);
  });
  data.spaces.forEach((space) => visit(space.nodes));
  return best;
}

/** Start and end of every in-range item below a node, merged so overlapping spans become one block. */
function spansBelow(nodes: TimelineNode[]): [string, string][] {
  const spans: [string, string][] = [];
  const visit = (list: TimelineNode[]) => list.forEach((node) => {
    if (node.in_range) {
      const end = node.due_date ?? node.start_date;
      const start = node.start_date ?? node.due_date;
      if (start && end) spans.push(start <= end ? [start, end] : [end, start]);
    }
    visit(node.children);
  });
  visit(nodes);
  spans.sort((a, b) => (a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0));
  const merged: [string, string][] = [];
  for (const span of spans) {
    const last = merged[merged.length - 1];
    // Overlapping or touching spans (the next day counts as touching) become one block.
    if (last && utc(span[0]) <= utc(last[1]) + DAY_MS) last[1] = span[1] > last[1] ? span[1] : last[1];
    else merged.push([span[0], span[1]]);
  }
  return merged;
}

/** Items in the range within one Space's tree; context-only ancestors are not counted. */
function countInRange(nodes: TimelineNode[]): number {
  return nodes.reduce((sum, node) => sum + (node.in_range ? 1 : 0) + countInRange(node.children), 0);
}

/** Read-only Gantt-style grid. Rows follow the real hierarchy, Space > Folder or Project > Task > Subtask. */
export function WorkspaceTimeline() {
  const { t } = useTranslation();
  const profile = useProfile();
  const [scale, setScale] = useState<Scale>("day");
  const [rangeKind, setRangeKind] = useState<RangeKind>("60");
  const [anchor, setAnchor] = useState<string | null>(null);
  const [customFrom, setCustomFrom] = useState(todayKey());
  const [customTo, setCustomTo] = useState(addDays(todayKey(), 59));
  const [openIds, setOpenIds] = useState<string[]>([]);
  const range = periodOf(rangeKind, anchor ?? todayKey(), customFrom, customTo);
  const q = useOverviewTimeline(range.from, range.days);
  const schedule = useQuery({ queryKey: workScheduleKey, queryFn: loadWorkSchedule });
  const holidays = useMemo(() => holidayMap(schedule.data?.data), [schedule.data]);
  const regionRef = useRef<HTMLDivElement>(null);
  const px = PX_PER_DAY[scale];
  const jumpToFirst = () => {
    const index = q.data ? firstScheduledIndex(q.data) : null;
    if (index === null || !regionRef.current) return;
    // Keep two days of context to the left of the first item; the label column is part of the scrolled width.
    regionRef.current.scrollLeft = Math.max(0, LABEL_PX + index * px - 2 * px);
  };
  // Opening a range should show where its work is, not the first days of an empty stretch.
  useEffect(() => { jumpToFirst(); }, [q.data]);
  if (q.isPending) return <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.loading")}</p>;
  if (q.isError) return <div><p role="alert">{t("dashboards.overview.error")}</p>
    <Button variant="outline" className="mt-3" onClick={() => q.refetch()}>{t("dashboards.overview.retry")}</Button></div>;
  const data = q.data;
  const days = range.days;
  const dates = Array.from({ length: days }, (_, i) => addDays(data.from, i));
  const toggle = (id: string) => setOpenIds((ids) => ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id]);
  const allOpen = data.spaces.length > 0 && data.spaces.every((space) => openIds.includes(space.id));
  const scheduled = data.spaces.reduce((sum, space) => sum + countInRange(space.nodes), 0);
  const ctx: Ctx = { from: data.from, to: data.to, dates, days, px, scale, profile, holidays, isOpen: (id) => openIds.includes(id), toggle };
  // Moving by the period nav: calendar ranges move their anchor, custom ranges keep their length.
  const move = (next: string | null) => {
    if (rangeKind === "custom") {
      if (next) { setCustomFrom(next); setCustomTo(addDays(next, range.days - 1)); } else { setCustomFrom(todayKey()); setCustomTo(addDays(todayKey(), range.days - 1)); }
      return;
    }
    setAnchor(next);
  };
  return <div className="space-y-4">
    <div className="flex flex-wrap items-end justify-between gap-3">
      <PeriodNav start={data.from} end={data.to} days={days} onMove={move} />
      <div className="flex max-w-full flex-wrap items-center gap-2 overflow-x-auto">
        <SegmentedControl aria-label={t("dashboards.overview.timeline.scale")} value={scale} onChange={(v) => setScale(v as Scale)}
          items={SCALES.map((value) => ({ value, label: t(`dashboards.overview.timeline.scales.${value}`) }))} />
        <SegmentedControl aria-label={t("dashboards.overview.timeline.range")} value={rangeKind} onChange={(v) => { setRangeKind(v as RangeKind); setAnchor(null); }}
          items={RANGES.map((value) => ({ value, label: t(`dashboards.overview.timeline.ranges.${value}`) }))} />
        {data.spaces.length > 0 && <Button variant="outline" size="sm"
          onClick={() => setOpenIds(allOpen ? [] : data.spaces.map((space) => space.id))}>
          {allOpen ? t("dashboards.overview.timeline.collapseAll") : t("dashboards.overview.timeline.expandAll")}</Button>}
      </div>
    </div>
    {rangeKind === "custom" && <div className="flex flex-wrap items-end gap-3 text-sm">
      <label className="space-y-1"><span className="block text-xs font-medium text-muted-foreground">{t("dashboards.overview.timeline.customFrom")}</span>
        <Input type="date" value={customFrom} onChange={(event) => { setCustomFrom(event.target.value); if (event.target.value && customTo < event.target.value) setCustomTo(event.target.value); }} /></label>
      <label className="space-y-1"><span className="block text-xs font-medium text-muted-foreground">{t("dashboards.overview.timeline.customTo")}</span>
        <Input type="date" min={customFrom} value={customTo} onChange={(event) => setCustomTo(event.target.value)} /></label>
      <span className="pb-2 text-xs text-muted-foreground">{t("dashboards.overview.timeline.customLimit", { count: MAX_DAYS })}</span>
    </div>}
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
      <div className="relative" style={{ minWidth: LABEL_PX + days * px }}>
        <Overlay ctx={ctx} />
        <Header ctx={ctx} />
        {data.spaces.map((space) => <SpaceRows key={space.id} space={space} ctx={ctx} />)}
      </div>
    </div>}
  </div>;
}

type Ctx = {
  from: string; to: string; dates: string[]; days: number; px: number; scale: Scale; profile?: CurrentUser;
  holidays: Map<string, string>; isOpen: (id: string) => boolean; toggle: (id: string) => void;
};

/** Index of a date inside the range (may be outside it). */
const indexOf = (ctx: Ctx, date: string) => Math.round((utc(date) - utc(ctx.from)) / DAY_MS);

/** Column tints behind the rows: weekends (day and week scales), holidays, and today. */
function Overlay({ ctx }: { ctx: Ctx }) {
  const today = todayKey();
  return <div aria-hidden className="pointer-events-none absolute inset-y-0 z-0" style={{ left: LABEL_PX, width: ctx.days * ctx.px }}>
    {ctx.dates.map((date, i) => {
      const weekend = (ctx.scale === "day" || ctx.scale === "week") && isWeekend(date);
      const holiday = ctx.holidays.has(date);
      const isToday = date === today;
      if (!weekend && !holiday && !isToday) return null;
      return <div key={date} className={cn("absolute inset-y-0", holiday ? "bg-rose-100/70 dark:bg-rose-500/15" : weekend ? "bg-muted/50" : "", isToday && "bg-primary/10")}
        style={{ left: i * ctx.px, width: ctx.px }} title={ctx.holidays.get(date)} />;
    })}
  </div>;
}

/** Top row: months (day and week scales) or years (coarser scales). Bottom row: one cell per day, week, month, quarter or year. */
function Header({ ctx }: { ctx: Ctx }) {
  const { t } = useTranslation();
  const locale = ctx.profile?.locale ?? "en";
  const top = groupBy(ctx.dates, (date) => ctx.scale === "day" || ctx.scale === "week"
    ? new Intl.DateTimeFormat(locale, { month: "short", year: "numeric", timeZone: "UTC" }).format(new Date(utc(date)))
    : String(new Date(utc(date)).getUTCFullYear()));
  const bottom = bottomCells(ctx, locale);
  return <div className="border-b text-xs text-muted-foreground">
    <div className="flex">
      <div className="sticky left-0 z-20 shrink-0 border-r bg-card" style={{ width: LABEL_PX }} />
      {top.map((m, i) => <div key={i} className="shrink-0 border-l px-2 py-1 font-medium text-foreground" style={{ width: m.span * ctx.px }}>{m.label}</div>)}
    </div>
    <div className="flex">
      <div className="sticky left-0 z-20 flex shrink-0 items-center border-r bg-card px-4 py-1.5 font-medium text-foreground" style={{ width: LABEL_PX }}>
        {t("dashboards.overview.timeline.item")}</div>
      {bottom.map((cell) => <div key={cell.key} style={{ width: cell.span * ctx.px }} title={cell.title}
        className={cn("shrink-0 truncate border-l py-1.5 text-center tabular-nums", cell.weekend && "bg-muted/40",
          cell.holiday && "bg-rose-100 font-semibold text-rose-700 dark:bg-rose-500/20 dark:text-rose-300", cell.today && "bg-primary/10 font-semibold text-foreground")}>
        {cell.label}</div>)}
    </div>
  </div>;
}

type Cell = { key: string; label: string; span: number; title?: string; weekend?: boolean; holiday?: boolean; today?: boolean };

/** Bottom header cells for the chosen scale. Day and week cells keep their dates; coarser cells are whole periods. */
function bottomCells(ctx: Ctx, locale: string): Cell[] {
  const today = todayKey();
  const dayLabel = new Intl.DateTimeFormat(locale, { day: "numeric", timeZone: "UTC" });
  const shortDate = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", timeZone: "UTC" });
  const monthName = new Intl.DateTimeFormat(locale, { month: "short", timeZone: "UTC" });
  if (ctx.scale === "day") {
    return ctx.dates.map((date) => ({ key: date, label: dayLabel.format(new Date(utc(date))), span: 1, title: ctx.holidays.get(date),
      weekend: isWeekend(date), holiday: ctx.holidays.has(date), today: date === today }));
  }
  const keyOf: Record<Exclude<Scale, "day">, (date: string) => string> = {
    week: (date) => String(Math.floor(indexOf(ctx, date) / 7)),
    month: (date) => date.slice(0, 7),
    quarter: (date) => `${date.slice(0, 4)}-Q${Math.floor((Number(date.slice(5, 7)) - 1) / 3)}`,
    year: (date) => date.slice(0, 4),
  };
  const labelOf = (scale: Exclude<Scale, "day">, first: string) => {
    if (scale === "week") return shortDate.format(new Date(utc(first)));
    if (scale === "month") return monthName.format(new Date(utc(first)));
    if (scale === "quarter") return `Q${Math.floor((Number(first.slice(5, 7)) - 1) / 3) + 1} ${first.slice(0, 4)}`;
    return first.slice(0, 4);
  };
  return groupBy(ctx.dates, keyOf[ctx.scale]).map((group) => ({
    key: group.key, label: labelOf(ctx.scale as Exclude<Scale, "day">, group.first), span: group.span,
    title: group.first, weekend: false, holiday: false, today: group.dates.includes(today),
  }));
}

/** Consecutive dates that share a key, with the first date and the span in days. */
function groupBy(dates: string[], keyOf: (date: string) => string) {
  const out: { key: string; label: string; first: string; span: number; dates: string[] }[] = [];
  for (const date of dates) {
    const key = keyOf(date);
    const last = out[out.length - 1];
    if (last && last.key === key) { last.span++; last.dates.push(date); } else out.push({ key, label: key, first: date, span: 1, dates: [date] });
  }
  return out.map((group) => ({ ...group, label: group.label }));
}

/** One line of the grid: a sticky label on the left, the day track on the right. */
function Row({ ctx, label, tone, children }: { ctx: Ctx; label: ReactNode; tone?: string; children?: ReactNode }) {
  return <div className={cn("relative flex border-b last:border-b-0", tone)}>
    <div className="sticky left-0 z-20 flex shrink-0 items-center gap-2 border-r bg-card px-4 py-1.5" style={{ width: LABEL_PX }}>{label}</div>
    <div className="relative shrink-0 overflow-hidden" style={{ width: ctx.days * ctx.px }}>{children}</div>
  </div>;
}

/** A bar from the start index to the end index (inclusive), clipped to the range. */
function Bar({ ctx, start, end, className, title, ...rest }: { ctx: Ctx; start: number; end: number; className: string; title: string } & Record<`data-${string}`, string>) {
  const first = Math.max(0, start);
  const last = Math.min(ctx.days - 1, end);
  if (last < first) return null;
  const width = (last - first + 1) * ctx.px;
  return <div title={title} className={cn("absolute z-10", className)} style={{ left: first * ctx.px + 2, width: Math.max(4, width - 4) }} {...rest} />;
}

/** Blocks on a collapsed row: the merged start-to-end spans of everything below it, so the range shows without expanding. */
function SpanBlocks({ ctx, spans }: { ctx: Ctx; spans: [string, string][] }) {
  return <>{spans.map(([start, end]) => {
    const first = indexOf(ctx, start);
    const last = indexOf(ctx, end);
    return <Bar key={`${start}|${end}`} ctx={ctx} start={first} end={last} className="top-1/2 h-3 -translate-y-1/2 rounded-sm bg-primary/40 ring-1 ring-primary/50"
      title={`${formatDate(start, ctx.profile)} – ${formatDate(end, ctx.profile)}`} data-span="" />;
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
      {!open && <SpanBlocks ctx={ctx} spans={spansBelow(space.nodes)} />}
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
  const container = node.type === "folder" || node.type === "project" || hasChildren;
  const text = cn("min-w-0 truncate", container ? "font-medium" : "text-sm", !node.in_range && "text-muted-foreground");
  const label = <>
    {hasChildren
      ? <button type="button" aria-expanded={open} onClick={() => ctx.toggle(node.id)} title={node.title}
        className="flex min-w-0 flex-1 items-center gap-1.5 rounded-sm text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        style={{ paddingLeft: depth * INDENT_PX }}>
        <Chevron className="size-4 shrink-0" aria-hidden />
        <span className={text}>{node.title}</span>
      </button>
      : <span title={node.title} className="flex min-w-0 flex-1 items-center gap-1.5" style={{ paddingLeft: depth * INDENT_PX + (container ? 0 : 20) }}>
        <span className={text}>{node.title}</span>
      </span>}
    {node.due_date && node.in_range && <span className="ml-auto shrink-0 text-xs text-muted-foreground">{formatDate(node.due_date, ctx.profile)}</span>}
    {!node.in_range && <span className="sr-only">{t("dashboards.overview.timeline.context")}</span>}
  </>;
  const barStart = node.start_date ? indexOf(ctx, node.start_date) : node.due_date ? indexOf(ctx, node.due_date) : null;
  const barEnd = node.due_date ? indexOf(ctx, node.due_date) : barStart;
  const barClass = container
    ? cn("top-1/2 h-4 -translate-y-1/2 rounded-md", node.status_group === "deferred" ? "bg-muted-foreground/50" : "bg-sky-500")
    : "top-1/2 h-3 -translate-y-1/2 rounded-sm bg-primary";
  return <>
    <Row ctx={ctx} label={label}>
      {node.in_range && barStart !== null && barEnd !== null && <Bar ctx={ctx} start={barStart} end={barEnd} className={barClass} title={node.title} />}
      {hasChildren && !open && <SpanBlocks ctx={ctx} spans={spansBelow(node.children)} />}
    </Row>
    {hasChildren && open && node.children.map((child) => <NodeRows key={child.id} node={child} depth={depth + 1} ctx={ctx} />)}
  </>;
}

