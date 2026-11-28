import { Link, createFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { ErrorState } from "@/components/ui/error-state";
import { LoadingState } from "@/components/ui/loading-state";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminEmptyState, AdminPageHeader, AdminStatus, AdminToolbar } from "@/components/admin/admin-ui";
import { useProfile } from "@/lib/profile";
import { CUSTOMER_PAGE_LIMIT, archiveCustomer, createCustomer, customerErrorKey, customerErrorText, customerKeys, listCustomers, type Customer, type CustomerFilter } from "@/lib/customers";

export const Route = createFileRoute("/_app/settings/customers/")({ component: CustomersPage });

const CUSTOMER_FILTERS: CustomerFilter[] = ["active", "archived"];

/** Workspace Admin only. The API enforces this; the page only avoids showing an empty, failing screen to others. */
function CustomersPage() {
  const { t } = useTranslation();
  const me = useProfile();
  const admin = me?.user_type === "admin";
  const [filter, setFilter] = useState<CustomerFilter>("active");
  const [query, setQuery] = useState("");
  const [creating, setCreating] = useState(false);
  const [archiving, setArchiving] = useState<Customer | null>(null);
  const cache = useQueryClient();
  const list = useQuery({ queryKey: customerKeys.list(filter), enabled: admin, queryFn: () => listCustomers(filter) });
  const archive = useMutation({
    mutationFn: (id: string) => archiveCustomer(id),
    onSuccess: () => {
      toast.success(t("settings.customers.archived.done"));
      setArchiving(null);
      void cache.invalidateQueries({ queryKey: ["clients"] });
    },
  });

  if (me && !admin) return <p role="alert">{t("settings.adminOnly")}</p>;

  const archiveError = archive.isError ? customerErrorText(t, archive.error) : undefined;
  const loaded = list.data?.items ?? [];
  const needle = query.trim().toLowerCase();
  const shown = needle ? loaded.filter((c) => `${c.name} ${c.code} ${c.short_name ?? ""}`.toLowerCase().includes(needle)) : loaded;
  const mayBeTruncated = loaded.length >= CUSTOMER_PAGE_LIMIT;

  return <section className="space-y-5">
    <AdminPageHeader description={t("settings.customers.description")}
      action={<Button onClick={() => setCreating(true)}>{t("settings.customers.new")}</Button>} />

    <div className="flex flex-wrap items-center gap-3">
      <AdminToolbar search={query} onSearch={setQuery} searchLabel={t("settings.customers.search")}>
        <div role="group" aria-label={t("settings.customers.filterLabel")} className="inline-flex rounded-md border p-0.5">
          {CUSTOMER_FILTERS.map((value) => (
            <button key={value} type="button" aria-pressed={filter === value} onClick={() => setFilter(value)}
              className={`rounded px-3 py-1 text-sm ${filter === value ? "bg-accent font-medium" : "text-muted-foreground hover:text-foreground"}`}>
              {t(`settings.customers.filter.${value}`)}
            </button>
          ))}
        </div>
      </AdminToolbar>
    </div>

    {mayBeTruncated && <p className="text-sm text-muted-foreground">{t("settings.customers.truncatedNotice", { count: CUSTOMER_PAGE_LIMIT })}</p>}

    {list.isPending ? <LoadingState label={t("settings.customers.loading")} />
      : list.isError ? <ErrorState message={customerErrorText(t, list.error)} retryLabel={t("settings.customers.retry")} onRetry={() => void list.refetch()} />
      : shown.length === 0 ? <AdminEmptyState title={needle ? t("settings.customers.noMatch") : t(`settings.customers.empty.${filter}`)}
          description={needle ? undefined : t("settings.customers.emptyHint")} action={!needle && filter === "active" ? <Button onClick={() => setCreating(true)}>{t("settings.customers.new")}</Button> : undefined} />
      : <div className="overflow-x-auto rounded-md border">
        <table className="w-full text-sm">
          <thead className="bg-muted/50 text-start text-xs text-muted-foreground">
            <tr>
              <th scope="col" className="px-3 py-2 text-start font-medium">{t("settings.customers.table.name")}</th>
              <th scope="col" className="px-3 py-2 text-start font-medium">{t("settings.customers.table.code")}</th>
              <th scope="col" className="px-3 py-2 text-start font-medium">{t("settings.customers.table.status")}</th>
              <th scope="col" className="px-3 py-2 text-end font-medium"><span className="sr-only">{t("settings.customers.table.actions")}</span></th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {shown.map((c) => (
              <tr key={c.id}>
                <td className="px-3 py-2">
                  <Link to="/settings/customers/$clientId" params={{ clientId: c.id }} className="font-medium text-primary underline-offset-2 hover:underline">{c.name}</Link>
                  {c.short_name && <span className="block text-xs text-muted-foreground">{c.short_name}</span>}
                </td>
                <td className="px-3 py-2 font-mono text-xs">{c.code}</td>
                <td className="px-3 py-2">
                  <AdminStatus tone={c.status === "active" ? "success" : "neutral"} label={t(`settings.customers.status.${c.status}`)} />
                </td>
                <td className="px-3 py-2 text-end">
                  <div className="flex justify-end gap-2">
                    <Link to="/settings/customers/$clientId" params={{ clientId: c.id }} className="inline-flex h-8 items-center rounded-md border px-3 text-sm hover:bg-accent">
                      {c.status === "active" ? t("settings.customers.open") : t("settings.customers.openHistory")}
                    </Link>
                    {c.status === "active" && <Button variant="outline" size="sm" onClick={() => setArchiving(c)}>{t("settings.customers.archive")}</Button>}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>}

    {creating && <CreateCustomerDialog onClose={() => setCreating(false)} />}

    <ConfirmDialog open={archiving !== null} onOpenChange={(open) => { if (!open) { setArchiving(null); archive.reset(); } }}
      title={t("settings.customers.archiveDialog.title", { name: archiving?.name ?? "" })}
      description={t("settings.customers.archiveDialog.description")}
      confirmLabel={t("settings.customers.archiveDialog.confirm")} cancelLabel={t("settings.customers.cancel")}
      onConfirm={() => archiving && archive.mutate(archiving.id)} busy={archive.isPending}
      error={archiveError} />
  </section>;
}

function CreateCustomerDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const cache = useQueryClient();
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [shortName, setShortName] = useState("");
  const [localError, setLocalError] = useState<string | null>(null);
  const create = useMutation({
    mutationFn: () => createCustomer({ code: code.trim(), name: name.trim(), ...(shortName.trim() ? { short_name: shortName.trim() } : {}) }),
    onSuccess: () => {
      toast.success(t("settings.customers.create.saved"));
      void cache.invalidateQueries({ queryKey: ["clients"] });
      onClose();
    },
  });
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (create.isPending) return; // no second request while the first is pending
    if (!code.trim()) return setLocalError(t("settings.customers.create.codeRequired"));
    if (!name.trim()) return setLocalError(t("settings.customers.create.nameRequired"));
    setLocalError(null);
    create.mutate();
  };
  const duplicate = create.isError && customerErrorKey(create.error).key === "settings.customers.errors.duplicateCode";
  const message = localError ?? (create.isError ? customerErrorText(t, create.error) : null);
  return <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{t("settings.customers.create.title")}</DialogTitle>
        <DialogDescription>{t("settings.customers.create.description")}</DialogDescription>
      </DialogHeader>
      <form onSubmit={submit} className="space-y-4" aria-label={t("settings.customers.create.title")}>
        <label className="block space-y-1 text-sm">
          <span className="font-medium">{t("settings.customers.create.code")}</span>
          <Input value={code} onChange={(e) => setCode(e.target.value)} autoFocus aria-invalid={duplicate || undefined} />
          <span className="block text-xs text-muted-foreground">{t("settings.customers.create.codeHint")}</span>
        </label>
        <label className="block space-y-1 text-sm">
          <span className="font-medium">{t("settings.customers.create.name")}</span>
          <Input value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        <label className="block space-y-1 text-sm">
          <span className="font-medium">{t("settings.customers.create.shortName")}</span>
          <Input value={shortName} onChange={(e) => setShortName(e.target.value)} />
        </label>
        {message && <p role="alert" className="text-sm text-destructive">{message}</p>}
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>{t("settings.customers.cancel")}</Button>
          <Button type="submit" disabled={create.isPending}>{create.isPending ? t("settings.customers.create.saving") : t("settings.customers.create.submit")}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>;
}
