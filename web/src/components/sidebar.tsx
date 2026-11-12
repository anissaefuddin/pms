import { Bell, ChevronDown, Inbox, MoreHorizontal, Search, Settings } from "lucide-react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "./ui/dropdown-menu";

export function Sidebar({ account }: { account: string }) {
  const { t } = useTranslation();
  return <aside className="flex h-screen w-64 shrink-0 flex-col border-r border-border bg-sidebar p-3 text-sidebar-foreground">
    <div className="mb-5 px-2 py-2"><p className="text-xs uppercase tracking-widest text-muted-foreground">{t("common.app.name")}</p><p className="truncate font-semibold text-foreground">{account}</p></div>
    <nav className="space-y-1" aria-label={t("nav.main.label")}>
      <Nav icon={<Search/>} label={t("nav.search.label")}/><Nav icon={<Inbox/>} label={t("nav.inbox.label")}/><Nav icon={<MoreHorizontal/>} label={t("nav.more.label")}/>
    </nav>
    <div className="mt-auto flex items-center justify-between border-t border-border pt-3"><Link to="/settings" className="rounded p-2 hover:bg-sidebar-accent" aria-label={t("nav.settings.label")}><Settings className="size-5"/></Link><Bell className="size-5"/><DropdownMenu><DropdownMenuTrigger className="flex items-center gap-1 rounded p-2 hover:bg-sidebar-accent">{t("nav.help.label")}<ChevronDown className="size-4"/></DropdownMenuTrigger><DropdownMenuContent><DropdownMenuItem>{t("nav.help.shortcuts")}</DropdownMenuItem><DropdownMenuItem>{t("nav.help.documentation")}</DropdownMenuItem></DropdownMenuContent></DropdownMenu></div>
  </aside>;
}
function Nav({ icon, label }: { icon: React.ReactElement<{className?: string}>; label: string }) { return <button className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm hover:bg-sidebar-accent">{icon && <span className="[&>svg]:size-4">{icon}</span>}{label}</button>; }
