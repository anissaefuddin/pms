import type { Item, ItemStatus, ItemUpdate } from "@/lib/items";

export type ItemCollectionProps = {
  items: Item[];
  statuses: ItemStatus[];
  spaceId: string;
  parentId?: string;
  canCreate: boolean;
  pending: boolean;
  onOpen: (item: Item) => void;
  onAdd: (status?: string) => void;
  onPatch: (item: Item, update: ItemUpdate) => Promise<void>;
};
