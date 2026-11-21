import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { Link, createFileRoute } from "@tanstack/react-router";
import { MoreHorizontal } from "lucide-react";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { useTranslation } from "react-i18next";
import { can, spaceKeys, type Space } from "@/lib/spaces";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SpaceDialog } from "@/components/space-dialog";
import { AdminEmptyState, AdminPageHeader, AdminStatus } from "@/components/admin/admin-ui";
import { FormError } from "@/components/auth/form-error";
import { SpaceIcon } from "@/components/space-icon";

// Filters, sort, grouping and page live in the URL, so returning from a Space's settings restores the same view.
// Only non-default values are kept in the URL; a missing key means the default.
type RegistrySearch = { q?: string; status?: "all" | "active" | "archived"; visibility?: "all" | "public" | "private"; sort?: SortKey; group?: "none" | "visibility" | "status"; page?: number };
const SORT_KEYS: SortKey[] = ["name", "created", "activity", "projects"];
export const Route = createFileRoute("/_app/settings/spaces")({
  validateSearch: (raw: Record<string, unknown>): RegistrySearch => ({
    ...(typeof raw.q === "string" && raw.q ? { q: raw.q.slice(0, 100) } : {}),
    ...(raw.status === "active" || raw.status === "archived" ? { status: raw.status } : {}),
    ...(raw.visibility === "public" || raw.visibility === "private" ? { visibility: raw.visibility } : {}),
    ...(SORT_KEYS.find((key) => key === raw.sort) ? { sort: raw.sort as SortKey } : {}),
    ...(raw.group === "visibility" || raw.group === "status" ? { group: raw.group } : {}),
    ...(Number.isInteger(Number(raw.page)) && Number(raw.page) > 0 ? { page: Number(raw.page) } : {}),
  }),
  component: SpacesManagement,
});

const EDIT_CAPABILITY = "edit" as const;
const PAGE_SIZE = 25;
const COLUMN = { description: "description", created: "created", activity: "activity", tools: "tools" } as const;
const OPTIONAL_COLUMNS = Object.values(COLUMN) as OptionalColumn[];
type OptionalColumn = (typeof COLUMN)[keyof typeof COLUMN];
type Row = Space & { created_at: string; last_activity: string | null; admins: string[]; shares: number; projects: number; tools: number };
type SortKey = "name" | "created" | "activity" | "projects";

/**
 * Workspace registry of Spaces (Batch 3): what you can see or manage, with counts, filters, sort, grouping and row actions.
 * Only Spaces the server already lets you see are listed; private metadata never reaches people without access.
 */
