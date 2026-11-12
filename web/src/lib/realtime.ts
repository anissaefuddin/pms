import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import type { components } from "@/api/schema";
import { notificationKey } from "@/lib/notifications";
import { isProjectDetail, itemKeys, type Item } from "@/lib/items";

const events = ["item.created", "item.updated", "item.deleted", "item.restored", "comment.created", "notification.created", "resync"];

export function useRealtime(enabled: boolean) {
  const client = useQueryClient();
  const [live, setLive] = useState(false);
  useEffect(() => {
    if (!enabled) return;
    let source: EventSource;
    let retry: ReturnType<typeof setTimeout> | undefined;
    function update(message: MessageEvent<string>) {
      const event: components["schemas"]["RealtimeEvent"] = JSON.parse(message.data);
      if (event.type === "resync") {
        void client.invalidateQueries();
        return;
      }
      if (event.type === "notification.created") {
        void client.invalidateQueries({ queryKey: notificationKey });
        void client.invalidateQueries({ queryKey: ["shared-items"] });
        void client.invalidateQueries({ queryKey: ["spaces"] });
        return;
      }
      const id = event.item_id ?? event.id;
      void client.invalidateQueries({ queryKey: ["item-comments", id] });
      void client.invalidateQueries({ queryKey: ["item-attachments", id] });
      void client.invalidateQueries({ queryKey: ["shared-items"] });
      if (event.type === "item.deleted" || event.type === "item.restored") {
        void client.invalidateQueries({ queryKey: notificationKey });
        void client.invalidateQueries({ queryKey: itemKeys.all });
        void client.invalidateQueries({ queryKey: ["recycle-bin"] });
        return;
      }
      void client.invalidateQueries({ queryKey: ["gantt-dependencies"] });
      const itemChanged = event.type !== "comment.created";
      const containers = new Set(event.container_ids ?? []);
      const updatedItems = new Set([id, ...(event.affected_items ?? []).map((item) => item.id)]);
      void client.invalidateQueries({ queryKey: itemKeys.all, predicate: (query) => {
        const [, kind, key] = query.queryKey;
        const inContainer = typeof key === "string" && containers.has(key);
        if (kind === "todo") return itemChanged;
        if (kind === "detail") {
          return inContainer || (typeof key === "string" && updatedItems.has(key)) || (itemChanged && isProjectDetail(query));
        }
        if (kind !== "root" && kind !== "children") return false;
        // SSE names direct containers. A deeper ancestor may exist outside the cached tree.
        return inContainer || (itemChanged && ((query.state.data as Item[] | undefined)?.some((item) => item.type === "project") ?? false));
      } });
    }
    function connect(restarted = false) {
      source = new EventSource("/api/v1/events");
      source.onopen = () => {
        setLive(true);
        if (restarted) void client.invalidateQueries();
      };
      source.onerror = () => {
        setLive(false);
        if (source.readyState === EventSource.CLOSED) {
          void client.invalidateQueries({ queryKey: ["me"] });
          retry = setTimeout(() => connect(true), 3000);
        }
      };
      for (const event of events) source.addEventListener(event, update);
    }
    connect();
    return () => { clearTimeout(retry); source.close(); };
  }, [client, enabled]);
  return enabled && live;
}
