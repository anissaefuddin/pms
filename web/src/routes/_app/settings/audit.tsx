import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { ApiError, api } from "@/lib/api";
import { toCsv } from "@/lib/csv";
import { formatDateTime, useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

type AuditEvent = components["schemas"]["AuditEvent"];
type Page = components["schemas"]["AuditPage"];

export const Route = createFileRoute("/_app/settings/audit")({ component: AuditPage });

export const AUDIT_CATEGORIES = ["auth", "user", "share", "role", "sso", "scim", "policy", "api_key", "webhook", "oauth", "audit"] as const;
const COLUMNS = ["time", "actor", "action", "target", "ip", "details"] as const;
const CSV_HEADER = ["id", "at", "actor_id", "actor_name", "action", "target_type", "target_id", "ip", "details"];
const select = "mt-1 block h-9 rounded-md border border-input bg-background px-2";
const CELL = "py-1.5 pr-4";
const DRAWER = "sm:max-w-lg";
const FIELD = "grid grid-cols-[9rem_1fr] gap-3 border-b py-2";
const FIELD_VALUE = "break-all font-mono text-xs";
const HINT = "text-xs text-muted-foreground";

function AuditPage() {
  const { t } = useTranslation();
  const profile = useProfile();
  const [category, setCategory] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<AuditEvent | null>(null);
  const params = new URLSearchParams();
  if (category) params.set("category", category);
  if (from) params.set("from", `${from}T00:00:00Z`);
  if (to) params.set("to", `${to}T23:59:59Z`);
  const query = useInfiniteQuery({
    queryKey: ["audit", params.toString()], initialPageParam: "", retry: false, enabled: profile?.user_type === "admin",
    queryFn: ({ pageParam }) => {
      const next = new URLSearchParams(params);
      if (pageParam) next.set("cursor", pageParam);
      return api<Page>(`/api/v1/audit-events?${next}`);
    },
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
  if (profile && profile.user_type !== "admin") return <p role="alert">{t("audit.adminOnly")}</p>;
  const events = query.data?.pages.flatMap((page) => page.items) ?? [];
  const needle = search.trim().toLowerCase();
  const shown = needle ? events.filter((event) => searchText(event, t(`audit.action.${event.action}`, { defaultValue: event.action })).includes(needle)) : events;
  return <section className="space-y-5">
    <header>
      <h2 className="text-lg font-semibold">{t("audit.title")}</h2>
      <p className="text-sm text-muted-foreground">{t("audit.description")}</p>
    </header>
    <div className="flex flex-wrap items-end gap-4">
      <label className="block text-sm">{t("audit.category")}
        <select className={select} value={category} onChange={(e) => setCategory(e.target.value)}>
          <option value="">{t("audit.allCategories")}</option>
          {AUDIT_CATEGORIES.map((value) => <option key={value} value={value}>{t(`audit.categories.${value}`)}</option>)}
        </select></label>
      <label className="block text-sm">{t("audit.from")}<Input type="date" className="mt-1" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} /></label>
      <label className="block text-sm">{t("audit.to")}<Input type="date" className="mt-1" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} /></label>
      <label className="block text-sm">{t("audit.search")}<Input type="search" className="mt-1" value={search} onChange={(e) => setSearch(e.target.value)} /></label>
      <a className="text-sm underline" href={`/api/v1/audit-events/export?${params}`} download>{t("audit.export")}</a>
      <Button variant="outline" size="sm" disabled={shown.length === 0} onClick={() => downloadCsv(shown)}>{t("audit.exportShown")}</Button>
    </div>
    <p className={HINT}>{t("audit.exportShownHint")}</p>
    {query.isPending && <p role="status" className="text-sm text-muted-foreground">{t("audit.loading")}</p>}
    {query.isError && <div><p role="alert">{(query.error as ApiError).status === 403 ? t("audit.adminOnly") : t("audit.error")}</p>
      <Button variant="outline" className="mt-3" onClick={() => void query.refetch()}>{t("audit.retry")}</Button></div>}
    {query.isSuccess && events.length === 0 && <p role="status" className="rounded-xl border border-dashed p-8 text-center text-sm text-muted-foreground">{t("audit.empty")}</p>}
    {events.length > 0 && shown.length === 0 && <p role="status" className="text-sm text-muted-foreground">{t("audit.noSearchMatch")}</p>}
    {shown.length > 0 && <div className="relative overflow-x-auto" role="region" tabIndex={0} aria-label={t("audit.title")}><table className="w-full text-left text-sm">
      <thead><tr className="border-b">
        {COLUMNS.map((key) => <th key={key} scope="col" className="py-2 pr-4">{t(`audit.col.${key}`)}</th>)}
        <th scope="col" className="py-2"><span className="sr-only">{t("audit.col.open")}</span></th>
      </tr></thead>
      <tbody>{shown.map((event) => <Row key={event.id} event={event} onView={setSelected} />)}</tbody>
    </table></div>}
    {query.hasNextPage && <Button variant="outline" disabled={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>{t("audit.more")}</Button>}
    <Dialog open={selected !== null} onOpenChange={(open) => { if (!open) setSelected(null); }}>
      <DialogContent className={DRAWER}>
        <DialogTitle>{t("audit.detailsTitle")}</DialogTitle>
        <DialogDescription className="sr-only">{t("audit.detailsDescription")}</DialogDescription>
        {selected && <EventFields event={selected} />}
      </DialogContent>
    </Dialog>
  </section>;
}

function Row({ event, onView }: { event: AuditEvent; onView: (event: AuditEvent) => void }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const label = t(`audit.action.${event.action}`, { defaultValue: event.action });
  const details = Object.keys(event.details).length ? JSON.stringify(event.details) : "";
  return <tr className="border-b align-top">
    <td className={`${CELL} whitespace-nowrap`}>{formatDateTime(event.at, profile)}</td>
    <td className={CELL}>{event.actor_name || t("audit.system")}</td>
    <td className={CELL}>{label}<span className="block text-xs text-muted-foreground">{event.action}</span></td>
    <td className={`${CELL} break-all`}>{event.target_type ? `${event.target_type} ${event.target_id}` : ""}</td>
    <td className={CELL}>{event.ip}</td>
    <td className={`${CELL} break-all font-mono text-xs`}>{details}</td>
    <td className="py-1.5"><Button variant="outline" size="sm" aria-haspopup="dialog" aria-label={t("audit.viewEvent", { action: label })} onClick={() => onView(event)}>{t("audit.view")}</Button></td>
  </tr>;
}

function EventFields({ event }: { event: AuditEvent }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const fields: [string, string][] = [
    [t("audit.field.id"), String(event.id)],
    [t("audit.field.at"), formatDateTime(event.at, profile)],
    [t("audit.field.actorId"), event.actor_id ?? t("audit.system")],
    [t("audit.field.actorName"), event.actor_name || t("audit.system")],
    [t("audit.field.action"), t(`audit.action.${event.action}`, { defaultValue: event.action })],
    [t("audit.field.actionCode"), event.action],
    [t("audit.field.targetType"), event.target_type],
    [t("audit.field.targetId"), event.target_id],
    [t("audit.field.ip"), event.ip],
  ];
  return <dl>
    {fields.map(([label, value]) => <div key={label} className={FIELD}><dt className="text-muted-foreground">{label}</dt><dd className={FIELD_VALUE}>{value}</dd></div>)}
    {Object.keys(event.details).length === 0
      ? <p className="pt-2 text-sm text-muted-foreground">{t("audit.noDetails")}</p>
      : Object.entries(event.details).map(([key, value]) => <div key={key} className={FIELD}><dt className="text-muted-foreground">{key}</dt><dd className={FIELD_VALUE}>{typeof value === "string" ? value : JSON.stringify(value)}</dd></div>)}
  </dl>;
}

// Search runs over the loaded rows only; it matches what the table shows (label, code, actor, target, IP, details).
function searchText(event: AuditEvent, label: string) {
  return [label, event.action, event.actor_name, event.target_type, event.target_id, event.ip, JSON.stringify(event.details)].join(" ").toLowerCase();
}

function downloadCsv(rows: AuditEvent[]) {
  const csv = toCsv([CSV_HEADER, ...rows.map((e) => [e.id, e.at, e.actor_id ?? "", e.actor_name, e.action, e.target_type, e.target_id, e.ip, JSON.stringify(e.details)])]);
  const url = URL.createObjectURL(new Blob(["﻿" + csv], { type: "text/csv;charset=utf-8" }));
  const link = document.createElement("a");
  link.href = url;
  link.download = `audit-events-${new Date().toISOString().slice(0, 10)}.csv`;
  link.click();
  URL.revokeObjectURL(url);
}
