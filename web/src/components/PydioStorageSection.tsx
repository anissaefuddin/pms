import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { components } from "@/api/schema";
import { api, ApiError } from "@/lib/api";
import type { Customer } from "@/lib/customers";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { AdminStatus } from "@/components/admin/admin-ui";

type Storage = components["schemas"]["PydioStorage"];
type Field = { id: string; name: string; type: string; archived_at?: string | null };
const KEY = ["pydio-storage"] as const;
const selectClass = "mt-1 block h-9 w-full max-w-md rounded-md border border-input bg-background px-2 focus-visible:outline-ring disabled:opacity-50";

// File storage in Pydio, saved apart from the folder links. The token is write only: the server never sends it back.
export function PydioStorageSection({ customers, linkAddress }: { customers: Customer[]; linkAddress: string }) {
  const { t } = useTranslation();
  // Not refetched behind the administrator's back: a new version would replace what is being typed.
  const storage = useQuery({ queryKey: KEY, queryFn: () => api<Storage>("/api/v1/pydio/storage"), staleTime: Infinity, refetchOnWindowFocus: false });
  const fields = useQuery({ queryKey: ["custom-fields", "workspace"], queryFn: () => api<{ items: Field[] }>("/api/v1/custom-fields") });
  const data = storage.data;
  return <section aria-labelledby="pydio-storage-title" className="space-y-4 border-t pt-6">
    <div className="space-y-1">
      <h3 id="pydio-storage-title" className="text-base font-semibold">{t("apps.pydio.storage.title")}</h3>
      <p className="text-sm text-muted-foreground">{t("apps.pydio.storage.description")}</p>
    </div>
    {(storage.isPending || fields.isPending) && <p role="status" className="text-sm text-muted-foreground">{t("apps.pydio.loading")}</p>}
    {(storage.isError || fields.isError) && <p role="alert" className="text-sm text-destructive">{(storage.error ?? fields.error)?.message}</p>}
    {data && fields.data && <>
      <div className="flex flex-wrap items-center gap-2">
        <AdminStatus tone={data.enabled ? "success" : "neutral"} label={t(data.enabled ? "apps.pydio.storage.on" : "apps.pydio.storage.off")} />
        <AdminStatus tone="neutral" label={t(data.token_set ? "apps.pydio.storage.tokenSet" : "apps.pydio.storage.tokenMissing")} />
      </div>
      <StorageForm key={data.version} storage={data} customers={customers} linkAddress={linkAddress}
        codeFields={fields.data.items.filter((f) => f.type === "text" && !f.archived_at)} reload={() => storage.refetch()} />
    </>}
  </section>;
}

