import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Columns3 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { CustomFieldDrawer } from "@/components/custom-field-drawer";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import type { CustomField } from "@/lib/custom-fields";
import { cardFieldGroups, ganttColumnGroups, columnGroups, fieldColumn, fieldFilterRef } from "@/lib/view-columns";

type Choice = { id: string; name: string; type: string; assigned: boolean };

/**
 * Columns for a table or board, with the Space's field management for people with the Full role: add a Workspace field to the
 * Space, remove it, or create a new field. The field list itself is the one the Space's views render.
 */
export function ColumnPicker({ board, fieldsOnly, selected, fields, canSave, saving, onSave, spaceId, spaceName, canManage }: {
  board?: boolean; fieldsOnly?: boolean; selected: string[]; fields: CustomField[]; canSave: boolean; saving: boolean; onSave: (columns: string[]) => void;
  spaceId?: string; spaceName?: string; canManage?: boolean;
}) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [open, setOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [fieldError, setFieldError] = useState("");
  const [busy, setBusy] = useState<string>();
  const manage = !!canManage && !!spaceId && !fieldsOnly;
  const choices = useQuery({ queryKey: ["field-choices", spaceId], enabled: open && manage, retry: false,
    queryFn: async () => (await api<{ items: Choice[] }>(`/api/v1/spaces/${spaceId}/field-choices`)).items });

  async function setAssigned(fieldId: string, assigned: boolean) {
    setBusy(fieldId);
    setFieldError("");
    try {
      const base = `/api/v1/spaces/${spaceId}/custom-fields/${fieldId}/assignments`;
      if (assigned) await api(`${base}/space/${spaceId}`, { method: "DELETE" });
      else await api(base, { method: "POST", body: JSON.stringify({ scope_kind: "space", scope_id: spaceId }) });
      await Promise.all([client.invalidateQueries({ queryKey: ["field-choices", spaceId] }), client.invalidateQueries({ queryKey: ["custom-fields", spaceId] })]);
    } catch (err) {
      setFieldError((err as Error).message);
    } finally {
      setBusy(undefined);
    }
  }

  const [draft, setDraft] = useState(selected);
  const toggle = (id: string) => setDraft((list) => list.includes(id) ? list.filter((entry) => entry !== id) : [...list, id]);
  const box = (id: string, label: string) => <label key={id} className="flex items-center gap-2">
    <input type="checkbox" checked={draft.includes(id)} disabled={!canSave} onChange={() => toggle(id)} />{label}
  </label>;
  return <>
    <Button variant="outline" size="sm" onClick={() => { setDraft(selected); setOpen(true); }}><Columns3 />{t(board ? "views.columns.cardOpen" : "views.columns.open")}</Button>
    {open && <Dialog open onOpenChange={setOpen}>
      <DialogContent>
        <DialogTitle>{t(board ? "views.columns.cardTitle" : "views.columns.title")}</DialogTitle>
        <p className="text-sm text-foreground">{t("views.columns.hint")}</p>
        <div className="max-h-[60vh] space-y-3 overflow-y-auto">
          {(fieldsOnly ? ganttColumnGroups : board ? cardFieldGroups : columnGroups).map((group) => <fieldset key={group.id} className="space-y-1">
            <legend className="font-medium">{t(`views.columns.${group.id}`)}</legend>
            {group.columns.map((id) => box(id, t(`views.columns.${id}`)))}
          </fieldset>)}
          <fieldset className="space-y-1">
            <legend className="font-medium">{t("views.columns.custom")}</legend>
            {fields.length ? fields.map((field) => box(fieldColumn(field.id), field.name)) : <p className="text-sm text-muted-foreground">{t("views.columns.noFields")}</p>}
          </fieldset>
          {manage && <fieldset className="space-y-2">
            <legend className="font-medium">{t("views.columns.manageFields")}</legend>
            <p className="text-xs text-muted-foreground">{t("views.columns.manageNote")}</p>
            {choices.isError && <p className="text-sm text-muted-foreground">{t("views.columns.manageForbidden")}</p>}
            <ul className="divide-y rounded-md border">
              {(choices.data ?? []).map((choice) => <li key={choice.id} className="flex items-center justify-between gap-2 px-3 py-2 text-sm">
                <span className="min-w-0 truncate">{choice.name}</span>
                <Button size="sm" variant="outline" disabled={busy === choice.id} onClick={() => void setAssigned(choice.id, choice.assigned)}>
                  {t(choice.assigned ? "views.columns.removeFromSpace" : "views.columns.addToSpace")}</Button>
              </li>)}
            </ul>
            <FormError message={fieldError} />
            <Button size="sm" variant="outline" onClick={() => { setOpen(false); setCreating(true); }}>{t("views.columns.createField")}</Button>
          </fieldset>}
          {!board && !fieldsOnly && <fieldset disabled className="space-y-1">
            <legend className="font-medium">{t("views.columns.baselines")} <span className="text-xs font-normal text-muted-foreground">{t("views.columns.soon")}</span></legend>
            <p className="text-sm text-muted-foreground">{t("views.columns.baselineNote")}</p>
          </fieldset>}
        </div>
        <div className="flex gap-2">
          {canSave && <Button size="sm" disabled={saving} onClick={() => { onSave(draft); setOpen(false); }}>{t("views.columns.save")}</Button>}
          <Button size="sm" variant="ghost" onClick={() => setOpen(false)}>{t("views.columns.cancel")}</Button>
        </div>
      </DialogContent>
    </Dialog>}
    {creating && spaceId && <CustomFieldDrawer key="new-field" open scope={{ spaceId, ownerName: spaceName ?? "" }}
      onOpenChange={(next) => { if (!next) setCreating(false); }}
      onSaved={() => { void client.invalidateQueries({ queryKey: ["custom-fields", spaceId] }); void client.invalidateQueries({ queryKey: ["field-choices", spaceId] }); }} />}
  </>;
}

/** One field filter in the URL as `field=<id>:<value>`; the server does the matching. */
export function FieldFilter({ fields, value, onChange }: { fields: CustomField[]; value?: string; onChange: (value: string) => void }) {
  const { t } = useTranslation();
  const { id, value: text } = fieldFilterRef(value);
  const field = fields.find((entry) => entry.id === id);
  const commit = (next: string) => onChange(id && next !== "" ? `${id}:${next}` : "");
  return <div className="flex items-end gap-2">
    <label className="space-y-1"><span>{t("views.fieldFilter.field")}</span>
      <select value={id} onChange={(event) => onChange(event.target.value ? `${event.target.value}:` : "")}
        className="block rounded border bg-background p-2">
        <option value="">{t("views.fieldFilter.none")}</option>
        {fields.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}
      </select></label>
    {field && <label className="space-y-1"><span>{t("views.fieldFilter.value")}</span>
      {field.type === "single_select" || field.type === "multi_select" || field.type === "checkbox"
        ? <select value={text} onChange={(event) => commit(event.target.value)} className="block rounded border bg-background p-2">
          <option value="" />
          {field.type === "checkbox"
            ? <><option value="true">{t("views.fieldFilter.yes")}</option><option value="false">{t("views.fieldFilter.no")}</option></>
            : field.options.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}
        </select>
        : <Input key={id} defaultValue={text} maxLength={200} type={field.type === "date" ? "date" : "text"}
          inputMode={field.type === "text" || field.type === "date" ? undefined : "decimal"}
          onBlur={(event) => { if (event.target.value !== text) commit(event.target.value); }}
          onKeyDown={(event) => { if (event.key === "Enter") commit(event.currentTarget.value); }} />}
    </label>}
  </div>;
}
