import { useState, type FormEvent } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { components } from "@/api/schema";
import { ApiError, api } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type Ownership = components["schemas"]["AccountOwnership"];

export const Route = createFileRoute("/_app/settings/ownership")({ component: OwnershipPage });

const KEY = ["account-ownership"] as const;
const select = "mt-1 block h-9 rounded-md border border-input bg-background px-2";

function OwnershipPage() {
  const { t } = useTranslation();
  const profile = useProfile();
  const admin = profile?.user_type === "admin";
  const qc = useQueryClient();
  const query = useQuery({ queryKey: KEY, enabled: admin, retry: false, queryFn: () => api<Ownership>("/api/v1/account/ownership") });
  const [target, setTarget] = useState("");
  const [password, setPassword] = useState("");
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState("");
  if (profile && !admin) return <p role="alert">{t("ownership.adminOnly")}</p>;
  const data = query.data;
  const chosen = data?.candidates.find((c) => c.id === target);

  function ask(event: FormEvent) {
    event.preventDefault();
    setError("");
    setConfirming(true);
  }
  async function transfer() {
    setError("");
    try {
      await api<Ownership>("/api/v1/account/ownership", { method: "POST", body: JSON.stringify({ new_owner_id: target, password }) });
      setConfirming(false); setPassword(""); setTarget("");
      toast.success(t("ownership.done"));
      await qc.invalidateQueries({ queryKey: KEY });
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "";
      setError(code === "invalid_password" ? t("ownership.invalidPassword") : code === "recent_sign_in_required" ? t("ownership.recentSignIn") : (err as Error).message || t("ownership.failed"));
    }
  }

  return <section className="space-y-5">
    <header>
      <p className="text-sm text-muted-foreground">{t("ownership.description")}</p>
    </header>
    {query.isPending && <p role="status" className="text-sm text-muted-foreground">{t("ownership.loading")}</p>}
    {query.isError && <p role="alert">{t("ownership.error")}</p>}
    {data && <>
      <p className="text-sm"><span className="font-medium">{t("ownership.owner")}: </span>
        {data.owner ? `${data.owner.name} (${data.owner.email})` : t("ownership.noOwner")}</p>
      {data.is_owner ? <p role="status" className="text-sm text-muted-foreground">{t("ownership.you")}</p> : <p role="status" className="text-sm text-muted-foreground">{t("ownership.notOwner")}</p>}
      {data.is_owner && (data.candidates.length === 0
        ? <p role="status" className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">{t("ownership.noCandidates")}</p>
        : <form onSubmit={ask} className="space-y-4 rounded-lg border p-4" aria-label={t("ownership.transfer")}>
          <label className="block text-sm">{t("ownership.newOwner")}
            <select required className={select} value={target} onChange={(e) => setTarget(e.target.value)}>
              <option value="">{t("ownership.choose")}</option>
              {data.candidates.map((c) => <option key={c.id} value={c.id}>{`${c.name} (${c.email})`}</option>)}
            </select></label>
          <label className="block text-sm">{t("ownership.password")}
            <Input type="password" autoComplete="current-password" className="mt-1 max-w-sm" value={password} onChange={(e) => setPassword(e.target.value)} aria-describedby="owner-password-hint" /></label>
          <p id="owner-password-hint" className="text-sm text-muted-foreground">{t("ownership.passwordHint")}</p>
          <Button type="submit" disabled={!target}>{t("ownership.submit")}</Button>
        </form>)}
    </>}
    <ConfirmDialog open={confirming} onOpenChange={(open) => !open && setConfirming(false)} error={error}
      title={t("ownership.confirmTitle", { name: chosen?.name ?? "" })} description={t("ownership.confirmDescription", { name: chosen?.name ?? "" })}
      confirmLabel={t("ownership.confirm")} cancelLabel={t("ownership.cancel")} onConfirm={() => void transfer()} />
  </section>;
}
