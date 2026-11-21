import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { AGENDA_DAYS, useOverviewAgenda, type AgendaItem, type AgendaKind } from "@/lib/overview";
import { formatDate, formatNumber, useProfile } from "@/lib/profile";
import { useSpaces } from "@/lib/space-queries";
import { Button } from "@/components/ui/button";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { PeriodNav } from "@/components/overview-period-nav";

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
    <ol className="space-y-4">
      {[...byDate].map(([date, items]) => <li key={date}>
        <h2 className="text-sm font-semibold">{formatDate(date, profile)}</h2>
        <ul className="mt-1 divide-y rounded-lg border bg-card">
          {items.map((item) => <li key={`${item.id}-${item.space_id}`} className="flex items-center justify-between gap-3 px-4 py-2 text-sm">
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
      </li>)}
    </ol>
  </div>;
}
