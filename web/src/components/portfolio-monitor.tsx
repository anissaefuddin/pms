import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useId, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { formatDate, useProfile } from "@/lib/profile";
import { Input } from "@/components/ui/input";

// PROMPT 034: consolidated project monitor. The server decides every row, filter option and figure from the caller's authorised
// projects. Missing progress or health is shown as unavailable, never as a value.
type Row = {
  id: string; title: string; project_code: string | null;
  customer: { id: string; code: string; name: string; archived: boolean } | null;
  owner_id: string | null; owner_name: string | null;
  status: string; status_group: string;
  progress_percent: number | null; health: "green" | "yellow" | "red" | null;
  start_date: string | null; finish_date: string | null;
  overdue_tasks: number; last_update: string;
};
type Monitor = {
  items: Row[];
  summary: { total: number; by_health: Record<string, number>; overdue_tasks: number; projects_with_overdue: number; unassigned: number };
  options: { customers: { id: string; code: string; name: string }[]; owners: { id: string; name: string }[] };
};

const SORTS = ["title", "customer", "owner", "finish", "progress", "health", "overdue", "updated"] as const;
const HEALTH = ["", "green", "yellow", "red", "none"] as const;
const STATUS = ["", "active", "completed", "deferred", "cancelled"] as const;
const select = "mt-1 block h-9 rounded-md border border-input bg-background px-2";

