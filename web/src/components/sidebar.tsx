import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent, type ReactElement } from "react";
import { Activity, Briefcase, Folder, ChevronDown, ChevronLeft, ChevronRight, Clock, ClipboardCheck, CalendarDays, ClipboardList, UserPen, FileText, Inbox, LayoutDashboard, ListChecks, Menu, MoreHorizontal, PanelLeft, Plus, Search, Settings, Share2, LayoutTemplate, Star, Trash2, Users } from "lucide-react";
import { Link, useNavigate, useLocation } from "@tanstack/react-router";
import { sidebarThemeVars, useSidebarTheme } from "@/lib/sidebar-theme";
import { SidebarFooter } from "@/components/topbar";
import { useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { listNotifications, notificationKey } from "@/lib/notifications";
import { starKeys, useStars } from "@/components/star-button";
import { createAnalyticsBoard } from "@/lib/analytics-board";
import { api, type CurrentUser } from "@/lib/api";
import { itemKeys, listItems, type Item } from "@/lib/items";
import { useSpaces } from "@/lib/space-queries";
import type { Space } from "@/lib/spaces";
import {
  FIXED_SHORTCUT, pinPatch, SHORTCUT_IDS, SHORTCUT_LABEL_KEY, SIDEBAR_THEME, SIDEBAR_WIDTH, clampWidth, isShortcutVisible,
  pushRecent, readOpenSpaces, readRecent, readWidth, sanitizePinOrder, toggleOpen, writeOpenSpaces, writeRecent, writeWidth,
  type RecentEntry, type ShortcutId,
} from "@/lib/sidebar-prefs";
import { Button } from "./ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "./ui/tooltip";
import type { Space as SpaceType } from "@/lib/spaces";
import { spaceActions } from "@/lib/space-actions";
import { WorkspaceBrandMark } from "@/lib/workspace-branding";

/** Route of a Space's settings tabs (kept as a constant so the route is not a literal in JSX). */
const SPACE_SETTINGS_ROUTE = "/spaces/$spaceId/settings/$tab" as const;

/** Copies the Space's address; a refused clipboard is reported instead of failing silently. */
async function copySpaceLink(id: string, t: (key: string) => string) {
  try {
    await navigator.clipboard.writeText(`${window.location.origin}/spaces/${id}`);
    toast.success(t("nav.space.menu.linkCopied"));
  } catch {
    toast.error(t("nav.space.menu.linkCopyFailed"));
  }
}
import { DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "./ui/dropdown-menu";
import { Dialog, DialogContent, DialogTitle, DialogTrigger } from "./ui/dialog";
import { SpaceDialog } from "./space-dialog";
import { SpaceIcon } from "./space-icon";
import { FOCUS_EVENT, readFocus } from "@/lib/view-focus";
import { CustomizeSidebarDialog, openCustomizeSidebar } from "./customize-sidebar-dialog";

const linkClass = "block min-w-0 flex-1 truncate rounded-md px-2 py-1.5 text-sm hover:bg-nav-hover";
const navRow = "flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm hover:bg-nav-hover";
const activeNav = { className: `${navRow} bg-nav-active font-medium`, "aria-current": "page" as const };
const activeLink = { className: `${linkClass} bg-nav-active font-medium`, "aria-current": "page" as const };

/** Hover atau fokus di baris (atau sentuhan) memunculkan aksi baris; tombol tetap bisa dijangkau lewat Tab. */
const revealOnHover = "opacity-0 transition-opacity group-hover/row:opacity-100 group-focus-within/row:opacity-100 data-[popup-open]:opacity-100 [@media(hover:none)]:opacity-100";

const recentQuery = (userId: string) => ["sidebar", "recent", userId] as const;

/** Catat item yang dibuka: cache dan localStorage selalu sama, jadi More langsung ikut berubah. */
function rememberItem(client: QueryClient, userId: string, entry: RecentEntry) {
  const list = pushRecent(client.getQueryData<RecentEntry[]>(recentQuery(userId)) ?? readRecent(userId), entry);
  client.setQueryData(recentQuery(userId), list);
  writeRecent(userId, list);
}

function itemIcon(type?: string) {
  return type === "project" ? <Briefcase aria-hidden="true" className="size-4 shrink-0" />
    : type === "folder" ? <Folder aria-hidden="true" className="size-4 shrink-0" />
    : <ListChecks aria-hidden="true" className="size-4 shrink-0" />;
}

const SHORTCUT_ROUTE = {
  request_forms: "/requests", stream: "/stream", portfolio: "/portfolio", workload: "/workload",
  timesheets: "/timesheets", starred: "/starred", dashboards: "/dashboards", reports: "/reports",
  my_todo: "/my-todo", calendars: "/calendars", created_by_me: "/created-by-me", reviews: "/reviews", recycle: "/recycle-bin",
} as const satisfies Record<ShortcutId, string>;

const SHORTCUT_ICON: Record<ShortcutId, ReactElement> = {
  request_forms: <ClipboardList />, stream: <Activity />, portfolio: <Briefcase />, workload: <Users />,
  timesheets: <Clock />, starred: <Star />, dashboards: <LayoutDashboard />, reports: <FileText />,
  my_todo: <ListChecks />, calendars: <CalendarDays />, created_by_me: <UserPen />, reviews: <ClipboardCheck />, recycle: <Trash2 />,
};

export function MobileSidebar({ me }: { me: CurrentUser }) {
  const { t } = useTranslation();
  const pathname = useLocation({ select: (location) => location.pathname });
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!open) return;
    const desktop = window.matchMedia("(min-width: 64rem)");
    const closeOnDesktop = () => { if (desktop.matches) setOpen(false); };
    desktop.addEventListener("change", closeOnDesktop);
    return () => {
      desktop.removeEventListener("change", closeOnDesktop);
    };
  }, [open]);

  useEffect(() => setOpen(false), [pathname]);

  return <Dialog open={open} onOpenChange={setOpen}>
    <DialogTrigger render={<Button variant="ghost" size="icon" className="lg:hidden"
      aria-label={t("nav.openNavigation")} />}><Menu /></DialogTrigger>
    <DialogContent className="inset-y-0 left-0 top-0 flex w-[var(--sidebar-width)] max-w-full translate-none flex-col gap-0 rounded-none p-0 [animation:none]! scale-none!">
      <DialogTitle className="sr-only">{t("nav.main.label")}</DialogTitle>
      <Sidebar me={me} mobile onNavigate={() => setOpen(false)} />
    </DialogContent>
  </Dialog>;
}