function SpacesManagement() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const registry = useQuery({
    queryKey: ["spaces-registry"],
    queryFn: async () => (await api<{ items: Row[] }>("/api/v1/spaces/registry")).items,
  });
  const [renaming, setRenaming] = useState<Row>();
  const search = Route.useSearch();
  const query = search.q ?? "";
  const status = search.status ?? "all";
  const visibility = search.visibility ?? "all";
  const sortKey = search.sort ?? "name";
  const group = search.group ?? "none";
  const page = search.page ?? 0;
  const navigate = Route.useNavigate();
  const [columns, setColumns] = useState<Set<OptionalColumn>>(new Set<OptionalColumn>([COLUMN.created, COLUMN.activity]));
  function setParams(patch: Partial<RegistrySearch>) {
    void navigate({ search: (previous) => ({ ...previous, ...patch }), replace: true });
  }
  const all = registry.data ?? [];
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = all.filter((row) =>
      (!q || row.name.toLowerCase().includes(q)) &&
      (status === "all" || (status === "archived") === row.archived) &&
      (visibility === "all" || row.visibility === visibility));
    const key = (row: Row): string | number => sortKey === "name" ? row.name.toLowerCase()
      : sortKey === "created" ? row.created_at : sortKey === "activity" ? (row.last_activity ?? "") : row.projects;
    return [...list].sort((a, b) => {
      const x = key(a), y = key(b);
      return x < y ? (sortKey === "projects" ? 1 : -1) : x > y ? (sortKey === "projects" ? -1 : 1) : 0;
    });
  }, [all, query, status, visibility, sortKey]);
  const pages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const current = Math.min(page, pages - 1);
  const pageRows = filtered.slice(current * PAGE_SIZE, current * PAGE_SIZE + PAGE_SIZE);
  const groups = group === "none" ? [{ label: "", rows: pageRows }] : groupRows(pageRows, group, t);
  const filtering = !!query.trim() || status !== "all" || visibility !== "all";

  function toggleColumn(column: OptionalColumn) {
    setColumns((current) => { const next = new Set(current); if (next.has(column)) next.delete(column); else next.add(column); return next; });
  }
  async function copyLink(row: Row) {
    try {
      await navigator.clipboard.writeText(`${window.location.origin}/spaces/${row.id}`);
      toast.success(t("spaces.spaceSettings.registry.copied"));
    } catch { toast.error(t("spaces.spaceSettings.registry.copyFailed")); }
  }
  const colCount = 4 + columns.size;
  return <section className="min-w-0 space-y-6">
    <AdminPageHeader description={t("settings.spaces.description")} />
    <FormError message={registry.error?.message ?? ""} />
    {registry.isPending && <p role="status" className="text-sm text-muted-foreground">{t("spaces.spaceSettings.registry.loading")}</p>}
    {registry.data && !all.length && <AdminEmptyState title={t("settings.spaces.emptyTitle")} description={t("spaces.spaceSettings.registry.emptyDescription")} />}
    {!!all.length && <div className="flex flex-wrap items-end gap-3">
      <Input type="search" value={query} onChange={(event) => setParams({ q: event.target.value, page: 0 })} placeholder={t("settings.spaces.search")}
        aria-label={t("settings.spaces.search")} className="max-w-xs" />
      <Filter label={t("spaces.spaceSettings.registry.status")} value={status} onChange={(v) => setParams({ status: v as RegistrySearch["status"], page: 0 })}
        options={[["all", t("spaces.spaceSettings.registry.all")], ["active", t("spaces.spaceSettings.registry.active")], ["archived", t("spaces.spaceSettings.registry.archived")]]} />
      <Filter label={t("spaces.spaceSettings.lifecycle.visibility")} value={visibility} onChange={(v) => setParams({ visibility: v as RegistrySearch["visibility"], page: 0 })}
        options={[["all", t("spaces.spaceSettings.registry.all")], ["public", t("spaces.spaceSettings.registry.public")], ["private", t("spaces.spaceSettings.registry.private")]]} />
      <Filter label={t("spaces.spaceSettings.registry.sort")} value={sortKey} onChange={(v) => setParams({ sort: v as SortKey })}
        options={[["name", t("spaces.spaceSettings.registry.sortName")], ["created", t("spaces.spaceSettings.registry.sortCreated")], ["activity", t("spaces.spaceSettings.registry.sortActivity")], ["projects", t("spaces.spaceSettings.registry.sortProjects")]]} />
      <Filter label={t("spaces.spaceSettings.registry.group")} value={group} onChange={(v) => setParams({ group: v as RegistrySearch["group"] })}
        options={[["none", t("spaces.spaceSettings.registry.noGroup")], ["visibility", t("spaces.spaceSettings.lifecycle.visibility")], ["status", t("spaces.spaceSettings.registry.status")]]} />
      <fieldset className="flex flex-wrap gap-3 text-sm">
        <legend className="sr-only">{t("spaces.spaceSettings.registry.columns")}</legend>
        {OPTIONAL_COLUMNS.map((column) => <label key={column} className="flex items-center gap-1.5">
          <input type="checkbox" checked={columns.has(column)} onChange={() => toggleColumn(column)} />
          {t(`spaces.spaceSettings.registry.col.${column}`)}
        </label>)}
      </fieldset>
    </div>}
    {!!all.length && !filtered.length && <div role="status" className="space-y-2 text-sm">
      <p>{t("spaces.spaceSettings.registry.noMatches")}</p>
      {filtering && <Button variant="outline" size="sm" onClick={() => setParams({ q: "", status: "all", visibility: "all", page: 0 })}>{t("spaces.spaceSettings.registry.clear")}</Button>}
    </div>}
    {!!filtered.length && <div className="overflow-x-auto rounded-lg border">
      <table className="w-full text-start text-sm">
        <caption className="sr-only">{t("settings.spacesManagement")}</caption>
        <thead className="bg-muted/60 text-xs font-medium text-foreground">
          <tr>
            <th scope="col" className="px-3 py-2">{t("settings.spaces.colName")}</th>
            <th scope="col" className="px-3 py-2">{t("spaces.spaceSettings.registry.status")}</th>
            <th scope="col" className="px-3 py-2">{t("spaces.spaceSettings.lifecycle.visibility")}</th>
            <th scope="col" className="px-3 py-2">{t("spaces.spaceSettings.registry.admins")}</th>
            <th scope="col" className="px-3 py-2">{t("spaces.spaceSettings.registry.shares")}</th>
            <th scope="col" className="px-3 py-2">{t("spaces.spaceSettings.registry.projects")}</th>
            {columns.has(COLUMN.description) && <th scope="col" className="px-3 py-2">{t("spaces.spaceSettings.registry.col.description")}</th>}
            {columns.has(COLUMN.tools) && <th scope="col" className="px-3 py-2">{t("spaces.spaceSettings.registry.col.tools")}</th>}
            {columns.has(COLUMN.created) && <th scope="col" className="px-3 py-2">{t("spaces.spaceSettings.registry.col.created")}</th>}
            {columns.has(COLUMN.activity) && <th scope="col" className="px-3 py-2">{t("spaces.spaceSettings.registry.col.activity")}</th>}
            <th scope="col" className="px-3 py-2"><span className="sr-only">{t("items.actions")}</span></th>
          </tr>
        </thead>
        {groups.map((section) => <tbody key={section.label || "all"} className="divide-y border-t">
          {section.label && <tr><th colSpan={colCount} scope="rowgroup" className="bg-muted/30 px-3 py-1.5 text-start text-xs font-semibold">{section.label}</th></tr>}
          {section.rows.map((row) => {
            const canEdit = can(row, EDIT_CAPABILITY);
            return <tr key={row.id}>
              <th scope="row" className="px-3 py-2.5 text-start font-medium">
                <span className="inline-flex items-center gap-2">
                  <SpaceIcon iconKey={row.icon_key} className="text-muted-foreground" />
                  <Link to="/spaces/$spaceId" params={{ spaceId: row.id }} className="underline-offset-2 hover:underline">{row.name}</Link>
                </span>
              </th>
              <td className="px-3 py-2.5"><AdminStatus tone={row.archived ? "neutral" : "success"} label={row.archived ? t("spaces.spaceSettings.registry.archived") : t("spaces.spaceSettings.registry.active")} /></td>
              <td className="px-3 py-2.5">{row.visibility === "public" ? t("spaces.spaceSettings.registry.public") : t("spaces.spaceSettings.registry.private")}</td>
              <td className="px-3 py-2.5">{row.admins.length ? row.admins.join(", ") : t("spaces.spaceSettings.registry.noAdmin")}</td>
              <td className="px-3 py-2.5 tabular-nums">{row.shares}</td>
              <td className="px-3 py-2.5 tabular-nums">{row.projects}</td>
              {columns.has(COLUMN.description) && <td className="max-w-xs truncate px-3 py-2.5 text-muted-foreground">{row.description ?? ""}</td>}
              {columns.has(COLUMN.tools) && <td className="px-3 py-2.5 tabular-nums">{row.tools}</td>}
              {columns.has(COLUMN.created) && <td className="px-3 py-2.5">{formatDate(row.created_at)}</td>}
              {columns.has(COLUMN.activity) && <td className="px-3 py-2.5">{row.last_activity ? formatDate(row.last_activity) : t("spaces.spaceSettings.registry.noActivity")}</td>}
              <td className="px-3 py-2.5 text-end">
                <span className="inline-flex flex-wrap justify-end gap-1">
                  {/* One menu per row: the actions follow the Space's effective capabilities. */}
                  <DropdownMenu>
                    <DropdownMenuTrigger render={<Button variant="ghost" size="icon" aria-label={t("settings.spaces.actionsNamed", { name: row.name })} />}><MoreHorizontal /></DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-40">
                      <DropdownMenuItem onClick={() => void navigate({ to: "/spaces/$spaceId", params: { spaceId: row.id } })}>{t("settings.spaces.open")}</DropdownMenuItem>
                      <DropdownMenuItem onClick={() => void copyLink(row)}>{t("spaces.spaceSettings.registry.copyLink")}</DropdownMenuItem>
                      {canEdit && <DropdownMenuItem onClick={() => setRenaming(row)}>{t("settings.spaces.rename")}</DropdownMenuItem>}
                      {canEdit && <DropdownMenuItem onClick={() => void navigate({ to: "/settings/spaces/$spaceId/$tab", params: { spaceId: row.id, tab: "general" } })}>{t("spaces.spaceSettings.open")}</DropdownMenuItem>}
                    </DropdownMenuContent>
                  </DropdownMenu>
                </span>
              </td>
            </tr>;
          })}
        </tbody>)}
      </table>
    </div>}
    {pages > 1 && <nav aria-label={t("spaces.spaceSettings.registry.pages")} className="flex items-center justify-between gap-2 text-sm">
      <Button variant="outline" size="sm" disabled={current === 0} onClick={() => setParams({ page: current - 1 })}>{t("spaces.spaceSettings.registry.prev")}</Button>
      <span>{t("spaces.spaceSettings.registry.pageOf", { page: current + 1, pages })}</span>
      <Button variant="outline" size="sm" disabled={current >= pages - 1} onClick={() => setParams({ page: current + 1 })}>{t("spaces.spaceSettings.registry.next")}</Button>
    </nav>}
    <DiscoverSection />
    {renaming && <SpaceDialog open={!!renaming} onOpenChange={(open) => { if (!open) { setRenaming(undefined); void queryClient.invalidateQueries({ queryKey: ["spaces-registry"] }); } }}
      space={renaming} />}
  </section>;
}

