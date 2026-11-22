import type { CustomField } from "@/lib/custom-fields";
import type { Item, ItemStatus, ItemUpdate } from "@/lib/items";

export type ItemCollectionProps = {
  items: Item[];
  statuses: ItemStatus[];
  spaceId: string;
  parentId?: string;
  canCreate: boolean;
  pending: boolean;
  onOpen: (item: Item) => void;
  /** Opens Create. A group passes the status, the type or the assignee it belongs to, so the new item starts there. */
  onAdd: (status?: string, typeKey?: string, assignee?: { id: string; name: string }) => void;
  /** Space custom field definitions and the field ids the view shows (as `field:<id>` columns). */
  customFields: CustomField[];
  columns: string[];
  onPatch: (item: Item, update: ItemUpdate) => Promise<void>;
  /** B-02: applies one update to each item in turn. Reports what was updated and what was not; never repeats an update that succeeded. */
  onBulkPatch?: (items: Item[], update: ItemUpdate) => Promise<{ done: number; failed: { title: string; message: string }[] }>;
};
