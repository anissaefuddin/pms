import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { useSpaces } from "@/lib/space-queries";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type Install = { id: string; kind: string; name: string; enabled: boolean; secret_names: string[]; config: Record<string, unknown>; last_sync_at: string | null; last_sync_ok: boolean | null; last_sync_message: string | null };
type Conflict = { id: string; item_title: string; remote_title: string; external_id: string; differs?: string };
const PILOTS = ["github", "google_calendar"] as const;

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
  return <div className="space-y-5">
    <AddConnector kinds={list.data?.kinds ?? []} onDone={refresh} />
    {list.data && !list.data.items.length && <p className="text-muted-foreground">{t("settings.apps.connectors.empty")}</p>}
    <InstallList items={list.data?.items ?? []} toggle={(c) => toggle.mutate(c)} sync={(id) => sync.mutate(id)} />
  </div>;
}

// Install form for the two pilot connectors (INT-03). Secrets go to the server once and are never shown again.
function AddConnector({ kinds, onDone }: { kinds: string[]; onDone: () => void }) {
  const { t } = useTranslation();
  const spaces = useSpaces();
  const [kind, setKind] = useState<(typeof PILOTS)[number]>("github");
  const [invalid, setInvalid] = useState(false);
  const available = PILOTS.filter((k) => kinds.includes(k));
  const create = useMutation({
    mutationFn: (body: object) => api("/api/v1/connectors", { method: "POST", body: JSON.stringify(body) }),
    onSuccess: onDone,
  });
  if (!available.length) return null;
  return <form className="space-y-3 rounded border p-3" aria-label={t("settings.apps.connectors.add")} onSubmit={(event) => {
    event.preventDefault();
    const f = new FormData(event.currentTarget);
    const text = (k: string) => String(f.get(k) ?? "").trim();
    const space = text("space_id");
    const refresh = [text("refresh_token"), text("client_id"), text("client_secret")];
    // Mirrors the server (gcal Validate): a token, or refresh token with client id and secret.
    const googleOk = !!text("token") || refresh.every(Boolean);
    setInvalid(kind === "google_calendar" && !googleOk);
    if (kind === "google_calendar" && !googleOk) return;
    const body = kind === "github"
      ? { kind, name: text("name"), config: { repo: text("repo"), ...(space && { space_id: space }), import: f.get("import") === "on", push_new: f.get("push_new") === "on" },
          secrets: { token: text("token"), ...(text("webhook_secret") && { webhook_secret: text("webhook_secret") }) } }
      : { kind, name: text("name"), config: { calendar_id: text("calendar_id"), ...(space && { space_id: space }) }, secrets: { ...(text("token") && { token: text("token") }), ...(refresh.every(Boolean) && { refresh_token: refresh[0], client_id: refresh[1], client_secret: refresh[2] }) } };
    create.mutate(body, { onSuccess: () => (event.target as HTMLFormElement | null)?.reset?.() });
  }}>
    <h3 className="font-medium">{t("settings.apps.connectors.add")}</h3>
    {create.error && <p role="alert">{create.error.message}</p>}
    <div className="flex flex-wrap items-end gap-3">
      <label className="block text-sm">{t("settings.apps.connectors.kind")}
        <select className="block rounded border bg-background p-2" value={kind} onChange={(e) => setKind(e.target.value as typeof kind)}>
          {available.map((k) => <option key={k} value={k}>{t(`settings.apps.connectors.kinds.${k}`)}</option>)}
        </select></label>
      <label className="block text-sm">{t("settings.apps.connectors.name")}<Input name="name" required maxLength={120} autoComplete="off" /></label>
      {kind === "github"
        ? <label className="block text-sm">{t("settings.apps.connectors.repo")}<Input name="repo" required autoComplete="off" placeholder={t("settings.apps.connectors.repoPlaceholder")} /></label>
        : <label className="block text-sm">{t("settings.apps.connectors.calendar")}<Input name="calendar_id" required autoComplete="off" /></label>}
      <label className="block text-sm">{t("settings.apps.connectors.space")}
        <select name="space_id" required className="block rounded border bg-background p-2" defaultValue="">
          <option value="" disabled>{t("settings.apps.connectors.chooseSpace")}</option>
          {spaces.data?.filter((s) => s.kind === "shared").map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
        </select></label>
      <label className="block text-sm">{t(kind === "github" ? "settings.apps.connectors.token" : "settings.apps.connectors.googleToken")}<Input name="token" type="password" autoComplete="new-password" required={kind === "github"} /></label>
      {kind === "github" && <label className="block text-sm">{t("settings.apps.connectors.webhookSecret")}<Input name="webhook_secret" type="password" autoComplete="new-password" /></label>}
    </div>
    {kind === "google_calendar" && <div className="flex flex-wrap items-end gap-3">
      <label className="block text-sm">{t("settings.apps.connectors.clientId")}<Input name="client_id" autoComplete="off" /></label>
      <label className="block text-sm">{t("settings.apps.connectors.clientSecret")}<Input name="client_secret" type="password" autoComplete="new-password" /></label>
      <label className="block text-sm">{t("settings.apps.connectors.refreshToken")}<Input name="refresh_token" type="password" autoComplete="new-password" /></label>
    </div>}
    {invalid && <p role="alert" className="text-sm">{t("settings.apps.connectors.googleInvalid")}</p>}
    {kind === "github" && <div className="flex flex-wrap gap-4 text-sm">
      <label className="flex items-center gap-2"><input type="checkbox" name="import" defaultChecked />{t("settings.apps.connectors.import")}</label>
      <label className="flex items-center gap-2"><input type="checkbox" name="push_new" />{t("settings.apps.connectors.pushNew")}</label>
    </div>}
    <p className="text-sm text-muted-foreground">{t(kind === "github" ? "settings.apps.connectors.githubHint" : "settings.apps.connectors.googleHint")}</p>
    {kind === "google_calendar" && <p className="text-sm text-muted-foreground">{t("settings.apps.connectors.googleRefreshHint")}</p>}
    <Button type="submit" disabled={create.isPending}>{t("settings.apps.connectors.install")}</Button>
  </form>;
}

function InstallList({ items, toggle, sync }: { items: Install[]; toggle: (c: Install) => void; sync: (id: string) => void }) {
  const { t, i18n } = useTranslation();
  return <ul className="space-y-4">
    {items.map((c) => <li key={c.id} className="rounded border p-3">
      <div className="flex flex-wrap items-center gap-3">
        <strong>{c.name}</strong><span className="text-sm text-muted-foreground">{t(`settings.apps.connectors.kinds.${c.kind}`, { defaultValue: c.kind })}</span>
        <span className="text-sm">{c.enabled ? t("settings.apps.connectors.enabled") : t("settings.apps.connectors.disabled")}</span>
        <Button type="button" variant="outline" onClick={() => toggle(c)}>{c.enabled ? t("settings.apps.connectors.disable") : t("settings.apps.connectors.enable")}</Button>
        {c.enabled && <Button type="button" variant="outline" onClick={() => sync(c.id)}>{t("settings.apps.connectors.sync")}</Button>}
      </div>
      <p className="mt-2 text-sm text-muted-foreground">{t(c.secret_names.includes("refresh_token") ? "settings.apps.connectors.authRefresh" : "settings.apps.connectors.authToken")}</p>
      <p className="mt-2 text-sm" role="status">
        {c.last_sync_at
          ? t(c.last_sync_ok ? "settings.apps.connectors.syncOk" : "settings.apps.connectors.syncFailed", { when: new Date(c.last_sync_at).toLocaleString(i18n.language), message: c.last_sync_message ?? "" })
          : t("settings.apps.connectors.neverSynced")}
      </p>
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
