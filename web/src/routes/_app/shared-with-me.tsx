import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { listItems } from "@/lib/items";
import { Button } from "@/components/ui/button";

export const Route = createFileRoute("/_app/shared-with-me")({ component: SharedItems });

function SharedItems() {
  const { t } = useTranslation();
  const items = useQuery({ queryKey: ["shared-items"], queryFn: () => listItems("/api/v1/me/shared") });
  return <main className="space-y-4 p-6">
    <h1 className="workspace-title">{t("nav.shared.label")}</h1>
    {items.isPending && <p role="status">{t("items.loading")}</p>}
    {items.isError && <div><p role="alert">{t("items.loadError")}</p>
      <Button variant="link" onClick={() => void items.refetch()}>{t("items.retry")}</Button></div>}
    {items.data?.length === 0 && <p>{t("nav.shared.empty")}</p>}
    <ul className="divide-y">{items.data?.map((item) => <li key={item.id}>
      <Link className="block py-2 hover:underline" to={item.type === "task" ? "/tasks/$itemId" : "/items/$itemId"}
        params={{ itemId: item.id }}>{item.title}</Link>
    </li>)}</ul>
  </main>;
}
