import { ArrowDown, ArrowUp, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export type FieldType = components["schemas"]["CustomFieldType"];
export type Option = components["schemas"]["CustomFieldOption"];
export type Field = components["schemas"]["CustomField"];
export type Config = components["schemas"]["CustomFieldConfig"];
type Marker = NonNullable<Config["markers"]>[number];
type MarkerDraft = Omit<Marker, "value"> & { id: string; value: string };
export type FieldDraft = {
  id?: string;
  name: string;
  type: FieldType;
  options: Option[];
  config: Omit<Config, "decimals" | "markers"> & { decimals?: string; markers: MarkerDraft[] };
  configChanged?: boolean;
};
/** What depends on a field, read from GET .../impact before it is archived. */
export type Impact = {
  items: number; item_types: number; status_guards: number; views: number; request_forms: number;
  automations: number; blueprints: number; widgets: number; relations: number; formulas: string[];
};

export const fieldTypes: FieldType[] = [
  "text", "number", "currency", "percent", "single_select", "multi_select", "people", "date", "duration", "checkbox", "relation", "formula",
];
export const aggregations: NonNullable<Config["aggregation"]>[] = ["sum", "average", "min", "max", "count"];
const operators: Marker["operator"][] = ["lt", "lte", "eq", "gte", "gt"];
const selectClass = "block h-8 w-full min-w-0 rounded-lg border border-input bg-background px-2.5";
const defaultColor = "#2563eb"; // allow-hex: warna data yang disimpan ke API (pola hex), sama dengan --brand-primary

export const hasOptionsType = (type?: FieldType) => type === "single_select" || type === "multi_select";
export const hasNumericType = (type?: FieldType) => type === "number" || type === "currency" || type === "percent" || type === "duration";

export function newOption(): Option {
  return { id: crypto.randomUUID(), label: "", color: defaultColor };
}

/** The editable draft of a saved field, or an empty draft for a new one. */
export function draftFrom(field?: Field): FieldDraft {
  const config = field?.config;
  return {
    id: field?.id,
    name: field?.name ?? "",
    type: field?.type ?? "single_select",
    options: field?.options ?? [newOption()],
    config: {
      ...config,
      decimals: config?.decimals?.toString(),
      markers: (config?.markers ?? []).map((marker) => ({ ...marker, id: crypto.randomUUID(), value: String(marker.value) })),
    },
  };
}

/**
 * Validates a draft and builds the request body. The error is a translation key. body.type is set by the caller for new fields.
 * The rules are the same for the dialog and the registry drawer, so both save the same shape.
 */
export function draftPayload(draft: FieldDraft): { error?: string; body?: components["schemas"]["CustomFieldUpdate"] } {
  const name = draft.name.trim();
  const options = draft.options.map((option) => ({ ...option, label: option.label.trim() }));
  if (!name || (hasOptionsType(draft.type) && options.some((option) => !option.label))) {
    return { error: hasOptionsType(draft.type) ? "spaces.customFields.required" : "spaces.customFields.nameRequired" };
  }
  const config: Config = {};
  if (draft.type === "currency") {
    config.project_budget = draft.config.project_budget;
    config.currency = (draft.config.currency ?? "USD").toUpperCase();
    config.decimals = Number(draft.config.decimals ?? "2");
  }
  if (draft.type === "percent") config.allow_over_100 = draft.config.allow_over_100;
  if (draft.type === "formula") config.expression = (draft.config.expression ?? "").trim();
  if (draft.type === "relation" && draft.config.target_type_key?.trim()) config.target_type_key = draft.config.target_type_key.trim();
  if (hasNumericType(draft.type)) {
    config.aggregation = draft.config.aggregation;
    config.markers = draft.config.markers.map(({ operator, value, color }) => ({ operator, value: Number(value), color }));
  }
  const withConfig = hasNumericType(draft.type) || draft.type === "formula" || draft.type === "relation";
  return {
    body: {
      name,
      options: hasOptionsType(draft.type) ? options : undefined,
      config: withConfig && (!draft.id || draft.configChanged) ? config : undefined,
    },
  };
}

/** The fields of the editor: General and Type settings. The caller supplies the form element, buttons and save. */
export function CustomFieldFields({ draft, onChange, pending, typeLocked }: {
  draft: FieldDraft;
  onChange: (next: FieldDraft) => void;
  pending: boolean;
  /** Saved fields cannot change type; the type is only chosen when the field is created. */
  typeLocked?: boolean;
}) {
  const { t } = useTranslation();
  const set = (update: Partial<FieldDraft>) => onChange({ ...draft, ...update });
  const editOption = (id: string, update: Partial<Pick<Option, "label" | "color">>) =>
    set({ options: draft.options.map((option) => option.id === id ? { ...option, ...update } : option) });
  const editConfig = (update: Partial<FieldDraft["config"]>) => set({ config: { ...draft.config, ...update }, configChanged: true });
  const editMarker = (id: string, update: Partial<MarkerDraft>) => editConfig({
    markers: draft.config.markers.map((marker) => marker.id === id ? { ...marker, ...update } : marker),
  });
  const moveMarker = (from: number, to: number) => {
    const markers = [...draft.config.markers];
    [markers[from], markers[to]] = [markers[to], markers[from]];
    editConfig({ markers });
  };
  const hasOptions = hasOptionsType(draft.type);
  const hasNumericConfig = hasNumericType(draft.type);

  return <div className="space-y-4">
    <section aria-labelledby="cf-general" className="space-y-3">
      <h3 id="cf-general" className="text-sm font-semibold">{t("spaces.customFields.sections.general")}</h3>
      <label className="block space-y-1">
        <span>{t("spaces.customFields.name")}</span>
        <Input value={draft.name} maxLength={120} required autoFocus disabled={pending}
          onChange={(event) => set({ name: event.target.value })} />
      </label>
      {!typeLocked && <label className="block space-y-1">
        <span>{t("spaces.customFields.typeLabel")}</span>
        <select className={selectClass} value={draft.type} disabled={pending}
          onChange={(event) => set({ type: event.target.value as FieldType })}>
          {fieldTypes.map((type) => <option key={type} value={type}>{t(`spaces.customFields.type.${type}`)}</option>)}
        </select>
      </label>}
    </section>

    <section aria-labelledby="cf-type" className="space-y-3">
      <h3 id="cf-type" className="text-sm font-semibold">{t("spaces.customFields.sections.type")}</h3>
      {hasOptions && <fieldset className="space-y-2">
        <legend>{t("spaces.customFields.options")}</legend>
        {draft.options.map((option, index) => <div className="flex items-center gap-2" key={option.id}>
          <Input aria-label={t("spaces.customFields.optionLabel", { number: index + 1 })} value={option.label}
            maxLength={120} required disabled={pending} onChange={(event) => editOption(option.id, { label: event.target.value })} />
          <input type="color" className="h-8 w-10 shrink-0 rounded border bg-background p-0.5" value={option.color}
            aria-label={t("spaces.customFields.optionColor", { number: index + 1 })} disabled={pending}
            onChange={(event) => editOption(option.id, { color: event.target.value })} />
          <Button type="button" variant="ghost" size="icon-sm" disabled={pending || draft.options.length === 1}
            aria-label={t("spaces.customFields.removeOption", { number: index + 1 })}
            onClick={() => set({ options: draft.options.filter((entry) => entry.id !== option.id) })}>
            <X />
          </Button>
        </div>)}
        <Button type="button" variant="outline" size="sm" disabled={pending || draft.options.length >= 100}
          onClick={() => set({ options: [...draft.options, newOption()] })}>{t("spaces.customFields.addOption")}</Button>
      </fieldset>}
      {draft.type === "formula" && <label className="block space-y-1">
        <span>{t("spaces.customFields.formula")}</span>
        <Input value={draft.config.expression ?? ""} maxLength={500} required disabled={pending}
          onChange={(event) => editConfig({ expression: event.target.value })} />
        <small className="text-muted-foreground">{t("spaces.customFields.formulaHint")}</small></label>}
      {draft.type === "relation" && <label className="block space-y-1">
        <span>{t("spaces.customFields.relationTarget")}</span>
        <Input value={draft.config.target_type_key ?? ""} maxLength={40} disabled={pending}
          onChange={(event) => editConfig({ target_type_key: event.target.value })} /></label>}
      {draft.type === "currency" && <div className="grid grid-cols-2 gap-2">
        <label className="block space-y-1">
          <span>{t("spaces.customFields.currencyCode")}</span>
          <Input value={draft.config.currency ?? "USD"} pattern="[A-Za-z]{3}" maxLength={3} required
            disabled={pending} onChange={(event) => editConfig({ currency: event.target.value })} />
        </label>
        <label className="block space-y-1">
          <span>{t("spaces.customFields.decimals")}</span>
          <Input type="number" value={draft.config.decimals ?? "2"} min={0} max={6} required
            disabled={pending} onChange={(event) => editConfig({ decimals: event.target.value })} />
        </label>
      </div>}
      {draft.type === "percent" && <label className="flex items-center gap-2">
        <input type="checkbox" checked={draft.config.allow_over_100 ?? false} disabled={pending}
          onChange={(event) => editConfig({ allow_over_100: event.target.checked })} />
        <span>{t("spaces.customFields.allowOver100")}</span>
      </label>}
      {draft.type === "currency" && <label className="flex items-center gap-2">
        <input type="checkbox" checked={draft.config.project_budget ?? false} disabled={pending}
          onChange={(event) => editConfig({ project_budget: event.target.checked })} />
        {t("time.budgetSetting")}
      </label>}
      {hasNumericConfig && <>
        <label className="block space-y-1">
          <span>{t("spaces.customFields.aggregationLabel")}</span>
          <select className={selectClass} value={draft.config.aggregation ?? ""} disabled={pending}
            onChange={(event) => editConfig({ aggregation: aggregations.find((value) => value === event.target.value) })}>
            <option value="">{t("spaces.customFields.aggregation.none")}</option>
            {aggregations.map((aggregation) => <option key={aggregation} value={aggregation}>
              {t(`spaces.customFields.aggregation.${aggregation}`)}
            </option>)}
          </select>
        </label>
        <fieldset className="space-y-2">
          <legend>{t("spaces.customFields.markers")}</legend>
          {draft.config.markers.length > 0 && <p className="text-muted-foreground">{t("spaces.customFields.markerOrder")}</p>}
          {draft.config.markers.map((marker, index) => <div className="grid grid-cols-[2fr_1fr] items-center gap-2 sm:flex" key={marker.id}>
            <select className={selectClass} value={marker.operator} disabled={pending}
              aria-label={t("spaces.customFields.markerOperator", { number: index + 1 })}
              onChange={(event) => editMarker(marker.id, { operator: event.target.value as Marker["operator"] })}>
              {operators.map((operator) => <option key={operator} value={operator}>{t(`spaces.customFields.operator.${operator}`)}</option>)}
            </select>
            <Input type="number" min={draft.type === "duration" ? 0 : -1e9} max={draft.type === "duration" ? 2_147_483_647 : 1e9}
              step="any" value={marker.value} required disabled={pending}
              aria-label={t("spaces.customFields.markerValue", { number: index + 1 })}
              onChange={(event) => editMarker(marker.id, { value: event.target.value })} />
            <div className="col-span-2 flex shrink-0 items-center gap-1">
              <input type="color" className="h-8 w-10 shrink-0 rounded border bg-background p-0.5" value={marker.color}
                aria-label={t("spaces.customFields.markerColor", { number: index + 1 })} disabled={pending}
                onChange={(event) => editMarker(marker.id, { color: event.target.value })} />
              <Button type="button" variant="ghost" size="icon-sm" disabled={pending || index === 0}
                aria-label={t("spaces.customFields.moveMarkerUp", { number: index + 1 })} onClick={() => moveMarker(index, index - 1)}>
                <ArrowUp />
              </Button>
              <Button type="button" variant="ghost" size="icon-sm" disabled={pending || index === draft.config.markers.length - 1}
                aria-label={t("spaces.customFields.moveMarkerDown", { number: index + 1 })} onClick={() => moveMarker(index, index + 1)}>
                <ArrowDown />
              </Button>
              <Button type="button" variant="ghost" size="icon-sm" disabled={pending}
                aria-label={t("spaces.customFields.removeMarker", { number: index + 1 })}
                onClick={() => editConfig({ markers: draft.config.markers.filter((entry) => entry.id !== marker.id) })}>
                <X />
              </Button>
            </div>
          </div>)}
          <Button type="button" variant="outline" size="sm" disabled={pending || draft.config.markers.length >= 10}
            onClick={() => editConfig({
              markers: [...draft.config.markers, { id: crypto.randomUUID(), operator: "gte", value: "", color: defaultColor }],
            })}>{t("spaces.customFields.addMarker")}</Button>
        </fieldset>
      </>}
    </section>
  </div>;
}

/** The dependencies of a field, listed only where the count is above zero. Used before an archive. */
export function FieldImpactList({ impact }: { impact: Impact }) {
  const { t } = useTranslation();
  const rows = [
    ["items", impact.items], ["itemTypes", impact.item_types], ["statusGuards", impact.status_guards], ["views", impact.views],
    ["requestForms", impact.request_forms], ["automations", impact.automations], ["blueprints", impact.blueprints],
    ["widgets", impact.widgets], ["relations", impact.relations],
  ] as const;
  const used = rows.filter(([, count]) => count > 0);
  return <div className="space-y-2">
    {used.length > 0 ? <ul className="list-disc pl-5">
      {used.map(([key, count]) => <li key={key}>{t(`spaces.customFields.usage.${key}`, { count })}</li>)}
    </ul> : <p>{t("spaces.customFields.usage.none")}</p>}
    {impact.formulas.length > 0 && <p role="alert">{t("spaces.customFields.usage.formulaBlocks", { names: impact.formulas.join(", ") })}</p>}
  </div>;
}
