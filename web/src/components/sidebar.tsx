import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent, type ReactElement } from "react";
import { Activity, BarChart3, Briefcase, Folder, ChevronDown, ChevronRight, Clock, ClipboardCheck, CalendarDays, ClipboardList, UserPen, FileText, Inbox, LayoutDashboard, ListChecks, Menu, MoreHorizontal, PanelLeft, Plus, Search, Settings, Star, Trash2, Users } from "lucide-react";
import { Link, useNavigate, useLocation } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { listNotifications, notificationKey } from "@/lib/notifications";
import { createAnalyticsBoard } from "@/lib/analytics-board";
import { api, type CurrentUser } from "@/lib/api";
import { itemKeys, listItems, type Item } from "@/lib/items";
import { useSpaces } from "@/lib/space-queries";
import type { Space } from "@/lib/spaces";
import {
  SHORTCUT_IDS, SHORTCUT_LABEL_KEY, SIDEBAR_THEME, SIDEBAR_WIDTH, clampWidth, isShortcutVisible,
  readOpenSpaces, readWidth, sanitizePinOrder, toggleOpen, writeOpenSpaces, writeWidth, type ShortcutId,
} from "@/lib/sidebar-prefs";
import { Button } from "./ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "./ui/dropdown-menu";
import { Dialog, DialogContent, DialogTitle, DialogTrigger } from "./ui/dialog";
import { SpaceDialog } from "./space-dialog";
import { CustomizeSidebarDialog, openCustomizeSidebar } from "./customize-sidebar-dialog";

const linkClass = "block min-w-0 flex-1 truncate rounded-md px-2 py-2 text-sm hover:bg-nav-hover";
const activeLink = { className: `${linkClass} bg-nav-active font-medium`, "aria-current": "page" as const };

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
  const { t } = useTranslation();
  const navigate = useNavigate();
  const pathname = useLocation({ select: (location) => location.pathname });
  const storageKey = `pms:sidebar:${me.id}`;
  const [desktopCollapsed, setCollapsed] = useState(() => localStorage.getItem(storageKey) === "collapsed");
  const collapsed = !mobile && desktopCollapsed;
  const [width, setWidth] = useState(readWidth);
  const client = useQueryClient();
  const pinOrder = sanitizePinOrder(me.sidebar_pinned);
  const [customizing, setCustomizing] = useState(false);
  const unread = useQuery({ queryKey: [...notificationKey, "badge"], queryFn: () => listNotifications(1) }).data?.unread_count;
  const drag = useRef<{ x: number; width: number } | null>(null);

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
      client.setQueryData(["me"], await api<CurrentUser>("/api/v1/me", { method: "PATCH", body: JSON.stringify({ sidebar_pinned: sanitizePinOrder(order) }) }));
    } catch {
      toast.error(t("nav.customize.saveFailed"));
    }
  }

  const hidden = me.sidebar_hidden ?? [];
  const visible = (id: ShortcutId) => isShortcutVisible(id, hidden);
  const pinned = pinOrder.filter(visible);
  const more = SHORTCUT_IDS.filter((id) => !pinOrder.includes(id) && visible(id));
  const isCurrent = (to: string) => pathname === to || pathname.startsWith(`${to}/`);
  const shortcut = (id: ShortcutId) => {
    const to = SHORTCUT_ROUTE[id];
    return <Nav key={id} icon={SHORTCUT_ICON[id]} label={t(SHORTCUT_LABEL_KEY[id])} collapsed={collapsed}
      current={isCurrent(to)} onClick={() => go(to)} />;
  };

  const style = mobile ? undefined : { width: collapsed ? "var(--sidebar-width-collapsed)" : `${width}px` };
  return <>
    <aside style={style} className={`${SIDEBAR_THEME} relative flex min-h-0 flex-col border-r border-border bg-nav text-nav-text ${mobile ? "flex-1 p-3 pt-10" : `h-dvh shrink-0 ${collapsed ? "p-1" : "p-3"}`}`}>
      <div className="mb-4 flex items-center justify-between gap-1">
        {!collapsed && <p className="min-w-0 truncate px-2 font-semibold" title={me.account_name}>{me.account_name}</p>}
        {!mobile && <Button variant="ghost" size="icon" onClick={toggle} aria-expanded={!collapsed}
          aria-label={t(collapsed ? "nav.expandSidebar" : "nav.collapseSidebar")} title={t(collapsed ? "nav.expandSidebar" : "nav.collapseSidebar")}
          className="text-nav-text hover:bg-nav-hover"><PanelLeft /></Button>}
      </div>
      <nav className="space-y-1" aria-label={t("nav.main.label")}>
        <Nav icon={<Search />} label={t("nav.search.label")} collapsed={collapsed} hint={/Mac|iPhone|iPad/.test(navigator.platform) ? "⌘K" : "Ctrl K"} onClick={() => { onNavigate?.(); window.dispatchEvent(new Event("pms:open-search")); }} />
        <Nav icon={<Inbox />} label={t("nav.inbox.label")} collapsed={collapsed} badge={unread} current={isCurrent("/inbox")} onClick={() => go("/inbox")} />
        {pinned.map(shortcut)}
        <DropdownMenu>
          <DropdownMenuTrigger render={<Button variant="ghost" aria-label={t("nav.more.label")}
            className={`text-nav-text hover:bg-nav-hover ${collapsed ? "w-full px-0" : "w-full justify-start px-3"}`} />}>
            <MoreHorizontal />{!collapsed && t("nav.more.label")}
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            {more.map((id) => <DropdownMenuItem key={id} onClick={() => go(SHORTCUT_ROUTE[id])}>{t(SHORTCUT_LABEL_KEY[id])}</DropdownMenuItem>)}
            <DropdownMenuItem onClick={() => openCustomizeSidebar()}>{t("nav.customize.open")}</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </nav>
      {!collapsed && <SpacesNav me={me} />}
      <div className="mt-auto space-y-1 border-t border-border pt-3">
        <Nav icon={<Settings />} label={t("nav.settings.label")} collapsed={collapsed} current={isCurrent("/settings")} onClick={() => go("/settings")} />
        {!collapsed && <p className="px-3 text-xs text-nav-muted">{t("nav.appVersion", { version: __APP_VERSION__ })}</p>}
      </div>
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
    className={`flex w-full items-center gap-3 rounded-md py-2 text-sm ${collapsed ? "justify-center px-0" : "px-3"} ${state}`}>
    <span className="[&>svg]:size-4">{icon}</span>{!collapsed && label}
    {!collapsed && !!badge && <span className="ml-auto rounded-full bg-destructive px-1.5 text-[10px] font-medium text-destructive-foreground" aria-hidden="true">{badge}</span>}
    {!collapsed && hint && <kbd aria-hidden="true" className="ml-auto text-xs text-nav-muted">{hint}</kbd>}
  </button>;
}