export function Sidebar({ me, mobile = false, onNavigate }: { me: CurrentUser; mobile?: boolean; onNavigate?: () => void }) {
  const sidebarTheme = useSidebarTheme();
  const { t } = useTranslation();
  const navigate = useNavigate();
  const pathname = useLocation({ select: (location) => location.pathname });
  const storageKey = `pms:sidebar:${me.id}`;
  const [desktopCollapsed, setCollapsed] = useState(() => readFocus() || localStorage.getItem(storageKey) === "collapsed");
  const collapsed = !mobile && desktopCollapsed;
  const [width, setWidth] = useState(readWidth);
  const client = useQueryClient();
  const pinOrder = sanitizePinOrder(me.sidebar_pinned);
  const [customizing, setCustomizing] = useState(false);
  const unread = useQuery({ queryKey: [...notificationKey, "badge"], queryFn: () => listNotifications(1) }).data?.unread_count;
  const drag = useRef<{ x: number; width: number } | null>(null);
  const recent = useQuery({ queryKey: recentQuery(me.id), queryFn: () => readRecent(me.id), staleTime: Infinity }).data ?? [];

  function toggle() {
    localStorage.setItem(storageKey, collapsed ? "expanded" : "collapsed");
    setCollapsed(!collapsed);
  }
  function go(to: Parameters<typeof navigate>[0]["to"]) {
    onNavigate?.();
    void navigate({ to });
  }

  useEffect(() => {
    if (mobile) return;
    writeWidth(width);
  }, [mobile, width]);

  // View focus mode is temporary: leaving it restores the user's previous sidebar state.
  useEffect(() => {
    if (mobile) return;
    const apply = (event: Event) => setCollapsed(() => {
      const focused = (event as CustomEvent<boolean>).detail;
      return focused || localStorage.getItem(storageKey) === "collapsed";
    });
    window.addEventListener(FOCUS_EVENT, apply);
    return () => window.removeEventListener(FOCUS_EVENT, apply);
  }, [mobile, storageKey]);

  // Pintasan "[" menciutkan atau membuka sidebar, kecuali saat mengetik.
  useEffect(() => {
    if (mobile) return;
    function onKey(event: globalThis.KeyboardEvent) {
      if (event.key !== "[" || event.metaKey || event.ctrlKey || event.altKey) return;
      const target = event.target as HTMLElement | null;
      if (target && (target.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName))) return;
      toggle();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  // Hanya instance desktop yang memasang modal; drawer mobile memanggil instance ini lewat event.
  useEffect(() => {
    if (mobile) return;
    const open = () => setCustomizing(true);
    window.addEventListener("pms:open-customize-sidebar", open);
    return () => window.removeEventListener("pms:open-customize-sidebar", open);
  }, [mobile]);

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

  async function savePins(order: ShortcutId[]) {
    try {
      client.setQueryData(["me"], await api<CurrentUser>("/api/v1/me", { method: "PATCH", body: JSON.stringify(pinPatch(order, hidden)) }));
    } catch {
      toast.error(t("nav.customize.saveFailed"));
    }
  }

  const hidden = me.sidebar_hidden ?? [];
  const visible = (id: ShortcutId) => isShortcutVisible(id, hidden);
  const pinned = pinOrder.filter(visible);
  const more = SHORTCUT_IDS.filter((id) => id !== FIXED_SHORTCUT && !pinOrder.includes(id) && visible(id));
  const isCurrent = (to: string) => pathname === to || pathname.startsWith(`${to}/`);
  const shortcut = (id: ShortcutId) => {
    const to = SHORTCUT_ROUTE[id];
    return <Nav key={id} icon={SHORTCUT_ICON[id]} label={t(SHORTCUT_LABEL_KEY[id])} collapsed={collapsed}
      current={isCurrent(to)} onClick={() => go(to)} />;
  };

  const style = mobile ? undefined : { width: collapsed ? "var(--sidebar-width-collapsed)" : `${width}px` };
  return <>
    <aside style={{ ...style, ...sidebarThemeVars(sidebarTheme) }} className={`${SIDEBAR_THEME} group/sidebar relative flex min-h-0 flex-col border-r border-nav-hover bg-nav text-nav-text ${mobile ? "flex-1 p-3 pt-10" : `h-dvh shrink-0 ${collapsed ? "p-1" : "p-3"}`}`}>
      <div className={`mb-4 flex items-center gap-2 ${collapsed ? "flex-col" : ""}`}>
        <WorkspaceBrandMark compact />
        {!collapsed && <p className="min-w-0 flex-1 truncate font-semibold" title={me.account_name}>{me.account_name}</p>}
        {/* The toggle shows when the pointer is over the sidebar (or it has keyboard focus); collapsed, it always shows so the sidebar can be opened again. */}
        {!mobile && <Tooltip>
          <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={toggle} aria-expanded={!collapsed} aria-label={t(collapsed ? "nav.expandSidebar" : "nav.collapseSidebar")}
            className={`shrink-0 bg-transparent text-nav-muted hover:bg-nav-hover hover:text-nav-text aria-expanded:bg-transparent aria-expanded:text-nav-muted ${collapsed ? "" : "opacity-0 transition-opacity group-hover/sidebar:opacity-100 focus-visible:opacity-100"}`} />}>
            <PanelLeft />
          </TooltipTrigger>
          <TooltipContent>{t(collapsed ? "nav.expandSidebar" : "nav.collapseSidebar")}</TooltipContent>
        </Tooltip>}
      </div>
      {/* Hanya area ini yang bisa di-scroll; header dan footer (akun, pengaturan) tetap terlihat. */}
      <div className="sidebar-scroll-area flex min-h-0 flex-1 flex-col overflow-y-auto">
      <nav className="space-y-1" aria-label={t("nav.main.label")}>
        {shortcut(FIXED_SHORTCUT)}
        <Nav icon={<Inbox />} label={t("nav.inbox.label")} collapsed={collapsed} badge={unread} current={isCurrent("/inbox")} onClick={() => go("/inbox")} />
        <Nav icon={<Search />} label={t("nav.search.label")} collapsed={collapsed} hint={/Mac|iPhone|iPad/.test(navigator.platform) ? "⌘K" : "Ctrl K"} onClick={() => { onNavigate?.(); window.dispatchEvent(new Event("pms:open-search")); }} />
        {pinned.map(shortcut)}
        <DropdownMenu>
          <DropdownMenuTrigger render={<Button variant="ghost" aria-label={t("nav.more.label")}
            className={`flex-row items-center gap-3 whitespace-nowrap text-nav-text hover:bg-nav-hover hover:text-nav-text ${collapsed ? "w-full px-0" : "w-full justify-start px-3"}`} />}>
            <MoreHorizontal />{!collapsed && t("nav.more.label")}
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            {more.map((id) => <DropdownMenuItem key={id} onClick={() => go(SHORTCUT_ROUTE[id])}>{t(SHORTCUT_LABEL_KEY[id])}</DropdownMenuItem>)}
            {recent.length > 0 && <>
              <DropdownMenuSeparator />
              <DropdownMenuGroup>
                <DropdownMenuLabel>{t("nav.recent.label")}</DropdownMenuLabel>
                {recent.map((item) => <DropdownMenuItem key={item.id} onClick={() => {
                  rememberItem(client, me.id, item);
                  onNavigate?.();
                  void navigate({ to: "/items/$itemId", params: { itemId: item.id } });
                }}>{item.title}</DropdownMenuItem>)}
              </DropdownMenuGroup>
              <DropdownMenuSeparator />
            </>}
            <DropdownMenuItem onClick={() => openCustomizeSidebar()}>{t("nav.customize.open")}</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </nav>
      {!collapsed && <Favorites userId={me.id} />}
      {!collapsed && <SpacesNav me={me} />}
      </div>
      {/* The top bar already shows the inbox bell; the footer bell would be a second link with the same name. */}
      <SidebarFooter collapsed={collapsed} inbox={false} />
      {!mobile && !collapsed && <div role="separator" aria-orientation="vertical" aria-label={t("nav.resize")} aria-valuemin={SIDEBAR_WIDTH.min} aria-valuemax={SIDEBAR_WIDTH.max} aria-valuenow={width} tabIndex={0}
        onKeyDown={onHandleKey} onPointerDown={onHandleDown} onPointerMove={onHandleMove} onPointerUp={() => { drag.current = null; }}
        className="absolute inset-y-0 right-0 w-1 cursor-col-resize touch-none hover:bg-nav-hover focus-visible:bg-nav-active focus-visible:outline-none" />}
    </aside>
    {!mobile && <CustomizeSidebarDialog open={customizing} onOpenChange={setCustomizing} pinOrder={pinOrder} hidden={hidden} onSave={savePins} />}
  </>;
}

function Nav({ icon, label, onClick, collapsed = false, current = false, hint, badge }: { icon: ReactElement; label: string; onClick: () => void; collapsed?: boolean; current?: boolean; hint?: string; badge?: number }) {
  const state = current ? "bg-nav-active font-medium" : "hover:bg-nav-hover";
  return <button type="button" onClick={onClick} title={collapsed ? label : undefined} aria-label={collapsed ? label : undefined}
    aria-current={current ? "page" : undefined}
    className={`flex w-full flex-row items-center gap-3 whitespace-nowrap rounded-md py-2 text-sm ${collapsed ? "justify-center px-0" : "px-3"} ${state}`}>
    <span className="[&>svg]:size-4">{icon}</span>{!collapsed && label}
    {!collapsed && !!badge && <span className="ml-auto rounded-full bg-destructive px-1.5 text-[10px] font-medium text-destructive-foreground" aria-hidden="true">{badge}</span>}
    {!collapsed && hint && <kbd aria-hidden="true" className="ml-auto text-xs text-nav-muted">{hint}</kbd>}
  </button>;
}

function SpacesNav({ me }: { me: CurrentUser }) {
  const { t } = useTranslation();
  const [creating, setCreating] = useState(false);
  const [openSpaces, setOpenSpaces] = useState(() => readOpenSpaces(me.id));
  const [finding, setFinding] = useState(false);
  const [query, setQuery] = useState("");
  useEffect(() => {
    const open = () => setCreating(true);
    window.addEventListener("pms:open-space-create", open);
    return () => window.removeEventListener("pms:open-space-create", open);
  }, []);
  const spaces = useSpaces();
  const q = query.trim().toLowerCase();
  // Spaces are filtered by name only; items stay in the Space pages and the global search.
  const shownSpaces = spaces.data?.filter((s) => !q || (s.kind === "personal" ? t("spaces.personal") : s.name).toLowerCase().includes(q));

  // Up/Down walk every link and button of the tree; Left/Right (on the toggles) open and close branches.
  function walk(event: KeyboardEvent<HTMLElement>) {
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
    const all = [...event.currentTarget.querySelectorAll<HTMLElement>("a[href], button:not(:disabled)")];
    const at = all.indexOf(document.activeElement as HTMLElement);
    if (at < 0) return;
    event.preventDefault();
    all[Math.min(all.length - 1, Math.max(0, at + (event.key === "ArrowDown" ? 1 : -1)))]?.focus();
  }

  function toggleSpace(id: string) {
    const next = toggleOpen(openSpaces, id);
    setOpenSpaces(next);
    writeOpenSpaces(me.id, next);
  }

  return (
    <nav className="mt-5 flex flex-col" aria-label={t("spaces.nav.label")} onKeyDown={walk}>
      <div className="flex items-center justify-between px-3 pb-1">
        <h2 className="text-xs font-semibold uppercase tracking-wider text-nav-muted">{t("spaces.nav.label")}</h2>
        <div className="flex items-center">
          {me.capabilities.create_space && (
            <Button variant="ghost" size="icon-sm" aria-label={t("spaces.new")} title={t("spaces.new")} className="text-nav-text hover:bg-nav-hover hover:text-nav-text" onClick={() => setCreating(true)}><Plus /></Button>
          )}
          <Button variant="ghost" size="icon-sm" aria-label={t("spaces.find")} aria-expanded={finding} title={t("spaces.find")} className="text-nav-text hover:bg-nav-hover hover:text-nav-text"
            onClick={() => { setFinding(!finding); setQuery(""); }}><Search /></Button>
        </div>
      </div>
      {finding && <input type="search" autoFocus aria-label={t("spaces.find")} placeholder={t("spaces.find")} value={query} onChange={(event) => setQuery(event.target.value)}
        className="mx-3 mb-2 rounded-md border border-nav-hover bg-nav-hover px-2 py-1 text-sm text-nav-text placeholder:text-nav-muted outline-none focus-visible:ring-2 focus-visible:ring-nav-active" />}
      {spaces.isPending && <p role="status" className="px-3 py-2 text-sm text-nav-muted">{t("spaces.nav.loading")}</p>}
      {spaces.isError && (
        <div className="px-3 py-2 text-sm"><p role="alert">{t("spaces.nav.error")}</p><Button variant="link" size="sm" className="px-0 text-nav-text" onClick={() => spaces.refetch()}>{t("spaces.nav.retry")}</Button></div>
      )}
      <ul>
        {shownSpaces?.map((s) => (
          <SpaceNode key={s.id} space={s} name={s.kind === "personal" ? t("spaces.personal") : s.name}
            open={openSpaces.includes(s.id)} onToggle={() => toggleSpace(s.id)} />
        ))}
      </ul>
      <ul className="border-t border-nav-hover pt-1">
        <li><Link to="/shared-with-me" className={navRow} activeProps={activeNav}><Share2 className="size-4" aria-hidden />{t("nav.shared.label")}</Link></li>
        <li><Link to="/blueprints" className={navRow} activeProps={activeNav}><LayoutTemplate className="size-4" aria-hidden />{t("blueprints.nav")}</Link></li>
      </ul>
      <SpaceDialog open={creating} onOpenChange={setCreating} />
    </nav>
  );
}

/** Item berbintang milik pengguna, tampil di atas daftar Space. Kosong berarti seksi tidak ditampilkan. */
function Favorites({ userId }: { userId: string }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const stars = useStars();
  if (!stars.data?.length) return null;
  return <section className="mt-5" aria-labelledby="sidebar-favorites">
    <h2 id="sidebar-favorites" className="px-3 pb-1 text-xs font-semibold uppercase tracking-wider text-nav-muted">{t("nav.favorites.label")}</h2>
    <ul>
      {stars.data.map((item) => <li key={item.id}>
        <Link to="/items/$itemId" params={{ itemId: item.id }} className={navRow} activeProps={activeNav} title={item.title}
          onClick={() => rememberItem(client, userId, item)}>
          {itemIcon(item.type)}<span className="truncate">{item.title}</span>
        </Link>
      </li>)}
    </ul>
  </section>;
}

/** SB-01: sidebar disclosure buttons keep nav colours when expanded (the ghost variant otherwise paints aria-expanded light). */
const treeToggle = "shrink-0 text-nav-text hover:bg-nav-hover hover:text-nav-text aria-expanded:bg-transparent aria-expanded:text-nav-text";

function SpaceNode({ space, name, open, onToggle }: { space: Space; name: string; open: boolean; onToggle: () => void }) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [busy, setBusy] = useState(false);
  async function analytics() {
    setBusy(true);
    try {
      const board = await createAnalyticsBoard(space.id, t);
      await queryClient.invalidateQueries({ queryKey: ["dashboards"] });
      await navigate({ to: "/dashboards/$dashboardId", params: { dashboardId: board.id } });
    } catch (err) { toast.error((err as Error).message || t("dashboards.actionFailed")); }
    finally { setBusy(false); }
  }
  const pathname = useLocation({ select: (location) => location.pathname });
  const onTool = pathname.startsWith("/dashboards") || pathname.startsWith("/reports");
  const [toolsOpen, setToolsOpen] = useState(onTool);
  const [projectFilter, setProjectFilter] = useState("");
  // The Space whose page (or Settings, or its tools) is open gets a soft highlight, so the active context is visible in the sidebar.
  const inSpace = pathname === `/spaces/${space.id}` || pathname.startsWith(`/spaces/${space.id}/`);
  return <li>
    <div className={`group/row flex min-w-0 items-center rounded-md ${inSpace ? "bg-nav-hover/50" : ""}`}>
      <Button variant="ghost" size="icon-sm" className={treeToggle} aria-expanded={open}
        aria-label={t(open ? "nav.tree.collapse" : "nav.tree.expand", { name })}
        onClick={onToggle} onKeyDown={(event) => {
          if (event.key === "ArrowRight" || event.key === "ArrowLeft") {
            event.preventDefault();
            if (open !== (event.key === "ArrowRight")) onToggle();
          }
        }}>{open ? <ChevronDown /> : <ChevronRight />}</Button>
      {/* The Space name is the overview: it is the active row only on the Space's own page, not on its tools or items. */}
      <Link to="/spaces/$spaceId" params={{ spaceId: space.id }} className={`${linkClass} min-w-0 flex-1`} title={name} activeOptions={{ exact: true }} activeProps={activeLink}>
        <SpaceIcon iconKey={(space as SpaceType).icon_key} className="me-2 inline-block align-[-2px]" />{name}{(space as SpaceType).archived && <span className="ms-1 text-xs text-nav-muted">{t("nav.space.archived")}</span>}
      </Link>
      <Button variant="ghost" size="icon-sm" className={`shrink-0 text-nav-text hover:bg-nav-hover hover:text-nav-text ${revealOnHover}`}
        aria-label={t("nav.space.createIn", { name })} title={t("nav.space.createIn", { name })}
        onClick={() => window.dispatchEvent(new CustomEvent("pms:create-item", { detail: { kind: "project", spaceId: space.id } }))}><Plus /></Button>
      {/* One menu per Space row, built from effective capabilities: a viewer still gets Open, Copy link and Settings. */}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" className={`shrink-0 text-nav-muted hover:bg-nav-hover hover:text-nav-text ${revealOnHover}`}
          aria-label={t("nav.space.actionsFor", { name })} title={t("nav.space.actionsFor", { name })} />}><MoreHorizontal /></DropdownMenuTrigger>
        <DropdownMenuContent align="start" sideOffset={6} className="w-44">
          {spaceActions(space as SpaceType).map((action) => {
            const spaceSettings = (tab: "general" | "members") => () => void navigate({ to: SPACE_SETTINGS_ROUTE, params: { spaceId: space.id, tab } });
            switch (action) {
              case "open": return <DropdownMenuItem key={action} onClick={() => void navigate({ to: "/spaces/$spaceId", params: { spaceId: space.id } })}>{t("nav.space.menu.open")}</DropdownMenuItem>;
              case "copy-link": return <DropdownMenuItem key={action} onClick={() => void copySpaceLink(space.id, (key) => String(t(key)))}>{t("nav.space.menu.copyLink")}</DropdownMenuItem>;
              case "rename": return <DropdownMenuItem key={action} onClick={() => setEditing(true)}>{t("spaces.menu.rename")}</DropdownMenuItem>;
              case "members": return <DropdownMenuItem key={action} onClick={spaceSettings("members")}>{t("nav.space.menu.members")}</DropdownMenuItem>;
              case "settings": return <DropdownMenuItem key={action} onClick={spaceSettings("general")}>{t("spaces.spaceSettings.open")}</DropdownMenuItem>;
              case "archive": return <DropdownMenuItem key={action} onClick={spaceSettings("general")}>{t("nav.space.menu.archive")}</DropdownMenuItem>;
            }
          })}
        </DropdownMenuContent>
      </DropdownMenu>
      <SpaceDialog open={editing} onOpenChange={setEditing} space={space as SpaceType} />
    </div>
    {open && <div className="ml-3 space-y-2 border-l border-nav-hover pl-2 pb-2">
      <div>
        <button type="button" aria-expanded={toolsOpen} onClick={() => setToolsOpen(!toolsOpen)}
          className={`flex w-full items-center gap-1 rounded px-2 text-left text-xs font-semibold uppercase tracking-wider hover:text-nav-text focus:outline-none focus-visible:ring-2 focus-visible:ring-ring/50 ${toolsOpen ? "text-nav-text" : "text-nav-muted"}`}>
          {toolsOpen ? <ChevronDown aria-hidden="true" className="size-3" /> : <ChevronRight aria-hidden="true" className="size-3" />}{t("nav.space.tools")}
        </button>
        {toolsOpen && <>
          {/* Creating an analytics board is an action, so it sits as a + beside Dashboards rather than as another destination. */}
          <div className="group/dash flex items-center gap-1">
            <Link to="/dashboards" className={`${linkClass} min-w-0 flex-1 text-sm`} activeProps={activeLink}>{t("dashboards.nav.label")}</Link>
            {space.my_capabilities?.includes("create") && <Button variant="ghost" size="icon-sm" disabled={busy} className={`shrink-0 text-nav-text hover:bg-nav-hover hover:text-nav-text ${revealOnHover.replace(/row/g, "dash")}`}
              aria-label={t("dashboards.analytics.create")} title={t("dashboards.analytics.create")} onClick={() => void analytics()}><Plus /></Button>}
          </div>
          <Link to="/reports" className={`${linkClass} text-sm`} activeProps={activeLink}>{t("reports.nav.label")}</Link>
        </>}
      </div>
      {/* The Space's projects and folders sit directly under the Space; the create action is on the Space row. Filter narrows the top level only. */}
      <p className="px-2 pb-1 text-xs font-semibold uppercase tracking-wider text-nav-muted">{t("nav.space.projects")}</p>
      <input type="search" value={projectFilter} onChange={(event) => setProjectFilter(event.target.value)}
        aria-label={t("nav.space.filterProjects", { name })} placeholder={t("nav.space.filterProjectsPlaceholder")}
        className="mx-2 w-[calc(100%-1rem)] rounded border border-nav-hover bg-transparent px-2 py-1 text-xs text-nav-text placeholder:text-nav-muted" />
      <ItemTree id={space.id} space filter={projectFilter} />
    </div>}
  </li>;
}

