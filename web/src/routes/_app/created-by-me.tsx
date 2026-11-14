import { Link, createFileRoute } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import type { CurrentUser } from "@/lib/api";
import { itemKeys, listItems } from "@/lib/items";
import { useSpaces } from "@/lib/space-queries";

export const Route = createFileRoute("/_app/created-by-me")({ component: CreatedByMe });

function CreatedByMe() {
  const { t } = useTranslation();
  const me = useQueryClient().getQueryData<CurrentUser>(["me"]);
  const spaces = useSpaces();
  // shortcut: no server filter on creator exists, so this reads each Space's top level and filters here; upgrade with a created_by query parameter
  const query = useQuery({
    queryKey: [...itemKeys.all, "created-by-me", spaces.data?.map((space) => space.id)], enabled: !!spaces.data && !!me,
    queryFn: async () => (await Promise.all(spaces.data!.map((space) => listItems(`/api/v1/spaces/${space.id}/items`))))
      .flat().filter((item) => item.created_by?.id === me?.id),
  });
  return <main className="space-y-5 p-8">
    <header className="border-b pb-5">
      <h1 className="workspace-title">{t("nav.created.title")}</h1>
      <p className="text-sm text-muted-foreground">{t("nav.created.description")}</p>
    </header>
    {query.isPending && <p role="status" className="text-sm text-muted-foreground">{t("items.loading")}</p>}
    {query.isError && <div><p role="alert">{t("nav.created.error")}</p><Button variant="outline" className="mt-3" onClick={() => void query.refetch()}>{t("nav.created.retry")}</Button></div>}
    {query.data?.length === 0 && <p role="status" className="rounded-xl border border-dashed bg-card p-12 text-center text-sm text-muted-foreground">{t("nav.created.empty")}</p>}
    {!!query.data?.length && <ul className="divide-y rounded-xl border bg-card">{query.data.map((item) => <li key={item.id} className="p-3 text-sm">
      <Link to="/tasks/$itemId" params={{ itemId: item.id }} className="underline">{item.title}</Link></li>)}</ul>}
  </main>;
}
