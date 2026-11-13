import { useId, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import type { Item } from "@/lib/items";
import { formatDate, useProfile } from "@/lib/profile";
import { FormError } from "@/components/auth/form-error";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { ItemUserPicker, type PersonOption } from "@/components/item-user-picker";
import { Button } from "@/components/ui/button";
import { DatePicker, calendarDate, dateValue } from "@/components/ui/date-picker";
import { Input } from "@/components/ui/input";

type Approval = components["schemas"]["Approval"];

/** Item approvals (SPC-12). Status is always text, never colour alone. */
export function ItemApprovals({ item, userId }: { item: Item; userId: string }) {
  const { t } = useTranslation();
  const heading = useId();
  const qc = useQueryClient();
  const profile = useProfile();
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<string>();
  const [deleting, setDeleting] = useState<Approval>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const key = ["item-approvals", item.id];
  const list = useQuery({ queryKey: key, queryFn: () => api<{ items: Approval[] }>(`/api/v1/items/${item.id}/approvals`) });
  const canEdit = item.my_capabilities.includes("edit");

  async function run(op: () => Promise<unknown>) {
    setBusy(true); setError("");
    try { await op(); await qc.invalidateQueries({ queryKey: key }); return true; }
    catch (err) { setError((err as Error).message); await qc.invalidateQueries({ queryKey: key }); return false; }
    finally { setBusy(false); }
  }
  const send = (path: string, method: string, body?: unknown, version?: number) => api(path, {
    method, body: body === undefined ? undefined : JSON.stringify(body), headers: version === undefined ? undefined : { "If-Match": `"${version}"` },
  });

  return <section className="space-y-3 border-t pt-4" aria-labelledby={heading}>
    <div className="flex items-center justify-between gap-2">
      <h3 id={heading} className="font-semibold">{t("items.approvals.title")}</h3>
      {canEdit && !adding && <Button size="sm" variant="outline" onClick={() => setAdding(true)}>{t("items.approvals.add")}</Button>}
    </div>
    {list.isError && <FormError message={t("items.loadError")} />}
    <FormError message={error} />
    {list.data?.items.length === 0 && !adding && <p className="text-muted-foreground">{t("items.approvals.empty")}</p>}
    {list.data?.items.map((approval) => {
      const mine = approval.approvers.find((entry) => entry.user.id === userId);
      return <article key={approval.id} className="space-y-2 rounded border p-3">
        {editing === approval.id
          ? <ApprovalForm initial={approval} busy={busy} onCancel={() => setEditing(undefined)}
            onSubmit={async (body) => { if (await run(() => send(`/api/v1/approvals/${approval.id}`, "PATCH", body, approval.version))) setEditing(undefined); }} />
          : <>
            <div className="flex flex-wrap items-center gap-2">
              <p className="min-w-0 flex-1 break-words font-medium">{approval.description || t("items.approvals.title")}</p>
              <span className="rounded border px-2 py-0.5 text-xs font-medium">
                {t("items.approvals.counter", { approved: approval.approved_count, total: approval.total })} · {t(`items.approvals.status.${approval.status}`)}
              </span>
            </div>
            <p className="text-sm text-muted-foreground">
              {t("items.approvals.createdBy", { name: approval.created_by.name })}
              {approval.due_date && ` · ${t("items.approvals.dueOn", { date: formatDate(approval.due_date, profile) })}`}
            </p>
            <ul className="space-y-1 text-sm">
              {approval.approvers.map((entry) => <li key={entry.user.id}>
                <span className="font-medium">{entry.user.name}</span>: {entry.decision ? t(`items.approvals.status.${entry.decision}`) : t("items.approvals.noDecision")}
                {entry.comment && <span className="text-muted-foreground"> — {entry.comment}</span>}
              </li>)}
            </ul>
            {mine && <Decision key={`${approval.id}-${mine.decision}-${mine.decided_at}`} current={mine} busy={busy}
              onDecide={(decision, comment) => void run(() => send(`/api/v1/approvals/${approval.id}/decision`, "PUT", { decision, comment }))} />}
            {canEdit && <div className="flex gap-2">
              <Button size="sm" variant="ghost" disabled={busy} onClick={() => setEditing(approval.id)}>{t("items.approvals.edit")}</Button>
              <Button size="sm" variant="ghost" disabled={busy} onClick={() => setDeleting(approval)}>{t("items.approvals.delete")}</Button>
            </div>}
          </>}
      </article>;
    })}
    {canEdit && adding && <div className="rounded border p-3">
      <ApprovalForm busy={busy} onCancel={() => setAdding(false)}
        onSubmit={async (body) => { if (await run(() => send(`/api/v1/items/${item.id}/approvals`, "POST", body))) setAdding(false); }} />
    </div>}
    <ConfirmDialog open={!!deleting} onOpenChange={(open) => { if (!open) setDeleting(undefined); }}
      title={t("items.approvals.delete")} description={t("items.approvals.deleteDescription")}
      confirmLabel={t("items.approvals.delete")} cancelLabel={t("items.cancel")} busy={busy}
      onConfirm={async () => { if (deleting && await run(() => send(`/api/v1/approvals/${deleting.id}`, "DELETE"))) setDeleting(undefined); }} />
  </section>;
}

function Decision({ current, busy, onDecide }: { current: Approval["approvers"][number]; busy: boolean; onDecide: (d: "approved" | "rejected", comment: string) => void }) {
  const { t } = useTranslation();
  const [comment, setComment] = useState(current.comment);
  return <div className="space-y-2 border-t pt-2">
    {current.decision && <p className="text-sm font-medium">{t("items.approvals.yourDecision", { decision: t(`items.approvals.status.${current.decision}`) })}</p>}
    <label className="block space-y-1 text-sm">{t("items.approvals.comment")}
      <Input value={comment} onChange={(event) => setComment(event.target.value)} /></label>
    <div className="flex gap-2">
      <Button size="sm" disabled={busy} onClick={() => onDecide("approved", comment)}>{t("items.approvals.approve")}</Button>
      <Button size="sm" variant="outline" disabled={busy} onClick={() => onDecide("rejected", comment)}>{t("items.approvals.reject")}</Button>
    </div>
  </div>;
}

function ApprovalForm({ initial, busy, onSubmit, onCancel }: {
  initial?: Approval; busy: boolean; onCancel: () => void;
  onSubmit: (body: { description: string; due_date: string | null; approver_ids: string[] }) => Promise<void>;
}) {
  const { t } = useTranslation();
  const [description, setDescription] = useState(initial?.description ?? "");
  const [due, setDue] = useState<Date | undefined>(calendarDate(initial?.due_date));
  const [approvers, setApprovers] = useState<PersonOption[]>(initial?.approvers.map((entry) => entry.user) ?? []);
  return <form className="space-y-3" onSubmit={(event) => {
    event.preventDefault();
    if (approvers.length) void onSubmit({ description, due_date: dateValue(due) || null, approver_ids: approvers.map((person) => person.id) });
  }}>
    <label className="block space-y-1 font-medium">{t("items.approvals.description")}
      <Input value={description} onChange={(event) => setDescription(event.target.value)} /></label>
    <div className="space-y-1">
      <DatePicker value={due} onChange={setDue} label={t("items.approvals.due")} />
    </div>
    <ItemUserPicker multiple maxSelected={20} label={t("items.approvals.approvers")} selected={approvers} onChange={setApprovers} />
    {!approvers.length && <p className="text-xs text-muted-foreground">{t("items.approvals.approversRequired")}</p>}
    <div className="flex gap-2">
      <Button type="submit" disabled={busy || !approvers.length}>{t(initial ? "items.approvals.save" : "items.approvals.add")}</Button>
      <Button type="button" variant="ghost" onClick={onCancel}>{t("items.cancel")}</Button>
    </div>
  </form>;
}