function ItemTree({ id, space = false, onEmpty, filter = "" }: { id: string; space?: boolean; onEmpty?: () => void; filter?: string }) {
  const { t } = useTranslation();
  const query = useQuery({
    queryKey: [...(space ? itemKeys.root(id) : itemKeys.children(id)), "sidebar"],
    queryFn: () => listItems(space ? `/api/v1/spaces/${id}/items` : `/api/v1/items/${id}/children`, "manual", "project,folder"),
  });
  const hasBranches = query.data?.some((item: Item) => item.type !== "task");
  useEffect(() => {
    if (query.isSuccess && !hasBranches) onEmpty?.();
  }, [query.isSuccess, hasBranches, onEmpty]);
  return <ul>
    {query.isPending && <li className="p-1 text-xs" role="status">{t("items.loading")}</li>}
    {query.isError && <li><Button variant="link" size="sm" className="text-nav-text" onClick={() => void query.refetch()}>{t("items.retry")}</Button></li>}
    {query.data?.filter((item: Item) => item.type !== "task" && item.title.toLowerCase().includes(filter.trim().toLowerCase()))
      .map((item) => <Branch key={item.id} id={item.id} name={item.title} type={item.type} />)}
    {filter.trim() && query.isSuccess && !query.data?.some((item: Item) => item.type !== "task" && item.title.toLowerCase().includes(filter.trim().toLowerCase())) &&
      <li className="p-1 text-xs text-nav-muted">{t("nav.space.noProjectMatch")}</li>}
  </ul>;
}

