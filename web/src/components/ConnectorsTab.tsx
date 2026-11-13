import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";

type Install = { id: string; kind: string; name: string; enabled: boolean; secret_names: string[] };
type Conflict = { id: string; item_title: string; remote_title: string; external_id: string };

// Connectors tab of Apps & Integrations: list, enable/disable, and resolve open sync conflicts.
export function ConnectorsTab() {
  const { t } = useTranslation();
  const cache = useQueryClient();
  const list = useQuery({ queryKey: ["connectors"], queryFn: () => api<{ kinds: string[]; items: Install[] }>("/api/v1/connectors") });
  const refresh = () => cache.invalidateQueries({ queryKey: ["connectors"] });
  const toggle = useMutation({
    mutationFn: (c: Install) => api(`/api/v1/connectors/${c.id}/${c.enabled ? "disable" : "enable"}`, { method: "POST" }),
    onSuccess: refresh,
  });
  const sync = useMutation({ mutationFn: (id: string) => api(`/api/v1/connectors/${id}/sync`, { method: "POST" }), onSuccess: refresh });
  if (list.error) return <p role="alert">{list.error.message}</p>;
  if (list.data && !list.data.items.length) return <p className="text-muted-foreground">{t("settings.apps.connectors.empty")}</p>;
  return <ul className="space-y-4">
    {list.data?.items.map((c) => <li key={c.id} className="rounded border p-3">
      <div className="flex flex-wrap items-center gap-3">
        <strong>{c.name}</strong><span className="text-sm text-muted-foreground">{c.kind}</span>
        <span className="text-sm">{c.enabled ? t("settings.apps.connectors.enabled") : t("settings.apps.connectors.disabled")}</span>
        <Button type="button" variant="outline" onClick={() => toggle.mutate(c)}>{c.enabled ? t("settings.apps.connectors.disable") : t("settings.apps.connectors.enable")}</Button>
        {c.enabled && <Button type="button" variant="outline" onClick={() => sync.mutate(c.id)}>{t("settings.apps.connectors.sync")}</Button>}
      </div>
      <Conflicts id={c.id} />
    </li>)}
  </ul>;
}

function Conflicts({ id }: { id: string }) {
  const { t } = useTranslation();
  const cache = useQueryClient();
  const open = useQuery({ queryKey: ["connector-conflicts", id], queryFn: () => api<{ items: Conflict[] }>(`/api/v1/connectors/${id}/conflicts`) });
  const resolve = useMutation({
    mutationFn: (v: { id: string; resolution: "item" | "remote" }) => api(`/api/v1/connector-conflicts/${v.id}/resolve`, { method: "POST", body: JSON.stringify({ resolution: v.resolution }) }),
    onSuccess: () => cache.invalidateQueries({ queryKey: ["connector-conflicts", id] }),
  });
  if (!open.data?.items.length) return null;
  return <ul className="mt-3 space-y-2" aria-label={t("settings.apps.connectors.conflicts")}>
    {open.data.items.map((c) => <li key={c.id} className="flex flex-wrap items-center gap-3 text-sm">
      <span>{t("settings.apps.connectors.conflictLine", { item: c.item_title, remote: c.remote_title })}</span>
      <Button type="button" variant="outline" onClick={() => resolve.mutate({ id: c.id, resolution: "item" })}>{t("settings.apps.connectors.keepItem")}</Button>
      <Button type="button" variant="outline" onClick={() => resolve.mutate({ id: c.id, resolution: "remote" })}>{t("settings.apps.connectors.keepRemote")}</Button>
    </li>)}
  </ul>;
}
