import { Link, createFileRoute } from "@tanstack/react-router";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { itemKeys } from "@/lib/items";
import { TopbarTitle } from "@/components/topbar-title";

export const Route = createFileRoute("/_app/created-by-me")({ component: CreatedByMe });

type Page = { items: { id: string; title: string; type: string }[]; next_cursor: string | null };

function CreatedByMe() {
  const { t } = useTranslation();
  const query = useInfiniteQuery({
    queryKey: [...itemKeys.all, "created-by-me"],
    initialPageParam: "",
    queryFn: ({ pageParam }) => api<Page>(`/api/v1/me/created${pageParam ? `?cursor=${pageParam}` : ""}`),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
  const items = query.data?.pages.flatMap((page) => page.items) ?? [];
  return <main className="space-y-5 p-8">
    <TopbarTitle>{t("nav.created.title")}</TopbarTitle>
    <header className="border-b pb-5">
      <p className="text-sm text-muted-foreground">{t("nav.created.description")}</p>
    </header>
    {query.isPending && <p role="status" className="text-sm text-muted-foreground">{t("items.loading")}</p>}
    {query.isError && <div><p role="alert">{t("nav.created.error")}</p><Button variant="outline" className="mt-3" onClick={() => void query.refetch()}>{t("nav.created.retry")}</Button></div>}
    {query.isSuccess && !items.length && !query.hasNextPage && <p role="status" className="rounded-xl border border-dashed bg-card p-12 text-center text-sm text-muted-foreground">{t("nav.created.empty")}</p>}
    {!!items.length && <ul className="divide-y rounded-xl border bg-card">{items.map((item) => <li key={item.id} className="p-3 text-sm">
      <Link to="/tasks/$itemId" params={{ itemId: item.id }} className="underline">{item.title}</Link></li>)}</ul>}
    {query.hasNextPage && <Button variant="outline" disabled={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>{t("items.activity.more")}</Button>}
  </main>;
}
