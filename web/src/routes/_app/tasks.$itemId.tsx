import { useState } from "react";
import { Link, createFileRoute, useNavigate, useSearch } from "@tanstack/react-router";
import { useSpace } from "@/lib/space-queries";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, type CurrentUser } from "@/lib/api";
import { FormError } from "@/components/auth/form-error";
import { ItemDetail } from "@/components/item-detail";
import { Button } from "@/components/ui/button";
import { getItem, itemKeys, listItems, type Item } from "@/lib/items";
import type { Share } from "@/lib/spaces";
import { ViewWorkspace } from "@/components/view-workspace";
import { ProjectHeader } from "@/components/project-header";
import { CreateItemDialog } from "@/components/create-item-dialog";
import { ShareDialog } from "@/components/share-dialog";
import { WorkspaceLayout } from "@/components/workspace-layout";
import { ErrorState } from "@/components/ui/error-state";
import { WorkspaceHeader } from "@/components/workspace-header";
import { TypeIcon } from "@/components/type-icon";
import { Info, Plus, Share2 } from "lucide-react";

export const Route = createFileRoute("/_app/tasks/$itemId")({ component: ItemPage });

// F2: a folder shows where it lives, like a project does. Space name links back to the Space.
function FolderBreadcrumb({ spaceId, title }: { spaceId: string; title: string }) {
  const { t } = useTranslation();
  const space = useSpace(spaceId);
  return <nav aria-label={t("views.project.breadcrumb")}>
    <ol className="flex flex-wrap items-center gap-1 text-sm text-muted-foreground">
      <li>{space.data && <Link to="/spaces/$spaceId" params={{ spaceId }} className="hover:text-foreground hover:underline">{space.data.name}</Link>}</li>
      <li aria-hidden="true">/</li>
      <li aria-current="page" className="min-w-0 truncate text-foreground">{title}</li>
    </ol>
  </nav>;
}

function ItemPage() {
  return <ItemWorkspace itemId={Route.useParams().itemId} />;
}

export function ItemWorkspace({ itemId }: { itemId: string }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const me = qc.getQueryData<CurrentUser>(["me"]);
  const detail = useQuery({ queryKey: itemKeys.detail(itemId), queryFn: () => getItem(itemId), staleTime: Infinity });
  const item = detail.data?.item;
  const container = item && item.type !== "task" ? item : undefined;
  const isProject = item?.type === "project";
  const canCreate = !!container?.my_capabilities.includes("create");
  const canEdit = !!container?.my_capabilities.includes("edit");
  const canShare = !!container?.my_capabilities.includes("share");
  // The project header reads the same children list the Table view uses, so both share one cache entry.
  const children = useQuery({ queryKey: itemKeys.children(itemId), enabled: isProject,
    queryFn: () => listItems(`/api/v1/items/${itemId}/children`) });
  // WA-D01: Info opens the contextual detail panel (?task=) instead of an inline copy of the title, so the header owns the primary title.
  const search = useSearch({ strict: false }) as { task?: string };
  const infoOpen = search.task === itemId;
  function toggleInfo() {
    void navigate({ to: ".", search: (previous) => ({ ...previous, task: infoOpen ? undefined : itemId }) });
  }
  const [addingTask, setAddingTask] = useState(false);
  const [sharing, setSharing] = useState(false);
  // Tasks open in the slide-right panel (?task=); projects and folders open as pages.
  function openChild(child: Item) {
    if (child.type === "task") void navigate({ to: ".", search: (previous) => ({ ...previous, task: child.id }) });
    else void navigate({ to: "/tasks/$itemId", params: { itemId: child.id } });
  }
  if (!me) return null;
  if (detail.isError) return <WorkspaceLayout><ErrorState message={t("items.loadError")} retryLabel={t("items.retry")} onRetry={() => void detail.refetch()} /></WorkspaceLayout>;
  return <WorkspaceLayout className="space-y-6">
    {isProject && item ? <ProjectHeader item={item} children={children.data ?? []} canCreate={canCreate} infoOpen={infoOpen}
      onToggleInfo={toggleInfo} onAddTask={() => setAddingTask(true)}
      onShare={canShare ? () => setSharing(true) : undefined} />
    : item && container ? <WorkspaceHeader icon={<TypeIcon name="folder" />} title={item.title} starItemId={item.id}
      breadcrumb={<FolderBreadcrumb spaceId={item.locations[0]?.space_id ?? ""} title={item.title} />}
      actions={<>
        <Button variant="outline" aria-pressed={infoOpen} onClick={toggleInfo}><Info />{t("views.project.info")}</Button>
        {canShare && <Button variant="outline" onClick={() => setSharing(true)}><Share2 />{t("views.project.share")}</Button>}
        {canCreate && <Button onClick={() => setAddingTask(true)}><Plus />{t("items.new")}</Button>}
      </>} />
    : item ? <ItemDetail key={itemId} id={itemId} me={me}
      onOpen={(id) => void navigate({ to: "/tasks/$itemId", params: { itemId: id } })}
      onDeleted={() => void navigate({ to: "/" })} />
    : null}
    {container && <ViewWorkspace spaceId={container.locations[0]?.space_id ?? ""} parent={container} me={me}
      canCreate={canCreate} canEdit={canEdit} onOpen={openChild} />}
    {item && container && <CreateItemDialog open={addingTask} onOpenChange={setAddingTask} taskOnly={isProject} me={me}
      location={{ space_id: item.locations[0]?.space_id ?? "", parent_item_id: item.id }}
      onCreated={openChild} />}
    {item && container && <ShareDialog
      open={sharing}
      onOpenChange={setSharing}
      title={t("share.title", { name: item.title })}
      queryKey={itemKeys.detail(item.id)}
      loadShares={async () => {
        let etag = "";
        const { items } = await api<{ items: Share[] }>(`/api/v1/items/${item.id}/shares`, undefined, (headers) => {
          etag = headers.get("ETag") ?? "";
        });
        if (!etag) throw new Error("Missing share ETag");
        return { items, etag };
      }}
      saveShares={(shares, etag) => api(`/api/v1/items/${item.id}/shares`, { method: "PUT", headers: { "If-Match": etag }, body: JSON.stringify({ shares }) })}
      canShare={canShare}
      me={me}
      onSaved={() => qc.invalidateQueries({ queryKey: itemKeys.detail(item.id) })}
    />}
  </WorkspaceLayout>;
}
