import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

export const Route = createFileRoute("/_app/")({ component: Workspace });

function Workspace() {
  const { t } = useTranslation();
  return (
    <main className="p-8">
      <div className="rounded-xl border border-dashed bg-card p-12 text-center">
        <p className="text-sm font-medium text-primary">{t("workspace.home.eyebrow")}</p>
        <h1 className="mt-2 text-3xl font-semibold">{t("workspace.home.title")}</h1>
        <p className="mx-auto mt-3 max-w-lg text-muted-foreground">{t("workspace.home.description")}</p>
      </div>
    </main>
  );
}
