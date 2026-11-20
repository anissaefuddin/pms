import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft, Search } from "lucide-react";
import type { CurrentUser } from "@/lib/api";
import { ProfileMenu } from "@/components/topbar";
import { SIDEBAR_THEME } from "@/lib/sidebar-prefs";
import { sidebarThemeVars, useSidebarTheme } from "@/lib/sidebar-theme";

type SettingsItem = { to: string; hash?: string; params?: { spaceId: string }; label: string };
type SettingsGroup = { title: string; scope?: string; items: SettingsItem[] };

/** Settings shell sidebar (prototype): its own groups, a "Find settings" filter, and a link back to the workspace. */
export function settingsGroups(me: CurrentUser | undefined, t: (key: string) => string): SettingsGroup[] {
  const isAdmin = me?.user_type === "admin";
  return [
    { title: t("settings.nav.personal"), scope: t("settings.nav.scopeOnlyYou"), items: [
      { to: "/settings/profile", label: t("settings.profile.title") },
      { to: "/settings/work-schedule", label: t("settings.workSchedule.title") },
      { to: "/settings/language", label: t("settings.nav.language") },
      { to: "/settings/work-preferences", label: t("settings.nav.work") },
      { to: "/settings/account", label: t("settings.account.title") },
      { to: "/settings/email", label: t("nav.account.emailPrefs") },
      { to: "/settings/appearance", label: t("nav.account.appearance") },
      { to: "/settings/security", label: t("settings.security.title") },
    ] },
    { title: t("settings.nav.accountManagement"), scope: t("settings.nav.scopeWorkspace"), items: [
      ...(me?.capabilities.manage_users ? [{ to: "/settings/users", label: t("settings.users.title") }] : []),
      { to: "/settings/resources", label: t("settings.resources") },
      { to: "/settings/apps", label: t("apps.title") },
      ...(isAdmin ? [{ to: "/settings/rates", label: t("settings.rates") }] : []),
      { to: "/settings/spaces", label: t("settings.spacesManagement") },
    ] },
    { title: t("settings.nav.workConfiguration"), scope: t("settings.nav.scopeSpaces"), items: [
      { to: "/settings/item-types", label: t("settings.itemTypes.title") },
      { to: "/settings/workflows", label: t("settings.workflows.title") },
      { to: "/settings/custom-fields", label: t("settings.customFields.title") },
      { to: "/settings/automation", label: t("automation.title") },
      { to: "/settings/request-forms", label: t("forms.manage.title") },
      { to: "/settings/blueprints", label: t("blueprints.nav") },
    ] },
    { title: t("settings.nav.security"), scope: t("settings.nav.scopeWorkspace"), items: isAdmin ? [
      { to: "/settings/sso", label: t("sso.admin.title") },
      { to: "/settings/provisioning", label: t("provisioning.title") },
      { to: "/settings/ownership", label: t("ownership.title") },
      { to: "/settings/policy", label: t("policy.title") },
      { to: "/settings/roles", label: t("roles.title") },
      { to: "/settings/audit", label: t("audit.title") },
    ] : [] },
  ];
}

const linkClass = "block rounded-md px-3 py-1.5 text-sm hover:bg-nav-hover";
const activeLink = { className: `${linkClass} bg-nav-active font-medium`, "aria-current": "page" as const };

export function SettingsSidebar({ me }: { me: CurrentUser }) {
  const { t } = useTranslation();
  const sidebarTheme = useSidebarTheme();
  const [query, setQuery] = useState("");
  const q = query.trim().toLowerCase();
  const groups = settingsGroups(me, t).map((group) => ({
    ...group,
    items: group.items.filter((item) => !q || item.label.toLowerCase().includes(q)),
  })).filter((group) => group.items.length > 0);
  return (
    <aside style={sidebarThemeVars(sidebarTheme)} className={`${SIDEBAR_THEME} relative flex h-dvh w-[216px] shrink-0 flex-col border-r border-border bg-nav p-3 text-nav-text`}>
      <div className="mb-3 flex items-center gap-1">
        {/* The label gets the room: the profile collapses to its avatar so "Back to workspace" fits on one line. */}
        <Link to="/" className="flex min-w-0 flex-1 items-center gap-1.5 rounded-md px-2 py-1.5 text-sm whitespace-nowrap text-nav-muted hover:bg-nav-hover hover:text-nav-text">
          <ArrowLeft aria-hidden="true" className="size-4 shrink-0" />
          <span className="min-w-0 truncate">{t("settings.nav.backToWorkspace")}</span>
        </Link>
        <div className="shrink-0">
          <ProfileMenu compact />
        </div>
      </div>
      <label className="mb-3 flex items-center gap-2 rounded-md border border-nav-hover px-2 py-1.5 text-sm text-nav-muted">
        <Search aria-hidden="true" className="size-4 shrink-0" />
        <span className="sr-only">{t("settings.nav.find")}</span>
        <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t("settings.nav.find")}
          className="min-w-0 flex-1 bg-transparent text-nav-text outline-none placeholder:text-nav-muted" />
      </label>
      <nav aria-label={t("settings.nav.label")} className="navscroll min-h-0 flex-1 space-y-4 overflow-y-auto">
        {groups.map((group) => <section key={group.title}>
          <h2 className="px-2 text-[11px] font-semibold uppercase tracking-wider text-nav-muted">{group.title}</h2>
          {group.scope && <p className="mb-1 px-2 text-[11px] text-nav-muted/80">{group.scope}</p>}
          {group.items.map((item) => <Link key={item.to + (item.hash ?? "")} to={item.to} params={item.params} hash={item.hash} activeOptions={{ includeHash: true }}
            className={linkClass} activeProps={activeLink} >{item.label}</Link>)}
        </section>)}
        {groups.length === 0 && <p role="status" className="px-2 text-sm text-nav-muted">{t("settings.nav.noMatch")}</p>}
      </nav>
    </aside>
  );
}
