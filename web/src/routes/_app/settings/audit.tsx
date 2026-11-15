import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { ApiError, api } from "@/lib/api";
import { formatDateTime, useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type AuditEvent = components["schemas"]["AuditEvent"];
type Page = components["schemas"]["AuditPage"];

export const Route = createFileRoute("/_app/settings/audit")({ component: AuditPage });

export const AUDIT_CATEGORIES = ["auth", "user", "share", "role", "sso", "scim", "policy", "api_key", "webhook", "oauth", "audit"] as const;
const COLUMNS = ["time", "actor", "action", "target", "ip", "details"] as const;
const select = "mt-1 block h-9 rounded-md border border-input bg-background px-2";

function AuditPage() {
  const { t } = useTranslation();
  const profile = useProfile();
  const [category, setCategory] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
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
      <a className="text-sm underline" href={`/api/v1/audit-events/export?${params}`} download>{t("audit.export")}</a>
    </div>
    {query.isPending && <p role="status" className="text-sm text-muted-foreground">{t("audit.loading")}</p>}
    {query.isError && <div><p role="alert">{(query.error as ApiError).status === 403 ? t("audit.adminOnly") : t("audit.error")}</p>
      <Button variant="outline" className="mt-3" onClick={() => void query.refetch()}>{t("audit.retry")}</Button></div>}
    {query.isSuccess && events.length === 0 && <p role="status" className="rounded-xl border border-dashed p-8 text-center text-sm text-muted-foreground">{t("audit.empty")}</p>}
    {events.length > 0 && <div className="relative overflow-x-auto" role="region" tabIndex={0} aria-label={t("audit.title")}><table className="w-full text-left text-sm">
      <thead><tr className="border-b">
        {COLUMNS.map((key) => <th key={key} scope="col" className="py-2 pr-4">{t(`audit.col.${key}`)}</th>)}
      </tr></thead>
      <tbody>{events.map((event) => <Row key={event.id} event={event} />)}</tbody>
    </table></div>}
    {query.hasNextPage && <Button variant="outline" disabled={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>{t("audit.more")}</Button>}
  </section>;
}

function Row({ event }: { event: AuditEvent }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const details = Object.keys(event.details).length ? JSON.stringify(event.details) : "";
  return <tr className="border-b align-top">
    <td className="whitespace-nowrap py-2 pr-4">{formatDateTime(event.at, profile)}</td>
    <td className="py-2 pr-4">{event.actor_name || t("audit.system")}</td>
    <td className="py-2 pr-4">{t(`audit.action.${event.action}`, { defaultValue: event.action })}<span className="block text-xs text-muted-foreground">{event.action}</span></td>
    <td className="py-2 pr-4 break-all">{event.target_type ? `${event.target_type} ${event.target_id}` : ""}</td>
    <td className="py-2 pr-4">{event.ip}</td>
    <td className="py-2 break-all font-mono text-xs">{details}</td>
  </tr>;
}
