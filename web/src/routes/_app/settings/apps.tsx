import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useProfile } from "@/lib/profile";
import { ConnectorsTab } from "@/components/ConnectorsTab";
import { OAuthClients } from "@/components/OAuthClients";
import { IntegrationGallery } from "@/components/integration-gallery";

export const Route = createFileRoute("/_app/settings/apps")({ component: Apps });

// Integration Center: each section reuses an existing tab component. Gallery is a read-only index of the sections.
// Admin-only sections (Connectors, OAuth apps) are hidden for everyone else; their APIs enforce the same rule.
function Apps() {
  const { t } = useTranslation();
  const me = useProfile();
  const admin = me?.user_type === "admin";
  const [active, setActive] = useState("gallery");
  const sections = [
    { id: "connectors", label: t("settings.apps.connectors.title"), body: <ConnectorsTab /> },
    { id: "clients", label: t("oauth.apps.clients"), body: <OAuthClients /> },
  ];
  const tabs = [
    { id: "gallery", label: t("apps.tab.gallery"), body: <IntegrationGallery admin={admin} onOpen={setActive} /> },
    ...sections,
  ];
  if (me && !admin) return <p role="alert">{t("settings.adminOnly")}</p>;
  const current = tabs.find((x) => x.id === active) ?? tabs[0];
  return <section className="space-y-5">
    <h2 className="text-lg font-semibold">{t("apps.center.title")}</h2>
    <div role="tablist" aria-label={t("apps.tabs")} className="flex flex-wrap gap-2 border-b">
      {tabs.map((x) => <button key={x.id} role="tab" type="button" id={`apps-tab-${x.id}`} aria-selected={x.id === current.id} aria-controls={`apps-panel-${x.id}`} onClick={() => setActive(x.id)}
        className={`px-3 py-2 text-sm ${x.id === current.id ? "border-b-2 border-foreground font-medium" : "text-muted-foreground"}`}>{x.label}</button>)}
    </div>
    <div role="tabpanel" id={`apps-panel-${current.id}`} aria-labelledby={`apps-tab-${current.id}`}>{current.body}</div>
  </section>;
}
