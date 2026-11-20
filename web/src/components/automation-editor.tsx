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
type DryRun = { conditions: { index: number; holds: boolean }[]; would_run: boolean; actions: string[]; item_title: string; trace?: string[] };
type FormRow = { id: string; name: string };

/** WHEN / IF / THEN editor (AUT-05). It only ever builds shapes the server's closed schema accepts. */
export type EditorStep = "when" | "if" | "then" | "test";

/** WHEN / IF / THEN / Try it, one step at a time in the builder's contextual panel (A3). */
export function AutomationEditor({ spaceId, ruleId, scope = "space", value, onChange, error, step }: { spaceId: string; ruleId?: string; scope?: "space" | "workspace"; value: Definition; onChange: (next: Definition) => void; error?: string; step: EditorStep }) {
  const { t } = useTranslation();
  // A Space's automations offer the Workspace types and that Space's own types only.
  const types = (useItemTypes().data ?? []).filter((def) => !def.space_id || def.space_id === spaceId);
  const workflows = useQuery({ queryKey: ["workflows"], queryFn: () => api<{ items: Workflow[] }>("/api/v1/workflows") });
  const statuses = (workflows.data?.items.find((w) => w.is_default) ?? workflows.data?.items[0])?.statuses ?? [];
  const forms = useQuery({ queryKey: ["forms", spaceId], enabled: spaceId !== "", retry: false, queryFn: async () => (await api<{ items: FormRow[] }>(`/api/v1/spaces/${spaceId}/forms`)).items });
  const [people, setPeople] = useState<Record<string, PersonOption[]>>({});
  // Custom fields a rule may read or set (AUT-07): a Space rule uses its Space's fields. Groups are for mentions.
  const customFields = useQuery({ queryKey: ["automation-custom-fields", spaceId], enabled: spaceId !== "" && scope === "space", retry: false,
    queryFn: async () => (await api<{ items: { id: string; name: string; type: string; archived_at?: string | null }[] }>(`/api/v1/spaces/${spaceId}/custom-fields`)).items });
  const fieldChoices = (customFields.data ?? []).filter((f) => !f.archived_at);
  const groups = useQuery({ queryKey: ["automation-groups"], retry: false, queryFn: async () => (await api<{ items: { id: string; name: string }[] }>("/api/v1/groups")).items });
  const fieldLabel = (key: string) => fieldChoices.find((f) => `fields.${f.id}` === key)?.name ?? key;
  // Operators for a condition field: the built-in fields have their own; a custom field supports set/unset and comparisons.
  const opsFor = (field: string): readonly string[] => conditionOps[field] ?? ["set", "unset", "eq", "neq", "gt", "lt"];
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
    try { setDry(await api<DryRun>(`/api/v1/spaces/${spaceId}/automation-rules/dry-run`, { method: "POST", body: JSON.stringify({ definition: cleanDefinition(value), item_id: itemId, ...(ruleId ? { rule_id: ruleId } : {}) }) })); }
    catch (err) { setDryError(err instanceof ApiError ? err.message : (err as Error).message); }
  }

  return <div className="space-y-5">
    {step === "when" && <fieldset id="step-when" className="scroll-mt-4 space-y-2"><legend className="font-medium">{t("automation.when")}</legend>
      <label className="block space-y-1 text-sm"><span>{t("automation.trigger")}</span>
        <select className={select} value={w.type} onChange={(e) => onChange({ ...value, when: { type: e.target.value } })}>
          {triggerTypes.filter((type) => scope !== "workspace" || type !== "item.status_changed").map((type) => <option key={type} value={type}>{t(`automation.triggers.${type}`)}</option>)}</select></label>
      {w.type === "item.status_changed" && <label className="block space-y-1 text-sm"><span>{t("automation.toStatus")}</span>
        <select className={select} value={w.to_status_id ?? ""} onChange={(e) => setWhen({ to_status_id: e.target.value || undefined })}>
          <option value="">{t("automation.anyStatus")}</option>{statuses.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}</select></label>}
      {w.type === "item.assigned" && <ItemUserPicker label={t("automation.assignedUser")} selected={people.when ?? []} maxSelected={1} onChange={(list) => pick("when", list, (ids) => setWhen({ user_id: ids[0] }))} />}
      {w.type === "item.created" && <label className="block space-y-1 text-sm"><span>{t("automation.createdType")}</span>
        <select className={select} value={w.item_type ?? ""} onChange={(e) => setWhen({ item_type: e.target.value || undefined })}>
          <option value="">{t("automation.anyType")}</option>{types.map((def) => <option key={def.key} value={def.key}>{def.name}</option>)}</select></label>}
      {w.type === "item.due_reached" && <label className="block space-y-1 text-sm"><span>{t("automation.daysBefore")}</span>
        <Input type="number" min={-30} max={30} value={w.days_before ?? 0} onChange={(e) => setWhen({ days_before: Number(e.target.value) })} />
        <span className="block text-xs text-muted-foreground">{t("automation.daysBeforeHint")}</span></label>}
      {w.type === "item.due_reached" && <label className="block space-y-1 text-sm"><span>{t("automation.calendar")}</span>
        <select className={select} value={w.calendar ?? ""} onChange={(e) => setWhen({ calendar: e.target.value || undefined })}>
          <option value="">{t("automation.calendarDays")}</option><option value="work">{t("automation.calendarWork")}</option></select></label>}
      {w.type === "item.inactive" && <label className="block space-y-1 text-sm"><span>{t("automation.inactiveDays")}</span>
        <Input type="number" min={1} max={365} value={w.days ?? 7} onChange={(e) => setWhen({ days: Number(e.target.value) })} /></label>}
      {w.type === "approval.decided" && <label className="block space-y-1 text-sm"><span>{t("automation.decision")}</span>
        <select className={select} value={w.decision ?? ""} onChange={(e) => setWhen({ decision: e.target.value || undefined })}>
          <option value="">{t("automation.anyDecision")}</option><option value="approved">{t("automation.decisionApproved")}</option><option value="rejected">{t("automation.decisionRejected")}</option></select></label>}
      {w.type === "form.submitted" && <label className="block space-y-1 text-sm"><span>{t("automation.form")}</span>
        <select className={select} value={w.form_id ?? ""} onChange={(e) => setWhen({ form_id: e.target.value || undefined })}>
          <option value="">{t("automation.anyForm")}</option>{(forms.data ?? []).map((f) => <option key={f.id} value={f.id}>{f.name}</option>)}</select></label>}
    </fieldset>}

    {step === "if" && <fieldset id="step-if" className="scroll-mt-4 space-y-2"><legend className="font-medium">{t("automation.if")}</legend>
      <p className="text-sm text-muted-foreground">{t("automation.ifHint")}</p>
      {value.if.map((c, i) => <div key={i} className="flex flex-wrap items-end gap-2 text-sm">
        <label className="space-y-1"><span>{t("automation.condField", { number: i + 1 })}</span>
          <select className={select} value={c.field} onChange={(e) => setCond(i, { field: e.target.value, op: opsFor(e.target.value)[0], value: undefined })}>
            {conditionFields.map((f) => <option key={f} value={f}>{t(`automation.fields.${f}`)}</option>)}
            {fieldChoices.map((f) => <option key={f.id} value={`fields.${f.id}`}>{f.name}</option>)}
            {!conditionFields.includes(c.field as never) && !fieldChoices.some((f) => `fields.${f.id}` === c.field) && <option value={c.field}>{fieldLabel(c.field)}</option>}</select></label>
        <label className="space-y-1"><span>{t("automation.condOp", { number: i + 1 })}</span>
          <select className={select} value={c.op} onChange={(e) => setCond(i, { op: e.target.value, value: undefined })}>
            {opsFor(c.field).map((op) => <option key={op} value={op}>{t(`automation.ops.${op}`)}</option>)}</select></label>
        {!["set", "unset", "empty", "not_empty"].includes(c.op) && <label className="space-y-1"><span>{t("automation.condValue", { number: i + 1 })}</span>
          {c.op === "in" && ["importance", "status_group", "item_type"].includes(c.field) ? <Input value={c.value ?? ""} placeholder={t("automation.inHint")} onChange={(e) => setCond(i, { value: e.target.value })} />
          : c.field === "importance" ? <select className={select} value={c.value ?? ""} onChange={(e) => setCond(i, { value: e.target.value })}><option value="">—</option>{importanceLevels.map((l) => <option key={l} value={l}>{t(`items.importance.${l}`)}</option>)}</select>
            : c.field === "status_group" ? <select className={select} value={c.value ?? ""} onChange={(e) => setCond(i, { value: e.target.value })}><option value="">—</option>{statusGroups.map((g) => <option key={g} value={g}>{t(`settings.workflows.group.${g}`)}</option>)}</select>
            : c.field === "item_type" ? <select className={select} value={c.value ?? ""} onChange={(e) => setCond(i, { value: e.target.value })}><option value="">—</option>{types.map((def) => <option key={def.key} value={def.key}>{def.name}</option>)}</select>
            : c.field === "assignee" ? <ItemUserPicker label={t("automation.condUser", { number: i + 1 })} selected={people[`c${i}`] ?? []} maxSelected={1} onChange={(list) => pick(`c${i}`, list, (ids) => setCond(i, { value: ids[0] }))} />
            : <Input value={c.value ?? ""} placeholder={t("automation.dateHint")} onChange={(e) => setCond(i, { value: e.target.value })} />}</label>}
        <Button type="button" variant="ghost" size="sm" aria-label={t("automation.removeCond", { number: i + 1 })} onClick={() => onChange({ ...value, if: value.if.filter((_, j) => j !== i) })}>×</Button>
      </div>)}
      <Button type="button" variant="outline" size="sm" disabled={value.if.length >= 10} onClick={() => onChange({ ...value, if: [...value.if, { field: "importance", op: "eq" }] })}>{t("automation.addCond")}</Button>
    </fieldset>}

    {step === "then" && <fieldset id="step-then" className="scroll-mt-4 space-y-3"><legend className="font-medium">{t("automation.then")}</legend>
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
              <option value="">—</option>{setFields.filter((f) => (scope === "workspace" ? f !== "status_id" : f !== "status_group")).map((f) => <option key={f} value={f}>{t(`automation.setFields.${f}`)}</option>)}
              {fieldChoices.map((f) => <option key={f.id} value={`fields.${f.id}`}>{f.name}</option>)}</select></label>
          <label className="space-y-1"><span>{t("automation.setValue", { number: i + 1 })}</span>
            {a.field === "importance" ? <select className={select} value={String(a.value ?? "")} onChange={(e) => setAction(i, { value: e.target.value })}><option value="">—</option>{importanceLevels.map((l) => <option key={l} value={l}>{t(`items.importance.${l}`)}</option>)}</select>
              : a.field === "status_id" ? <select className={select} value={String(a.value ?? "")} onChange={(e) => setAction(i, { value: e.target.value })}><option value="">—</option>{statuses.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}</select>
              : a.field === "status_group" ? <select className={select} value={String(a.value ?? "")} onChange={(e) => setAction(i, { value: e.target.value })}><option value="">—</option>{statusGroups.map((g) => <option key={g} value={g}>{t(`settings.workflows.group.${g}`)}</option>)}</select>
              : a.field?.startsWith("fields.") && fieldChoices.find((f) => `fields.${f.id}` === a.field)?.type === "single_select" ? <Input value={String(a.value ?? "")} onChange={(e) => setAction(i, { value: e.target.value })} />
              : a.field === "due_date" || a.field === "start_date" ? <Input type="date" value={String(a.value ?? "")} onChange={(e) => setAction(i, { value: e.target.value || null })} />
              : <Input type="number" min={0} value={a.value === undefined || a.value === null ? "" : String(a.value)} onChange={(e) => setAction(i, { value: e.target.value === "" ? null : Number(e.target.value) })} />}</label>
        </div>}
        {a.type === "assign" && <ItemUserPicker multiple maxSelected={20} label={t("automation.assignUsers", { number: i + 1 })} selected={people[`a${i}`] ?? []} onChange={(list) => pick(`a${i}`, list, (ids) => setAction(i, { user_ids: ids }))} />}
        {a.type === "comment" && <label className="block space-y-1"><span>{t("automation.commentText", { number: i + 1 })}</span>
          <Input value={a.text ?? ""} maxLength={2000} onChange={(e) => setAction(i, { text: e.target.value })} /></label>}
        {a.type === "mention" && <div className="space-y-2">
          <label className="block space-y-1"><span>{t("automation.mentionText", { number: i + 1 })}</span>
            <Input value={a.text ?? ""} maxLength={2000} onChange={(e) => setAction(i, { text: e.target.value })} /></label>
          <ItemUserPicker multiple maxSelected={20} label={t("automation.mentionUsers", { number: i + 1 })} selected={people[`m${i}`] ?? []} onChange={(list) => pick(`m${i}`, list, (ids) => setAction(i, { user_ids: ids }))} />
          {(groups.data ?? []).length > 0 && <fieldset className="space-y-1 text-sm">
            <legend>{t("automation.mentionGroups", { number: i + 1 })}</legend>
            {(groups.data ?? []).map((g) => <label key={g.id} className="flex items-center gap-2"><input type="checkbox" checked={(a.group_ids ?? []).includes(g.id)}
              onChange={(e) => setAction(i, { group_ids: e.target.checked ? [...(a.group_ids ?? []), g.id].slice(0, 5) : (a.group_ids ?? []).filter((id) => id !== g.id) })} />{g.name}</label>)}
          </fieldset>}
          <label className="flex items-center gap-2"><input type="checkbox" checked={!!a.followers} onChange={(e) => setAction(i, { followers: e.target.checked })} />{t("automation.mentionFollowers", { number: i + 1 })}</label>
        </div>}
        {a.type === "move" && <MoveTarget number={i + 1} spaceId={spaceId} chosen={a.container_id} onChoose={(id) => setAction(i, { container_id: id })} />}
        {a.type === "notify" && <div className="space-y-2">
          <ItemUserPicker multiple maxSelected={20} label={t("automation.notifyUsers", { number: i + 1 })} selected={people[`n${i}`] ?? []} onChange={(list) => pick(`n${i}`, list, (ids) => setAction(i, { user_ids: ids }))} />
          <label className="flex items-center gap-2"><input type="checkbox" checked={!!a.assignees} onChange={(e) => setAction(i, { assignees: e.target.checked })} />{t("automation.notifyAssignees", { number: i + 1 })}</label>
          <label className="block space-y-1"><span>{t("automation.notifyMessage", { number: i + 1 })}</span><Input value={a.message ?? ""} maxLength={500} onChange={(e) => setAction(i, { message: e.target.value })} /></label></div>}
        {a.type === "create_item" && <div className="grid gap-2 sm:grid-cols-2">
          <label className="space-y-1 sm:col-span-2"><span>{t("automation.newItemTitle", { number: i + 1 })}</span><Input value={a.title ?? ""} maxLength={500} onChange={(e) => setAction(i, { title: e.target.value })} /></label>
          <label className="flex items-center gap-2 sm:col-span-2"><input type="checkbox" checked={a.under === "item"} onChange={(e) => setAction(i, { under: e.target.checked ? "item" : undefined })} />{t("automation.asSubitem", { number: i + 1 })}</label>
          <label className="space-y-1"><span>{t("automation.newType", { number: i + 1 })}</span>
            <select className={select} value={a.type_key ?? ""} onChange={(e) => setAction(i, { type_key: e.target.value || undefined })}>
              <option value="">{t("items.task")}</option>{types.filter((def) => !def.is_builtin && def.active).map((def) => <option key={def.key} value={def.key}>{def.name}</option>)}</select></label>
        </div>}
      </div>)}
      <Button type="button" variant="outline" size="sm" disabled={value.then.length >= 10} onClick={() => onChange({ ...value, then: [...value.then, { type: "comment", text: "" }] })}>{t("automation.addAction")}</Button>
    </fieldset>}

    {step === "test" && <fieldset id="step-test" className="scroll-mt-4 space-y-2"><legend className="font-medium">{t("automation.test")}</legend>
      <p className="text-sm text-muted-foreground">{t("automation.testHint")}</p>
      <label className="block space-y-1 text-sm"><span>{t("automation.testItem")}</span><Input value={itemText} onChange={(e) => setItemText(e.target.value)} /></label>
      {(search.data?.items ?? []).length > 0 && <ul className="rounded border text-sm">{search.data!.items.map((it) => <li key={it.id}>
        <Button type="button" variant="ghost" className="w-full justify-start" aria-label={t("automation.testOn", { title: it.title })} onClick={() => void testOn(it.id)}>{it.title}</Button></li>)}</ul>}
      <FormError message={dryError} />
      {dry && <div role="region" aria-label={t("automation.testResult")} className="space-y-1 rounded border p-3 text-sm">
        {dry.trace && dry.trace.length > 0 && <ol aria-label={t("automation.trace")} className="list-decimal space-y-0.5 pl-5">{dry.trace.map((line, i) => <li key={i}>{line}</li>)}</ol>}
        <p className="font-medium">{dry.would_run ? t("automation.wouldRun", { title: dry.item_title }) : t("automation.wouldNot", { title: dry.item_title })}</p>
        <ul className="list-disc pl-5">{dry.conditions.map((c) => <li key={c.index}>{t("automation.condResult", { number: c.index + 1 })}: {c.holds ? t("automation.holds") : t("automation.fails")}</li>)}</ul>
        {dry.actions.length > 0 && <ul className="list-disc pl-5">{dry.actions.map((line, i) => <li key={i}>{line}</li>)}</ul>}
      </div>}
    </fieldset>}
    <FormError message={error ?? ""} />
  </div>;
}

