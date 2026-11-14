import { useState, type FormEvent } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/settings/sso")({ component: SsoPage });

type Config = {
  configured: boolean; name: string; enabled: boolean; issuer: string; client_id: string; secret_set: boolean; auto_link: boolean;
  allowed_domains: string[]; redirect_url: string; key_configured: boolean;
};

function SsoPage() {
  const { t } = useTranslation();
  const profile = useProfile();
  const admin = profile?.user_type === "admin";
  const config = useQuery({ queryKey: ["sso-config"], enabled: admin, retry: false, queryFn: () => api<Config>("/api/v1/sso/oidc") });
  if (profile && !admin) return <p role="alert">{t("sso.admin.adminOnly")}</p>;
  if (config.isPending) return <p role="status" className="text-sm text-muted-foreground">{t("sso.admin.loading")}</p>;
  if (config.isError) return <p role="alert">{t("sso.admin.error")}</p>;
  return <OidcForm key={`${config.data.configured}:${config.data.enabled}:${config.data.issuer}`} config={config.data} />;
}

function OidcForm({ config }: { config: Config }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [name, setName] = useState(config.configured ? config.name : "Company login");
  const [issuer, setIssuer] = useState(config.issuer);
  const [clientId, setClientId] = useState(config.client_id);
  const [secret, setSecret] = useState("");
  const [autoLink, setAutoLink] = useState(config.auto_link);
  const [domains, setDomains] = useState(config.allowed_domains.join(", "));
  const [enabled, setEnabled] = useState(config.enabled);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [removing, setRemoving] = useState(false);

  async function save(event: FormEvent) {
    event.preventDefault();
    setBusy(true); setError("");
    try {
      await api("/api/v1/sso/oidc", { method: "PUT", body: JSON.stringify({
        name, enabled, issuer, client_id: clientId, auto_link: autoLink,
        allowed_domains: domains.split(",").map((d) => d.trim()).filter(Boolean), ...(secret ? { client_secret: secret } : {}) }) });
      toast.success(t("sso.admin.saved"));
      setSecret("");
      await Promise.all([qc.invalidateQueries({ queryKey: ["sso-config"] }), qc.invalidateQueries({ queryKey: ["sso-providers"] })]);
    } catch (err) { setError(err instanceof ApiError ? err.message : t("sso.admin.saveFailed")); }
    finally { setBusy(false); }
  }

  async function remove() {
    try {
      await api("/api/v1/sso/oidc", { method: "DELETE" });
      setRemoving(false);
      toast.success(t("sso.admin.removed"));
      await Promise.all([qc.invalidateQueries({ queryKey: ["sso-config"] }), qc.invalidateQueries({ queryKey: ["sso-providers"] })]);
    } catch { setError(t("sso.admin.saveFailed")); setRemoving(false); }
  }

  return <section className="max-w-2xl space-y-5">
    <header>
      <h2 className="text-lg font-semibold">{t("sso.admin.title")}</h2>
      <p className="text-sm text-muted-foreground">{t("sso.admin.description")}</p>
    </header>
    {!config.key_configured && <p role="alert" className="rounded border border-warning bg-warning-subtle p-3 text-sm">{t("sso.admin.noKey")}</p>}
    <div className="space-y-1 text-sm">
      <p className="font-medium">{t("sso.admin.redirect")}</p>
      <code className="block break-all rounded bg-muted p-2 text-xs">{config.redirect_url}</code>
      <p className="text-muted-foreground">{t("sso.admin.redirectHint")}</p>
    </div>
    <form className="space-y-4" onSubmit={(e) => void save(e)} aria-label={t("sso.admin.formLabel")}>
      <label className="block text-sm">{t("sso.admin.name")}<Input required maxLength={60} value={name} onChange={(e) => setName(e.target.value)} className="mt-1 max-w-sm" /></label>
      <label className="block text-sm">{t("sso.admin.issuer")}<Input required type="url" value={issuer} onChange={(e) => setIssuer(e.target.value)} className="mt-1" placeholder={t("sso.admin.issuerExample")} /></label>
      <label className="block text-sm">{t("sso.admin.clientId")}<Input required maxLength={255} value={clientId} onChange={(e) => setClientId(e.target.value)} className="mt-1 max-w-md" /></label>
      <label className="block text-sm">{t("sso.admin.secret")}
        <Input type="password" autoComplete="new-password" maxLength={1000} value={secret} onChange={(e) => setSecret(e.target.value)} className="mt-1 max-w-md"
          placeholder={config.secret_set ? t("sso.admin.secretKept") : ""} /></label>
      <label className="block text-sm">{t("sso.admin.domains")}<Input value={domains} onChange={(e) => setDomains(e.target.value)} className="mt-1 max-w-md" placeholder={t("sso.admin.domainsExample")} />
        <span className="block text-xs text-muted-foreground">{t("sso.admin.domainsHint")}</span></label>
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="size-6" checked={autoLink} onChange={(e) => setAutoLink(e.target.checked)} />{t("sso.admin.autoLink")}</label>
      <p className="-mt-2 text-xs text-muted-foreground">{t("sso.admin.autoLinkHint")}</p>
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="size-6" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />{t("sso.admin.enabled")}</label>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      <div className="flex gap-2">
        <Button type="submit" disabled={busy}>{t("sso.admin.save")}</Button>
        {config.configured && <Button type="button" variant="outline" onClick={() => setRemoving(true)}>{t("sso.admin.remove")}</Button>}
      </div>
    </form>
    <ConfirmDialog open={removing} onOpenChange={setRemoving} title={t("sso.admin.removeTitle")} description={t("sso.admin.removeDescription")}
      confirmLabel={t("sso.admin.removeConfirm")} cancelLabel={t("sso.admin.cancel")} onConfirm={() => void remove()} />
  </section>;
}
