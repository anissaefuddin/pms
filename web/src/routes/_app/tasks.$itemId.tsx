import { useState } from "react";
import { Link, createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Info } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { CurrentUser } from "@/lib/api";
import { ItemDetail } from "@/components/item-detail";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { getItem, itemKeys } from "@/lib/items";
import { useSpaces } from "@/lib/space-queries";
import { FormError } from "@/components/auth/form-error";
import { ViewWorkspace } from "@/components/view-workspace";

export const Route = createFileRoute("/_app/tasks/$itemId")({ component: ItemPage });

function ItemPage() {
  return <ItemWorkspace itemId={Route.useParams().itemId} />;
}

/** The single place that mounts the item detail form: the "Info" panel of a project, or the page body of a task. */
function ProjectInfoPanel({ itemId, me }: { itemId: string; me: CurrentUser }) {
  const navigate = useNavigate();
  return <ItemDetail key={itemId} id={itemId} me={me}
    onOpen={(id) => void navigate({ to: "/tasks/$itemId", params: { itemId: id } })}
    onDeleted={() => void navigate({ to: "/" })} />;
}

export function ItemWorkspace({ itemId }: { itemId: string }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [info, setInfo] = useState(false);
  const me = useQueryClient().getQueryData<CurrentUser>(["me"]);
  const spaces = useSpaces();
  const detail = useQuery({ queryKey: itemKeys.detail(itemId), queryFn: () => getItem(itemId), staleTime: Infinity });
  const item = detail.data?.item;
  const container = item && item.type !== "task" ? item : undefined;
  const canCreate = !!container?.my_capabilities.includes("create");
  const canEdit = !!container?.my_capabilities.includes("edit");
  if (!me) return null;
  if (detail.isError) return <main className="mx-auto w-full space-y-2 p-6"><FormError message={t("items.loadError")} /><Button variant="outline" onClick={() => void detail.refetch()}>{t("items.retry")}</Button></main>;
  if (item && !container) return <main className="mx-auto w-full p-6"><ProjectInfoPanel itemId={itemId} me={me} /></main>;
  const spaceId = container?.locations[0]?.space_id ?? "";
  const space = spaces.data?.find((entry) => entry.id === spaceId);
  return <main className="mx-auto w-full space-y-4 p-6">
    {container ? <>
      <header className="space-y-1">
        {space && <nav aria-label={t("nav.breadcrumb")} className="text-sm text-muted-foreground">
          <Link to="/spaces/$spaceId" params={{ spaceId }} className="hover:underline">{space.kind === "personal" ? t("spaces.personal") : space.name}</Link></nav>}
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="workspace-title">{container.title}</h1>
          <Button variant="outline" size="sm" onClick={() => setInfo(true)}><Info />{t("items.info")}</Button>
        </div>
      </header>
      <Dialog open={info} onOpenChange={setInfo}>
        <DialogContent className="inset-y-0 right-0 left-auto w-full max-w-xl translate-none overflow-y-auto rounded-none sm:max-w-xl">
          <DialogTitle>{t("items.info")}</DialogTitle>
          {info && <ProjectInfoPanel itemId={itemId} me={me} />}
        </DialogContent>
      </Dialog>
      <ViewWorkspace spaceId={spaceId} parent={container} me={me}
        canCreate={canCreate} canEdit={canEdit} onOpen={(child) => void navigate({ to: "/tasks/$itemId", params: { itemId: child.id } })} />
    </> : <p role="status" className="text-sm text-muted-foreground">{t("items.loading")}</p>}
  </main>;
}
