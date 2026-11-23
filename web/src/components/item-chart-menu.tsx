import { MoreHorizontal } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSub, DropdownMenuSubContent, DropdownMenuSubTrigger, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import type { Item, ItemStatus, ItemUpdate } from "@/lib/items";

export function ItemChartMenu({ item, statuses, readonly = false, onOpen, onPatch, onDelete }: {
  item: Item; statuses: ItemStatus[]; onOpen: (item: Item) => void;
  readonly?: boolean; onPatch: (item: Item, update: ItemUpdate) => Promise<void>; onDelete: (item: Item) => void;
}) {
  const { t } = useTranslation();
  const editable = !readonly && item.my_capabilities.includes("edit");
  const removable = !readonly && item.my_capabilities.includes("delete");
  return <DropdownMenu>
    <DropdownMenuTrigger render={<Button size="icon-sm" variant="ghost" aria-label={`${t("items.actions")}: ${item.title}`}
      className="size-5 bg-card/80 opacity-0 shadow-sm group-focus-within:opacity-100 group-hover:opacity-100 aria-expanded:opacity-100" onPointerDown={(event) => event.stopPropagation()} />}>
      <MoreHorizontal />
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end">
      <DropdownMenuItem onClick={() => onOpen(item)}>{t("items.chips.fullDetails")}</DropdownMenuItem>
      <DropdownMenuItem onClick={async () => {
        try { await navigator.clipboard.writeText(new URL(`/tasks/${item.id}`, window.location.origin).href); toast.success(t("items.copied")); }
        catch (error) { toast.error((error as Error).message); }
      }}>{t("items.copyLink")}</DropdownMenuItem>
      {editable && statuses.length > 0 && <DropdownMenuSub>
        <DropdownMenuSubTrigger>{t("items.status")}</DropdownMenuSubTrigger>
        <DropdownMenuSubContent>{statuses.map((status) => <DropdownMenuItem key={status.id} disabled={status.id === item.status?.id}
          onClick={() => void onPatch(item, { status_id: status.id })}>{status.name}</DropdownMenuItem>)}</DropdownMenuSubContent>
      </DropdownMenuSub>}
      {removable && <DropdownMenuItem variant="destructive" onClick={() => onDelete(item)}>{t("items.delete")}</DropdownMenuItem>}
    </DropdownMenuContent>
  </DropdownMenu>;
}
