import { useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import type { Item } from "@/lib/items";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { FileCompare } from "@/components/file-compare";
import { uploadWithProgress } from "@/lib/upload";
import { progressPercent } from "@/lib/upload-progress";
import { FileProofing } from "@/components/file-proofing";
import { reviewStateFor, type ApprovalLike } from "@/lib/file-review";
import { ConfirmDialog } from "@/components/confirm-dialog";

type Attachment = components["schemas"]["Attachment"];
type Version = components["schemas"]["AttachmentVersion"];
const attachmentPath = (id: string) => `/api/v1/attachments/${id}`;

export function ItemAttachments({ item, userId }: { item: Item; userId: string }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const path = `/api/v1/items/${item.id}/attachments`;
  const queryKey = ["item-attachments", item.id];
  const { data, isPending, isError } = useQuery({
    queryKey, queryFn: () => api<{ items: Attachment[]; max_upload_bytes: number }>(path),
  });
  const input = useRef<HTMLInputElement>(null);
  const working = useRef(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [removing, setRemoving] = useState<Attachment>();
  const [linkUrl, setLinkUrl] = useState("");
  const [history, setHistory] = useState<string>();
  const [replacing, setReplacing] = useState<string>();
  const versionInput = useRef<HTMLInputElement>(null);
  const canAttach = item.my_capabilities.includes("attach");

  async function change(operation: () => Promise<unknown>) {
    if (working.current) return;
    working.current = true;
    setBusy(true);
    setError("");
    try {
      await operation();
      setRemoving(undefined);
      await client.invalidateQueries({ queryKey });
    } catch (err) {
      setError((err as Error).message);
    } finally {
      working.current = false;
      setBusy(false);
    }
  }

  // D-05: one file at a time with real progress. A failed upload keeps the same File object so "Try again" re-sends it.
  const [transfer, setTransfer] = useState<{ file: File; percent?: number; error?: string }>();
  function startUpload(file: File) {
    if (working.current) return;
    working.current = true;
    setBusy(true);
    setTransfer({ file });
    const body = new FormData();
    body.append("file", file);
    uploadWithProgress(path, body, (progress) => setTransfer((current) => current && { ...current, percent: progress.computable ? progressPercent(progress.loaded, progress.total) : undefined }))
      .then(async () => {
        setTransfer(undefined);
        await client.invalidateQueries({ queryKey });
      }, (err: Error) => setTransfer((current) => current && { ...current, error: err.message }))
      .finally(() => {
        working.current = false;
        setBusy(false);
      });
  }
  function upload(files: FileList | null) {
    if (!files?.length || working.current) return;
    if (files.length !== 1) {
      setError(t("items.oneAttachment"));
      return;
    }
    startUpload(files[0]);
  }

  function uploadVersion(files: FileList | null) {
    if (files?.length !== 1 || !replacing || working.current) return;
    const body = new FormData();
    body.append("file", files[0]);
    const id = replacing;
    void change(async () => {
      await api(`${attachmentPath(id)}/versions`, { method: "POST", body });
      await client.invalidateQueries({ queryKey: ["attachment-versions", id] });
    });
  }

  return <section className="space-y-2 border-t pt-4">
    <h3 className="font-medium">{t("items.attachments")}</h3>
    {isPending && <p role="status">{t("items.loading")}</p>}
    {isError && <FormError message={t("items.loadError")} />}
    <ul className="divide-y">
      {data?.items.map((file) => {
        const canRemove = item.my_capabilities.includes("delete") || (canAttach && file.uploader.id === userId);
        return <li key={file.id} className="py-1">
        <div className="flex items-center justify-between gap-2">
          {file.link_url
            ? <a className="min-w-0 break-all underline" href={file.link_url} target="_blank" rel="noopener noreferrer">{file.filename}</a>
            : <a className="min-w-0 break-all underline" href={attachmentPath(file.id) + "/download"} download>{file.filename}</a>}
          <span className="flex shrink-0 items-center gap-1">
          {!file.link_url && <Button variant="ghost" size="sm" aria-expanded={history === file.id} onClick={() => setHistory(history === file.id ? undefined : file.id)}>
            {t("items.versionLabel", { version: file.version })}
          </Button>}
          {canAttach && !file.link_url && <Button variant="ghost" size="sm" disabled={busy} onClick={() => {
            setReplacing(file.id);
            versionInput.current?.click();
          }}>{t("items.uploadVersion")}</Button>}
          {canRemove && <Button variant="ghost" size="sm" disabled={busy} onClick={() => {
            setError("");
            setRemoving(file);
          }}>{t("items.removeAttachment")}</Button>}
          </span>
        </div>
        {history === file.id && <VersionHistory itemId={item.id} attachmentId={file.id} canComment={item.my_capabilities.includes("comment")} />}
        </li>;
      })}
    </ul>
    <input type="file" ref={versionInput} hidden aria-label={t("items.uploadVersion")} onChange={(event) => {
      uploadVersion(event.currentTarget.files);
      event.currentTarget.value = "";
    }} />
    {canAttach && <div className="rounded border border-dashed p-3" onDragOver={(event) => event.preventDefault()}
      onDrop={(event) => { event.preventDefault(); upload(event.dataTransfer.files); }}>
      <input type="file" ref={input} hidden onChange={(event) => {
        upload(event.currentTarget.files);
        event.currentTarget.value = "";
      }} />
      <Button variant="outline" disabled={busy} onClick={() => input.current?.click()}>{t("items.chooseAttachment")}</Button>
      <p className="mt-1 text-sm text-muted-foreground">{t("items.dropAttachment")}</p>
      {transfer && <div className="mt-2 space-y-1 text-sm" role="status">
        {transfer.error ? <p role="alert">{t("items.upload.failed", { message: transfer.error })}
          <Button variant="link" className="px-1" onClick={() => startUpload(transfer.file)}>{t("items.upload.retry")}</Button></p>
          : <>
            <p>{t("items.upload.uploading", { name: transfer.file.name })}</p>
            <progress className="h-2 w-full" aria-label={t("items.upload.uploading", { name: transfer.file.name })}
              {...(transfer.percent !== undefined ? { value: transfer.percent, max: 100 } : {})} />
            <p className="text-xs text-muted-foreground">{transfer.percent !== undefined ? t("items.upload.progress", { percent: transfer.percent }) : t("items.upload.pending")}</p>
          </>}
      </div>}
      {!!data?.max_upload_bytes && <p className="text-sm text-muted-foreground">
        {t("items.attachmentLimit", { limit: data.max_upload_bytes / 2 ** 20 })}
      </p>}
    </div>}
    {canAttach && <form className="flex gap-2" onSubmit={(event) => {
      event.preventDefault();
      const url = linkUrl.trim();
      if (url) void change(async () => { await api(`/api/v1/items/${item.id}/attachment-links`, { method: "POST", body: JSON.stringify({ url }) }); setLinkUrl(""); });
    }}>
      <input className="min-w-0 flex-1 rounded border px-2 py-1" type="url" value={linkUrl} aria-label={t("items.linkUrl")} onChange={(event) => setLinkUrl(event.target.value)} />
      <Button type="submit" variant="outline" disabled={busy || !linkUrl.trim()}>{t("items.addLink")}</Button>
    </form>}
    {!removing && <FormError message={error} />}
    <ConfirmDialog open={!!removing} onOpenChange={(open) => { if (!open) setRemoving(undefined); }}
      title={t("items.removeAttachment")} description={removing?.filename ?? ""}
      confirmLabel={t("items.removeAttachment")} cancelLabel={t("items.cancel")} busy={busy} error={error}
      onConfirm={() => removing ? change(() => api(attachmentPath(removing.id), { method: "DELETE" })) : undefined} />
  </section>;
}

function VersionHistory({ itemId, attachmentId, canComment }: { itemId: string; attachmentId: string; canComment: boolean }) {
  const { t } = useTranslation();
  const { data, isPending, isError } = useQuery({
    queryKey: ["attachment-versions", attachmentId],
    queryFn: () => api<{ items: Version[] }>(`${attachmentPath(attachmentId)}/versions`),
  });
  // WA-D08: each version shows its own review status. Review is bound to attachment + version, so a new version starts "not requested".
  const approvals = useQuery({
    queryKey: ["item-approvals", itemId],
    queryFn: () => api<{ items: ApprovalLike[] }>(`/api/v1/items/${itemId}/approvals`),
  });
  // WA-D08: withdrawn reviews are read from the event feed; events carry the file version they were about.
  const events = useQuery({
    queryKey: ["item-approval-events", itemId],
    queryFn: () => api<{ items: { kind: string; attachment_id?: string | null; attachment_version?: number | null }[] }>(`/api/v1/items/${itemId}/approval-events`),
  });
  const withdrawn = (events.data?.items ?? []).filter((event) => event.kind === "withdrawn");
  const load = approvals.isError ? "error" : approvals.data ? "ok" : "loading";
  if (isPending) return <p role="status">{t("items.loading")}</p>;
  if (isError) return <FormError message={t("items.loadError")} />;
  return <ol className="mt-1 space-y-1 pl-4 text-sm" aria-label={t("items.versionHistory")}>
    {data.items.map((v) => <li key={v.version} className="flex flex-wrap gap-x-2">
      <a className="underline" href={`${attachmentPath(attachmentId)}/versions/${v.version}/download`} download>
        {t("items.versionLabel", { version: v.version })} {v.filename}
      </a>
      <span className="text-muted-foreground">{v.uploader.name} · {new Date(v.created_at).toLocaleString()}{v.is_current ? ` · ${t("items.currentVersion")}` : ""}</span>
      <span className="rounded bg-muted px-1.5 text-xs" data-review-state={reviewStateFor(approvals.data?.items, attachmentId, v.version, load, withdrawn).state}>
        {t(`items.reviewState.${reviewStateFor(approvals.data?.items, attachmentId, v.version, load, withdrawn).state}`)}
      </span>
      <FileProofing attachmentId={attachmentId} version={v.version} filename={v.filename} canComment={canComment} />
    </li>)}
    {data.items.length > 1 && <li><FileCompare attachmentId={attachmentId} versions={data.items.map((v) => v.version)} filename={data.items[0]?.filename ?? ""} /></li>}
  </ol>;
}
