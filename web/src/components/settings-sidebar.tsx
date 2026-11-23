import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft, Search, BadgeDollarSign, Blocks, Boxes, CalendarClock, ClipboardList, Crown, History, IdCard, KeyRound, Languages, Layers, LayoutTemplate, ListChecks, Mail, Palette, ScrollText, Shapes, ShieldCheck, SlidersHorizontal, UserCog, UserPlus, UserRound, Users, Workflow, Zap } from "lucide-react";

// One icon per Settings page, so each item reads at a glance.
const SETTINGS_ICON: Record<string, LucideIcon> = {
  "/settings/profile": UserRound,
  "/settings/work-schedule": CalendarClock,
  "/settings/language": Languages,
  "/settings/work-preferences": SlidersHorizontal,
  "/settings/account": IdCard,
  "/settings/email": Mail,
  "/settings/appearance": Palette,
  "/settings/security": ShieldCheck,
  "/settings/users": Users,
  "/settings/resources": Boxes,
  "/settings/apps": Blocks,
  "/settings/rates": BadgeDollarSign,
  "/settings/spaces": Layers,
  "/settings/item-types": Shapes,
  "/settings/workflows": Workflow,
  "/settings/custom-fields": ListChecks,
  "/settings/automation": Zap,
  "/settings/request-forms": ClipboardList,
  "/settings/blueprints": LayoutTemplate,
  "/settings/sso": KeyRound,
  "/settings/provisioning": UserPlus,
  "/settings/ownership": Crown,
  "/settings/policy": ScrollText,
  "/settings/roles": UserCog,
  "/settings/audit": History,
};
import type { CurrentUser } from "@/lib/api";
import type { LucideIcon } from "lucide-react";
import { SidebarFooter } from "@/components/topbar";
import { SIDEBAR_THEME, SIDEBAR_WIDTH, clampWidth, readWidth, writeWidth } from "@/lib/sidebar-prefs";
import { sidebarThemeVars, useSidebarTheme } from "@/lib/sidebar-theme";

type SettingsItem = { to: string; hash?: string; params?: { spaceId: string }; label: string };
type SettingsGroup = { title: string; items: SettingsItem[] };

/** Settings shell sidebar (prototype): its own groups, a "Find settings" filter, and a link back to the workspace. */
export function settingsGroups(me: CurrentUser | undefined, t: (key: string) => string): SettingsGroup[] {
  const isAdmin = me?.user_type === "admin";
  return [
    { title: t("settings.nav.personal"), items: [
      { to: "/settings/profile", label: t("settings.profile.title") },
      { to: "/settings/work-schedule", label: t("settings.workSchedule.title") },
      { to: "/settings/language", label: t("settings.nav.language") },
      { to: "/settings/work-preferences", label: t("settings.nav.work") },
      { to: "/settings/email", label: t("nav.account.emailPrefs") },
      { to: "/settings/appearance", label: t("nav.account.appearance") },
      { to: "/settings/security", label: t("settings.security.title") },
    ] },
    { title: t("settings.nav.accountManagement"), items: [
      ...(isAdmin ? [{ to: "/settings/account", label: t("settings.account.title") }] : []),
      ...(me?.capabilities.manage_users ? [{ to: "/settings/users", label: t("settings.users.title") }] : []),
      { to: "/settings/resources", label: t("settings.resources") },
      { to: "/settings/apps", label: t("apps.title") },
      ...(isAdmin ? [{ to: "/settings/rates", label: t("settings.rates") }] : []),
      { to: "/settings/spaces", label: t("settings.spacesManagement") },
    ] },
    { title: t("settings.nav.workConfiguration"), items: [
      { to: "/settings/item-types", label: t("settings.itemTypes.title") },
      { to: "/settings/workflows", label: t("settings.workflows.title") },
      { to: "/settings/custom-fields", label: t("settings.customFields.title") },
      { to: "/settings/automation", label: t("automation.title") },
      { to: "/settings/request-forms", label: t("forms.manage.title") },
      { to: "/settings/blueprints", label: t("blueprints.nav") },
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
  // Same width as the Workspace sidebar: one stored value, dragged or set with the arrow keys on the edge.
  const [width, setWidth] = useState(readWidth);
  const drag = useRef<{ x: number; width: number } | null>(null);
  useEffect(() => { writeWidth(width); }, [width]);
  function onHandleKey(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
    event.preventDefault();
    setWidth((w) => clampWidth(w + (event.key === "ArrowRight" ? SIDEBAR_WIDTH.step : -SIDEBAR_WIDTH.step)));
  }
  function onHandleDown(event: PointerEvent<HTMLDivElement>) {
    drag.current = { x: event.clientX, width };
    event.currentTarget.setPointerCapture(event.pointerId);
  }
  function onHandleMove(event: PointerEvent<HTMLDivElement>) {
    if (drag.current) setWidth(clampWidth(drag.current.width + event.clientX - drag.current.x));
  }
  const q = query.trim().toLowerCase();
  const groups = settingsGroups(me, t).map((group) => ({
    ...group,
    items: group.items.filter((item) => !q || item.label.toLowerCase().includes(q)),
  })).filter((group) => group.items.length > 0);
  return (
    <aside style={{ width: `${width}px`, ...sidebarThemeVars(sidebarTheme) }} className={`${SIDEBAR_THEME} relative flex h-dvh shrink-0 flex-col border-r border-nav-hover bg-nav p-3 text-nav-text`}>
      <div className="mb-3 flex items-center gap-1">
        {/* The label gets the room: the profile collapses to its avatar so "Back to workspace" fits on one line. */}
        <Link to="/" className="flex min-w-0 flex-1 items-center gap-1.5 rounded-md px-2 py-1.5 text-sm whitespace-nowrap text-nav-muted hover:bg-nav-hover hover:text-nav-text">
          <ArrowLeft aria-hidden="true" className="size-4 shrink-0" />
          <span className="min-w-0 truncate">{t("settings.nav.backToWorkspace")}</span>
        </Link>
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
          {group.items.map((item) => <Link key={item.to + (item.hash ?? "")} to={item.to} params={item.params} hash={item.hash} activeOptions={{ includeHash: true }}
            className={linkClass} activeProps={activeLink} >{(() => { const Icon = SETTINGS_ICON[item.to]; return <span className="flex items-center gap-2.5">{Icon && <Icon aria-hidden="true" className="size-4 shrink-0 text-nav-muted" />}{item.label}</span>; })()}</Link>)}
        </section>)}
        {groups.length === 0 && <p role="status" className="px-2 text-sm text-nav-muted">{t("settings.nav.noMatch")}</p>}
        {/* Installation: the running version, in the same scroll area as the sections above. */}
        <div className="border-t border-nav-hover px-2 pt-3">
          <p className="text-[11px] font-semibold uppercase tracking-wider text-nav-muted">{t("settings.nav.installation")}</p>
          <p className="mt-1 text-sm text-nav-text">{t("settings.nav.version", { version: __APP_VERSION__ })}</p>
        </div>
      </nav>
      <SidebarFooter />
      <div role="separator" aria-orientation="vertical" aria-label={t("nav.resize")} aria-valuemin={SIDEBAR_WIDTH.min} aria-valuemax={SIDEBAR_WIDTH.max} aria-valuenow={width} tabIndex={0}
        onKeyDown={onHandleKey} onPointerDown={onHandleDown} onPointerMove={onHandleMove} onPointerUp={() => { drag.current = null; }}
        className="absolute inset-y-0 right-0 w-1 cursor-col-resize touch-none hover:bg-nav-hover focus-visible:bg-nav-active focus-visible:outline-none" />
    </aside>
  );
}