function Branch({ id, name, type }: { id: string; name: string; type?: string }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const stars = useStars();
  const starred = !!stars.data?.some((item) => item.id === id);
  const [expanded, setExpanded] = useState(false);
  // Once expanded and found empty, the item has no children: the toggle goes away and the title keeps its indent.
  const [leaf, setLeaf] = useState(false);
  function openItem() {
    const userId = client.getQueryData<CurrentUser>(["me"])?.id;
    if (userId) rememberItem(client, userId, { id, title: name, type: type ?? "" });
  }
  async function toggleStar() {
    try {
      await api(`/api/v1/items/${id}/star`, { method: starred ? "DELETE" : "PUT" });
      await client.invalidateQueries({ queryKey: starKeys.all });
    } catch { toast.error(t("starred.failed")); }
  }
  return <li>
    <div className="group/row flex min-w-0 items-center">
      {leaf ? <span aria-hidden="true" className="size-7 shrink-0" /> : <Button variant="ghost" size="icon-sm" className={treeToggle} aria-expanded={expanded}
        aria-label={t(expanded ? "nav.tree.collapse" : "nav.tree.expand", { name })}
        onClick={() => setExpanded(!expanded)} onKeyDown={(event) => {
          if (event.key === "ArrowRight" || event.key === "ArrowLeft") {
            event.preventDefault();
            setExpanded(event.key === "ArrowRight");
          }
        }}>{expanded ? <ChevronDown /> : <ChevronRight />}</Button>}
      <Link to="/items/$itemId" params={{ itemId: id }} className={linkClass} title={name} aria-label={name} activeProps={activeLink} onClick={openItem}><span className="flex items-center gap-2">{type === "project" ? <Briefcase aria-hidden="true" className="size-4 shrink-0" /> : type === "folder" ? <Folder aria-hidden="true" className="size-4 shrink-0" /> : null}<span className="truncate">{name}</span></span></Link>
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" className={`${treeToggle} ${revealOnHover}`}
          aria-label={t("nav.item.actions", { name })} title={t("nav.item.actions", { name })} />}><MoreHorizontal /></DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onClick={() => void toggleStar()}>{t(starred ? "starred.remove" : "starred.add")}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
    {expanded && <div className="ml-3 border-l border-nav-hover pl-1"><ItemTree id={id} onEmpty={() => { setLeaf(true); setExpanded(false); }} /></div>}
  </li>;
}
