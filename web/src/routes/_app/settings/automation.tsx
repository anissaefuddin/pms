import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { ApiError, api } from "@/lib/api";
import { formatDateTime, useProfile } from "@/lib/profile";
import type { AutomationRule } from "@/lib/automation";
import { AdminPageHeader } from "@/components/admin/admin-ui";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { TargetsDialog, WorkspaceLimitsCard, WorkspaceRuleDialog, workspaceRulesKey } from "@/components/workspace-automation";
import { useSpaces } from "@/lib/space-queries";

export const Route = createFileRoute("/_app/settings/automation")({ component: WorkspaceAutomationSettings });

/** Workspace Settings > Automation (AUT-02): every workspace rule, its Spaces, and the execution caps. Workspace admins only. */
function WorkspaceAutomationSettings() {
  const { t } = useTranslation();
  const profile = useProfile();
  const client = useQueryClient();
  const spaces = useSpaces();
  const [editing, setEditing] = useState<AutomationRule | "new">();
  const [targeting, setTargeting] = useState<AutomationRule>();
  const [error, setError] = useState("");
  const rules = useQuery({ queryKey: workspaceRulesKey, enabled: profile?.user_type === "admin", retry: false,
    queryFn: async () => (await api<{ items: AutomationRule[] }>("/api/v1/automation/workspace-rules")).items });

  if (profile && profile.user_type !== "admin") {
    return <section className="space-y-3"><p role="alert">{t("automation.workspaceAdminOnly")}</p></section>;
  }

  const nameOf = (id: string) => spaces.data?.find((s) => s.id === id)?.name;
  async function toggle(rule: AutomationRule, enabled: boolean) {
    setError("");
    try {
      await api(`/api/v1/automation-rules/${rule.id}`, { method: "PATCH", body: JSON.stringify({ enabled }) });
      await client.invalidateQueries({ queryKey: workspaceRulesKey });
    } catch (err) { setError(err instanceof ApiError ? err.message : (err as Error).message); }
  }

  return <section className="space-y-6">
    <AdminPageHeader description={t("automation.workspaceDescription")}
      action={<Button onClick={() => setEditing("new")}>{t("automation.newWorkspaceRule")}</Button>} />
    <FormError message={error} />
    <WorkspaceLimitsCard />
    {rules.isPending && <p role="status" className="text-sm text-muted-foreground">{t("automation.loading")}</p>}
    {rules.isError && <p role="alert">{t("automation.error")}</p>}
    {rules.data && rules.data.length === 0 && <p className="text-sm text-muted-foreground">{t("automation.noWorkspaceRules")}</p>}
    {rules.data && rules.data.length > 0 && <div className="overflow-x-auto"><table className="w-full text-left text-sm">
      <thead><tr className="border-b">
        <th scope="col" className="py-2 pr-4">{t("automation.name")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.stateCol")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.spacesCol")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.owner")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.lastRun")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.errors")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.enabled")}</th>
        <th scope="col" className="py-2"><span className="sr-only">{t("automation.rowActions")}</span></th></tr></thead>
      <tbody>{rules.data.map((rule) => <tr key={rule.id} className="border-b align-top">
        <td className="py-2 pr-4 font-medium">{rule.name}</td>
        <td className="py-2 pr-4">{rule.health !== "ok" ? t(`automation.health.${rule.health}`, { reason: t(`automation.reasons.${rule.health_reason}`, { defaultValue: rule.health_reason }) })
          : rule.current_version_id ? t("automation.published") : t("automation.draftOnly")}</td>
        <td className="py-2 pr-4">{rule.target_space_ids.length === 0 ? t("automation.noTargets") : rule.target_space_ids.map((id) => nameOf(id) ?? t("automation.hiddenSpace")).join(", ")}</td>
        <td className="py-2 pr-4">{rule.owner_name}</td>
        <td className="py-2 pr-4 whitespace-nowrap">{rule.last_run_at ? <>{formatDateTime(rule.last_run_at, profile)} <span className="text-muted-foreground">{t(`automation.runStatus.${rule.last_run_status}`, { defaultValue: rule.last_run_status ?? "" })}</span></> : t("automation.neverRun")}</td>
        <td className="py-2 pr-4">{rule.error_count}</td>
        <td className="py-2 pr-4"><label className="flex items-center gap-2"><input type="checkbox" checked={rule.enabled} onChange={(e) => void toggle(rule, e.target.checked)} />
          <span className="sr-only">{t("automation.enable", { name: rule.name })}</span>{rule.enabled ? t("automation.on") : t("automation.off")}</label></td>
        <td className="py-2"><div className="flex flex-wrap justify-end gap-2">
          <Button variant="outline" size="sm" aria-label={t("automation.editRule", { name: rule.name })} onClick={() => setEditing(rule)}>{t("automation.edit")}</Button>
          <Button variant="outline" size="sm" aria-label={t("automation.targetsOf", { name: rule.name })} onClick={() => setTargeting(rule)}>{t("automation.targets")}</Button>
        </div></td></tr>)}</tbody>
    </table></div>}
    {editing && <WorkspaceRuleDialog rule={editing === "new" ? undefined : editing} onClose={() => setEditing(undefined)} />}
    {targeting && <TargetsDialog rule={targeting} onClose={() => setTargeting(undefined)} />}
  </section>;
}
