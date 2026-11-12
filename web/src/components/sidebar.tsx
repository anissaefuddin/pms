import { useState } from "react";
import { Bell, ChevronDown, Inbox, MoreHorizontal, Plus, Search, Settings } from "lucide-react";
import { Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, type CurrentUser } from "@/lib/api";
import { useSpaces } from "@/lib/space-queries";
import { Button } from "./ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "./ui/dropdown-menu";
import { SpaceDialog } from "./space-dialog";

export function Sidebar({ account }: { account: string }) {
  const { t } = useTranslation();
  return <aside className="flex h-screen w-64 shrink-0 flex-col border-r border-border bg-sidebar p-3 text-sidebar-foreground">
    <div className="mb-5 px-2 py-2"><p className="text-xs uppercase tracking-widest text-muted-foreground">{t("common.app.name")}</p><p className="truncate font-semibold text-foreground">{account}</p></div>
    <nav className="space-y-1" aria-label={t("nav.main.label")}>
      <Nav icon={<Search/>} label={t("nav.search.label")}/><Nav icon={<Inbox/>} label={t("nav.inbox.label")}/><Nav icon={<MoreHorizontal/>} label={t("nav.more.label")}/>
    </nav>
    <SpacesNav />
    <div className="mt-auto flex items-center justify-between border-t border-border pt-3"><Link to="/settings" className="rounded p-2 hover:bg-sidebar-accent" aria-label={t("nav.settings.label")}><Settings className="size-5"/></Link><Bell className="size-5"/><DropdownMenu><DropdownMenuTrigger className="flex items-center gap-1 rounded p-2 hover:bg-sidebar-accent">{t("nav.help.label")}<ChevronDown className="size-4"/></DropdownMenuTrigger><DropdownMenuContent><DropdownMenuItem>{t("nav.help.shortcuts")}</DropdownMenuItem><DropdownMenuItem>{t("nav.help.documentation")}</DropdownMenuItem></DropdownMenuContent></DropdownMenu></div>
  </aside>;
}
function Nav({ icon, label }: { icon: React.ReactElement<{className?: string}>; label: string }) { return <button className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm hover:bg-sidebar-accent">{icon && <span className="[&>svg]:size-4">{icon}</span>}{label}</button>; }

function SpacesNav() {
  const { t } = useTranslation();
  const [creating, setCreating] = useState(false);
  const spaces = useSpaces();
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<CurrentUser>("/api/v1/me") });
  const linkClass = "block truncate rounded-md px-3 py-2 text-sm hover:bg-sidebar-accent";
  return (
    <nav className="mt-5 min-h-0 flex-1 space-y-1 overflow-y-auto" aria-label={t("spaces.nav.label")}>
      <h2 className="px-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">{t("spaces.nav.label")}</h2>
      {spaces.isPending && <p role="status" className="px-3 py-2 text-sm text-muted-foreground">{t("spaces.nav.loading")}</p>}
      {spaces.isError && (
        <div className="px-3 py-2 text-sm"><p role="alert">{t("spaces.nav.error")}</p><Button variant="link" size="sm" className="px-0" onClick={() => spaces.refetch()}>{t("spaces.nav.retry")}</Button></div>
      )}
      <ul>
        {spaces.data?.map((s) => (
          <li key={s.id}>
            <Link to="/spaces/$spaceId" params={{ spaceId: s.id }} className={linkClass} activeProps={{ className: `${linkClass} bg-sidebar-accent font-medium`, "aria-current": "page" }}>
              {s.kind === "personal" ? t("spaces.personal") : s.name}
            </Link>
          </li>
        ))}
      </ul>
      {me.data?.capabilities.create_space && (
        <Button variant="ghost" size="sm" className="w-full justify-start gap-3 px-3" onClick={() => setCreating(true)}><Plus />{t("spaces.new")}</Button>
      )}
      <SpaceDialog open={creating} onOpenChange={setCreating} />
    </nav>
  );
}
