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

export function ProjectFiles({ project, onOpen }: { project: Item; onOpen: (item: Item) => void }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const files = useQuery({ queryKey: ["project-files", project.id], queryFn: () => api<{ items: ProjectFile[] }>(`/api/v1/items/${project.id}/files`) });
  if (files.isPending) return <p role="status">{t("items.loading")}</p>;
  if (files.isError) return <p role="alert">{t("items.loadError")}</p>;
  if (!files.data.items.length) return <EmptyState icon={<FilesIllustration />} title={t("views.files.empty")} description={t("views.files.emptyHint")} />;
  return <table aria-label={t("views.types.files")} className="w-full text-left text-sm">
    <thead><tr>
      <th className="p-2">{t("views.files.name")}</th><th className="p-2">{t("views.files.type")}</th><th className="p-2">{t("views.files.version")}</th><th className="p-2">{t("views.files.item")}</th><th className="p-2">{t("views.files.size")}</th>
      <th className="p-2">{t("views.files.uploader")}</th><th className="p-2">{t("views.files.added")}</th></tr></thead>
    <tbody>{files.data.items.map((file) => <tr key={file.id} className="border-t">
      {/* Link attachments have no file to download: they open their URL instead (E2). */}
      <td className="p-2">{file.link_url
        ? <a className="underline" href={file.link_url} rel="noopener noreferrer" target="_blank">{file.filename || t("views.files.linkOnly")}</a>
        : <a className="underline" href={`/api/v1/attachments/${file.id}/download`} download>{file.filename}</a>}</td>
      <td className="p-2 text-muted-foreground">{file.link_url ? t("views.files.linkOnly") : file.content_type}</td>
      <td className="p-2">{file.version ? t("views.files.versionNumber", { version: file.version }) : "—"}</td>
      <td className="p-2"><button type="button" className="underline" onClick={() => onOpen({ id: file.item_id, title: file.item_title } as Item)}>{file.item_title}</button></td>
      <td className="p-2">{file.link_url ? t("views.files.dash") : t("views.files.kb", { count: Math.max(1, Math.round(file.size_bytes / 1024)) })}</td>
      <td className="p-2">{file.uploader.name}</td>
      <td className="p-2">{formatDate(file.created_at, profile)}</td>
    </tr>)}</tbody>
  </table>;
}

export function ProjectChart({ items }: { items: Item[] }) {
  const { t } = useTranslation();
  // F1: one bar per workflow status, in the status's own colour. Counts are over the project's direct items (a verified scope).
  const counts = new Map<string, { name: string; color?: string; count: number }>();
  for (const item of items) {
    const name = item.status?.name ?? t("views.chart.noStatus");
    const entry = counts.get(name) ?? { name, color: item.status?.color, count: 0 };
    entry.count += 1;
    counts.set(name, entry);
  }
  if (!items.length) return <EmptyState icon={<ProjectIllustration />} title={t("views.noItems.title")} description={t("views.noItems.description")} />;
  return <figure className="space-y-2">
    <figcaption className="flex items-center justify-between gap-2 font-medium">
      <span>{t("views.chart.title")}</span>
      <span className="text-sm font-normal text-muted-foreground">{t("views.chart.total", { count: items.length })}</span>
    </figcaption>
    <ul className="space-y-1">{[...counts.values()].map((entry) => {
      const percent = Math.round((entry.count / items.length) * 100);
      return <li key={entry.name} className="flex items-center gap-2">
        <span className="flex w-40 min-w-0 items-center gap-2"><span aria-hidden className={`size-2.5 shrink-0 rounded-full ${statusDotClass(entry.color ?? "slate")}`} /><span className="truncate">{entry.name}</span></span>
        <span role="img" aria-label={t("views.chart.bar", { name: entry.name, count: entry.count })} className={`h-4 rounded ${statusDotClass(entry.color ?? "slate")}`}
          style={{ width: `${percent}%`, minWidth: 4 }} />
        <span className="tabular-nums">{entry.count}</span>
        <span className="text-xs text-muted-foreground">{t("views.chart.share", { percent })}</span>
      </li>;
    })}</ul>
  </figure>;
}
