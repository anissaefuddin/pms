import { useProfile, formatNumber } from "@/lib/profile";
import { useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { Bell, Blocks, Layers, Plus, Sliders, SquareCheck, Folder, FolderKanban, ClipboardList, UserPlus } from "lucide-react";
import { Link, useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { listNotifications, notificationKey } from "@/lib/notifications";
import { api } from "@/lib/api";
import { clearSessionState } from "@/lib/session-state";
import { nextIndex } from "@/lib/ui-kit";
import type { CreateKind } from "@/lib/create-item-validate";
import { toast } from "sonner";
import { FeedbackDialog } from "@/components/feedback-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

export function Topbar({ onCreate, live, search, navigation }: { onCreate?: (kind?: CreateKind) => void; live: boolean; search?: ReactNode; navigation?: ReactNode }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const unread = useQuery({ queryKey: [...notificationKey, "badge"], queryFn: () => listNotifications(1) }).data?.unread_count;
  return (
    <header className="flex h-12 shrink-0 items-center justify-end border-b px-2 sm:px-4">
      {navigation}
      <span role="status" data-live={live ? "true" : "false"} className="mr-auto text-xs text-muted-foreground">{!live && t("nav.realtime.reconnecting")}</span>
      {search}
      <CreateMenu onCreate={onCreate} />
      <Link to="/inbox" className="relative rounded p-2 hover:bg-accent" aria-label={unread ? t("nav.inbox.badge", { count: unread }) : t("nav.inbox.label")}>
        <Bell className="size-5" />
        {!!unread && <span className="absolute -top-1 right-0 rounded-full bg-primary px-1 text-[10px] text-primary-foreground">{formatNumber(unread, profile)}</span>}
      </Link>
    </header>
  );
}

/** Profile menu. It lives in the sidebar header (top right), not in the top bar. */
export function ProfileMenu() {
  const { t } = useTranslation();
  const profile = useProfile();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState(false);
  const fullName = profile ? `${profile.first_name} ${profile.last_name}`.trim() : "";
  const initials = `${profile?.first_name?.[0] ?? ""}${profile?.last_name?.[0] ?? ""}`.toUpperCase();
  async function logout() {
    await api("/api/v1/auth/logout", { method: "POST" }).catch(() => undefined);
    if (profile) clearSessionState(globalThis.localStorage, profile.id);
    queryClient.clear();
    location.assign("/login");
  }
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" size="icon" />}>
          <span className="sr-only">{t("nav.profile.label")}</span>
          <span aria-hidden="true" className="grid size-7 place-items-center rounded-md bg-primary text-xs font-semibold text-primary-foreground">{initials}</span>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-56">
          <div className="px-2 py-2"><p className="truncate text-sm font-semibold">{fullName}</p>
            {profile?.job_title && <p className="truncate text-xs text-muted-foreground">{profile.job_title}</p>}</div>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={() => navigate({ to: "/settings" })}>{t("nav.settings.label")}</DropdownMenuItem>
          <DropdownMenuItem onClick={() => navigate({ to: "/settings/profile", hash: "appearance" })}>{t("nav.account.appearance")}</DropdownMenuItem>
          <DropdownMenuItem onClick={() => navigate({ to: "/recycle-bin" })}>{t("nav.recycle.label")}</DropdownMenuItem>
          <DropdownMenuItem onClick={() => navigate({ to: "/settings/apps" })}>{t("nav.apps.label")}</DropdownMenuItem>
          <DropdownMenuItem onClick={() => toast.info(t("nav.help.shortcutsText"))}>{t("nav.help.label")}</DropdownMenuItem>
          <DropdownMenuItem onClick={() => setFeedback(true)}>{t("nav.feedback.menu")}</DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={logout}>{t("nav.profile.logout")}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <FeedbackDialog open={feedback} onOpenChange={setFeedback} />
    </>
  );
}

type CreateEntry = { id: string; label: string; description?: string; icon: ReactNode; run: () => void };

/** Menu "+ Create": kotak cari, entri sesuai izin, navigasi panah. Entri yang tidak diizinkan tidak dirender. */
function CreateMenu({ onCreate }: { onCreate?: (kind?: CreateKind) => void }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const itemRefs = useRef<(HTMLButtonElement | null)[]>([]);
  const hidden = profile?.sidebar_hidden ?? [];
  const capabilities = profile?.capabilities;
  const close = (run: () => void) => () => { setOpen(false); run(); };

  const sections: (CreateEntry | false | undefined)[][] = [
    onCreate ? [
      { id: "task", label: t("items.task"), description: t("items.typeDesc.task"), icon: <SquareCheck />, run: close(() => onCreate("task")) },
      { id: "project", label: t("items.project"), description: t("items.typeDesc.project"), icon: <FolderKanban />, run: close(() => onCreate("project")) },
      { id: "folder", label: t("items.folder"), description: t("items.typeDesc.folder"), icon: <Folder />, run: close(() => onCreate("folder")) },
    ] : [],
    [
      capabilities?.create_space && { id: "space", label: t("nav.create.space"), description: t("nav.create.spaceDesc"), icon: <Layers />, run: close(() => window.dispatchEvent(new Event("pms:open-space-create"))) },
      !hidden.includes("request_forms") && { id: "request", label: t("nav.create.request"), description: t("nav.create.requestDesc"), icon: <ClipboardList />, run: close(() => navigate({ to: "/requests" })) },
      capabilities?.manage_users && { id: "invite", label: t("nav.create.invite"), description: t("nav.create.inviteDesc"), icon: <UserPlus />, run: close(() => navigate({ to: "/settings/users" })) },
      onCreate && { id: "blueprint", label: t("nav.create.blueprint"), description: t("nav.create.blueprintDesc"), icon: <Blocks />, run: close(() => onCreate()) },
    ],
    [{ id: "customize", label: t("nav.create.customize"), icon: <Sliders />, run: close(() => window.dispatchEvent(new Event("pms:open-customize-sidebar"))) }],
  ];
  const q = query.trim().toLowerCase();
  const visible = sections
    .map((section) => section.filter((entry): entry is CreateEntry => !!entry && (!q || `${entry.label} ${entry.description ?? ""}`.toLowerCase().includes(q))))
    .filter((section) => section.length > 0);
  const flat = visible.flat();

  function onItemKeyDown(event: KeyboardEvent, index: number) {
    const next = nextIndex(index, flat.length, event.key);
    if (next === null) return;
    event.preventDefault();
    itemRefs.current[next]?.focus();
  }

  let index = -1;
  return (
    <Popover open={open} onOpenChange={(next) => { setOpen(next); if (!next) setQuery(""); }}>
      <PopoverTrigger render={<Button size="sm" className="mr-1" aria-haspopup="true" />}>
        <Plus />{t("nav.create.button")}
      </PopoverTrigger>
      <PopoverContent align="end" sideOffset={6} className="max-h-(--available-height) w-80 overflow-y-auto p-2">
        <Input type="search" autoFocus aria-label={t("nav.create.search")} placeholder={t("nav.create.search")} value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "ArrowDown") { event.preventDefault(); itemRefs.current[0]?.focus(); }
            if (event.key === "Enter" && flat[0]) { event.preventDefault(); flat[0].run(); }
          }} />
        {flat.length === 0 && <p role="status" className="px-2 py-3 text-sm text-muted-foreground">{t("nav.create.noMatch")}</p>}
        <div role="menu" aria-label={t("nav.create.button")} className="mt-2 space-y-1">
          {visible.map((section, s) => <div key={s}>
            {s > 0 && <div role="separator" className="my-1 h-px bg-border" />}
            {section.map((entry) => {
              index += 1;
              const at = index;
              return <button key={entry.id} type="button" role="menuitem" ref={(el) => { itemRefs.current[at] = el; }}
                onClick={entry.run} onKeyDown={(event) => onItemKeyDown(event, at)}
                className="flex w-full items-start gap-3 rounded-md px-2 py-1.5 text-left outline-none hover:bg-accent focus-visible:bg-accent">
                <span className="mt-0.5 text-muted-foreground [&_svg]:size-4" aria-hidden="true">{entry.icon}</span>
                <span className="min-w-0">
                  <span className="block text-sm font-medium text-foreground">{entry.label}</span>
                  {entry.description && <span className="block text-xs text-muted-foreground">{entry.description}</span>}
                </span>
              </button>;
            })}
          </div>)}
        </div>
      </PopoverContent>
    </Popover>
  );
}
