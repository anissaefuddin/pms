import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { TopbarTitle } from "@/components/topbar-title";
import { WorkspaceOverview } from "@/components/workspace-overview";

export const Route = createFileRoute("/_app/dashboards/")({ component: Dashboards });

function Dashboards() {
  const { t } = useTranslation();
  return <main className="space-y-5 p-8">
    <TopbarTitle>{t("dashboards.title")}</TopbarTitle>
    <header className="border-b pb-5">
      <p className="text-sm text-muted-foreground">{t("dashboards.description")}</p>
    </header>
    <WorkspaceOverview />
  </main>;
}
