import { Outlet, createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

export const Route = createFileRoute("/_app/settings")({ component: SettingsLayout });

const linkClass = "block rounded px-3 py-2 text-sm";

function SettingsLayout() {
  const { t } = useTranslation();
  return (
    <main>
      <header className="border-b px-4 py-5 sm:px-8"><h1 className="workspace-title">{t("nav.settings.label")}</h1></header>
      <div className="p-4 sm:p-8"><Outlet /></div>
    </main>
  );
}
