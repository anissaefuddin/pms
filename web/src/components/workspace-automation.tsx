import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import { cleanDefinition, emptyDefinition, type AutomationRule, type Definition } from "@/lib/automation";
import { useSpaces } from "@/lib/space-queries";
import { AutomationEditor, type EditorStep } from "@/components/automation-editor";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

export const workspaceRulesKey = ["automation", "workspace-rules"] as const;

const steps = ["apply", "when", "if", "then", "test"] as const;
type StepId = (typeof steps)[number];

/** Create or edit a workspace rule (AUT-02). The builder works on status categories, so the rule fits every Space. */
export function WorkspaceRuleDialog({ rule, onClose }: { rule?: AutomationRule; onClose: () => void }) {
  const { t } = useTranslation();
  const loaded = useQuery({ queryKey: [...workspaceRulesKey, "one", rule?.id ?? "new"], enabled: !!rule, retry: false,
    queryFn: () => api<AutomationRule>(`/api/v1/automation-rules/${rule!.id}`) });
  if (rule && !loaded.data) {
    return <Dialog open onOpenChange={(open) => !open && onClose()}><DialogContent>
      <DialogTitle>{t("automation.editTitle")}</DialogTitle>
      {loaded.isError ? <FormError message={t("automation.error")} /> : <p role="status">{t("automation.loading")}</p>}
    </DialogContent></Dialog>;
  }
  const latest = loaded.data?.versions?.[0];
  return <WorkspaceRuleForm rule={loaded.data} initial={latest ? (latest.definition as unknown as Definition) : emptyDefinition()} onClose={onClose} />;
}

function WorkspaceRuleForm({ rule, initial, onClose }: { rule?: AutomationRule; initial: Definition; onClose: () => void }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const start = { ...initial, if: initial.if ?? [] };
  const [name, setName] = useState(rule?.name ?? "");
  const [definition, setDefinition] = useState<Definition>(start);
  const [baseline] = useState(() => JSON.stringify({ name: rule?.name ?? "", definition: start }));
  const [step, setStep] = useState<StepId>("apply");
  // Try it runs in one Space, as the rule would there. The owner's rights apply; a new rule uses its creator.
  const spaces = useSpaces();
  const [testSpace, setTestSpace] = useState("");
  const chosenSpace = testSpace || spaces.data?.[0]?.id || "";
  const [confirmDiscard, setConfirmDiscard] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const dirty = JSON.stringify({ name, definition }) !== baseline;
  const requestClose = () => (dirty ? setConfirmDiscard(true) : onClose());

  async function save(publish: boolean) {
    setBusy(true); setError("");
    try {
      const body = cleanDefinition(definition);
      let id = rule?.id;
      if (!id) {
        id = (await api<AutomationRule>("/api/v1/automation/workspace-rules", { method: "POST", body: JSON.stringify({ name: name.trim(), definition: body }) })).id;
      } else {
        if (name.trim() && name.trim() !== rule?.name) await api(`/api/v1/automation-rules/${id}`, { method: "PATCH", body: JSON.stringify({ name: name.trim() }) });
        await api(`/api/v1/automation-rules/${id}/draft`, { method: "PUT", body: JSON.stringify({ definition: body }) });
      }
      if (publish) await api(`/api/v1/automation-rules/${id}/publish`, { method: "POST" });
      await client.invalidateQueries({ queryKey: workspaceRulesKey });
      toast.success(t(publish ? "automation.published" : "automation.draftSaved"));
      onClose();
    } catch (err) { setError(err instanceof ApiError ? err.message : (err as Error).message); } finally { setBusy(false); }
  }

  return <>
  <Dialog open onOpenChange={(open) => !open && requestClose()}>
    <DialogContent className="h-dvh max-h-dvh w-screen max-w-none overflow-y-auto rounded-none sm:max-w-none">
      <DialogHeader><DialogTitle>{rule ? t("automation.editTitle") : t("automation.newWorkspaceRule")}</DialogTitle>
        <DialogDescription>{t("automation.workspaceDialogHint")}</DialogDescription></DialogHeader>
      <div className="grid gap-6 md:grid-cols-[11rem_minmax(0,1fr)]">
        <nav aria-label={t("automation.steps")} className="flex flex-row flex-wrap gap-1 text-sm md:flex-col">
          {steps.map((id) => <Button key={id} type="button" variant={step === id ? "default" : "ghost"} size="sm" className="justify-start"
            aria-current={step === id ? "step" : undefined} onClick={() => setStep(id)}>{t(`automation.step-${id}-label`)}</Button>)}
        </nav>
        <div className="min-w-0 space-y-5">
          {step === "apply" && <section className="space-y-4 text-sm">
            <h3 className="font-medium">{t("automation.applyTo")}</h3>
            <p className="text-muted-foreground">{t("automation.appliesToWorkspace")}</p>
            <label className="block space-y-1"><span>{t("automation.ruleName")}</span><Input value={name} maxLength={120} onChange={(e) => setName(e.target.value)} required /></label>
          </section>}
          {step === "test" && <section className="space-y-4">
            <label className="block max-w-sm space-y-1 text-sm"><span>{t("automation.trySpace")}</span>
              <select className="block w-full rounded-lg border border-input bg-background px-2 py-1.5 text-sm" value={chosenSpace} onChange={(e) => setTestSpace(e.target.value)}>
                {(spaces.data ?? []).map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}</select></label>
            {chosenSpace ? <AutomationEditor key={chosenSpace} spaceId={chosenSpace} ruleId={rule?.id} scope="workspace" value={definition} onChange={setDefinition} error={error} step="test" />
              : <p role="status" className="text-sm text-muted-foreground">{t("automation.noSpaceToTry")}</p>}
          </section>}
          {step !== "apply" && step !== "test" && <AutomationEditor spaceId="" scope="workspace" value={definition} onChange={setDefinition} error={error} step={step as EditorStep} />}
        </div>
      </div>
      <DialogFooter>
        <Button type="button" variant="outline" onClick={requestClose}>{t("automation.cancel")}</Button>
        <Button type="button" variant="outline" disabled={busy || !name.trim()} onClick={() => void save(false)}>{t("automation.saveDraft")}</Button>
        <Button type="button" disabled={busy || !name.trim()} onClick={() => void save(true)}>{t("automation.publish")}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
  <Dialog open={confirmDiscard} onOpenChange={setConfirmDiscard}><DialogContent>
    <DialogTitle>{t("automation.discardTitle")}</DialogTitle>
    <DialogDescription>{t("automation.discardDescription")}</DialogDescription>
    <DialogFooter>
      <Button type="button" variant="outline" onClick={() => setConfirmDiscard(false)}>{t("automation.keepEditing")}</Button>
      <Button type="button" onClick={onClose}>{t("automation.discard")}</Button>
    </DialogFooter>
  </DialogContent></Dialog>
  </>;
}

