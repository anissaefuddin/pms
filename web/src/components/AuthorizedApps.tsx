import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";

type Grant = { id: string; app_name: string; scopes: string[]; created_at: string };

export function AuthorizedApps() {
  const { t, i18n } = useTranslation();
  const cache = useQueryClient();
  const grants = useQuery({ queryKey: ["oauth-grants"], queryFn: () => api<{ items: Grant[] }>("/api/v1/oauth/grants") });
  const revoke = useMutation({
    mutationFn: (id: string) => api(`/api/v1/oauth/grants/${id}`, { method: "DELETE" }),
    onSuccess: () => cache.invalidateQueries({ queryKey: ["oauth-grants"] }),
  });
  return <section className="space-y-4">
    <p className="text-sm text-muted-foreground">{t("oauth.authorized.hint")}</p>
    {(grants.error || revoke.error) && <p role="alert">{(grants.error ?? revoke.error)?.message}</p>}
    {grants.data && !grants.data.items.length && <p className="text-muted-foreground">{t("oauth.authorized.empty")}</p>}
    <ul className="divide-y rounded border">
      {grants.data?.items.map((g) => <li key={g.id} className="flex items-center justify-between gap-3 p-3">
        <div>
          <p className="font-medium">{g.app_name}</p>
          <p className="text-sm text-muted-foreground">{t("oauth.authorized.scopes")}: {g.scopes.map((s) => t(`oauth.scope.${s}`, s)).join(", ")} · {t("oauth.authorized.since")} {new Date(g.created_at).toLocaleDateString(i18n.language)}</p>
        </div>
        <Button variant="outline" disabled={revoke.isPending} aria-label={t("oauth.authorized.revokeConfirm", { name: g.app_name })} onClick={() => revoke.mutate(g.id)}>{t("oauth.authorized.revoke")}</Button>
      </li>)}
    </ul>
  </section>;
}
