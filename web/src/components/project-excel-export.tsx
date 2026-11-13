import { useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import type { Item } from "@/lib/items";
import { useSpaces } from "@/lib/space-queries";
import { Button, buttonVariants } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { FormError } from "@/components/auth/form-error";

type ExportJob = { id: string; status: "pending" | "ready" | "failed"; error?: string };

export function ProjectExcelExport({ item, open, onOpenChange }: {
  item: Item;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation();
  return <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent>
      <DialogTitle>{t("excel.exportTitle")}</DialogTitle>
      <DialogDescription>{t("excel.exportDescription")}</DialogDescription>
      {open && <ExportForm item={item} onClose={() => onOpenChange(false)} />}
    </DialogContent>
  </Dialog>;
}

function ExportForm({ item, onClose }: { item: Item; onClose: () => void }) {
  const { t } = useTranslation();
  const { data: spaces } = useSpaces();
  const choices = [...new Map(item.locations.map((location) => [location.space_id, location.space_name])).entries()];
  const [selected, setSelected] = useState("");
  const [job, setJob] = useState<ExportJob>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const target = choices.some(([id]) => id === selected) ? selected : choices[0]?.[0] ?? "";
  const path = job ? `/api/v1/excel-exports/${job.id}` : "";
  const status = useQuery({
    queryKey: ["excel-export", job?.id],
    queryFn: ({ signal }) => api<ExportJob>(path, { signal }),
    enabled: job?.status === "pending",
    refetchInterval: ({ state }) => state.status !== "error" && (!state.data || state.data.status === "pending") ? 1000 : false,
    retry: false,
  });
  const current = status.data ?? job;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!target || busy || current?.status === "pending") return;
    setJob(undefined);
    setBusy(true);
    setError("");
    try {
      setJob(await api<ExportJob>(`/api/v1/items/${item.id}/excel-exports`, {
        method: "POST", body: JSON.stringify({ space_id: target }),
      }));
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return <form onSubmit={submit} className="space-y-4">
    <label className="block space-y-1">
      <span>{t("items.space")}</span>
      <select value={target} onChange={(event) => setSelected(event.target.value)}
        className="block w-full rounded border bg-background p-2" disabled={busy || current?.status === "pending" || current?.status === "ready"}>
        {choices.map(([id, name]) => <option key={id} value={id}>{name || spaces?.find((space) => space.id === id)?.name || id}</option>)}
      </select>
    </label>
    {current?.status === "pending" && !status.error && <p role="status">{t("excel.exportPending")}</p>}
    {current?.status === "ready" && <p role="status">{t("excel.exportReady")}</p>}
    <FormError message={error || status.error?.message || (current?.status === "failed" ? current.error || t("excel.exportFailed") : "")} />
    <DialogFooter>
      <Button variant="outline" onClick={onClose}>{t("excel.close")}</Button>
      {current?.status === "ready"
        ? <a className={buttonVariants()} href={path + "/file"} download>{t("excel.download")}</a>
        : status.error
          ? <Button type="button" disabled={status.isFetching} onClick={() => void status.refetch()}>{t("items.retry")}</Button>
          : <Button type="submit" disabled={!target || busy || current?.status === "pending"}>{t("excel.startExport")}</Button>}
    </DialogFooter>
  </form>;
}
