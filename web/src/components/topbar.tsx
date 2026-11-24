import { useProfile, formatNumber } from "@/lib/profile";
import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { Maximize2, Minimize2, MoreHorizontal, Shapes, Bell, Blocks, ChevronDown, CircleHelp, Languages, Layers, LogOut, MessageSquare, Palette, Plus, Settings, Sliders, SquareCheck, Folder, FolderKanban, ClipboardList, Trash2, UserPlus } from "lucide-react";
import { Link, useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { listNotifications, notificationKey } from "@/lib/notifications";
import { api, type CurrentUser } from "@/lib/api";
import { LANGUAGES, isLanguage, setLanguage, type Language } from "@/i18n";

import { clearSessionState } from "@/lib/session-state";
import { nextIndex } from "@/lib/ui-kit";
import { FOCUS_EVENT, readFocus, setFocus } from "@/lib/view-focus";
import { UserAvatar } from "@/components/user-avatar";
import type { CreateKind } from "@/lib/create-item-validate";
import { toast } from "sonner";
import { FeedbackDialog } from "@/components/feedback-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuItem, DropdownMenuLabel, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { useItemTypes } from "@/lib/item-types";
import { menuAllows, useCreateMenu } from "@/lib/create-menu";

// Each language is named in its own language, so it stays readable whichever UI language is active.
const LANGUAGE_NAMES: Record<Language, string> = { en: "English", id: "Bahasa Indonesia" };
const LANGUAGE_SHORT: Record<Language, string> = { en: "EN", id: "ID" };

export function Topbar({ onCreate, live, search, navigation, inSettings = false, spaceId, containerActive = false }: { onCreate?: (kind?: CreateKind, typeKey?: string) => void; live: boolean; search?: ReactNode; navigation?: ReactNode; inSettings?: boolean; spaceId?: string; containerActive?: boolean }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const unread = useQuery({ queryKey: [...notificationKey, "badge"], queryFn: () => listNotifications(1) }).data?.unread_count;
  return (
    <header className="flex h-12 shrink-0 items-center justify-end border-b px-2 sm:px-4">
      {navigation}
      {/* Page breadcrumbs are portaled here (see WorkspaceHeader), so the top bar holds the path instead of a separate row. */}
      <div id="topbar-breadcrumb" className="flex min-w-0 flex-1 items-center text-sm" />
      <span role="status" data-live={live ? "true" : "false"} className="shrink-0 text-xs text-muted-foreground">{!live && t("nav.realtime.reconnecting")}</span>
      {!inSettings && <FocusToggle />}
      {search}
      {/* D-CREATE: global Create only where no container is open; a Space, Folder, Project or Task has its own primary Create. */}
      {!inSettings && !containerActive && <CreateMenu onCreate={onCreate} spaceId={spaceId} />}
      {!inSettings && <Link to="/inbox" className="relative rounded p-2 hover:bg-accent" aria-label={unread ? t("nav.inbox.badge", { count: unread }) : t("nav.inbox.label")}>
        <Bell className="size-5" />
        {!!unread && <span className="absolute -top-1 right-0 rounded-full bg-primary px-1 text-[10px] text-primary-foreground">{formatNumber(unread, profile)}</span>}
      </Link>}
    </header>
  );
}

/** Focus mode for every page, the same kind of app-wide control as the sidebar toggle. */
function FocusToggle() {
  const { t } = useTranslation();
  const [focused, setFocused] = useState(readFocus);
  useEffect(() => {
    const sync = (event: Event) => setFocused((event as CustomEvent<boolean>).detail);
    window.addEventListener(FOCUS_EVENT, sync);
    return () => window.removeEventListener(FOCUS_EVENT, sync);
  }, []);
  return <Button size="sm" variant={focused ? "secondary" : "ghost"} aria-pressed={focused} onClick={() => setFocus(!focused)} className="shrink-0">
    {focused ? <Minimize2 /> : <Maximize2 />}{t(focused ? "views.focus.exit" : "views.focus.enter")}
  </Button>;
}

/** Sidebar footer: profile (name and menu) and the inbox bell. Collapsed, it is the avatar only. Shared by the Workspace and Settings sidebars. */
export function SidebarFooter({ collapsed = false, inbox = true }: { collapsed?: boolean; inbox?: boolean }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const unread = useQuery({ queryKey: [...notificationKey, "badge"], queryFn: () => listNotifications(1) }).data?.unread_count;
  return <div className={`mt-2 flex shrink-0 items-center gap-1 border-t border-nav-hover pt-2 ${collapsed ? "flex-col" : ""}`}>
    <ProfileMenu compact={collapsed} footer={!collapsed} />
    {!collapsed && inbox && <Link to="/inbox" className="relative shrink-0 rounded-md p-2 text-nav-muted hover:bg-nav-hover hover:text-nav-text"
      aria-label={unread ? t("nav.inbox.badge", { count: unread }) : t("nav.inbox.label")}>
      <Bell className="size-4" aria-hidden="true" />
      {!!unread && <span className="absolute -top-0.5 right-0 rounded-full bg-primary px-1 text-[10px] text-primary-foreground">{formatNumber(unread, profile)}</span>}
    </Link>}
  </div>;
}

/** Profile menu. It lives in the sidebar footer, not in the top bar. */
/** Profile control: avatar plus a dropdown indicator. Compact (collapsed sidebar) shows the avatar only. */
export function ProfileMenu({ compact = false, footer = false }: { compact?: boolean; footer?: boolean }) {
  const { t, i18n } = useTranslation();
  const profile = useProfile();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState(false);
  async function changeLanguage(language: Language) {
    if (language === i18n.resolvedLanguage) return;
    await setLanguage(language);
    try {
      const updated = await api<CurrentUser>("/api/v1/me", { method: "PATCH", body: JSON.stringify({ locale: language }) });
      queryClient.setQueryData(["me"], updated);
    } catch (err) {
      toast.error((err as Error).message);
    }
  }
  const fullName = profile ? `${profile.first_name} ${profile.last_name}`.trim() : "";
  async function logout() {
    await api("/api/v1/auth/logout", { method: "POST" }).catch(() => undefined);
    if (profile) clearSessionState(globalThis.localStorage, profile.id);
    queryClient.clear();
    location.assign("/login");
  }
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" className={footer ? "h-auto min-w-0 flex-1 justify-start gap-2 bg-transparent px-2 py-1.5 text-nav-text hover:bg-nav-hover hover:text-nav-text aria-expanded:bg-nav-hover" : compact ? "size-8 shrink-0 bg-transparent p-0 text-nav-text hover:bg-nav-hover hover:text-nav-text aria-expanded:bg-nav-hover" : "h-8 shrink-0 gap-1 bg-transparent px-1 text-nav-text hover:bg-nav-hover hover:text-nav-text aria-expanded:bg-nav-hover"} />}>
          <span className="sr-only">{t("nav.profile.label")}</span>
          <UserAvatar profile={profile} className="size-7 rounded-md text-xs" />
          {footer && <span className="min-w-0 flex-1 truncate text-start text-sm text-nav-text">{fullName}</span>}
          {footer ? <MoreHorizontal aria-hidden="true" className="size-4 shrink-0 text-nav-muted" /> : !compact && <ChevronDown aria-hidden="true" className="size-3.5 text-nav-muted" />}
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-64">
          <div className="flex items-center gap-3 px-2 py-2">
            <UserAvatar profile={profile} className="size-10 rounded-md text-sm" />
            <div className="min-w-0"><p className="truncate text-sm font-semibold">{fullName}</p>
              {profile?.job_title && <p className="truncate text-xs text-muted-foreground">{profile.job_title}</p>}
              {profile?.company && <p className="truncate text-xs text-muted-foreground">{profile.company}</p>}</div>
          </div>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem onClick={() => navigate({ to: "/settings" })}><Settings />{t("nav.settings.label")}</DropdownMenuItem>
            <DropdownMenuItem onClick={() => navigate({ to: "/settings/appearance" })}><Palette />{t("nav.account.appearance")}</DropdownMenuItem>
            {/* The quick switch below and the full page use the same setting: this link goes to the page that shows it all. */}
            <DropdownMenuItem onClick={() => navigate({ to: "/settings/language" })}><Languages />{t("settings.nav.language")}</DropdownMenuItem>
            <div className="flex items-center gap-2 px-1.5 py-1 text-sm">
              <Languages className="size-4 shrink-0" aria-hidden="true" />
              <span className="flex-1">{t("nav.profile.language")}</span>
              <DropdownMenuRadioGroup value={i18n.resolvedLanguage} onValueChange={(value) => { if (isLanguage(value)) void changeLanguage(value); }}
                className="flex gap-0.5 rounded-md border p-0.5">
                {LANGUAGES.map((language) => <DropdownMenuRadioItem key={language} value={language} aria-label={LANGUAGE_NAMES[language]}
                  className="justify-center rounded px-2 py-0.5 text-xs font-medium pr-2 data-checked:bg-primary data-checked:text-primary-foreground [&>[data-slot=dropdown-menu-radio-item-indicator]]:hidden">
                  {LANGUAGE_SHORT[language]}
                </DropdownMenuRadioItem>)}
              </DropdownMenuRadioGroup>
            </div>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem onClick={() => navigate({ to: "/recycle-bin" })}><Trash2 />{t("nav.recycle.label")}</DropdownMenuItem>
            <DropdownMenuItem onClick={() => navigate({ to: profile?.user_type === "admin" ? "/settings/apps" : "/settings/developer" })}><Blocks />{t("nav.apps.label")}</DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem onClick={() => toast.info(t("nav.help.shortcutsText"))}><CircleHelp />{t("nav.help.section")}</DropdownMenuItem>
            <DropdownMenuItem onClick={() => setFeedback(true)}><MessageSquare />{t("nav.feedback.menu")}</DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={logout} className="text-destructive focus:text-destructive"><LogOut />{t("nav.profile.logout")}</DropdownMenuItem>
          <p className="px-2 pt-2 text-[11px] text-muted-foreground">{t("nav.appVersion", { version: __APP_VERSION__ })}</p>
        </DropdownMenuContent>
      </DropdownMenu>
      <FeedbackDialog open={feedback} onOpenChange={setFeedback} />
    </>
  );
}

