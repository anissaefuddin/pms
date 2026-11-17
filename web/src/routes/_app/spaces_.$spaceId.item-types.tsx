import { Link, createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ItemTypesPanel } from "@/routes/_app/settings/item-types";
import { useSpace } from "@/lib/space-queries";
import { buttonVariants } from "@/components/ui/button";

export const Route = createFileRoute("/_app/spaces_/$spaceId/item-types")({ component: SpaceItemTypesPage });

function SpaceItemTypesPage() {
  const { t } = useTranslation();
  const { spaceId } = Route.useParams();
  const space = useSpace(spaceId);
  return <main className="space-y-5 p-8">
    <header className="flex flex-wrap items-start justify-between gap-4 border-b pb-5">
      <div>
        <Link to="/spaces/$spaceId" params={{ spaceId }} className="text-sm text-muted-foreground underline">{space.data?.name ?? ""}</Link>
        <h1 className="mt-1 workspace-title">{t("settings.itemTypes.spaceTitle")}</h1>
      </div>
      <Link to="/settings/item-types" className={buttonVariants({ variant: "ghost" })}>{t("settings.itemTypes.title")}</Link>
    </header>
    <ItemTypesPanel spaceId={spaceId} />
  </main>;
}
