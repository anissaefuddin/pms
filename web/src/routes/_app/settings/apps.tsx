import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useProfile } from "@/lib/profile";
import { AuthorizedApps } from "@/components/AuthorizedApps";
import { OAuthClients } from "@/components/OAuthClients";

export const Route = createFileRoute("/_app/settings/apps")({ component: Apps });

// Minimal shell: S11.3.1/S11.3.2 add their own tabs to `tabs` below.
function Apps() {
  const { t } = useTranslation();
  const me = useProfile();
  const tabs = [
    { id: "authorized", label: t("oauth.apps.authorized"), body: <AuthorizedApps /> },
    ...(me?.user_type === "admin" ? [{ id: "clients", label: t("oauth.apps.clients"), body: <OAuthClients /> }] : []),
  ];
  const [active, setActive] = useState("authorized");
  const current = tabs.find((x) => x.id === active) ?? tabs[0];
  return <section className="space-y-5">
    <h2 className="text-lg font-semibold">{t("oauth.apps.title")}</h2>
    <div role="tablist" aria-label={t("oauth.apps.tabs")} className="flex gap-2 border-b">
      {tabs.map((x) => <button key={x.id} role="tab" type="button" aria-selected={x.id === current.id} onClick={() => setActive(x.id)}
        className={`px-3 py-2 text-sm ${x.id === current.id ? "border-b-2 border-foreground font-medium" : "text-muted-foreground"}`}>{x.label}</button>)}
    </div>
    <div role="tabpanel">{current.body}</div>
  </section>;
}
