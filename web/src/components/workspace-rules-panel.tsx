import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { ApiError, api } from "@/lib/api";
import { formatDateTime, useProfile } from "@/lib/profile";
import type { AutomationRule } from "@/lib/automation";
import { useSpaces } from "@/lib/space-queries";
import { FormError } from "@/components/auth/form-error";
import { RuleToolbar, SortHeader, filterRules, type RuleSort, type RuleState } from "@/components/automation-rule-list";
import { TargetsDialog, WorkspaceLimitsCard, WorkspaceRuleDialog, workspaceRulesKey, type UsageCounter } from "@/components/workspace-automation";
import { Button } from "@/components/ui/button";

/** Settings > Automation > Workspace: every workspace rule with its targets, state and last run. Workspace admins only; the server checks each call. */
export function WorkspaceRulesPanel() {
  const { t } = useTranslation();
  const profile = useProfile();
  const client = useQueryClient();
  const spaces = useSpaces();
  const [editing, setEditing] = useState<AutomationRule | "new">();
  const [targeting, setTargeting] = useState<AutomationRule>();
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [state, setState] = useState<RuleState>("all");
  const [sort, setSort] = useState<RuleSort>({ key: "name", dir: 1 });
  const rules = useQuery({ queryKey: workspaceRulesKey, retry: false,
    queryFn: async () => (await api<{ items: AutomationRule[] }>("/api/v1/automation/workspace-rules")).items });
  const usage = useQuery({ queryKey: ["automation", "usage", "workspace"], retry: false,
    queryFn: () => api<{ workspace: UsageCounter | null; window: string }>("/api/v1/automation/usage") });
  const visible = filterRules(rules.data ?? [], search, state, sort);

  const nameOf = (id: string) => spaces.data?.find((space) => space.id === id)?.name;
  async function toggle(rule: AutomationRule, enabled: boolean) {
    setError("");
    try {
      await api(`/api/v1/automation-rules/${rule.id}`, { method: "PATCH", body: JSON.stringify({ enabled }) });
      await client.invalidateQueries({ queryKey: workspaceRulesKey });
    } catch (err) { setError(err instanceof ApiError ? err.message : (err as Error).message); }
  }

  return <section className="space-y-5" aria-labelledby="workspace-rules-title">
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div className="space-y-1">
        <h2 id="workspace-rules-title" className="text-lg font-semibold">{t("automation.tab.workspace")}</h2>
        <p className="text-sm text-muted-foreground">{t("automation.workspaceDescription")}</p>
      </div>
      <Button onClick={() => setEditing("new")}>{t("automation.newWorkspaceRule")}</Button>
    </div>

    <details className="group rounded-lg border">
      <summary className="flex cursor-pointer flex-wrap items-center gap-2 px-4 py-3 text-sm font-medium">
        {t("automation.usageAndLimits")}
        {usage.data?.workspace && <span className="font-normal text-muted-foreground" data-state={usage.data.workspace.state}>
          {t("automation.usage.workspaceLabel")}: {usage.data.workspace.used} / {usage.data.workspace.limit}</span>}
      </summary>
      <div className="border-t p-4"><WorkspaceLimitsCard /></div>
    </details>

    <RuleToolbar search={search} onSearch={setSearch} state={state} onState={setState} />
    <FormError message={error} />
    {rules.isPending && <p role="status" className="text-sm text-muted-foreground">{t("automation.loading")}</p>}
    {rules.isError && <p role="alert">{t("automation.error")}</p>}
    {rules.data && rules.data.length === 0 && <p className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">{t("automation.noWorkspaceRules")}</p>}
    {rules.data && rules.data.length > 0 && visible.length === 0 && <p role="status" className="text-sm text-muted-foreground">{t("automation.noMatch")}</p>}
    {visible.length > 0 && <div className="overflow-x-auto rounded-lg border"><table className="w-full text-left text-sm">
      <thead className="border-b bg-muted/40"><tr>
        <SortHeader labelKey="automation.name" sortKey="name" sort={sort} onSort={setSort} />
        <th scope="col" className="py-2 pr-4 font-medium">{t("automation.trigger")}</th>
        <th scope="col" className="py-2 pr-4 font-medium">{t("automation.spacesCol")}</th>
        <SortHeader labelKey="automation.stateCol" sortKey="state" sort={sort} onSort={setSort} />
        <th scope="col" className="py-2 pr-4 font-medium">{t("automation.owner")}</th>
        <SortHeader labelKey="automation.lastRun" sortKey="last_run" sort={sort} onSort={setSort} />
        <th scope="col" className="py-2 pr-4 font-medium">{t("automation.errors")}</th>
        <th scope="col" className="py-2 pr-4 font-medium">{t("automation.enabled")}</th>
        <th scope="col" className="py-2"><span className="sr-only">{t("automation.rowActions")}</span></th></tr></thead>
      <tbody className="divide-y">{visible.map((rule) => <tr key={rule.id} className="align-top">
        <td className="py-2 pr-4 font-medium">{rule.name}</td>
        <td className="py-2 pr-4">{rule.trigger ? t(`automation.triggers.${rule.trigger}`, { defaultValue: rule.trigger }) : "—"}</td>
        <td className="py-2 pr-4">{(rule.target_space_ids ?? []).length === 0 ? t("automation.noTargets") : (rule.target_space_ids ?? []).map((id) => nameOf(id) ?? t("automation.hiddenSpace")).join(", ")}</td>
        <td className="py-2 pr-4">{rule.health !== "ok" ? t(`automation.health.${rule.health}`, { reason: t(`automation.reasons.${rule.health_reason}`, { defaultValue: rule.health_reason }) })
          : rule.current_version_id ? t("automation.published") : t("automation.draftOnly")}</td>
        <td className="py-2 pr-4">{rule.owner_name}</td>
        <td className="py-2 pr-4 whitespace-nowrap">{rule.last_run_at ? <>{formatDateTime(rule.last_run_at, profile)} <span className="text-muted-foreground">{t(`automation.runStatus.${rule.last_run_status}`, { defaultValue: rule.last_run_status ?? "" })}</span></> : t("automation.neverRun")}</td>
        <td className="py-2 pr-4">{rule.error_count}</td>
        <td className="py-2 pr-4"><label className="flex items-center gap-2"><input type="checkbox" checked={rule.enabled} onChange={(e) => void toggle(rule, e.target.checked)} />
          <span className="sr-only">{t("automation.enable", { name: rule.name })}</span>{rule.enabled ? t("automation.on") : t("automation.off")}</label></td>
        <td className="py-2 pr-3"><div className="flex flex-wrap justify-end gap-2">
          <Button variant="outline" size="sm" aria-label={t("automation.editRule", { name: rule.name })} onClick={() => setEditing(rule)}>{t("automation.edit")}</Button>
          <Button variant="outline" size="sm" aria-label={t("automation.targetsOf", { name: rule.name })} onClick={() => setTargeting(rule)}>{t("automation.targets")}</Button>
        </div></td></tr>)}</tbody>
    </table></div>}
    {editing && <WorkspaceRuleDialog rule={editing === "new" ? undefined : editing} onClose={() => setEditing(undefined)} />}
    {targeting && <TargetsDialog rule={targeting} onClose={() => setTargeting(undefined)} />}
  </section>;
}
