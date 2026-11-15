import { useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, type CurrentUser } from "@/lib/api";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { ItemDetail } from "@/components/item-detail";
import { getItem, itemKeys, listItems, type Item } from "@/lib/items";
import type { Share } from "@/lib/spaces";
import { ViewWorkspace } from "@/components/view-workspace";
import { ProjectHeader } from "@/components/project-header";
import { CreateItemDialog } from "@/components/create-item-dialog";
import { ShareDialog } from "@/components/share-dialog";

export const Route = createFileRoute("/_app/tasks/$itemId")({ component: ItemPage });

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
  const [infoOpen, setInfoOpen] = useState(false);
  const [addingTask, setAddingTask] = useState(false);
  const [sharing, setSharing] = useState(false);
  if (!me) return null;
  if (detail.isError) return <main className="mx-auto w-full space-y-2 p-6"><FormError message={t("items.loadError")} /><Button variant="outline" onClick={() => void detail.refetch()}>{t("items.retry")}</Button></main>;
  return <main className="mx-auto w-full space-y-6 p-6">
    {isProject && item ? <>
      <ProjectHeader item={item} children={children.data ?? []} canCreate={canCreate} infoOpen={infoOpen}
        onToggleInfo={() => setInfoOpen((open) => !open)} onAddTask={() => setAddingTask(true)}
        onShare={canShare ? () => setSharing(true) : undefined} />
      {infoOpen && <section aria-label={t("views.project.info")} className="rounded-xl border bg-card p-4">
        <ItemDetail key={itemId} id={itemId} me={me}
          onOpen={(id) => void navigate({ to: "/tasks/$itemId", params: { itemId: id } })}
          onDeleted={() => void navigate({ to: "/" })} />
      </section>}
    </> : <ItemDetail key={itemId} id={itemId} me={me}
      onOpen={(id) => void navigate({ to: "/tasks/$itemId", params: { itemId: id } })}
      onDeleted={() => void navigate({ to: "/" })} />}
    {container && <ViewWorkspace spaceId={container.locations[0]?.space_id ?? ""} parent={container} me={me}
      canCreate={canCreate} canEdit={canEdit} onOpen={(child) => void navigate({ to: "/tasks/$itemId", params: { itemId: child.id } })} />}
    {isProject && item && <CreateItemDialog open={addingTask} onOpenChange={setAddingTask} taskOnly me={me}
      location={{ space_id: item.locations[0]?.space_id ?? "", parent_item_id: item.id }}
      onCreated={(created: Item) => void navigate({ to: "/tasks/$itemId", params: { itemId: created.id } })} />}
    {isProject && item && <ShareDialog
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
  </main>;
}
