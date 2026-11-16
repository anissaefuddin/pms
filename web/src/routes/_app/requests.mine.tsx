import { Link, createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api, type CurrentUser } from "@/lib/api";
import { formatDate } from "@/lib/profile";
import { statusDotClass } from "@/lib/status-colors";
import { Button } from "@/components/ui/button";
import { RequestTabs } from "@/components/request-tabs";

export const Route = createFileRoute("/_app/requests/mine")({ component: MyRequests });

const FILTERS = ["active", "closed", "all"] as const;

type MyRequest = components["schemas"]["MyRequest"];
type Page = { items: MyRequest[]; next_cursor: string | null };

function MyRequests() {
  const { t } = useTranslation();
  const me = useQueryClient().getQueryData<CurrentUser>(["me"]);
  const list = useInfiniteQuery({
    queryKey: ["my-requests"], retry: false, initialPageParam: "",
    queryFn: ({ pageParam }) => api<Page>(`/api/v1/me/requests?limit=25${pageParam ? `&cursor=${encodeURIComponent(pageParam)}` : ""}`),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  const rows = list.data?.pages.flatMap((page) => page.items) ?? [];
  // RF-11: most people only need what is still moving, so Active is the default view and Closed holds the rest.
  const [filter, setFilter] = useState<"all" | "active" | "closed">("active");
  const isActive = (row: MyRequest) => row.status.group === "active";
  const shown = rows.filter((row) => filter === "all" || (filter === "active") === isActive(row));
  const counts = { all: rows.length, active: rows.filter(isActive).length, closed: rows.filter((row) => !isActive(row)).length };
  return <main className="space-y-5 p-8">
    <RequestTabs />
    <header className="border-b pb-5">
      <h1 className="workspace-title">{t("forms.mine.title")}</h1>
      <p className="text-sm text-muted-foreground">{t("forms.mine.description")}</p>
    </header>
    {list.isPending && <p role="status">{t("forms.mine.loading")}</p>}
    {list.isError && <div role="alert"><p>{t("forms.mine.loadError")}</p>
      <Button variant="link" className="px-0" onClick={() => void list.refetch()}>{t("forms.mine.retry")}</Button></div>}
    {list.isSuccess && rows.length === 0 && <section className="rounded-lg border border-dashed p-8 text-center">
      <h2 className="text-lg font-semibold">{t("forms.mine.empty.title")}</h2>
      <Link to="/requests" className="text-sm underline">{t("forms.mine.empty.link")}</Link></section>}
    {rows.length > 0 && <div role="group" aria-label={t("forms.mine.filterLabel")} className="flex flex-wrap gap-2">
      {FILTERS.map((value) => <Button key={value} type="button" size="sm" variant={filter === value ? "default" : "outline"}
        aria-pressed={filter === value} onClick={() => setFilter(value)}>{t(`forms.mine.filter.${value}`)} ({counts[value]})</Button>)}
    </div>}
    {rows.length > 0 && shown.length === 0 && <p role="status" className="text-sm text-muted-foreground">{t("forms.mine.filter.empty")}</p>}
    {shown.length > 0 && <ul aria-label={t("forms.mine.list")} className="divide-y">
      {shown.map((row) => <li key={row.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
        <div className="min-w-0">
          <Link to="/tasks/$itemId" params={{ itemId: row.item_id }} className="font-medium underline-offset-2 hover:underline">{row.title}</Link>
          <p className="text-sm text-muted-foreground">{t("forms.mine.form", { name: row.form_name })}</p>
          <p className="text-xs text-muted-foreground">{t("forms.mine.submitted", { date: formatDate(row.created_at, me) })} · {t("forms.mine.updated", { date: formatDate(row.updated_at, me) })}</p>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">{t(`forms.mine.group.${row.status.group}`)}</span>
          <span className="inline-flex items-center gap-2 rounded-full border px-2.5 py-0.5 text-sm">
            <span aria-hidden className={`size-2 rounded-full ${statusDotClass(row.status.color)}`} />{row.status.name}</span>
        </div>
      </li>)}
    </ul>}
    {list.hasNextPage && <Button variant="outline" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}>{t("forms.mine.more")}</Button>}
  </main>;
}
