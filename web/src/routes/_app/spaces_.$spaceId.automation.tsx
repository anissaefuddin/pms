import { useRef, useState } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import { cleanDefinition, emptyDefinition, ruleKeys, type AutomationRule, type AutomationRun, type Definition } from "@/lib/automation";
import { formatDateTime, useProfile } from "@/lib/profile";
import { useSpace } from "@/lib/space-queries";
import { can } from "@/lib/spaces";
import { AutomationEditor, type EditorStep } from "@/components/automation-editor";
import { FormError } from "@/components/auth/form-error";
import { UsageLine } from "@/components/workspace-automation";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { ItemUserPicker, type PersonOption } from "@/components/item-user-picker";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

type Template = { id: string; name: string; description: string; definition: Definition };
const stepIds = ["apply", "when", "if", "then", "test"] as const;
type StepId = (typeof stepIds)[number];

export const Route = createFileRoute("/_app/spaces_/$spaceId/automation")({
  validateSearch: (search: Record<string, unknown>): { container?: string } => ({ container: typeof search.container === "string" && /^[0-9a-f-]{36}$/i.test(search.container) ? search.container : undefined }),
  component: AutomationPage,
});

const statuses = ["done", "denied", "skipped", "loop_blocked", "quota_blocked", "failed", "stopped"] as const;
const stateFilters = ["all", "enabled", "disabled", "attention", "archived"] as const;
type StateFilter = (typeof stateFilters)[number];
type DeadEvent = { event_id: string; type: string; item_id?: string | null; attempts: number; last_error: string; created_at: string };

/** A rule's state in one word: archived wins, then a paused or invalid rule, then on or off. */
function stateOf(rule: AutomationRule): Exclude<StateFilter, "all"> {
  if (rule.archived_at) return "archived";
  if (rule.health !== "ok") return "attention";
  return rule.enabled ? "enabled" : "disabled";
}

