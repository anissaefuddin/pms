import { useMemo, useState } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useQueries } from "@tanstack/react-query";
import { MoreHorizontal } from "lucide-react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { blueprintKey, type Blueprint } from "@/lib/blueprints";
import { groupRows, filterRows, sortRows, type RegistryGroup, type RegistryRow, type RegistrySort } from "@/lib/blueprint-registry";
import { can, type Space } from "@/lib/spaces";
import { useSpaces } from "@/lib/space-queries";
import { formatDate, useProfile } from "@/lib/profile";
import { CreateBlueprintDialog } from "@/components/blueprint-create-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";

export const Route = createFileRoute("/_app/settings/blueprints")({ component: BlueprintRegistryPage });

const field = "h-8 rounded-md border bg-background px-2 text-sm";
const NA = "blueprints.registry.unavailable";

function BlueprintRegistryPage() {
  const { t } = useTranslation();
  const profile = useProfile();
  const spaces = useSpaces();
  const readable = spaces.data ?? [];
  // One list per Space the user can see; the API enforces what each list returns.
  const lists = useQueries({
    queries: readable.map((space) => ({
      queryKey: blueprintKey(space.id),
      queryFn: async () => (await api<{ items: Blueprint[] }>(`/api/v1/spaces/${space.id}/blueprints`)).items,
    })),
  });
  const failed = lists.some((result) => result.isError);
  const loading = spaces.isPending || lists.some((result) => result.isPending);

  const rows: RegistryRow[] = useMemo(() => readable.flatMap((space, i) =>
    (lists[i]?.data ?? []).map((bp) => ({
      id: bp.id, name: bp.name, item_type: bp.item_type, scope: bp.scope, space_id: bp.space_id, spaceName: space.name,
      item_count: bp.item_count, updated_at: bp.updated_at, canManage: can(space, "manage_blueprint"),
    }))), [readable, lists]);
  const spaceById = useMemo(() => new Map(readable.map((s) => [s.id, s])), [readable]);

  const [query, setQuery] = useState("");
  const [type, setType] = useState("all");
  const [scope, setScope] = useState("all");
  const [sort, setSort] = useState<RegistrySort>("updated_desc");
  const [group, setGroup] = useState<RegistryGroup>("scope");
  const [creating, setCreating] = useState(false);

  const visible = useMemo(() => sortRows(filterRows(rows, { query, type, scope }), sort), [rows, query, type, scope, sort]);
  const groups = useMemo(() => groupRows(visible, group), [visible, group]);
  const creatable = readable.filter((space: Space) => can(space, "create_blueprint"));

  function groupLabel(key: string) {
    if (group === "scope") return t(`blueprints.registry.scope.${key}`);
    if (group === "type") return t(`blueprints.type.${key}`);
    return spaceById.get(key)?.name ?? key;
  }

  return <section className="space-y-6" aria-labelledby="registry-title">
    <div className="flex flex-wrap items-start justify-between gap-4">
      <div className="space-y-1">
        <h2 id="registry-title" className="text-base font-semibold">{t("blueprints.registry.title")}</h2>
        <p className="text-sm text-muted-foreground">{t("blueprints.registry.description")}</p>
        <p className="text-xs text-muted-foreground">{t("blueprints.registry.notice")}</p>
      </div>
      {creatable.length > 0 && <Button onClick={() => setCreating(true)}>{t("blueprints.registry.create")}</Button>}
    </div>

    <div className="flex flex-wrap items-end gap-3">
      <label className="space-y-1"><span className="sr-only">{t("blueprints.registry.search")}</span>
        <Input type="search" placeholder={t("blueprints.registry.search")} value={query} onChange={(e) => setQuery(e.target.value)} className="w-64" />
      </label>
      <label className="space-y-1"><span className="text-xs text-muted-foreground">{t("blueprints.registry.filterType")}</span>
        <select className={field} value={type} onChange={(e) => setType(e.target.value)}>
          <option value="all">{t("blueprints.registry.allTypes")}</option>
          <option value="project">{t("blueprints.type.project")}</option>
          <option value="folder">{t("blueprints.type.folder")}</option>
          <option value="task">{t("blueprints.type.task")}</option>
        </select>
      </label>
      <label className="space-y-1"><span className="text-xs text-muted-foreground">{t("blueprints.registry.filterScope")}</span>
        <select className={field} value={scope} onChange={(e) => setScope(e.target.value)}>
          <option value="all">{t("blueprints.registry.allScopes")}</option>
          <option value="space">{t("blueprints.registry.scope.space")}</option>
          <option value="account">{t("blueprints.registry.scope.account")}</option>
        </select>
      </label>
      <label className="space-y-1"><span className="text-xs text-muted-foreground">{t("blueprints.registry.sort")}</span>
        <select className={field} value={sort} onChange={(e) => setSort(e.target.value as RegistrySort)}>
          <option value="updated_desc">{t("blueprints.registry.sortUpdatedDesc")}</option>
          <option value="updated_asc">{t("blueprints.registry.sortUpdatedAsc")}</option>
          <option value="name">{t("blueprints.registry.sortName")}</option>
        </select>
      </label>
      <label className="space-y-1"><span className="text-xs text-muted-foreground">{t("blueprints.registry.groupBy")}</span>
        <select className={field} value={group} onChange={(e) => setGroup(e.target.value as RegistryGroup)}>
          <option value="none">{t("blueprints.registry.groupNone")}</option>
          <option value="scope">{t("blueprints.registry.groupScope")}</option>
          <option value="space">{t("blueprints.registry.groupSpace")}</option>
          <option value="type">{t("blueprints.registry.groupType")}</option>
        </select>
      </label>
    </div>

    {loading && <p role="status" className="text-sm text-muted-foreground">{t("items.loading")}</p>}
    {failed && <p role="alert" className="text-sm">{t("blueprints.registry.partialError")}</p>}
    {!loading && rows.length === 0 && <div className="rounded-lg border p-6 text-center">
      <p className="font-medium">{t("blueprints.registry.emptyTitle")}</p>
      <p className="mt-1 text-sm text-muted-foreground">{t("blueprints.registry.emptyDescription")}</p>
    </div>}
    {!loading && rows.length > 0 && visible.length === 0 && <p className="text-sm text-muted-foreground">{t("blueprints.registry.noMatches")}</p>}

    {visible.length > 0 && <div className="overflow-x-auto">
      {groups.map((section) => <div key={section.key || "all"} className="mb-6 space-y-2">
        {group !== "none" && <h3 className="text-sm font-semibold">{groupLabel(section.key)} <span className="font-normal text-muted-foreground">({section.rows.length})</span></h3>}
        <table className="w-full min-w-[720px] text-sm">
          <thead><tr className="border-b text-left text-muted-foreground">
            <th className="py-2 font-medium">{t("blueprints.registry.columns.name")}</th>
            <th className="font-medium">{t("blueprints.registry.columns.type")}</th>
            <th className="font-medium">{t("blueprints.registry.columns.scope")}</th>
            <th className="font-medium">{t("blueprints.registry.columns.space")}</th>
            <th className="font-medium">{t("blueprints.registry.columns.items")}</th>
            <th className="font-medium">{t("blueprints.registry.columns.updated")}</th>
            <th className="w-12"><span className="sr-only">{t("blueprints.registry.columns.actions")}</span></th>
          </tr></thead>
          <tbody>{section.rows.map((row) => <tr key={row.id} className="border-b">
            <td className="py-2 font-medium">{row.name}</td>
            <td>{t(`blueprints.type.${row.item_type}`)}</td>
            <td>{t(`blueprints.registry.scope.${row.scope}`)}</td>
            <td>{row.spaceName}</td>
            <td>{t("blueprints.count", { count: row.item_count })}</td>
            <td>{formatDate(row.updated_at, profile)}</td>
            <td>
              <DropdownMenu>
                <DropdownMenuTrigger render={<Button variant="ghost" size="sm" aria-label={t("blueprints.registry.rowActions", { name: row.name })} />}><MoreHorizontal className="size-4" aria-hidden /></DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  {row.canManage
                    ? <DropdownMenuItem render={<Link to="/blueprints/$blueprintId" params={{ blueprintId: row.id }} />}>{t("blueprints.registry.actions.edit")}</DropdownMenuItem>
                    : <DropdownMenuItem disabled title={t("blueprints.registry.disabled.manage")}>{t("blueprints.registry.actions.edit")}</DropdownMenuItem>}
                  <DropdownMenuItem disabled title={t("blueprints.registry.disabled.duplicate")}>{t("blueprints.registry.actions.duplicate")}</DropdownMenuItem>
                  <DropdownMenuItem disabled title={t("blueprints.registry.disabled.archive")}>{t("blueprints.registry.actions.archive")}</DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </td>
          </tr>)}</tbody>
        </table>
      </div>)}
    </div>}

    {creating && <CreateBlueprintDialog key="registry-create" initial={{ name: "", type: "project" }} spaces={creatable}
      defaultSpaceId={creatable[0]?.id ?? ""} onClose={() => setCreating(false)} />}
  </section>;
}
