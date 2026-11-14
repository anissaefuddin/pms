import { Bug, Circle, CheckSquare, Flag, Folder, Lightbulb, type LucideIcon } from "lucide-react";
import type { Item } from "@/lib/items";

const icons: Record<string, LucideIcon> = { circle: Circle, bug: Bug, folder: Folder, "check-square": CheckSquare, flag: Flag, lightbulb: Lightbulb };

/** The item's type (built-in or custom) as icon + name; the icon slug falls back to a circle. */
export function TypeBadge({ item }: { item: Pick<Item, "item_type"> }) {
  const type = item.item_type;
  if (!type) return null;
  const Icon = icons[type.icon] ?? Circle;
  return <span className="inline-flex items-center gap-1" data-item-type={type.key}><Icon className="size-3 shrink-0" aria-hidden />{type.name}</span>;
}

/** Icon only, decorative: the type name is already shown as text elsewhere on the row or card. */
export function TypeGlyph({ item, className = "size-4" }: { item: Pick<Item, "item_type">; className?: string }) {
  const Icon = icons[item.item_type?.icon ?? ""] ?? Circle;
  return <Icon className={`${className} shrink-0`} aria-hidden />;
}
