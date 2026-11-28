import { useDeferredValue, useState } from "react";
import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/react-query";
import { ExternalLink } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { formatDate, useProfile } from "@/lib/profile";
import type { Item } from "@/lib/items";
import { EmptyState } from "@/components/ui/empty-state";
import { FilesIllustration, ProjectIllustration } from "@/components/ui/empty-illustrations";
import { statusDotClass } from "@/lib/status-colors";
import { AddFile } from "@/components/project-files-add";
import { FileActions } from "@/components/project-file-actions";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type Listing = {
  items: components["schemas"]["ProjectFile"][]; total: number; truncated: boolean;
  counts: { all: number; direct: number; subitems: number; projects: number }; projects: { id: string; title: string; files: number }[];
};
const reviewKey: Record<string, string> = { not_requested: "notRequested", in_review: "inReview", approved: "approved", rejected: "rejected", cancelled: "cancelled" };

const PAGE = 50;
const REVIEWS = Object.keys(reviewKey);
const SCOPES = [["", "all"], ["direct", "direct"], ["subitems", "subitems"]] as const;
const noFilters = { q: "", scope: "", kind: "", review: "", project: "" };
const selectClass = "block h-9 rounded-md border border-input bg-background px-2 focus-visible:outline-ring";

// A short type from the extension (PDF, DOCX); the content type says too much for a row.
function shortType(filename: string, contentType: string) {
  const extension = /\.([A-Za-z0-9]{1,8})$/.exec(filename)?.[1];
  return (extension ?? contentType.split(/[;/]/)[1] ?? contentType).toUpperCase();
}

/**
 * Files view of a Space, project or folder: the documents of its items that the viewer may see. A Space holds no files itself, so
 * without `project` there is nothing to add to, no direct/subitem split and no folder link.
 */