export function PortfolioMonitor() {
  const { t } = useTranslation();
  const profile = useProfile();
  const ids = useId();
  const [customer, setCustomer] = useState("");
  const [owner, setOwner] = useState("");
  const [health, setHealth] = useState<(typeof HEALTH)[number]>("");
  const [status, setStatus] = useState<(typeof STATUS)[number]>("");
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<(typeof SORTS)[number]>("finish");
  const [desc, setDesc] = useState(false);
  const params = useMemo(() => {
    const p = new URLSearchParams({ sort, order: desc ? "desc" : "asc" });
    if (customer) p.set("customer", customer);
    if (owner) p.set("owner", owner);
    if (health) p.set("health", health);
    if (status) p.set("status", status);
    if (query.trim()) p.set("q", query.trim().slice(0, 200));
    return p.toString();
  }, [customer, owner, health, status, query, sort, desc]);
  const data = useQuery({
    queryKey: ["portfolio-monitor", params],
    queryFn: () => api<Monitor>(`/api/v1/portfolio/projects?${params}`),
  });
  const na = t("views.monitor.unavailable");
  const unassigned = t("views.monitor.unassigned");
  const healthLabel = (h: Row["health"]) => h ? t(`items.progress.risk.${h}`) : na;
  const day = (d: string | null) => d ? formatDate(d, profile) : "—";

  return <section aria-labelledby={ids} className="space-y-4">
    <h2 id={ids} className="font-medium">{t("views.monitor.title")}</h2>
    {data.isPending && <p role="status" className="text-sm text-muted-foreground">{t("views.monitor.loading")}</p>}
    {data.isError && <p role="alert" className="text-sm text-destructive">{t("views.monitor.loadError")}</p>}
    {data.data && <>
      <dl className="grid grid-cols-2 gap-3 text-sm md:grid-cols-5">
        <div className="rounded-lg border bg-card p-3"><dt className="text-muted-foreground">{t("views.monitor.total")}</dt>
          <dd className="text-lg font-semibold tabular-nums">{data.data.summary.total}</dd></div>
        <div className="rounded-lg border bg-card p-3"><dt className="text-muted-foreground">{t("views.monitor.health")}</dt>
          <dd className="tabular-nums">{t("views.monitor.healthCounts", { green: data.data.summary.by_health.green, yellow: data.data.summary.by_health.yellow, red: data.data.summary.by_health.red, none: data.data.summary.by_health.none })}</dd></div>
        <div className="rounded-lg border bg-card p-3"><dt className="text-muted-foreground">{t("views.monitor.overdueTasks")}</dt>
          <dd className="text-lg font-semibold tabular-nums">{data.data.summary.overdue_tasks}</dd></div>
        <div className="rounded-lg border bg-card p-3"><dt className="text-muted-foreground">{t("views.monitor.lateProjects")}</dt>
          <dd className="text-lg font-semibold tabular-nums">{data.data.summary.projects_with_overdue}</dd></div>
        <div className="rounded-lg border bg-card p-3"><dt className="text-muted-foreground">{t("views.monitor.unassignedCount")}</dt>
          <dd className="text-lg font-semibold tabular-nums">{data.data.summary.unassigned}</dd></div>
      </dl>
      <div className="flex flex-wrap items-end gap-3">
        <label className="block text-sm">{t("views.monitor.search")}
          <Input type="search" value={query} onChange={(e) => setQuery(e.target.value)} placeholder={t("views.monitor.searchPlaceholder")} className="mt-1 h-9 w-56" /></label>
        <label className="block text-sm">{t("views.monitor.customer")}
          <select className={select} value={customer} onChange={(e) => setCustomer(e.target.value)}>
            <option value="">{t("views.monitor.any")}</option>
            {data.data.options.customers.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
          </select></label>
        <label className="block text-sm">{t("views.monitor.owner")}
          <select className={select} value={owner} onChange={(e) => setOwner(e.target.value)}>
            <option value="">{t("views.monitor.any")}</option>
            <option value="none">{unassigned}</option>
            {data.data.options.owners.map((o) => <option key={o.id} value={o.id}>{o.name}</option>)}
          </select></label>
        <label className="block text-sm">{t("views.monitor.healthFilter")}
          <select className={select} value={health} onChange={(e) => setHealth(e.target.value as (typeof HEALTH)[number])}>
            {HEALTH.map((h) => <option key={h || "all"} value={h}>{h ? t(`views.monitor.healthOption.${h}`) : t("views.monitor.any")}</option>)}
          </select></label>
        <label className="block text-sm">{t("views.monitor.statusFilter")}
          <select className={select} value={status} onChange={(e) => setStatus(e.target.value as (typeof STATUS)[number])}>
            {STATUS.map((s) => <option key={s || "all"} value={s}>{s ? t(`views.monitor.statusOption.${s}`) : t("views.monitor.any")}</option>)}
          </select></label>
        <label className="block text-sm">{t("views.monitor.sort")}
          <select className={select} value={sort} onChange={(e) => setSort(e.target.value as (typeof SORTS)[number])}>
            {SORTS.map((s) => <option key={s} value={s}>{t(`views.monitor.sortOption.${s}`)}</option>)}
          </select></label>
        <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={desc} onChange={(e) => setDesc(e.target.checked)} />{t("views.monitor.descending")}</label>
      </div>
      {data.data.items.length === 0
        ? <p className="text-sm text-muted-foreground">{t("views.monitor.empty")}</p>
        : <div className="overflow-x-auto">
          <table className="w-full min-w-[60rem] border-collapse text-left text-sm">
            <caption className="sr-only">{t("views.monitor.title")}</caption>
            <thead><tr className="border-b text-muted-foreground">
              <th scope="col" className="py-2 pr-3 font-medium">{t("views.monitor.col.code")}</th>
              <th scope="col" className="py-2 pr-3 font-medium">{t("views.monitor.col.project")}</th>
              <th scope="col" className="py-2 pr-3 font-medium">{t("views.monitor.col.customer")}</th>
              <th scope="col" className="py-2 pr-3 font-medium">{t("views.monitor.col.owner")}</th>
              <th scope="col" className="py-2 pr-3 font-medium">{t("views.monitor.col.status")}</th>
              <th scope="col" className="py-2 pr-3 font-medium">{t("views.monitor.col.progress")}</th>
              <th scope="col" className="py-2 pr-3 font-medium">{t("views.monitor.col.health")}</th>
              <th scope="col" className="py-2 pr-3 font-medium">{t("views.monitor.col.start")}</th>
              <th scope="col" className="py-2 pr-3 font-medium">{t("views.monitor.col.finish")}</th>
              <th scope="col" className="py-2 pr-3 font-medium">{t("views.monitor.col.overdue")}</th>
              <th scope="col" className="py-2 font-medium">{t("views.monitor.col.updated")}</th>
            </tr></thead>
            <tbody>
              {data.data.items.map((r) => <tr key={r.id} className="border-b align-top">
                <td className="py-2 pr-3 tabular-nums">{r.project_code ?? "—"}</td>
                <td className="py-2 pr-3"><Link to="/tasks/$itemId" params={{ itemId: r.id }} className="font-medium underline-offset-2 hover:underline">{r.title}</Link></td>
                <td className="py-2 pr-3">{r.customer ? `${r.customer.name}${r.customer.archived ? ` (${t("views.monitor.archived")})` : ""}` : t("views.monitor.internal")}</td>
                <td className="py-2 pr-3">{r.owner_name ?? unassigned}</td>
                <td className="py-2 pr-3">{r.status || na}</td>
                <td className="py-2 pr-3 tabular-nums">{r.progress_percent === null ? na : t("views.project.percent", { value: r.progress_percent })}</td>
                <td className="py-2 pr-3">{healthLabel(r.health)}</td>
                <td className="py-2 pr-3 tabular-nums">{day(r.start_date)}</td>
                <td className="py-2 pr-3 tabular-nums">{day(r.finish_date)}</td>
                <td className={`py-2 pr-3 tabular-nums ${r.overdue_tasks > 0 ? "font-semibold text-destructive" : ""}`}>{r.overdue_tasks}</td>
                <td className="py-2 tabular-nums">{day(r.last_update.slice(0, 10))}</td>
              </tr>)}
            </tbody>
          </table>
        </div>}
    </>}
  </section>;
}
