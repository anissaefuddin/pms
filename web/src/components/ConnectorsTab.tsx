import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { useSpaces } from "@/lib/space-queries";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { AdminEmptyState, AdminMetrics, AdminPageHeader, AdminStatus } from "@/components/admin/admin-ui";

type Install = { id: string; kind: string; name: string; enabled: boolean; secret_names: string[]; config: Record<string, unknown>; last_sync_at: string | null; last_sync_ok: boolean | null; last_sync_message: string | null };
type Conflict = { id: string; item_title: string; remote_title: string; external_id: string; differs?: string };
const PILOTS = ["github", "google_calendar"] as const;
const TONE_FAILED = "danger", TONE_ON = "success", TONE_OFF = "neutral";

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
  const [adding, setAdding] = useState(false);
  if (list.error) return <p role="alert">{list.error.message}</p>;
  const items = list.data?.items ?? [];
  const enabled = items.filter((c) => c.enabled).length;
  return <div className="space-y-6">
    <AdminPageHeader description={t("settings.apps.connectors.description")} action={<Button onClick={() => setAdding(true)}>{t("settings.apps.connectors.addNew")}</Button>} />
    {list.data && <AdminMetrics items={[
      { label: t("settings.apps.connectors.installed"), value: items.length },
      { label: t("settings.apps.connectors.enabledCount"), value: enabled },
      { label: t("settings.apps.connectors.disabledCount"), value: items.length - enabled },
    ]} />}
    {list.data && !items.length && <AdminEmptyState title={t("settings.apps.connectors.empty")} description={t("settings.apps.connectors.emptyDescription")} />}
    <InstallList items={items} toggle={(c) => toggle.mutate(c)} sync={(id) => sync.mutate(id)} />
    <Dialog open={adding} onOpenChange={setAdding}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("settings.apps.connectors.addNew")}</DialogTitle>
          <DialogDescription>{t("settings.apps.connectors.addDescription")}</DialogDescription>
        </DialogHeader>
        <AddConnector kinds={list.data?.kinds ?? []} onDone={() => { refresh(); setAdding(false); }} />
      </DialogContent>
    </Dialog>
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
  });
  if (!available.length) return null;
  return <form className="space-y-3" aria-label={t("settings.apps.connectors.add")} onSubmit={(event) => {
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
    create.mutate(body, { onSuccess: () => { (event.target as HTMLFormElement | null)?.reset?.(); onDone(); } });
  }}>
    {create.error && <p role="alert" className="text-sm text-destructive">{create.error.message}</p>}
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
  return <ul className="grid gap-4 lg:grid-cols-2">
    {items.map((c) => {
      // Status is derived from what the server reports: a failed last sync wins over enabled or disabled.
      const tone = c.last_sync_ok === false ? TONE_FAILED : c.enabled ? TONE_ON : TONE_OFF;
      const label = c.last_sync_ok === false ? t("settings.apps.connectors.syncFailedBadge") : c.enabled ? t("settings.apps.connectors.enabled") : t("settings.apps.connectors.disabled");
      const target = typeof c.config.repo === "string" ? c.config.repo : typeof c.config.calendar_id === "string" ? c.config.calendar_id : "";
      return <li key={c.id} className="space-y-4 rounded-lg border p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="font-medium">{c.name}</p>
            <p className="text-sm text-muted-foreground">{t(`settings.apps.connectors.kinds.${c.kind}`, { defaultValue: c.kind })}{target && ` · ${target}`}</p>
          </div>
          <AdminStatus tone={tone} label={label} />
        </div>
        {c.last_sync_ok === false && c.last_sync_message && <p role="alert" className="rounded-md bg-red-500/10 px-3 py-2 text-sm">{c.last_sync_message}</p>}
        <p className="text-xs text-muted-foreground" role="status">
          {c.last_sync_at
            ? t(c.last_sync_ok ? "settings.apps.connectors.syncOk" : "settings.apps.connectors.syncFailed", { when: new Date(c.last_sync_at).toLocaleString(i18n.language), message: c.last_sync_message ?? "" })
            : t("settings.apps.connectors.neverSynced")}
        </p>
        <p className="text-xs text-muted-foreground">{t(c.secret_names.includes("refresh_token") ? "settings.apps.connectors.authRefresh" : "settings.apps.connectors.authToken")}</p>
        <div className="flex flex-wrap gap-2">
          <Button type="button" variant="outline" size="sm" onClick={() => toggle(c)}>{c.enabled ? t("settings.apps.connectors.disable") : t("settings.apps.connectors.enable")}</Button>
          {c.enabled && <Button type="button" variant="outline" size="sm" onClick={() => sync(c.id)}>{t("settings.apps.connectors.sync")}</Button>}
        </div>
        <Conflicts id={c.id} />
      </li>;
    })}
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
