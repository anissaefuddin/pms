import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { FormError } from "@/components/auth/form-error";
import { ItemUserPicker, type PersonOption } from "@/components/item-user-picker";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { customFieldMarker, formatCustomNumber, parseCustomNumber, validCustomNumber, type CustomField, type CustomFieldValue } from "@/lib/custom-fields";
import type { CurrentUser } from "@/lib/api";
import { formatDate } from "@/lib/profile";

function ColorDot({ color, label }: { color: string; label?: string }) {
  return <span className="size-2.5 shrink-0 rounded-full border" style={{ backgroundColor: color }}
    role={label ? "img" : undefined} title={label} aria-label={label} aria-hidden={label ? undefined : true} />;
}

export function CustomFieldDisplay({ field, value, people, profile }: {
  field: CustomField; value?: CustomFieldValue; people: PersonOption[]; profile: CurrentUser;
}) {
  const { t } = useTranslation();
  if (field.type === "formula" && value == null) {
    return <span className="inline-flex items-center gap-1 text-muted-foreground" title={t("items.customFields.formulaNoResultHint")}>
      {t("items.customFields.formulaNoResult")}<span className="sr-only">{t("items.customFields.formulaNoResultHint")}</span></span>;
  }
  if (value == null || value === "" || Array.isArray(value) && !value.length) {
    return <span className="text-muted-foreground">{t("items.customFields.empty")}</span>;
  }
  if (typeof value === "number") {
    const marker = customFieldMarker(field, value);
    const formatted = formatCustomNumber(field, value, profile.locale);
    return <span className="inline-flex items-center gap-1.5 tabular-nums">
      {marker && <ColorDot color={marker.color} label={t(`items.customFields.marker.${marker.operator}`, {
        threshold: new Intl.NumberFormat(profile.locale, { maximumSignificantDigits: 21 }).format(marker.value),
      })} />}
      {field.type === "duration" ? t("items.customFields.days", { count: value, formatted }) : formatted}
    </span>;
  }
  if (typeof value === "boolean") return t(value ? "items.customFields.checked" : "items.customFields.unchecked");
  if (field.type === "people") return people.map((person) => person.status && person.status !== "active"
    ? t("items.customFields.inactivePerson", { name: person.name }) : person.name).join(", ");
  if (field.type === "single_select" || field.type === "multi_select") {
    const ids = Array.isArray(value) ? value : [value];
    return <span className="flex flex-wrap gap-x-3 gap-y-1">
      {field.options.filter((option) => ids.includes(option.id)).map((option) => <span key={option.id} className="inline-flex items-center gap-1.5">
        <ColorDot color={option.color} />{option.label}
      </span>)}
    </span>;
  }
  if (typeof value === "string") return field.type === "date" ? formatDate(value, profile) : <span className="whitespace-pre-wrap">{value}</span>;
}

