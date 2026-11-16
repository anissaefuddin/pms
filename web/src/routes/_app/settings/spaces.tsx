import { useState } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { useSpaces } from "@/lib/space-queries";
import { can, type Space } from "@/lib/spaces";
import { Button } from "@/components/ui/button";
import { SpaceDialog } from "@/components/space-dialog";
import { AdminEmptyState, AdminPageHeader, AdminStatus } from "@/components/admin/admin-ui";
import { FormError } from "@/components/auth/form-error";

export const Route = createFileRoute("/_app/settings/spaces")({ component: SpacesManagement });

const EDIT_CAPABILITY = "edit" as const;

// Spaces management: a list of every Space you can see, with your access. Settings for one Space open from its row.
function SpacesManagement() {
  const { t } = useTranslation();
  const spaces = useSpaces();
  const [renaming, setRenaming] = useState<Space>();
  const list = spaces.data ?? [];
  return <section className="min-w-0 space-y-6">
    <AdminPageHeader description={t("settings.spaces.description")} />
    <FormError message={spaces.error?.message ?? ""} />
    {spaces.data && !list.length && <AdminEmptyState title={t("settings.spaces.emptyTitle")} description={t("settings.spaces.emptyDescription")} />}
    {!!list.length && <div className="overflow-x-auto rounded-lg border">
      <table className="w-full text-start text-sm">
        <caption className="sr-only">{t("settings.spacesManagement")}</caption>
        <thead className="bg-muted/60 text-xs font-medium text-foreground">
          <tr>
            <th scope="col" className="px-3 py-2">{t("settings.spaces.colName")}</th>
            <th scope="col" className="px-3 py-2">{t("settings.spaces.colType")}</th>
            <th scope="col" className="px-3 py-2">{t("settings.spaces.colAccess")}</th>
            <th scope="col" className="px-3 py-2"><span className="sr-only">{t("items.actions")}</span></th>
          </tr>
        </thead>
        <tbody className="divide-y">{list.map((space) => <tr key={space.id}>
          <th scope="row" className="px-3 py-2.5 text-start font-medium">
            <Link to="/spaces/$spaceId" params={{ spaceId: space.id }} className="underline-offset-2 hover:underline">{space.name}</Link>
          </th>
          <td className="px-3 py-2.5">{space.kind === "personal" ? t("spaces.personal") : t("settings.spaces.shared")}</td>
          <td className="px-3 py-2.5">
            <AdminStatus tone={can(space, EDIT_CAPABILITY) ? "success" : "neutral"} label={can(space, EDIT_CAPABILITY) ? t("settings.spaces.canEdit") : t("settings.spaces.viewOnly")} />
          </td>
          <td className="px-3 py-2.5 text-end">
            <span className="inline-flex gap-1">
              {can(space, EDIT_CAPABILITY) && <Button size="sm" variant="ghost" aria-label={t("settings.spaces.renameNamed", { name: space.name })} onClick={() => setRenaming(space)}>{t("settings.spaces.rename")}</Button>}
              <Button size="sm" variant="outline" nativeButton={false} render={<Link to="/spaces/$spaceId" params={{ spaceId: space.id }} />}>{t("settings.spaces.open")}</Button>
            </span>
          </td>
        </tr>)}</tbody>
      </table>
    </div>}
    {renaming && <SpaceDialog open={!!renaming} onOpenChange={(open) => { if (!open) setRenaming(undefined); }} space={renaming} />}
  </section>;
}
