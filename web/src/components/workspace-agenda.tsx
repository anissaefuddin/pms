import { useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { AGENDA_DAYS, addDays, useOverviewAgenda, type AgendaItem, type AgendaKind } from "@/lib/overview";
import { formatDate, formatNumber, useProfile } from "@/lib/profile";
import type { CurrentUser } from "@/lib/api";
import { useSpaces } from "@/lib/space-queries";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { PeriodNav } from "@/components/overview-period-nav";

const select = "mt-1 block h-9 rounded-md border border-input bg-background px-2";
const KINDS = ["all", "project", "task"] as const;
const MODES = ["list", "calendar"] as const;
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

/** Due Projects and Tasks for the next two weeks. The list is the default; the calendar shows three items per day with the rest behind +N. */
export function WorkspaceAgenda() {
  const { t } = useTranslation();
  const profile = useProfile();
  const spaces = (useSpaces().data ?? []).filter((space) => space.kind === "shared");
  const [from, setFrom] = useState<string | null>(null);
  const [spaceId, setSpaceId] = useState("");
  const [kind, setKind] = useState<(typeof KINDS)[number]>("all");
  const [mode, setMode] = useState<(typeof MODES)[number]>("list");
  const [openDay, setOpenDay] = useState<string | null>(null);
  const q = useOverviewAgenda(from, spaceId, kind === "all" ? "" : kind as AgendaKind);
  const items = useMemo(() => uniqueItems(q.data?.items ?? []), [q.data]);
  if (q.isPending) return <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.loading")}</p>;
  if (q.isError) return <div><p role="alert">{t("dashboards.overview.error")}</p>
    <Button variant="outline" className="mt-3" onClick={() => q.refetch()}>{t("dashboards.overview.retry")}</Button></div>;
  const data = q.data;
  const dates = Array.from({ length: AGENDA_DAYS }, (_, i) => addDays(data.from, i));
  const byDate = new Map<string, AgendaItem[]>();
  for (const item of items) byDate.set(item.due_date, [...(byDate.get(item.due_date) ?? []), item]);
  const n = (value: number) => formatNumber(value, profile);
  return <div className="space-y-4">
    <div className="flex flex-wrap items-end justify-between gap-3">
      <PeriodNav start={data.from} end={data.to} days={AGENDA_DAYS} onMove={setFrom} />
      <div className="flex flex-wrap items-end gap-3">
        <label className="block text-sm">{t("dashboards.overview.agenda.space")}
          <select className={select} value={spaceId} onChange={(e) => setSpaceId(e.target.value)}>
            <option value="">{t("dashboards.overview.agenda.allSpaces")}</option>
            {spaces.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
          </select></label>
        <SegmentedControl aria-label={t("dashboards.overview.agenda.kind")} value={kind} onChange={(v) => setKind(v as (typeof KINDS)[number])}
          items={KINDS.map((key) => ({ value: key, label: t(`dashboards.overview.agenda.kinds.${key}`) }))} />
        <SegmentedControl aria-label={t("dashboards.overview.agenda.mode")} value={mode} onChange={(v) => setMode(v as (typeof MODES)[number])}
          items={MODES.map((key) => ({ value: key, label: t(`dashboards.overview.agenda.modes.${key}`) }))} />
      </div>
    </div>
    {data.overdue_count > 0 && <p role="status" className="text-sm text-destructive">
      {t("dashboards.overview.agenda.overdueCount", { count: n(data.overdue_count) })}</p>}
    {data.truncated && <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.agenda.truncated")}</p>}
    {byDate.size === 0 && <p className="text-sm text-muted-foreground">{t("dashboards.overview.agenda.empty")}</p>}
    {byDate.size > 0 && mode === "list" && <div className="space-y-4">
      {dates.filter((date) => byDate.has(date)).map((date) => <section key={date} aria-label={formatDate(date, profile)} className="space-y-1.5">
        <h2 className="flex items-baseline gap-2 text-sm font-semibold">{formatDate(date, profile)}
          <span className="text-xs font-normal text-muted-foreground">{t("dashboards.overview.agenda.count", { count: byDate.get(date)!.length })}</span></h2>
        <ul className="divide-y rounded-lg border bg-card">
          {byDate.get(date)!.map((item) => <li key={`${item.id}-${item.space_id}`} className="flex items-center justify-between gap-3 px-4 py-2 text-sm">
            <div className="min-w-0">
              <p className="truncate font-medium">{item.title}</p>
              <p className="flex flex-wrap gap-x-2 text-xs text-muted-foreground">
                <Link to="/spaces/$spaceId" params={{ spaceId: item.space_id }} className="underline">{item.space_name}</Link>
                <span>{t(`dashboards.overview.type.${item.type}`)}</span>
              </p>
            </div>
            {item.overdue && <span className="whitespace-nowrap rounded bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive">
              {t("dashboards.overview.agenda.overdue")}</span>}
          </li>)}
        </ul>
      </section>)}
    </div>}
    {byDate.size > 0 && mode === "calendar" && <div className="grid gap-2 sm:grid-cols-2 md:grid-cols-7">
      {dates.map((date) => <DayBox key={date} date={date} items={byDate.get(date) ?? []} profile={profile} onOpen={() => setOpenDay(date)} />)}
    </div>}
    <Dialog open={openDay !== null} onOpenChange={(open) => { if (!open) setOpenDay(null); }}>
      <DialogContent>
        <DialogHeader><DialogTitle>{openDay ? formatDate(openDay, profile) : ""}</DialogTitle></DialogHeader>
        <ul className="max-h-[60vh] divide-y overflow-y-auto">
          {(openDay ? byDate.get(openDay) ?? [] : []).map((item) => <li key={`${item.id}-${item.space_id}`} className="py-2 text-sm">
            <p className="font-medium">{item.title}</p>
            <p className="text-xs text-muted-foreground">{item.space_name} · {t(`dashboards.overview.type.${item.type}`)}{item.overdue ? ` · ${t("dashboards.overview.agenda.overdue")}` : ""}</p>
          </li>)}
        </ul>
      </DialogContent>
    </Dialog>
  </div>;
}

/** One calendar box per day: weekday and date, then up to three items. The rest open in a drawer. */
function DayBox({ date, items, profile, onOpen }: { date: string; items: AgendaItem[]; profile?: CurrentUser; onOpen: () => void }) {
  const { t } = useTranslation();
  const weekday = new Intl.DateTimeFormat(profile?.locale ?? "en", { weekday: "short", timeZone: "UTC" }).format(new Date(`${date}T00:00:00Z`));
  const extra = items.length - DAY_CHIPS;
  return <div className={cn("flex min-h-28 flex-col gap-1.5 rounded-lg border bg-card p-2", date === todayKey() && "border-primary/60 ring-1 ring-primary/30")}>
    <div className="flex items-baseline justify-between gap-2">
      <h2 className="text-xs text-muted-foreground"><span>{weekday}</span> <span className="sr-only">{formatDate(date, profile)}</span></h2>
      <span className="text-sm font-semibold tabular-nums" aria-hidden>{Number(date.slice(8))}</span>
    </div>
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
