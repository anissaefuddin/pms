import { useState } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useQueries } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { blueprintKey, type Blueprint } from "@/lib/blueprints";
import { can, type Space } from "@/lib/spaces";
import { useSpaces } from "@/lib/space-queries";
import { groupBlueprintsBySpace, suggestedBlueprints, type SuggestedBlueprint } from "@/lib/workspace-metrics";
import { CreateItemDialog } from "@/components/create-item-dialog";
import { CreateBlueprintDialog } from "@/components/blueprint-create-dialog";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { BlueprintIllustration } from "@/components/ui/empty-illustrations";
import { useProfile } from "@/lib/profile";

export const Route = createFileRoute("/_app/blueprints")({ component: BlueprintsPage });

const ALL = "all";

function BlueprintsPage() {
  const { t } = useTranslation();
  const me = useProfile();
  const spaces = useSpaces();
  const [picked, setPicked] = useState(ALL);
  const [creating, setCreating] = useState<{ name: string; type: SuggestedBlueprint["itemType"] } | null>(null);
  const [using, setUsing] = useState<string>();
  const writable = (spaces.data ?? []).filter((space) => can(space, "create_blueprint"));
  const manageable = new Set((spaces.data ?? []).filter((space) => can(space, "manage_blueprint")).map((space) => space.id));
  const usable = new Set((spaces.data ?? []).filter((space) => can(space, "use_blueprint")).map((space) => space.id));
  const shown = (spaces.data ?? []).filter((space) => picked === ALL || space.id === picked);
  // One query per Space with the same key as the create dialog, so both screens share the cache.
  const lists = useQueries({
    queries: (spaces.data ?? []).map((space) => ({
      queryKey: blueprintKey(space.id),
      queryFn: async () => (await api<{ items: Blueprint[] }>(`/api/v1/spaces/${space.id}/blueprints`)).items
        .map((bp) => ({ ...bp, space_id: bp.space_id ?? space.id })),
    })),
    combine: (results) => results.flatMap((result) => result.data ?? []),
  });
  const groups = groupBlueprintsBySpace(lists, shown);
  const total = groups.reduce((sum, group) => sum + group.items.length, 0);
  const canCreate = writable.length > 0;
  const defaultSpace = writable[0] ?? spaces.data?.[0];

  return <main className="space-y-6 p-4 sm:p-8">
    <header className="flex flex-wrap items-end justify-between gap-4 border-b pb-5">
      <div>
        <h1 className="workspace-title">{t("blueprints.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("blueprints.description")}</p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <label className="flex items-center gap-2 text-sm">{t("blueprints.space")}
          <select value={picked} onChange={(event) => setPicked(event.target.value)} className="h-8 rounded-md border bg-background px-2 text-sm">
            <option value={ALL}>{t("blueprints.allSpaces")}</option>
            {(spaces.data ?? []).map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
          </select>
        </label>
        {canCreate && <Button onClick={() => setCreating({ name: "", type: "project" })}>{t("blueprints.create")}</Button>}
      </div>
    </header>

    {spaces.isPending && <p role="status" className="text-sm text-muted-foreground">{t("items.loading")}</p>}
    {spaces.isError && <p role="alert">{t("items.loadError")}</p>}

    {canCreate && <section aria-labelledby="blueprint-suggested" className="space-y-3">
      <h2 id="blueprint-suggested" className="text-base font-semibold">{t("blueprints.suggested")}</h2>
      <p className="text-sm text-muted-foreground">{t("blueprints.suggestedHint")}</p>
      <ul className="grid gap-3 md:grid-cols-3">
        {suggestedBlueprints.map((preset) => <li key={preset.id} className="flex flex-col gap-3 rounded-xl border bg-card p-4">
          <h3 className="font-semibold">{preset.name}</h3>
          <p className="flex-1 text-sm text-muted-foreground">{preset.description}</p>
          <Button variant="outline" className="self-start" onClick={() => setCreating({ name: preset.name, type: preset.itemType })}>
            {t("blueprints.useSuggestion")}
          </Button>
        </li>)}
      </ul>
    </section>}

    {spaces.isSuccess && total === 0 && <EmptyState icon={<BlueprintIllustration />} title={t("blueprints.emptyTitle")}
      description={t("blueprints.emptyDescription")} canCreate={canCreate}
      primaryAction={{ label: t("blueprints.create"), onClick: () => setCreating({ name: "", type: "project" }) }} />}

    {groups.map((group) => group.items.length > 0 && <section key={group.spaceId} aria-labelledby={`bp-${group.spaceId}`} className="space-y-3">
      <h2 id={`bp-${group.spaceId}`} className="text-base font-semibold">{group.spaceName}</h2>
      <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {group.items.map((blueprint) => <li key={blueprint.id} className="flex flex-col gap-3 rounded-xl border bg-card p-4">
          <div className="min-w-0">
            <h3 className="truncate font-semibold">{blueprint.name}</h3>
            <p className="text-sm text-muted-foreground">{t(`blueprints.type.${blueprint.item_type}`)} · {t("blueprints.count", { count: blueprint.item_count })}</p>
          </div>
          {usable.has(group.spaceId) && <Button variant="outline" className="self-start" onClick={() => setUsing(group.spaceId)}>{t("blueprints.use")}</Button>}
          {manageable.has(group.spaceId) && <Link to="/blueprints/$blueprintId" params={{ blueprintId: blueprint.id }} className="text-sm underline">{t("blueprints.editor.open")}</Link>}
        </li>)}
      </ul>
    </section>)}

    {using && me && <CreateItemDialog open onOpenChange={(open) => { if (!open) setUsing(undefined); }} me={me}
      location={{ space_id: using }} onCreated={() => setUsing(undefined)} />}
    {creating && <CreateBlueprintDialog key={`${creating.name}-${creating.type}`} initial={creating} spaces={writable}
      defaultSpaceId={defaultSpace?.id ?? ""} onClose={() => setCreating(null)} />}
  </main>;
}
