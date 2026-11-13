import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { formatDateTime, useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

type Snapshot = components["schemas"]["GanttSnapshot"];
const days = [7, 30, 90];

/** Public Gantt snapshots (SPC-16): a frozen, read-only copy behind a secret link. The link is shown once. */
export function GanttSnapshotDialog({ projectId, open, onOpenChange }: { projectId: string; open: boolean; onOpenChange: (open: boolean) => void }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const qc = useQueryClient();
  const key = ["gantt-snapshots", projectId];
  const [link, setLink] = useState("");
  const list = useQuery({ queryKey: key, enabled: open, queryFn: () => api<{ items: Snapshot[] }>(`/api/v1/items/${projectId}/gantt-snapshots`) });
  const create = useMutation({
    mutationFn: (body: object) => api<Snapshot>(`/api/v1/items/${projectId}/gantt-snapshots`, { method: "POST", body: JSON.stringify(body) }),
    onSuccess: async (made) => { setLink(`${window.location.origin}/g/${made.token}`); await qc.invalidateQueries({ queryKey: key }); },
  });
  const revoke = useMutation({
    mutationFn: (id: string) => api(`/api/v1/gantt-snapshots/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
  });
  const error = create.error ?? revoke.error ?? list.error;
  return <Dialog open={open} onOpenChange={(next) => { if (!next) setLink(""); onOpenChange(next); }}>
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{t("views.snapshot.title")}</DialogTitle>
        <DialogDescription>{t("views.snapshot.hint")}</DialogDescription>
      </DialogHeader>
      {error && <p role="alert">{error.message}</p>}
      <form className="space-y-3" onSubmit={(event) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        const expires = new Date(Date.now() + Number(data.get("days")) * 86_400_000);
        create.mutate({ name: String(data.get("name")), expires_at: expires.toISOString(), show_assignees: data.get("assignees") === "on" });
      }}>
        <label className="block text-sm">{t("views.snapshot.name")}<Input name="name" required maxLength={120} /></label>
        <label className="block text-sm">{t("views.snapshot.expires")}
          <select name="days" defaultValue="30" className="block rounded border bg-background p-2">
            {days.map((d) => <option key={d} value={d}>{t("views.snapshot.days", { count: d })}</option>)}
          </select></label>
        <label className="flex items-center gap-2 text-sm"><input type="checkbox" name="assignees" />{t("views.snapshot.assignees")}</label>
        <Button type="submit" disabled={create.isPending}>{t("views.snapshot.create")}</Button>
      </form>
      {link && <div role="status" className="space-y-1 rounded border p-3 text-sm">
        <p>{t("views.snapshot.linkOnce")}</p>
        <Input readOnly value={link} aria-label={t("views.snapshot.link")} onFocus={(event) => event.currentTarget.select()} />
      </div>}
      <section aria-label={t("views.snapshot.history")} className="space-y-1">
        <h3 className="font-medium">{t("views.snapshot.history")}</h3>
        {list.data && !list.data.items.length && <p className="text-sm text-muted-foreground">{t("views.snapshot.empty")}</p>}
        <ul className="space-y-1">
          {list.data?.items.map((snapshot) => <li key={snapshot.id} className="flex flex-wrap items-center gap-2 text-sm">
            <span className="font-medium">{snapshot.name}</span>
            <span className="text-muted-foreground">{t("views.snapshot.until", { date: formatDateTime(snapshot.expires_at, profile) })}</span>
            <Button size="sm" variant="outline" disabled={revoke.isPending} onClick={() => revoke.mutate(snapshot.id)}
              aria-label={t("views.snapshot.revokeNamed", { name: snapshot.name })}>{t("views.snapshot.revoke")}</Button>
          </li>)}
        </ul>
      </section>
    </DialogContent>
  </Dialog>;
}