function AutomationPage() {
  const { t } = useTranslation();
  const { spaceId } = Route.useParams();
  const { container } = Route.useSearch();
  const client = useQueryClient();
  const profile = useProfile();
  const isAdmin = profile?.user_type === "admin";
  const space = useSpace(spaceId);
  const allowed = !space.data || can(space.data, "edit");
  const rules = useQuery({ queryKey: ruleKeys.list(spaceId), enabled: allowed, retry: false,
    queryFn: async () => (await api<{ items: AutomationRule[] }>(`/api/v1/spaces/${spaceId}/automation-rules`)).items });
  const [editing, setEditing] = useState<AutomationRule | "new">();
  const [logging, setLogging] = useState<AutomationRule>();
  const [archiving, setArchiving] = useState<AutomationRule>();
  const [transferring, setTransferring] = useState<AutomationRule>();
  const [filter, setFilter] = useState<StateFilter>("all");
  const [error, setError] = useState("");
  const fileInput = useRef<HTMLInputElement>(null);
  const forbidden = !allowed || (rules.error instanceof ApiError && rules.error.status === 403);
  const refresh = () => client.invalidateQueries({ queryKey: ruleKeys.list(spaceId) });
  // Usage of this Space in the rolling hour: the same counter that enforces its cap (AUT-16).
  const spaceUsage = useQuery({ queryKey: ["automation", "usage", "space", spaceId], enabled: allowed, retry: false,
    queryFn: () => api<{ space: { used: number; limit: number; state: "ok" | "warning" | "exhausted" } | null }>(`/api/v1/automation/usage?space_id=${spaceId}`) });
  // Workspace rules that run in this Space: name and state only (decision D4).
  const workspaceHere = useQuery({ queryKey: ["automation", "space-workspace", spaceId], enabled: allowed, retry: false,
    queryFn: async () => (await api<{ items: { id: string; name: string; enabled: boolean; health: string }[] }>(`/api/v1/spaces/${spaceId}/workspace-automation`)).items });
  const visible = (rules.data ?? []).filter((rule) => (!container || rule.container_id === container) && (filter === "all" || stateOf(rule) === filter));

  async function act(path: string, method: "POST" | "PATCH", body?: unknown, done?: string) {
    setError("");
    try {
      await api(path, { method, body: body === undefined ? undefined : JSON.stringify(body) });
      if (done) toast.success(t(done));
      await refresh();
    } catch (err) { setError(err instanceof ApiError ? err.message : (err as Error).message); }
  }
  const toggle = (rule: AutomationRule, enabled: boolean) => act(`/api/v1/automation-rules/${rule.id}`, "PATCH", { enabled });
  const duplicate = (rule: AutomationRule) => act(`/api/v1/automation-rules/${rule.id}/duplicate`, "POST", undefined, "automation.duplicated");
  const restore = (rule: AutomationRule) => act(`/api/v1/automation-rules/${rule.id}/restore`, "POST", undefined, "automation.restored");
  async function exportRule(rule: AutomationRule) {
    setError("");
    try {
      const data = await api<unknown>(`/api/v1/automation-rules/${rule.id}/export`);
      const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2)], { type: "application/json" }));
      const link = document.createElement("a");
      link.href = url;
      link.download = `${rule.name}.json`;
      link.click();
      URL.revokeObjectURL(url);
    } catch (err) { setError(err instanceof ApiError ? err.message : (err as Error).message); }
  }
  async function importFile(file: File | undefined) {
    if (!file) return;
    setError("");
    try {
      const body = JSON.parse(await file.text()) as unknown;
      await api(`/api/v1/spaces/${spaceId}/automation-rules/import`, { method: "POST", body: JSON.stringify(body) });
      toast.success(t("automation.imported"));
      await refresh();
    } catch (err) { setError(err instanceof ApiError ? err.message : t("automation.importInvalid")); }
    finally { if (fileInput.current) fileInput.current.value = ""; }
  }
  async function archive() {
    if (!archiving) return;
    await act(`/api/v1/automation-rules/${archiving.id}/archive`, "POST", undefined, "automation.deleted");
    setArchiving(undefined);
  }

  return <main className="space-y-5 p-8">
    <header className="flex flex-wrap items-start justify-between gap-4 border-b pb-5">
      <div>
        <Link to="/spaces/$spaceId" params={{ spaceId }} className="text-sm text-muted-foreground underline">{t("automation.back", { name: space.data?.name ?? "" })}</Link>
        <h1 className="mt-1 workspace-title">{t("automation.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("automation.description")}</p>
      </div>
      {!forbidden && <div className="flex flex-wrap gap-2">
        <Button variant="outline" onClick={() => fileInput.current?.click()}>{t("automation.import")}</Button>
        <input ref={fileInput} type="file" accept="application/json,.json" className="hidden" aria-label={t("automation.importFile")} onChange={(e) => void importFile(e.target.files?.[0])} />
        <Button onClick={() => setEditing("new")}>{t("automation.new")}</Button></div>}
    </header>
    {forbidden && <p role="alert">{t("automation.forbidden")}</p>}
    {!forbidden && rules.isPending && <p role="status" className="text-sm text-muted-foreground">{t("automation.loading")}</p>}
    {rules.isError && !forbidden && <p role="alert">{t("automation.error")}</p>}
    <FormError message={error} />
    {rules.data && (rules.data.length > 0) && <label className="flex items-center gap-2 text-sm">{t("automation.filterState")}
      <select className="rounded border bg-background px-2 py-1" value={filter} onChange={(e) => setFilter(e.target.value as StateFilter)}>
        {stateFilters.map((f) => <option key={f} value={f}>{t(`automation.filter.${f}`)}</option>)}</select></label>}
    {rules.data && visible.length === 0 && <section className="rounded-xl border border-dashed bg-card p-12 text-center">
      <h2 className="text-lg font-semibold">{t("automation.empty.title")}</h2>
      <p className="mx-auto mt-1 max-w-md text-sm text-muted-foreground">{t("automation.empty.description")}</p></section>}
    {visible.length > 0 && <div className="overflow-x-auto"><table className="w-full text-left text-sm">
      <thead><tr className="border-b">
        <th scope="col" className="py-2 pr-4">{t("automation.name")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.appliesTo")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.trigger")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.stateCol")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.owner")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.lastRun")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.errors")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.actionsCol")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.enabled")}</th>
        <th scope="col" className="py-2"><span className="sr-only">{t("automation.rowActions")}</span></th></tr></thead>
      <tbody>{visible.map((rule) => <tr key={rule.id} className="border-b align-top">
        <td className="py-2 pr-4 font-medium">{rule.name}</td>
        <td className="py-2 pr-4">{rule.container_title ?? t("automation.wholeSpace")}</td>
        <td className="py-2 pr-4">{rule.trigger ? t(`automation.triggers.${rule.trigger}`, { defaultValue: rule.trigger }) : "—"}</td>
        <td className="py-2 pr-4">
          {rule.archived_at ? t("automation.archived") : rule.health !== "ok" ? t(`automation.health.${rule.health}`, { reason: t(`automation.reasons.${rule.health_reason}`, { defaultValue: rule.health_reason }) })
            : rule.current_version_id ? t("automation.published") : t("automation.draftOnly")}
          {rule.has_draft && rule.current_version_id && <span className="block text-xs text-muted-foreground">{t("automation.draftChanges")}</span>}
        </td>
        <td className="py-2 pr-4">{rule.owner_name}</td>
        <td className="py-2 pr-4 whitespace-nowrap">{rule.last_run_at ? <>{formatDateTime(rule.last_run_at, profile)} <span className="text-muted-foreground">{t(`automation.runStatus.${rule.last_run_status}`, { defaultValue: rule.last_run_status ?? "" })}</span></> : t("automation.neverRun")}</td>
        <td className="py-2 pr-4">{rule.error_count}</td>
        <td className="py-2 pr-4">{rule.action_count}</td>
        <td className="py-2 pr-4"><label className="flex items-center gap-2"><input type="checkbox" checked={rule.enabled} disabled={!!rule.archived_at} onChange={(e) => void toggle(rule, e.target.checked)} />
          <span className="sr-only">{t("automation.enable", { name: rule.name })}</span>{rule.enabled ? t("automation.on") : t("automation.off")}</label></td>
        <td className="py-2"><div className="flex flex-wrap justify-end gap-2">
          {!rule.archived_at && <Button variant="outline" size="sm" aria-label={t("automation.editRule", { name: rule.name })} onClick={() => setEditing(rule)}>{t("automation.edit")}</Button>}
          <Button variant="outline" size="sm" aria-label={t("automation.logOf", { name: rule.name })} onClick={() => setLogging(rule)}>{t("automation.log")}</Button>
          <Button variant="outline" size="sm" aria-label={t("automation.duplicateRule", { name: rule.name })} onClick={() => void duplicate(rule)}>{t("automation.duplicate")}</Button>
          <Button variant="outline" size="sm" aria-label={t("automation.exportRule", { name: rule.name })} onClick={() => void exportRule(rule)}>{t("automation.export")}</Button>
          {isAdmin && !rule.archived_at && <Button variant="outline" size="sm" aria-label={t("automation.transferRule", { name: rule.name })} onClick={() => setTransferring(rule)}>{t("automation.transferOwner")}</Button>}
          {rule.archived_at
            ? <Button variant="ghost" size="sm" aria-label={t("automation.restoreRule", { name: rule.name })} onClick={() => void restore(rule)}>{t("automation.restore")}</Button>
            : <Button variant="ghost" size="sm" aria-label={t("automation.archiveRule", { name: rule.name })} onClick={() => setArchiving(rule)}>{t("automation.archive")}</Button>}
        </div></td></tr>)}</tbody>
    </table></div>}
    {spaceUsage.data?.space && <UsageLine label={t("automation.usage.spaceLabel")} counter={spaceUsage.data.space} />}
    {workspaceHere.data && workspaceHere.data.length > 0 && <section className="space-y-2 border-t pt-5" aria-labelledby="workspace-here-title">
      <h2 id="workspace-here-title" className="font-medium">{t("automation.workspaceHere")}</h2>
      <p className="text-sm text-muted-foreground">{t("automation.workspaceHereHint")}</p>
      <ul className="divide-y rounded border text-sm">{workspaceHere.data.map((rule) => <li key={rule.id} className="flex flex-wrap justify-between gap-2 px-3 py-2">
        <span className="font-medium">{rule.name}</span>
        <span>{rule.enabled ? t("automation.on") : t("automation.off")}{rule.health !== "ok" ? ` · ${t(`automation.health.${rule.health}`, { reason: "" })}` : ""}</span></li>)}</ul>
    </section>}
    {isAdmin && <DeadLetters />}
    {editing && <RuleDialog spaceId={spaceId} container={container} rule={editing === "new" ? undefined : editing} onClose={() => { setEditing(undefined); void refresh(); }} />}
    {logging && <LogDialog rule={logging} onClose={() => setLogging(undefined)} />}
    {transferring && <TransferDialog rule={transferring} onClose={() => { setTransferring(undefined); void refresh(); }} />}
    <ConfirmDialog open={!!archiving} onOpenChange={(open) => !open && setArchiving(undefined)} title={t("automation.archiveTitle")}
      description={t("automation.archiveDescription", { name: archiving?.name ?? "" })} confirmLabel={t("automation.archive")} cancelLabel={t("automation.cancel")} onConfirm={() => void archive()} />
  </main>;
}

