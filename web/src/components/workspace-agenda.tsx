import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { AGENDA_DAYS, addDays, useOverviewAgenda, type AgendaItem, type AgendaKind } from "@/lib/overview";
import { formatDate, formatNumber, useProfile } from "@/lib/profile";
import { useSpaces } from "@/lib/space-queries";
import { Button } from "@/components/ui/button";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { PeriodNav } from "@/components/overview-period-nav";
import { cn } from "@/lib/utils";

const todayKey = () => {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};

/** One calendar box per day: weekday and date at the top, due items below as coloured chips. */
function DayBox({ date, items }: { date: string; items: AgendaItem[] }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const weekday = new Intl.DateTimeFormat(profile?.locale ?? "en", { weekday: "short", timeZone: "UTC" }).format(new Date(`${date}T00:00:00Z`));
  return <div
    className={cn("flex min-h-28 flex-col gap-1.5 rounded-lg border bg-card p-2", date === todayKey() && "border-primary/60 ring-1 ring-primary/30")}>
    <div className="flex items-baseline justify-between gap-2">
      <h2 className="sr-only">{formatDate(date, profile)}</h2>
      <span className="text-xs text-muted-foreground" aria-hidden>{weekday}</span>
      <span className="text-sm font-semibold tabular-nums" aria-hidden>{Number(date.slice(8))}</span>
    </div>
    {items.map((item) => <Link key={`${item.id}-${item.space_id}`} to="/spaces/$spaceId" params={{ spaceId: item.space_id }}
      title={`${item.title} · ${item.space_name} · ${t(`dashboards.overview.type.${item.type}`)}`}
      className={cn("block rounded-md border-l-2 px-1.5 py-1 text-xs outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50",
        item.overdue ? "border-destructive bg-destructive/10" : item.type === "project" ? "border-sky-500 bg-sky-500/10" : "border-primary bg-primary/5")}>
      <span className="block truncate font-medium">{item.title}</span>
      <span className="block truncate text-xs text-foreground/80">{item.space_name}</span>
    </Link>)}
  </div>;
}

const select = "mt-1 block h-9 rounded-md border border-input bg-background px-2";
const KINDS = ["all", "project", "task"] as const;

/** Due Projects and Tasks for the next two weeks, grouped by date. Read-only. Completed items are not listed. */
export function WorkspaceAgenda() {
  const { t } = useTranslation();
  const profile = useProfile();
  const spaces = (useSpaces().data ?? []).filter((space) => space.kind === "shared");
  const [from, setFrom] = useState<string | null>(null);
  const [spaceId, setSpaceId] = useState("");
  const [kind, setKind] = useState<(typeof KINDS)[number]>("all");
  const q = useOverviewAgenda(from, spaceId, kind === "all" ? "" : kind as AgendaKind);
  if (q.isPending) return <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.loading")}</p>;
  if (q.isError) return <div><p role="alert">{t("dashboards.overview.error")}</p>
    <Button variant="outline" className="mt-3" onClick={() => q.refetch()}>{t("dashboards.overview.retry")}</Button></div>;
  const data = q.data;
  const dates = Array.from({ length: AGENDA_DAYS }, (_, i) => addDays(data.from, i));
  const byDate = new Map<string, AgendaItem[]>();
  for (const item of data.items) byDate.set(item.due_date, [...(byDate.get(item.due_date) ?? []), item]);
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
      </div>
    </div>
    {data.overdue_count > 0 && <p role="status" className="text-sm text-destructive">
      {t("dashboards.overview.agenda.overdueCount", { count: formatNumber(data.overdue_count, profile) })}</p>}
    {data.truncated && <p role="status" className="text-sm text-muted-foreground">{t("dashboards.overview.agenda.truncated")}</p>}
    {byDate.size === 0 && <p className="text-sm text-muted-foreground">{t("dashboards.overview.agenda.empty")}</p>}
    <div className="grid gap-2 sm:grid-cols-2 md:grid-cols-7">
      {dates.map((date) => <DayBox key={date} date={date} items={byDate.get(date) ?? []} />)}
    </div>
  </div>;
}
