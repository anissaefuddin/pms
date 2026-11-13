import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import type { WorkflowStatus } from "@/lib/status-colors";
import { ItemUserPicker, type PersonOption } from "@/components/item-user-picker";
import { Button } from "@/components/ui/button";

const builtinFields = ["due_date", "start_date", "assignees", "description", "effort_minutes"] as const;
const rules = ["anyone", "assignee", "admin"] as const;
const selectClass = "rounded border bg-background px-2 py-1 text-sm";

/** Guard and automatic approval of one status (WF-04, WF-05, WF-03B). Every control saves through the status PATCH. */
export function StatusGuardEditor({ status, siblings, busy, onPatch }: {
  status: WorkflowStatus; siblings: WorkflowStatus[]; busy: boolean; onPatch: (patch: object) => void;
}) {
  const { t } = useTranslation();
  const required = status.required_fields ?? [];
  const approvers = status.approval_approver_ids ?? [];
  const [picked, setPicked] = useState<PersonOption[]>([]);
  // Names of saved approvers come from the user directory; unknown ones are shown by id until it answers.
  const names = useQuery({ queryKey: ["status-approvers", approvers.join(",")], enabled: approvers.length > 0, retry: false,
    queryFn: async () => Promise.all(approvers.map(async (id) => {
      try { const user = await api<{ first_name: string; last_name: string }>(`/api/v1/users/${id}`); return { id, name: `${user.first_name} ${user.last_name}`.trim() }; }
      catch { return { id, name: id }; }
    })) });
  const selected = picked.length || approvers.length === 0 ? picked : names.data ?? [];
  const toggle = (key: string) => onPatch({ required_fields: required.includes(key) ? required.filter((k) => k !== key) : [...required, key] });
  const targets = siblings.filter((s) => s.id !== status.id);
  return <fieldset disabled={busy} className="space-y-2 pl-6">
    <legend className="mb-1 text-xs font-medium text-muted-foreground">{t("settings.workflows.guard.title")}: {status.name}</legend>
    <div className="flex flex-wrap gap-3">
      {builtinFields.map((key) => <label key={key} className="flex items-center gap-1 text-sm">
        <input type="checkbox" checked={required.includes(key)} onChange={() => toggle(key)} />
        {t(`settings.workflows.guard.field.${key}`)}<span className="sr-only">: {status.name}</span></label>)}
    </div>
    <div className="flex flex-wrap items-center gap-3">
      <label className="flex items-center gap-1 text-sm">{t("settings.workflows.guard.actor")}
        <select className={selectClass} aria-label={`${t("settings.workflows.guard.actor")}: ${status.name}`} value={status.actor_rule ?? "anyone"}
          onChange={(event) => onPatch({ actor_rule: event.target.value })}>
          {rules.map((rule) => <option key={rule} value={rule}>{t(`settings.workflows.guard.rule.${rule}`)}</option>)}
        </select></label>
      <label className="flex items-center gap-1 text-sm">
        <input type="checkbox" checked={!!status.require_children_done} onChange={(event) => onPatch({ require_children_done: event.target.checked })} />
        {t("settings.workflows.guard.childrenDone")}<span className="sr-only">: {status.name}</span></label>
    </div>
    <div className="space-y-2">
      <ItemUserPicker multiple label={`${t("settings.workflows.guard.approvers")}: ${status.name}`} selected={selected} onChange={setPicked} />
      <div className="flex flex-wrap items-center gap-3">
        <Button type="button" variant="outline" size="sm" disabled={busy}
          onClick={() => onPatch({ approval_approver_ids: picked.map((person) => person.id) })}>{t("settings.workflows.guard.saveApprovers")}</Button>
        <label className="flex items-center gap-1 text-sm">{t("settings.workflows.guard.approvedTo")}
          <select className={selectClass} aria-label={`${t("settings.workflows.guard.approvedTo")}: ${status.name}`} value={status.approved_status_id ?? ""}
            onChange={(event) => onPatch({ approved_status_id: event.target.value || null })}>
            <option value="">{t("settings.workflows.guard.stay")}</option>
            {targets.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select></label>
        <label className="flex items-center gap-1 text-sm">{t("settings.workflows.guard.rejectedTo")}
          <select className={selectClass} aria-label={`${t("settings.workflows.guard.rejectedTo")}: ${status.name}`} value={status.rejected_status_id ?? ""}
            onChange={(event) => onPatch({ rejected_status_id: event.target.value || null })}>
            <option value="">{t("settings.workflows.guard.stay")}</option>
            {targets.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select></label>
      </div>
    </div>
  </fieldset>;
}
