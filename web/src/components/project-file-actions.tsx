import { useRef, useState } from "react";
import { Download, ExternalLink, FolderOpen, Link2, MoreHorizontal, Trash2, Upload } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { progressPercent, uploadWithProgress } from "@/lib/upload";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Button, buttonVariants } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";

type FileRow = components["schemas"]["ProjectFile"];

// The Action column of the Files view: download or open stays visible, the rest is in a menu. What is offered follows the rights
// the server sends with the row; the server checks them again.
export function FileActions({ file, onOpenOwner, onChanged }: { file: FileRow; onOpenOwner?: () => void; onChanged: () => Promise<unknown> }) {
  const { t } = useTranslation();
  const input = useRef<HTMLInputElement>(null);
  const [removing, setRemoving] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [progress, setProgress] = useState<string>();
  const name = file.filename || t("views.files.linkOnly");
  const download = `/api/v1/attachments/${file.id}/download`;
  const icon = buttonVariants({ variant: "ghost", size: "icon-sm" });

  async function copy() {
    try {
      await navigator.clipboard.writeText(file.link_url ?? new URL(download, window.location.origin).href);
      toast.success(t("views.files.copied"));
    } catch { toast.error(t("views.files.copyFailed")); }
  }
  async function upload(next: File) {
    setBusy(true); setError(""); setProgress(t("items.upload.uploading", { name: next.name }));
    const body = new FormData();
    body.append("file", next);
    try {
      await uploadWithProgress(`/api/v1/attachments/${file.id}/versions`, body, (p) => {
        if (p.computable) setProgress(`${t("items.upload.uploading", { name: next.name })} ${progressPercent(p.loaded, p.total)}%`);
      });
      toast.success(t("views.files.versionAdded"));
      await onChanged();
    } catch (err) { setError(t("items.upload.failed", { message: (err as Error).message })); }
    finally { setBusy(false); setProgress(undefined); }
  }
  async function remove() {
    setBusy(true); setError("");
    try {
      await api(`/api/v1/attachments/${file.id}`, { method: "DELETE" });
      setRemoving(false);
      toast.success(t("views.files.deleted"));
      await onChanged();
    } catch (err) { setError((err as Error).message); }
    finally { setBusy(false); }
  }

  return <>
    <div className="flex items-center justify-end gap-1">
      {file.link_url
        ? <a className={icon} href={file.link_url} rel="noopener noreferrer" target="_blank" aria-label={t("views.files.openNamed", { name })} title={t("views.files.openLink")}><ExternalLink aria-hidden="true" /></a>
        : <a className={icon} href={download} download aria-label={t("views.files.downloadNamed", { name })} title={t("views.files.download")}><Download aria-hidden="true" /></a>}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" disabled={busy} aria-label={t("views.files.actionsFor", { name })} />}><MoreHorizontal aria-hidden="true" /></DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-56">
          {onOpenOwner && <DropdownMenuItem onClick={onOpenOwner}><FolderOpen />{t("views.files.openDetails")}</DropdownMenuItem>}
          {file.can_upload_version && <DropdownMenuItem onClick={() => input.current?.click()}><Upload />{t("views.files.newVersion")}</DropdownMenuItem>}
          <DropdownMenuItem onClick={() => void copy()}><Link2 />{t("views.files.copyLink")}</DropdownMenuItem>
          {file.can_delete && <DropdownMenuItem variant="destructive" onClick={() => { setError(""); setRemoving(true); }}><Trash2 />{t("views.files.delete")}</DropdownMenuItem>}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
    {file.can_upload_version && <input type="file" ref={input} hidden aria-label={t("views.files.newVersionFor", { name })} onChange={(event) => {
      const next = event.currentTarget.files?.[0];
      event.currentTarget.value = "";
      if (next && !busy) void upload(next);
    }} />}
    {progress && <p role="status" className="text-xs text-muted-foreground">{progress}</p>}
    {error && !removing && <p role="alert" className="text-xs text-destructive">{error}</p>}
    <ConfirmDialog open={removing} onOpenChange={setRemoving} busy={busy} error={error} title={t("views.files.deleteTitle", { name })} description={t("views.files.deleteHint")}
      confirmLabel={t("views.files.delete")} cancelLabel={t("views.files.cancel")} onConfirm={() => void remove()} />
  </>;
}
