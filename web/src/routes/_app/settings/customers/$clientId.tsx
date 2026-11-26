import { Link, createFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { components } from "@/api/schema";
import { api, type CurrentUser } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { ErrorState } from "@/components/ui/error-state";
import { LoadingState } from "@/components/ui/loading-state";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminEmptyState, AdminPageHeader, AdminStatus, AdminToolbar, InitialsAvatar } from "@/components/admin/admin-ui";
import { useProfile, formatDateTime } from "@/lib/profile";
import {
  archiveCustomer, customerErrorText, customerKeys, eligibleForMembership, getCustomer, grantMembership, listMembers,
  revokeMembership, updateCustomer, type Customer, type CustomerMembership,
} from "@/lib/customers";

export const Route = createFileRoute("/_app/settings/customers/$clientId")({ component: CustomerDetail });

type Tab = "general" | "members";
const DETAIL_TABS: Tab[] = ["general", "members"];

/** One customer. Workspace Admin only; the API enforces it, this page only refuses to render for others. */
function CustomerDetail() {
  const { t } = useTranslation();
  const me = useProfile();
  const { clientId } = Route.useParams();
  const admin = me?.user_type === "admin";
  const [tab, setTab] = useState<Tab>("general");
  const detail = useQuery({ queryKey: customerKeys.detail(clientId), enabled: admin, queryFn: () => getCustomer(clientId) });

  if (me && !admin) return <p role="alert">{t("settings.adminOnly")}</p>;

  return <section className="space-y-5">
    <div className="flex flex-wrap items-center gap-3">
      <Link to="/settings/customers" className="text-sm text-muted-foreground underline-offset-2 hover:underline">{t("settings.customers.backToList")}</Link>
    </div>
    {detail.isPending ? <LoadingState label={t("settings.customers.loadingOne")} />
      : detail.isError ? <ErrorState message={customerErrorText(t, detail.error)} retryLabel={t("settings.customers.retry")} onRetry={() => void detail.refetch()} />
      : <>
        <AdminPageHeader title={detail.data.name}
          description={detail.data.status === "archived" ? t("settings.customers.archivedNotice") : t("settings.customers.description.detail")} />
        <div role="tablist" aria-label={t("settings.customers.tabsLabel")} className="flex gap-1 border-b">
          {DETAIL_TABS.map((value) => (
            <button key={value} role="tab" type="button" aria-selected={tab === value} aria-controls={`customer-${value}`}
              onClick={() => setTab(value)}
              className={`-mb-px border-b-2 px-3 pb-2 text-sm ${tab === value ? "border-primary font-medium text-foreground" : "border-transparent text-muted-foreground hover:text-foreground"}`}>
              {t(`settings.customers.tab.${value}`)}
            </button>
          ))}
        </div>
        {/* key: switching customers remounts the panels, so no form or dialog state carries over to another customer. */}
        {tab === "general"
          ? <GeneralPanel key={`general-${clientId}`} customer={detail.data} profile={me} />
          : <MembersPanel key={`members-${clientId}`} customer={detail.data} />}
      </>}
  </section>;
}

