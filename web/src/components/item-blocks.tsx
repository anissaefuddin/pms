import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import type { Item } from "@/lib/items";

/** The panel blocks in their standard order; a type's layout may regroup and reorder them (CUST-04). */
export const BLOCK_ORDER = ["status", "importance", "dates", "assignees", "effort", "locations", "custom", "description", "subitems", "comments", "approvals", "activity", "attachments"] as const;
export type BlockKey = (typeof BLOCK_ORDER)[number];

type Layout = { sections?: { title: string; fields: string[] }[] };

// Layout names are the API's; the panel groups a few of them into one block.
const alias: Record<string, BlockKey> = { dates: "dates", status: "status", importance: "importance", assignees: "assignees", effort: "effort", description: "description",
  locations: "locations", subitems: "subitems", attachments: "attachments", comments: "comments", approvals: "approvals", activity: "activity" };

/** Renders the item's blocks as its type's layout says; blocks the layout does not mention follow in the standard order. */
export function ItemBlocks({ item, blocks }: { item: Item; blocks: Record<BlockKey, ReactNode> }) {
  const { t } = useTranslation();
  const layout = (item.item_type?.layout ?? {}) as Layout;
  if (!layout.sections?.length) return <>{BLOCK_ORDER.map((key) => <div key={key} className="contents">{blocks[key]}</div>)}</>;
  const placed = new Set<BlockKey>();
  const sections = layout.sections.map((section) => {
    const keys: BlockKey[] = [];
    for (const field of section.fields) {
      const key = alias[field] ?? "custom"; // a custom field id places the custom fields block
      if (!placed.has(key)) { placed.add(key); keys.push(key); }
    }
    return { title: section.title, keys };
  });
  const rest = BLOCK_ORDER.filter((key) => !placed.has(key));
  return <>
    {sections.filter((section) => section.keys.some((key) => blocks[key] != null)).map((section) => <section key={section.title} className="space-y-3" aria-label={section.title}>
      <h2 className="border-b pb-1 text-sm font-semibold uppercase tracking-wide text-muted-foreground">{section.title}</h2>
      {section.keys.map((key) => <div key={key} className="contents">{blocks[key]}</div>)}
    </section>)}
    {rest.length > 0 && <section className="space-y-3" aria-label={t("items.type.otherSection")}>
      {rest.map((key) => <div key={key} className="contents">{blocks[key]}</div>)}
    </section>}
  </>;
}