export function ProjectFiles({ project, spaceId, onOpen }: { project?: Item; spaceId: string; onOpen: (item: Item) => void }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const client = useQueryClient();
  const [filters, setFilters] = useState(noFilters);
  const [page, setPage] = useState(0);
  const search = useDeferredValue(filters.q.trim());
  const params = new URLSearchParams({ limit: String(PAGE), offset: String(page * PAGE) });
  for (const [key, value] of Object.entries({ ...filters, q: search })) if (value) params.set(key, value);
  const owner = project?.id ?? spaceId;
  const files = useQuery({
    queryKey: ["project-files", owner, params.toString()], placeholderData: keepPreviousData,
    queryFn: () => api<Listing>(`/api/v1/${project ? "items" : "spaces"}/${owner}/files?${params}`),
  });
  // The folder link of this item, shown only when the server says it is valid for the item's Customer.
  const dms = useQuery({ queryKey: ["pydio-reference", owner, project?.version], enabled: !!project, queryFn: () => api<{ state: string; open_url?: string }>(`/api/v1/items/${owner}/pydio-reference`), retry: false });
  if (files.isPending) return <p role="status">{t("items.loading")}</p>;
  if (files.isError) return <p role="alert">{t("items.loadError")}</p>;
  const data = files.data;
  const folder = project?.type === "folder";
  const here = t(folder ? "views.files.thisFolder" : "views.files.thisProject");
  const change = (update: Partial<typeof noFilters>) => { setFilters((current) => ({ ...current, ...update })); setPage(0); };
  const filtered = Object.values(filters).some(Boolean);
  const dmsURL = dms.data?.state === "linked" ? dms.data.open_url : undefined;
  const changed = (itemId: string) => Promise.all([client.invalidateQueries({ queryKey: ["project-files", owner] }), client.invalidateQueries({ queryKey: ["item-attachments", itemId] })]);
  const add = project?.my_capabilities.includes("attach") && <AddFile itemId={owner} destination={t(folder ? "views.files.addedToFolder" : "views.files.addedToProject")}
    onAdded={() => changed(owner)} />;
  const dmsLink = dmsURL && <a href={dmsURL} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-sm text-primary underline-offset-2 hover:underline">
    <ExternalLink aria-hidden="true" className="size-4" />{t(folder ? "views.files.openFolder" : "views.files.openProject")}</a>;
  if (!data.counts.all) return <div className="space-y-3" data-files-view>{add}
    <EmptyState icon={<FilesIllustration />} title={t("views.files.empty")} description={t(project ? "views.files.emptyHint" : "views.files.emptySpaceHint")} />{dmsLink}</div>;
  const stats = [["total", data.counts.all] as const,
    ...(project ? [["direct", data.counts.direct] as const, ["subitems", data.counts.subitems] as const] : []),
    ...(!project || folder ? [["projects", data.counts.projects] as const] : [])];
  const first = page * PAGE;
  return <div className="space-y-3" data-files-view>
    <div role="group" aria-label={t("views.files.summary")} className="grid grid-cols-2 gap-3 sm:grid-cols-4">
      {stats.map(([key, value]) => <div key={key} className="rounded-lg border bg-card px-3 py-2">
        <p className="text-xs text-muted-foreground">{t(`views.files.stats.${key}`)}</p><p className="text-2xl font-semibold tabular-nums">{value}</p></div>)}
    </div>
    <div className="flex flex-wrap items-end gap-2">
      <Input type="search" aria-label={t("views.files.search")} placeholder={t("views.files.search")} maxLength={200} value={filters.q} onChange={(event) => change({ q: event.target.value })} className="w-full sm:w-64" />
      {data.projects.length > 0 && <label className="text-xs text-muted-foreground">{t("views.files.project")}
        <select className={`${selectClass} max-w-56 text-sm text-foreground`} aria-label={t("views.files.project")} value={filters.project} onChange={(event) => change({ project: event.target.value })}>
          <option value="">{t("views.files.any")}</option>{data.projects.map((p) => <option key={p.id} value={p.id}>{p.title} ({p.files})</option>)}</select></label>}
      <label className="text-xs text-muted-foreground">{t("views.files.type")}
        <select className={`${selectClass} text-sm text-foreground`} aria-label={t("views.files.type")} value={filters.kind} onChange={(event) => change({ kind: event.target.value })}>
          <option value="">{t("views.files.any")}</option><option value="file">{t("views.files.fileOnly")}</option><option value="link">{t("views.files.linkOnly")}</option></select></label>
      <label className="text-xs text-muted-foreground">{t("items.reviewLabel")}
        <select className={`${selectClass} text-sm text-foreground`} aria-label={t("items.reviewLabel")} value={filters.review} onChange={(event) => change({ review: event.target.value })}>
          <option value="">{t("views.files.any")}</option>{REVIEWS.map((key) => <option key={key} value={key}>{t(`items.reviewState.${reviewKey[key]}`)}</option>)}</select></label>
      {filtered && <Button variant="outline" onClick={() => change(noFilters)}>{t("views.clearFilters")}</Button>}
      <div className="ms-auto">{add}</div>
    </div>
    {project && <div role="group" aria-label={t("views.files.scope")} className="flex flex-wrap gap-1 border-b">
      {SCOPES.map(([scope, key]) =>
        <button key={key} type="button" aria-pressed={filters.scope === scope} onClick={() => change({ scope })}
          className={`px-3 py-1.5 text-sm ${filters.scope === scope ? "border-b-2 border-foreground font-medium" : "text-muted-foreground"}`}>{t(`views.files.tabs.${key}`, { number: data.counts[key] })}</button>)}
    </div>}
    {data.items.length === 0 ? <p role="status" className="py-4 text-sm text-muted-foreground">{t("views.files.noMatch")}</p>
      : <div className="overflow-x-auto" tabIndex={0} role="region" aria-label={t("views.files.listRegion")}><table aria-label={t("views.types.files")} className="w-full text-left text-sm">
        <thead><tr>
          <th className="p-2">{t("views.files.name")}</th><th className="hidden p-2 md:table-cell">{t("views.files.location")}</th><th className="p-2">{t("items.reviewLabel")}</th>
          <th className="hidden p-2 md:table-cell">{t("views.files.added")}</th><th className="p-2 text-right">{t("views.files.actions")}</th></tr></thead>
        <tbody>{data.items.map((file) => {
          const open = file.item_id === project?.id ? undefined : () => onOpen({ id: file.item_id, title: file.item_title } as Item);
          const location = open ? <button type="button" className="text-left underline" onClick={open}>{file.path.join(" › ") || file.item_title}</button> : here;
          const details = [file.link_url ? t("views.files.linkOnly") : shortType(file.filename, file.content_type),
            ...(file.link_url ? [] : [t("views.files.kb", { count: Math.max(1, Math.round(file.size_bytes / 1024)) })]),
            ...(file.version && !file.link_url ? [t("views.files.versionNumber", { version: file.version })] : [])].join(" · ");
          return <tr key={file.id} className="border-t align-top">
            {/* Link attachments have no file to download: they open their URL instead (E2). */}
            <td className="min-w-40 p-2">{file.link_url
              ? <a className="break-all font-medium underline" href={file.link_url} rel="noopener noreferrer" target="_blank">{file.filename || t("views.files.linkOnly")}</a>
              : <a className="break-all font-medium underline" href={`/api/v1/attachments/${file.id}/download`} download>{file.filename}</a>}
              <span className="block text-xs text-muted-foreground">{details}</span>
              <span className="block text-xs text-muted-foreground md:hidden">{location}</span></td>
            <td className="hidden p-2 md:table-cell">{location}</td>
            {/* WA-D08: review status of the newest version of this file (not inherited from older versions). */}
            <td className="p-2"><span data-review-status={file.review_status}>{t(`items.reviewState.${reviewKey[file.review_status ?? "not_requested"] ?? "notRequested"}`)}</span></td>
            <td className="hidden p-2 md:table-cell"><span className="whitespace-nowrap">{formatDate(file.created_at, profile)}</span>
              <span className="block text-xs text-muted-foreground">{file.uploader.name}</span></td>
            <td className="p-2"><FileActions file={file} onOpenOwner={open} onChanged={() => changed(file.item_id)} /></td>
          </tr>;
        })}</tbody>
      </table></div>}
    {data.truncated && <p className="text-sm text-muted-foreground">{t("views.files.truncated")}</p>}
    <div className="flex flex-wrap items-center gap-2 text-sm">
      {data.items.length > 0 && <p className="text-muted-foreground">{t("views.files.showing", { from: first + 1, to: first + data.items.length, total: data.total })}</p>}
      {(page > 0 || first + data.items.length < data.total) && <div className="flex gap-1">
        <Button variant="outline" size="sm" disabled={page === 0 || files.isPlaceholderData} onClick={() => setPage(page - 1)}>{t("views.files.previous")}</Button>
        <Button variant="outline" size="sm" disabled={first + data.items.length >= data.total || files.isPlaceholderData} onClick={() => setPage(page + 1)}>{t("views.files.next")}</Button></div>}
      <div className="ms-auto">{dmsLink}</div>
    </div>
  </div>;
}

