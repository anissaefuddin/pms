import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type Client = { id: string; client_id: string; name: string; redirect_uris: string[] };

export function OAuthClients() {
  const { t } = useTranslation();
  const cache = useQueryClient();
  const [secret, setSecret] = useState<string | null>(null);
  const list = useQuery({ queryKey: ["oauth-clients"], queryFn: () => api<{ items: Client[] }>("/api/v1/oauth/clients") });
  const create = useMutation({
    mutationFn: (body: object) => api<{ client_secret: string }>("/api/v1/oauth/clients", { method: "POST", body: JSON.stringify(body) }),
    onSuccess: (made) => { setSecret(made.client_secret); cache.invalidateQueries({ queryKey: ["oauth-clients"] }); },
  });
  const remove = useMutation({
    mutationFn: (id: string) => api(`/api/v1/oauth/clients/${id}`, { method: "DELETE" }),
    onSuccess: () => cache.invalidateQueries({ queryKey: ["oauth-clients"] }),
  });
  return <section className="space-y-4">
    <p className="text-sm text-muted-foreground">{t("oauth.clients.hint")}</p>
    {(list.error || create.error || remove.error) && <p role="alert">{(list.error ?? create.error ?? remove.error)?.message}</p>}
    {secret && <div role="status" className="rounded border p-3">
      <p className="font-medium">{t("oauth.clients.secretTitle")}</p>
      <code className="block break-all">{secret}</code>
      <p className="text-sm text-muted-foreground">{t("oauth.clients.secretHint")}</p>
    </div>}
    <form className="space-y-3" onSubmit={(event) => {
      event.preventDefault();
      const data = new FormData(event.currentTarget);
      create.mutate({ name: String(data.get("name")), redirect_uris: String(data.get("uris")).split("\n").map((u) => u.trim()).filter(Boolean) });
      event.currentTarget.reset();
    }}>
      <label className="block text-sm">{t("oauth.clients.name")}<Input name="name" required maxLength={120} /></label>
      <label className="block text-sm">{t("oauth.clients.redirects")}<textarea name="uris" required rows={3} className="block w-full rounded border bg-background p-2" /></label>
      <Button type="submit" disabled={create.isPending}>{t("oauth.clients.create")}</Button>
    </form>
    {list.data && !list.data.items.length && <p className="text-muted-foreground">{t("oauth.clients.empty")}</p>}
    <ul className="divide-y rounded border">
      {list.data?.items.map((c) => <li key={c.id} className="flex items-center justify-between gap-3 p-3">
        <div className="min-w-0"><p className="font-medium">{c.name}</p>
          <p className="break-all text-sm text-muted-foreground">{t("oauth.clients.clientId")}: {c.client_id}</p>
          <p className="break-all text-sm text-muted-foreground">{c.redirect_uris.join(", ")}</p></div>
        <Button variant="outline" onClick={() => remove.mutate(c.id)}>{t("oauth.clients.remove")}</Button>
      </li>)}
    </ul>
  </section>;
}
