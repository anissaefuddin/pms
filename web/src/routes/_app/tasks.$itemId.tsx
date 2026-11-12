import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { CurrentUser } from "@/lib/api";
import { ItemDetail } from "@/components/item-detail";
import { getItem, itemKeys } from "@/lib/items";
import { ViewWorkspace } from "@/components/view-workspace";

export const Route = createFileRoute("/_app/tasks/$itemId")({ component: ItemPage });

function ItemPage() {
  return <ItemWorkspace itemId={Route.useParams().itemId} />;
}

export function ItemWorkspace({ itemId }: { itemId: string }) {
  const navigate = useNavigate();
  const me = useQueryClient().getQueryData<CurrentUser>(["me"]);
  const detail = useQuery({ queryKey: itemKeys.detail(itemId), queryFn: () => getItem(itemId), staleTime: Infinity });
  const item = detail.data?.item;
  const container = item && item.type !== "task" ? item : undefined;
  const canCreate = !!container?.my_capabilities.includes("create");
  const canEdit = !!container?.my_capabilities.includes("edit");
  if (!me) return null;
  return <main className="mx-auto w-full space-y-6 p-6">
    <ItemDetail key={itemId} id={itemId} me={me}
      onOpen={(id) => void navigate({ to: "/tasks/$itemId", params: { itemId: id } })}
      onDeleted={() => void navigate({ to: "/" })} />
    {container && <ViewWorkspace spaceId={container.locations[0]?.space_id ?? ""} parent={container} me={me}
      canCreate={canCreate} canEdit={canEdit} onOpen={(child) => void navigate({ to: "/tasks/$itemId", params: { itemId: child.id } })} />}
  </main>;
}