function StorageForm({ storage, customers, linkAddress, codeFields, reload }: { storage: Storage; customers: Customer[]; linkAddress: string; codeFields: Field[]; reload: () => Promise<unknown> }) {
  const { t } = useTranslation();
  const cache = useQueryClient();
  const [enabled, setEnabled] = useState(storage.enabled);
  const [baseUrl, setBaseUrl] = useState(storage.base_url || linkAddress);
  const [root, setRoot] = useState(storage.root);
  const [token, setToken] = useState("");
  const [codeField, setCodeField] = useState(storage.code_field_id ?? "");
  const [rows, setRows] = useState(() => Object.entries(storage.customer_roots).sort(([a], [b]) => a.localeCompare(b)).map(([code, folder]) => ({ code, folder })));
  const [error, setError] = useState("");
  const [conflict, setConflict] = useState(false);
  const [busy, setBusy] = useState(false);
  const unused = customers.filter((c) => !rows.some((row) => row.code === c.code));
  const moved = storage.token_set && baseUrl.trim().replace(/\/+$/, "") !== storage.base_url;
  const failure = (err: unknown) => !(err instanceof ApiError) ? t("apps.pydio.saveFailed")
    : err.code === "pydio_storage_check_failed" ? t("apps.pydio.storage.checkFailed") : err.message;

  async function save(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    setBusy(true); setError(""); setConflict(false);
    try {
      const saved = await api<Storage>("/api/v1/pydio/storage", { method: "PUT", body: JSON.stringify({
        version: storage.version, enabled, base_url: baseUrl.trim(), root: root.trim(),
        customer_roots: Object.fromEntries(rows.map((row) => [row.code, row.folder.trim()])), code_field_id: codeField || null,
        ...(token.trim() ? { token: token.trim() } : {}) }) });
      toast.success(t("apps.pydio.storage.saved"));
      cache.setQueryData(KEY, saved);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) setConflict(true);
      else setError(failure(err));
    } finally { setBusy(false); }
  }

  async function test() {
    setBusy(true); setError("");
    try {
      await api("/api/v1/pydio/storage/test", { method: "POST" });
      toast.success(t("apps.pydio.storage.tested"));
    } catch (err) { setError(failure(err)); }
    finally { setBusy(false); }
  }

  return <form className="space-y-4" onSubmit={(e) => void save(e)} aria-label={t("apps.pydio.storage.formLabel")}>
    {!storage.key_configured && <p role="status" className="rounded border border-warning bg-warning-subtle p-3 text-sm">{t("apps.pydio.storage.noKey")}</p>}
    <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="size-6" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />{t("apps.pydio.storage.enabled")}</label>
    <p className="-mt-2 text-xs text-muted-foreground">{t("apps.pydio.storage.enabledHint")}</p>
    <div>
      <label className="block text-sm">{t("apps.pydio.storage.address")}
        <Input type="url" required maxLength={500} value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder={t("apps.pydio.addressExample")} className="mt-1" /></label>
      {moved && <p className="mt-1 text-xs text-destructive">{t("apps.pydio.storage.moved")}</p>}
    </div>
    <div>
      <label className="block text-sm">{t("apps.pydio.storage.root")}
        <Input required maxLength={400} spellCheck={false} value={root} onChange={(e) => setRoot(e.target.value)} placeholder={t("apps.pydio.storage.rootExample")} className="mt-1 font-mono" /></label>
      <p className="mt-1 text-xs text-muted-foreground">{t("apps.pydio.storage.rootHint")}</p>
    </div>
    <div>
      <label className="block text-sm">{t("apps.pydio.storage.token")}
        <Input type="password" autoComplete="new-password" maxLength={4000} required={!storage.token_set || moved} value={token} onChange={(e) => setToken(e.target.value)}
          placeholder={storage.token_set && !moved ? t("apps.pydio.storage.tokenKept") : ""} className="mt-1 max-w-md" /></label>
      <p className="mt-1 text-xs text-muted-foreground">{t("apps.pydio.storage.tokenHint")}</p>
    </div>
    <div>
      <label className="block text-sm">{t("apps.pydio.storage.codeField")}
        <select value={codeField} onChange={(e) => setCodeField(e.target.value)} className={selectClass}>
          <option value="">{t("apps.pydio.storage.codeNone")}</option>
          {codeFields.map((f) => <option key={f.id} value={f.id}>{f.name}</option>)}
        </select></label>
      <p className="mt-1 text-xs text-muted-foreground">{t("apps.pydio.storage.codeHint")}</p>
    </div>
    <fieldset className="space-y-3">
      <legend className="text-sm font-medium">{t("apps.pydio.storage.customers")}</legend>
      <p className="text-xs text-muted-foreground">{t("apps.pydio.storage.customersHint")}</p>
      {rows.map((row) => <div key={row.code} className="flex flex-wrap items-end gap-2">
        <label className="block min-w-0 flex-1 text-sm">{t("apps.pydio.storage.folderFor", { code: row.code })}
          <Input required maxLength={400} spellCheck={false} value={row.folder} className="mt-1 font-mono"
            onChange={(e) => setRows(rows.map((r) => r.code === row.code ? { ...r, folder: e.target.value } : r))} /></label>
        <Button type="button" variant="ghost" size="sm" aria-label={t("apps.pydio.storage.removeCustomer", { code: row.code })}
          onClick={() => setRows(rows.filter((r) => r.code !== row.code))}>{t("apps.pydio.remove")}</Button>
      </div>)}
      <label className="block text-sm">{t("apps.pydio.storage.addCustomer")}
        <select value="" disabled={unused.length === 0} onChange={(e) => e.target.value && setRows([...rows, { code: e.target.value, folder: "" }])} className={selectClass}>
          <option value="">{t(unused.length ? "apps.pydio.chooseCustomer" : "apps.pydio.noCustomer")}</option>
          {unused.map((c) => <option key={c.id} value={c.code}>{c.code} · {c.name}</option>)}
        </select></label>
    </fieldset>
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    {conflict && <div role="alert" className="flex flex-wrap items-center gap-2 text-sm text-destructive">{t("apps.pydio.conflict")}
      <Button type="button" variant="outline" size="sm" onClick={() => void reload()}>{t("apps.pydio.reload")}</Button></div>}
    <div className="flex flex-wrap items-center gap-2">
      <Button type="submit" disabled={busy || !storage.key_configured}>{t("apps.pydio.storage.save")}</Button>
      {storage.version > 0 && <Button type="button" variant="outline" disabled={busy} onClick={() => void test()}>{t("apps.pydio.storage.test")}</Button>}
      {busy && <span role="status" className="text-sm text-muted-foreground">{t("apps.pydio.storage.checking")}</span>}
    </div>
    {storage.version > 0 && <p className="text-xs text-muted-foreground">{t("apps.pydio.storage.testHint")}</p>}
  </form>;
}
