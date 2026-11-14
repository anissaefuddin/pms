import { Link, Outlet, createFileRoute } from "@tanstack/react-router";
import { useProfile } from "@/lib/profile";
import { useTranslation } from "react-i18next";

export const Route = createFileRoute("/_app/settings")({ component: SettingsLayout });

const linkClass = "block rounded px-3 py-2 text-sm";

function SettingsLayout() {
  const { t } = useTranslation();
  const me = useProfile();
  return (
    <main>
      <header className="border-b px-4 py-5 sm:px-8"><h1 className="workspace-title">{t("nav.settings.label")}</h1></header>
      <div className="grid grid-cols-1 gap-5 p-4 sm:p-8 lg:grid-cols-[15rem_minmax(0,1fr)] lg:gap-8">
        <nav className="grid grid-cols-2 gap-3 lg:block lg:space-y-6" aria-label={t("settings.nav.label")}>
          <section>
            <h2 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">{t("settings.nav.personal")}</h2>
            <Link to="/settings/profile" className={linkClass} activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("settings.profile.title")}</Link>
            <Link to="/settings/account" className={linkClass} activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("settings.account.title")}</Link>
            <Link to="/settings/security" className={linkClass} activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("settings.security.title")}</Link>
          </section>
          <section>
            <h2 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">{t("settings.nav.accountManagement")}</h2>
            {me?.capabilities.manage_users && <Link to="/settings/users" className={linkClass}
              activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("settings.users.title")}</Link>}
            <Link to="/settings/resources" className={linkClass} activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("settings.resources")}</Link>
            {me?.user_type === "admin" && <Link to="/settings/apps" className={linkClass}
              activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("settings.apps.title")}</Link>}
            {me?.user_type === "admin" && <Link to="/settings/rates" className={linkClass}
              activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("settings.rates")}</Link>}
            <Link to="/settings/apps" className={linkClass}
              activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("oauth.apps.title")}</Link>
            <Link to="/settings/work-schedule" className={linkClass}
              activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("settings.workSchedule.title")}</Link>
            <Link to="/settings/item-types" className={linkClass}
              activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("settings.itemTypes.title")}</Link>
            <Link to="/settings/workflows" className={linkClass}
              activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("settings.workflows.title")}</Link>
            {me?.user_type === "admin" && <Link to="/settings/sso" className={linkClass}
              activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("sso.admin.title")}</Link>}
            {me?.user_type === "admin" && <Link to="/settings/provisioning" className={linkClass}
              activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("provisioning.title")}</Link>}
            {me?.user_type === "admin" && <Link to="/settings/ownership" className={linkClass}
              activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("ownership.title")}</Link>}
            {me?.user_type === "admin" && <Link to="/settings/policy" className={linkClass}
              activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("policy.title")}</Link>}
            {me?.user_type === "admin" && <Link to="/settings/roles" className={linkClass}
              activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("roles.title")}</Link>}
            {me?.user_type === "admin" && <Link to="/settings/audit" className={linkClass}
              activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("audit.title")}</Link>}
            <Link to="/settings/apps" className={linkClass}
              activeProps={{ className: `${linkClass} bg-muted font-medium text-foreground`, "aria-current": "page" }}>{t("apps.title")}</Link>
            <Link to="/spikes/gantt" className={linkClass}>{t("settings.nav.gantt")}</Link>
          </section>
        </nav>
        <div className="min-w-0"><Outlet /></div>
      </div>
    </main>
  );
}
