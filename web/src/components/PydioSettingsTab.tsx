import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { components } from "@/api/schema";
import { api, ApiError } from "@/lib/api";
import { customerKeys, listCustomers, type Customer } from "@/lib/customers";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { AdminPageHeader, AdminStatus } from "@/components/admin/admin-ui";
import { PydioStorageSection } from "@/components/PydioStorageSection";

type Settings = components["schemas"]["PydioSettings"];
const KEY = ["pydio-settings"] as const;
const areaClass = "mt-1 block w-full rounded-lg border border-input bg-transparent px-2.5 py-1.5 font-mono text-sm focus-visible:outline-ring";

// One folder per line; blank lines and surrounding spaces are ignored. The server validates every path.
const folders = (text: string) => text.split("\n").map((line) => line.trim()).filter(Boolean);
// Extensions may be separated by commas, spaces or lines. The server lowercases them and strips the leading dot.
const extensions = (text: string) => text.split(/[\s,;]+/).filter(Boolean);

// Pydio tab of Apps & Integrations: where folder links may point, per instance and per Customer, then file storage.
// Folder links need no credential; storage has its own form and write-only token.
export function PydioSettingsTab() {
  const { t } = useTranslation();
  // Not refetched behind the administrator's back: a new version would replace what is being typed.
  const settings = useQuery({ queryKey: KEY, queryFn: () => api<Settings>("/api/v1/pydio/settings"), staleTime: Infinity, refetchOnWindowFocus: false });
  const customers = useQuery({ queryKey: customerKeys.list("active"), queryFn: () => listCustomers("active") });
  if (settings.isPending || customers.isPending) return <p role="status" className="text-sm text-muted-foreground">{t("apps.pydio.loading")}</p>;
  if (settings.isError || customers.isError) return <p role="alert">{(settings.error ?? customers.error)?.message}</p>;
  const data = settings.data;
  return <div className="max-w-3xl space-y-5">
    <AdminPageHeader title={t("apps.pydio.linksTitle")} description={t("apps.pydio.description")} />
    <div className="flex flex-wrap items-center gap-2">
      <AdminStatus tone={data.enabled ? "success" : "neutral"} label={t(data.enabled ? "apps.pydio.on" : "apps.pydio.off")} />
      <AdminStatus tone="neutral" label={t(`apps.pydio.source.${data.source}`)} />
      <AdminStatus tone="neutral" label={t(data.listing_enabled ? "apps.pydio.listingOn" : "apps.pydio.listingOff")} />
    </div>
    <p className="text-sm text-muted-foreground">{t(`apps.pydio.sourceHint.${data.source}`)} {t("apps.pydio.listingHint")}</p>
    <SettingsForm key={`${data.source}:${data.version}`} settings={data} customers={customers.data.items} reload={() => settings.refetch()} />
    <PydioStorageSection customers={customers.data.items} linkAddress={data.base_url} />
  </div>;
}