/** Events the dispatcher gave up on (A2). Administrators see them and replay one once its cause is fixed. */
function DeadLetters() {
  const { t } = useTranslation();
  const client = useQueryClient();
  const profile = useProfile();
  const [error, setError] = useState("");
  const dead = useQuery({ queryKey: ["automation", "dead-events"], retry: false,
    queryFn: async () => (await api<{ items: DeadEvent[] }>("/api/v1/automation/dead-events")).items });
  async function replay(eventId: string) {
    setError("");
    try {
      await api(`/api/v1/automation/events/${eventId}/replay`, { method: "POST" });
      toast.success(t("automation.replayed"));
      await client.invalidateQueries({ queryKey: ["automation", "dead-events"] });
    } catch (err) { setError(err instanceof ApiError ? err.message : (err as Error).message); }
  }
  return <section className="space-y-3 border-t pt-5" aria-labelledby="dead-letters-title">
    <h2 id="dead-letters-title" className="font-medium">{t("automation.deadLetters")}</h2>
    <p className="text-sm text-muted-foreground">{t("automation.deadLettersHint")}</p>
    <FormError message={error} />
    {dead.data?.length === 0 && <p className="text-sm text-muted-foreground">{t("automation.noDeadLetters")}</p>}
    {(dead.data?.length ?? 0) > 0 && <div className="overflow-x-auto"><table className="w-full text-left text-sm">
      <thead><tr className="border-b">
        <th scope="col" className="py-1 pr-3">{t("automation.logWhen")}</th>
        <th scope="col" className="py-1 pr-3">{t("automation.logEvent")}</th>
        <th scope="col" className="py-1 pr-3">{t("automation.attempts")}</th>
        <th scope="col" className="py-1 pr-3">{t("automation.lastError")}</th>
        <th scope="col" className="py-1"><span className="sr-only">{t("automation.rowActions")}</span></th></tr></thead>
      <tbody>{dead.data!.map((ev) => <tr key={ev.event_id} className="border-b align-top">
        <td className="py-1 pr-3 whitespace-nowrap">{formatDateTime(ev.created_at, profile)}</td>
        <td className="py-1 pr-3">{ev.type}</td>
        <td className="py-1 pr-3">{ev.attempts}</td>
        <td className="py-1 pr-3">{ev.last_error}</td>
        <td className="py-1 text-right"><Button variant="outline" size="sm" aria-label={t("automation.replayEvent", { id: ev.event_id.slice(0, 8) })} onClick={() => void replay(ev.event_id)}>{t("automation.replay")}</Button></td></tr>)}</tbody>
    </table></div>}
  </section>;
}

