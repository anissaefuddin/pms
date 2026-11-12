import { useId, useState } from "react";
import { useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { getItem, itemKeys, listItems, type Item } from "@/lib/items";
import { useSpaces } from "@/lib/space-queries";

type Location = Item["locations"][number];

/** One Item in several places (SPC-10): the same Item, not a copy. The server refuses cycles and a last-location removal. */
export function ItemLocations({ item, editable, disabled }: { item: Item; editable: boolean; disabled: boolean }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const heading = useId();
  const [adding, setAdding] = useState(false);
  const [space, setSpace] = useState("");
  const [parent, setParent] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const spaces = useSpaces();
  const parentIds = [...new Set(item.locations.flatMap((loc) => loc.parent_item_id ? [loc.parent_item_id] : []))];
  const parents = useQueries({ queries: parentIds.map((id) => ({ queryKey: itemKeys.detail(id), queryFn: () => getItem(id) })) });
  const parentTitle = new Map(parentIds.map((id, index) => [id, parents[index].data?.item.title]));
  const projects = useQuery({
    queryKey: ["location-targets", space], enabled: adding && !!space,
    queryFn: () => listItems(`/api/v1/spaces/${space}/items`, undefined, "project"),
  });
  const targets = (spaces.data ?? []).filter((entry) => entry.my_capabilities?.includes("create"));

  async function replace(locations: { space_id: string; parent_item_id: string | null }[]) {
    setBusy(true);
    setError("");
    try {
      await api(`/api/v1/items/${item.id}/locations`, { method: "PUT", body: JSON.stringify({ locations }) });
      await qc.invalidateQueries({ queryKey: itemKeys.all });
      setAdding(false);
      setSpace("");
      setParent("");
    } catch (err) { setError((err as Error).message); }
    finally { setBusy(false); }
  }
  const current = item.locations.map((loc) => ({ space_id: loc.space_id, parent_item_id: loc.parent_item_id ?? null }));
  const label = (loc: Location) => [loc.space_name, loc.parent_item_id ? parentTitle.get(loc.parent_item_id) ?? "…" : undefined].filter(Boolean).join(" / ");

  return <section aria-labelledby={heading} className="space-y-2 border-t pt-4">
    <h3 id={heading} className="font-medium">{t("items.locations.title")}</h3>
    <ul className="space-y-1">
      {item.locations.map((loc) => <li key={`${loc.space_id}/${loc.parent_item_id ?? ""}`} className="flex items-center justify-between gap-2">
        <span className="min-w-0 truncate">{label(loc)}</span>
        {editable && <Button size="sm" variant="outline" disabled={disabled || busy || item.locations.length < 2}
          aria-label={t("items.locations.remove", { name: label(loc) })}
          onClick={() => void replace(current.filter((entry) => entry.space_id !== loc.space_id || entry.parent_item_id !== (loc.parent_item_id ?? null)))}>
          {t("items.locations.removeShort")}
        </Button>}
      </li>)}
    </ul>
    {editable && item.locations.length < 2 && <p className="text-xs text-muted-foreground">{t("items.locations.last")}</p>}
    <FormError message={error} />
    {editable && (adding ? <form className="flex flex-wrap items-end gap-2" onSubmit={(event) => {
      event.preventDefault();
      if (space) void replace([...current, { space_id: space, parent_item_id: parent || null }]);
    }}>
      <label className="space-y-1"><span className="block text-sm">{t("items.locations.space")}</span>
        <select aria-label={t("items.locations.space")} value={space} required className="rounded border bg-background p-1" onChange={(event) => { setSpace(event.target.value); setParent(""); }}>
          <option value="" disabled />
          {targets.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}
        </select></label>
      <label className="space-y-1"><span className="block text-sm">{t("items.locations.project")}</span>
        <select aria-label={t("items.locations.project")} value={parent} disabled={!space} className="rounded border bg-background p-1" onChange={(event) => setParent(event.target.value)}>
          <option value="">{t("items.locations.root")}</option>
          {(projects.data ?? []).filter((project) => project.id !== item.id).map((project) => <option key={project.id} value={project.id}>{project.title}</option>)}
        </select></label>
      <Button type="submit" size="sm" disabled={busy || disabled || !space}>{t("items.locations.add")}</Button>
      <Button type="button" size="sm" variant="ghost" onClick={() => setAdding(false)}>{t("items.cancel")}</Button>
    </form> : <Button size="sm" variant="outline" disabled={disabled} onClick={() => setAdding(true)}>{t("items.locations.add")}</Button>)}
  </section>;
}