type CreateEntry = { id: string; label: string; description?: string; icon: ReactNode; run: () => void };

/** Menu "+ Create": kotak cari, entri sesuai izin, navigasi panah. Entri yang tidak diizinkan tidak dirender. */
function CreateMenu({ onCreate, spaceId }: { onCreate?: (kind?: CreateKind, typeKey?: string) => void; spaceId?: string }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [moreOpen, setMoreOpen] = useState(false);
  const itemTypes = useItemTypes(!!onCreate).data ?? [];
  // CIT-D05: Quick Access entries are eligible custom types that are active and usable here: Workspace types, and this Space's own.
  // The item dialog checks the destination again when the Space is chosen, so a shortcut never grants a choice it could not make.
  // In a Space, Quick Access and More follow that Space's create menu (SPC-14), so a type the Space hides is not offered here.
  const menu = useCreateMenu(spaceId ?? "", !!spaceId && !!onCreate).data;
  const eligible = itemTypes.filter((type) => !type.is_builtin && type.active && (!type.space_id || type.space_id === spaceId)
    && (!spaceId || menuAllows(menu, type.key)));
  const quick = eligible.filter((type) => type.quick_access);
  const more = eligible.filter((type) => !type.quick_access);
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
    onCreate ? quick.map((type) => ({ id: `type-${type.key}`, label: type.name, description: t("items.typeDesc.custom", { base: t(`items.${type.base}`) }), icon: <Shapes />,
      run: close(() => onCreate(type.base as CreateKind, type.key)) })) : [],
    onCreate && more.length > 0 ? [{ id: "more-types", label: t("nav.create.moreTypes"), description: t("nav.create.moreTypesDesc", { count: more.length }), icon: <Shapes />,
      run: close(() => setMoreOpen(true)) }] : [],
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
    <>
    <Dialog open={moreOpen} onOpenChange={setMoreOpen}>
      <DialogContent>
        <DialogTitle>{t("nav.create.moreTypes")}</DialogTitle>
        <DialogDescription>{t("nav.create.moreTypesHint")}</DialogDescription>
        <div className="space-y-1">{more.map((type) => <button key={type.key} type="button" className="block w-full rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent"
          onClick={() => { setMoreOpen(false); setOpen(false); onCreate?.(type.base as CreateKind, type.key); }}>{type.name}</button>)}</div>
      </DialogContent>
    </Dialog>
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
    </>
  );
}
