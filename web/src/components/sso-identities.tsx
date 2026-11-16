import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import { formatDate, useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";

type Linked = { kind: string; provider_name: string; email: string; created_at: string; last_login_at: string | null };

/** Single sign-on identities linked to the signed-in user, with a way to link one and to unlink it. */
export function SsoIdentities() {
  const { t } = useTranslation();
  const profile = useProfile();
  const qc = useQueryClient();
  const providers = useQuery({ queryKey: ["sso-providers"], retry: false, staleTime: 60_000, queryFn: async () => (await api<{ items: { kind: string; name: string }[] }>("/api/v1/auth/sso/providers")).items });
  const linked = useQuery({ queryKey: ["me-identities"], retry: false, queryFn: async () => (await api<{ items: Linked[] }>("/api/v1/me/identities")).items });
  const result = new URLSearchParams(location.search);
  const errorCode = result.get("sso_error");
  if (!providers.data?.length && !linked.data?.length) return null;
  async function unlink(kind: string) {
    try {
      await api(`/api/v1/me/identities/${kind}`, { method: "DELETE" });
      toast.success(t("sso.unlinked"));
      await qc.invalidateQueries({ queryKey: ["me-identities"] });
    } catch (err) { toast.error(err instanceof ApiError && err.status === 409 ? t("sso.needPassword") : t("sso.unlinkFailed")); }
  }
  return <section aria-labelledby="sso-identities" className="mt-8 space-y-3">
    <h2 id="sso-identities" className="text-base font-semibold">{t("sso.title")}</h2>
    <p className="text-sm text-muted-foreground">{t("sso.description")}</p>
    {result.get("sso") === "linked" && <p role="status" className="rounded border border-success p-2 text-sm">{t("sso.linked")}</p>}
    {errorCode && <p role="alert" className="rounded border border-destructive p-2 text-sm">{t(`auth.sso.error.${errorCode}`, { defaultValue: t("auth.sso.error.failed") })}</p>}
    {linked.data?.map((identity) => <div key={identity.kind} className="flex items-center justify-between gap-3 rounded-lg border p-3 text-sm">
      <span><span className="font-medium">{identity.provider_name}</span>{identity.email && <span className="block text-muted-foreground">{identity.email}</span>}
        <span className="block text-xs text-muted-foreground">{t("sso.since", { date: formatDate(identity.created_at, profile) })}</span></span>
      <Button variant="outline" size="sm" aria-label={t("sso.unlinkNamed", { name: identity.provider_name })} onClick={() => void unlink(identity.kind)}>{t("sso.unlink")}</Button>
    </div>)}
    {providers.data?.filter((p) => !linked.data?.some((l) => l.kind === p.kind)).map((provider) =>
      <a key={provider.kind} href={`/api/v1/auth/sso/${provider.kind}/start?link=1&next=${encodeURIComponent("/settings/security")}`}
        className="inline-block rounded-lg border border-input px-3 py-2 text-sm font-medium hover:bg-muted">{t("sso.link", { name: provider.name })}</a>)}
  </section>;
}
