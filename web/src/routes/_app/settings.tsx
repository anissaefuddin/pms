import { Link, Outlet, createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

export const Route = createFileRoute("/_app/settings")({ component: SettingsLayout });

const linkClass = "block rounded px-3 py-2 text-sm";

function SettingsLayout() {
  const { t } = useTranslation();
  return (
    <main>
      <header className="border-b px-8 py-5"><h1 className="text-xl font-semibold">{t("nav.settings.label")}</h1></header>
      <div className="grid grid-cols-[15rem_1fr] gap-8 p-8">
        <nav className="space-y-6" aria-label={t("settings.nav.label")}>
          <section>
            <h2 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">{t("settings.nav.personal")}</h2>
            <Link to="/settings/security" className={linkClass} activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("settings.security.title")}</Link>
          </section>
          <section>
            <h2 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">{t("settings.nav.accountManagement")}</h2>
            <Link to="/spikes/gantt" className={linkClass}>{t("settings.nav.gantt")}</Link>
          </section>
        </nav>
        <Outlet />
      </div>
    </main>
  );
}
