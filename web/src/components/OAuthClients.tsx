import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus, Trash2, Copy } from "lucide-react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { FormError } from "@/components/auth/form-error";
import { AdminEmptyState, AdminPageHeader } from "@/components/admin/admin-ui";

type Client = { id: string; client_id: string; name: string; redirect_uris: string[] };

/** Registered apps as cards. Registration happens in a dialog; redirect URIs are one input each, sent as the list. */
export function OAuthClients() {
  const { t } = useTranslation();
  const cache = useQueryClient();
  const [creating, setCreating] = useState(false);
  const [uris, setUris] = useState([""]);
  const [secret, setSecret] = useState<string | null>(null);
  const [removing, setRemoving] = useState<Client>();
  const list = useQuery({ queryKey: ["oauth-clients"], queryFn: () => api<{ items: Client[] }>("/api/v1/oauth/clients") });
  const create = useMutation({
    mutationFn: (body: object) => api<{ client_secret: string }>("/api/v1/oauth/clients", { method: "POST", body: JSON.stringify(body) }),
    onSuccess: (made) => { setSecret(made.client_secret); void cache.invalidateQueries({ queryKey: ["oauth-clients"] }); },
  });
  const remove = useMutation({
    mutationFn: (id: string) => api(`/api/v1/oauth/clients/${id}`, { method: "DELETE" }),
    onSuccess: () => { setRemoving(undefined); void cache.invalidateQueries({ queryKey: ["oauth-clients"] }); },
  });
  const error = list.error ?? create.error ?? remove.error;

  function close() { setCreating(false); setSecret(null); setUris([""]); create.reset(); }

  return <section className="space-y-6">
    <AdminPageHeader description={t("oauth.clients.hint")} action={<Button onClick={() => setCreating(true)}><Plus />{t("oauth.clients.create")}</Button>} />
    <FormError message={error?.message ?? ""} />

    {list.data && !list.data.items.length && <AdminEmptyState title={t("oauth.clients.emptyTitle")} description={t("oauth.clients.empty")} />}
    {!!list.data?.items.length && <ul className="grid gap-3 md:grid-cols-2">
      {list.data.items.map((client) => <li key={client.id} className="space-y-3 rounded-lg border p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="font-medium">{client.name}</p>
            <p className="text-xs text-muted-foreground">{t("oauth.clients.authCodePkce")}</p>
          </div>
          <Button variant="ghost" size="sm" aria-label={t("oauth.clients.removeNamed", { name: client.name })} onClick={() => setRemoving(client)}><Trash2 />{t("oauth.clients.remove")}</Button>
        </div>
        <div>
          <p className="text-xs text-muted-foreground">{t("oauth.clients.clientId")}</p>
          <div className="flex items-center gap-2">
            <code className="min-w-0 flex-1 truncate rounded-md bg-muted px-2 py-1 text-xs">{client.client_id}</code>
            <Button variant="ghost" size="icon-sm" aria-label={t("oauth.clients.copyNamed", { name: client.name })} onClick={() => void navigator.clipboard?.writeText(client.client_id)}><Copy /></Button>
          </div>
        </div>
        <div>
          <p className="text-xs text-muted-foreground">{t("oauth.clients.redirectUris")}</p>
          <ul className="space-y-1 text-sm">{client.redirect_uris.map((uri) => <li key={uri} className="break-all">{uri}</li>)}</ul>
        </div>
      </li>)}
    </ul>}

    <Dialog open={creating} onOpenChange={(open) => { if (!open) close(); }}>
      <DialogContent className="sm:max-w-lg">
        {!secret ? <>
          <DialogHeader>
            <DialogTitle>{t("oauth.clients.registerTitle")}</DialogTitle>
            <DialogDescription>{t("oauth.clients.registerDescription")}</DialogDescription>
          </DialogHeader>
          <form id="register-oauth-app" className="space-y-4" onSubmit={(event) => {
            event.preventDefault();
            const data = new FormData(event.currentTarget);
            create.mutate({ name: String(data.get("name")), redirect_uris: uris.map((uri) => uri.trim()).filter(Boolean) });
          }}>
            <label className="block space-y-1 text-sm">{t("oauth.clients.name")}<Input name="name" required maxLength={120} /></label>
            <fieldset className="space-y-2">
              <legend className="text-sm font-medium">{t("oauth.clients.redirectUris")}</legend>
              {uris.map((uri, index) => <div key={index} className="flex gap-2">
                <Input type="url" required aria-label={t("oauth.clients.redirectNumber", { number: index + 1 })} value={uri} placeholder={t("oauth.clients.redirectPlaceholder")}
                  onChange={(event) => setUris(uris.map((entry, position) => position === index ? event.target.value : entry))} />
                {uris.length > 1 && <Button type="button" variant="ghost" size="icon-sm" aria-label={t("oauth.clients.removeRedirect", { number: index + 1 })}
                  onClick={() => setUris(uris.filter((_, position) => position !== index))}><Trash2 /></Button>}
              </div>)}
              <Button type="button" variant="outline" size="sm" onClick={() => setUris([...uris, ""])}><Plus />{t("oauth.clients.addRedirect")}</Button>
            </fieldset>
            <FormError message={create.error?.message ?? ""} />
          </form>
          <DialogFooter>
            <Button variant="outline" onClick={close}>{t("items.cancel")}</Button>
            <Button type="submit" form="register-oauth-app" disabled={create.isPending}>{t("oauth.clients.create")}</Button>
          </DialogFooter>
        </> : <>
          <DialogHeader>
            <DialogTitle>{t("oauth.clients.secretTitle")}</DialogTitle>
            <DialogDescription>{t("oauth.clients.secretHint")}</DialogDescription>
          </DialogHeader>
          <code className="block break-all rounded-md bg-muted p-3 text-sm">{secret}</code>
          <DialogFooter>
            <Button variant="outline" onClick={() => void navigator.clipboard?.writeText(secret)}><Copy />{t("apps.api.copy")}</Button>
            <Button onClick={close}>{t("apps.api.dismiss")}</Button>
          </DialogFooter>
        </>}
      </DialogContent>
    </Dialog>

    <ConfirmDialog open={!!removing} onOpenChange={(open) => { if (!open) setRemoving(undefined); }}
      title={t("oauth.clients.removeTitle", { name: removing?.name ?? "" })} description={t("oauth.clients.removeBody")}
      confirmLabel={t("oauth.clients.remove")} cancelLabel={t("items.cancel")} busy={remove.isPending} error={remove.error?.message}
      onConfirm={() => { if (removing) remove.mutate(removing.id); }} />
  </section>;
}
