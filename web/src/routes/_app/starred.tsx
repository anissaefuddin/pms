import { Link, createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { useStars } from "@/components/star-button";

export const Route = createFileRoute("/_app/starred")({ component: Starred });

function Starred() {
  const { t } = useTranslation();
  const stars = useStars();
  return <main className="space-y-5 p-8">
    <header className="border-b pb-5">
      <h1 className="text-2xl font-semibold">{t("starred.title")}</h1>
      <p className="text-sm text-muted-foreground">{t("starred.description")}</p>
    </header>
    {stars.isPending && <p role="status" className="text-sm text-muted-foreground">{t("starred.loading")}</p>}
    {stars.isError && <div><p role="alert">{t("starred.error")}</p><Button variant="outline" className="mt-3" onClick={() => stars.refetch()}>{t("starred.retry")}</Button></div>}
    {stars.data?.length === 0 && <p role="status" className="rounded-xl border border-dashed bg-card p-12 text-center text-sm text-muted-foreground">{t("starred.empty")}</p>}
    {!!stars.data?.length && <ul className="divide-y rounded-xl border bg-card">{stars.data.map((item) => <li key={item.id} className="flex items-baseline justify-between gap-3 p-3 text-sm">
      <Link to="/items/$itemId" params={{ itemId: item.id }} className="min-w-0 truncate underline">{item.title}</Link>
      <span className="shrink-0 text-muted-foreground">{t(`starred.type.${item.type}`)}</span></li>)}</ul>}
  </main>;
}