/** Hand a rule to another owner (A1). Only a workspace admin can; the server checks the new owner can run it here. */
function TransferDialog({ rule, onClose }: { rule: AutomationRule; onClose: () => void }) {
  const { t } = useTranslation();
  const [owner, setOwner] = useState<PersonOption[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function save() {
    setBusy(true); setError("");
    try {
      await api(`/api/v1/automation-rules/${rule.id}/owner`, { method: "POST", body: JSON.stringify({ owner_id: owner[0]!.id }) });
      toast.success(t("automation.transferred"));
      onClose();
    } catch (err) { setError(err instanceof ApiError ? err.message : (err as Error).message); } finally { setBusy(false); }
  }
  return <Dialog open onOpenChange={(open) => !open && onClose()}>
    <DialogContent className="sm:max-w-md">
      <DialogHeader><DialogTitle>{t("automation.transferTitle", { name: rule.name })}</DialogTitle>
        <DialogDescription>{t("automation.transferHint")}</DialogDescription></DialogHeader>
      <ItemUserPicker label={t("automation.newOwner")} selected={owner} maxSelected={1} onChange={setOwner} />
      <FormError message={error} />
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onClose}>{t("automation.cancel")}</Button>
        <Button type="button" disabled={busy || owner.length === 0} onClick={() => void save()}>{t("automation.transfer")}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>;
}

