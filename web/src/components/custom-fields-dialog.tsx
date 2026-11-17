import { useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowDown, ArrowUp, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

type FieldType = components["schemas"]["CustomFieldType"];
type Option = components["schemas"]["CustomFieldOption"];
type Field = components["schemas"]["CustomField"];
type Config = components["schemas"]["CustomFieldConfig"];
type Marker = NonNullable<Config["markers"]>[number];
type MarkerDraft = Omit<Marker, "value"> & { id: string; value: string };
type FieldDraft = {
  id?: string;
  name: string;
  type: FieldType;
  options: Option[];
  config: Omit<Config, "decimals" | "markers"> & { decimals?: string; markers: MarkerDraft[] };
  configChanged?: boolean;
};

const fieldTypes: FieldType[] = [
  "text", "number", "currency", "percent", "single_select", "multi_select", "people", "date", "duration", "checkbox", "relation", "formula",
];
const aggregations: NonNullable<Config["aggregation"]>[] = ["sum", "average", "min", "max", "count"];
const operators: Marker["operator"][] = ["lt", "lte", "eq", "gte", "gt"];
const selectClass = "block h-8 w-full min-w-0 rounded-lg border border-input bg-background px-2.5";
const defaultColor = "#2563eb"; // allow-hex: warna data yang disimpan ke API (pola hex), sama dengan --brand-primary

function newOption(): Option {
  return { id: crypto.randomUUID(), label: "", color: defaultColor };
}

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
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const hasOptions = draft?.type === "single_select" || draft?.type === "multi_select";
  const hasNumericConfig = draft?.type === "number" || draft?.type === "currency"
    || draft?.type === "percent" || draft?.type === "duration";

  async function change(resource: string, method: "POST" | "PATCH" | "DELETE", value?: unknown) {
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
    const name = draft.name.trim();
    const options = draft.options.map((option) => ({ ...option, label: option.label.trim() }));
    if (!name || (hasOptions && options.some((option) => !option.label))) {
      setError(t(hasOptions ? "spaces.customFields.required" : "spaces.customFields.nameRequired"));
      return;
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
    if (hasNumericConfig) {
      config.aggregation = draft.config.aggregation;
      config.markers = draft.config.markers.map(({ operator, value, color }) => ({ operator, value: Number(value), color }));
    }
    const value = {
      name,
      options: hasOptions ? options : undefined,
      config: (hasNumericConfig || draft.type === "formula" || draft.type === "relation") && (!draft.id || draft.configChanged) ? config : undefined,
    } satisfies components["schemas"]["CustomFieldUpdate"];
    if (draft.id) void change(`${path}/${draft.id}`, "PATCH", value);
    else void change(path, "POST", { ...value, type: draft.type });
  }

  function edit(field?: Field) {
    setError("");
    const config = field?.config;
    setDraft({
      id: field?.id,
      name: field?.name ?? "",
      type: field?.type ?? "single_select",
      options: field?.options ?? [newOption()],
      config: {
        ...config,
        decimals: config?.decimals?.toString(),
        markers: (config?.markers ?? []).map((marker) => ({ ...marker, id: crypto.randomUUID(), value: String(marker.value) })),
      },
    });
  }

  function editOption(id: string, update: Partial<Pick<Option, "label" | "color">>) {
    setDraft((current) => current && {
      ...current,
      options: current.options.map((option) => option.id === id ? { ...option, ...update } : option),
    });
  }

  function editConfig(update: Partial<FieldDraft["config"]>) {
    setDraft((current) => current && { ...current, config: { ...current.config, ...update }, configChanged: true });
  }

  function editMarker(id: string, update: Partial<MarkerDraft>) {
    setDraft((current) => current && {
      ...current,
      configChanged: true,
      config: {
        ...current.config,
        markers: current.config.markers.map((marker) => marker.id === id ? { ...marker, ...update } : marker),
      },
    });
  }

  function moveMarker(from: number, to: number) {
    if (!draft) return;
    const markers = [...draft.config.markers];
    [markers[from], markers[to]] = [markers[to], markers[from]];
    editConfig({ markers });
  }

  const rows = fields.data?.items ?? [];
  const actionsDisabled = pending || draft !== null || fields.isFetching;

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
          {field.archived_at ? <span>{t("spaces.customFields.archived")}</span> : <div className="flex flex-wrap items-center gap-1">
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
              onClick={() => edit(field)}>{t("spaces.customFields.edit")}</Button>
            <Button variant="ghost" size="sm" disabled={actionsDisabled}
              onClick={() => void change(`${path}/${field.id}`, "DELETE")}>{t("spaces.customFields.archive")}</Button>
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
    {draft ? <form className="space-y-3 rounded-lg border p-3" onSubmit={save}>
      <label className="block space-y-1">
        <span>{t("spaces.customFields.name")}</span>
        <Input value={draft.name} maxLength={120} required autoFocus disabled={pending}
          onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
      </label>
      {!draft.id && <label className="block space-y-1">
        <span>{t("spaces.customFields.typeLabel")}</span>
        <select className={selectClass} value={draft.type}
          disabled={pending} onChange={(event) => setDraft({ ...draft, type: event.target.value as FieldType })}>
          {fieldTypes.map((type) => <option key={type} value={type}>{t(`spaces.customFields.type.${type}`)}</option>)}
        </select>
      </label>}
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
            onClick={() => setDraft({ ...draft, options: draft.options.filter((entry) => entry.id !== option.id) })}>
            <X />
          </Button>
        </div>)}
        <Button type="button" variant="outline" size="sm" disabled={pending || draft.options.length >= 100}
          onClick={() => setDraft({ ...draft, options: [...draft.options, newOption()] })}>{t("spaces.customFields.addOption")}</Button>
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
      <FormError message={error} />
      <div className="flex flex-wrap justify-end gap-2">
        <Button type="button" variant="outline" disabled={pending} onClick={() => { setDraft(null); setError(""); }}>{t("spaces.form.cancel")}</Button>
        <Button type="submit" disabled={pending}>{t("spaces.form.edit.submit")}</Button>
      </div>
    </form> : <>
      <FormError message={error} />
      <Button disabled={actionsDisabled || !fields.isSuccess} onClick={() => edit()}>{t("spaces.customFields.add")}</Button>
    </>}
  </div>;
}
