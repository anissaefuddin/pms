import { Bug, CheckSquare, ClipboardList, Calendar, File, Flag, Folder, FolderKanban, Lightbulb, Package, Shapes, Star, Target, Wrench, type LucideIcon } from "lucide-react";

// item_type_defs.icon is a lucide-style name. The curated names below are the ones an administrator can pick; the three
// built-ins keep their own drawings. Any other name gets a generic shape.
const icons: Record<string, LucideIcon> = {
  "check-square": CheckSquare, "folder-kanban": FolderKanban, folder: Folder, shapes: Shapes, file: File, bug: Bug,
  lightbulb: Lightbulb, "clipboard-list": ClipboardList, flag: Flag, target: Target, package: Package, wrench: Wrench,
  calendar: Calendar, star: Star,
};

export const TYPE_ICON_NAMES = Object.keys(icons);

export function TypeIcon({ name }: { name?: string }) {
  const Icon = icons[name ?? ""] ?? Shapes;
  return <Icon className="size-4 shrink-0" aria-hidden="true" />;
}
