import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ApiKeysTab } from "@/components/ApiKeysTab";
import { WebhooksTab } from "@/components/WebhooksTab";

export const Route = createFileRoute("/_app/settings/developer")({ component: DeveloperSettings });

function DeveloperSettings() {
  const { t } = useTranslation();
  return <div className="space-y-8">
    <header><p className="text-sm text-muted-foreground">{t("settings.developer.description")}</p></header>
    <ApiKeysTab />
    <WebhooksTab />
  </div>;
}
