import { useState, type ReactNode } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import {
  DIMENSIONS, ENUM_DIMENSIONS, IMPORTANCE, METRICS, SOURCES, STATUS_GROUPS, changeSource, cleanQuery, weekdays,
  type AnalyticsFilters, type AnalyticsQuery, type AnalyticsResult, type Report, type Source,
} from "@/lib/reports";
import { useProfile } from "@/lib/profile";
import { useSearchResults } from "@/lib/search";
import { useSpaces } from "@/lib/space-queries";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/reports/$reportId")({ component: ReportPage });

const select = "mt-1 block h-9 rounded-md border border-input bg-background px-2";

function ReportPage() {
  const { t } = useTranslation();
  const { reportId } = Route.useParams();
  const report = useQuery({ queryKey: ["report", reportId], retry: false, queryFn: () => api<Report>(`/api/v1/reports/${reportId}`) });
  if (report.isPending) return <main className="p-8"><p role="status" className="text-sm text-muted-foreground">{t("reports.editor.loading")}</p></main>;
  if (report.isError) return <main className="space-y-3 p-8"><p role="alert">{t("reports.editor.notFound")}</p>
    <Link to="/reports" className="underline">{t("reports.editor.back")}</Link></main>;
  return <Editor key={`${report.data.id}:${report.data.version}`} report={report.data} />;
}

function Fieldset({ legend, children }: { legend: string; children: ReactNode }) {
  return <fieldset className="space-y-3 rounded-lg border p-4"><legend className="px-1 text-sm font-semibold">{legend}</legend>{children}</fieldset>;
}

function Check({ label, checked, onChange }: { label: string; checked: boolean; onChange: (on: boolean) => void }) {
  return <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="size-6" checked={checked} onChange={(e) => onChange(e.target.checked)} />{label}</label>;
}

function toggle<T>(list: T[] | undefined, value: T, on: boolean): T[] {
  return on ? [...(list ?? []).filter((v) => v !== value), value] : (list ?? []).filter((v) => v !== value);
}

