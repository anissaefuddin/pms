import { Outlet, createFileRoute, useLocation } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { MobileSidebar } from "@/components/sidebar";
import { settingsGroups } from "@/components/settings-sidebar";
import { useProfile } from "@/lib/profile";

export const Route = createFileRoute("/_app/settings")({ component: SettingsLayout });

// One content width per page type, so pages line up: forms narrow, tables medium, boards wide.
const FORM_PAGES = ["/settings/profile", "/settings/language", "/settings/work-preferences", "/settings/account", "/settings/email",
  "/settings/appearance", "/settings/security", "/settings/sso", "/settings/provisioning", "/settings/policy", "/settings/ownership"];
const BOARD_PAGES = ["/settings/work-schedule", "/settings/workflows"];
function contentWidth(pathname: string) {
  if (BOARD_PAGES.some((page) => pathname.startsWith(page))) return "max-w-6xl";
  if (FORM_PAGES.some((page) => pathname.startsWith(page))) return "max-w-2xl";
  return "max-w-5xl";
}

// Page header for settings: breadcrumb (Settings / group) and the current page title. Replaces the empty top bar.
function SettingsLayout() {
  const { t } = useTranslation();
  const profile = useProfile();
  const pathname = useLocation({ select: (location) => location.pathname });
  const groups = settingsGroups(profile, t);
  const matches = groups.flatMap((group) => group.items.map((item) => ({ group, item })))
    .filter(({ item }) => pathname === item.to || pathname.startsWith(`${item.to}/`))
    .sort((a, b) => b.item.to.length - a.item.to.length);
  const current = matches[0];
  return (
    <main>
      <header className="flex items-start gap-3 border-b px-4 py-4 sm:px-8">
        {profile && <MobileSidebar me={profile} />}
        <div className="min-w-0 flex-1 space-y-1">
          <nav aria-label={t("nav.breadcrumb")} className="truncate text-xs text-muted-foreground">
            {t("nav.settings.label")}{current && <> / {current.group.title}</>}
          </nav>
          <h1 className="workspace-title">{current?.item.label ?? t("nav.settings.label")}</h1>
        </div>
      </header>
      <div className={`p-4 sm:p-8 ${contentWidth(pathname)}`}><Outlet /></div>
    </main>
  );
}
