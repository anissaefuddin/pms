import { useState, type FormEvent } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { components } from "@/api/schema";
import { ApiError, api } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";

type Policy = components["schemas"]["AccessPolicy"];

export const Route = createFileRoute("/_app/settings/policy")({ component: PolicyPage });

const KEY = ["access-policy"] as const;

function PolicyPage() {
  const { t } = useTranslation();
  const profile = useProfile();
  const admin = profile?.user_type === "admin";
  const query = useQuery({ queryKey: KEY, enabled: admin, retry: false, queryFn: () => api<Policy>("/api/v1/access-policy") });
  if (profile && !admin) return <p role="alert">{t("policy.adminOnly")}</p>;
  return <section className="space-y-5">
    <header>
      <p className="text-sm text-muted-foreground">{t("policy.description")}</p>
    </header>
    {query.isPending && <p role="status" className="text-sm text-muted-foreground">{t("policy.loading")}</p>}
    {query.isError && <p role="alert">{t("policy.error")}</p>}
    {query.data && <PolicyForm key={query.data.version} policy={query.data} />}
  </section>;
}

function PolicyForm({ policy }: { policy: Policy }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [enabled, setEnabled] = useState(policy.ip_allowlist_enabled);
  const [otp, setOtp] = useState(policy.otp_login_enabled);
  const [text, setText] = useState(policy.ip_allowlist.join("\n"));
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const lines = () => text.split(/[\n,]+/).map((x) => x.trim()).filter(Boolean);
  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true); setError("");
    try {
      await api<Policy>("/api/v1/access-policy", { method: "PUT", headers: { "If-Match": `"${policy.version}"` }, body: JSON.stringify({ ip_allowlist_enabled: enabled, ip_allowlist: lines(), otp_login_enabled: otp }) });
      toast.success(t("policy.saved"));
      await qc.invalidateQueries({ queryKey: KEY });
    } catch (err) {
      setError(err instanceof ApiError && err.status === 409 ? t("policy.conflict") : (err as Error).message || t("policy.saveFailed"));
    } finally { setBusy(false); }
  }
  return <form onSubmit={(e) => void submit(e)} className="space-y-4 rounded-lg border p-4" aria-label={t("policy.title")}>
    <h2 className="font-semibold">{t("policy.ip.title")}</h2>
    <p className="text-sm text-muted-foreground">{t("policy.ip.hint")}</p>
    {policy.override_active && <p role="status" className="text-sm text-amber-700 dark:text-amber-400">{t("policy.ip.override")}</p>}
    <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="size-6" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />{t("policy.ip.enabled")}</label>
    <label className="block text-sm">{t("policy.ip.list")}
      <textarea rows={6} className="mt-1 block w-full max-w-lg rounded-md border border-input bg-background p-2 font-mono text-xs" value={text} onChange={(e) => setText(e.target.value)} aria-describedby="ip-list-hint" /></label>
    <p id="ip-list-hint" className="text-sm text-muted-foreground">{t("policy.ip.listHint")}</p>
    <div className="flex flex-wrap items-center gap-3 text-sm">
      <span>{t("policy.ip.yourAddress", { ip: policy.your_ip })}</span>
      <Button type="button" size="sm" variant="outline" disabled={lines().includes(policy.your_ip)} onClick={() => setText([...lines(), policy.your_ip].join("\n"))}>{t("policy.ip.useMine")}</Button>
    </div>
    <p className="text-sm text-muted-foreground">{t("policy.ip.recovery")}</p>
    <h2 className="pt-2 font-semibold">{t("policy.otp.title")}</h2>
    <p className="text-sm text-muted-foreground">{t("policy.otp.hint")}</p>
    <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="size-6" checked={otp} onChange={(e) => setOtp(e.target.checked)} />{t("policy.otp.enabled")}</label>
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    <Button type="submit" disabled={busy}>{t("policy.save")}</Button>
  </form>;
}
