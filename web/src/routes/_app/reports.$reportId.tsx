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
import { QueryEditor, Fieldset } from "@/components/query-editor";
import { ResultTable } from "@/components/result-table";
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
      <h1 className="workspace-title">{report.name}</h1>
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

      <QueryEditor query={query} setQuery={setQuery} />

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
    <ResultTable result={result} query={query} />
  </>;
}

/** The browser's list of IANA zones; an empty list (old engines) leaves only the profile zone. */
const zones = () => (typeof Intl.supportedValuesOf === "function" ? Intl.supportedValuesOf("timeZone") : []);

function Subscription({ report }: { report: Report }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [weekday, setWeekday] = useState<string>(report.subscription ? String(report.subscription.weekday ?? "daily") : "1");
  const [timezone, setTimezone] = useState(report.subscription?.timezone ?? "");
  const [hour, setHour] = useState(report.subscription?.hour ?? 8);
  const [busy, setBusy] = useState(false);
  async function change(remove: boolean) {
    setBusy(true);
    try {
      await api(`/api/v1/reports/${report.id}/subscription`, remove ? { method: "DELETE" } : { method: "PUT", body: JSON.stringify({ weekday: weekday === "daily" ? null : Number(weekday), hour, timezone: timezone || null }) });
      toast.success(t(remove ? "reports.subscribe.removed" : "reports.subscribe.saved"));
      await Promise.all([qc.invalidateQueries({ queryKey: ["report", report.id] }), qc.invalidateQueries({ queryKey: ["reports"] })]);
    } catch { toast.error(t("reports.subscribe.failed")); }
    finally { setBusy(false); }
  }
  return <Fieldset legend={t("reports.subscribe.title")}>
    <div className="flex flex-wrap items-end gap-4">
      <label className="block text-sm">{t("reports.subscribe.day")}
        <select className={select} value={weekday} onChange={(e) => setWeekday(e.target.value)}>
          <option value="daily">{t("reports.subscribe.daily")}</option>
          {weekdays.map((day) => <option key={day} value={day}>{t(`reports.day.${day}`)}</option>)}
        </select></label>
      <label className="block text-sm">{t("reports.subscribe.hour")}
        <select className={select} value={hour} onChange={(e) => setHour(Number(e.target.value))}>
          {Array.from({ length: 24 }, (_, h) => <option key={h} value={h}>{String(h).padStart(2, "0")}:00</option>)}
        </select></label>
      <label className="block text-sm">{t("reports.subscribe.timezone")}
        <select className={select} value={timezone} onChange={(e) => setTimezone(e.target.value)}>
          <option value="">{t("reports.subscribe.profileZone")}</option>
          {zones().map((zone) => <option key={zone} value={zone}>{zone}</option>)}
        </select></label>
      <Button type="button" disabled={busy} onClick={() => void change(false)}>{t("reports.subscribe.save")}</Button>
      {report.subscription && <Button type="button" variant="outline" disabled={busy} onClick={() => void change(true)}>{t("reports.subscribe.remove")}</Button>}
    </div>
    <p className="text-sm text-muted-foreground">{t("reports.subscribe.note")}</p>
  </Fieldset>;
}