/** F-01: items by status over the whole project subtree (tasks only, milestones excluded), counted by the server like the header KPIs. */
export function ProjectChart({ project }: { project: Item }) {
  const { t } = useTranslation();
  const tally = useQuery({
    queryKey: ["project-chart", project.id],
    queryFn: () => api<components["schemas"]["AnalyticsResult"]>("/api/v1/analytics/query", { method: "POST", body: JSON.stringify({
      reporting_on: "tasks", metric: "count", group_by: "status_id", container_ids: [project.id], filters: { exclude_milestones: true },
    }) }),
  });
  const workflows = useQuery({ queryKey: ["workflows"], queryFn: () => api<{ items: components["schemas"]["Workflow"][] }>("/api/v1/workflows") });
  const colors = new Map((workflows.data?.items ?? []).flatMap((wf) => wf.statuses.map((status) => [status.id, status.color] as const)));
  if (tally.isPending) return <p role="status">{t("items.loading")}</p>;
  if (tally.isError) return <p role="alert">{t("items.loadError")}</p>;
  const rows = tally.data.rows;
  const total = rows.reduce((sum, row) => sum + row.value, 0);
  if (!total) return <EmptyState icon={<ProjectIllustration />} title={t("views.noItems.title")} description={t("views.noItems.description")} />;
  return <figure className="space-y-2">
    <figcaption className="flex items-center justify-between gap-2 font-medium">
      <span>{t("views.chart.title")}</span>
      <span className="text-sm font-normal text-muted-foreground">{t("views.chart.total", { count: total })}</span>
    </figcaption>
    <p className="text-xs text-muted-foreground">{t("views.chartScope")}</p>
    <ul className="space-y-1">{rows.map((row) => {
      const percent = Math.round((row.value / total) * 100);
      const color = colors.get(row.group.key) ?? "";
      return <li key={row.group.key} className="flex items-center gap-2">
        <span className="flex w-40 min-w-0 items-center gap-2"><span aria-hidden className={`size-2.5 shrink-0 rounded-full ${statusDotClass(color)}`} /><span className="truncate">{row.group.label}</span></span>
        <span role="img" aria-label={t("views.chart.bar", { name: row.group.label, count: row.value })} className={`h-4 rounded ${statusDotClass(color)}`}
          style={{ width: `${percent}%`, minWidth: 4 }} />
        <span className="tabular-nums">{row.value}</span>
        <span className="text-xs text-muted-foreground">{t("views.chart.share", { percent })}</span>
      </li>;
    })}</ul>
  </figure>;
}
