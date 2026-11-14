import { useState, type ReactNode } from "react";
import { useQueries } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import {
  ADMIN_METRICS, DIMENSIONS, IMPORTANCE, METRICS, SOURCES, STATUS_GROUPS, changeSource,
  type AnalyticsFilters, type AnalyticsQuery, type Source,
} from "@/lib/reports";
import { useSearchResults } from "@/lib/search";
import { useSpaces } from "@/lib/space-queries";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

const select = "mt-1 block h-9 rounded-md border border-input bg-background px-2";

export function Fieldset({ legend, children }: { legend: string; children: ReactNode }) {
  return <fieldset className="space-y-3 rounded-lg border p-4"><legend className="px-1 text-sm font-semibold">{legend}</legend>{children}</fieldset>;
}

function Check({ label, checked, onChange }: { label: string; checked: boolean; onChange: (on: boolean) => void }) {
  return <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="size-6" checked={checked} onChange={(e) => onChange(e.target.checked)} />{label}</label>;
}

function toggle<T>(list: T[] | undefined, value: T, on: boolean): T[] {
  return on ? [...(list ?? []).filter((v) => v !== value), value] : (list ?? []).filter((v) => v !== value);
}


/** Data source, measure and filters of an analytics query; used by the report editor and the widget editor. */
/** `part` lets the widget editor show the data and the filter halves on separate tabs. */
export function QueryEditor({ query, setQuery, part }: { query: AnalyticsQuery; setQuery: (query: AnalyticsQuery) => void; part?: "data" | "filters" }) {
  const { t } = useTranslation();
  const spaces = useSpaces().data ?? [];
  const admin = useProfile()?.user_type === "admin";
  const source = query.reporting_on as Source;
  const filters = query.filters ?? {};
  const setFilters = (patch: Partial<AnalyticsFilters>) => setQuery({ ...query, filters: { ...filters, ...patch } });
  const metricLabel = (metric: string) => t(source === "time_entries" && metric === "count" ? "reports.metric.countEntries" : `reports.metric.${metric}`);
  return <>
    {part !== "filters" && <>
      <Fieldset legend={t("reports.source.legend")}>
      <label className="block text-sm">{t("reports.source.basedOn")}
        <select className={select} value={source} onChange={(e) => setQuery(changeSource(query, e.target.value as Source))}>
          {SOURCES.map((value) => <option key={value} value={value}>{t(`reports.based.${value}`)}</option>)}
        </select></label>
      <div role="group" aria-label={t("reports.source.spaces")}>
        <p className="text-sm font-medium">{t("reports.source.spaces")}</p>
        <p className="text-sm text-muted-foreground">{t("reports.source.spacesHint")}</p>
        <div className="mt-1 grid gap-1 sm:grid-cols-2">{spaces.map((space) =>
          <Check key={space.id} label={space.name} checked={!!query.space_ids?.includes(space.id)}
            onChange={(on) => setQuery({ ...query, space_ids: toggle(query.space_ids, space.id, on) })} />)}</div>
      </div>
      <Containers ids={query.container_ids ?? []} onChange={(ids) => setQuery({ ...query, container_ids: ids })} />
    </Fieldset>

    <Fieldset legend={t("reports.measure.legend")}>
      <div className="flex flex-wrap gap-4">
        <label className="block text-sm">{t("reports.measure.metric")}
          <select className={select} value={query.metric} onChange={(e) => {
              const metric = e.target.value as AnalyticsQuery["metric"];
              // Utilization is per person over a period, so choosing it sets both.
              setQuery(metric === "utilization" ? { ...query, metric, group_by: "user", break_by: undefined, filters: { ...filters, entry_this_month: filters.entry_from || filters.entry_to ? undefined : true } } : { ...query, metric });
            }}>
            {METRICS[source].filter((value) => admin || !ADMIN_METRICS.includes(value)).map((value) => <option key={value} value={value}>{metricLabel(value)}</option>)}
          </select></label>
        <label className="block text-sm">{t("reports.measure.groupBy")}
          <select className={select} value={query.group_by ?? ""} onChange={(e) => setQuery({ ...query, group_by: (e.target.value || undefined) as AnalyticsQuery["group_by"],
            break_by: e.target.value && e.target.value !== query.break_by ? query.break_by : undefined })}>
            <option value="">{t("reports.measure.none")}</option>
            {DIMENSIONS[source].map((value) => <option key={value} value={value}>{t(`reports.dim.${value}`)}</option>)}
          </select></label>
        <label className="block text-sm">{t("reports.measure.breakBy")}
          <select className={select} value={query.break_by ?? ""} disabled={!query.group_by}
            onChange={(e) => setQuery({ ...query, break_by: (e.target.value || undefined) as AnalyticsQuery["break_by"] })}>
            <option value="">{t("reports.measure.none")}</option>
            {DIMENSIONS[source].filter((value) => value !== query.group_by).map((value) => <option key={value} value={value}>{t(`reports.dim.${value}`)}</option>)}
          </select></label>
      </div>
    </Fieldset>

    </>}
    {part !== "data" && <>
    <Fieldset legend={t("reports.filters.legend")}>
      {source === "time_entries" ? <div className="flex flex-wrap gap-4">
        <Check label={t("reports.filters.loggedByMe")} checked={!!filters.assignees?.includes("me")} onChange={(on) => setFilters({ assignees: on ? ["me"] : [] })} />
        <Check label={t("reports.filters.entryThisMonth")} checked={!!filters.entry_this_month} onChange={(on) => setFilters({ entry_this_month: on })} />
        <label className="block text-sm">{t("reports.filters.entryFrom")}
          <Input type="date" className="mt-1" value={filters.entry_from ?? ""} onChange={(e) => setFilters({ entry_from: e.target.value })} /></label>
        <label className="block text-sm">{t("reports.filters.entryTo")}
          <Input type="date" className="mt-1" value={filters.entry_to ?? ""} onChange={(e) => setFilters({ entry_to: e.target.value })} /></label>
        <label className="block text-sm">{t("reports.filters.billing")}
          <select className={select} value={filters.billing ?? ""} onChange={(e) => setFilters({ billing: (e.target.value || undefined) as AnalyticsFilters["billing"] })}>
            <option value="">{t("reports.filters.billingAny")}</option>
            <option value="billable">{t("reports.value.billable")}</option>
            <option value="non_billable">{t("reports.value.non_billable")}</option>
          </select></label>
      </div> : <div className="space-y-3">
        <div role="group" aria-label={t("reports.filters.status")} className="flex flex-wrap gap-x-4">
          <span className="w-full text-sm font-medium">{t("reports.filters.status")}</span>
          {STATUS_GROUPS.map((value) => <Check key={value} label={t(`reports.value.${value}`)} checked={!!filters.status_groups?.includes(value)}
            onChange={(on) => setFilters({ status_groups: toggle(filters.status_groups, value, on) })} />)}
        </div>
        <div role="group" aria-label={t("reports.filters.importance")} className="flex flex-wrap gap-x-4">
          <span className="w-full text-sm font-medium">{t("reports.filters.importance")}</span>
          {IMPORTANCE.map((value) => <Check key={value} label={t(`reports.value.${value}`)} checked={!!filters.importance?.includes(value)}
            onChange={(on) => setFilters({ importance: toggle(filters.importance, value, on) })} />)}
        </div>
        <div className="flex flex-wrap gap-x-4">
          <Check label={t("reports.filters.assignedToMe")} checked={!!filters.assignees?.includes("me")} onChange={(on) => setFilters({ assignees: on ? ["me"] : [] })} />
          <Check label={t("reports.filters.unassigned")} checked={!!filters.unassigned} onChange={(on) => setFilters({ unassigned: on })} />
          <Check label={t("reports.filters.followed")} checked={!!filters.followed} onChange={(on) => setFilters({ followed: on })} />
          <Check label={t("reports.filters.overdue")} checked={!!filters.overdue} onChange={(on) => setFilters({ overdue: on })} />
          <Check label={t("reports.filters.dueThisMonth")} checked={!!filters.due_this_month} onChange={(on) => setFilters({ due_this_month: on })} />
        </div>
        <div className="flex flex-wrap gap-4">
          <label className="block text-sm">{t("reports.filters.dueFrom")}
            <Input type="date" className="mt-1" value={filters.due_from ?? ""} onChange={(e) => setFilters({ due_from: e.target.value })} /></label>
          <label className="block text-sm">{t("reports.filters.dueTo")}
            <Input type="date" className="mt-1" value={filters.due_to ?? ""} onChange={(e) => setFilters({ due_to: e.target.value })} /></label>
          <label className="block text-sm">{t("reports.filters.effortMin")}
            <Input type="number" min={0} className="mt-1 w-40" value={filters.effort_min ?? ""} onChange={(e) => setFilters({ effort_min: e.target.value === "" ? undefined : Number(e.target.value) })} /></label>
          <label className="block text-sm">{t("reports.filters.effortMax")}
            <Input type="number" min={0} className="mt-1 w-40" value={filters.effort_max ?? ""} onChange={(e) => setFilters({ effort_max: e.target.value === "" ? undefined : Number(e.target.value) })} /></label>
        </div>
      </div>}
    </Fieldset>

    </>}
  </>;
}

