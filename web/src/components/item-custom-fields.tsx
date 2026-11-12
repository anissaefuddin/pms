import { useId, useRef, useState } from "react";
import { useQueries } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { CustomFieldDisplay, CustomFieldEditor } from "@/components/custom-field-editor";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { api, type CurrentUser } from "@/lib/api";
import type { CustomField, CustomFieldValue } from "@/lib/custom-fields";
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
  const { t } = useTranslation();
  const heading = useId();
  const [editing, setEditing] = useState<string>();
  const returnFocus = useRef<string | undefined>(undefined);
  const spaceIds = [...new Set(item.locations.map(({ space_id }) => space_id))];
  const queries = useQueries({ queries: spaceIds.map((spaceId) => ({
    queryKey: ["custom-fields", spaceId],
    queryFn: () => api<{ items: CustomField[] }>(`/api/v1/spaces/${spaceId}/custom-fields`),
  })) });
  const fields = queries.flatMap((query) => query.data?.items ?? [])
    .filter((field) => !field.archived_at || item.fields?.[field.id] != null);
  const pending = queries.some((query) => query.isPending);
  const failed = queries.filter((query) => query.isError);
  const editingField = fields.find((field) => field.id === editing && !field.archived_at);

  function close(id: string) {
    onCancel();
    returnFocus.current = id;
    setEditing(undefined);
  }

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
          {editable && editingField?.id === field.id ? <CustomFieldEditor field={field} value={value} people={people}
            locale={profile.locale} disabled={disabled} onCancel={() => close(field.id)} onSave={async (next) => {
              if (JSON.stringify(next) === JSON.stringify(value ?? null) || await onSave(field.id, next)) close(field.id);
            }} /> : <div className="flex items-start justify-between gap-2">
            <div className="min-w-0 break-words">
              <p className="font-medium">
                {field.name}
                {field.archived_at && <span className="ms-2 text-xs font-normal text-muted-foreground">{t("spaces.customFields.archived")}</span>}
              </p>
              <CustomFieldDisplay field={field} value={value} people={people} profile={profile} />
            </div>
            {editable && !field.archived_at && <Button type="button" size="sm" variant="outline" disabled={disabled || !!editingField}
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
