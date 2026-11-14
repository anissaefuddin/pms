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
import type { Item } from "@/lib/items";
import { toast } from "sonner";
import { can, etag, sortSpaces, spaceKeys, type Share, type Space } from "@/lib/spaces";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { ShareDialog } from "@/components/share-dialog";
import { SpaceExcelImport } from "@/components/space-excel-import";
import { CustomFieldsDialog } from "@/components/custom-fields-dialog";
import { SpaceBookmarksDialog } from "@/components/space-bookmarks-dialog";
import { SpaceDialog } from "@/components/space-dialog";
import { Button, buttonVariants } from "@/components/ui/button";
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
  const [managingFields, setManagingFields] = useState(false);
  const [sharing, setSharing] = useState(false);
  const [bookmarks, setBookmarks] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const [pending, setPending] = useState(false);
  const [creating, setCreating] = useState(false);
  const items = useQuery({ queryKey: itemKeys.root(space.id), queryFn: () => listItems(`/api/v1/spaces/${space.id}/items`) });

  const shared = space.kind === "shared";
  const canShare = shared && can(space, "share");
  const canRename = shared && can(space, "edit");
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
    <main className="p-4 sm:p-8">
      <header className="flex flex-wrap items-start justify-between gap-4 border-b pb-5">
        <div className="min-w-0">
          <h1 className="truncate workspace-title">{personal ? t("spaces.personal") : space.name}</h1>
          {space.description && <p className="mt-1 max-w-2xl text-muted-foreground">{space.description}</p>}
          {space.locked && <p className="mt-1 flex items-center gap-1 text-sm text-muted-foreground"><Lock className="size-4" aria-hidden="true" />{t("spaces.lock.badge")}<span className="sr-only">. {t("spaces.lock.explain")}</span></p>}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {me && canCreate && <Button onClick={() => setCreating(true)}>{t("items.new")}</Button>}
          {canEditForms && <Link to="/spaces/$spaceId/forms" params={{ spaceId: space.id }} className={buttonVariants({ variant: "outline" })}>{t("forms.link")}</Link>}
          {canEditForms && <Link to="/spaces/$spaceId/automation" params={{ spaceId: space.id }} className={buttonVariants({ variant: "outline" })}>{t("automation.link")}</Link>}
          <Button variant="outline" onClick={() => setBookmarks(true)}>{t("spaces.bookmarks.link")}</Button>
          {canShare && <Button variant="outline" onClick={() => setSharing(true)}><Share2 />{t("spaces.share")}</Button>}
          {(canRename || canDelete || canManageFields || canCreate || canLock) && (
            <DropdownMenu>
              <DropdownMenuTrigger render={<Button variant="ghost" size="icon" aria-label={t("spaces.menu.label")} />}><MoreHorizontal /></DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-40">
                {canCreate && <DropdownMenuItem onClick={() => setImportingExcel(true)}>{t("excel.importTitle")}</DropdownMenuItem>}
                {canManageFields && <DropdownMenuItem onClick={() => setManagingFields(true)}>{t("spaces.customFields.title")}</DropdownMenuItem>}
                {canRename && <DropdownMenuItem onClick={() => setRenaming(true)}>{t("spaces.menu.rename")}</DropdownMenuItem>}
                {canLock && <DropdownMenuItem onClick={() => void setLocked(!space.locked)}>{space.locked ? t("spaces.lock.unlock") : t("spaces.lock.lock")}</DropdownMenuItem>}
                {canDelete && <DropdownMenuItem variant="destructive" onClick={() => { setDeleteError(""); setDeleting(true); }}>{t("spaces.menu.delete")}</DropdownMenuItem>}
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
      </header>
      {items.isPending && <p className="mt-6" role="status">{t("items.loading")}</p>}
      {items.isError && <p className="mt-6" role="alert">{t("items.loadError")}</p>}
      {items.data?.length === 0 && <section className="mt-8 rounded-xl border border-dashed bg-card p-12 text-center">
        <FolderOpen className="mx-auto size-8 text-muted-foreground" />
        <h2 className="mt-3 text-lg font-semibold">{t("spaces.empty.title")}</h2>
        <p className="mx-auto mt-1 max-w-md text-sm text-muted-foreground">{t("spaces.empty.description")}</p>
      </section>}
      <div className="mt-6"><ViewWorkspace spaceEmpty={items.data?.length === 0} spaceId={space.id} me={me} canCreate={canCreate} canEdit={can(space, "edit")} onOpen={openItem} /></div>

      {me && <CreateItemDialog open={creating} onOpenChange={setCreating} me={me} location={{ space_id: space.id }}
        onCreated={openItem} />}
      {importingExcel && <SpaceExcelImport open={importingExcel} onOpenChange={setImportingExcel} spaceId={space.id} />}
      {canManageFields && <CustomFieldsDialog open={managingFields} onOpenChange={setManagingFields} spaceId={space.id} />}
      <SpaceBookmarksDialog spaceId={space.id} canEdit={canEditForms} open={bookmarks} onOpenChange={setBookmarks} />
      <SpaceDialog open={renaming} onOpenChange={setRenaming} space={space} />
      {me && (
        <ShareDialog
          open={sharing}
          onOpenChange={setSharing}
          title={t("share.title", { name: space.name })}
          queryKey={spaceKeys.one(space.id)}
          loadShares={async () => {
            let etag = "";
            const { items } = await api<{ items: Share[] }>(`/api/v1/spaces/${space.id}/shares`, undefined, (headers) => {
              etag = headers.get("ETag") ?? "";
            });
            if (!etag) throw new Error("Missing share ETag");
            return { items, etag };
          }}
          saveShares={(shares, etag) => api(`/api/v1/spaces/${space.id}/shares`, { method: "PUT", headers: { "If-Match": etag }, body: JSON.stringify({ shares }) })}
          canShare={canShare}
          me={me}
          onSaved={() => queryClient.invalidateQueries({ queryKey: spaceKeys.all })}
        />
      )}
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={t("spaces.delete.title")}
        description={t("spaces.delete.description", { name: space.name })}
        confirmLabel={t("spaces.delete.confirm")}
        cancelLabel={t("spaces.form.cancel")}
        onConfirm={remove}
        busy={pending}
        error={deleteError}
      />
    </main>
  );
}
