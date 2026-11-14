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

type SamlConfig = Config & { entity_id?: string; acs_url?: string; metadata_url?: string; idp_entity_id?: string; idp_sign_on_url?: string; email_attribute?: string; has_metadata: boolean };

function SsoPage() {
  const { t } = useTranslation();
  const profile = useProfile();
  const admin = profile?.user_type === "admin";
  if (profile && !admin) return <p role="alert">{t("sso.admin.adminOnly")}</p>;
  return <div className="space-y-12">
    <Provider kind="oidc" />
    <Provider kind="saml" />
  </div>;
}

function Provider({ kind }: { kind: "oidc" | "saml" }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const config = useQuery({ queryKey: ["sso-config", kind], enabled: profile?.user_type === "admin", retry: false, queryFn: () => api<SamlConfig>(`/api/v1/sso/${kind}`) });
  if (config.isPending) return <p role="status" className="text-sm text-muted-foreground">{t("sso.admin.loading")}</p>;
  if (config.isError) return <p role="alert">{t("sso.admin.error")}</p>;
  const key = `${config.data.configured}:${config.data.enabled}:${config.data.issuer}:${config.data.idp_entity_id}`;
  return kind === "oidc" ? <OidcForm key={key} config={config.data} /> : <SamlForm key={key} config={config.data} />;
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
      await Promise.all([qc.invalidateQueries({ queryKey: ["sso-config", "oidc"] }), qc.invalidateQueries({ queryKey: ["sso-providers"] })]);
    } catch (err) { setError(err instanceof ApiError ? err.message : t("sso.admin.saveFailed")); }
    finally { setBusy(false); }
  }

  async function remove() {
    try {
      await api("/api/v1/sso/oidc", { method: "DELETE" });
      setRemoving(false);
      toast.success(t("sso.admin.removed"));
      await Promise.all([qc.invalidateQueries({ queryKey: ["sso-config", "oidc"] }), qc.invalidateQueries({ queryKey: ["sso-providers"] })]);
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

function SamlForm({ config }: { config: SamlConfig }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [name, setName] = useState(config.configured ? config.name : "Company SAML");
  const [metadata, setMetadata] = useState("");
  const [attribute, setAttribute] = useState(config.email_attribute ?? "");
  const [domains, setDomains] = useState(config.allowed_domains.join(", "));
  const [autoLink, setAutoLink] = useState(config.auto_link);
  const [enabled, setEnabled] = useState(config.enabled);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [removing, setRemoving] = useState(false);
  const refresh = () => Promise.all([qc.invalidateQueries({ queryKey: ["sso-config", "saml"] }), qc.invalidateQueries({ queryKey: ["sso-providers"] })]);

  async function save(event: FormEvent) {
    event.preventDefault();
    setBusy(true); setError("");
    try {
      // The stored metadata is never sent back; leaving the box empty keeps it.
      await api("/api/v1/sso/saml", { method: "PUT", body: JSON.stringify({ name, enabled, idp_metadata_xml: metadata, email_attribute: attribute, auto_link: autoLink,
        allowed_domains: domains.split(",").map((d) => d.trim()).filter(Boolean) }) });
      toast.success(t("sso.admin.saved"));
      setMetadata("");
      await refresh();
    } catch (err) { setError(err instanceof ApiError ? err.message : t("sso.admin.saveFailed")); }
    finally { setBusy(false); }
  }
  async function remove() {
    try {
      await api("/api/v1/sso/saml", { method: "DELETE" });
      setRemoving(false);
      toast.success(t("sso.admin.removed"));
      await refresh();
    } catch { setError(t("sso.admin.saveFailed")); setRemoving(false); }
  }

  return <section className="max-w-2xl space-y-5" aria-labelledby="saml-title">
    <header>
      <h2 id="saml-title" className="text-lg font-semibold">{t("sso.saml.title")}</h2>
      <p className="text-sm text-muted-foreground">{t("sso.saml.description")}</p>
    </header>
    <dl className="space-y-3 text-sm">
      <div><dt className="font-medium">{t("sso.saml.entityId")}</dt><dd><code className="block break-all rounded bg-muted p-2 text-xs">{config.entity_id}</code></dd></div>
      <div><dt className="font-medium">{t("sso.saml.acsUrl")}</dt><dd><code className="block break-all rounded bg-muted p-2 text-xs">{config.acs_url}</code></dd></div>
      <div><dt className="font-medium">{t("sso.saml.metadataUrl")}</dt><dd><a className="break-all underline" href={config.metadata_url}>{config.metadata_url}</a></dd></div>
    </dl>
    <p className="text-sm text-muted-foreground">{t("sso.saml.hint")}</p>
    {config.configured && <p className="text-sm">{config.has_metadata ? t("sso.saml.current", { entity: config.idp_entity_id }) : t("sso.saml.noMetadata")}</p>}
    <form className="space-y-4" onSubmit={(e) => void save(e)} aria-label={t("sso.saml.formLabel")}>
      <label className="block text-sm">{t("sso.admin.name")}<Input required maxLength={60} value={name} onChange={(e) => setName(e.target.value)} className="mt-1 max-w-sm" /></label>
      <label className="block text-sm">{t("sso.saml.metadata")}
        <textarea required={!config.has_metadata} rows={6} maxLength={262144} value={metadata} onChange={(e) => setMetadata(e.target.value)} spellCheck={false}
          className="mt-1 block w-full rounded-lg border border-input bg-transparent px-2.5 py-1.5 font-mono text-xs" />
        <span className="block text-xs text-muted-foreground">{t("sso.saml.metadataHint")}</span></label>
      <label className="block text-sm">{t("sso.saml.attribute")}<Input maxLength={200} value={attribute} onChange={(e) => setAttribute(e.target.value)} className="mt-1 max-w-md" />
        <span className="block text-xs text-muted-foreground">{t("sso.saml.attributeHint")}</span></label>
      <label className="block text-sm">{t("sso.admin.domains")}<Input value={domains} onChange={(e) => setDomains(e.target.value)} className="mt-1 max-w-md" placeholder={t("sso.admin.domainsExample")} />
        <span className="block text-xs text-muted-foreground">{t("sso.admin.domainsHint")}</span></label>
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="size-6" checked={autoLink} onChange={(e) => setAutoLink(e.target.checked)} />{t("sso.admin.autoLink")}</label>
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
