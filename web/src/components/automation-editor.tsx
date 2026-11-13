import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, ApiError } from "@/lib/api";
import { actionTypes, cleanDefinition, conditionFields, conditionOps, importanceLevels, setFields, statusGroups, triggerTypes, type Action, type Condition, type Definition } from "@/lib/automation";
import { useItemTypes } from "@/lib/item-types";
import type { Workflow } from "@/lib/status-colors";
import { FormError } from "@/components/auth/form-error";
import { ItemUserPicker, type PersonOption } from "@/components/item-user-picker";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

const select = "block w-full rounded-lg border border-input bg-background px-2 py-1.5 text-sm";
type DryRun = { conditions: { index: number; holds: boolean }[]; would_run: boolean; actions: string[]; item_title: string };
type FormRow = { id: string; name: string };

/** WHEN / IF / THEN editor (AUT-05). It only ever builds shapes the server's closed schema accepts. */
export function AutomationEditor({ spaceId, value, onChange, error }: { spaceId: string; value: Definition; onChange: (next: Definition) => void; error?: string }) {
  const { t } = useTranslation();
  const types = useItemTypes().data ?? [];
  const workflows = useQuery({ queryKey: ["workflows"], queryFn: () => api<{ items: Workflow[] }>("/api/v1/workflows") });
  const statuses = (workflows.data?.items.find((w) => w.is_default) ?? workflows.data?.items[0])?.statuses ?? [];
  const forms = useQuery({ queryKey: ["forms", spaceId], retry: false, queryFn: async () => (await api<{ items: FormRow[] }>(`/api/v1/spaces/${spaceId}/forms`)).items });
  const [people, setPeople] = useState<Record<string, PersonOption[]>>({});
  const [itemText, setItemText] = useState("");
  const [dry, setDry] = useState<DryRun>();
  const [dryError, setDryError] = useState("");
  const search = useQuery({ queryKey: ["automation-item-search", itemText], enabled: itemText.trim().length >= 2, retry: false,
    queryFn: () => api<{ items: { id: string; title: string }[] }>(`/api/v1/search?q=${encodeURIComponent(itemText.trim())}&space_id=${spaceId}&limit=6`) });
  const w = value.when;
  const setWhen = (patch: Partial<typeof w>) => onChange({ ...value, when: { ...w, ...patch } });
  const setCond = (i: number, patch: Partial<Condition>) => onChange({ ...value, if: value.if.map((c, j) => (j === i ? { ...c, ...patch } : c)) });
  const setAction = (i: number, patch: Partial<Action>) => onChange({ ...value, then: value.then.map((a, j) => (j === i ? { ...a, ...patch } : a)) });
  const pick = (key: string, list: PersonOption[], apply: (ids: string[]) => void) => { setPeople((p) => ({ ...p, [key]: list })); apply(list.map((p) => p.id)); };

  async function testOn(itemId: string) {
    setDry(undefined); setDryError("");
    try { setDry(await api<DryRun>(`/api/v1/spaces/${spaceId}/automation-rules/dry-run`, { method: "POST", body: JSON.stringify({ definition: cleanDefinition(value), item_id: itemId }) })); }
    catch (err) { setDryError(err instanceof ApiError ? err.message : (err as Error).message); }
  }

  return <div className="space-y-5">
    <fieldset className="space-y-2"><legend className="font-medium">{t("automation.when")}</legend>
      <label className="block space-y-1 text-sm"><span>{t("automation.trigger")}</span>
        <select className={select} value={w.type} onChange={(e) => onChange({ ...value, when: { type: e.target.value } })}>
          {triggerTypes.map((type) => <option key={type} value={type}>{t(`automation.triggers.${type}`)}</option>)}</select></label>
      {w.type === "item.status_changed" && <label className="block space-y-1 text-sm"><span>{t("automation.toStatus")}</span>
        <select className={select} value={w.to_status_id ?? ""} onChange={(e) => setWhen({ to_status_id: e.target.value || undefined })}>
          <option value="">{t("automation.anyStatus")}</option>{statuses.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}</select></label>}
      {w.type === "item.assigned" && <ItemUserPicker label={t("automation.assignedUser")} selected={people.when ?? []} maxSelected={1} onChange={(list) => pick("when", list, (ids) => setWhen({ user_id: ids[0] }))} />}
      {w.type === "item.created" && <label className="block space-y-1 text-sm"><span>{t("automation.createdType")}</span>
        <select className={select} value={w.item_type ?? ""} onChange={(e) => setWhen({ item_type: e.target.value || undefined })}>
          <option value="">{t("automation.anyType")}</option>{types.map((def) => <option key={def.key} value={def.key}>{def.name}</option>)}</select></label>}
      {w.type === "item.due_reached" && <label className="block space-y-1 text-sm"><span>{t("automation.daysBefore")}</span>
        <Input type="number" min={0} max={30} value={w.days_before ?? 0} onChange={(e) => setWhen({ days_before: Number(e.target.value) })} /></label>}
      {w.type === "form.submitted" && <label className="block space-y-1 text-sm"><span>{t("automation.form")}</span>
        <select className={select} value={w.form_id ?? ""} onChange={(e) => setWhen({ form_id: e.target.value || undefined })}>
          <option value="">{t("automation.anyForm")}</option>{(forms.data ?? []).map((f) => <option key={f.id} value={f.id}>{f.name}</option>)}</select></label>}
    </fieldset>

    <fieldset className="space-y-2"><legend className="font-medium">{t("automation.if")}</legend>
      <p className="text-sm text-muted-foreground">{t("automation.ifHint")}</p>
      {value.if.map((c, i) => <div key={i} className="flex flex-wrap items-end gap-2 text-sm">
        <label className="space-y-1"><span>{t("automation.condField", { number: i + 1 })}</span>
          <select className={select} value={c.field} onChange={(e) => setCond(i, { field: e.target.value, op: conditionOps[e.target.value][0], value: undefined })}>
            {conditionFields.map((f) => <option key={f} value={f}>{t(`automation.fields.${f}`)}</option>)}</select></label>
        <label className="space-y-1"><span>{t("automation.condOp", { number: i + 1 })}</span>
          <select className={select} value={c.op} onChange={(e) => setCond(i, { op: e.target.value, value: undefined })}>
            {conditionOps[c.field].map((op) => <option key={op} value={op}>{t(`automation.ops.${op}`)}</option>)}</select></label>
        {!["set", "unset", "empty", "not_empty"].includes(c.op) && <label className="space-y-1"><span>{t("automation.condValue", { number: i + 1 })}</span>
          {c.field === "importance" ? <select className={select} value={c.value ?? ""} onChange={(e) => setCond(i, { value: e.target.value })}><option value="">—</option>{importanceLevels.map((l) => <option key={l} value={l}>{t(`items.importance.${l}`)}</option>)}</select>
            : c.field === "status_group" ? <select className={select} value={c.value ?? ""} onChange={(e) => setCond(i, { value: e.target.value })}><option value="">—</option>{statusGroups.map((g) => <option key={g} value={g}>{t(`settings.workflows.group.${g}`)}</option>)}</select>
            : c.field === "item_type" ? <select className={select} value={c.value ?? ""} onChange={(e) => setCond(i, { value: e.target.value })}><option value="">—</option>{types.map((def) => <option key={def.key} value={def.key}>{def.name}</option>)}</select>
            : c.field === "assignee" ? <ItemUserPicker label={t("automation.condUser", { number: i + 1 })} selected={people[`c${i}`] ?? []} maxSelected={1} onChange={(list) => pick(`c${i}`, list, (ids) => setCond(i, { value: ids[0] }))} />
            : <Input value={c.value ?? ""} placeholder={t("automation.dateHint")} onChange={(e) => setCond(i, { value: e.target.value })} />}</label>}
        <Button type="button" variant="ghost" size="sm" aria-label={t("automation.removeCond", { number: i + 1 })} onClick={() => onChange({ ...value, if: value.if.filter((_, j) => j !== i) })}>×</Button>
      </div>)}
      <Button type="button" variant="outline" size="sm" disabled={value.if.length >= 10} onClick={() => onChange({ ...value, if: [...value.if, { field: "importance", op: "eq" }] })}>{t("automation.addCond")}</Button>
    </fieldset>

    <fieldset className="space-y-3"><legend className="font-medium">{t("automation.then")}</legend>
      {value.then.map((a, i) => <div key={i} className="space-y-2 rounded border p-3 text-sm">
        <div className="flex items-end gap-2">
          <label className="min-w-0 flex-1 space-y-1"><span>{t("automation.actionType", { number: i + 1 })}</span>
            <select className={select} value={a.type} onChange={(e) => onChange({ ...value, then: value.then.map((x, j) => (j === i ? { type: e.target.value } : x)) })}>
              {actionTypes.map((type) => <option key={type} value={type}>{t(`automation.actions.${type}`)}</option>)}</select></label>
          <Button type="button" variant="ghost" size="sm" disabled={value.then.length === 1} aria-label={t("automation.removeAction", { number: i + 1 })} onClick={() => onChange({ ...value, then: value.then.filter((_, j) => j !== i) })}>×</Button>
        </div>
        {a.type === "set_field" && <div className="grid gap-2 sm:grid-cols-2">
          <label className="space-y-1"><span>{t("automation.setField", { number: i + 1 })}</span>
            <select className={select} value={a.field ?? ""} onChange={(e) => setAction(i, { field: e.target.value, value: undefined })}>
              <option value="">—</option>{setFields.map((f) => <option key={f} value={f}>{t(`automation.setFields.${f}`)}</option>)}</select></label>
          <label className="space-y-1"><span>{t("automation.setValue", { number: i + 1 })}</span>
            {a.field === "importance" ? <select className={select} value={String(a.value ?? "")} onChange={(e) => setAction(i, { value: e.target.value })}><option value="">—</option>{importanceLevels.map((l) => <option key={l} value={l}>{t(`items.importance.${l}`)}</option>)}</select>
              : a.field === "status_id" ? <select className={select} value={String(a.value ?? "")} onChange={(e) => setAction(i, { value: e.target.value })}><option value="">—</option>{statuses.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}</select>
              : a.field === "due_date" || a.field === "start_date" ? <Input type="date" value={String(a.value ?? "")} onChange={(e) => setAction(i, { value: e.target.value || null })} />
              : <Input type="number" min={0} value={a.value === undefined || a.value === null ? "" : String(a.value)} onChange={(e) => setAction(i, { value: e.target.value === "" ? null : Number(e.target.value) })} />}</label>
        </div>}
        {a.type === "assign" && <ItemUserPicker multiple maxSelected={20} label={t("automation.assignUsers", { number: i + 1 })} selected={people[`a${i}`] ?? []} onChange={(list) => pick(`a${i}`, list, (ids) => setAction(i, { user_ids: ids }))} />}
        {a.type === "comment" && <label className="block space-y-1"><span>{t("automation.commentText", { number: i + 1 })}</span>
          <Input value={a.text ?? ""} maxLength={2000} onChange={(e) => setAction(i, { text: e.target.value })} /></label>}
        {a.type === "notify" && <div className="space-y-2">
          <ItemUserPicker multiple maxSelected={20} label={t("automation.notifyUsers", { number: i + 1 })} selected={people[`n${i}`] ?? []} onChange={(list) => pick(`n${i}`, list, (ids) => setAction(i, { user_ids: ids }))} />
          <label className="flex items-center gap-2"><input type="checkbox" checked={!!a.assignees} onChange={(e) => setAction(i, { assignees: e.target.checked })} />{t("automation.notifyAssignees", { number: i + 1 })}</label>
          <label className="block space-y-1"><span>{t("automation.notifyMessage", { number: i + 1 })}</span><Input value={a.message ?? ""} maxLength={500} onChange={(e) => setAction(i, { message: e.target.value })} /></label></div>}
        {a.type === "create_item" && <div className="grid gap-2 sm:grid-cols-2">
          <label className="space-y-1 sm:col-span-2"><span>{t("automation.newItemTitle", { number: i + 1 })}</span><Input value={a.title ?? ""} maxLength={500} onChange={(e) => setAction(i, { title: e.target.value })} /></label>
          <label className="space-y-1"><span>{t("automation.newType", { number: i + 1 })}</span>
            <select className={select} value={a.type_key ?? ""} onChange={(e) => setAction(i, { type_key: e.target.value || undefined })}>
              <option value="">{t("items.task")}</option>{types.filter((def) => !def.is_builtin && def.active).map((def) => <option key={def.key} value={def.key}>{def.name}</option>)}</select></label>
        </div>}
      </div>)}
      <Button type="button" variant="outline" size="sm" disabled={value.then.length >= 10} onClick={() => onChange({ ...value, then: [...value.then, { type: "comment", text: "" }] })}>{t("automation.addAction")}</Button>
    </fieldset>

    <fieldset className="space-y-2"><legend className="font-medium">{t("automation.test")}</legend>
      <p className="text-sm text-muted-foreground">{t("automation.testHint")}</p>
      <label className="block space-y-1 text-sm"><span>{t("automation.testItem")}</span><Input value={itemText} onChange={(e) => setItemText(e.target.value)} /></label>
      {(search.data?.items ?? []).length > 0 && <ul className="rounded border text-sm">{search.data!.items.map((it) => <li key={it.id}>
        <Button type="button" variant="ghost" className="w-full justify-start" aria-label={t("automation.testOn", { title: it.title })} onClick={() => void testOn(it.id)}>{it.title}</Button></li>)}</ul>}
      <FormError message={dryError} />
      {dry && <div role="region" aria-label={t("automation.testResult")} className="space-y-1 rounded border p-3 text-sm">
        <p className="font-medium">{dry.would_run ? t("automation.wouldRun", { title: dry.item_title }) : t("automation.wouldNot", { title: dry.item_title })}</p>
        <ul className="list-disc pl-5">{dry.conditions.map((c) => <li key={c.index}>{t("automation.condResult", { number: c.index + 1 })}: {c.holds ? t("automation.holds") : t("automation.fails")}</li>)}</ul>
        {dry.actions.length > 0 && <ul className="list-disc pl-5">{dry.actions.map((line, i) => <li key={i}>{line}</li>)}</ul>}
      </div>}
    </fieldset>
    <FormError message={error ?? ""} />
  </div>;
}
