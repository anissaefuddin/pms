import { CheckSquare, Folder, FolderKanban, Shapes, type LucideIcon } from "lucide-react";

// item_type_defs.icon is a free lucide-style name; the three built-ins are drawn, anything else gets a generic shape.
const icons: Record<string, LucideIcon> = { "check-square": CheckSquare, "folder-kanban": FolderKanban, folder: Folder };

export function TypeIcon({ name }: { name?: string }) {
  const Icon = icons[name ?? ""] ?? Shapes;
  return <Icon className="size-4 shrink-0" aria-hidden="true" />;
}
