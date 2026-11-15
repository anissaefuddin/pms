import { useState } from "react";
import { Columns3 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import type { CustomField } from "@/lib/custom-fields";
import { cardFieldGroups, ganttColumnGroups, columnGroups, fieldColumn, fieldFilterRef } from "@/lib/view-columns";

export function ColumnPicker({ board, fieldsOnly, selected, fields, canSave, saving, onSave }: {
  board?: boolean; fieldsOnly?: boolean; selected: string[]; fields: CustomField[]; canSave: boolean; saving: boolean; onSave: (columns: string[]) => void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
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