function Editor({ report }: { report: Report }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const spaces = useSpaces().data ?? [];
  const [name, setName] = useState(report.name);
  const [sharing, setSharing] = useState(report.sharing);
  const [layout, setLayout] = useState(report.definition.layout);
  const [query, setQuery] = useState<AnalyticsQuery>(report.definition.query);
  const [applied, setApplied] = useState<AnalyticsQuery>(report.definition.query);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const source = query.reporting_on as Source;
  const filters = query.filters ?? {};
  const setFilters = (patch: Partial<AnalyticsFilters>) => setQuery({ ...query, filters: { ...filters, ...patch } });
  const isOwner = useProfile()?.id === report.owner.id;
  const dirty = name !== report.name || sharing !== report.sharing || layout !== report.definition.layout
    || JSON.stringify(cleanQuery(query)) !== JSON.stringify(cleanQuery(report.definition.query));
  const metricLabel = (metric: string) => t(source === "time_entries" && metric === "count" ? "reports.metric.countEntries" : `reports.metric.${metric}`);

  const result = useQuery({
    queryKey: ["analytics", JSON.stringify(cleanQuery(applied))], retry: false,
    queryFn: () => api<AnalyticsResult>("/api/v1/analytics/query", { method: "POST", body: JSON.stringify(cleanQuery(applied)) }),
  });

  async function save() {
    setSaving(true); setError("");
    try {
      await api(`/api/v1/reports/${report.id}`, { method: "PATCH", headers: { "If-Match": `"${report.version}"` },
        body: JSON.stringify({ name, sharing, definition: { layout, query: cleanQuery(query) } }) });
      toast.success(t("reports.editor.saved"));
      await Promise.all([qc.invalidateQueries({ queryKey: ["report", report.id] }), qc.invalidateQueries({ queryKey: ["reports"] })]);
    } catch (err) {
      setError(err instanceof ApiError && err.status === 409 ? t("reports.editor.conflict") : (err as Error).message || t("reports.editor.saveFailed"));
    } finally { setSaving(false); }
  }

  return <main className="mx-auto max-w-5xl space-y-5 p-8">
    <header className="space-y-2 border-b pb-5">
      <Link to="/reports" className="text-sm underline">{t("reports.editor.back")}</Link>
      <h1 className="text-2xl font-semibold">{report.name}</h1>
      <p className="text-sm text-muted-foreground">{report.space ? t("reports.editor.location", { space: report.space.name }) : t("reports.personal")}</p>
    </header>
    <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); void save(); }}>
      <div className="flex flex-wrap items-end gap-4">
        <label className="block text-sm">{t("reports.editor.name")}
          <Input required maxLength={120} value={name} onChange={(e) => setName(e.target.value)} className="mt-1 w-72" /></label>
        {report.space && <label className="block text-sm">{t("reports.editor.sharing")}
          <select className={select} value={sharing} disabled={!isOwner} onChange={(e) => setSharing(e.target.value as typeof sharing)}>
            <option value="private">{t("reports.sharing.private")}</option>
            <option value="space">{t("reports.sharing.space")}</option>
          </select></label>}
        <label className="block text-sm">{t("reports.layout.label")}
          <select className={select} value={layout} onChange={(e) => setLayout(e.target.value as typeof layout)}>
            <option value="table">{t("reports.layout.table")}</option>
            <option value="column">{t("reports.layout.column")}</option>
          </select></label>
      </div>

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
            <select className={select} value={query.metric} onChange={(e) => setQuery({ ...query, metric: e.target.value as AnalyticsQuery["metric"] })}>
              {METRICS[source].map((value) => <option key={value} value={value}>{metricLabel(value)}</option>)}
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

      <Fieldset legend={t("reports.filters.legend")}>
        {source === "time_entries" ? <div className="flex flex-wrap gap-4">
          <Check label={t("reports.filters.loggedByMe")} checked={!!filters.assignees?.includes("me")} onChange={(on) => setFilters({ assignees: on ? ["me"] : [] })} />
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

      <div className="flex flex-wrap items-center gap-3">
        <Button type="submit" disabled={saving || !dirty || !name.trim()}>{t("reports.editor.save")}</Button>
        <Button type="button" variant="outline" onClick={() => setApplied(query)}>{t("reports.editor.run")}</Button>
        <a className="text-sm underline" href={`/api/v1/reports/${report.id}/export`} download>{t("reports.editor.export")}</a>
        {dirty && <span className="text-sm text-muted-foreground">{t("reports.editor.unsaved")}</span>}
      </div>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    </form>

    <section aria-labelledby="report-result" className="space-y-3">
      <h2 id="report-result" className="text-lg font-semibold">{t("reports.result.title")}</h2>
      <ResultView result={result.data} pending={result.isPending} error={result.error} layout={layout} query={applied} metricLabel={metricLabel} />
    </section>
    <Subscription report={report} />
  </main>;
}

