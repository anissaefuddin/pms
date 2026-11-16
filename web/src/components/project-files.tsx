import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { formatDate, useProfile } from "@/lib/profile";
import type { Item } from "@/lib/items";
import { EmptyState } from "@/components/ui/empty-state";
import { FilesIllustration, ProjectIllustration } from "@/components/ui/empty-illustrations";
import { statusDotClass } from "@/lib/status-colors";

type ProjectFile = components["schemas"]["ProjectFile"];
const reviewKey: Record<string, string> = { not_requested: "notRequested", in_review: "inReview", approved: "approved", rejected: "rejected", cancelled: "cancelled" };

export function ProjectFiles({ project, onOpen }: { project: Item; onOpen: (item: Item) => void }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const files = useQuery({ queryKey: ["project-files", project.id], queryFn: () => api<{ items: ProjectFile[] }>(`/api/v1/items/${project.id}/files`) });
  if (files.isPending) return <p role="status">{t("items.loading")}</p>;
  if (files.isError) return <p role="alert">{t("items.loadError")}</p>;
  if (!files.data.items.length) return <EmptyState icon={<FilesIllustration />} title={t("views.files.empty")} description={t("views.files.emptyHint")} />;
  return <table aria-label={t("views.types.files")} className="w-full text-left text-sm">
    <thead><tr>
      <th className="p-2">{t("views.files.name")}</th><th className="p-2">{t("views.files.type")}</th><th className="p-2">{t("views.files.version")}</th><th className="p-2">{t("items.reviewLabel")}</th><th className="p-2">{t("views.files.item")}</th><th className="p-2">{t("views.files.size")}</th>
      <th className="p-2">{t("views.files.uploader")}</th><th className="p-2">{t("views.files.added")}</th></tr></thead>
    <tbody>{files.data.items.map((file) => <tr key={file.id} className="border-t">
      {/* Link attachments have no file to download: they open their URL instead (E2). */}
      <td className="p-2">{file.link_url
        ? <a className="underline" href={file.link_url} rel="noopener noreferrer" target="_blank">{file.filename || t("views.files.linkOnly")}</a>
        : <a className="underline" href={`/api/v1/attachments/${file.id}/download`} download>{file.filename}</a>}</td>
      <td className="p-2 text-muted-foreground">{file.link_url ? t("views.files.linkOnly") : file.content_type}</td>
      <td className="p-2">{file.version ? t("views.files.versionNumber", { version: file.version }) : "—"}</td>
      {/* WA-D08: review status of the newest version of this file (not inherited from older versions). */}
      <td className="p-2"><span data-review-status={file.review_status}>{t(`items.reviewState.${reviewKey[file.review_status ?? "not_requested"] ?? "notRequested"}`)}</span></td>
      <td className="p-2"><button type="button" className="underline" onClick={() => onOpen({ id: file.item_id, title: file.item_title } as Item)}>{file.item_title}</button></td>
      <td className="p-2">{file.link_url ? t("views.files.dash") : t("views.files.kb", { count: Math.max(1, Math.round(file.size_bytes / 1024)) })}</td>
      <td className="p-2">{file.uploader.name}</td>
      <td className="p-2">{formatDate(file.created_at, profile)}</td>
    </tr>)}</tbody>
  </table>;
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
