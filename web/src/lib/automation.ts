import type { components } from "@/api/schema";

export type AutomationRun = components["schemas"]["AutomationRun"];
export type AutomationVersion = components["schemas"]["AutomationVersion"];
export type AutomationRule = components["schemas"]["AutomationRule"];

export type Trigger = { type: string; to_status_id?: string; user_id?: string; item_type?: string; days_before?: number; form_id?: string };
export type Condition = { field: string; op: string; value?: string };
export type Action = { type: string; field?: string; value?: unknown; user_ids?: string[]; assignees?: boolean; text?: string; message?: string; title?: string; type_key?: string; kind?: string };
export type Definition = { when: Trigger; if: Condition[]; then: Action[] };

export const triggerTypes = ["item.status_changed", "item.assigned", "item.created", "item.due_reached", "form.submitted"] as const;
export const conditionFields = ["importance", "status_group", "item_type", "assignee", "due_date", "start_date"] as const;
export const conditionOps: Record<string, readonly string[]> = {
  importance: ["eq", "neq"], status_group: ["eq", "neq"], item_type: ["eq", "neq"],
  assignee: ["has", "has_not", "empty", "not_empty"], due_date: ["set", "unset", "before", "after"], start_date: ["set", "unset", "before", "after"],
};
export const actionTypes = ["set_field", "assign", "comment", "notify", "create_item"] as const;
export const setFields = ["importance", "due_date", "start_date", "effort_minutes", "duration_days", "status_id"] as const;
export const statusGroups = ["active", "completed", "deferred", "cancelled"] as const;
export const importanceLevels = ["high", "normal", "low"] as const;

export const emptyDefinition = (): Definition => ({ when: { type: "item.status_changed" }, if: [], then: [{ type: "comment", text: "" }] });

/** Drops the settings that do not belong to the chosen trigger or action, which the server would refuse. */
export function cleanDefinition(d: Definition): Definition {
  const w = d.when;
  const when: Trigger = { type: w.type };
  if (w.type === "item.status_changed" && w.to_status_id) when.to_status_id = w.to_status_id;
  if (w.type === "item.assigned" && w.user_id) when.user_id = w.user_id;
  if (w.type === "item.created" && w.item_type) when.item_type = w.item_type;
  if (w.type === "item.due_reached") when.days_before = Number(w.days_before ?? 0);
  if (w.type === "form.submitted" && w.form_id) when.form_id = w.form_id;
  const conds = d.if.map((c): Condition => (["set", "unset", "empty", "not_empty"].includes(c.op) ? { field: c.field, op: c.op } : { field: c.field, op: c.op, value: c.value }));
  const then = d.then.map((a): Action => {
    switch (a.type) {
      case "set_field": return { type: a.type, field: a.field, value: a.value };
      case "assign": return { type: a.type, user_ids: a.user_ids ?? [] };
      case "comment": return { type: a.type, text: a.text ?? "" };
      case "notify": return { type: a.type, ...(a.user_ids?.length ? { user_ids: a.user_ids } : {}), ...(a.assignees ? { assignees: true } : {}), message: a.message ?? "" };
      default: return { type: a.type, title: a.title ?? "", ...(a.type_key ? { type_key: a.type_key } : { kind: a.kind || "task" }) };
    }
  });
  return { when, if: conds, then };
}

export const ruleKeys = { list: (space: string) => ["automation", space] as const, one: (id: string) => ["automation", "rule", id] as const };
