import { useEffect, useState } from "react";
import { Outlet, createFileRoute, useParams, useRouter, useRouterState } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import i18n, { isLanguage, setLanguage } from "@/i18n";
import { ApiError, api, type CurrentUser } from "@/lib/api";
import { MobileSidebar, Sidebar } from "@/components/sidebar";
import { SettingsSidebar } from "@/components/settings-sidebar";
import { SearchPalette } from "@/components/search-palette";
import { Topbar } from "@/components/topbar";
import { CreateItemDialog } from "@/components/create-item-dialog";
import type { CreateKind } from "@/lib/create-item-validate";
import type { Location } from "@/lib/items";
import { useSpaces } from "@/lib/space-queries";
import { lastSpaceKey } from "@/lib/session-state";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { ProfileContext, useAppearance } from "@/lib/profile";
import { useRealtime } from "@/lib/realtime";
import { ItemDetail } from "@/components/item-detail";
import { OnboardingWizard } from "@/components/onboarding-wizard";

export const Route = createFileRoute("/_app")({
  validateSearch: (search: Record<string, unknown>): { task?: string; view?: string; filter?: string; sort?: string; group?: string; field?: string; preset?: string; type?: string } => ({
    task: typeof search.task === "string" && /^[0-9a-f-]{36}$/i.test(search.task) ? search.task : undefined,
    view: typeof search.view === "string" && /^[0-9a-f-]{36}$/i.test(search.view) ? search.view : undefined,
    filter: typeof search.filter === "string" ? search.filter.slice(0, 120) : undefined,
    sort: typeof search.sort === "string" && (["manual", "title", "status", "start_date", "due_date", "priority", "predecessors"].includes(search.sort) || /^field:[0-9a-f-]{36}$/i.test(search.sort)) ? search.sort : undefined,
    preset: search.preset === "active" || search.preset === "mine" ? search.preset : undefined,
    field: typeof search.field === "string" && /^[0-9a-f-]{36}:.{0,200}$/is.test(search.field) ? search.field : undefined,
    type: typeof search.type === "string" && /^[a-z0-9_-]{1,64}$/.test(search.type) ? search.type : undefined,
    group: search.group === "type" || search.group === "status" || search.group === "assignee" ? search.group : undefined,
  }),
  component: AppShell,
});

