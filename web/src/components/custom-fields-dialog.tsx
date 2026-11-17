import { useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowDown, ArrowUp } from "lucide-react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { FormError } from "@/components/auth/form-error";
import {
  CustomFieldFields, FieldImpactList, draftFrom, draftPayload, type Field, type FieldDraft, type Impact,
} from "@/components/custom-field-form";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";

/** spaceId null manages the Workspace's custom fields (administrators only). */
export function CustomFieldsDialog({ open, onOpenChange, spaceId }: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  spaceId: string | null;
}) {
  const { t } = useTranslation();

  return <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-2xl">
      <div className="space-y-1.5 pr-8">
        <DialogTitle>{spaceId ? t("spaces.customFields.title") : t("settings.itemTypes.workspaceFields")}</DialogTitle>
        <DialogDescription>{spaceId ? t("spaces.customFields.description") : t("settings.itemTypes.workspaceFieldsHint")}</DialogDescription>
      </div>
      {open && <CustomFields key={spaceId ?? "workspace"} spaceId={spaceId} />}
    </DialogContent>
  </Dialog>;
}

function CustomFields({ spaceId }: { spaceId: string | null }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const path = spaceId ? `/api/v1/spaces/${spaceId}/custom-fields` : "/api/v1/custom-fields";
  const key = ["custom-fields", spaceId ?? "workspace"];
  const fields = useQuery({ queryKey: key, queryFn: () => api<{ items: Field[] }>(path) });
  const [draft, setDraft] = useState<FieldDraft | null>(null);
  const [archiving, setArchiving] = useState<{ field: Field; impact?: Impact } | null>(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  async function change(resource: string, method: "POST" | "PATCH" | "DELETE" | "PUT", value?: unknown) {
    setError("");
    setPending(true);
    try {
      await api(resource, { method, body: value === undefined ? undefined : JSON.stringify(value) });
      setDraft(null);
      await client.invalidateQueries({ queryKey: key });
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setPending(false);
    }
  }

  function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!draft || pending) return;
    const built = draftPayload(draft);
    if (built.error || !built.body) {
      setError(t(built.error ?? "spaces.customFields.nameRequired"));
      return;
    }
    if (draft.id) void change(`${path}/${draft.id}`, "PATCH", built.body);
    else void change(path, "POST", { ...built.body, type: draft.type });
  }

  /** Archive asks first, with what the field is used by; the counts come from the server. */
  async function startArchive(field: Field) {
    setError("");
    setArchiving({ field });
    try {
      setArchiving({ field, impact: await api<Impact>(`${path}/${field.id}/impact`) });
    } catch (err) {
      setArchiving(null);
      setError((err as Error).message);
    }
  }

  const rows = fields.data?.items ?? [];
  const actionsDisabled = pending || draft !== null || fields.isFetching || archiving !== null;

  return <div className="space-y-4">
    {fields.isPending && <p role="status">{t("spaces.customFields.loading")}</p>}
    {fields.isError && <div className="flex items-center justify-between gap-2">
      <FormError message={t("spaces.customFields.loadError")} />
      <Button variant="outline" size="sm" onClick={() => void fields.refetch()}>{t("spaces.page.retry")}</Button>
    </div>}
    {fields.isSuccess && rows.length === 0 && !draft && <p className="text-muted-foreground">{t("spaces.customFields.empty")}</p>}
    <ul className="divide-y">
      {rows.map((field, index) => <li key={field.id} className="space-y-2 py-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <p className="break-words font-medium">{field.name}</p>
            <p className="text-muted-foreground">{t(`spaces.customFields.type.${field.type}`)}</p>
          </div>
          {field.archived_at ? <div className="flex flex-wrap items-center gap-2">
            <span>{t("spaces.customFields.archived")}</span>
            <Button variant="outline" size="sm" disabled={actionsDisabled}
              onClick={() => void change(`${path}/${field.id}/restore`, "POST")}>{t("spaces.customFields.restore")}</Button>
          </div> : <div className="flex flex-wrap items-center gap-1">
            <Button variant="ghost" size="icon-sm" disabled={actionsDisabled || index === 0}
              aria-label={t("spaces.customFields.moveUp", { name: field.name })}
              onClick={() => void change(`${path}/${field.id}`, "PATCH", { position: rows[index - 1].position })}>
              <ArrowUp />
            </Button>
            <Button variant="ghost" size="icon-sm" disabled={actionsDisabled || index === rows.length - 1}
              aria-label={t("spaces.customFields.moveDown", { name: field.name })}
              onClick={() => void change(`${path}/${field.id}`, "PATCH", { position: rows[index + 1].position })}>
              <ArrowDown />
            </Button>
            <Button variant="outline" size="sm" disabled={actionsDisabled}
              onClick={() => { setError(""); setDraft(draftFrom(field)); }}>{t("spaces.customFields.edit")}</Button>
            <Button variant="ghost" size="sm" disabled={actionsDisabled}
              onClick={() => void startArchive(field)}>{t("spaces.customFields.archive")}</Button>
          </div>}
        </div>
        <div className="flex flex-wrap gap-x-3 gap-y-1">
          {field.options.map((option) => <span key={option.id} className="inline-flex items-center gap-1.5 break-all">
            <span className="size-3 shrink-0 rounded-full border" style={{ backgroundColor: option.color }} aria-hidden="true" />
            {option.label}
          </span>)}
        </div>
      </li>)}
    </ul>
    {archiving && <div role="alertdialog" aria-labelledby="archive-title" className="space-y-2 rounded-lg border p-3">
      <p id="archive-title" className="font-medium">{t("spaces.customFields.archiveTitle", { name: archiving.field.name })}</p>
      {!archiving.impact ? <p role="status" className="text-muted-foreground">{t("spaces.customFields.usage.loading")}</p> : <>
        <p className="text-muted-foreground">{t("spaces.customFields.archiveHint")}</p>
        <FieldImpactList impact={archiving.impact} />
      </>}
      <div className="flex justify-end gap-2">
        <Button variant="ghost" size="sm" disabled={pending} onClick={() => setArchiving(null)}>{t("spaces.customFields.cancel")}</Button>
        <Button variant="outline" size="sm" disabled={pending || !archiving.impact || archiving.impact.formulas.length > 0}
          onClick={() => { const field = archiving.field; setArchiving(null); void change(`${path}/${field.id}`, "DELETE"); }}>
          {t("spaces.customFields.archiveConfirm")}</Button>
      </div>
    </div>}
    {draft ? <form className="space-y-3 rounded-lg border p-3" onSubmit={save}>
      <CustomFieldFields draft={draft} onChange={setDraft} pending={pending} typeLocked={!!draft.id} />
      <FormError message={error} />
      <div className="flex flex-wrap justify-end gap-2">
        <Button type="button" variant="outline" disabled={pending} onClick={() => { setDraft(null); setError(""); }}>{t("spaces.form.cancel")}</Button>
        <Button type="submit" disabled={pending}>{t("spaces.form.edit.submit")}</Button>
      </div>
    </form> : <>
      <FormError message={error} />
      <Button disabled={actionsDisabled || !fields.isSuccess} onClick={() => { setError(""); setDraft(draftFrom()); }}>{t("spaces.customFields.add")}</Button>
    </>}
  </div>;
}