/** Picks the project or folder a move action lands in: a search in this Space, limited to projects and folders. */
function MoveTarget({ number, spaceId, chosen, onChoose }: { number: number; spaceId: string; chosen?: string; onChoose: (id: string) => void }) {
  const { t } = useTranslation();
  const [text, setText] = useState("");
  const found = useQuery({ queryKey: ["automation-move-search", text, spaceId], enabled: text.trim().length >= 2, retry: false,
    queryFn: () => api<{ items: { id: string; title: string; type: string }[] }>(`/api/v1/search?q=${encodeURIComponent(text.trim())}&space_id=${spaceId}&limit=10`) });
  const containers = (found.data?.items ?? []).filter((it) => it.type === "project" || it.type === "folder");
  return <div className="space-y-2">
    <label className="block space-y-1"><span>{t("automation.moveTarget", { number })}</span>
      <Input value={text} placeholder={t("automation.moveSearch")} onChange={(e) => setText(e.target.value)} /></label>
    {chosen && <p className="text-xs text-muted-foreground">{chosen}</p>}
    {text.trim().length >= 2 && containers.length === 0 && !found.isPending && <p className="text-xs text-muted-foreground">{t("automation.moveNone")}</p>}
    {containers.length > 0 && <ul className="rounded border text-sm">{containers.map((it) => <li key={it.id}>
      <Button type="button" variant="ghost" className="w-full justify-start" aria-label={t("automation.moveTo", { title: it.title })} onClick={() => onChoose(it.id)}>{it.title}</Button></li>)}</ul>}
  </div>;
}