function GeneralPanel({ customer, profile }: { customer: Customer; profile: CurrentUser | undefined }) {
  const { t } = useTranslation();
  const cache = useQueryClient();
  const archived = customer.status === "archived";
  const [name, setName] = useState(customer.name);
  const [shortName, setShortName] = useState(customer.short_name ?? "");
  const [archiveOpen, setArchiveOpen] = useState(false);
  const save = useMutation({
    mutationFn: () => updateCustomer(customer.id, { name: name.trim(), short_name: shortName.trim() }),
    onSuccess: (next) => {
      cache.setQueryData(customerKeys.detail(customer.id), next);
      toast.success(t("settings.customers.edit.saved"));
      void cache.invalidateQueries({ queryKey: ["clients"] });
    },
  });
  const archive = useMutation({
    mutationFn: () => archiveCustomer(customer.id),
    onSuccess: (next) => {
      cache.setQueryData(customerKeys.detail(customer.id), next);
      setArchiveOpen(false);
      toast.success(t("settings.customers.archived.done"));
      void cache.invalidateQueries({ queryKey: ["clients"] });
    },
  });
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (save.isPending || !name.trim()) return;
    save.mutate();
  };
  const dirty = name.trim() !== customer.name || shortName.trim() !== (customer.short_name ?? "");
  return <div id="customer-general" role="tabpanel" className="space-y-6">
    <dl className="grid gap-4 sm:grid-cols-2">
      <Field label={t("settings.customers.fields.code")} value={customer.code} mono />
      <Field label={t("settings.customers.fields.status")} value={<AdminStatus tone={archived ? "neutral" : "success"} label={t(`settings.customers.status.${customer.status}`)} />} />
      <Field label={t("settings.customers.fields.created")} value={formatDateTime(customer.created_at, profile)} />
      <Field label={t("settings.customers.fields.updated")} value={formatDateTime(customer.updated_at, profile)} />
      {archived && <Field label={t("settings.customers.fields.archivedAt")} value={formatDateTime(customer.archived_at ?? "", profile)} />}
    </dl>

    <form onSubmit={submit} className="max-w-xl space-y-4" aria-label={t("settings.customers.tab.general")}>
      <label className="block space-y-1 text-sm">
        <span className="font-medium">{t("settings.customers.fields.name")}</span>
        <Input value={name} onChange={(e) => setName(e.target.value)} disabled={archived} />
      </label>
      <label className="block space-y-1 text-sm">
        <span className="font-medium">{t("settings.customers.fields.shortName")}</span>
        <Input value={shortName} onChange={(e) => setShortName(e.target.value)} disabled={archived} />
        <span className="block text-xs text-muted-foreground">{t("settings.customers.edit.shortNameHint")}</span>
      </label>
      {save.isError && <p role="alert" className="text-sm text-destructive">{customerErrorText(t, save.error)}</p>}
      {archive.isError && <p role="alert" className="text-sm text-destructive">{customerErrorText(t, archive.error)}</p>}
      {!archived && <div className="flex gap-2">
        <Button type="submit" disabled={!dirty || save.isPending || !name.trim()}>{save.isPending ? t("settings.customers.edit.saving") : t("settings.customers.edit.save")}</Button>
        <Button type="button" variant="outline" disabled={!dirty || save.isPending} onClick={() => { setName(customer.name); setShortName(customer.short_name ?? ""); }}>{t("settings.customers.edit.cancel")}</Button>
      </div>}
    </form>

    {!archived && <div className="space-y-2 border-t pt-6">
      <h2 className="text-sm font-medium">{t("settings.customers.archive.heading")}</h2>
      <p className="max-w-xl text-sm text-muted-foreground">{t("settings.customers.archive.explain")}</p>
      <Button variant="outline" onClick={() => setArchiveOpen(true)}>{t("settings.customers.archive")}</Button>
    </div>}

    <ConfirmDialog open={archiveOpen} onOpenChange={(open) => { if (!open) { setArchiveOpen(false); archive.reset(); } }}
      title={t("settings.customers.archiveDialog.title", { name: customer.name })} description={t("settings.customers.archiveDialog.description")}
      confirmLabel={t("settings.customers.archiveDialog.confirm")} cancelLabel={t("settings.customers.cancel")}
      onConfirm={() => archive.mutate()} busy={archive.isPending}
      error={archive.isError ? customerErrorText(t, archive.error) : undefined} />
  </div>;
}

function Field({ label, value, mono }: { label: string; value: ReactNode; mono?: boolean }) {
  return <div className="space-y-1">
    <dt className="text-xs text-muted-foreground">{label}</dt>
    <dd className={`text-sm ${mono ? "font-mono" : ""}`}>{value}</dd>
  </div>;
}

