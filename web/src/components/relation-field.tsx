import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { useDebounced } from "@/lib/space-queries";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export type RelationGroup = { field_id: string; name: string; hidden: number; items: { id: string; title: string; type: string; type_key: string }[] };
type SearchItems = components["schemas"]["SearchResults"];

export const useItemRelations = (itemId: string, version: number, enabled: boolean) =>
  useQuery({ queryKey: ["item-relations", itemId, version], enabled,
    queryFn: async () => (await api<{ items: RelationGroup[] }>(`/api/v1/items/${itemId}/relations`)).items });

/** Targets of a relation field. Only items the viewer may see are named; the rest are only counted (CUST-05). */
export function RelationDisplay({ group }: { group?: RelationGroup }) {
  const { t } = useTranslation();
  if (!group || (!group.items.length && !group.hidden)) return <span className="text-muted-foreground">{t("items.customFields.empty")}</span>;
  return <span className="flex flex-wrap gap-x-3 gap-y-1">
    {group.items.map((item) => <a key={item.id} className="underline" href={`/tasks/${item.id}`}>{item.title}</a>)}
    {group.hidden > 0 && <span className="text-muted-foreground">{t("items.relation.hidden", { count: group.hidden })}</span>}
  </span>;
}

/** Picks up to 50 related items by searching. Editing is closed when the field holds targets the viewer cannot see, so none is dropped unseen. */
export function RelationEditor({ itemId, group, disabled, onCancel, onSave }: {
  itemId: string; group?: RelationGroup; disabled?: boolean; onCancel: () => void; onSave: (ids: string[]) => void;
}) {
  const { t } = useTranslation();
  const [picked, setPicked] = useState(group?.items ?? []);
  const [text, setText] = useState("");
  const q = useDebounced(text.trim(), 250);
  const results = useQuery({ queryKey: ["relation-search", q], enabled: q.length >= 2, retry: false,
    queryFn: () => api<SearchItems>(`/api/v1/search?q=${encodeURIComponent(q)}&limit=8`) });
  const hits = (results.data?.items ?? []).filter((hit) => hit.id !== itemId && !picked.some((p) => p.id === hit.id));
  if (group?.hidden) return <div className="space-y-2">
    <p className="text-sm">{t("items.relation.locked", { count: group.hidden })}</p>
    <Button type="button" variant="outline" size="sm" onClick={onCancel}>{t("items.cancel")}</Button></div>;
  return <div className="space-y-2">
    <label className="block space-y-1 text-sm"><span>{t("items.relation.search")}</span>
      <Input value={text} onChange={(event) => setText(event.target.value)} disabled={disabled || picked.length >= 50} /></label>
    {hits.length > 0 && <ul className="rounded border text-sm" aria-label={t("items.relation.results")}>{hits.map((hit) => <li key={hit.id}>
      <Button type="button" variant="ghost" className="w-full justify-start" aria-label={t("items.relation.add", { title: hit.title })}
        onClick={() => { setPicked([...picked, { id: hit.id, title: hit.title, type: hit.type, type_key: hit.type }]); setText(""); }}>{hit.title}</Button></li>)}</ul>}
    <ul className="flex flex-wrap gap-2 text-sm">{picked.map((p) => <li key={p.id}>
      <Button type="button" variant="outline" size="sm" aria-label={t("items.relation.remove", { title: p.title })}
        onClick={() => setPicked(picked.filter((x) => x.id !== p.id))}>{p.title} ×</Button></li>)}</ul>
    <div className="flex gap-2">
      <Button type="button" disabled={disabled} onClick={() => onSave(picked.map((p) => p.id))}>{t("items.save")}</Button>
      <Button type="button" variant="outline" onClick={onCancel}>{t("items.cancel")}</Button>
    </div>
  </div>;
}
