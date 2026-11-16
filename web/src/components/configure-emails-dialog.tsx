import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import { buildInput, draftFromForm, formKeys, mapErrors, type Form, type FormDraft, type FormSummary } from "@/lib/forms";
import { FormError } from "@/components/auth/form-error";
import { EmailFields, EmailPreview } from "@/components/form-routing-settings";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";

const select = "block w-full rounded-lg border border-input bg-background px-2 py-1.5 text-sm";

/**
 * RF-10: the confirmation email for one form, separate from personal Email preferences. Wording on the left, the
 * requester's view on the right. Saved as a form change, so an open builder sees it as a new version.
 */
export function ConfigureEmailsDialog({ open, onOpenChange, initialFormId }: { open: boolean; onOpenChange: (open: boolean) => void; initialFormId?: string }) {
  const { t } = useTranslation();
  const forms = useQuery({ queryKey: ["forms", "manage"], enabled: open, retry: false,
    queryFn: async () => (await api<{ items: FormSummary[] }>("/api/v1/forms")).items });
  const [picked, setPicked] = useState<string>();
  const formId = picked ?? initialFormId ?? forms.data?.[0]?.id;
  const current = useQuery({ queryKey: formKeys.one(formId ?? ""), enabled: open && !!formId, retry: false, staleTime: 0,
    queryFn: () => api<Form>(`/api/v1/forms/${formId}`) });

  return <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent className="sm:max-w-4xl">
      <DialogHeader>
        <DialogTitle>{t("forms.email.configureTitle")}</DialogTitle>
        <DialogDescription>{t("forms.email.configureHint")}</DialogDescription>
      </DialogHeader>
      {forms.isSuccess && forms.data.length === 0 && <p role="status" className="text-sm text-muted-foreground">{t("forms.email.noForms")}</p>}
      {!!forms.data?.length && <label className="block max-w-sm space-y-1 text-sm"><span>{t("forms.email.formLabel")}</span>
        <select className={select} value={formId ?? ""} onChange={(e) => setPicked(e.target.value)}>
          {forms.data.map((form) => <option key={form.id} value={form.id}>{form.name}</option>)}</select></label>}
      {current.isError && <p role="alert" className="text-sm">{t("forms.builder.loadError")}</p>}
      {current.data && <EmailEditor key={`${current.data.id}:${current.data.version}`} form={current.data} onDone={() => onOpenChange(false)} />}
    </DialogContent>
  </Dialog>;
}

function EmailEditor({ form, onDone }: { form: Form; onDone: () => void }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [draft, setDraft] = useState<FormDraft>(() => draftFromForm(form));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  async function save() {
    setBusy(true); setErrors({}); setMessage("");
    try {
      await api<Form>(`/api/v1/forms/${form.id}`, { method: "PUT", body: JSON.stringify(buildInput(draft)), headers: { "If-Match": `"${form.version}"` } });
      toast.success(t("forms.email.saved"));
      await qc.invalidateQueries({ queryKey: ["forms"] });
      onDone();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) setMessage(t("forms.builder.conflict"));
      else if (err instanceof ApiError && err.fields.length) setErrors(mapErrors(err.fields).byControl);
      else setMessage((err as Error).message);
    } finally { setBusy(false); }
  }

  return <div className="space-y-4">
    <div className="grid gap-6 md:grid-cols-2">
      <EmailFields draft={draft} errors={errors} onChange={(patch) => setDraft((d) => ({ ...d, ...patch }))} />
      <EmailPreview draft={draft} />
    </div>
    <FormError message={message} />
    <DialogFooter>
      <Button type="button" variant="outline" onClick={onDone}>{t("forms.list.cancel")}</Button>
      <Button type="button" disabled={busy} onClick={() => void save()}>{busy ? t("forms.builder.saving") : t("forms.email.save")}</Button>
    </DialogFooter>
  </div>;
}