function SettingsForm({ settings, customers, reload }: { settings: Settings; customers: Customer[]; reload: () => Promise<unknown> }) {
  const { t } = useTranslation();
  const cache = useQueryClient();
  const [enabled, setEnabled] = useState(settings.source === "none" || settings.enabled);
  const [baseUrl, setBaseUrl] = useState(settings.base_url);
  const [roots, setRoots] = useState(settings.roots.join("\n"));
  const [rows, setRows] = useState(() => Object.entries(settings.customer_roots).sort(([a], [b]) => a.localeCompare(b))
    .map(([code, list]) => ({ code, folders: list.join("\n") })));
  const [maxUpload, setMaxUpload] = useState(String(settings.max_upload_mb));
  const [types, setTypes] = useState(settings.allowed_extensions.join(", "));
  const [error, setError] = useState("");
  const [conflict, setConflict] = useState(false);
  const [busy, setBusy] = useState(false);
  const [removing, setRemoving] = useState(false);
  const unused = customers.filter((c) => !rows.some((row) => row.code === c.code));

  async function save(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    setBusy(true); setError(""); setConflict(false);
    try {
      const saved = await api<Settings>("/api/v1/pydio/settings", { method: "PUT", body: JSON.stringify({
        version: settings.version, enabled, base_url: baseUrl.trim(), roots: folders(roots),
        customer_roots: Object.fromEntries(rows.map((row) => [row.code, folders(row.folders)])),
        max_upload_mb: Number(maxUpload), allowed_extensions: extensions(types) }) });
      toast.success(t("apps.pydio.saved"));
      cache.setQueryData(KEY, saved);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) setConflict(true);
      else setError(err instanceof ApiError ? err.message : t("apps.pydio.saveFailed"));
    } finally { setBusy(false); }
  }

  async function remove() {
    setBusy(true); setError("");
    try {
      await api("/api/v1/pydio/settings", { method: "DELETE" });
      toast.success(t("apps.pydio.removed"));
      setRemoving(false);
      await reload();
    } catch (err) {
      setRemoving(false);
      setError(err instanceof ApiError ? err.message : t("apps.pydio.saveFailed"));
    } finally { setBusy(false); }
  }

  return <>
    <form className="space-y-5" onSubmit={(e) => void save(e)} aria-label={t("apps.pydio.formLabel")}>
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="size-6" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />{t("apps.pydio.enabled")}</label>
      <label className="block text-sm">{t("apps.pydio.address")}
        <Input type="url" required maxLength={500} value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder={t("apps.pydio.addressExample")} className="mt-1" /></label>
      <div>
        <label className="block text-sm">{t("apps.pydio.roots")}
          <textarea required rows={3} spellCheck={false} value={roots} onChange={(e) => setRoots(e.target.value)} className={areaClass} /></label>
        <p className="mt-1 text-xs text-muted-foreground">{t("apps.pydio.rootsHint")}</p>
      </div>
      <fieldset className="space-y-3">
        <legend className="text-sm font-medium">{t("apps.pydio.customers")}</legend>
        <p className="text-xs text-muted-foreground">{t("apps.pydio.customersHint")}</p>
        {rows.map((row) => <div key={row.code} className="space-y-1 rounded-lg border p-3">
          <div className="flex items-center justify-between gap-2">
            <p className="min-w-0 break-words text-sm font-medium">{row.code}
              <span className="ms-2 font-normal text-muted-foreground">{customers.find((c) => c.code === row.code)?.name}</span></p>
            <Button type="button" variant="ghost" size="sm" aria-label={t("apps.pydio.removeCustomer", { code: row.code })}
              onClick={() => setRows(rows.filter((r) => r.code !== row.code))}>{t("apps.pydio.remove")}</Button>
          </div>
          <textarea required rows={2} spellCheck={false} aria-label={t("apps.pydio.foldersFor", { code: row.code })} value={row.folders}
            onChange={(e) => setRows(rows.map((r) => r.code === row.code ? { ...r, folders: e.target.value } : r))} className={areaClass} />
        </div>)}
        <label className="block text-sm">{t("apps.pydio.addCustomer")}
          <select value="" disabled={unused.length === 0} onChange={(e) => e.target.value && setRows([...rows, { code: e.target.value, folders: "" }])}
            className="mt-1 block h-9 w-full max-w-md rounded-md border border-input bg-background px-2 focus-visible:outline-ring disabled:opacity-50">
            <option value="">{t(unused.length ? "apps.pydio.chooseCustomer" : "apps.pydio.noCustomer")}</option>
            {unused.map((c) => <option key={c.id} value={c.code}>{c.code} · {c.name}</option>)}
          </select></label>
      </fieldset>
      <fieldset className="space-y-3">
        <legend className="text-sm font-medium">{t("apps.pydio.uploads")}</legend>
        <p className="text-xs text-muted-foreground">{t("apps.pydio.uploadsHint")}</p>
        <label className="block text-sm">{t("apps.pydio.maxUpload")}
          <Input type="number" required min={1} max={1024} step={1} value={maxUpload} onChange={(e) => setMaxUpload(e.target.value)} className="mt-1 max-w-40" /></label>
        <div>
          <label className="block text-sm">{t("apps.pydio.types")}
            <textarea rows={2} spellCheck={false} value={types} onChange={(e) => setTypes(e.target.value)} placeholder={t("apps.pydio.typesExample")} className={areaClass} /></label>
          <p className="mt-1 text-xs text-muted-foreground">{t("apps.pydio.typesHint")}</p>
        </div>
      </fieldset>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      {conflict && <div role="alert" className="flex flex-wrap items-center gap-2 text-sm text-destructive">{t("apps.pydio.conflict")}
        <Button type="button" variant="outline" size="sm" onClick={() => void reload()}>{t("apps.pydio.reload")}</Button></div>}
      <div className="flex flex-wrap gap-2">
        <Button type="submit" disabled={busy}>{t("apps.pydio.save")}</Button>
        {settings.source === "database" && <Button type="button" variant="outline" disabled={busy} onClick={() => setRemoving(true)}>{t("apps.pydio.useEnvironment")}</Button>}
      </div>
    </form>
    <ConfirmDialog open={removing} onOpenChange={setRemoving} busy={busy} title={t("apps.pydio.removeTitle")} description={t("apps.pydio.removeDescription")}
      confirmLabel={t("apps.pydio.removeConfirm")} cancelLabel={t("apps.pydio.cancel")} onConfirm={() => void remove()} />
  </>;
}
