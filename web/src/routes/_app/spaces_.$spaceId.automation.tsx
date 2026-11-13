import { useState } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import { cleanDefinition, emptyDefinition, ruleKeys, type AutomationRule, type AutomationRun, type Definition } from "@/lib/automation";
import { formatDateTime, useProfile } from "@/lib/profile";
import { useSpace } from "@/lib/space-queries";
import { can } from "@/lib/spaces";
import { AutomationEditor } from "@/components/automation-editor";
import { FormError } from "@/components/auth/form-error";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/spaces_/$spaceId/automation")({
  validateSearch: (search: Record<string, unknown>): { container?: string } => ({ container: typeof search.container === "string" && /^[0-9a-f-]{36}$/i.test(search.container) ? search.container : undefined }),
  component: AutomationPage,
});

const statuses = ["done", "denied", "skipped", "loop_blocked", "quota_blocked", "failed"] as const;

function AutomationPage() {
  const { t } = useTranslation();
  const { spaceId } = Route.useParams();
  const { container } = Route.useSearch();
  const client = useQueryClient();
  const space = useSpace(spaceId);
  const allowed = !space.data || can(space.data, "edit");
  const rules = useQuery({ queryKey: ruleKeys.list(spaceId), enabled: allowed, retry: false,
    queryFn: async () => (await api<{ items: AutomationRule[] }>(`/api/v1/spaces/${spaceId}/automation-rules`)).items });
  const [editing, setEditing] = useState<AutomationRule | "new">();
  const [logging, setLogging] = useState<AutomationRule>();
  const [deleting, setDeleting] = useState<AutomationRule>();
  const [error, setError] = useState("");
  const forbidden = !allowed || (rules.error instanceof ApiError && rules.error.status === 403);
  const refresh = () => client.invalidateQueries({ queryKey: ruleKeys.list(spaceId) });
  const visible = (rules.data ?? []).filter((rule) => !container || rule.container_id === container);

  async function toggle(rule: AutomationRule, enabled: boolean) {
    setError("");
    try { await api(`/api/v1/automation-rules/${rule.id}`, { method: "PATCH", body: JSON.stringify({ enabled }) }); await refresh(); }
    catch (err) { setError((err as Error).message); }
  }
  async function remove() {
    if (!deleting) return;
    try { await api(`/api/v1/automation-rules/${deleting.id}`, { method: "DELETE" }); setDeleting(undefined); toast.success(t("automation.deleted")); await refresh(); }
    catch (err) { setError((err as Error).message); }
  }

  return <main className="space-y-5 p-8">
    <header className="flex flex-wrap items-start justify-between gap-4 border-b pb-5">
      <div>
        <Link to="/spaces/$spaceId" params={{ spaceId }} className="text-sm text-muted-foreground underline">{t("automation.back", { name: space.data?.name ?? "" })}</Link>
        <h1 className="mt-1 workspace-title">{t("automation.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("automation.description")}</p>
      </div>
      {!forbidden && <Button onClick={() => setEditing("new")}>{t("automation.new")}</Button>}
    </header>
    {forbidden && <p role="alert">{t("automation.forbidden")}</p>}
    {!forbidden && rules.isPending && <p role="status" className="text-sm text-muted-foreground">{t("automation.loading")}</p>}
    {rules.isError && !forbidden && <p role="alert">{t("automation.error")}</p>}
    <FormError message={error} />
    {rules.data && visible.length === 0 && <section className="rounded-xl border border-dashed bg-card p-12 text-center">
      <h2 className="text-lg font-semibold">{t("automation.empty.title")}</h2>
      <p className="mx-auto mt-1 max-w-md text-sm text-muted-foreground">{t("automation.empty.description")}</p></section>}
    {visible.length > 0 && <table className="w-full text-left text-sm">
      <thead><tr className="border-b">
        <th scope="col" className="py-2 pr-4">{t("automation.name")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.state")}</th>
        <th scope="col" className="py-2 pr-4">{t("automation.enabled")}</th>
        <th scope="col" className="py-2"><span className="sr-only">{t("automation.actionsCol")}</span></th></tr></thead>
      <tbody>{visible.map((rule) => <tr key={rule.id} className="border-b">
        <td className="py-2 pr-4 font-medium">{rule.name}</td>
        <td className="py-2 pr-4">{rule.current_version_id ? t("automation.published") : t("automation.draftOnly")}</td>
        <td className="py-2 pr-4"><label className="flex items-center gap-2"><input type="checkbox" checked={rule.enabled} onChange={(e) => void toggle(rule, e.target.checked)} />
          <span className="sr-only">{t("automation.enable", { name: rule.name })}</span>{rule.enabled ? t("automation.on") : t("automation.off")}</label></td>
        <td className="py-2"><div className="flex flex-wrap justify-end gap-2">
          <Button variant="outline" size="sm" aria-label={t("automation.editRule", { name: rule.name })} onClick={() => setEditing(rule)}>{t("automation.edit")}</Button>
          <Button variant="outline" size="sm" aria-label={t("automation.logOf", { name: rule.name })} onClick={() => setLogging(rule)}>{t("automation.log")}</Button>
          <Button variant="ghost" size="sm" aria-label={t("automation.deleteRule", { name: rule.name })} onClick={() => setDeleting(rule)}>{t("automation.delete")}</Button></div></td></tr>)}</tbody>
    </table>}
    {editing && <RuleDialog spaceId={spaceId} container={container} rule={editing === "new" ? undefined : editing} onClose={() => { setEditing(undefined); void refresh(); }} />}
    {logging && <LogDialog rule={logging} onClose={() => setLogging(undefined)} />}
    <ConfirmDialog open={!!deleting} onOpenChange={(open) => !open && setDeleting(undefined)} title={t("automation.deleteTitle")}
      description={t("automation.deleteDescription", { name: deleting?.name ?? "" })} confirmLabel={t("automation.delete")} cancelLabel={t("automation.cancel")} onConfirm={() => void remove()} />
  </main>;
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
  const [name, setName] = useState(rule?.name ?? "");
  const [definition, setDefinition] = useState<Definition>({ ...initial, if: initial.if ?? [] });
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
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
  return <Dialog open onOpenChange={(open) => !open && onClose()}>
    <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
      <DialogHeader><DialogTitle>{rule ? t("automation.editTitle") : t("automation.newTitle")}</DialogTitle>
        <DialogDescription>{t("automation.dialogHint")}</DialogDescription></DialogHeader>
      <label className="block space-y-1 text-sm"><span>{t("automation.ruleName")}</span><Input value={name} maxLength={120} onChange={(e) => setName(e.target.value)} required /></label>
      <AutomationEditor spaceId={spaceId} value={definition} onChange={setDefinition} error={error} />
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onClose}>{t("automation.cancel")}</Button>
        <Button type="button" variant="outline" disabled={busy || !name.trim()} onClick={() => void save(false)}>{t("automation.saveDraft")}</Button>
        <Button type="button" disabled={busy || !name.trim()} onClick={() => void save(true)}>{t("automation.publish")}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>;
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
          <td className="py-1 pr-3">{t(run.action_index >= 0 ? "automation.logEventAction" : "automation.logEventRule", { type: run.event_type, version: run.version, action: run.action_index + 1 })}</td>
          <td className="py-1 pr-3">{run.item_title ?? "—"}</td>
          <td className="py-1"><span className="font-medium">{t(`automation.runStatus.${run.status}`)}</span>{run.detail ? ` — ${run.detail}` : ""}</td></tr>)}</tbody></table>}
      {log.hasNextPage && <Button variant="outline" disabled={log.isFetchingNextPage} onClick={() => void log.fetchNextPage()}>{t("automation.logMore")}</Button>}
    </DialogContent>
  </Dialog>;
}