function MembersPanel({ customer }: { customer: Customer }) {
  const { t } = useTranslation();
  const cache = useQueryClient();
  const [history, setHistory] = useState(false);
  const [adding, setAdding] = useState(false);
  const [revoking, setRevoking] = useState<CustomerMembership | null>(null);
  const archived = customer.status === "archived";
  const members = useQuery({ queryKey: customerKeys.members(customer.id, history), queryFn: () => listMembers(customer.id, history) });
  const revoke = useMutation({
    mutationFn: (userId: string) => revokeMembership(customer.id, userId),
    onSuccess: () => {
      setRevoking(null);
      toast.success(t("settings.customers.members.revoked"));
      void cache.invalidateQueries({ queryKey: ["client-members", customer.id] });
    },
  });
  const rows = members.data?.items ?? [];
  return <div id="customer-members" role="tabpanel" className="space-y-5">
    <p className="max-w-2xl rounded-md border bg-muted/40 p-3 text-sm">{t("settings.customers.members.info")}</p>

    <div className="flex flex-wrap items-center justify-between gap-3">
      <label className="inline-flex items-center gap-2 text-sm">
        <input type="checkbox" checked={history} onChange={(e) => setHistory(e.target.checked)} className="size-4" />
        {t("settings.customers.members.showHistory")}
      </label>
      <div className="flex flex-col items-end gap-1">
        <Button onClick={() => setAdding(true)} disabled={archived}>{t("settings.customers.members.add")}</Button>
        {archived && <span className="text-xs text-muted-foreground">{t("settings.customers.members.archivedNoGrant")}</span>}
      </div>
    </div>

    {members.isPending ? <LoadingState label={t("settings.customers.members.loading")} />
      : members.isError ? <ErrorState message={customerErrorText(t, members.error)} retryLabel={t("settings.customers.retry")} onRetry={() => void members.refetch()} />
      : rows.length === 0 ? <AdminEmptyState title={t(history ? "settings.customers.members.emptyHistory" : "settings.customers.members.empty")} />
      : <div className="overflow-x-auto rounded-md border">
        <table className="w-full text-sm">
          <thead className="bg-muted/50 text-xs text-muted-foreground">
            <tr>
              <th scope="col" className="px-3 py-2 text-start font-medium">{t("settings.customers.members.person")}</th>
              <th scope="col" className="px-3 py-2 text-start font-medium">{t("settings.customers.members.status")}</th>
              <th scope="col" className="px-3 py-2 text-start font-medium">{t("settings.customers.members.grantedAt")}</th>
              <th scope="col" className="px-3 py-2 text-start font-medium">{t("settings.customers.members.revokedAt")}</th>
              <th scope="col" className="px-3 py-2 text-end font-medium"><span className="sr-only">{t("settings.customers.table.actions")}</span></th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {rows.map((row) => (
              <tr key={row.id}>
                <td className="px-3 py-2">{row.user_name || t("settings.customers.members.unknownPerson")}</td>
                <td className="px-3 py-2"><AdminStatus tone={row.status === "active" ? "success" : "neutral"} label={t(`settings.customers.membership.${row.status}`)} /></td>
                <td className="px-3 py-2 text-xs">{formatDateTime(row.granted_at)}</td>
                <td className="px-3 py-2 text-xs">{row.revoked_at ? formatDateTime(row.revoked_at) : "—"}</td>
                <td className="px-3 py-2 text-end">
                  {row.status === "active" && <Button variant="outline" size="sm" onClick={() => setRevoking(row)}>{t("settings.customers.members.revoke")}</Button>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>}

    {adding && <AddMemberDialog customer={customer} activeIds={new Set(rows.filter((r) => r.status === "active").map((r) => r.user_id))} onClose={() => setAdding(false)} />}

    <ConfirmDialog open={revoking !== null} onOpenChange={(open) => { if (!open) { setRevoking(null); revoke.reset(); } }}
      title={t("settings.customers.members.revokeTitle", { name: revoking?.user_name ?? "" })}
      description={t("settings.customers.members.revokeDescription")}
      confirmLabel={t("settings.customers.members.revokeConfirm")} cancelLabel={t("settings.customers.cancel")}
      onConfirm={() => revoking && revoke.mutate(revoking.user_id)} busy={revoke.isPending}
      error={revoke.isError ? customerErrorText(t, revoke.error) : undefined} />
  </div>;
}

type EligibleUser = components["schemas"]["User"];

function AddMemberDialog({ customer, activeIds, onClose }: { customer: Customer; activeIds: Set<string>; onClose: () => void }) {
  const { t } = useTranslation();
  const cache = useQueryClient();
  const [query, setQuery] = useState("");
  const users = useQuery({
    queryKey: ["users-for-membership", query.trim()],
    queryFn: () => api<{ items: EligibleUser[] }>(`/api/v1/users?status=active&limit=50${query.trim() ? `&q=${encodeURIComponent(query.trim())}` : ""}`),
  });
  const grant = useMutation({
    mutationFn: (userId: string) => grantMembership(customer.id, userId),
    onSuccess: () => {
      toast.success(t("settings.customers.members.granted"));
      void cache.invalidateQueries({ queryKey: ["client-members", customer.id] });
      onClose();
    },
  });
  const eligible = eligibleForMembership(users.data?.items ?? [], activeIds);
  return <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{t("settings.customers.members.pickTitle")}</DialogTitle>
        <DialogDescription>{t("settings.customers.members.pickDescription")}</DialogDescription>
      </DialogHeader>
      <AdminToolbar search={query} onSearch={setQuery} searchLabel={t("settings.customers.members.search")} />
      {users.isPending ? <LoadingState label={t("settings.customers.members.loadingUsers")} />
        : users.isError ? <ErrorState message={customerErrorText(t, users.error)} retryLabel={t("settings.customers.retry")} onRetry={() => void users.refetch()} />
        : eligible.length === 0 ? <p className="text-sm text-muted-foreground">{t("settings.customers.members.noEligible")}</p>
        : <ul className="max-h-72 divide-y overflow-y-auto rounded-md border" aria-label={t("settings.customers.members.pickTitle")}>
          {eligible.map((user) => {
            const name = `${user.first_name} ${user.last_name}`.trim() || user.email;
            return <li key={user.id} className="flex items-center gap-3 px-3 py-2">
              <InitialsAvatar name={name} size="size-7" />
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium">{name}</span>
                <span className="block truncate text-xs text-muted-foreground">{user.email}</span>
              </span>
              <Button size="sm" variant="outline" disabled={grant.isPending} onClick={() => grant.mutate(user.id)}
                aria-label={t("settings.customers.members.grantFor", { name })}>{t("settings.customers.members.grant")}</Button>
            </li>;
          })}
        </ul>}
      {grant.isError && <p role="alert" className="text-sm text-destructive">{customerErrorText(t, grant.error)}</p>}
      <DialogFooter>
        <Button variant="outline" onClick={onClose}>{t("settings.customers.cancel")}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>;
}