export function CustomFieldEditor({ field, value, people, locale, disabled, onCancel, onSave }: {
  field: CustomField;
  value?: CustomFieldValue;
  people: PersonOption[];
  locale: string;
  disabled?: boolean;
  onCancel: () => void;
  onSave: (value: CustomFieldValue) => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState<CustomFieldValue>(typeof value === "number" ? formatCustomNumber(field, value, locale) : value ?? null);
  const [selectedPeople, setSelectedPeople] = useState(people);
  const [dirty, setDirty] = useState(false);
  const [error, setError] = useState("");
  const numeric = field.type === "number" || field.type === "currency" || field.type === "percent" || field.type === "duration";
  const selectedIds = Array.isArray(draft) ? draft : [];
  const inputValue = typeof draft === "string" ? draft : "";

  function edit(value: CustomFieldValue) {
    setError("");
    setDraft(value);
    setDirty(true);
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (disabled || !dirty) return;
    let update = draft;
    if (update === "" || Array.isArray(update) && !update.length || numeric && typeof update === "string" && !update.trim()) update = null;
    if (numeric && typeof update === "string") {
      const parsed = parseCustomNumber(field, update, locale);
      if (parsed === undefined || !validCustomNumber(field, parsed)) {
        const max = formatCustomNumber({ type: "number" }, 1_000_000_000, locale);
        if (field.type === "percent") {
          setError(t("items.customFields.invalidPercent", { max: field.config?.allow_over_100 ? max : "100" }));
        } else if (field.type === "duration") {
          setError(t("items.customFields.invalidDuration", { max: formatCustomNumber(field, 2_147_483_647, locale) }));
        } else {
          setError(t("items.customFields.invalidNumber", { max, count: field.type === "currency" ? field.config?.decimals ?? 2 : 6 }));
        }
        return;
      }
      onSave(parsed);
      return;
    }
    if (field.type === "text" && typeof update === "string" && [...update].length > 5000) {
      setError(t("items.customFields.invalidText"));
      return;
    }
    onSave(update);
  }

  return <form className="space-y-2" aria-label={field.name} onSubmit={submit}>
    {field.type === "people" ? <ItemUserPicker multiple autoFocus maxSelected={100} label={field.name}
      selected={selectedPeople} disabled={disabled} onChange={(people) => {
        setSelectedPeople(people);
        edit(people.map((person) => person.id));
      }} /> : field.type === "multi_select" ? <fieldset className="space-y-1" disabled={disabled}>
      <legend className="font-medium">{field.name}</legend>
      {field.options.map((option, index) => <label key={option.id} className="flex items-center gap-2">
        <input type="checkbox" autoFocus={index === 0} checked={selectedIds.includes(option.id)} onChange={(event) => {
          edit(event.target.checked ? [...selectedIds, option.id] : selectedIds.filter((id) => id !== option.id));
        }} />
        <ColorDot color={option.color} />{option.label}
      </label>)}
    </fieldset> : field.type === "single_select" ? <label className="block space-y-1">
      <span className="font-medium">{field.name}</span>
      <select autoFocus className="block h-8 w-full rounded-lg border border-input bg-background px-2.5"
        disabled={disabled} value={inputValue} onChange={(event) => edit(event.target.value)}>
        <option value="">{t("items.customFields.empty")}</option>
        {field.options.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}
      </select>
    </label> : field.type === "checkbox" ? <label className="flex items-center gap-2 font-medium">
      <input type="checkbox" autoFocus disabled={disabled} checked={draft === true} ref={(input) => {
        if (input) input.indeterminate = draft === null;
      }} onChange={(event) => edit(event.target.checked)} />
      {field.name}
    </label> : <label className="block space-y-1">
      <span className="font-medium">{field.type === "duration" ? t("items.customFields.durationLabel", { name: field.name }) : field.name}</span>
      {field.type === "text" ? <textarea autoFocus value={inputValue} disabled={disabled} aria-invalid={!!error} rows={3}
        className="block w-full rounded-lg border border-input bg-transparent p-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-50 aria-invalid:border-destructive"
        onChange={(event) => edit(event.target.value)} /> : <Input autoFocus type={field.type === "date" ? "date" : "text"} value={inputValue} disabled={disabled}
        inputMode={numeric ? "decimal" : undefined} aria-invalid={!!error}
        min={field.type === "date" ? "0001-01-01" : undefined} max={field.type === "date" ? "9999-12-31" : undefined}
        onChange={(event) => edit(event.target.value)} />}
    </label>}
    <FormError message={error} />
    <div className="flex flex-wrap gap-2">
      <Button size="sm" type="submit" disabled={disabled || !dirty}>{t("items.save")}</Button>
      <Button size="sm" type="button" variant="outline" disabled={disabled} onClick={onCancel}>{t("items.cancel")}</Button>
      {draft != null && <Button size="sm" type="button" variant="ghost" disabled={disabled} onClick={() => {
        setSelectedPeople([]);
        edit(null);
      }}>{t("items.customFields.clear")}</Button>}
    </div>
  </form>;
}
