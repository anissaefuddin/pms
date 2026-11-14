import { useEffect, useState } from "react";
import { Activity, BarChart3, Briefcase, ChevronDown, ChevronRight, ClipboardList, Folder, Inbox, LayoutDashboard, Menu, MoreHorizontal, PanelLeft, Plus, Search } from "lucide-react";
import { Link, useNavigate, useLocation } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { CurrentUser } from "@/lib/api";
import { itemKeys, listItems, type Item } from "@/lib/items";
import { useSpaces } from "@/lib/space-queries";
import { Button } from "./ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "./ui/dropdown-menu";
import { Dialog, DialogContent, DialogTitle, DialogTrigger } from "./ui/dialog";
import { SpaceDialog } from "./space-dialog";
import { CreateItemDialog } from "./create-item-dialog";
import { createAnalyticsBoard } from "@/lib/analytics-board";
import { toast } from "sonner";

const linkClass = "block min-w-0 flex-1 truncate rounded-md px-2 py-2 text-sm hover:bg-sidebar-accent";
const activeLink = { className: `${linkClass} bg-sidebar-accent font-medium`, "aria-current": "page" as const };

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
  const storageKey = `pms:sidebar:${me.id}`;
  const [desktopCollapsed, setCollapsed] = useState(() => localStorage.getItem(storageKey) === "collapsed");
  const collapsed = !mobile && desktopCollapsed;
  function toggle() {
    localStorage.setItem(storageKey, collapsed ? "expanded" : "collapsed");
    setCollapsed(!collapsed);
  }
  function go(to: Parameters<typeof navigate>[0]["to"]) {
    onNavigate?.();
    void navigate({ to });
  }

  const hidden = me.sidebar_hidden ?? [];
  const showRequests = !hidden.includes("request_forms"), showStream = !hidden.includes("stream"), showTodo = !hidden.includes("my_todo");
  return <aside className={`flex min-h-0 flex-col border-r border-border bg-sidebar text-sidebar-foreground ${mobile ? "flex-1 p-3 pt-10" : `h-dvh shrink-0 ${collapsed ? "w-14 p-1" : "w-[var(--sidebar-width)] p-3"}`}`}>
    <div className="mb-4 flex items-center justify-between gap-1">
      {!collapsed && <p className="min-w-0 truncate px-2 font-semibold text-foreground" title={me.account_name}>{me.account_name}</p>}
      {!mobile && <Button variant="ghost" size="icon" onClick={toggle} aria-expanded={!collapsed}
        aria-label={t(collapsed ? "nav.expandSidebar" : "nav.collapseSidebar")}><PanelLeft /></Button>}
    </div>
    <nav className="space-y-1" aria-label={t("nav.main.label")}>
      {!collapsed && <><Nav icon={<Search/>} label={t("nav.search.label")} hint={/Mac|iPhone|iPad/.test(navigator.platform) ? "⌘K" : "Ctrl K"} onClick={() => { onNavigate?.(); window.dispatchEvent(new Event("pms:open-search")); }}/><Nav icon={<Inbox/>} label={t("nav.inbox.label")} onClick={() => go("/inbox")}/>{showRequests && <Nav icon={<ClipboardList/>} label={t("nav.requests.label")} onClick={() => go("/requests")}/>}{showStream && <Nav icon={<Activity/>} label={t("nav.stream.label")} onClick={() => go("/stream")}/>}</>}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" aria-label={t("nav.more.label")}
          className={collapsed ? "w-full px-0" : "w-full justify-start px-3"} />}>
          <MoreHorizontal />{!collapsed && t("nav.more.label")}
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onClick={() => go("/portfolio")}>{t("nav.portfolio")}</DropdownMenuItem>
          {!hidden.includes("workload") && <DropdownMenuItem onClick={() => go("/workload")}>{t("nav.workload")}</DropdownMenuItem>}
          {!hidden.includes("timesheets") && <DropdownMenuItem onClick={() => go("/timesheets")}>{t("time.timesheet")}</DropdownMenuItem>}
          {!hidden.includes("starred") && <DropdownMenuItem onClick={() => go("/starred")}>{t("starred.nav")}</DropdownMenuItem>}
          {!hidden.includes("dashboards") && <DropdownMenuItem onClick={() => go("/dashboards")}>{t("dashboards.nav.label")}</DropdownMenuItem>}
          {!hidden.includes("reports") && <DropdownMenuItem onClick={() => go("/reports")}>{t("reports.nav.label")}</DropdownMenuItem>}
          {showTodo && <DropdownMenuItem onClick={() => go("/my-todo")}>{t("nav.todo.label")}</DropdownMenuItem>}
          {!hidden.includes("calendars") && <DropdownMenuItem onClick={() => go("/calendars")}>{t("nav.calendars.label")}</DropdownMenuItem>}
          {!hidden.includes("created_by_me") && <DropdownMenuItem onClick={() => go("/created-by-me")}>{t("nav.created.label")}</DropdownMenuItem>}
          <DropdownMenuItem onClick={() => go("/reviews")}>{t("items.fileReview.nav")}</DropdownMenuItem>
          <DropdownMenuItem onClick={() => go("/settings/profile")}>{t("nav.customize.label")}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </nav>
    {!collapsed && <SpacesNav me={me} />}
  </aside>;
}
function Nav({ icon, label, hint, onClick }: { icon: React.ReactElement<{className?: string}>; label: string; hint?: string; onClick: () => void }) { return <button onClick={onClick} className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm hover:bg-sidebar-accent"><span className="[&>svg]:size-4">{icon}</span>{label}{hint && <kbd aria-hidden="true" className="ml-auto text-xs text-muted-foreground">{hint}</kbd>}</button>; }

function SpacesNav({ me }: { me: CurrentUser }) {
  const { t } = useTranslation();
  const [creating, setCreating] = useState(false);
  const spaces = useSpaces();
  // Up/Down walk every link and button of the tree; Left/Right (on the toggles) open and close branches.
  function walk(event: React.KeyboardEvent<HTMLElement>) {
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
    const all = [...event.currentTarget.querySelectorAll<HTMLElement>("a[href], button:not(:disabled)")];
    const at = all.indexOf(document.activeElement as HTMLElement);
    if (at < 0) return;
    event.preventDefault();
    all[Math.min(all.length - 1, Math.max(0, at + (event.key === "ArrowDown" ? 1 : -1)))]?.focus();
  }
  return (
    <nav className="mt-5 min-h-0 flex-1 space-y-1 overflow-y-auto" aria-label={t("spaces.nav.label")} onKeyDown={walk}>
      <h2 className="px-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">{t("spaces.nav.label")}</h2>
      {spaces.isPending && <p role="status" className="px-3 py-2 text-sm text-muted-foreground">{t("spaces.nav.loading")}</p>}
      {spaces.isError && (
        <div className="px-3 py-2 text-sm"><p role="alert">{t("spaces.nav.error")}</p><Button variant="link" size="sm" className="px-0" onClick={() => spaces.refetch()}>{t("spaces.nav.retry")}</Button></div>
      )}
      <ul>
        {spaces.data?.map((s) => (
          <Branch key={s.id} id={s.id} space me={me} canCreate={!!s.my_capabilities?.includes("create")} name={s.kind === "personal" ? t("spaces.personal") : s.name} />
        ))}
      </ul>
      {me.capabilities.create_space && (
        <Button variant="ghost" size="sm" className="w-full justify-start gap-3 px-3" onClick={() => setCreating(true)}><Plus />{t("spaces.new")}</Button>
      )}
      <ul className="border-t pt-1">
        <li><Link to="/shared-with-me" className={linkClass} activeProps={activeLink}>{t("nav.shared.label")}</Link></li>
        <li><Link to="/blueprints" className={linkClass} activeProps={activeLink}>{t("blueprints.nav")}</Link></li>
      </ul>
      <SpaceDialog open={creating} onOpenChange={setCreating} />
    </nav>
  );
}

function ItemIcon({ type }: { type?: string }) {
  return type === "project" ? <Briefcase aria-hidden="true" className="size-4 shrink-0" /> : type === "folder" ? <Folder aria-hidden="true" className="size-4 shrink-0" /> : null;
}

function Branch({ id, name, type, space = false, me, canCreate = false }: { id: string; name: string; type?: string; space?: boolean; me?: CurrentUser; canCreate?: boolean }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [expanded, setExpanded] = useState(false);
  const [adding, setAdding] = useState(false);
  const [busy, setBusy] = useState(false);
  const query = useQuery({
    queryKey: [...(space ? itemKeys.root(id) : itemKeys.children(id)), "sidebar"],
    queryFn: () => listItems(space ? `/api/v1/spaces/${id}/items` : `/api/v1/items/${id}/children`, "manual", "project,folder"),
    enabled: expanded,
  });
  async function analytics() {
    setBusy(true);
    try {
      const board = await createAnalyticsBoard(id, t);
      await queryClient.invalidateQueries({ queryKey: ["dashboards"] });
      await navigate({ to: "/dashboards/$dashboardId", params: { dashboardId: board.id } });
    } catch (err) { toast.error((err as Error).message || t("dashboards.actionFailed")); }
    finally { setBusy(false); }
  }
  const children = query.data?.filter((item: Item) => item.type !== "task");
  const list = <>
    {query.isPending && <li className="p-1 text-xs" role="status">{t("items.loading")}</li>}
    {query.isError && <li><Button variant="link" size="sm" onClick={() => void query.refetch()}>{t("items.retry")}</Button></li>}
    {children?.map((item) => <Branch key={item.id} id={item.id} name={item.title} type={item.type} />)}
  </>;
  return <li>
    <div className="flex min-w-0 items-center">
      <Button variant="ghost" size="icon-sm" className="shrink-0" aria-expanded={expanded}
        aria-label={t(expanded ? "nav.tree.collapse" : "nav.tree.expand", { name })}
        onClick={() => setExpanded(!expanded)} onKeyDown={(event) => {
          if (event.key === "ArrowRight" || event.key === "ArrowLeft") {
            event.preventDefault();
            setExpanded(event.key === "ArrowRight");
          }
        }}>{expanded ? <ChevronDown /> : <ChevronRight />}</Button>
      {space ? <Link to="/spaces/$spaceId" params={{ spaceId: id }} className={linkClass} activeProps={activeLink}>{name}</Link>
        : <Link to="/items/$itemId" params={{ itemId: id }} className={`${linkClass} flex items-center gap-2`} activeProps={{ ...activeLink, className: `${activeLink.className} flex items-center gap-2` }}><ItemIcon type={type} /><span className="truncate">{name}</span></Link>}
    </div>
    {expanded && (space
      ? <div className="ml-3 border-l pl-1">
        <Link to="/spaces/$spaceId" params={{ spaceId: id }} className={linkClass} activeOptions={{ exact: true }}>{t("nav.space.overview")}</Link>
        <p className="px-2 pt-1 text-xs font-semibold text-muted-foreground">{t("nav.space.tools")}</p>
        <Link to="/dashboards" className={`${linkClass} flex items-center gap-2`}><LayoutDashboard aria-hidden="true" className="size-4" />{t("dashboards.nav.label")}</Link>
        {canCreate && <Button variant="ghost" size="sm" disabled={busy} className="w-full justify-start gap-2 px-2" onClick={() => void analytics()}><BarChart3 />{t("dashboards.analytics.create")}</Button>}
        <div className="flex items-center justify-between pt-1">
          <p className="px-2 text-xs font-semibold text-muted-foreground">{t("nav.space.projects")}</p>
          {canCreate && me && <Button variant="ghost" size="icon-sm" aria-label={t("nav.space.add", { name })} onClick={() => setAdding(true)}><Plus /></Button>}
        </div>
        <ul>{list}</ul>
        {me && <CreateItemDialog open={adding} onOpenChange={setAdding} me={me} location={{ space_id: id }}
          onCreated={(item) => { void queryClient.invalidateQueries({ queryKey: itemKeys.root(id) }); void navigate({ to: "/tasks/$itemId", params: { itemId: item.id } }); }} />}
      </div>
      : <ul className="ml-3 border-l pl-1">{list}</ul>)}
  </li>;
}
