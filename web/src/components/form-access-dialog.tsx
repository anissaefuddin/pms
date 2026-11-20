import { useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import type { components } from "@/api/schema";

type Access = components["schemas"]["FormAccess"];
const CHOICES: Access[] = ["private", "internal"];

type FormAccessTarget = { id: string; name: string; access: Access; public_token?: string | null };

/** Who can fill a form: only managers, everyone signed in, and the public link. Used by the Share dialog and the builder. */
export function AccessForm({ form, onSaved, onCancel }: { form: FormAccessTarget; onSaved?: () => void; onCancel?: () => void }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [choice, setChoice] = useState<Access>(form.access);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function save() {
    setBusy(true); setError("");
    try {
      await api(`/api/v1/forms/${form.id}/access`, { method: "PUT", body: JSON.stringify({ access: choice }) });
      toast.success(t("forms.access.saved"));
      onSaved?.();
      await qc.invalidateQueries({ queryKey: ["forms"] });
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) setError(t("forms.access.forbidden"));
      else if (err instanceof ApiError && err.status === 404) {
        setError(t("forms.access.gone"));
        await qc.invalidateQueries({ queryKey: ["forms"] });
      } else setError((err as Error).message);
    } finally { setBusy(false); }
  }

  return <div className="space-y-4">
    <fieldset className="space-y-3">
      <legend className="sr-only">{t("forms.access.legend")}</legend>
      {CHOICES.map((value) => <label key={value} className="flex items-start gap-3 text-sm">
        <input type="radio" name="form-access" className="mt-1" checked={choice === value} onChange={() => setChoice(value)} />
        <span><span className="block font-medium">{t(`forms.access.${value}`)}</span><span className="block text-muted-foreground">{t(`forms.access.${value}Help`)}</span></span>
      </label>)}
    </fieldset>
    <PublicLink key={form.id} formId={form.id} initial={form.public_token ?? null} />
    <FormError message={error} />
    <div className="flex flex-wrap justify-end gap-2">
      {onCancel && <Button variant="outline" onClick={onCancel}>{t("forms.list.cancel")}</Button>}
      <Button disabled={busy} onClick={() => void save()}>{busy ? t("forms.builder.saving") : t("forms.access.save")}</Button>
    </div>
  </div>;
}

/** "Share form" dialog from the Manage forms list. Keyed by form so each open starts from the form's current access. */
export function FormAccessDialog({ form: current, onOpenChange }: { form?: FormAccessTarget; onOpenChange: (open: boolean) => void }) {
  const { t } = useTranslation();
  // keep the last form while the close animation runs, else the overlay is left behind
  const last = useRef(current);
  if (current) last.current = current;
  const form = last.current;
  return <Dialog open={!!current} onOpenChange={onOpenChange}>
    <DialogContent key={form?.id}>
      <DialogHeader>
        <DialogTitle>{t("forms.access.title")}</DialogTitle>
        <DialogDescription>{t("forms.access.description", { name: form?.name ?? "" })}</DialogDescription>
      </DialogHeader>
      {form && <AccessForm form={form} onSaved={() => onOpenChange(false)} onCancel={() => onOpenChange(false)} />}
    </DialogContent>
  </Dialog>;
}

/** Public link (FORM-03): create, replace (old link dies at once) or turn off. Each change is saved on its own. */
export function PublicLink({ formId, initial }: { formId?: string; initial: string | null }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [token, setToken] = useState(initial);
  const [confirming, setConfirming] = useState<"rotate" | "revoke" | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const link = token ? `${window.location.origin}/f/${token}` : "";

  async function change(action: "rotate" | "revoke") {
    if (!formId) return;
    setBusy(true); setError("");
    try {
      const result = await api<{ public_token?: string | null }>(`/api/v1/forms/${formId}/public-link`, { method: action === "rotate" ? "POST" : "DELETE" });
      setToken(result.public_token ?? null);
      setConfirming(null);
      toast.success(t(action === "revoke" ? "forms.access.publicRevoked" : token ? "forms.access.publicRotated" : "forms.access.publicCreated"));
      await qc.invalidateQueries({ queryKey: ["forms"] });
    } catch (err) {
      setError(err instanceof ApiError && err.status === 403 ? t("forms.access.forbidden") : (err as Error).message);
    } finally { setBusy(false); }
  }

  async function copy() {
    try {
      await navigator.clipboard.writeText(link);
      toast.success(t("forms.access.publicCopied"));
    } catch { /* clipboard blocked: the address stays selectable in the field */ }
  }

  return <section className="space-y-2 border-t pt-3" aria-labelledby="public-link-title">
    <h3 id="public-link-title" className="text-sm font-medium">{t("forms.access.publicTitle")}</h3>
    <p className="text-sm text-muted-foreground">{t("forms.access.publicHelp")}</p>
    {token ? <>
      <div className="flex gap-2">
        <input readOnly aria-label={t("forms.access.publicLinkLabel")} value={link} onFocus={(event) => event.currentTarget.select()}
          className="h-8 min-w-0 flex-1 rounded-lg border border-input bg-transparent px-2.5 text-sm" />
        <Button type="button" size="sm" variant="outline" onClick={() => void copy()}>{t("forms.access.publicCopy")}</Button>
      </div>
      {confirming ? <div role="group" aria-label={t("forms.access.publicTitle")} className="space-y-2 rounded-lg border p-3 text-sm">
        <p role="alert">{t(confirming === "rotate" ? "forms.access.publicRotateConfirm" : "forms.access.publicRevokeConfirm")}</p>
        <div className="flex gap-2">
          <Button type="button" size="sm" variant="destructive" disabled={busy} onClick={() => void change(confirming)}>
            {t(confirming === "rotate" ? "forms.access.publicRotate" : "forms.access.publicRevoke")}</Button>
          <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => setConfirming(null)}>{t("forms.list.cancel")}</Button>
        </div>
      </div> : <div className="flex gap-2">
        <Button type="button" size="sm" variant="outline" onClick={() => setConfirming("rotate")}>{t("forms.access.publicRotate")}</Button>
        <Button type="button" size="sm" variant="outline" onClick={() => setConfirming("revoke")}>{t("forms.access.publicRevoke")}</Button>
      </div>}
    </> : <>
      <p className="text-sm">{t("forms.access.publicNone")}</p>
      <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void change("rotate")}>{t("forms.access.publicCreate")}</Button>
    </>}
    <FormError message={error} />
  </section>;
}
