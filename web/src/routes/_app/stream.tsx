import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useInfiniteQuery } from "@tanstack/react-query";
import { Trans, useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { formatDateTime, useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";
import { groupBy } from "@/lib/todo-groups";

type Entry = components["schemas"]["StreamEntry"];
type Page = components["schemas"]["StreamPage"];
const filters = ["everything", "followed", "assigned", "conversations"] as const;
type Filter = (typeof filters)[number];

export const Route = createFileRoute("/_app/stream")({
  validateSearch: (search: Record<string, unknown>): { filter?: Filter; hideAuto?: boolean } => ({
    filter: filters.find((value) => value === search.filter),
    hideAuto: search.hideAuto === true ? true : undefined,
  }),
  component: Stream,
});

function Stream() {
  const { t } = useTranslation();
  const profile = useProfile();
  const { filter = "everything", hideAuto = false } = Route.useSearch();
  const navigate = useNavigate({ from: "/stream" });
  const query = useInfiniteQuery({
    queryKey: ["stream", filter, hideAuto],
    initialPageParam: "",
    queryFn: ({ pageParam, signal }) => {
      const params = new URLSearchParams({ filter, hide_auto: String(hideAuto), limit: "30" });
      if (pageParam) params.set("cursor", pageParam);
      return api<Page>(`/api/v1/stream?${params}`, { signal });
    },
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  const entries = query.data?.pages.flatMap((page) => page.items) ?? [];
  // One card per item, entries newest first inside it (the reference Stream groups updates by item).
  const cards = groupBy(entries, (entry) => entry.item.id).map((group) => group.rows);
  return <main className="grid gap-6 p-6 lg:grid-cols-[minmax(0,1fr)_16rem]">
    <div className="mx-auto w-full max-w-3xl space-y-4">
      <h1 className="workspace-title">{t("nav.stream.title")}</h1>
      {query.isPending && <p role="status" className="text-muted-foreground">{t("items.loading")}</p>}
      {query.isError && <div><p role="alert">{t("nav.stream.error")}</p>
        <Button variant="link" className="px-0" onClick={() => void query.refetch()}>{t("nav.stream.retry")}</Button></div>}
      {query.isSuccess && !entries.length && <p role="status" className="text-muted-foreground">{t("nav.stream.empty")}</p>}
      {cards.map((group) => <section key={group[0]!.item.id} className="divide-y rounded-lg border bg-card px-4">
        {group.map((entry) => <div key={`${entry.kind}:${entry.id}`} className="space-y-1 py-3 text-sm">
          <Line entry={entry} />
          <p className="text-xs text-muted-foreground">{profile ? formatDateTime(entry.created_at, profile) : entry.created_at}</p>
        </div>)}
      </section>)}
      {query.hasNextPage && <Button variant="outline" disabled={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>
        {t("nav.stream.more")}
      </Button>}
    </div>
    <aside aria-label={t("nav.stream.filtersTitle")} className="space-y-3 lg:border-l lg:pl-4">
      <h2 className="text-base font-semibold">{t("nav.stream.filtersTitle")}</h2>
      <div className="space-y-2 border-b pb-3" role="radiogroup" aria-label={t("nav.stream.filter")}>
        {filters.map((value) => <label key={value} className="flex items-center gap-2 text-sm">
          <input type="radio" name="stream-filter" checked={filter === value}
            onChange={() => void navigate({ search: { filter: value, hideAuto } })} />
          {t(`nav.stream.filters.${value}`)}
        </label>)}
      </div>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={hideAuto} onChange={(event) => void navigate({ search: { filter, hideAuto: event.target.checked } })} />
        {t("nav.stream.hideAuto")}
      </label>
    </aside>
  </main>;
}

function Line({ entry }: { entry: Entry }) {
  const { t } = useTranslation();
  const link = <Link className="font-medium underline-offset-2 hover:underline" to={entry.item.type === "task" ? "/tasks/$itemId" : "/items/$itemId"}
    params={{ itemId: entry.item.id }}></Link>;
  const key = entry.kind === "comment" ? "commented" : `action.${["update", "delete", "restore", "detach", "create"].includes(entry.action) ? entry.action : "other"}`;
  const fields = Object.keys(entry.changes);
  return <>
    <p><Trans i18nKey={`nav.stream.${key}`} values={{ actor: entry.actor.name, title: entry.item.title }} components={{ item: link }} /></p>
    {entry.snippet && <p className="text-muted-foreground">{entry.snippet}</p>}
    {entry.kind === "activity" && fields.length > 0 && <p className="text-xs text-muted-foreground">{t("nav.stream.changes", { fields: fields.join(", ") })}</p>}
  </>;
}
