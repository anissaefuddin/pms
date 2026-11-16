import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { MoreHorizontal } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { buildInput, draftFromForm, type Form, type FormSummary } from "@/lib/forms";
import { ApiError } from "@/lib/api";
import { ConfigureEmailsDialog } from "@/components/configure-emails-dialog";
import { formatDate, useProfile } from "@/lib/profile";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { FormAccessDialog } from "@/components/form-access-dialog";
import { AdminStatus } from "@/components/admin/admin-ui";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";

const publicUrl = (token: string) => new URL(`/f/${token}`, window.location.origin).href;
const fillUrl = (id: string) => new URL(`/requests/${id}`, window.location.origin).href;

/** Every request form the signed-in person can manage: the list, its links, and the row actions. Shared by Settings and /forms. */
export function FormManagePanel() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const profile = useProfile();
  const list = useQuery({
    queryKey: ["forms", "manage"], retry: false,
    queryFn: async () => (await api<{ items: FormSummary[] }>("/api/v1/forms")).items,
  });
  const [filter, setFilter] = useState("");
  const [sharing, setSharing] = useState<FormSummary>();
  const [deleting, setDeleting] = useState<FormSummary>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const rows = (list.data ?? []).filter((f) => f.name.toLowerCase().includes(filter.trim().toLowerCase()));
  const [emailing, setEmailing] = useState<string>();

  /** Publish or move back to draft from the list; the builder shows the same status. */
  async function setPublished(form: FormSummary) {
    try {
      await api(`/api/v1/forms/${form.id}/${form.status === "published" ? "unpublish" : "publish"}`, { method: "POST" });
      toast.success(t(form.status === "published" ? "forms.builder.unpublished" : "forms.builder.published"));
      await qc.invalidateQueries({ queryKey: ["forms"] });
    } catch (err) { toast.error((err as Error).message); }
  }

  /** A copy starts as a draft in the same Space, with the same questions and routing. Public links are not copied. */
  async function duplicate(form: FormSummary) {
    try {
      const source = await api<Form>(`/api/v1/forms/${form.id}`);
      const name = t("forms.list.copyName", { name: form.name }).slice(0, 120);
      await api(`/api/v1/spaces/${form.space_id}/forms`, { method: "POST", body: JSON.stringify(buildInput({ ...draftFromForm(source), name })) });
      toast.success(t("forms.list.duplicated"));
      await qc.invalidateQueries({ queryKey: ["forms"] });
    } catch (err) {
      toast.error(err instanceof ApiError && err.status === 409 ? t("forms.list.duplicateTaken") : (err as Error).message);
    }
  }

  async function remove() {
    if (!deleting) return;
    setBusy(true); setError("");
    try {
      await api(`/api/v1/forms/${deleting.id}`, { method: "DELETE" });
      setDeleting(undefined);
      toast.success(t("forms.list.deleted"));
      await qc.invalidateQueries({ queryKey: ["forms"] });
    } catch (err) { setError((err as Error).message); }
    finally { setBusy(false); }
  }

  async function copy(url: string) {
    try { await navigator.clipboard.writeText(url); toast.success(t("forms.manage.copied")); }
    catch { toast.error(t("forms.manage.copyFailed")); }
  }

  return <>
    {list.isPending && <p role="status" className="text-sm text-muted-foreground">{t("forms.list.loading")}</p>}
    {list.isError && <div><p role="alert">{t("forms.list.error")}</p><Button variant="outline" className="mt-3" onClick={() => list.refetch()}>{t("forms.list.retry")}</Button></div>}
    {list.data?.length === 0 && <section className="rounded-xl border border-dashed bg-card p-12 text-center">
      <h2 className="text-lg font-semibold">{t("forms.manage.empty.title")}</h2>
      <p className="mx-auto mt-1 max-w-md text-sm text-muted-foreground">{t("forms.manage.empty.description")}</p>
    </section>}
    {!!list.data?.length && <>
      <label className="block max-w-sm text-sm"><span className="sr-only">{t("forms.manage.filter")}</span>
        <Input type="search" placeholder={t("forms.manage.filter")} value={filter} onChange={(e) => setFilter(e.target.value)} /></label>
      {rows.length === 0 ? <p role="status" className="text-sm text-muted-foreground">{t("forms.manage.noMatch")}</p> :
      <div className="relative overflow-x-auto"><table className="w-full text-left text-sm">
        <thead><tr className="border-b">
          <th scope="col" className="py-2 pr-4">{t("forms.list.name")}</th>
          <th scope="col" className="py-2 pr-4">{t("forms.manage.space")}</th>
          <th scope="col" className="py-2 pr-4">{t("forms.manage.access")}</th>
          <th scope="col" className="py-2 pr-4">{t("forms.manage.status")}</th>
          <th scope="col" className="py-2 pr-4">{t("forms.manage.link")}</th>
          <th scope="col" className="py-2 pr-4">{t("forms.manage.modified")}</th>
          <th scope="col" className="py-2"><span className="sr-only">{t("forms.list.actions")}</span></th>
        </tr></thead>
        <tbody>{rows.map((form) => {
          const url = fillUrl(form.id);
          return <tr key={form.id} className="border-b align-top">
            <th scope="row" className="py-2 pr-4 font-medium">
              <Link to="/spaces/$spaceId/forms/$formId" params={{ spaceId: form.space_id, formId: form.id }} className="underline">{form.name}</Link></th>
            <td className="py-2 pr-4"><Link to="/spaces/$spaceId" params={{ spaceId: form.space_id }} className="underline">{form.space_name}</Link></td>
            <td className="py-2 pr-4"><span className="rounded-full border px-2 py-0.5 text-xs">{t(`forms.access.${form.access}`)}</span></td>
            <td className="py-2 pr-4"><AdminStatus tone={form.status === "published" ? "success" : "neutral"}
              label={t(form.status === "published" ? "forms.builder.statusPublished" : "forms.builder.statusDraft")} /></td>
            <td className="space-y-1 py-2 pr-4">{form.access === "internal" && <span className="flex items-center gap-2"><a href={url} className="whitespace-nowrap underline">{t("forms.manage.openInternal")}</a>
                <Button size="sm" variant="outline" aria-label={t("forms.manage.copyNamed", { name: form.name })} onClick={() => void copy(url)}>{t("forms.manage.copy")}</Button></span>}
              {form.public_token && <span className="flex items-center gap-2"><a href={publicUrl(form.public_token)} className="whitespace-nowrap underline">{t("forms.manage.openPublic")}</a>
                <Button size="sm" variant="outline" aria-label={t("forms.manage.copyPublicNamed", { name: form.name })} onClick={() => void copy(publicUrl(form.public_token!))}>{t("forms.manage.copyPublic")}</Button></span>}
              {form.access !== "internal" && !form.public_token && <span aria-label={t("forms.manage.noLink")}>—</span>}</td>
            <td className="py-2 pr-4">{form.updated_by.name}<span className="block text-muted-foreground">{formatDate(form.updated_at, profile)}</span></td>
            <td className="py-2">
              <DropdownMenu>
                <DropdownMenuTrigger render={<Button size="sm" variant="outline" aria-label={t("forms.manage.actionsNamed", { name: form.name })} />}><MoreHorizontal /></DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem render={<Link to="/spaces/$spaceId/forms/$formId" params={{ spaceId: form.space_id, formId: form.id }} />}>{t("forms.list.edit")}</DropdownMenuItem>
                  <DropdownMenuItem onClick={() => setSharing(form)}>{t("forms.access.menu")}</DropdownMenuItem>
                  <DropdownMenuItem onClick={() => void setPublished(form)}>{t(form.status === "published" ? "forms.list.unpublish" : "forms.list.publish")}</DropdownMenuItem>
                  <DropdownMenuItem onClick={() => setEmailing(form.id)}>{t("forms.list.configureEmails")}</DropdownMenuItem>
                  <DropdownMenuItem onClick={() => void duplicate(form)}>{t("forms.list.duplicate")}</DropdownMenuItem>
                  {form.access === "internal" && <DropdownMenuItem onClick={() => void copy(url)}>{t("forms.list.copyLink")}</DropdownMenuItem>}
                  <DropdownMenuItem onClick={() => { setError(""); setDeleting(form); }}>{t("forms.list.delete")}</DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </td>
          </tr>;
        })}</tbody>
      </table></div>}
    </>}
    <FormAccessDialog form={sharing} onOpenChange={(open) => !open && setSharing(undefined)} />
    <ConfigureEmailsDialog open={!!emailing} initialFormId={emailing} onOpenChange={(open) => !open && setEmailing(undefined)} />
    <ConfirmDialog open={!!deleting} onOpenChange={(open) => !open && setDeleting(undefined)} busy={busy} error={error}
      title={t("forms.list.deleteTitle")} description={t("forms.list.deleteDescription", { name: deleting?.name ?? "" })}
      confirmLabel={t("forms.list.deleteConfirm")} cancelLabel={t("forms.list.cancel")} onConfirm={() => void remove()} />
  </>;
}
