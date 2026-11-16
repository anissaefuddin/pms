import { useEffect, useMemo, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, type CurrentUser } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { viewPrefsFrom, emptyViewPrefs, type ViewPrefs } from "@/lib/view-preferences";

const SAVE_DELAY_MS = 400;

/** Hidden views and tab order for one workspace: read from the profile, saved to the server in a debounced batch. */
export function useViewPreferences(scopeId?: string) {
  const me = useProfile();
  const queryClient = useQueryClient();
  const prefs = useMemo(() => (scopeId ? viewPrefsFrom(me?.view_prefs, scopeId) : emptyViewPrefs), [me?.view_prefs, scopeId]);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const pending = useRef<{ scopeId: string; value: ViewPrefs } | undefined>(undefined);

  function send() {
    const next = pending.current;
    pending.current = undefined;
    if (!next) return;
    api(`/api/v1/me/view-preferences/${next.scopeId}`, { method: "PUT", body: JSON.stringify(next.value) })
      .catch(() => toast.error("Pengaturan tampilan tidak tersimpan."));
  }

  function save(value: ViewPrefs) {
    if (!scopeId) return;
    queryClient.setQueryData<CurrentUser>(["me"], (old) => old && {
      ...old, view_prefs: { ...old.view_prefs, [scopeId]: value },
    });
    pending.current = { scopeId, value };
    clearTimeout(timer.current);
    timer.current = setTimeout(send, SAVE_DELAY_MS);
  }

  useEffect(() => () => { clearTimeout(timer.current); send(); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return { prefs, save };
}
