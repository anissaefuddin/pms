import { Link, createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { ClipboardList } from "lucide-react";
import { api } from "@/lib/api";
import { buttonVariants } from "@/components/ui/button";
import { Button } from "@/components/ui/button";
import { RequestTabs } from "@/components/request-tabs";

export const Route = createFileRoute("/_app/requests/")({ component: Requests });

type Available = { id: string; name: string; description: string };

function Requests() {
  const { t } = useTranslation();
  const list = useQuery({ queryKey: ["forms", "available"], retry: false,
    queryFn: async () => (await api<{ items: Available[] }>("/api/v1/forms/available")).items });
  return <main className="space-y-5 p-8">
    <RequestTabs />
    <header className="border-b pb-5">
      <h1 className="workspace-title">{t("forms.requests.title")}</h1>
      <p className="text-sm text-muted-foreground">{t("forms.requests.description")}</p>
    </header>
    {list.isPending && <p role="status">{t("forms.requests.loading")}</p>}
    {list.isError && <div role="alert"><p>{t("forms.requests.loadError")}</p>
      <Button variant="link" className="px-0" onClick={() => void list.refetch()}>{t("forms.requests.retry")}</Button></div>}
    {list.data?.length === 0 && <section className="rounded-lg border border-dashed p-8 text-center">
      <h2 className="text-lg font-semibold">{t("forms.requests.empty.title")}</h2>
      <p className="text-sm text-muted-foreground">{t("forms.requests.empty.description")}</p></section>}
    {/* A card per form: what it is for, then the one action. */}
    <ul className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      {list.data?.map((form) => <li key={form.id} className="flex flex-col gap-4 rounded-lg border p-4">
        <div className="flex items-start gap-3">
          <span aria-hidden="true" className="grid size-10 shrink-0 place-items-center rounded-lg bg-muted text-muted-foreground"><ClipboardList className="size-5" /></span>
          <div className="min-w-0 space-y-1">
            <p className="font-medium">{form.name}</p>
            {form.description && <p className="line-clamp-3 text-sm text-muted-foreground">{form.description}</p>}
          </div>
        </div>
        <Link to="/requests/$formId" params={{ formId: form.id }} className={`${buttonVariants({ variant: "outline" })} mt-auto`}
          aria-label={t("forms.requests.startNamed", { name: form.name })}>{t("forms.requests.start")}</Link>
      </li>)}
    </ul>
  </main>;
}