function ResultView({ result, pending, error, layout, query, metricLabel }: {
  result?: AnalyticsResult; pending: boolean; error: Error | null; layout: "table" | "column"; query: AnalyticsQuery; metricLabel: (metric: string) => string;
}) {
  const { t } = useTranslation();
  if (pending) return <p role="status" className="text-sm text-muted-foreground">{t("reports.result.loading")}</p>;
  if (error) return <p role="alert" className="text-sm text-destructive">{(error as ApiError).status === 422 ? t("reports.result.tooLarge") : t("reports.result.error")}</p>;
  if (!result || result.rows.length === 0) return <p role="status" className="text-sm text-muted-foreground">{t("reports.result.empty")}</p>;
  const label = (dimension: string | undefined, key: string, text: string) => !text && !key ? t("reports.value.none")
    : dimension && ENUM_DIMENSIONS.includes(dimension) ? t(`reports.value.${key}`, { defaultValue: text }) : text;
  const max = Math.max(1, ...result.rows.map((row) => row.value));
  const group = query.group_by ? t(`reports.dim.${query.group_by}`) : t("reports.result.group");
  return <>
    {layout === "column" && <div role="img" aria-label={t("reports.result.chart", { metric: metricLabel(query.metric), group })}
      className="flex h-48 items-end gap-2 overflow-x-auto border-b px-1">
      {result.rows.slice(0, 40).map((row) => <div key={`${row.group.key}/${row.break?.key ?? ""}`} className="flex min-w-8 flex-1 flex-col items-center justify-end" style={{ height: "100%" }}>
        <div className="w-full rounded-t bg-primary" style={{ height: `${Math.max(2, (row.value / max) * 100)}%` }} />
      </div>)}
    </div>}
    <table className="w-full text-left text-sm">
      <thead><tr className="border-b">
        <th scope="col" className="py-2 pr-4">{group}</th>
        {query.break_by && <th scope="col" className="py-2 pr-4">{t(`reports.dim.${query.break_by}`)}</th>}
        <th scope="col" className="py-2 pr-4 text-right">{metricLabel(query.metric)}</th>
      </tr></thead>
      <tbody>{result.rows.map((row) => <tr key={`${row.group.key}/${row.break?.key ?? ""}`} className="border-b">
        <th scope="row" className="py-2 pr-4 font-normal">{label(query.group_by, row.group.key, row.group.label)}</th>
        {query.break_by && <td className="py-2 pr-4">{row.break ? label(query.break_by, row.break.key, row.break.label) : ""}</td>}
        <td className="py-2 pr-4 text-right tabular-nums">{row.value}</td>
      </tr>)}</tbody>
      <tfoot><tr><th scope="row" colSpan={query.break_by ? 2 : 1} className="py-2 pr-4 text-left">{t("reports.result.total", { count: result.total })}</th><td /></tr></tfoot>
    </table>
  </>;
}

function Subscription({ report }: { report: Report }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [weekday, setWeekday] = useState(report.subscription?.weekday ?? 1);
  const [hour, setHour] = useState(report.subscription?.hour ?? 8);
  const [busy, setBusy] = useState(false);
  async function change(remove: boolean) {
    setBusy(true);
    try {
      await api(`/api/v1/reports/${report.id}/subscription`, remove ? { method: "DELETE" } : { method: "PUT", body: JSON.stringify({ weekday, hour }) });
      toast.success(t(remove ? "reports.subscribe.removed" : "reports.subscribe.saved"));
      await Promise.all([qc.invalidateQueries({ queryKey: ["report", report.id] }), qc.invalidateQueries({ queryKey: ["reports"] })]);
    } catch { toast.error(t("reports.subscribe.failed")); }
    finally { setBusy(false); }
  }
  return <Fieldset legend={t("reports.subscribe.title")}>
    <div className="flex flex-wrap items-end gap-4">
      <label className="block text-sm">{t("reports.subscribe.day")}
        <select className={select} value={weekday} onChange={(e) => setWeekday(Number(e.target.value))}>
          {weekdays.map((day) => <option key={day} value={day}>{t(`reports.day.${day}`)}</option>)}
        </select></label>
      <label className="block text-sm">{t("reports.subscribe.hour")}
        <select className={select} value={hour} onChange={(e) => setHour(Number(e.target.value))}>
          {Array.from({ length: 24 }, (_, h) => <option key={h} value={h}>{String(h).padStart(2, "0")}:00</option>)}
        </select></label>
      <Button type="button" disabled={busy} onClick={() => void change(false)}>{t("reports.subscribe.save")}</Button>
      {report.subscription && <Button type="button" variant="outline" disabled={busy} onClick={() => void change(true)}>{t("reports.subscribe.remove")}</Button>}
    </div>
    <p className="text-sm text-muted-foreground">{t("reports.subscribe.note")}</p>
  </Fieldset>;
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