function RuleDialog({ spaceId, container, rule, onClose }: { spaceId: string; container?: string; rule?: AutomationRule; onClose: () => void }) {
  const { t } = useTranslation();
  const loaded = useQuery({ queryKey: ruleKeys.one(rule?.id ?? "new"), enabled: !!rule, retry: false, queryFn: () => api<AutomationRule>(`/api/v1/automation-rules/${rule!.id}`) });
  const latest = loaded.data?.versions?.[0];
  if (rule && !latest) return <Dialog open onOpenChange={(open) => !open && onClose()}><DialogContent><DialogTitle>{t("automation.editTitle")}</DialogTitle>
    {loaded.isError ? <FormError message={t("automation.error")} /> : <p role="status">{t("automation.loading")}</p>}</DialogContent></Dialog>;
  return <RuleForm spaceId={spaceId} container={container} rule={loaded.data} initial={latest ? (latest.definition as unknown as Definition) : emptyDefinition()} onClose={onClose} />;
}

function RuleForm({ spaceId, container, rule, initial, onClose }: { spaceId: string; container?: string; rule?: AutomationRule; initial: Definition; onClose: () => void }) {
  const { t } = useTranslation();
  const start = { ...initial, if: initial.if ?? [] };
  const [name, setName] = useState(rule?.name ?? "");
  const [definition, setDefinition] = useState<Definition>(start);
  const [baseline] = useState(() => JSON.stringify({ name: rule?.name ?? "", definition: start }));
  const [confirmDiscard, setConfirmDiscard] = useState(false);
  const [step, setStep] = useState<StepId>("apply");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const templates = useQuery({ queryKey: ["automation-templates"], enabled: !rule, retry: false,
    queryFn: async () => (await api<{ items: Template[] }>("/api/v1/automation/templates")).items });
  const dirty = JSON.stringify({ name, definition }) !== baseline;
  const requestClose = () => (dirty ? setConfirmDiscard(true) : onClose());
  const jump = (id: string) => document.getElementById(id)?.scrollIntoView({ block: "start", behavior: "smooth" });
  function useTemplate(id: string) {
    const tpl = templates.data?.find((x) => x.id === id);
    if (!tpl) return;
    setDefinition({ ...tpl.definition, if: tpl.definition.if ?? [] });
    if (!name.trim()) setName(tpl.name);
  }
  async function save(publish: boolean) {
    setBusy(true); setError("");
    try {
      const body = cleanDefinition(definition);
      let id = rule?.id;
      if (!id) id = (await api<AutomationRule>(`/api/v1/spaces/${spaceId}/automation-rules`, { method: "POST", body: JSON.stringify({ name: name.trim(), container_id: container ?? null, definition: body }) })).id;
      else {
        if (name.trim() && name.trim() !== rule?.name) await api(`/api/v1/automation-rules/${id}`, { method: "PATCH", body: JSON.stringify({ name: name.trim() }) });
        await api(`/api/v1/automation-rules/${id}/draft`, { method: "PUT", body: JSON.stringify({ definition: body }) });
      }
      if (publish) await api(`/api/v1/automation-rules/${id}/publish`, { method: "POST" });
      toast.success(t(publish ? "automation.published" : "automation.draftSaved"));
      onClose();
    } catch (err) { setError(err instanceof ApiError ? err.message : (err as Error).message); } finally { setBusy(false); }
  }
  return <>
  <Dialog open onOpenChange={(open) => !open && requestClose()}>
    <DialogContent className="h-dvh max-h-dvh w-screen max-w-none overflow-y-auto rounded-none sm:max-w-none">
      <DialogHeader><DialogTitle>{rule ? t("automation.editTitle") : t("automation.newTitle")}</DialogTitle>
        <DialogDescription>{t("automation.dialogHint")}</DialogDescription></DialogHeader>
      <div className="grid gap-6 md:grid-cols-[11rem_minmax(0,1fr)]">
        <nav aria-label={t("automation.steps")} className="flex flex-row flex-wrap gap-1 text-sm md:flex-col">
          {stepIds.map((id) => <Button key={id} type="button" variant={step === id ? "default" : "ghost"} size="sm" className="justify-start"
            aria-current={step === id ? "step" : undefined} onClick={() => setStep(id)}>{t(`automation.step-${id}-label`)}</Button>)}
        </nav>
        <div className="min-w-0 space-y-5">
          {step === "apply" && <section className="space-y-4 text-sm">
            <h3 className="font-medium">{t("automation.applyTo")}</h3>
            <p className="text-muted-foreground">{container ? t("automation.appliesToContainer") : t("automation.appliesToSpace")}</p>
            {!rule && templates.data && templates.data.length > 0 && <label className="block space-y-1">
              <span>{t("automation.startFromTemplate")}</span>
              <select className="block w-full rounded-lg border border-input bg-background px-2 py-1.5 text-sm" value="" onChange={(e) => useTemplate(e.target.value)}>
                <option value="">{t("automation.noTemplate")}</option>{templates.data.map((tpl) => <option key={tpl.id} value={tpl.id}>{tpl.name}</option>)}</select></label>}
            <label className="block space-y-1"><span>{t("automation.ruleName")}</span><Input value={name} maxLength={120} onChange={(e) => setName(e.target.value)} required /></label>
          </section>}
          {step !== "apply" && <AutomationEditor spaceId={spaceId} ruleId={rule?.id} value={definition} onChange={setDefinition} error={error} step={step as EditorStep} />}
        </div>
      </div>
      <DialogFooter>
        <Button type="button" variant="outline" onClick={requestClose}>{t("automation.cancel")}</Button>
        <Button type="button" variant="outline" disabled={busy || !name.trim()} onClick={() => void save(false)}>{t("automation.saveDraft")}</Button>
        <Button type="button" disabled={busy || !name.trim()} onClick={() => void save(true)}>{t("automation.publish")}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
  <ConfirmDialog open={confirmDiscard} onOpenChange={setConfirmDiscard} title={t("automation.discardTitle")}
    description={t("automation.discardDescription")} confirmLabel={t("automation.discard")} cancelLabel={t("automation.keepEditing")} onConfirm={onClose} />
  </>;
}

