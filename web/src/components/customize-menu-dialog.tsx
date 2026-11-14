import { useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, type CurrentUser } from "@/lib/api";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";

export const sidebarChoices = ["my_todo", "request_forms", "stream", "calendars", "created_by_me", "dashboards", "reports", "starred", "workload", "timesheets"] as const;
type Choice = (typeof sidebarChoices)[number];

/** Hidden menu entries, saved at once (optimistic, rolled back on a failed PATCH). */
export function useSidebarHidden(profile: CurrentUser) {
  const client = useQueryClient();
  const [error, setError] = useState("");
  const [hidden, setHidden] = useState(profile.sidebar_hidden ?? []);
  async function toggle(key: Choice, visible: boolean) {
    setError("");
    const previous = hidden;
    const next = visible ? hidden.filter((entry) => entry !== key) : [...hidden, key];
    setHidden(next);
    try {
      client.setQueryData(["me"], await api<CurrentUser>("/api/v1/me", { method: "PATCH", body: JSON.stringify({ sidebar_hidden: next }) }));
    } catch (err) {
      setHidden(previous);
      setError((err as Error).message);
    }
  }
  return { hidden, toggle, error };
}

// Two lists as in the reference "Customize menu" (Shown in menu / Hidden from menu); no drag order, the server stores only the hidden keys.
export function CustomizeMenuDialog({ me, open, onOpenChange }: { me: CurrentUser; open: boolean; onOpenChange: (open: boolean) => void }) {
  const { t } = useTranslation();
  const { hidden, toggle, error } = useSidebarHidden(me);
  const row = (key: Choice, visible: boolean) => <li key={key} className="flex items-center justify-between rounded px-2 py-1.5 text-sm hover:bg-muted">
    <span className={visible ? "" : "text-muted-foreground"}>{t(`settings.profile.sidebar.${key}`)}</span>
    <Button variant="ghost" size="icon-sm" aria-pressed={visible}
      aria-label={t(visible ? "nav.customize.hide" : "nav.customize.show", { name: t(`settings.profile.sidebar.${key}`) })}
      onClick={() => void toggle(key, !visible)}>{visible ? <Eye /> : <EyeOff />}</Button>
  </li>;
  return <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent className="max-w-md">
      <DialogTitle className="text-lg font-semibold">{t("nav.customize.label")}</DialogTitle>
      <DialogDescription>{t("nav.customize.hint")}</DialogDescription>
      <section aria-label={t("nav.customize.shown")}>
        <h3 className="text-xs font-semibold text-muted-foreground">{t("nav.customize.shown")}</h3>
        <ul>{sidebarChoices.filter((key) => !hidden.includes(key)).map((key) => row(key, true))}</ul>
      </section>
      <section aria-label={t("nav.customize.hidden")}>
        <h3 className="text-xs font-semibold text-muted-foreground">{t("nav.customize.hidden")}</h3>
        <ul>{sidebarChoices.filter((key) => hidden.includes(key)).map((key) => row(key, false))}</ul>
      </section>
      <FormError message={error} />
      <div className="flex justify-end"><Button onClick={() => onOpenChange(false)}>{t("nav.customize.done")}</Button></div>
    </DialogContent>
  </Dialog>;
}
