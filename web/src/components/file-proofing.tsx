import { useState, type MouseEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";

type Preview = components["schemas"]["AttachmentPreview"];
type Annotation = components["schemas"]["AttachmentAnnotation"];

/** Proofing viewer for one file version (FIL-02): the rendered pages with comments pinned at relative positions. */
export function FileProofing({ attachmentId, version, filename, canComment }: { attachmentId: string; version: number; filename: string; canComment: boolean }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  return <>
    <Button variant="ghost" size="sm" onClick={() => setOpen(true)} aria-label={t("items.proofOpen", { name: filename })}>{t("items.proof")}</Button>
    {open && <Dialog open onOpenChange={setOpen}>
      <DialogContent className="max-w-4xl">
        <DialogHeader>
          <DialogTitle>{filename} · {t("items.versionLabel", { version })}</DialogTitle>
          <DialogDescription>{canComment ? t("items.proofHint") : t("items.proofReadOnly")}</DialogDescription>
        </DialogHeader>
        <Viewer attachmentId={attachmentId} version={version} canComment={canComment} />
      </DialogContent>
    </Dialog>}
  </>;
}

function Viewer({ attachmentId, version, canComment }: { attachmentId: string; version: number; canComment: boolean }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const base = `/api/v1/attachments/${attachmentId}/versions/${version}`;
  const previewKey = ["attachment-preview", attachmentId, version];
  const notesKey = ["attachment-annotations", attachmentId, version];
  const preview = useQuery({
    queryKey: previewKey, queryFn: () => api<Preview>(`${base}/preview`),
    refetchInterval: (query) => query.state.data?.status === "pending" ? 2000 : false,
  });
  const notes = useQuery({ queryKey: notesKey, queryFn: () => api<{ items: Annotation[] }>(`${base}/annotations`) });
  const [page, setPage] = useState(1);
  const [pin, setPin] = useState<{ x: number; y: number }>();
  const [body, setBody] = useState("");
  const [error, setError] = useState("");
  const refresh = () => client.invalidateQueries({ queryKey: notesKey });
  const run = useMutation({
    mutationFn: (request: () => Promise<unknown>) => request(),
    onSuccess: async () => { setError(""); await refresh(); },
    onError: (err) => setError((err as Error).message),
  });

  const pages = preview.data?.pages ?? [];
  const shown = (notes.data?.items ?? []).filter((n) => n.page === page);
  function place(event: MouseEvent<HTMLDivElement>) {
    if (!canComment) return;
    const box = event.currentTarget.getBoundingClientRect();
    setPin({ x: Math.min(1, Math.max(0, (event.clientX - box.left) / box.width)), y: Math.min(1, Math.max(0, (event.clientY - box.top) / box.height)) });
  }
  function save() {
    if (!pin || !body.trim()) return;
    run.mutate(() => api(`${base}/annotations`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ page, ...pin, body: body.trim() }) }), {
      onSuccess: () => { setPin(undefined); setBody(""); },
    });
  }

  if (preview.isPending || preview.data?.status === "pending") return <p role="status">{t("items.proofPreparing")}</p>;
  if (preview.isError) return <FormError message={t("items.loadError")} />;
  if (preview.data.status !== "ready") return <p role="status">{t(`items.proofUnavailable.${preview.data.status}`)}</p>;
  return <div className="grid gap-3 md:grid-cols-[1fr_16rem]">
    <div className="space-y-2">
      {pages.length > 1 && <div className="flex items-center gap-2">
        <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => { setPage(page - 1); setPin(undefined); }}>{t("items.proofPrevious")}</Button>
        <span aria-live="polite">{t("items.proofPage", { page, count: pages.length })}</span>
        <Button variant="outline" size="sm" disabled={page >= pages.length} onClick={() => { setPage(page + 1); setPin(undefined); }}>{t("items.proofNext")}</Button>
      </div>}
      <div className="relative max-h-[70vh] overflow-auto rounded border" data-testid="proof-surface">
        <div className="relative inline-block min-w-full cursor-crosshair" onClick={place}>
          <img src={`${base}/preview/${page}`} alt={t("items.proofImage", { page })} className="block w-full" draggable={false} />
          {shown.map((n, i) => <span key={n.id} title={n.body} data-testid="proof-pin"
            className={`absolute grid size-5 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-full text-xs text-white ${n.resolved ? "bg-muted-foreground" : "bg-primary"}`}
            style={{ left: `${n.x * 100}%`, top: `${n.y * 100}%` }}>{i + 1}</span>)}
          {pin && <span className="absolute size-3 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-primary bg-background" style={{ left: `${pin.x * 100}%`, top: `${pin.y * 100}%` }} />}
        </div>
      </div>
      {pin && <form className="space-y-2" onSubmit={(e) => { e.preventDefault(); save(); }}>
        <label className="block text-sm">{t("items.proofComment")}
          <textarea className="mt-1 w-full rounded border bg-background p-2" rows={2} maxLength={4000} value={body} onChange={(e) => setBody(e.target.value)} autoFocus />
        </label>
        <div className="flex gap-2">
          <Button type="submit" size="sm" disabled={run.isPending || !body.trim()}>{t("items.proofPost")}</Button>
          <Button type="button" variant="outline" size="sm" onClick={() => setPin(undefined)}>{t("items.cancel")}</Button>
        </div>
      </form>}
      <FormError message={error} />
    </div>
    <ol className="space-y-2 text-sm" aria-label={t("items.proofComments")}>
      {shown.length === 0 && <li className="text-muted-foreground">{t("items.proofNone")}</li>}
      {shown.map((n, i) => <li key={n.id} className="rounded border p-2">
        <p className="font-medium">{i + 1}. {n.author.name}{n.resolved ? ` · ${t("items.proofResolved")}` : ""}</p>
        <p className="break-words">{n.body}</p>
        {canComment && <div className="mt-1 flex gap-1">
          <Button variant="ghost" size="sm" onClick={() => run.mutate(() => api(`${base}/annotations/${n.id}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ resolved: !n.resolved }) }))}>
            {n.resolved ? t("items.proofReopen") : t("items.proofResolve")}
          </Button>
          <Button variant="ghost" size="sm" onClick={() => run.mutate(() => api(`${base}/annotations/${n.id}`, { method: "DELETE" }))}>{t("items.proofDelete")}</Button>
        </div>}
      </li>)}
    </ol>
  </div>;
}
