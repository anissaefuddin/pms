import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { ItemCalendar } from "@/components/item-calendar";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { itemKeys, type ItemPage } from "@/lib/items";
import { TopbarTitle } from "@/components/topbar-title";

export const Route = createFileRoute("/_app/calendars")({ component: Calendars });

function Calendars() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  // shortcut: first page (the server's default size) of My to-do, read-only; upgrade with a date-range endpoint
  const query = useQuery({ queryKey: [...itemKeys.all, "calendars"], queryFn: () => api<ItemPage>("/api/v1/me/todo?filter=all&sort=date") });
  return <main className="space-y-5 p-8">
    <TopbarTitle>{t("nav.calendars.title")}</TopbarTitle>
    <header className="border-b pb-5">
      <p className="text-sm text-muted-foreground">{t("nav.calendars.description")}</p>
    </header>
    {query.isPending && <p role="status" className="text-sm text-muted-foreground">{t("items.loading")}</p>}
    {query.isError && <div><p role="alert">{t("nav.created.error")}</p><Button variant="outline" className="mt-3" onClick={() => void query.refetch()}>{t("nav.created.retry")}</Button></div>}
    {query.data && <ItemCalendar items={query.data.items} readonly onPatch={async () => undefined}
      onOpen={(item) => void navigate({ to: "/tasks/$itemId", params: { itemId: item.id } })} />}
  </main>;
}
