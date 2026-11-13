import { useState } from "react";
import { Bell, ChevronDown, ClipboardList, ChevronRight, Inbox, MoreHorizontal, PanelLeft, Plus, Search, Settings } from "lucide-react";
import { Link, useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { CurrentUser } from "@/lib/api";
import { itemKeys, listItems, type Item } from "@/lib/items";
import { useSpaces } from "@/lib/space-queries";
import { Button } from "./ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "./ui/dropdown-menu";
import { SpaceDialog } from "./space-dialog";

const linkClass = "block min-w-0 flex-1 truncate rounded-md px-2 py-2 text-sm hover:bg-sidebar-accent";
const activeLink = { className: `${linkClass} bg-sidebar-accent font-medium`, "aria-current": "page" as const };

export function Sidebar({ me }: { me: CurrentUser }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const storageKey = `pms:sidebar:${me.id}`;
  const [collapsed, setCollapsed] = useState(() => localStorage.getItem(storageKey) === "collapsed");
  function toggle() {
    localStorage.setItem(storageKey, collapsed ? "expanded" : "collapsed");
    setCollapsed(!collapsed);
  }
  return <aside className={`sticky top-0 flex h-dvh shrink-0 flex-col border-r border-border bg-sidebar text-sidebar-foreground ${collapsed ? "w-14 p-1" : "w-[248px] p-3"}`}>
    <div className="mb-4 flex items-center justify-between gap-1">
      {!collapsed && <p className="min-w-0 truncate px-2 font-semibold text-foreground" title={me.account_name}>{me.account_name}</p>}
      <Button variant="ghost" size="icon" onClick={toggle} aria-expanded={!collapsed}
        aria-label={t(collapsed ? "nav.expandSidebar" : "nav.collapseSidebar")}><PanelLeft /></Button>
    </div>
    <nav className="space-y-1" aria-label={t("nav.main.label")}>
      {!collapsed && <><Nav icon={<Search/>} label={t("nav.search.label")} onClick={() => window.dispatchEvent(new Event("pms:open-search"))}/><Nav icon={<Inbox/>} label={t("nav.inbox.label")} onClick={() => void navigate({ to: "/inbox" })}/><Nav icon={<ClipboardList/>} label={t("nav.requests.label")} onClick={() => void navigate({ to: "/requests" })}/></>}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" aria-label={t("nav.more.label")}
          className={collapsed ? "w-full px-0" : "w-full justify-start px-3"} />}>
          <MoreHorizontal />{!collapsed && t("nav.more.label")}
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onClick={() => void navigate({ to: "/my-todo" })}>{t("nav.todo.label")}</DropdownMenuItem>
          <DropdownMenuItem onClick={() => void navigate({ to: "/shared-with-me" })}>{t("nav.shared.label")}</DropdownMenuItem>
          <DropdownMenuItem onClick={() => void navigate({ to: "/recycle-bin" })}>{t("nav.recycle.label")}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </nav>
    {!collapsed && <SpacesNav me={me} />}
    <div className="mt-auto flex items-center justify-between border-t border-border pt-3"><Link to="/settings" className="rounded p-2 hover:bg-sidebar-accent" aria-label={t("nav.settings.label")}><Settings className="size-5"/></Link>{!collapsed && <><Bell className="size-5"/><DropdownMenu><DropdownMenuTrigger className="flex items-center gap-1 rounded p-2 hover:bg-sidebar-accent">{t("nav.help.label")}<ChevronDown className="size-4"/></DropdownMenuTrigger><DropdownMenuContent><DropdownMenuItem>{t("nav.help.shortcuts")}</DropdownMenuItem><DropdownMenuItem>{t("nav.help.documentation")}</DropdownMenuItem></DropdownMenuContent></DropdownMenu></>}</div>
  </aside>;
}
function Nav({ icon, label, onClick }: { icon: React.ReactElement<{className?: string}>; label: string; onClick: () => void }) { return <button onClick={onClick} className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm hover:bg-sidebar-accent">{icon && <span className="[&>svg]:size-4">{icon}</span>}{label}</button>; }

function SpacesNav({ me }: { me: CurrentUser }) {
  const { t } = useTranslation();
  const [creating, setCreating] = useState(false);
  const spaces = useSpaces();
  return (
    <nav className="mt-5 min-h-0 flex-1 space-y-1 overflow-y-auto" aria-label={t("spaces.nav.label")}>
      <h2 className="px-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">{t("spaces.nav.label")}</h2>
      {spaces.isPending && <p role="status" className="px-3 py-2 text-sm text-muted-foreground">{t("spaces.nav.loading")}</p>}
      {spaces.isError && (
        <div className="px-3 py-2 text-sm"><p role="alert">{t("spaces.nav.error")}</p><Button variant="link" size="sm" className="px-0" onClick={() => spaces.refetch()}>{t("spaces.nav.retry")}</Button></div>
      )}
      <ul>
        {spaces.data?.map((s) => (
          <Branch key={s.id} id={s.id} space name={s.kind === "personal" ? t("spaces.personal") : s.name} />
        ))}
      </ul>
      {me.capabilities.create_space && (
        <Button variant="ghost" size="sm" className="w-full justify-start gap-3 px-3" onClick={() => setCreating(true)}><Plus />{t("spaces.new")}</Button>
      )}
      <SpaceDialog open={creating} onOpenChange={setCreating} />
    </nav>
  );
}


function Branch({ id, name, space = false }: { id: string; name: string; space?: boolean }) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  const query = useQuery({
    queryKey: [...(space ? itemKeys.root(id) : itemKeys.children(id)), "sidebar"],
    queryFn: () => listItems(space ? `/api/v1/spaces/${id}/items` : `/api/v1/items/${id}/children`, "manual", "project,folder"),
    enabled: expanded,
  });
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
        : <Link to="/items/$itemId" params={{ itemId: id }} className={linkClass} activeProps={activeLink}>{name}</Link>}
    </div>
    {expanded && <ul className="ml-3 border-l pl-1">
      {query.isPending && <li className="p-1 text-xs" role="status">{t("items.loading")}</li>}
      {query.isError && <li><Button variant="link" size="sm" onClick={() => void query.refetch()}>{t("items.retry")}</Button></li>}
      {query.data?.filter((item: Item) => item.type !== "task").map((item) => <Branch key={item.id} id={item.id} name={item.title} />)}
    </ul>}
  </li>;
}