function Filter({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: [string, string][] }) {
  const id = `filter-${label.replace(/\s+/g, "-").toLowerCase()}`;
  return <div className="space-y-1">
    <label htmlFor={id} className="block text-xs font-medium text-muted-foreground">{label}</label>
    <select id={id} value={value} onChange={(e) => onChange(e.target.value)} className="block h-8 rounded-md border border-input bg-background px-2 text-sm">
      {options.map(([v, text]) => <option key={v} value={v}>{text}</option>)}
    </select>
  </div>;
}

function groupRows(rows: Row[], by: "visibility" | "status", t: (key: string) => string) {
  const label = (row: Row) => by === "visibility"
    ? (row.visibility === "public" ? t("spaces.spaceSettings.registry.public") : t("spaces.spaceSettings.registry.private"))
    : (row.archived ? t("spaces.spaceSettings.registry.archived") : t("spaces.spaceSettings.registry.active"));
  const map = new Map<string, Row[]>();
  for (const row of rows) map.set(label(row), [...(map.get(label(row)) ?? []), row]);
  return [...map.entries()].map(([name, list]) => ({ label: name, rows: list }));
}

function formatDate(value: string) {
  return new Date(value).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

type DiscoverItem = { id: string; name: string; description: string | null; icon_key: string | null };

/** Public Spaces you can find and join. Joining gives Read only; private Spaces never appear here. */
function DiscoverSection() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [query, setQuery] = useState("");
  const q = query.trim();
  const discover = useQuery({
    queryKey: ["spaces-discover", q],
    queryFn: async () => (await api<{ items: DiscoverItem[] }>(`/api/v1/spaces/discover${q ? `?q=${encodeURIComponent(q)}` : ""}`)).items,
  });
  async function join(item: DiscoverItem) {
    try {
      await api(`/api/v1/spaces/${item.id}/join`, { method: "POST" });
      await queryClient.invalidateQueries({ queryKey: spaceKeys.all });
      await queryClient.invalidateQueries({ queryKey: ["spaces-discover"] });
      toast.success(t("spaces.spaceSettings.discover.joined", { name: item.name }));
    } catch (e) { toast.error((e as Error).message); }
  }
  return <section className="space-y-3">
    <h2 className="text-base font-semibold">{t("spaces.spaceSettings.discover.title")}</h2>
    <p className="text-sm text-muted-foreground">{t("spaces.spaceSettings.discover.description")}</p>
    <Input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t("spaces.spaceSettings.discover.search")}
      aria-label={t("spaces.spaceSettings.discover.search")} className="max-w-sm" />
    {discover.isError && <FormError message={(discover.error as Error).message} />}
    {discover.data && !discover.data.length && <p className="text-sm text-muted-foreground">{t("spaces.spaceSettings.discover.empty")}</p>}
    {!!discover.data?.length && <ul className="divide-y rounded-lg border">
      {discover.data.map((item) => <li key={item.id} className="flex items-center justify-between gap-3 px-3 py-2.5">
        <span className="inline-flex min-w-0 items-center gap-2 text-sm font-medium"><SpaceIcon iconKey={item.icon_key} className="text-muted-foreground" />{item.name}</span>
        <Button size="sm" variant="outline" aria-label={t("spaces.spaceSettings.discover.joinNamed", { name: item.name })} onClick={() => void join(item)}>{t("spaces.spaceSettings.discover.join")}</Button>
      </li>)}
    </ul>}
  </section>;
}
