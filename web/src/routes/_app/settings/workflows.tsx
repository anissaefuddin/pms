import { useState, type FormEvent } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { moveId, statusColorTokens, statusDotClass, statusGroups, type Workflow, type WorkflowStatus } from "@/lib/status-colors";
import { FormError } from "@/components/auth/form-error";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { StatusGuardEditor } from "@/components/status-guard-editor";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/settings/workflows")({ component: WorkflowsRoute });

const base = "/api/v1";
const selectClass = "rounded border bg-background px-2 py-1 text-sm";
const send = <T,>(path: string, method: string, body?: unknown) =>
  api<T>(base + path, { method, body: body === undefined ? undefined : JSON.stringify(body) });

function WorkflowsRoute() {
  const { t } = useTranslation();
  const client = useQueryClient();
  const admin = useProfile()?.user_type === "admin";
  const [picked, setPicked] = useState("");
  const [name, setName] = useState("");
  const [rename, setRename] = useState<string>();
  const [newStatus, setNewStatus] = useState("");
  const [adding, setAdding] = useState<(typeof statusGroups)[number]>(); // column whose inline "add status" field is open
  const [selected, setSelected] = useState(""); // status shown in the editor under the board
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [removing, setRemoving] = useState<WorkflowStatus>(); // status awaiting a delete confirmation
  const [moveFrom, setMoveFrom] = useState<WorkflowStatus>(); // status in use: ask where its items go
  const [moveTo, setMoveTo] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const query = useQuery({ queryKey: ["workflows"], queryFn: () => api<{ items: Workflow[] }>(`${base}/workflows`) });
  const workflows = query.data?.items;
  if (!workflows) return null;
  const workflow = workflows.find((wf) => wf.id === picked) ?? workflows.find((wf) => wf.is_default) ?? workflows[0];

  const store = (updated: Workflow) => client.setQueryData<{ items: Workflow[] }>(["workflows"],
    (data) => ({ items: (data?.items ?? []).map((wf) => wf.id === updated.id ? updated : wf) }));
  // One guard for every call: busy flag, inline server `detail` on failure.
  async function run<T>(action: () => Promise<T>): Promise<T | undefined> {
    if (busy) return;
    setBusy(true); setError("");
    try { return await action(); }
    catch (err) { setError(err instanceof ApiError ? err.message : (err as Error).message); }
    finally { setBusy(false); }
  }
  const saveStatus = (status: WorkflowStatus, patch: object) =>
    run(async () => store(await send<Workflow>(`/statuses/${status.id}`, "PATCH", patch)));

  async function create(event: FormEvent) {
    event.preventDefault();
    if (!name.trim()) return;
    const created = await run(() => send<Workflow>("/workflows", "POST", { name: name.trim() }));
    if (!created) return;
    await client.invalidateQueries({ queryKey: ["workflows"] });
    setPicked(created.id); setName("");
  }
  async function saveName(event: FormEvent) {
    event.preventDefault();
    if (rename === undefined || !rename.trim()) return;
    const updated = await run(() => send<Workflow>(`/workflows/${workflow.id}`, "PATCH", { name: rename.trim() }));
    if (updated) { store(updated); setRename(undefined); toast.success(t("settings.workflows.saved")); }
  }
  async function removeWorkflow() {
    const ok = await run(async () => { await send(`/workflows/${workflow.id}`, "DELETE"); return true; });
    setConfirmDelete(false);
    if (ok) { setPicked(""); await client.invalidateQueries({ queryKey: ["workflows"] }); }
  }
  async function addStatus(event: FormEvent) {
    event.preventDefault();
    if (!newStatus.trim()) return;
    const updated = await run(() => send<Workflow>(`/workflows/${workflow.id}/statuses`, "POST",
      { name: newStatus.trim(), group: adding ?? "active", color: "slate" }));
    if (updated) { store(updated); setNewStatus(""); setAdding(undefined); }
  }
  async function reorder(status: WorkflowStatus, delta: -1 | 1) {
    const ids = moveId(workflow.statuses.map((s) => s.id), status.id, delta);
    await run(async () => store(await send<Workflow>(`/workflows/${workflow.id}/status-order`, "PUT", { status_ids: ids })));
  }
  // Delete first; only a 409 status_in_use opens the "move items" dialog, then retries with move_to.
  async function removeStatus(status: WorkflowStatus, moveTarget?: string) {
    if (busy) return;
    setBusy(true); setError("");
    try {
      store(await send<Workflow>(`/statuses/${status.id}${moveTarget ? `?move_to=${moveTarget}` : ""}`, "DELETE"));
      setRemoving(undefined); setMoveFrom(undefined);
    } catch (err) {
      setRemoving(undefined);
      if (err instanceof ApiError && err.status === 409 && err.code === "status_in_use" && !moveTarget) {
        setMoveTo(workflow.statuses.find((s) => s.id !== status.id)?.id ?? "");
        setMoveFrom(status);
      } else setError((err as Error).message);
    } finally { setBusy(false); }
  }

  return <div className="max-w-3xl space-y-6">
    <header>
      <h2 className="text-lg font-semibold">{t("settings.workflows.title")}</h2>
      <p className="text-sm text-muted-foreground">{admin ? t("settings.workflows.description") : t("settings.workflows.readOnly")}</p>
    </header>
    <div className="flex flex-wrap items-end gap-3">
      <label className="space-y-1 text-sm"><span className="block">{t("settings.workflows.select")}</span>
        <select className={selectClass} value={workflow.id} onChange={(event) => { setPicked(event.target.value); setRename(undefined); setError(""); }}>
          {workflows.map((wf) => <option key={wf.id} value={wf.id}>{wf.is_default ? `${wf.name} (${t("settings.workflows.default")})` : wf.name}</option>)}
        </select></label>
      {admin && rename === undefined && <>
        <Button variant="outline" onClick={() => setRename(workflow.name)}>{t("settings.workflows.rename")}</Button>
        <Button variant="destructive" onClick={() => setConfirmDelete(true)}>{t("settings.workflows.delete")}</Button>
      </>}
      {admin && rename !== undefined && <form className="flex flex-wrap items-end gap-2" onSubmit={saveName}>
        <label className="space-y-1 text-sm"><span className="block">{t("settings.workflows.renameName")}</span>
          <Input value={rename} maxLength={120} onChange={(event) => setRename(event.target.value)} /></label>
        <Button type="submit" disabled={busy}>{t("settings.workflows.save")}</Button>
        <Button type="button" variant="outline" onClick={() => setRename(undefined)}>{t("settings.workflows.cancel")}</Button>
      </form>}
    </div>
    {admin && <form className="flex flex-wrap items-end gap-2" onSubmit={create}>
      <label className="space-y-1 text-sm"><span className="block">{t("settings.workflows.newName")}</span>
        <Input value={name} maxLength={120} onChange={(event) => setName(event.target.value)} /></label>
      <Button type="submit" variant="outline" disabled={busy || !name.trim()}>{t("settings.workflows.create")}</Button>
    </form>}
    <FormError message={error} />
    <section aria-labelledby="wf-statuses" className="space-y-3">
      <h3 id="wf-statuses" className="text-sm font-medium">{t("settings.workflows.statuses")}</h3>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {statusGroups.map((group) => <div key={group} role="group" aria-label={t(`settings.workflows.group.${group}`)} className="space-y-2">
          <h4 className="text-sm font-semibold">{t(`settings.workflows.group.${group}`)}</h4>
          {workflow.statuses.filter((s) => s.group === group).map((s) => <button key={s.id} type="button" aria-pressed={selected === s.id}
            onClick={() => setSelected(selected === s.id ? "" : s.id)}
            className="flex w-full items-center gap-2 rounded-lg border bg-card px-3 py-2 text-left text-sm hover:bg-muted aria-pressed:border-primary aria-pressed:ring-1 aria-pressed:ring-primary focus-visible:outline-2 focus-visible:outline-ring">
            <span aria-hidden className={`size-3 shrink-0 rounded-full ${statusDotClass(s.color)}`} />{s.name}</button>)}
          {admin && (adding === group
            ? <form className="space-y-1" onSubmit={addStatus}>
                <Input autoFocus aria-label={t("settings.workflows.statusName")} value={newStatus} maxLength={120} onChange={(event) => setNewStatus(event.target.value)} />
                <div className="flex gap-2">
                  <Button type="submit" size="sm" disabled={busy || !newStatus.trim()}>{t("settings.workflows.addStatusSubmit")}</Button>
                  <Button type="button" size="sm" variant="ghost" onClick={() => { setAdding(undefined); setNewStatus(""); }}>{t("settings.workflows.cancel")}</Button>
                </div>
              </form>
            : <Button type="button" variant="ghost" size="sm" className="text-primary" onClick={() => { setAdding(group); setNewStatus(""); }}>+ {t("settings.workflows.addStatus")}</Button>)}
        </div>)}
      </div>
      <ol>
        {workflow.statuses.filter((s) => s.id === selected).map((status) => {
          const index = workflow.statuses.indexOf(status);
          return <StatusRow key={`${status.id}:${status.name}`} status={status} siblings={workflow.statuses}
            admin={admin} busy={busy} first={index === 0} last={index === workflow.statuses.length - 1}
            onPatch={(patch) => saveStatus(status, patch)} onMove={(delta) => reorder(status, delta)} onRemove={() => setRemoving(status)} />;
        })}
      </ol>
    </section>

    <ConfirmDialog open={confirmDelete} onOpenChange={setConfirmDelete} busy={busy} error={error}
      title={t("settings.workflows.deleteTitle")} description={t("settings.workflows.deleteDescription", { name: workflow.name })}
      confirmLabel={t("settings.workflows.confirmDelete")} cancelLabel={t("settings.workflows.cancel")} onConfirm={() => void removeWorkflow()} />
    <ConfirmDialog open={!!removing} onOpenChange={(open) => !open && setRemoving(undefined)} busy={busy}
      title={t("settings.workflows.deleteStatusTitle")} description={t("settings.workflows.deleteStatusDescription", { name: removing?.name ?? "" })}
      confirmLabel={t("settings.workflows.confirmDelete")} cancelLabel={t("settings.workflows.cancel")}
      onConfirm={() => removing && void removeStatus(removing)} />
    <Dialog open={!!moveFrom} onOpenChange={(open) => !open && setMoveFrom(undefined)}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("settings.workflows.moveTitle")}</DialogTitle>
          <DialogDescription>{t("settings.workflows.moveDescription", { name: moveFrom?.name ?? "" })}</DialogDescription>
        </DialogHeader>
        <label className="space-y-1 text-sm"><span className="block">{t("settings.workflows.moveTo")}</span>
          <select className={`${selectClass} w-full`} value={moveTo} onChange={(event) => setMoveTo(event.target.value)}>
            {workflow.statuses.filter((s) => s.id !== moveFrom?.id).map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select></label>
        <FormError message={error} />
        <DialogFooter>
          <Button variant="outline" onClick={() => setMoveFrom(undefined)}>{t("settings.workflows.cancel")}</Button>
          <Button variant="destructive" disabled={busy || !moveTo} onClick={() => moveFrom && void removeStatus(moveFrom, moveTo)}>{t("settings.workflows.moveConfirm")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>;
}

function StatusRow({ status, siblings, admin, busy, first, last, onPatch, onMove, onRemove }: {
  status: WorkflowStatus; siblings: WorkflowStatus[]; admin: boolean; busy: boolean; first: boolean; last: boolean;
  onPatch: (patch: object) => void; onMove: (delta: -1 | 1) => void; onRemove: () => void;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState(status.name);
  const label = (text: string) => `${text}: ${status.name}`;
  const toggleNext = (id: string) => onPatch({
    allowed_next: status.allowed_next.includes(id) ? status.allowed_next.filter((x) => x !== id) : [...status.allowed_next, id] });
  return <li className="list-none space-y-2 rounded-lg border bg-card p-4">
    <fieldset disabled={!admin || busy} className="flex flex-wrap items-center gap-3">
      <legend className="sr-only">{status.name}</legend>
      <span aria-hidden className={`size-3 shrink-0 rounded-full ${statusDotClass(status.color)}`} />
      <Input className="w-44" aria-label={label(t("settings.workflows.statusName"))} value={name} maxLength={120}
        onChange={(event) => setName(event.target.value)}
        onBlur={() => name.trim() && name.trim() !== status.name && onPatch({ name: name.trim() })} />
      <div role="group" aria-label={label(t("settings.workflows.color"))} className="flex flex-wrap gap-1">
        {statusColorTokens.map((token) => <button key={token} type="button" aria-label={token} aria-pressed={status.color === token} title={token}
          onClick={() => onPatch({ color: token })}
          className={`size-6 rounded-full ${statusDotClass(token)} aria-pressed:ring-2 aria-pressed:ring-foreground aria-pressed:ring-offset-2 focus-visible:outline-2 focus-visible:outline-ring`} />)}
      </div>
      <select className={selectClass} aria-label={label(t("settings.workflows.group"))} value={status.group}
        onChange={(event) => onPatch({ group: event.target.value })}>
        {statusGroups.map((group) => <option key={group} value={group}>{t(`settings.workflows.group.${group}`)}</option>)}
      </select>
      <label className="flex items-center gap-1 text-sm">
        <input type="checkbox" checked={status.auto_assign} onChange={(event) => onPatch({ auto_assign: event.target.checked })} />
        {t("settings.workflows.autoAssign")}<span className="sr-only">: {status.name}</span></label>
      <label className="flex items-center gap-1 text-sm">
        <input type="checkbox" checked={status.lock_transitions} onChange={(event) => onPatch({ lock_transitions: event.target.checked })} />
        {t("settings.workflows.lockTransitions")}<span className="sr-only">: {status.name}</span></label>
      <Button type="button" variant="outline" size="sm" disabled={first} aria-label={label(t("settings.workflows.moveUp"))} onClick={() => onMove(-1)}>↑</Button>
      <Button type="button" variant="outline" size="sm" disabled={last} aria-label={label(t("settings.workflows.moveDown"))} onClick={() => onMove(1)}>↓</Button>
      {admin && <Button type="button" variant="ghost" size="sm" aria-label={label(t("settings.workflows.deleteStatus"))} onClick={onRemove}>×</Button>}
    </fieldset>
    {admin && <StatusGuardEditor status={status} siblings={siblings} busy={busy} onPatch={onPatch} />}
    {status.lock_transitions && <fieldset disabled={!admin || busy} className="flex flex-wrap gap-3 pl-6">
      <legend className="mb-1 text-xs font-medium text-muted-foreground">{t("settings.workflows.allowedNext")}: {status.name}</legend>
      {siblings.filter((s) => s.id !== status.id).map((s) => <label key={s.id} className="flex items-center gap-1 text-sm">
        <input type="checkbox" checked={status.allowed_next.includes(s.id)} onChange={() => toggleNext(s.id)} />{s.name}</label>)}
    </fieldset>}
  </li>;
}
