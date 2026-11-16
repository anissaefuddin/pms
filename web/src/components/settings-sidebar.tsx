import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft, Search } from "lucide-react";
import type { CurrentUser } from "@/lib/api";
import { ProfileMenu } from "@/components/topbar";
import { SIDEBAR_THEME } from "@/lib/sidebar-prefs";
import { sidebarThemeVars, useSidebarTheme } from "@/lib/sidebar-theme";

type SettingsItem = { to: string; hash?: string; params?: { spaceId: string }; label: string };
type SettingsGroup = { title: string; items: SettingsItem[] };

/** Settings shell sidebar (prototype): its own groups, a "Find settings" filter, and a link back to the workspace. */
export function settingsGroups(me: CurrentUser | undefined, t: (key: string) => string): SettingsGroup[] {
  const isAdmin = me?.user_type === "admin";
  // The settings shell has no current Space, so Space-scoped pages open the user's personal Space.
  const inPersonalSpace = (to: "/spaces/$spaceId" | "/spaces/$spaceId/automation", label: string): SettingsItem[] =>
    me?.personal_space_id ? [{ to, params: { spaceId: me.personal_space_id }, label }] : [];
  return [
    { title: t("settings.nav.personal"), items: [
      { to: "/settings/profile", label: t("settings.profile.title") },
      { to: "/settings/work-schedule", label: t("settings.workSchedule.title") },
      { to: "/settings/language", label: t("settings.nav.language") },
      { to: "/settings/work-preferences", label: t("settings.nav.work") },
      { to: "/settings/account", label: t("settings.account.title") },
      { to: "/settings/email", label: t("nav.account.emailPrefs") },
      { to: "/settings/appearance", label: t("nav.account.appearance") },
      { to: "/settings/security", label: t("settings.security.title") },
    ] },
    { title: t("settings.nav.accountManagement"), items: [
      ...(me?.capabilities.manage_users ? [{ to: "/settings/users", label: t("settings.users.title") }] : []),
      { to: "/settings/resources", label: t("settings.resources") },
      { to: "/settings/apps", label: t("apps.title") },
      ...(isAdmin ? [{ to: "/settings/rates", label: t("settings.rates") }] : []),
      { to: "/settings/spaces", label: t("settings.spacesManagement") },
    ] },
    { title: t("settings.nav.workConfiguration"), items: [
      { to: "/settings/item-types", label: t("settings.itemTypes.title") },
      { to: "/settings/workflows", label: t("settings.workflows.title") },
      ...inPersonalSpace("/spaces/$spaceId", t("items.customFields.title")),
      ...inPersonalSpace("/spaces/$spaceId/automation", t("automation.title")),
      { to: "/requests", label: t("forms.requests.title") },
      { to: "/blueprints", label: t("blueprints.nav") },
    ] },
    { title: t("settings.nav.security"), items: isAdmin ? [
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
    <aside style={sidebarThemeVars(sidebarTheme)} className={`${SIDEBAR_THEME} relative flex h-dvh w-[248px] shrink-0 flex-col border-r border-border bg-nav p-3 text-nav-text`}>
      <div className="mb-3 flex items-center gap-1">
        <Link to="/" className="flex min-w-0 flex-1 items-center gap-2 rounded-md px-2 py-1.5 text-sm text-nav-muted hover:bg-nav-hover hover:text-nav-text">
          <ArrowLeft aria-hidden="true" className="size-4 shrink-0" />
          <span className="truncate">{t("settings.nav.backToWorkspace")}</span>
        </Link>
        <div className="flex shrink-0 items-center gap-1">
          <ProfileMenu />
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
          <h2 className="mb-1 px-2 text-[11px] font-semibold uppercase tracking-wider text-nav-muted">{group.title}</h2>
          {group.items.map((item) => <Link key={item.to + (item.hash ?? "")} to={item.to} params={item.params} hash={item.hash} activeOptions={{ includeHash: true }}
            className={linkClass} activeProps={activeLink} >{item.label}</Link>)}
        </section>)}
        {groups.length === 0 && <p role="status" className="px-2 text-sm text-nav-muted">{t("settings.nav.noMatch")}</p>}
      </nav>
    </aside>
  );
}
