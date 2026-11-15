import { useEffect, useMemo, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, type CurrentUser } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { emptyViewExpansion, viewExpansionFrom, type ViewExpansion } from "@/lib/view-expansion";

const SAVE_DELAY_MS = 400;

/**
 * Expansion for one view, read from the signed-in profile and saved to the server. Saves are batched: a burst of clicks sends one PUT.
 * The profile cache is updated at once so the screen never waits for the network.
 */
export function useViewExpansion(viewId?: string) {
  const me = useProfile();
  const queryClient = useQueryClient();
  const initial = useMemo(() => (viewId ? viewExpansionFrom(me?.view_expansion, viewId) : emptyViewExpansion), [me?.view_expansion, viewId]);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const pending = useRef<{ viewId: string; value: ViewExpansion } | undefined>(undefined);

  function send() {
    const next = pending.current;
    pending.current = undefined;
    if (!next) return;
    api(`/api/v1/me/view-expansion/${next.viewId}`, { method: "PUT", body: JSON.stringify(next.value) })
      .catch(() => toast.error("Expansi tidak tersimpan. Tampilan tetap seperti sekarang."));
  }

  function save(value: ViewExpansion) {
    if (!viewId) return;
    queryClient.setQueryData<CurrentUser>(["me"], (old) => old && {
      ...old, view_expansion: { ...old.view_expansion, [viewId]: value },
    });
    pending.current = { viewId, value };
    clearTimeout(timer.current);
    timer.current = setTimeout(send, SAVE_DELAY_MS);
  }

  // Leaving the view sends whatever is still waiting, so the last click is never lost.
  useEffect(() => () => { clearTimeout(timer.current); send(); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return { initial, save };
}