/** Which Spaces a workspace rule may run in (D2). Select all takes a snapshot of the Spaces visible to this admin. Spaces the admin cannot see stay as they are. */
export function TargetsDialog({ rule, onClose }: { rule: AutomationRule; onClose: () => void }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const spaces = useSpaces();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const visible = spaces.data ?? [];
  const visibleIds = new Set(visible.map((s) => s.id));
  const hidden = (rule.target_space_ids ?? []).filter((id) => !visibleIds.has(id));
  const [picked, setPicked] = useState<string[] | undefined>(undefined);
  const current = picked ?? (rule.target_space_ids ?? []).filter((id) => visibleIds.has(id));

  async function save() {
    setBusy(true); setError("");
    try {
      await api(`/api/v1/automation-rules/${rule.id}/targets`, { method: "POST", body: JSON.stringify({ space_ids: [...new Set([...current, ...hidden])] }) });
      await client.invalidateQueries({ queryKey: workspaceRulesKey });
      toast.success(t("automation.targetsSaved"));
      onClose();
    } catch (err) { setError(err instanceof ApiError ? err.message : (err as Error).message); } finally { setBusy(false); }
  }

  return <Dialog open onOpenChange={(open) => !open && onClose()}>
    <DialogContent className="sm:max-w-md">
      <DialogHeader><DialogTitle>{t("automation.targetsTitle", { name: rule.name })}</DialogTitle>
        <DialogDescription>{t("automation.targetsHint")}</DialogDescription></DialogHeader>
      {spaces.isPending && <p role="status" className="text-sm text-muted-foreground">{t("automation.loading")}</p>}
      {spaces.data && <fieldset className="space-y-2 text-sm">
        <legend className="sr-only">{t("automation.targetsTitle", { name: rule.name })}</legend>
        <div className="flex gap-2">
          <Button type="button" variant="outline" size="sm" onClick={() => setPicked(visible.map((s) => s.id))}>{t("automation.selectAll")}</Button>
          <Button type="button" variant="ghost" size="sm" onClick={() => setPicked([])}>{t("automation.clearTargets")}</Button>
        </div>
        <ul className="max-h-72 space-y-1 overflow-y-auto">
          {visible.map((space) => <li key={space.id}><label className="flex items-center gap-2">
            <input type="checkbox" checked={current.includes(space.id)} onChange={(e) => setPicked(e.target.checked ? [...current, space.id] : current.filter((id) => id !== space.id))} />
            {space.name}</label></li>)}
        </ul>
        {hidden.length > 0 && <p className="text-muted-foreground">{t("automation.hiddenTargets", { count: hidden.length })}</p>}
      </fieldset>}
      <FormError message={error} />
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onClose}>{t("automation.cancel")}</Button>
        <Button type="button" disabled={busy || spaces.isPending} onClick={() => void save()}>{t("automation.saveTargets")}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>;
}

