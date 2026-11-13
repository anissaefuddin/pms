import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";

type Preview = components["schemas"]["AttachmentPreview"];

/** Side-by-side view of two versions of one file (FIL-03); reuses the per-version preview endpoint's data. */
export function FileCompare({ attachmentId, versions, filename }: { attachmentId: string; versions: number[]; filename: string }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  if (versions.length < 2) return null;
  return <>
    <Button variant="outline" size="sm" onClick={() => setOpen(true)}>{t("items.compare")}</Button>
    {open && <Dialog open onOpenChange={setOpen}>
      <DialogContent className="max-w-6xl">
        <DialogHeader>
          <DialogTitle>{t("items.compareTitle", { name: filename })}</DialogTitle>
          <DialogDescription>{t("items.compareHint")}</DialogDescription>
        </DialogHeader>
        <Compare attachmentId={attachmentId} versions={versions} />
      </DialogContent>
    </Dialog>}
  </>;
}

function Compare({ attachmentId, versions }: { attachmentId: string; versions: number[] }) {
  const { t } = useTranslation();
  const [left, setLeft] = useState(versions[1]); // versions arrive newest first: default old vs new
  const [right, setRight] = useState(versions[0]);
  const [page, setPage] = useState(1);
  const query = `left_attachment=${attachmentId}&left_version=${left}&right_attachment=${attachmentId}&right_version=${right}`;
  const result = useQuery({
    queryKey: ["attachment-compare", attachmentId, left, right],
    queryFn: () => api<{ left: Preview; right: Preview }>(`/api/v1/attachment-compare?${query}`),
    refetchInterval: (q) => q.state.data && (q.state.data.left.status === "pending" || q.state.data.right.status === "pending") ? 2000 : false,
  });
  const select = (label: string, value: number, set: (v: number) => void) => <label className="text-sm">{label}
    <select className="ml-2 rounded border bg-background p-1" value={value} onChange={(e) => { set(Number(e.target.value)); setPage(1); }}>
      {versions.map((v) => <option key={v} value={v}>{t("items.versionLabel", { version: v })}</option>)}
    </select>
  </label>;
  const most = Math.max(result.data?.left.pages.length ?? 1, result.data?.right.pages.length ?? 1);
  const side = (preview: Preview, label: string) => {
    const shown = preview.pages.find((p) => p.page === page);
    return <section className="min-w-0 space-y-1" aria-label={label} data-testid="compare-side">
      <h3 className="font-medium">{label}</h3>
      {preview.status === "pending" ? <p role="status">{t("items.proofPreparing")}</p>
        : preview.status !== "ready" ? <p role="status">{t(`items.proofUnavailable.${preview.status}`)}</p>
        : shown ? <img src={`/api/v1/attachments/${preview.attachment_id}/versions/${preview.version}/preview/${page}`} alt={t("items.compareImage", { version: preview.version, page })} className="block max-h-[65vh] w-full object-contain" draggable={false} />
        : <p role="status">{t("items.compareNoPage")}</p>}
    </section>;
  };
  return <div className="space-y-3">
    <div className="flex flex-wrap items-center gap-4">
      {select(t("items.compareLeft"), left, setLeft)}
      {select(t("items.compareRight"), right, setRight)}
      {most > 1 && <span className="flex items-center gap-2">
        <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage(page - 1)}>{t("items.proofPrevious")}</Button>
        <span aria-live="polite">{t("items.proofPage", { page, count: most })}</span>
        <Button variant="outline" size="sm" disabled={page >= most} onClick={() => setPage(page + 1)}>{t("items.proofNext")}</Button>
      </span>}
    </div>
    {result.isPending ? <p role="status">{t("items.loading")}</p>
      : result.isError ? <FormError message={left === right ? t("items.compareSame") : t("items.loadError")} />
      : <div className="grid gap-3 md:grid-cols-2">
        {side(result.data.left, t("items.versionLabel", { version: left }))}
        {side(result.data.right, t("items.versionLabel", { version: right }))}
      </div>}
  </div>;
}
