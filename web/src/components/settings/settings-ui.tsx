import { createContext, useContext, useState, type FormEvent, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api, type CurrentUser } from "@/lib/api";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";

// Shared building blocks for Personal settings pages, so each page uses the same layout, spacing and save behaviour.

/** A titled group of settings on a page. Sections after the first are separated by a rule. */
export function SettingsSection({ title, description, children, id }: { title: string; description?: string; children: ReactNode; id?: string }) {
  return <section id={id} className="scroll-mt-4 space-y-4 border-t pt-6 first:border-t-0 first:pt-0">
    <div className="space-y-1">
      <h2 className="text-base font-semibold">{title}</h2>
      {description && <p className="text-sm text-muted-foreground">{description}</p>}
    </div>
    {children}
  </section>;
}

/** One setting: label and hint on the left, the control on the right. */
export function SettingsRow({ label, hint, icon, children }: { label: string; hint?: string; icon?: ReactNode; children: ReactNode }) {
  return <div className="flex items-center justify-between gap-4 border-b py-3 last:border-b-0">
    <div className="flex min-w-0 items-center gap-3">
      {icon && <span aria-hidden="true" className="grid size-9 shrink-0 place-items-center rounded-lg bg-muted text-foreground [&_svg]:size-4">{icon}</span>}
      <div className="min-w-0">
      <p className="text-sm font-medium">{label}</p>
        {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      </div>
    </div>
    <div className="shrink-0">{children}</div>
  </div>;
}

/** Marks the enclosing batch form as changed. Choice cards are not plain inputs, so they call this themselves. */
const DirtyContext = createContext<() => void>(() => undefined);

/** Sticky footer for batch-save forms: shows whether changes are waiting, with Cancel and Save. */
export function SettingsSaveBar({ dirty, busy, onCancel }: { dirty: boolean; busy: boolean; onCancel: () => void }) {
  const { t } = useTranslation();
  return <div className="sticky bottom-0 -mx-4 flex items-center justify-between gap-3 border-t bg-background/95 px-4 py-3 backdrop-blur sm:-mx-8 sm:px-8">
    <p role="status" className="text-sm text-muted-foreground">{dirty ? t("settings.unsaved") : t("settings.allSaved")}</p>
    <div className="flex gap-2">
      <Button type="button" variant="outline" onClick={onCancel} disabled={busy || !dirty}>{t("settings.cancel")}</Button>
      <Button type="submit" disabled={busy || !dirty}>{t("settings.saveChanges")}</Button>
    </div>
  </div>;
}

/**
 * Batch-save form for profile fields. Inputs keep their values until Save or Cancel, which re-reads the profile.
 * `numberFields` are sent as numbers (or null when empty).
 */
export function ProfileSettingsForm({ profile, numberFields = [], children }: { profile: CurrentUser; numberFields?: string[]; children: ReactNode }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const values: Record<string, unknown> = Object.fromEntries(new FormData(event.currentTarget));
    for (const key of numberFields) values[key] = values[key] === "" ? null : Number(values[key]);
    setBusy(true);
    setError("");
    try {
      client.setQueryData(["me"], await api<CurrentUser>("/api/v1/me", { method: "PATCH", body: JSON.stringify(values) }));
      setDirty(false);
      setRevision((current) => current + 1);
      toast.success(t("settings.profile.saved"));
    } catch (err) {
      setError((err as Error).message);
    } finally { setBusy(false); }
  }
  function cancel() {
    setError("");
    setDirty(false);
    setRevision((current) => current + 1);
  }
  return <DirtyContext.Provider value={() => setDirty(true)}>
    <form key={`${revision}-${profile.id}`} className="max-w-3xl space-y-6" onSubmit={save} onChange={() => setDirty(true)} noValidate={false}>
      <fieldset disabled={busy} className="space-y-6">{children}</fieldset>
      <FormError message={error} />
      <SettingsSaveBar dirty={dirty} busy={busy} onCancel={cancel} />
    </form>
  </DirtyContext.Provider>;
}

export type ChoiceOption = { value: string; label: string; hint?: string; preview?: ReactNode };

/** Visual single choice (cards with a preview). The value is posted with the enclosing form via a hidden input. */
export function ChoiceCards({ name, label, options, defaultValue, columns = "sm:grid-cols-3" }: { name: string; label: string; options: ChoiceOption[]; defaultValue: string; columns?: string }) {
  const [value, setValue] = useState(defaultValue);
  const markDirty = useContext(DirtyContext);
  return <fieldset className="space-y-2">
    <legend className="mb-2 text-sm font-medium">{label}</legend>
    <input type="hidden" name={name} value={value} />
    <div className={`grid grid-cols-1 gap-3 ${columns}`}>
      {options.map((option) => {
        const selected = value === option.value;
        return <button key={option.value} type="button" aria-pressed={selected} onClick={() => { setValue(option.value); markDirty(); }}
          className={`overflow-hidden rounded-lg border text-start ${selected ? "border-primary ring-2 ring-ring" : "hover:bg-accent"}`}>
          {option.preview && <span className="block p-2" aria-hidden="true">{option.preview}</span>}
          <span className="block px-3 py-2 text-sm font-medium">{option.label}</span>
          {option.hint && <span className="block px-3 pb-2 text-xs text-muted-foreground">{option.hint}</span>}
        </button>;
      })}
    </div>
  </fieldset>;
}

/** On/off switch for settings that save immediately (no Save button). */
export function SettingsSwitch({ label, checked, onChange, disabled = false }: { label: string; checked: boolean; onChange: (next: boolean) => void; disabled?: boolean }) {
  return <button type="button" role="switch" aria-checked={checked} aria-label={label} disabled={disabled} onClick={() => onChange(!checked)}
    className={`relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors disabled:opacity-50 ${checked ? "bg-primary" : "bg-muted"}`}>
    <span className={`inline-block size-5 rounded-full bg-background shadow transition-transform ${checked ? "translate-x-5" : "translate-x-0.5"}`} aria-hidden="true" />
  </button>;
}

/** Native select inside a batch form. `optionLabel` turns a stored value into the text shown. */
export function SelectField({ name, label, value, options, optionLabel }: { name: string; label: string; value: string; options: readonly string[]; optionLabel: (value: string) => string }) {
  return <label className="space-y-1 text-sm">
    <span>{label}</span>
    <select name={name} defaultValue={value} className="block h-9 w-full rounded-md border border-input bg-background px-2 focus-visible:outline-ring">
      {options.map((option) => <option key={option} value={option}>{optionLabel(option)}</option>)}
    </select>
  </label>;
}