function SpacesNav({ me }: { me: CurrentUser }) {
  const { t } = useTranslation();
  const [creating, setCreating] = useState(false);
  const [openSpaces, setOpenSpaces] = useState(() => readOpenSpaces(me.id));
  useEffect(() => {
    const open = () => setCreating(true);
    window.addEventListener("pms:open-space-create", open);
    return () => window.removeEventListener("pms:open-space-create", open);
  }, []);
  const spaces = useSpaces();

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
    <nav className="mt-5 flex min-h-0 max-h-[42vh] flex-col overflow-y-auto" aria-label={t("spaces.nav.label")} onKeyDown={walk}>
      <div className="flex items-center justify-between px-3 pb-1">
        <h2 className="text-xs font-semibold uppercase tracking-wider text-nav-muted">{t("spaces.nav.label")}</h2>
        {me.capabilities.create_space && (
          <Button variant="ghost" size="icon-sm" aria-label={t("spaces.new")} title={t("spaces.new")} className="text-nav-text hover:bg-nav-hover" onClick={() => setCreating(true)}><Plus /></Button>
        )}
      </div>
      {spaces.isPending && <p role="status" className="px-3 py-2 text-sm text-nav-muted">{t("spaces.nav.loading")}</p>}
      {spaces.isError && (
        <div className="px-3 py-2 text-sm"><p role="alert">{t("spaces.nav.error")}</p><Button variant="link" size="sm" className="px-0" onClick={() => spaces.refetch()}>{t("spaces.nav.retry")}</Button></div>
      )}
      <ul>
        {spaces.data?.map((s) => (
          <SpaceNode key={s.id} space={s} name={s.kind === "personal" ? t("spaces.personal") : s.name}
            open={openSpaces.includes(s.id)} onToggle={() => toggleSpace(s.id)} />
        ))}
      </ul>
      <ul className="border-t border-border pt-1">
        <li><Link to="/shared-with-me" className={linkClass} activeProps={activeLink}>{t("nav.shared.label")}</Link></li>
        <li><Link to="/blueprints" className={linkClass} activeProps={activeLink}>{t("blueprints.nav")}</Link></li>
      </ul>
      <SpaceDialog open={creating} onOpenChange={setCreating} />
    </nav>
  );
}

