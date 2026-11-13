import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ApiKeysTab } from "@/components/ApiKeysTab";

export const Route = createFileRoute("/_app/settings/apps")({ component: Apps });

// Other integration tabs (webhooks, connectors) add an entry here and their own component.
function Apps() {
  const { t } = useTranslation();
  const tabs = [{ id: "api", label: t("apps.tab.api"), body: <ApiKeysTab /> }];
  const [active, setActive] = useState("api");
  return <section className="space-y-5">
    <h2 className="text-lg font-semibold">{t("apps.title")}</h2>
    <div role="tablist" aria-label={t("apps.tabs")} className="flex gap-2 border-b">
      {tabs.map((tab) => <button key={tab.id} role="tab" id={`apps-tab-${tab.id}`} aria-selected={active === tab.id} aria-controls={`apps-panel-${tab.id}`}
        onClick={() => setActive(tab.id)} className={`px-3 py-2 text-sm ${active === tab.id ? "border-b-2 border-primary font-medium" : "text-muted-foreground"}`}>{tab.label}</button>)}
    </div>
    {tabs.map((tab) => active === tab.id && <div key={tab.id} role="tabpanel" id={`apps-panel-${tab.id}`} aria-labelledby={`apps-tab-${tab.id}`}>{tab.body}</div>)}
  </section>;
}
