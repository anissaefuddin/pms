import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import type { components } from "@/api/schema";
import { notificationKey } from "@/lib/notifications";
import { itemKeys } from "@/lib/items";

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
      if (event.type !== "comment.created") void client.invalidateQueries({ queryKey: ["items", "todo"] });
      void client.invalidateQueries({ queryKey: itemKeys.detail(id) });
      void client.invalidateQueries({ queryKey: ["item-comments", id] });
      void client.invalidateQueries({ queryKey: ["item-attachments", id] });
      void client.invalidateQueries({ queryKey: ["shared-items"] });
      if (event.type === "item.deleted" || event.type === "item.restored") {
        void client.invalidateQueries({ queryKey: notificationKey });
        void client.invalidateQueries({ queryKey: itemKeys.all });
        void client.invalidateQueries({ queryKey: ["recycle-bin"] });
        return;
      }
      for (const container of event.container_ids ?? []) {
        void client.invalidateQueries({ queryKey: itemKeys.root(container) });
        void client.invalidateQueries({ queryKey: itemKeys.children(container) });
        void client.invalidateQueries({ queryKey: itemKeys.detail(container) });
      }
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
