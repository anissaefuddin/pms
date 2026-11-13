import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type Hook = { id: string; name: string; url: string; event_types: string[]; enabled: boolean; consecutive_failures: number; disabled_reason: string | null; secret?: string };
type Delivery = { id: string; event_type: string; status: string; attempts: number; response_status: number | null; last_error: string | null; created_at: string };

// Webhook management tab for Apps & Integrations (API-02).
export function WebhooksTab() {
  const { t } = useTranslation("translation", { keyPrefix: "webhooks" });
  const cache = useQueryClient();
  const [secret, setSecret] = useState<string | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const [picked, setPicked] = useState<string[]>([]);
  const hooks = useQuery({ queryKey: ["webhooks"], queryFn: () => api<{ items: Hook[]; event_types: string[] }>("/api/v1/webhooks") });
  const refresh = () => cache.invalidateQueries({ queryKey: ["webhooks"] });
  const create = useMutation({
    mutationFn: (body: object) => api<Hook>("/api/v1/webhooks", { method: "POST", body: JSON.stringify(body) }),
    onSuccess: (made) => { setSecret(made.secret ?? null); setPicked([]); refresh(); },
  });
  const patch = useMutation({ mutationFn: (v: { id: string; enabled: boolean }) => api(`/api/v1/webhooks/${v.id}`, { method: "PATCH", body: JSON.stringify({ enabled: v.enabled }) }), onSuccess: refresh });
  const remove = useMutation({ mutationFn: (id: string) => api(`/api/v1/webhooks/${id}`, { method: "DELETE" }), onSuccess: refresh });
  const log = useQuery({ queryKey: ["webhook-log", open], enabled: !!open, queryFn: () => api<{ items: Delivery[] }>(`/api/v1/webhooks/${open}/deliveries`) });
  const error = create.error ?? patch.error ?? remove.error ?? hooks.error;
  return <section className="space-y-5" aria-labelledby="webhooks-title">
    <h2 id="webhooks-title" className="text-lg font-semibold">{t("title")}</h2>
    <p className="text-sm text-muted-foreground">{t("hint")}</p>
    {error && <p role="alert">{error.message}</p>}
    {secret && <div role="status" className="space-y-2 rounded border p-3">
      <p className="font-medium">{t("secretTitle")}</p>
      <code data-testid="webhook-secret" className="block break-all">{secret}</code>
      <p className="text-sm text-muted-foreground">{t("secretHint")}</p>
      <Button type="button" onClick={() => setSecret(null)}>{t("secretDone")}</Button>
    </div>}
    <form className="space-y-3" onSubmit={(event) => {
      event.preventDefault();
      const data = new FormData(event.currentTarget);
      create.mutate({ name: String(data.get("name")), url: String(data.get("url")), event_types: picked });
      event.currentTarget.reset();
    }}>
      <label className="block text-sm">{t("name")}<Input name="name" required maxLength={100} /></label>
      <label className="block text-sm">{t("url")}<Input name="url" type="url" required /></label>
      <fieldset className="flex flex-wrap gap-3 text-sm"><legend>{t("events")}</legend>
        {hooks.data?.event_types.map((type) => <label key={type} className="flex items-center gap-1">
          <input type="checkbox" checked={picked.includes(type)} onChange={(e) => setPicked(e.target.checked ? [...picked, type] : picked.filter((x) => x !== type))} />{type}</label>)}
      </fieldset>
      <Button type="submit" disabled={create.isPending || !picked.length}>{t("create")}</Button>
    </form>
    {hooks.data && !hooks.data.items.length && <p className="text-muted-foreground">{t("empty")}</p>}
    <ul className="space-y-3">{hooks.data?.items.map((h) => <li key={h.id} className="rounded border p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div><p className="font-medium">{h.name}</p><p className="break-all text-sm text-muted-foreground">{h.url}</p>
          <p className="text-xs">{h.event_types.join(", ")}</p>
          {!h.enabled && <p role="status" className="text-sm text-destructive">{t("disabled", { reason: h.disabled_reason ?? "" })}</p>}</div>
        <div className="flex gap-2">
          <Button type="button" variant="outline" onClick={() => patch.mutate({ id: h.id, enabled: !h.enabled })}>{h.enabled ? t("disable") : t("enable")}</Button>
          <Button type="button" variant="outline" aria-expanded={open === h.id} onClick={() => setOpen(open === h.id ? null : h.id)}>{t("log")}</Button>
          <Button type="button" variant="outline" onClick={() => remove.mutate(h.id)}>{t("delete")}</Button>
        </div>
      </div>
      {open === h.id && <div className="mt-3">
        {log.data && !log.data.items.length && <p className="text-sm text-muted-foreground">{t("noLog")}</p>}
        {!!log.data?.items.length && <table className="w-full text-left text-sm"><caption className="sr-only">{t("log")}</caption><tbody>
          {log.data.items.map((d) => <tr key={d.id} className="border-t"><td className="p-1">{d.event_type}</td><td className="p-1">{d.status}</td>
            <td className="p-1">{t("attempts", { count: d.attempts })}</td><td className="p-1">{d.response_status ?? d.last_error ?? ""}</td></tr>)}
        </tbody></table>}
      </div>}
    </li>)}</ul>
  </section>;
}
