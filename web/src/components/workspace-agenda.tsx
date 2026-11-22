import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { CalendarX, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { AGENDA_DAYS, addDays, useOverviewAgenda, type AgendaItem, type AgendaKind } from "@/lib/overview";
import { formatDate, formatNumber, useProfile } from "@/lib/profile";
import type { CurrentUser } from "@/lib/api";
import { useSpaces } from "@/lib/space-queries";
import { holidayMap, loadWorkSchedule, workScheduleKey } from "@/lib/work-schedule";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { PeriodNav } from "@/components/overview-period-nav";

const select = "mt-1 block h-9 rounded-md border border-input bg-background px-2";
const KINDS = ["all", "project", "task"] as const;
const RANGES = [14, 30] as const;
const DAY_CHIPS = 3;

const todayKey = () => {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};

/** Same item can come back once per location; one entry per item and day is shown. */
function uniqueItems(items: AgendaItem[]): AgendaItem[] {
  const seen = new Set<string>();
  return items.filter((item) => {
    const key = `${item.id}|${item.due_date}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

/** Due Projects and Tasks as a calendar: one box per day, three items shown and the rest behind +N. Range is 14 days or one month. */
export function WorkspaceAgenda() {
  const { t } = useTranslation();
  const profile = useProfile();
  const spaces = (useSpaces().data ?? []).filter((space) => space.kind === "shared");
  const [from, setFrom] = useState<string | null>(null);
  const [spaceId, setSpaceId] = useState("");
  const [kind, setKind] = useState<(typeof KINDS)[number]>("all");
  const [days, setDays] = useState<number>(AGENDA_DAYS);
  const [openDay, setOpenDay] = useState<string | null>(null);
  const q = useOverviewAgenda(from, spaceId, kind === "all" ? "" : kind as AgendaKind, days);
  const schedule = useQuery({ queryKey: workScheduleKey, queryFn: loadWorkSchedule });
  const holidays = useMemo(() => holidayMap(schedule.data?.data), [schedule.data]);
  const working = new Set(schedule.data?.data.working_days ?? [1, 2, 3, 4, 5]);
  const items = useMemo(() => uniqueItems(q.data?.items ?? []), [q.data]);
  if (q.isPending) return <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.loading")}</p>;
  if (q.isError) return <div><p role="alert">{t("dashboards.overview.error")}</p>
    <Button variant="outline" className="mt-3" onClick={() => q.refetch()}>{t("dashboards.overview.retry")}</Button></div>;
  const data = q.data;
  const dates = Array.from({ length: days }, (_, i) => addDays(data.from, i));
  const byDate = new Map<string, AgendaItem[]>();
  for (const item of items) byDate.set(item.due_date, [...(byDate.get(item.due_date) ?? []), item]);
  const n = (value: number) => formatNumber(value, profile);
  return <div className="space-y-4">
    <div className="flex flex-wrap items-end justify-between gap-3">
      <PeriodNav start={data.from} end={data.to} days={days} onMove={setFrom} />
      <div className="flex flex-wrap items-end gap-3">
        <label className="block text-sm">{t("dashboards.overview.agenda.space")}
          <select className={select} value={spaceId} onChange={(e) => setSpaceId(e.target.value)}>
            <option value="">{t("dashboards.overview.agenda.allSpaces")}</option>
            {spaces.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
          </select></label>
        <SegmentedControl aria-label={t("dashboards.overview.agenda.kind")} value={kind} onChange={(v) => setKind(v as (typeof KINDS)[number])}
          items={KINDS.map((key) => ({ value: key, label: t(`dashboards.overview.agenda.kinds.${key}`) }))} />
        <SegmentedControl aria-label={t("dashboards.overview.agenda.range")} value={String(days)} onChange={(v) => setDays(Number(v))}
          items={RANGES.map((n) => ({ value: String(n), label: t(n === 30 ? "dashboards.overview.agenda.month" : "dashboards.overview.agenda.days", { count: n }) }))} />
      </div>
    </div>
    {data.overdue_count > 0 && <p role="status" className="text-sm text-destructive">
      {t("dashboards.overview.agenda.overdueCount", { count: n(data.overdue_count) })}</p>}
    {data.truncated && <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.agenda.truncated")}</p>}
    {byDate.size === 0 && <EmptyState icon={<CalendarX />} title={t("dashboards.overview.agenda.empty.title")}
      description={t("dashboards.overview.agenda.empty.description", { count: days })} canCreate={false} />}
    {byDate.size > 0 && <label className="block max-w-xs text-sm">{t("dashboards.overview.agenda.day")}
      <select className={select} value={openDay ?? ""} onChange={(e) => setOpenDay(e.target.value || null)}>
        <option value="">{t("dashboards.overview.agenda.pickDay")}</option>
        {dates.map((date) => <option key={date} value={date}>{formatDate(date, profile)}{byDate.has(date) ? ` (${byDate.get(date)!.length})` : ""}</option>)}
      </select></label>}
    {byDate.size > 0 && <div className="grid gap-2 sm:grid-cols-2 md:grid-cols-7">
      {dates.map((date) => <DayBox key={date} date={date} items={byDate.get(date) ?? []} profile={profile} holiday={holidays.get(date)}
        off={!working.has(new Date(`${date}T00:00:00Z`).getUTCDay())} onOpen={() => setOpenDay(date)} />)}
    </div>}
    {openDay !== null && <SlidePanel title={formatDate(openDay, profile)} subtitle={holidays.get(openDay)} onClose={() => setOpenDay(null)}>
      {(byDate.get(openDay) ?? []).length === 0
        ? <EmptyState icon={<CalendarX />} title={t("dashboards.overview.agenda.dayEmpty.title")}
            description={t("dashboards.overview.agenda.dayEmpty.description")} canCreate={false} />
        : <ul className="divide-y">{(byDate.get(openDay) ?? []).map((item) => <li key={`${item.id}-${item.space_id}`} className="py-3 text-sm">
            <p className="font-medium">{item.title}</p>
            <p className="mt-0.5 flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">
              <Link to="/spaces/$spaceId" params={{ spaceId: item.space_id }} className="underline">{item.space_name}</Link>
              <span>{t(`dashboards.overview.type.${item.type}`)}</span>
              {item.overdue && <span className="rounded bg-destructive/10 px-1.5 py-0.5 font-medium text-destructive">{t("dashboards.overview.agenda.overdue")}</span>}
            </p>
          </li>)}</ul>}
    </SlidePanel>}
  </div>;
}

/** One calendar box per day: weekday and date, then up to three items. The rest open in a drawer. */
function DayBox({ date, items, profile, holiday, off, onOpen }: { date: string; items: AgendaItem[]; profile?: CurrentUser; holiday?: string; off: boolean; onOpen: () => void }) {
  const { t } = useTranslation();
  const weekday = new Intl.DateTimeFormat(profile?.locale ?? "en", { weekday: "short", timeZone: "UTC" }).format(new Date(`${date}T00:00:00Z`));
  const extra = items.length - DAY_CHIPS;
  return <div className={cn("flex min-h-28 flex-col gap-1.5 rounded-lg border p-2",
    holiday ? "border-rose-300 bg-rose-50 dark:border-rose-500/40 dark:bg-rose-500/10" : off ? "bg-muted/40" : "bg-card",
    date === todayKey() && "border-primary/60 ring-1 ring-primary/30")} title={holiday}>
    <div className="flex items-baseline justify-between gap-2">
      <h2 className="text-xs text-muted-foreground"><span className={cn(holiday && "font-semibold text-rose-700 dark:text-rose-300")}>{weekday}</span> <span className="sr-only">{formatDate(date, profile)}</span></h2>
      <span className={cn("text-sm font-semibold tabular-nums", holiday && "text-rose-700 dark:text-rose-300")} aria-hidden>{Number(date.slice(8))}</span>
    </div>
    {holiday && <p className="truncate text-xs font-medium text-rose-700 dark:text-rose-300">{holiday}</p>}
    {items.slice(0, DAY_CHIPS).map((item) => <Link key={`${item.id}-${item.space_id}`} to="/spaces/$spaceId" params={{ spaceId: item.space_id }}
      title={`${item.title} · ${item.space_name} · ${t(`dashboards.overview.type.${item.type}`)}`}
      className={cn("block rounded-md border-l-2 px-1.5 py-1 text-xs outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50",
        item.overdue ? "border-destructive bg-destructive/10" : item.type === "project" ? "border-sky-500 bg-sky-500/10" : "border-primary bg-primary/5")}>
      <span className="block truncate font-medium">{item.title}</span>
      <span className="block truncate text-xs text-foreground/80">{item.space_name}</span>
    </Link>)}
    {extra > 0 && <Button variant="ghost" size="sm" className="h-6 justify-start px-1.5 text-xs" onClick={onOpen}>
      {t("dashboards.overview.agenda.more", { count: extra })}</Button>}
  </div>;
}

/** Right-hand panel for one day. Escape or the backdrop closes it. */
function SlidePanel({ title, subtitle, onClose, children }: { title: string; subtitle?: string; onClose: () => void; children: ReactNode }) {
  const { t } = useTranslation();
  const asideRef = useRef<HTMLElement>(null);
  useEffect(() => {
    asideRef.current?.focus();
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);
  return <div className="fixed inset-0 z-50">
    <button type="button" tabIndex={-1} aria-label={t("dashboards.overview.agenda.close")} className="absolute inset-0 bg-black/30" onClick={onClose} />
    <aside ref={asideRef} tabIndex={-1} role="dialog" aria-modal="true" aria-label={title}
      className="absolute inset-y-0 right-0 flex w-full max-w-md flex-col border-l bg-card shadow-xl outline-none">
      <header className="flex items-center justify-between gap-3 border-b px-5 py-4">
        <div className="min-w-0"><h2 className="text-base font-semibold">{title}</h2>{subtitle && <p className="truncate text-xs font-medium text-rose-700 dark:text-rose-300">{subtitle}</p>}</div>
        <button type="button" onClick={onClose} aria-label={t("dashboards.overview.agenda.close")}
          className="inline-flex size-8 items-center justify-center rounded-md hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50 outline-none"><X className="size-4" /></button>
      </header>
      <div className="flex-1 overflow-y-auto p-5">{children}</div>
    </aside>
  </div>;
}

