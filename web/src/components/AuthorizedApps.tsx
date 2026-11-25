import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { AppWindow } from "lucide-react";
import { api } from "@/lib/api";
import { formatDate, useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { FormError } from "@/components/auth/form-error";
import { AdminEmptyState, AdminPageHeader } from "@/components/admin/admin-ui";

type Grant = { id: string; app_name: string; scopes: string[]; created_at: string };

/** Apps you let sign in as you. This list is personal: it does not show connectors, which belong to a workspace or Space. */
export function AuthorizedApps() {
  const { t } = useTranslation();
  const profile = useProfile();
  const cache = useQueryClient();
  const [revoking, setRevoking] = useState<Grant>();
  const grants = useQuery({ queryKey: ["oauth-grants"], queryFn: () => api<{ items: Grant[] }>("/api/v1/oauth/grants") });
  const revoke = useMutation({
    mutationFn: (id: string) => api(`/api/v1/oauth/grants/${id}`, { method: "DELETE" }),
    onSuccess: () => { setRevoking(undefined); void cache.invalidateQueries({ queryKey: ["oauth-grants"] }); },
  });
  return <section className="space-y-6">
    <AdminPageHeader title={t("oauth.authorized.title")} description={t("oauth.authorized.hint")} />
    <p className="text-xs text-muted-foreground">{t("oauth.authorized.personalNote")}</p>
    <FormError message={grants.error?.message ?? revoke.error?.message ?? ""} />

    {grants.data && !grants.data.items.length && <AdminEmptyState title={t("oauth.authorized.emptyTitle")} description={t("oauth.authorized.empty")}
      action={<AppWindow aria-hidden="true" className="size-6 text-muted-foreground" />} />}
    {!!grants.data?.items.length && <ul className="divide-y rounded-lg border">
      {grants.data.items.map((grant) => <li key={grant.id} className="flex flex-wrap items-center gap-3 px-4 py-3">
        <AppWindow aria-hidden="true" className="size-5 shrink-0 text-muted-foreground" />
        <div className="min-w-0 flex-1">
          <p className="truncate font-medium">{grant.app_name}</p>
          <p className="text-xs text-muted-foreground">{t("oauth.authorized.since")} {formatDate(grant.created_at, profile)}</p>
        </div>
        <div className="flex flex-wrap gap-1.5">
          {grant.scopes.map((scope) => <span key={scope} className="rounded-full bg-muted px-2 py-0.5 text-xs">{t(`oauth.scope.${scope}`, { defaultValue: scope })}</span>)}
        </div>
        <Button variant="outline" size="sm" disabled={revoke.isPending} aria-label={t("oauth.authorized.revokeConfirm", { name: grant.app_name })} onClick={() => setRevoking(grant)}>{t("oauth.authorized.revoke")}</Button>
      </li>)}
    </ul>}

    <ConfirmDialog open={!!revoking} onOpenChange={(open) => { if (!open) setRevoking(undefined); }}
      title={t("oauth.authorized.revokeTitle", { name: revoking?.app_name ?? "" })} description={t("oauth.authorized.revokeBody")}
      confirmLabel={t("oauth.authorized.revoke")} cancelLabel={t("items.cancel")} busy={revoke.isPending} error={revoke.error?.message}
      onConfirm={() => { if (revoking) revoke.mutate(revoking.id); }} />
  </section>;
}
