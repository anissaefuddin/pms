import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useInfiniteQuery } from "@tanstack/react-query";
import { Trans, useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { formatDateTime, useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";

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
  return <main className="mx-auto w-full max-w-3xl space-y-4 p-6">
    <h1 className="text-xl font-semibold">{t("nav.stream.title")}</h1>
    <div className="flex flex-wrap items-center gap-2">
      <div className="flex flex-wrap gap-1" role="group" aria-label={t("nav.stream.filter")}>
        {filters.map((value) => <Button key={value} type="button" size="sm" variant={filter === value ? "secondary" : "ghost"}
          aria-pressed={filter === value} onClick={() => void navigate({ search: { filter: value, hideAuto } })}>
          {t(`nav.stream.filters.${value}`)}
        </Button>)}
      </div>
      <label className="ml-auto flex items-center gap-2 text-sm">
        <input type="checkbox" checked={hideAuto} onChange={(event) => void navigate({ search: { filter, hideAuto: event.target.checked } })} />
        {t("nav.stream.hideAuto")}
      </label>
    </div>
    {query.isPending && <p role="status" className="text-muted-foreground">{t("items.loading")}</p>}
    {query.isError && <div><p role="alert">{t("nav.stream.error")}</p>
      <Button variant="link" className="px-0" onClick={() => void query.refetch()}>{t("nav.stream.retry")}</Button></div>}
    {query.isSuccess && !entries.length && <p role="status" className="text-muted-foreground">{t("nav.stream.empty")}</p>}
    <ul className="divide-y">
      {entries.map((entry) => <li key={`${entry.kind}:${entry.id}`} className="space-y-1 py-3 text-sm">
        <Line entry={entry} />
        <p className="text-xs text-muted-foreground">{profile ? formatDateTime(entry.created_at, profile) : entry.created_at}</p>
      </li>)}
    </ul>
    {query.hasNextPage && <Button variant="outline" disabled={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>
      {t("nav.stream.more")}
    </Button>}
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
