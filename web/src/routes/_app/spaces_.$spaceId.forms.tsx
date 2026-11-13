import { useState } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import { formKeys, type FormSummary } from "@/lib/forms";
import { formatDate, useProfile } from "@/lib/profile";
import { useSpace } from "@/lib/space-queries";
import { can } from "@/lib/spaces";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Button, buttonVariants } from "@/components/ui/button";

export const Route = createFileRoute("/_app/spaces_/$spaceId/forms")({ component: FormsList });

function FormsList() {
  const { t } = useTranslation();
  const { spaceId } = Route.useParams();
  const qc = useQueryClient();
  const profile = useProfile();
  const space = useSpace(spaceId);
  const allowed = !space.data || can(space.data, "edit");
  const list = useQuery({
    queryKey: formKeys.list(spaceId), enabled: allowed, retry: false,
    queryFn: async () => (await api<{ items: FormSummary[] }>(`/api/v1/spaces/${spaceId}/forms`)).items,
  });
  const [deleting, setDeleting] = useState<FormSummary>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const forbidden = !allowed || (list.error instanceof ApiError && list.error.status === 403);

  async function remove() {
    if (!deleting) return;
    setBusy(true); setError("");
    try {
      await api(`/api/v1/forms/${deleting.id}`, { method: "DELETE" });
      setDeleting(undefined);
      toast.success(t("forms.list.deleted"));
      await qc.invalidateQueries({ queryKey: formKeys.list(spaceId) });
    } catch (err) { setError((err as Error).message); }
    finally { setBusy(false); }
  }

  return <main className="space-y-5 p-8">
    <header className="flex flex-wrap items-start justify-between gap-4 border-b pb-5">
      <div>
        <Link to="/spaces/$spaceId" params={{ spaceId }} className="text-sm text-muted-foreground underline">{t("forms.list.back", { name: space.data?.kind === "personal" ? t("spaces.personal") : space.data?.name ?? "" })}</Link>
        <h1 className="mt-1 workspace-title">{t("forms.list.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("forms.list.description")}</p>
      </div>
      {!forbidden && <div className="flex gap-2">
        <Link to="/forms" className={buttonVariants({ variant: "outline" })}>{t("forms.list.manageLink")}</Link>
        <Link to="/spaces/$spaceId/forms/$formId" params={{ spaceId, formId: "new" }} className={buttonVariants()}>{t("forms.list.new")}</Link>
      </div>}
    </header>
    {forbidden && <p role="alert">{t("forms.list.forbidden")}</p>}
    {!forbidden && list.isPending && <p role="status" className="text-sm text-muted-foreground">{t("forms.list.loading")}</p>}
    {!forbidden && list.isError && <div><p role="alert">{t("forms.list.error")}</p><Button variant="outline" className="mt-3" onClick={() => list.refetch()}>{t("forms.list.retry")}</Button></div>}
    {list.data?.length === 0 && <section className="rounded-xl border border-dashed bg-card p-12 text-center">
      <h2 className="text-lg font-semibold">{t("forms.list.empty.title")}</h2>
      <p className="mx-auto mt-1 max-w-md text-sm text-muted-foreground">{t("forms.list.empty.description")}</p>
    </section>}
    {!!list.data?.length && <table className="w-full text-left text-sm">
      <thead><tr className="border-b">
        <th scope="col" className="py-2 pr-4">{t("forms.list.name")}</th>
        <th scope="col" className="py-2 pr-4">{t("forms.list.questions")}</th>
        <th scope="col" className="py-2 pr-4">{t("forms.list.updatedBy")}</th>
        <th scope="col" className="py-2 pr-4">{t("forms.list.updatedAt")}</th>
        <th scope="col" className="py-2"><span className="sr-only">{t("forms.list.actions")}</span></th>
      </tr></thead>
      <tbody>{list.data.map((form) => <tr key={form.id} className="border-b align-top">
        <th scope="row" className="py-2 pr-4 font-medium">{form.name}{form.description && <span className="block font-normal text-muted-foreground">{form.description}</span>}</th>
        <td className="py-2 pr-4">{form.question_count}</td>
        <td className="py-2 pr-4">{form.updated_by.name}</td>
        <td className="py-2 pr-4">{formatDate(form.updated_at, profile)}</td>
        <td className="flex gap-2 py-2">
          <Link to="/spaces/$spaceId/forms/$formId" params={{ spaceId, formId: form.id }} aria-label={t("forms.list.editNamed", { name: form.name })} className={buttonVariants({ variant: "outline", size: "sm" })}>{t("forms.list.edit")}</Link>
          <Button size="sm" variant="outline" aria-label={t("forms.list.deleteNamed", { name: form.name })} onClick={() => { setError(""); setDeleting(form); }}>{t("forms.list.delete")}</Button>
        </td>
      </tr>)}</tbody>
    </table>}
    <ConfirmDialog open={!!deleting} onOpenChange={(open) => !open && setDeleting(undefined)} busy={busy} error={error}
      title={t("forms.list.deleteTitle")} description={t("forms.list.deleteDescription", { name: deleting?.name ?? "" })}
      confirmLabel={t("forms.list.deleteConfirm")} cancelLabel={t("forms.list.cancel")} onConfirm={() => void remove()} />
  </main>;
}
