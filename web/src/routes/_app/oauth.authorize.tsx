import { createFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";

export const Route = createFileRoute("/_app/oauth/authorize")({ component: Consent });

function Consent() {
  const { t } = useTranslation();
  const query = window.location.search;
  const params = new URLSearchParams(query);
  const info = useQuery({ queryKey: ["oauth-authorize", query], retry: false, queryFn: () => api<{ app_name: string; scopes: string[] }>(`/api/v1/oauth/authorize${query}`) });
  const decide = useMutation({
    mutationFn: (approve: boolean) => api<{ redirect_to: string }>("/api/v1/oauth/authorize", { method: "POST",
      body: JSON.stringify({ client_id: params.get("client_id"), redirect_uri: params.get("redirect_uri"), scope: params.get("scope"), state: params.get("state"),
        code_challenge: params.get("code_challenge"), code_challenge_method: params.get("code_challenge_method"), approve }) }),
    onSuccess: (r) => { window.location.assign(r.redirect_to); }, // server only returns a pre-registered redirect URI
  });
  // An invalid request never redirects anywhere: the redirect URI is not trusted yet.
  if (info.error) return <main className="mx-auto max-w-md p-6"><p role="alert">{t("oauth.consent.invalid")}</p></main>;
  if (!info.data) return null;
  return <main className="mx-auto max-w-md space-y-4 p-6">
    <h1 className="text-xl font-semibold">{t("oauth.consent.title", { name: info.data.app_name })}</h1>
    <p>{t("oauth.consent.intro", { name: info.data.app_name })}</p>
    <ul className="list-disc pl-5">{info.data.scopes.map((s) => <li key={s}>{t(`oauth.scope.${s}`, s)}</li>)}</ul>
    {decide.error && <p role="alert">{decide.error.message}</p>}
    <div className="flex gap-3">
      <Button disabled={decide.isPending} onClick={() => decide.mutate(true)}>{t("oauth.consent.allow")}</Button>
      <Button variant="outline" disabled={decide.isPending} onClick={() => decide.mutate(false)}>{t("oauth.consent.deny")}</Button>
    </div>
  </main>;
}
