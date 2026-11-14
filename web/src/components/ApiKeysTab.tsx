import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type ApiKey = { id: string; name: string; scope: "read" | "write"; prefix: string; expires_at: string | null; last_used_at: string | null };
type Created = { key: ApiKey; secret: string };

export function ApiKeysTab() {
  const { t, i18n } = useTranslation();
  const cache = useQueryClient();
  const [created, setCreated] = useState<Created | null>(null);
  const [copied, setCopied] = useState(false);
  const keys = useQuery({ queryKey: ["api-keys"], queryFn: () => api<{ items: ApiKey[] }>("/api/v1/api-keys") });
  const done = (c: Created) => { setCreated(c); setCopied(false); void cache.invalidateQueries({ queryKey: ["api-keys"] }); };
  const create = useMutation({ mutationFn: (body: object) => api<Created>("/api/v1/api-keys", { method: "POST", body: JSON.stringify(body) }), onSuccess: done });
  const rotate = useMutation({ mutationFn: (id: string) => api<Created>(`/api/v1/api-keys/${id}/rotate`, { method: "POST" }), onSuccess: done });
  const revoke = useMutation({ mutationFn: (id: string) => api(`/api/v1/api-keys/${id}`, { method: "DELETE" }), onSuccess: () => cache.invalidateQueries({ queryKey: ["api-keys"] }) });
  const error = create.error ?? rotate.error ?? revoke.error ?? keys.error;
  const when = (v: string | null) => v ? new Date(v).toLocaleString(i18n.language) : t("apps.api.never");
  return <section className="space-y-5">
    <p className="text-sm text-muted-foreground">{t("apps.api.hint")}</p>
    {error && <p role="alert">{error.message}</p>}
    {created && <div role="status" className="space-y-2 rounded border p-3">
      <h3 className="font-medium">{t("apps.api.secretTitle")}</h3>
      <p className="text-sm text-muted-foreground">{t("apps.api.secretHint")}</p>
      <code className="block break-all rounded bg-muted p-2 text-sm">{created.secret}</code>
      <div className="flex gap-2">
        <Button type="button" onClick={() => { void navigator.clipboard?.writeText(created.secret); setCopied(true); }}>{copied ? t("apps.api.copied") : t("apps.api.copy")}</Button>
        <Button type="button" variant="outline" onClick={() => setCreated(null)}>{t("apps.api.dismiss")}</Button>
      </div>
    </div>}
    <form className="flex flex-wrap items-end gap-3" onSubmit={(event) => {
      event.preventDefault();
      const data = new FormData(event.currentTarget);
      const expires = String(data.get("expires") ?? "");
      create.mutate({ name: String(data.get("name")), scope: String(data.get("scope")), expires_at: expires ? new Date(expires).toISOString() : null });
      event.currentTarget.reset();
    }}>
      <label className="block text-sm">{t("apps.api.name")}<Input name="name" required maxLength={120} /></label>
      <label className="block text-sm">{t("apps.api.scope")}
        <select name="scope" defaultValue="read" className="block rounded border bg-background p-2">
          <option value="read">{t("apps.api.read")}</option><option value="write">{t("apps.api.write")}</option>
        </select></label>
      <label className="block text-sm">{t("apps.api.expires")}<Input name="expires" type="date" /></label>
      <Button type="submit" disabled={create.isPending}>{t("apps.api.create")}</Button>
    </form>
    {keys.data && !keys.data.items.length && <p className="text-muted-foreground">{t("apps.api.empty")}</p>}
    {!!keys.data?.items.length && <table className="w-full text-left text-sm">
      <caption className="sr-only">{t("apps.tab.api")}</caption>
      <thead><tr><th scope="col" className="p-2">{t("apps.api.name")}</th><th scope="col" className="p-2">{t("apps.api.prefix")}</th>
        <th scope="col" className="p-2">{t("apps.api.scope")}</th><th scope="col" className="p-2">{t("apps.api.lastUsed")}</th><th scope="col" className="p-2"><span className="sr-only">{t("apps.api.actions")}</span></th></tr></thead>
      <tbody>{keys.data.items.map((k) => <tr key={k.id} className="border-t">
        <th scope="row" className="p-2">{k.name}</th><td className="p-2"><code>{k.prefix}…</code></td>
        <td className="p-2">{t(`apps.api.${k.scope}`)}</td><td className="p-2">{when(k.last_used_at)}</td>
        <td className="flex gap-2 p-2">
          <Button type="button" variant="outline" aria-label={`${t("apps.api.rotate")} ${k.name}`} onClick={() => rotate.mutate(k.id)}>{t("apps.api.rotate")}</Button>
          <Button type="button" variant="outline" aria-label={`${t("apps.api.revoke")} ${k.name}`}
            onClick={() => { if (window.confirm(t("apps.api.revokeConfirm"))) revoke.mutate(k.id); }}>{t("apps.api.revoke")}</Button>
        </td></tr>)}</tbody>
    </table>}
  </section>;
}