function AppShell() {
  const inSettings = useRouterState({ select: (state) => state.location.pathname.startsWith("/settings") });
  const { t } = useTranslation();
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<CurrentUser>("/api/v1/me"),
    retry: (count, err) => !(err instanceof ApiError && err.status === 401) && count < 2,
  });
  useAppearance(me.data);
  // The saved profile language wins over the browser choice once signed in.
  const locale = me.data?.locale;
  useEffect(() => {
    if (isLanguage(locale) && locale !== i18n.resolvedLanguage) void setLanguage(locale);
  }, [locale]);
  const live = useRealtime(!!me.data && !me.isError);
  const router = useRouter();
  const { task } = Route.useSearch();
  const { spaceId, itemId } = useParams({ strict: false });
  const spaces = useSpaces(!!me.data);
  // Dibuka dari menu "+ Create" (kind) atau dari komponen lain lewat event pms:create-item (kind, spaceId, parentId).
  const [creating, setCreating] = useState<{ kind?: CreateKind; typeKey?: string; location?: Location } | null>(null);
  useEffect(() => {
    const open = (event: Event) => {
      const detail = (event as CustomEvent<{ kind?: CreateKind; spaceId?: string; parentId?: string }>).detail ?? {};
      setCreating({ kind: detail.kind, location: detail.spaceId ? { space_id: detail.spaceId, parent_item_id: detail.parentId } : undefined });
    };
    window.addEventListener("pms:create-item", open);
    return () => window.removeEventListener("pms:create-item", open);
  }, []);
  // On a Space page, Create (and Quick Access) follows that Space's own create permission; elsewhere any Space will do.
  const canCreate = spaceId
    ? !!spaces.data?.find((space) => space.id === spaceId)?.my_capabilities?.includes("create")
    : spaces.data?.some((space) => space.my_capabilities?.includes("create"));
  useEffect(() => {
    if (me.data && spaceId && spaces.data?.some((space) => space.id === spaceId)) {
      localStorage.setItem(lastSpaceKey(me.data.id), spaceId);
    }
  }, [me.data, spaceId, spaces.data]);
  function setTask(id?: string) {
    const url = new URL(window.location.href);
    if (id) url.searchParams.set("task", id);
    else url.searchParams.delete("task");
    const href = url.pathname + url.search + url.hash;
    if (id) router.history.push(href);
    else router.history.replace(href);
  }
  useEffect(() => {
    const expired = () => redirectToLogin(true);
    window.addEventListener("pms:session-expired", expired);
    return () => window.removeEventListener("pms:session-expired", expired);
  }, []);
  useEffect(() => {
    if (me.error instanceof ApiError && me.error.status === 401) {
      redirectToLogin(!!me.data || me.error.sessionExpired);
    }
  }, [me.data, me.error]);
  if (me.isError && !(me.error instanceof ApiError && me.error.status === 401)) {
    return <main className="p-6"><p role="alert">{me.error.message}</p>
      <button onClick={() => void me.refetch()}>{t("items.retry")}</button></main>;
  }
  if (!me.data) return <div className="grid min-h-screen place-items-center" role="status">{t("workspace.home.loading")}</div>;
  return (
    <ProfileContext.Provider value={me.data}>
    <div className="flex min-h-screen">
      <div className="sticky top-0 hidden self-start shrink-0 lg:block">{inSettings ? <SettingsSidebar me={me.data} /> : <Sidebar key={me.data.id} me={me.data} />}</div>
      <div className="flex min-w-0 flex-1 flex-col">
        {inSettings
          // Settings has its own page header (breadcrumb, mobile menu) and no top bar; the palette stays mounted for Ctrl+K.
          ? <SearchPalette spaceId={spaceId} containerId={itemId} hideTrigger />
          : <Topbar live={live} navigation={<MobileSidebar me={me.data} />} search={<SearchPalette spaceId={spaceId} containerId={itemId} hideTrigger />} onCreate={canCreate ? (kind, typeKey) => setCreating({ kind, typeKey }) : undefined} spaceId={spaceId} containerActive={!!spaceId || !!itemId} />}
        <div className="min-w-0 flex-1"><Outlet /></div>
      </div>
      {me.data.onboarding_pending && <OnboardingWizard me={me.data} />}
      <CreateItemDialog open={!!creating} onOpenChange={(open) => { if (!open) setCreating(null); }} me={me.data}
        initialKind={creating?.kind} initialTypeKey={creating?.typeKey} location={creating?.location ?? (spaceId ? { space_id: spaceId } : undefined)}
        onCreated={(item) => {
          if (item.type === "task") setTask(item.id);
          else void router.navigate({ to: "/tasks/$itemId", params: { itemId: item.id } });
        }} />
      <Dialog open={!!task} onOpenChange={(open) => { if (!open) setTask(); }}>
        <DialogContent className="inset-y-0 right-0 left-auto w-full max-w-xl translate-none overflow-y-auto rounded-none sm:max-w-xl">
          <DialogTitle>{t("items.detail")}</DialogTitle>
          {task && <ItemDetail key={task} id={task} me={me.data} onOpen={setTask} onDeleted={() => { const pageItemDeleted = task === itemId; setTask(); if (pageItemDeleted) void router.navigate({ to: "/" }); }}
            onFullDetails={() => { const id = task; setTask(); void router.navigate({ to: "/tasks/$itemId", params: { itemId: id } }); }} />}
        </DialogContent>
      </Dialog>
    </div>
    </ProfileContext.Provider>
  );
}


function redirectToLogin(expired: boolean) {
  const here = location.pathname + location.search + location.hash;
  const params = new URLSearchParams();
  if (here !== "/") params.set("next", here);
  if (expired) params.set("reason", "expired");
  location.replace(params.size ? `/login?${params}` : "/login");
}
