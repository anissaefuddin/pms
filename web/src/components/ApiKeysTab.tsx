import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { MoreHorizontal, ShieldCheck } from "lucide-react";
import { api } from "@/lib/api";
import { formatDate, formatDateTime, useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminEmptyState, AdminPageHeader } from "@/components/admin/admin-ui";

type ApiKey = { id: string; name: string; scope: "read" | "write"; prefix: string; expires_at: string | null; last_used_at: string | null };
type Created = { key: ApiKey; secret: string };

/** Keys are the main content. Creating one happens in a dialog; the full secret is shown once, in that dialog only. */
export function ApiKeysTab() {
  const { t } = useTranslation();
  const profile = useProfile();
  const cache = useQueryClient();
  const [creating, setCreating] = useState(false);
  const [created, setCreated] = useState<Created | null>(null);
  const [copied, setCopied] = useState(false);
  const [revoking, setRevoking] = useState<ApiKey>();
  const keys = useQuery({ queryKey: ["api-keys"], queryFn: () => api<{ items: ApiKey[] }>("/api/v1/api-keys") });
  const done = (c: Created) => { setCreated(c); setCopied(false); setCreating(true); void cache.invalidateQueries({ queryKey: ["api-keys"] }); };
  const create = useMutation({ mutationFn: (body: object) => api<Created>("/api/v1/api-keys", { method: "POST", body: JSON.stringify(body) }), onSuccess: done });
  const rotate = useMutation({ mutationFn: (id: string) => api<Created>(`/api/v1/api-keys/${id}/rotate`, { method: "POST" }), onSuccess: done });
  const revoke = useMutation({ mutationFn: (id: string) => api(`/api/v1/api-keys/${id}`, { method: "DELETE" }),
    onSuccess: () => { setRevoking(undefined); void cache.invalidateQueries({ queryKey: ["api-keys"] }); } });
  const error = create.error ?? rotate.error ?? revoke.error ?? keys.error;
  const when = (v: string | null) => v ? formatDateTime(v, profile) : t("apps.api.never");
  const items = keys.data?.items ?? [];
  return <section className="space-y-5">
    <AdminPageHeader title={t("apps.tab.api")} description={t("apps.api.hint")} action={<Button onClick={() => { setCreated(null); setCreating(true); }}>{t("apps.api.createNew")}</Button>} />
    {error && <p role="alert" className="text-sm text-destructive">{error.message}</p>}

    {items.length ? <div className="overflow-x-auto rounded-lg border">
      <table className="w-full text-start text-sm">
        <caption className="sr-only">{t("apps.tab.api")}</caption>
        <thead className="bg-muted/60 text-xs font-medium text-foreground">
          <tr>
            <th scope="col" className="px-3 py-2">{t("apps.api.name")}</th>
            <th scope="col" className="px-3 py-2">{t("apps.api.scope")}</th>
            <th scope="col" className="px-3 py-2">{t("apps.api.expires")}</th>
            <th scope="col" className="px-3 py-2">{t("apps.api.lastUsed")}</th>
            <th scope="col" className="w-12 px-3 py-2"><span className="sr-only">{t("apps.api.actions")}</span></th>
          </tr>
        </thead>
        <tbody className="divide-y">{items.map((k) => <tr key={k.id}>
          <th scope="row" className="px-3 py-2.5 text-start font-medium">
            <span className="block">{k.name}</span>
            <code className="text-xs font-normal text-muted-foreground">{k.prefix}…</code>
          </th>
          <td className="px-3 py-2.5">{t(`apps.api.${k.scope}`)}</td>
          <td className="px-3 py-2.5">{k.expires_at ? formatDate(k.expires_at, profile) : t("apps.api.noExpiry")}</td>
          <td className="px-3 py-2.5">{when(k.last_used_at)}</td>
          <td className="px-3 py-2.5 text-end">
            <DropdownMenu>
              <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label={t("apps.api.actionsFor", { name: k.name })} />}><MoreHorizontal /></DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onClick={() => rotate.mutate(k.id)}>{t("apps.api.rotate")}</DropdownMenuItem>
                <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => setRevoking(k)}>{t("apps.api.revoke")}</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </td>
        </tr>)}</tbody>
      </table>
    </div> : keys.data && <AdminEmptyState title={t("apps.api.empty")} description={t("apps.api.emptyDescription")} />}
    <p className="flex items-start gap-2 rounded-md bg-muted px-3 py-2 text-xs text-foreground"><ShieldCheck aria-hidden="true" className="mt-0.5 size-4 shrink-0" />{t("apps.api.storeNote")}</p>

    <Dialog open={creating} onOpenChange={(open) => { if (!open) { setCreating(false); setCreated(null); } }}>
      <DialogContent className="sm:max-w-md">
        {!created ? <>
          <DialogHeader>
            <DialogTitle>{t("apps.api.createTitle")}</DialogTitle>
            <DialogDescription>{t("apps.api.createDescription")}</DialogDescription>
          </DialogHeader>
          <form id="create-api-key" className="space-y-4" onSubmit={(event) => {
            event.preventDefault();
            const data = new FormData(event.currentTarget);
            const expires = String(data.get("expires") ?? "");
            create.mutate({ name: String(data.get("name")), scope: String(data.get("scope")), expires_at: expires ? new Date(expires).toISOString() : null });
          }}>
            <label className="block space-y-1 text-sm">{t("apps.api.name")}<Input name="name" required maxLength={120} /></label>
            <label className="block space-y-1 text-sm">{t("apps.api.scope")}
              <select name="scope" defaultValue="read" className="block h-9 w-full rounded-md border border-input bg-background px-2 text-sm">
                <option value="read">{t("apps.api.read")}</option><option value="write">{t("apps.api.write")}</option>
              </select>
            </label>
            <label className="block space-y-1 text-sm">{t("apps.api.expires")}<Input name="expires" type="date" /></label>
          </form>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreating(false)}>{t("items.cancel")}</Button>
            <Button type="submit" form="create-api-key" disabled={create.isPending}>{t("apps.api.create")}</Button>
          </DialogFooter>
        </> : <>
          <DialogHeader>
            <DialogTitle>{t("apps.api.secretTitle")}</DialogTitle>
            <DialogDescription>{t("apps.api.secretHint")}</DialogDescription>
          </DialogHeader>
          <code className="block break-all rounded-md bg-muted p-3 text-sm">{created.secret}</code>
          <DialogFooter>
            <Button variant="outline" onClick={() => { void navigator.clipboard?.writeText(created.secret); setCopied(true); }}>{copied ? t("apps.api.copied") : t("apps.api.copy")}</Button>
            <Button onClick={() => { setCreated(null); setCreating(false); }}>{t("apps.api.dismiss")}</Button>
          </DialogFooter>
        </>}
      </DialogContent>
    </Dialog>

    <ConfirmDialog open={!!revoking} onOpenChange={(open) => { if (!open) setRevoking(undefined); }}
      title={t("apps.api.revokeTitle", { name: revoking?.name ?? "" })} description={t("apps.api.revokeConfirm")}
      confirmLabel={t("apps.api.revoke")} cancelLabel={t("items.cancel")} busy={revoke.isPending} error={revoke.error?.message}
      onConfirm={() => { if (revoking) revoke.mutate(revoking.id); }} />
  </section>;
}