/** Limits the data source to Projects or Folders; they are found by name and shown as removable chips. */
function Containers({ ids, onChange }: { ids: string[]; onChange: (ids: string[]) => void }) {
  const { t } = useTranslation();
  const [text, setText] = useState("");
  const found = useSearchResults(text.trim(), "everywhere", {}, 10, true);
  const titles = useQueries({ queries: ids.map((id) => ({
    queryKey: ["item-title", id], staleTime: 60_000, retry: false,
    queryFn: async () => (await api<{ title: string }>(`/api/v1/items/${id}`)).title,
  })) });
  const options = (found.data?.items ?? []).filter((item) => (item.type === "project" || item.type === "folder") && !ids.includes(item.id));
  return <div role="group" aria-label={t("reports.source.containers")} className="space-y-2">
    <p className="text-sm font-medium">{t("reports.source.containers")}</p>
    <p className="text-sm text-muted-foreground">{t("reports.source.containersHint")}</p>
    <ul className="flex flex-wrap gap-2">{ids.map((id, i) => <li key={id} className="flex items-center gap-1 rounded-full border px-2 py-0.5 text-sm">
      {titles[i]?.data ?? t("reports.source.unknownContainer")}
      <Button type="button" variant="ghost" size="sm" aria-label={t("reports.source.removeContainer", { name: titles[i]?.data ?? id })}
        onClick={() => onChange(ids.filter((other) => other !== id))}>{t("reports.source.remove")}</Button></li>)}</ul>
    <Input type="search" value={text} onChange={(e) => setText(e.target.value)} aria-label={t("reports.source.findContainer")} placeholder={t("reports.source.findContainer")} className="max-w-sm" />
    {text.trim().length >= 2 && <ul aria-label={t("reports.source.containerResults")} className="max-w-sm rounded-lg border">
      {options.length === 0 ? <li className="p-2 text-sm text-muted-foreground">{t("reports.source.noContainers")}</li>
        : options.map((item) => <li key={item.id}><Button type="button" variant="ghost" className="w-full justify-start"
          onClick={() => { onChange([...ids, item.id]); setText(""); }}>{item.title}</Button></li>)}
    </ul>}
  </div>;
}
