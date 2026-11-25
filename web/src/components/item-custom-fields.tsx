import { useId, useRef, useState } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { RelationDisplay, RelationEditor, useItemRelations } from "@/components/relation-field";
import { CustomFieldDisplay, CustomFieldEditor } from "@/components/custom-field-editor";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { api, type CurrentUser } from "@/lib/api";
import { formatCustomNumber, type CustomField, type CustomFieldValue } from "@/lib/custom-fields";
import { formatMinutes } from "@/lib/time-entries";
import type { Item } from "@/lib/items";

export function ItemCustomFields({ item, profile, editable, disabled, onEdit, onCancel, onSave }: {
  item: Item;
  profile: CurrentUser;
  editable: boolean;
  disabled: boolean;
  onEdit: () => void;
  onCancel: () => void;
  onSave: (id: string, value: CustomFieldValue) => Promise<boolean>;
}) {
  const { t, i18n } = useTranslation();
  const heading = useId();
  const [editing, setEditing] = useState<string>();
  const returnFocus = useRef<string | undefined>(undefined);
  const spaceIds = [...new Set(item.locations.map(({ space_id }) => space_id))];
  const queries = useQueries({ queries: spaceIds.map((spaceId) => ({
    queryKey: ["custom-fields", spaceId],
    queryFn: () => api<{ items: CustomField[] }>(`/api/v1/spaces/${spaceId}/custom-fields`),
  })) });
  const fields = queries.flatMap((query) => query.data?.items ?? [])
    .filter((field) => (!field.archived_at || item.fields?.[field.id] != null) &&
      (item.type === "project" || !field.config?.project_budget));
  const budgetFields = fields.filter((field) => field.config?.project_budget);
  const actual = useQuery({
    queryKey: ["time-entries", "summary", item.id], enabled: budgetFields.length > 0,
    queryFn: ({ signal }) => api<{ actual_minutes: number; actual_cost_minor?: number; cost_status: "full" | "partial" | "unauthorized"; scope: "project" | "permitted_entries" }>(
      `/api/v1/items/${item.id}/time-summary`, { signal }),
  });
  const pending = queries.some((query) => query.isPending);
  const failed = queries.filter((query) => query.isError);
  const editingField = fields.find((field) => field.id === editing && !field.archived_at);

  function close(id: string) {
    onCancel();
    returnFocus.current = id;
    setEditing(undefined);
  }

  const relations = useItemRelations(item.id, item.version, fields.some((field) => field.type === "relation"));
  const relationOf = (id: string) => relations.data?.find((group) => group.field_id === id);
  if (!fields.length && !pending && !failed.length) return null;
  return <section aria-labelledby={heading} className="space-y-2 border-t pt-4">
    <h3 id={heading} className="font-medium">{t("items.customFields.title")}</h3>
    {pending && <p role="status">{t("items.customFields.loading")}</p>}
    {!!failed.length && <div className="flex items-center gap-2">
      <FormError message={t("items.customFields.loadError")} />
      <Button variant="outline" size="sm" onClick={() => failed.forEach((query) => void query.refetch())}>{t("items.retry")}</Button>
    </div>}
    <ul className="divide-y">
      {fields.map((field) => {
        const value = item.fields?.[field.id];
        const people = field.type === "people" && Array.isArray(value) ? value.map((id) =>
          item.field_people?.[id] ?? { id, name: t("items.customFields.unknownPerson") }) : [];
        return <li className="py-2" key={field.id}>
          {editable && editingField?.id === field.id && field.type === "relation" ? <RelationEditor itemId={item.id} group={relationOf(field.id)} disabled={disabled}
            onCancel={() => close(field.id)} onSave={async (ids) => { if (await onSave(field.id, ids)) close(field.id); }} />
          : editable && editingField?.id === field.id ? <CustomFieldEditor field={field} value={value} people={people}
            locale={profile.locale} disabled={disabled} onCancel={() => close(field.id)} onSave={async (next) => {
              if (JSON.stringify(next) === JSON.stringify(value ?? null) || await onSave(field.id, next)) close(field.id);
            }} /> : <div className="flex items-start justify-between gap-2">
            <div className="min-w-0 break-words">
              <p className="font-medium">
                {field.name}
                {field.archived_at && <span className="ms-2 text-xs font-normal text-muted-foreground">{t("spaces.customFields.archived")}</span>}
              </p>
              {field.type === "relation" ? <RelationDisplay group={relationOf(field.id)} /> : <CustomFieldDisplay field={field} value={value} people={people} profile={profile} />}
              {field.config?.project_budget && actual.data && <p className="text-sm text-muted-foreground">
                {t(actual.data.scope === "project" ? "time.actual" : "time.actualVisible", { duration: formatMinutes(actual.data.actual_minutes) })}
              </p>}
              {field.config?.project_budget && actual.data?.actual_cost_minor != null && <p className="text-sm text-muted-foreground">
                {t("time.actualCost", { cost: formatCustomNumber(field, actual.data.actual_cost_minor / 100, i18n.language) })}
                {typeof value === "number" && value > 0 && ` · ${t("time.budgetUsed", { percent: Math.round(actual.data.actual_cost_minor / 100 / value * 100) })}`}
              </p>}
              {field.config?.project_budget && actual.data?.cost_status === "partial" && <p className="text-sm text-muted-foreground">{t("time.costPartial")}</p>}
              {field.config?.project_budget && actual.isError && <FormError message={actual.error.message} />}
            </div>
            {editable && !field.archived_at && field.type !== "formula" && <Button type="button" size="sm" variant="outline" disabled={disabled || !!editingField}
              ref={(button) => {
                if (button && returnFocus.current === field.id) {
                  button.focus();
                  returnFocus.current = undefined;
                }
              }}
              aria-label={t("items.customFields.editField", { name: field.name })} onClick={() => {
                onEdit();
                setEditing(field.id);
              }}>{t("items.customFields.edit")}</Button>}
          </div>}
        </li>;
      })}
    </ul>
  </section>;
}
