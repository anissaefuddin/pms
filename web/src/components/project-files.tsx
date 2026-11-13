import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { formatDate, useProfile } from "@/lib/profile";
import type { Item } from "@/lib/items";

type ProjectFile = components["schemas"]["ProjectFile"];

export function ProjectFiles({ project, onOpen }: { project: Item; onOpen: (item: Item) => void }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const files = useQuery({ queryKey: ["project-files", project.id], queryFn: () => api<{ items: ProjectFile[] }>(`/api/v1/items/${project.id}/files`) });
  if (files.isPending) return <p role="status">{t("items.loading")}</p>;
  if (files.isError) return <p role="alert">{t("items.loadError")}</p>;
  if (!files.data.items.length) return <p role="status">{t("views.files.empty")}</p>;
  return <table aria-label={t("views.types.files")} className="w-full text-left text-sm">
    <thead><tr>
      <th className="p-2">{t("views.files.name")}</th><th className="p-2">{t("views.files.item")}</th><th className="p-2">{t("views.files.size")}</th>
      <th className="p-2">{t("views.files.uploader")}</th><th className="p-2">{t("views.files.added")}</th></tr></thead>
    <tbody>{files.data.items.map((file) => <tr key={file.id} className="border-t">
      <td className="p-2"><a className="underline" href={`/api/v1/attachments/${file.id}/download`} download>{file.filename}</a></td>
      <td className="p-2"><button type="button" className="underline" onClick={() => onOpen({ id: file.item_id, title: file.item_title } as Item)}>{file.item_title}</button></td>
      <td className="p-2">{t("views.files.kb", { count: Math.max(1, Math.round(file.size_bytes / 1024)) })}</td>
      <td className="p-2">{file.uploader.name}</td>
      <td className="p-2">{formatDate(file.created_at, profile)}</td>
    </tr>)}</tbody>
  </table>;
}

export function ProjectChart({ items }: { items: Item[] }) {
  const { t } = useTranslation();
  const counts = new Map<string, number>();
  for (const item of items) counts.set(item.status?.name ?? t("views.chart.noStatus"), (counts.get(item.status?.name ?? t("views.chart.noStatus")) ?? 0) + 1);
  if (!items.length) return <p role="status">{t("views.empty")}</p>;
  return <figure className="space-y-2">
    <figcaption className="font-medium">{t("views.chart.title")}</figcaption>
    <ul className="space-y-1">{[...counts].map(([name, count]) => <li key={name} className="flex items-center gap-2">
      <span className="w-40 truncate">{name}</span>
      <span role="img" aria-label={t("views.chart.bar", { name, count })} className="h-4 rounded bg-primary" style={{ width: `${(count / items.length) * 100}%`, minWidth: 4 }} />
      <span>{count}</span>
    </li>)}</ul>
  </figure>;
}
