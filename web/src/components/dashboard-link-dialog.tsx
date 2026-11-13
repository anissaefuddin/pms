import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { formatDateTime, useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

type Link = components["schemas"]["DashboardPublicLink"];
const days = [7, 30, 90, 365];

/** Public link of a dashboard (DASH-07): on or off, an expiry, an optional password. The link is shown once. */
export function DashboardLinkDialog({ dashboardId, open, onOpenChange }: { dashboardId: string; open: boolean; onOpenChange: (open: boolean) => void }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const qc = useQueryClient();
  const key = ["dashboard-link", dashboardId];
  const [link, setLink] = useState("");
  const status = useQuery({ queryKey: key, enabled: open, queryFn: () => api<Link>(`/api/v1/dashboards/${dashboardId}/public-link`) });
  const enable = useMutation({
    mutationFn: (body: object) => api<Link>(`/api/v1/dashboards/${dashboardId}/public-link`, { method: "PUT", body: JSON.stringify(body) }),
    onSuccess: async (made) => { setLink(`${window.location.origin}/d/${made.token}`); await qc.invalidateQueries({ queryKey: key }); },
  });
  const disable = useMutation({
    mutationFn: () => api(`/api/v1/dashboards/${dashboardId}/public-link`, { method: "DELETE" }),
    onSuccess: async () => { setLink(""); await qc.invalidateQueries({ queryKey: key }); },
  });
  const error = enable.error ?? disable.error ?? status.error;
  const on = status.data?.enabled;
  return <Dialog open={open} onOpenChange={(next) => { if (!next) setLink(""); onOpenChange(next); }}>
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{t("dashboards.link.title")}</DialogTitle>
        <DialogDescription>{t("dashboards.link.hint")}</DialogDescription>
      </DialogHeader>
      {error && <p role="alert">{error.message}</p>}
      {status.data && <p role="status">{on ? t("dashboards.link.on", { date: formatDateTime(status.data.expires_at!, profile), password: status.data.has_password ? t("dashboards.link.withPassword") : "" }) : t("dashboards.link.off")}</p>}
      <form className="space-y-3" onSubmit={(event) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        const expires = new Date(Date.now() + Number(data.get("days")) * 86_400_000);
        enable.mutate({ expires_at: expires.toISOString(), ...(data.get("password") ? { password: String(data.get("password")) } : {}) });
      }}>
        <label className="block text-sm">{t("dashboards.link.expires")}
          <select name="days" defaultValue="30" className="block rounded border bg-background p-2">
            {days.map((d) => <option key={d} value={d}>{t("dashboards.link.days", { count: d })}</option>)}
          </select></label>
        <label className="block text-sm">{t("dashboards.link.password")}<Input name="password" type="password" autoComplete="new-password" /></label>
        <div className="flex gap-2">
          <Button type="submit" disabled={enable.isPending}>{t(on ? "dashboards.link.replace" : "dashboards.link.enable")}</Button>
          {on && <Button type="button" variant="outline" disabled={disable.isPending} onClick={() => disable.mutate()}>{t("dashboards.link.disable")}</Button>}
        </div>
      </form>
      {link && <div role="status" className="space-y-1 rounded border p-3 text-sm">
        <p>{t("dashboards.link.once")}</p>
        <Input readOnly value={link} aria-label={t("dashboards.link.url")} onFocus={(event) => event.currentTarget.select()} />
      </div>}
    </DialogContent>
  </Dialog>;
}
