import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { getItem, type Item } from "@/lib/items";
import { useSpace } from "@/lib/space-queries";

const MAX_DEPTH = 30; // the same limit the server puts on the hierarchy

/** Every ancestor of an item, from the top down to its direct parent. The API gives only parent ids, so each is loaded once and cached. */
export function useItemAncestors(item?: Item) {
  const parentId = item?.locations[0]?.parent_item_id ?? null;
  return useQuery({
    queryKey: ["item-ancestors", item?.id, parentId],
    enabled: !!parentId,
    staleTime: 60_000,
    queryFn: async () => {
      const chain: Item[] = [];
      let next: string | null = parentId;
      for (let depth = 0; next && depth < MAX_DEPTH; depth++) {
        const { item: parent } = await getItem(next);
        chain.unshift(parent);
        next = parent.locations[0]?.parent_item_id ?? null;
      }
      return chain;
    },
  });
}

/** Space / ancestor / ancestor / item: the whole path, each step linking to its page. */
export function ItemPathBreadcrumb({ item, ancestors }: { item: Item; ancestors: Item[] }) {
  const { t } = useTranslation();
  const spaceId = item.locations[0]?.space_id ?? "";
  const space = useSpace(spaceId);
  return <nav aria-label={t("views.project.breadcrumb")} className="min-w-0">
    <ol className="flex min-w-0 flex-wrap items-center gap-1 text-sm text-muted-foreground">
      {spaceId && <li className="shrink-0">{space.data && <Link to="/spaces/$spaceId" params={{ spaceId }} className="hover:text-foreground hover:underline">{space.data.name}</Link>}</li>}
      {ancestors.map((ancestor) => <li key={ancestor.id} className="flex min-w-0 items-center gap-1">
        <span aria-hidden="true" className="shrink-0">/</span>
        <Link to="/tasks/$itemId" params={{ itemId: ancestor.id }} className="min-w-0 truncate hover:text-foreground hover:underline" title={ancestor.title}>{ancestor.title}</Link>
      </li>)}
      <li className="flex min-w-0 items-center gap-1">
        <span aria-hidden="true" className="shrink-0">/</span>
        <span aria-current="page" className="min-w-0 truncate text-foreground" title={item.title}>{item.title}</span>
      </li>
    </ol>
  </nav>;
}
