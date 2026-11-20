import { useState } from "react";
import { Link, createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { FolderOpen, Lock, MoreHorizontal, Share2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { ApiError, api, type CurrentUser } from "@/lib/api";
import { useSpace } from "@/lib/space-queries";
import { itemKeys, listItems } from "@/lib/items";
import { CreateItemDialog } from "@/components/create-item-dialog";
import { ViewWorkspace } from "@/components/view-workspace";
import { WorkspaceHeader } from "@/components/workspace-header";
import { WorkspaceLayout } from "@/components/workspace-layout";
import { LoadingState } from "@/components/ui/loading-state";
import { ErrorState } from "@/components/ui/error-state";
import type { Item } from "@/lib/items";
import { toast } from "sonner";
import { can, etag, sortSpaces, spaceKeys, type Share, type Space } from "@/lib/spaces";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { SpaceShareDialog } from "@/components/space-share-dialog";
import { SpaceIcon } from "@/components/space-icon";
import { SpaceExcelImport } from "@/components/space-excel-import";
import { SpaceExcelExport } from "@/components/project-excel-export";
import { CustomFieldsDialog } from "@/components/custom-fields-dialog";
import { CreateMenuDialog } from "@/components/create-menu-dialog";
import { SpaceBookmarksDialog } from "@/components/space-bookmarks-dialog";
import { SpaceDialog } from "@/components/space-dialog";
import { Button, buttonVariants } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ProjectIllustration } from "@/components/ui/empty-illustrations";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";

export const Route = createFileRoute("/_app/spaces/$spaceId")({ component: SpacePage });

function SpacePage() {
  const { t } = useTranslation();
  const { spaceId } = Route.useParams();
  const space = useSpace(spaceId);
  const queryClient = useQueryClient();
  const me = queryClient.getQueryData<CurrentUser>(["me"]);

  if (space.isPending) return <main className="p-8"><p role="status" className="text-sm text-muted-foreground">{t("spaces.page.loading")}</p></main>;
  // 404 covers "does not exist" and "no access" alike, so the screen never says which.
  if (space.error instanceof ApiError && space.error.status === 404) {
    return (
      <main className="grid place-items-center p-12 text-center">
        <div className="max-w-md space-y-3">
          <h1 className="workspace-title">{t("spaces.notFound.title")}</h1>
          <p className="text-muted-foreground">{t("spaces.notFound.description")}</p>
          {me && <Link to="/spaces/$spaceId" params={{ spaceId: me.personal_space_id }} className={buttonVariants({ variant: "outline" })}>{t("spaces.notFound.home")}</Link>}
        </div>
      </main>
    );
  }
  if (space.isError) {
    return <main className="p-8"><p role="alert">{t("spaces.page.error")}</p><Button variant="outline" className="mt-3" onClick={() => space.refetch()}>{t("spaces.page.retry")}</Button></main>;
  }
  return <SpaceView space={space.data} me={me} />;
}

function SpaceView({ space, me }: { space: Space; me?: CurrentUser }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [renaming, setRenaming] = useState(false);
  const [importingExcel, setImportingExcel] = useState(false);
  const [exportingSpace, setExportingSpace] = useState(false);
  const [managingFields, setManagingFields] = useState(false);
  const [sharing, setSharing] = useState(false);
  const [bookmarks, setBookmarks] = useState(false);
  const [menuConfig, setMenuConfig] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const [pending, setPending] = useState(false);
  const [creating, setCreating] = useState(false);
  const items = useQuery({ queryKey: itemKeys.root(space.id), queryFn: () => listItems(`/api/v1/spaces/${space.id}/items`) });
  const impact = useQuery({
    queryKey: ["space-delete-impact", space.id], enabled: deleting,
    queryFn: () => api<{ projects_and_folders: number; items_only_here: number; items_elsewhere: number; members: number; groups: number;
      automation_rules: number; request_forms: number; blueprints: number; tools: number }>(`/api/v1/spaces/${space.id}/delete-impact`),
  });

  const shared = space.kind === "shared";
  const canShare = shared && can(space, "share");
  const canRename = shared && !space.archived && can(space, "edit"); // same rule as the Space General tab
  const canDelete = shared && can(space, "delete");
  const canCreate = can(space, "create");
  const canLock = shared && me?.user_type === "admin" && can(space, "share");
  const canEditForms = can(space, "edit"); // also gates the Forms link
  const canManageFields = can(space, "share") && can(space, "delete");
  const personal = space.kind === "personal";
  function openItem(item: Item) {
    if (item.type === "task") void navigate({ to: "/spaces/$spaceId", params: { spaceId: space.id },
      search: (previous) => ({ ...previous, task: item.id }) });
    else void navigate({ to: "/tasks/$itemId", params: { itemId: item.id } });
  }

  async function setLocked(locked: boolean) {
    try {
      await api(`/api/v1/spaces/${space.id}`, { method: "PATCH", headers: { "If-Match": etag(space) }, body: JSON.stringify({ locked }) });
      toast.success(t(locked ? "spaces.lock.locked" : "spaces.lock.unlocked"));
      await queryClient.invalidateQueries({ queryKey: spaceKeys.all });
      await queryClient.invalidateQueries({ queryKey: spaceKeys.one(space.id) });
    } catch (e) { toast.error(e instanceof ApiError && e.status === 409 ? t("spaces.lock.conflict") : (e as Error).message); }
  }

  async function remove() {
    setPending(true); setDeleteError("");
    try {
      await api(`/api/v1/spaces/${space.id}`, { method: "DELETE" });
      const rest = sortSpaces(queryClient.getQueryData<Space[]>(spaceKeys.all) ?? []).filter((s) => s.id !== space.id);
      const next = rest[0]?.id ?? me?.personal_space_id;
      if (next) await navigate({ to: "/spaces/$spaceId", params: { spaceId: next } });
      queryClient.removeQueries({ queryKey: spaceKeys.one(space.id) });
      await queryClient.invalidateQueries({ queryKey: spaceKeys.all });
    } catch (e) {
      setDeleteError((e as Error).message);
    } finally {
      setPending(false);
    }
  }

  return (
    <WorkspaceLayout>
      <WorkspaceHeader titleInTopbar icon={<SpaceIcon iconKey={space.icon_key} className="size-5" />} title={personal ? t("spaces.personal") : space.name}
        meta={<>
          {space.description && <p className="mt-1 max-w-2xl text-muted-foreground">{space.description}</p>}
          {space.locked && <p className="mt-1 flex items-center gap-1 text-sm text-muted-foreground"><Lock className="size-4" aria-hidden="true" />{t("spaces.lock.badge")}<span className="sr-only">. {t("spaces.lock.explain")}</span></p>}
        </>}
        actions={<>
          {/* Same order as a Project or Folder: shared and configuration actions first, the primary create action last. */}
          {canShare && <Button variant="outline" onClick={() => setSharing(true)}><Share2 />{t("spaces.share")}</Button>}
          <Button variant="outline" onClick={() => setBookmarks(true)}>{t("spaces.bookmarks.link")}</Button>
          {canEditForms && <Link to="/spaces/$spaceId/settings/$tab" params={{ spaceId: space.id, tab: "general" }} className={buttonVariants({ variant: "outline" })}>{t("spaces.spaceSettings.open")}</Link>}
          {me && canCreate && <Button onClick={() => setCreating(true)}>{t("items.new")}</Button>}
          {(canRename || canDelete || canManageFields || canCreate || canLock || canEditForms) && (
            <DropdownMenu>
              <DropdownMenuTrigger render={<Button variant="ghost" size="icon" aria-label={t("spaces.menu.label")} />}><MoreHorizontal /></DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-40">
                {canCreate && <DropdownMenuItem onClick={() => setImportingExcel(true)}>{t("excel.importTitle")}</DropdownMenuItem>}
                <DropdownMenuItem onClick={() => setExportingSpace(true)}>{t("excel.exportSpaceTitle")}</DropdownMenuItem>
                {canEditForms && <DropdownMenuItem onClick={() => setMenuConfig(true)}>{t("spaces.createMenu.link")}</DropdownMenuItem>}
                {canManageFields && <DropdownMenuItem onClick={() => setManagingFields(true)}>{t("spaces.customFields.title")}</DropdownMenuItem>}
                {canRename && <DropdownMenuItem onClick={() => setRenaming(true)}>{t("spaces.menu.rename")}</DropdownMenuItem>}
                {canLock && <DropdownMenuItem onClick={() => void setLocked(!space.locked)}>{space.locked ? t("spaces.lock.unlock") : t("spaces.lock.lock")}</DropdownMenuItem>}
                {canDelete && <DropdownMenuItem variant="destructive" onClick={() => { setDeleteError(""); setDeleting(true); }}>{t("spaces.menu.delete")}</DropdownMenuItem>}
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </>} />
      {items.isPending && <LoadingState className="mt-6 text-sm text-muted-foreground" label={t("items.loading")} />}
      {items.isError && <ErrorState className="mt-6 space-y-2" message={t("items.loadError")} retryLabel={t("items.retry")} onRetry={() => void items.refetch()} />}
      {items.data?.length === 0 && <EmptyState className="mt-8 p-12" icon={<ProjectIllustration />} title={t("spaces.empty.title")}
        description={t("spaces.empty.description")} canCreate={canCreate}
        primaryAction={{ label: t("spaces.empty.newProject"), onClick: () => setCreating(true) }}
        secondaryAction={{ label: t("spaces.empty.browseBlueprints"), onClick: () => void navigate({ to: "/blueprints" }) }} />}
      <div className="mt-6"><ViewWorkspace spaceEmpty={items.data?.length === 0} spaceId={space.id} me={me} canCreate={canCreate} canEdit={can(space, "edit")} onOpen={openItem} /></div>

      {me && <CreateItemDialog open={creating} onOpenChange={setCreating} me={me} location={{ space_id: space.id }}
        onCreated={openItem} />}
      {importingExcel && <SpaceExcelImport open={importingExcel} onOpenChange={setImportingExcel} spaceId={space.id} />}
      {exportingSpace && <SpaceExcelExport open={exportingSpace} onOpenChange={setExportingSpace} spaceId={space.id} spaceName={space.name} />}
      {canManageFields && <CustomFieldsDialog open={managingFields} onOpenChange={setManagingFields} spaceId={space.id} />}
      {canEditForms && <CreateMenuDialog spaceId={space.id} open={menuConfig} onOpenChange={setMenuConfig} />}
      <SpaceBookmarksDialog spaceId={space.id} canEdit={canEditForms} open={bookmarks} onOpenChange={setBookmarks} />
      <SpaceDialog open={renaming} onOpenChange={setRenaming} space={space} />
      {me && <SpaceShareDialog open={sharing} onOpenChange={setSharing} space={space} me={me} canShare={canShare} />}
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={t("spaces.delete.title")}
        description={<>
          <span>{t("spaces.delete.description", { name: space.name })}</span>
          {impact.data && <ul className="mt-3 list-disc space-y-1 ps-5 text-sm">
            <li>{t("spaces.spaceSettings.delete.projects", { count: impact.data.projects_and_folders })}</li>
            <li>{t("spaces.spaceSettings.delete.onlyHere", { count: impact.data.items_only_here })}</li>
            <li>{t("spaces.spaceSettings.delete.elsewhere", { count: impact.data.items_elsewhere })}</li>
            <li>{t("spaces.spaceSettings.delete.people", { members: impact.data.members, groups: impact.data.groups })}</li>
            <li>{t("spaces.spaceSettings.delete.automation", { count: impact.data.automation_rules })}</li>
            <li>{t("spaces.spaceSettings.delete.forms", { count: impact.data.request_forms })}</li>
            <li>{t("spaces.spaceSettings.delete.blueprints", { count: impact.data.blueprints })}</li>
            <li>{t("spaces.spaceSettings.delete.tools", { count: impact.data.tools })}</li>
          </ul>}
          {impact.isError && <p role="status" className="mt-2 text-sm text-muted-foreground">{t("spaces.spaceSettings.delete.impactError")}</p>}
        </>}
        confirmLabel={t("spaces.delete.confirm")}
        cancelLabel={t("spaces.form.cancel")}
        onConfirm={remove}
        busy={pending}
        error={deleteError}
      />
    </WorkspaceLayout>
  );
}