function LogDialog({ rule, onClose }: { rule: AutomationRule; onClose: () => void }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const [status, setStatus] = useState("");
  const log = useInfiniteQuery({ queryKey: ["automation-runs", rule.id, status], initialPageParam: "", retry: false,
    queryFn: ({ pageParam }) => api<{ items: AutomationRun[]; next_cursor?: string | null }>(`/api/v1/automation-rules/${rule.id}/runs?limit=20${status ? `&status=${status}` : ""}${pageParam ? `&cursor=${pageParam}` : ""}`),
    getNextPageParam: (page) => page.next_cursor ?? undefined });
  const runs = log.data?.pages.flatMap((page) => page.items) ?? [];
  return <Dialog open onOpenChange={(open) => !open && onClose()}>
    <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-3xl">
      <DialogHeader><DialogTitle>{t("automation.logTitle", { name: rule.name })}</DialogTitle><DialogDescription>{t("automation.logHint")}</DialogDescription></DialogHeader>
      <label className="flex items-center gap-2 text-sm">{t("automation.logFilter")}
        <select className="rounded border bg-background px-2 py-1" value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">{t("automation.logAll")}</option>{statuses.map((s) => <option key={s} value={s}>{t(`automation.runStatus.${s}`)}</option>)}</select></label>
      {log.isError && <FormError message={t("automation.error")} />}
      {log.isSuccess && runs.length === 0 && <p className="text-sm text-muted-foreground">{t("automation.logEmpty")}</p>}
      {runs.length > 0 && <table className="w-full text-left text-sm">
        <thead><tr className="border-b"><th scope="col" className="py-1 pr-3">{t("automation.logWhen")}</th><th scope="col" className="py-1 pr-3">{t("automation.logEvent")}</th>
          <th scope="col" className="py-1 pr-3">{t("automation.logItem")}</th><th scope="col" className="py-1">{t("automation.logResult")}</th></tr></thead>
        <tbody>{runs.map((run) => <tr key={run.id} className="border-b align-top">
          <td className="py-1 pr-3 whitespace-nowrap">{formatDateTime(run.created_at, profile)}</td>
          <td className="py-1 pr-3">{t(run.action_index >= 0 ? "automation.logEventAction" : "automation.logEventRule", { type: run.event_type, version: run.version, action: run.action_index + 1 })}
            {run.action_type && <span className="block text-xs text-muted-foreground">{t(`automation.actions.${run.action_type}`, { defaultValue: run.action_type })}</span>}</td>
          <td className="py-1 pr-3">{run.item_title ?? "—"}</td>
          <td className="py-1"><span className="font-medium">{t(`automation.runStatus.${run.status}`)}</span>
            {run.reason && <span className="ml-2 rounded bg-muted px-1.5 py-0.5 text-xs">{t(`automation.reasons.${run.reason}`, { defaultValue: run.reason })}</span>}
            {run.detail ? ` — ${run.detail}` : ""}</td></tr>)}</tbody></table>}
      {log.hasNextPage && <Button variant="outline" disabled={log.isFetchingNextPage} onClick={() => void log.fetchNextPage()}>{t("automation.logMore")}</Button>}
    </DialogContent>
  </Dialog>;
}
