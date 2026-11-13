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

/** "Share form": who can fill it. Closes and invalidates every forms query on success. */
export function FormAccessDialog({ form: current, onOpenChange }: { form?: { id: string; name: string; access: Access }; onOpenChange: (open: boolean) => void }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  // keep the last form while the close animation runs, else the overlay is left behind
  const last = useRef(current);
  if (current) last.current = current;
  const form = last.current;
  const [choice, setChoice] = useState<Access>(form?.access ?? "private");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function save() {
    if (!form) return;
    setBusy(true); setError("");
    try {
      await api(`/api/v1/forms/${form.id}/access`, { method: "PUT", body: JSON.stringify({ access: choice }) });
      toast.success(t("forms.access.saved"));
      onOpenChange(false);
      await qc.invalidateQueries({ queryKey: ["forms"] });
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) setError(t("forms.access.forbidden"));
      else if (err instanceof ApiError && err.status === 404) {
        setError(t("forms.access.gone"));
        await qc.invalidateQueries({ queryKey: ["forms"] });
      } else setError((err as Error).message);
    } finally { setBusy(false); }
  }

  return <Dialog open={!!current} onOpenChange={onOpenChange}>
    {/* keyed so the radios restart from the form's current access each time it opens */}
    <DialogContent key={form?.id}>
      <DialogHeader>
        <DialogTitle>{t("forms.access.title")}</DialogTitle>
        <DialogDescription>{t("forms.access.description", { name: form?.name ?? "" })}</DialogDescription>
      </DialogHeader>
      <fieldset className="space-y-3">
        <legend className="sr-only">{t("forms.access.legend")}</legend>
        {CHOICES.map((value) => <label key={value} className="flex items-start gap-3 text-sm">
          <input type="radio" name="form-access" className="mt-1" checked={choice === value} onChange={() => setChoice(value)} />
          <span><span className="block font-medium">{t(`forms.access.${value}`)}</span><span className="block text-muted-foreground">{t(`forms.access.${value}Help`)}</span></span>
        </label>)}
      </fieldset>
      <FormError message={error} />
      <DialogFooter>
        <Button variant="outline" onClick={() => onOpenChange(false)}>{t("forms.list.cancel")}</Button>
        <Button disabled={busy} onClick={() => void save()}>{busy ? t("forms.builder.saving") : t("forms.access.save")}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>;
}
