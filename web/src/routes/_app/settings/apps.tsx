import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useProfile } from "@/lib/profile";
import { AuthorizedApps } from "@/components/AuthorizedApps";
import { ConnectorsTab } from "@/components/ConnectorsTab";
import { OAuthClients } from "@/components/OAuthClients";

export const Route = createFileRoute("/_app/settings/apps")({ component: Apps });

// Apps & Integrations: each story adds its own tab to `tabs` below (S11.3.1 API keys and S11.3.2 webhooks still to come).
function Apps() {
  const { t } = useTranslation();
  const me = useProfile();
  const admin = me?.user_type === "admin";
  const tabs = [
    { id: "authorized", label: t("oauth.apps.authorized"), body: <AuthorizedApps /> },
    ...(admin ? [
      { id: "clients", label: t("oauth.apps.clients"), body: <OAuthClients /> },
      { id: "connectors", label: t("settings.apps.connectors.title"), body: <ConnectorsTab /> },
    ] : []),
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
