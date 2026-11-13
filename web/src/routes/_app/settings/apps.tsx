import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { WebhooksTab } from "@/components/WebhooksTab";

export const Route = createFileRoute("/_app/settings/apps")({ component: Apps });

// Minimal Apps & Integrations shell (S11.3.1 owns the full page): one tab per integration kind; merge by adding tabs here.
function Apps() {
  const { t } = useTranslation();
  const [tab, setTab] = useState("webhooks");
  return <div className="space-y-4">
    <div role="tablist" aria-label={t("webhooks.title")} className="flex gap-2 border-b">
      <button role="tab" id="tab-webhooks" aria-selected={tab === "webhooks"} aria-controls="panel-webhooks" className="px-3 py-2 text-sm font-medium" onClick={() => setTab("webhooks")}>{t("webhooks.title")}</button>
    </div>
    <div role="tabpanel" id="panel-webhooks" aria-labelledby="tab-webhooks">{tab === "webhooks" && <WebhooksTab />}</div>
  </div>;
}
