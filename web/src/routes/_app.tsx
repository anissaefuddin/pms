import { useEffect, useState } from "react";
import { Outlet, createFileRoute, useParams, useRouter } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, type CurrentUser } from "@/lib/api";
import { Sidebar } from "@/components/sidebar";
import { Topbar } from "@/components/topbar";
import { CreateItemDialog } from "@/components/create-item-dialog";
import { useSpaces } from "@/lib/space-queries";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { ItemDetail } from "@/components/item-detail";

export const Route = createFileRoute("/_app")({
  validateSearch: (search: Record<string, unknown>): { task?: string; view?: string; filter?: string; sort?: string; group?: string } => ({
    task: typeof search.task === "string" && /^[0-9a-f-]{36}$/i.test(search.task) ? search.task : undefined,
    view: typeof search.view === "string" && /^[0-9a-f-]{36}$/i.test(search.view) ? search.view : undefined,
    filter: typeof search.filter === "string" ? search.filter.slice(0, 120) : undefined,
    sort: typeof search.sort === "string" && ["manual", "title", "status", "start_date", "due_date"].includes(search.sort) ? search.sort : undefined,
    group: search.group === "type" || search.group === "status" ? search.group : undefined,
  }),
  component: AppShell,
});

function AppShell() {
  const { t } = useTranslation();
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<CurrentUser>("/api/v1/me") });
  const router = useRouter();
  const { task } = Route.useSearch();
  const { spaceId } = useParams({ strict: false });
  const spaces = useSpaces(!!me.data);
  const [creating, setCreating] = useState(false);
  const canCreate = spaces.data?.some((space) => space.my_capabilities?.includes("create"));
  function setTask(id?: string) {
    const url = new URL(window.location.href);
    if (id) url.searchParams.set("task", id);
    else url.searchParams.delete("task");
    const href = url.pathname + url.search + url.hash;
    if (id) router.history.push(href);
    else router.history.replace(href);
  }
  useEffect(() => {
    if (me.isError) {
      const here = location.pathname + location.search;
      location.replace(here === "/" ? "/login" : `/login?next=${encodeURIComponent(here)}`);
    }
  }, [me.isError]);
  if (!me.data) return <div className="grid min-h-screen place-items-center" role="status">{t("workspace.home.loading")}</div>;
  return (
    <div className="flex min-h-screen">
      <Sidebar account={`${me.data.first_name} ${me.data.last_name}`.trim()} />
      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar onCreate={canCreate ? () => setCreating(true) : undefined} />
        <div className="min-w-0 flex-1"><Outlet /></div>
      </div>
      <CreateItemDialog open={creating} onOpenChange={setCreating} me={me.data}
        location={spaceId ? { space_id: spaceId } : undefined}
        onCreated={(item) => {
          if (item.type === "task") setTask(item.id);
          else void router.navigate({ to: "/tasks/$itemId", params: { itemId: item.id } });
        }} />
      <Dialog open={!!task} onOpenChange={(open) => { if (!open) setTask(); }}>
        <DialogContent className="inset-y-0 right-0 left-auto w-full max-w-xl translate-none overflow-y-auto rounded-none sm:max-w-xl">
          <DialogTitle>{t("items.detail")}</DialogTitle>
          {task && <ItemDetail key={task} id={task} me={me.data} onOpen={setTask} onDeleted={() => setTask()} />}
        </DialogContent>
      </Dialog>
    </div>
  );
}
