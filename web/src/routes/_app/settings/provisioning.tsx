import { useState, type FormEvent } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { formatDateTime, useProfile } from "@/lib/profile";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type ScimToken = components["schemas"]["ScimToken"];
type TokenList = components["schemas"]["ScimTokenList"];

export const Route = createFileRoute("/_app/settings/provisioning")({ component: ProvisioningPage });

const TYPES = ["regular", "external", "collaborator", "viewer"] as const;
const KEY = ["scim-tokens"] as const;
const select = "mt-1 block h-9 rounded-md border border-input bg-background px-2";

function ProvisioningPage() {
  const { t } = useTranslation();
  const profile = useProfile();
  const admin = profile?.user_type === "admin";
  const qc = useQueryClient();
  const tokens = useQuery({ queryKey: KEY, enabled: admin, retry: false, queryFn: () => api<TokenList>("/api/v1/scim-tokens") });
  const [creating, setCreating] = useState(false);
  const [fresh, setFresh] = useState<ScimToken>();
  const [revoking, setRevoking] = useState<ScimToken>();
  const [error, setError] = useState("");
  if (profile && !admin) return <p role="alert">{t("provisioning.adminOnly")}</p>;

  async function revoke() {
    if (!revoking) return;
    setError("");
    try {
      await api(`/api/v1/scim-tokens/${revoking.id}`, { method: "DELETE" });
      setRevoking(undefined);
      toast.success(t("provisioning.revoked.toast"));
      await qc.invalidateQueries({ queryKey: KEY });
    } catch (err) { setError((err as Error).message); }
  }

  const list = tokens.data?.items ?? [];
  return <section className="space-y-5">
    <header className="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 className="text-lg font-semibold">{t("provisioning.title")}</h2>
        <p className="text-sm text-muted-foreground">{t("provisioning.description")}</p>
      </div>
      <Button onClick={() => { setFresh(undefined); setCreating(true); }}>{t("provisioning.new")}</Button>
    </header>
    {tokens.data && <Copyable label={t("provisioning.baseUrl")} hint={t("provisioning.baseUrlHint")} value={tokens.data.base_url} />}
    {fresh?.token && <div role="alert" className="space-y-3 rounded-lg border border-amber-500 p-4">
      <h3 className="font-semibold">{t("provisioning.tokenTitle")}</h3>
      <p className="text-sm">{t("provisioning.tokenWarning")}</p>
      <Copyable label={t("provisioning.tokenLabel", { name: fresh.name })} value={fresh.token} />
      <Button variant="outline" onClick={() => setFresh(undefined)}>{t("provisioning.done")}</Button>
    </div>}
    {creating && <TokenForm onClose={() => setCreating(false)} onCreated={(token) => { setCreating(false); setFresh(token); void qc.invalidateQueries({ queryKey: KEY }); }} />}
    {tokens.isPending && <p role="status" className="text-sm text-muted-foreground">{t("provisioning.loading")}</p>}
    {tokens.isError && <p role="alert">{t("provisioning.error")}</p>}
    {tokens.isSuccess && list.length === 0 && <p role="status" className="rounded-xl border border-dashed p-8 text-center text-sm text-muted-foreground">{t("provisioning.empty")}</p>}
    {list.length > 0 && <table className="w-full text-left text-sm">
      <thead><tr className="border-b">
        {(["name", "type", "prefix", "lastUsed", "status"] as const).map((k) => <th key={k} scope="col" className="py-2 pr-4">{t(`provisioning.col.${k}`)}</th>)}
        <th scope="col" className="py-2"><span className="sr-only">{t("provisioning.col.actions")}</span></th>
      </tr></thead>
      <tbody>{list.map((token) => <tr key={token.id} className="border-b align-top">
        <th scope="row" className="py-2 pr-4 font-medium">{token.name}</th>
        <td className="py-2 pr-4">{t(`provisioning.types.${token.default_user_type}`, { defaultValue: token.default_user_type })}</td>
        <td className="py-2 pr-4 font-mono text-xs">{token.prefix}…</td>
        <td className="py-2 pr-4">{token.last_used_at ? formatDateTime(token.last_used_at, profile) : t("provisioning.never")}</td>
        <td className="py-2 pr-4">{token.revoked_at ? t("provisioning.revoked") : t("provisioning.active")}</td>
        <td className="py-2 text-right">{!token.revoked_at && <Button size="sm" variant="outline" aria-label={t("provisioning.revokeNamed", { name: token.name })}
          onClick={() => { setError(""); setRevoking(token); }}>{t("provisioning.revoke")}</Button>}</td>
      </tr>)}</tbody>
    </table>}
    <ConfirmDialog open={!!revoking} onOpenChange={(open) => !open && setRevoking(undefined)} error={error}
      title={t("provisioning.revokeTitle")} description={t("provisioning.revokeDescription", { name: revoking?.name ?? "" })}
      confirmLabel={t("provisioning.revokeConfirm")} cancelLabel={t("provisioning.cancel")} onConfirm={() => void revoke()} />
  </section>;
}

function Copyable({ label, hint, value }: { label: string; hint?: string; value: string }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  return <div className="text-sm">
    <label className="block font-medium">{label}
      <div className="mt-1 flex max-w-2xl gap-2">
        <Input readOnly value={value} className="font-mono text-xs" onFocus={(e) => e.currentTarget.select()} />
        <Button type="button" variant="outline" onClick={() => { void navigator.clipboard?.writeText(value).then(() => setCopied(true)); }}>{copied ? t("provisioning.copied") : t("provisioning.copy")}</Button>
      </div></label>
    {hint && <p className="mt-1 text-muted-foreground">{hint}</p>}
  </div>;
}

function TokenForm({ onClose, onCreated }: { onClose: () => void; onCreated: (token: ScimToken) => void }) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [type, setType] = useState<(typeof TYPES)[number]>("regular");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true); setError("");
    try {
      onCreated(await api<ScimToken>("/api/v1/scim-tokens", { method: "POST", body: JSON.stringify({ name, default_user_type: type }) }));
    } catch (err) { setError((err as Error).message || t("provisioning.createFailed")); } finally { setBusy(false); }
  }
  return <form onSubmit={(e) => void submit(e)} className="space-y-4 rounded-lg border p-4" aria-label={t("provisioning.newTitle")}>
    <h3 className="font-semibold">{t("provisioning.newTitle")}</h3>
    <label className="block text-sm">{t("provisioning.name")}<Input required maxLength={80} placeholder={t("provisioning.nameHint")} value={name} onChange={(e) => setName(e.target.value)} className="mt-1 max-w-sm" /></label>
    <label className="block text-sm">{t("provisioning.defaultType")}
      <select className={select} value={type} onChange={(e) => setType(e.target.value as (typeof TYPES)[number])}>
        {TYPES.map((v) => <option key={v} value={v}>{t(`provisioning.types.${v}`)}</option>)}
      </select></label>
    <p className="text-sm text-muted-foreground">{t("provisioning.defaultTypeHint")}</p>
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    <div className="flex gap-2">
      <Button type="submit" disabled={busy || !name.trim()}>{t("provisioning.create")}</Button>
      <Button type="button" variant="outline" onClick={onClose}>{t("provisioning.cancel")}</Button>
    </div>
  </form>;
}