function SpaceNode({ space, name, open, onToggle }: { space: Space; name: string; open: boolean; onToggle: () => void }) {
  const { t } = useTranslation();
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
  return <li>
    <div className="flex min-w-0 items-center">
      <Button variant="ghost" size="icon-sm" className="shrink-0 text-nav-text hover:bg-nav-hover" aria-expanded={open}
        aria-label={t(open ? "nav.tree.collapse" : "nav.tree.expand", { name })}
        onClick={onToggle} onKeyDown={(event) => {
          if (event.key === "ArrowRight" || event.key === "ArrowLeft") {
            event.preventDefault();
            if (open !== (event.key === "ArrowRight")) onToggle();
          }
        }}>{open ? <ChevronDown /> : <ChevronRight />}</Button>
      <Link to="/spaces/$spaceId" params={{ spaceId: space.id }} className={linkClass} activeProps={activeLink}>{name}</Link>
    </div>
    {open && <div className="ml-3 space-y-2 border-l border-border pl-2 pb-2">
      <Link to="/spaces/$spaceId" params={{ spaceId: space.id }} className={`${linkClass} text-sm`} activeOptions={{ exact: true }} activeProps={activeLink}>{t("nav.space.overview")}</Link>
      <div>
        <p className="px-2 text-xs font-semibold uppercase tracking-wider text-nav-muted">{t("nav.space.tools")}</p>
        <Link to="/dashboards" className={`${linkClass} text-sm`} activeProps={activeLink}>{t("dashboards.nav.label")}</Link>
        <Link to="/reports" className={`${linkClass} text-sm`} activeProps={activeLink}>{t("reports.nav.label")}</Link>
        {space.my_capabilities?.includes("create") && <Button variant="ghost" size="sm" disabled={busy} className="w-full justify-start gap-2 px-2 text-nav-text hover:bg-nav-hover" onClick={() => void analytics()}><BarChart3 />{t("dashboards.analytics.create")}</Button>}
      </div>
      <div>
        <div className="flex items-center justify-between pr-1">
          <p className="px-2 text-xs font-semibold uppercase tracking-wider text-nav-muted">{t("nav.space.projects")}</p>
          <Button variant="ghost" size="icon-sm" className="text-nav-text hover:bg-nav-hover" aria-label={t("nav.space.createIn", { name })} title={t("nav.space.createIn", { name })}
            onClick={() => window.dispatchEvent(new CustomEvent("pms:create-item", { detail: { kind: "project", spaceId: space.id } }))}><Plus /></Button>
        </div>
        <ItemTree id={space.id} space />
      </div>
    </div>}
  </li>;
}

function ItemTree({ id, space = false }: { id: string; space?: boolean }) {
  const { t } = useTranslation();
  const query = useQuery({
    queryKey: [...(space ? itemKeys.root(id) : itemKeys.children(id)), "sidebar"],
    queryFn: () => listItems(space ? `/api/v1/spaces/${id}/items` : `/api/v1/items/${id}/children`, "manual", "project,folder"),
  });
  return <ul>
    {query.isPending && <li className="p-1 text-xs" role="status">{t("items.loading")}</li>}
    {query.isError && <li><Button variant="link" size="sm" onClick={() => void query.refetch()}>{t("items.retry")}</Button></li>}
    {query.data?.filter((item: Item) => item.type !== "task").map((item) => <Branch key={item.id} id={item.id} name={item.title} type={item.type} />)}
  </ul>;
}

function Branch({ id, name, type }: { id: string; name: string; type?: string }) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  return <li>
    <div className="flex min-w-0 items-center">
      <Button variant="ghost" size="icon-sm" className="shrink-0 text-nav-text hover:bg-nav-hover" aria-expanded={expanded}
        aria-label={t(expanded ? "nav.tree.collapse" : "nav.tree.expand", { name })}
        onClick={() => setExpanded(!expanded)} onKeyDown={(event) => {
          if (event.key === "ArrowRight" || event.key === "ArrowLeft") {
            event.preventDefault();
            setExpanded(event.key === "ArrowRight");
          }
        }}>{expanded ? <ChevronDown /> : <ChevronRight />}</Button>
      <Link to="/items/$itemId" params={{ itemId: id }} className={linkClass} activeProps={activeLink}><span className="flex items-center gap-2">{type === "project" ? <Briefcase aria-hidden="true" className="size-4 shrink-0" /> : type === "folder" ? <Folder aria-hidden="true" className="size-4 shrink-0" /> : null}<span className="truncate">{name}</span></span></Link>
    </div>
    {expanded && <div className="ml-3 border-l border-border pl-1"><ItemTree id={id} /></div>}
  </li>;
}