export type UsageCounter = { used: number; limit: number; state: "ok" | "warning" | "exhausted" };

/** One cap's rolling-hour usage: used of limit, with the state (warning from 80 percent, exhausted at 100). */
export function UsageLine({ label, counter }: { label: string; counter: UsageCounter }) {
  const { t } = useTranslation();
  return <p className="text-sm" data-state={counter.state}>
    <span>{label}: </span><span className="font-medium">{counter.used} / {counter.limit}</span>
    <span className="ml-2 text-muted-foreground">{t(`automation.usage.${counter.state}`)}</span>
  </p>;
}

/** The three execution caps (D3). Admin only; the server records each change. */
export function WorkspaceLimitsCard() {
  const { t } = useTranslation();
  const client = useQueryClient();
  const limits = useQuery({ queryKey: ["automation", "limits"], retry: false, queryFn: () => api<{ rule_per_hour: number; workspace_per_hour: number; space_per_hour: number }>("/api/v1/automation/limits") });
  const [draft, setDraft] = useState<Record<string, string> | undefined>(undefined);
  const [error, setError] = useState("");
  const values = draft ?? (limits.data ? { rule_per_hour: String(limits.data.rule_per_hour), workspace_per_hour: String(limits.data.workspace_per_hour), space_per_hour: String(limits.data.space_per_hour) } : undefined);

  async function save() {
    if (!values) return;
    setError("");
    try {
      await api("/api/v1/automation/limits", { method: "PUT", body: JSON.stringify({ rule_per_hour: Number(values.rule_per_hour), workspace_per_hour: Number(values.workspace_per_hour), space_per_hour: Number(values.space_per_hour) }) });
      setDraft(undefined);
      await client.invalidateQueries({ queryKey: ["automation", "limits"] });
      toast.success(t("automation.limits.saved"));
    } catch (err) { setError(err instanceof ApiError ? err.message : (err as Error).message); }
  }

  const usage = useQuery({ queryKey: ["automation", "usage", "workspace"], retry: false, queryFn: () => api<{ workspace: UsageCounter | null; window: string }>("/api/v1/automation/usage") });
  return <section aria-labelledby="limits-title" className="space-y-3 rounded-lg border p-4">
    <h2 id="limits-title" className="font-medium">{t("automation.limits.title")}</h2>
    <p className="text-sm text-muted-foreground">{t("automation.limits.hint")}</p>
    {usage.data?.workspace && <UsageLine label={t("automation.usage.workspaceLabel")} counter={usage.data.workspace} />}
    {limits.isPending && <p role="status" className="text-sm text-muted-foreground">{t("automation.loading")}</p>}
    {values && <form className="grid gap-3 sm:grid-cols-3" onSubmit={(e) => { e.preventDefault(); void save(); }}>
      <label className="space-y-1 text-sm"><span>{t("automation.limits.rule")}</span>
        <Input type="number" min={1} value={values.rule_per_hour} onChange={(e) => setDraft({ ...values, rule_per_hour: e.target.value })} /></label>
      <label className="space-y-1 text-sm"><span>{t("automation.limits.workspace")}</span>
        <Input type="number" min={1} value={values.workspace_per_hour} onChange={(e) => setDraft({ ...values, workspace_per_hour: e.target.value })} /></label>
      <label className="space-y-1 text-sm"><span>{t("automation.limits.space")}</span>
        <Input type="number" min={1} value={values.space_per_hour} onChange={(e) => setDraft({ ...values, space_per_hour: e.target.value })} /></label>
      <div className="sm:col-span-3"><Button type="submit" disabled={draft === undefined}>{t("automation.limits.save")}</Button></div>
    </form>}
    <FormError message={error} />
  </section>;
}
