import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";

type Node = { id: string; title: string; type: string; can_create: boolean };
type Path = { id: string; title: string }[];

// DestinationTree picks where an import lands: the Space root, or any project or folder in it, nested as deep as the
// hierarchy goes. Children load when a node opens; the server returns only items the user can see, and it checks create
// rights and allowed children again at commit.
export function DestinationTree({ spaceId, value, onChange, disabled }: {
  spaceId: string;
  value: string;
  onChange: (id: string, path: Path) => void;
  disabled: boolean;
}) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const [selectedPath, setSelectedPath] = useState<Path>([]);
  const roots = useQuery({
    queryKey: ["excel-destinations", spaceId],
    queryFn: ({ signal }) => api<{ items: Node[] }>(`/api/v1/spaces/${spaceId}/excel-destinations`, { signal }),
  });
  const needle = query.trim().toLowerCase();
  const pick = (node: Node, path: Path) => {
    if (!node.can_create) return;
    const chain = [...path, { id: node.id, title: node.title }];
    setSelectedPath(chain);
    onChange(node.id, chain);
  };
  return <div className="space-y-2">
    <span>{t("excel.destination")}</span>
    <div className="text-sm text-muted-foreground" aria-live="polite">
      {value ? selectedPath.map((step) => step.title).join(" › ") : t("excel.spaceRoot")}
    </div>
    <input type="search" className="w-full rounded border bg-background p-2" placeholder={t("excel.destinationSearch")}
      aria-label={t("excel.destinationSearch")} value={query} onChange={(event) => setQuery(event.target.value)} />
    <ul role="tree" aria-label={t("excel.destination")} className="max-h-56 overflow-auto rounded border p-1">
      <li role="treeitem" aria-selected={!value} className="px-2 py-1">
        <button type="button" disabled={disabled} className={`w-full text-left ${!value ? "font-medium" : ""}`}
          onClick={() => { setSelectedPath([]); onChange("", []); }}>{t("excel.spaceRoot")}</button>
      </li>
      {roots.isPending && <li className="px-2 py-1 text-sm">{t("items.loading")}</li>}
      {(roots.data?.items ?? []).filter((node) => !needle || node.title.toLowerCase().includes(needle)).map((node) =>
        <Branch key={node.id} spaceId={spaceId} node={node} path={[]} value={value} onPick={pick} disabled={disabled} filter={needle} />)}
    </ul>
  </div>;
}

function Branch({ spaceId, node, path, value, onPick, disabled, filter }: {
  spaceId: string; node: Node; path: Path; value: string; onPick: (node: Node, path: Path) => void; disabled: boolean; filter: string;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const children = useQuery({
    queryKey: ["excel-destination-children", node.id],
    enabled: open,
    queryFn: ({ signal }) => api<{ items: Node[] }>(`/api/v1/spaces/${spaceId}/excel-destinations?parent=${node.id}`, { signal }),
  });
  const chain = [...path, { id: node.id, title: node.title }];
  return <li role="treeitem" aria-expanded={open} aria-selected={value === node.id} className="px-2">
    <div className="flex items-center gap-1 py-1">
      <button type="button" aria-label={open ? t("excel.collapse") : t("excel.expand")} className="w-5 text-sm"
        onClick={() => setOpen(!open)}>{open ? "▾" : "▸"}</button>
      <button type="button" disabled={disabled || !node.can_create} aria-disabled={!node.can_create}
        className={`flex-1 text-left ${value === node.id ? "font-medium underline" : ""} ${node.can_create ? "" : "text-muted-foreground"}`}
        onClick={() => onPick(node, path)} title={node.can_create ? chain.map((step) => step.title).join(" › ") : t("excel.noCreate")}>{node.title}</button>
    </div>
    {open && <ul role="group" className="ml-4 border-l pl-2">
      {children.isPending && <li className="text-sm">{t("items.loading")}</li>}
      {(children.data?.items ?? []).filter((child) => !filter || child.title.toLowerCase().includes(filter)).map((child) =>
        <Branch key={child.id} spaceId={spaceId} node={child} path={chain} value={value} onPick={onPick} disabled={disabled} filter={filter} />)}
    </ul>}
  </li>;
}
