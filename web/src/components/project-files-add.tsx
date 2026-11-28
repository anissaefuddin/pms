import { useRef, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Link2, Plus, Upload } from "lucide-react";
import { api } from "@/lib/api";
import { progressPercent, uploadWithProgress } from "@/lib/upload";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";

// "Add file" of the Files view: it always attaches to the folder or project being viewed, never to a row's item.
export function AddFile({ itemId, destination, onAdded }: { itemId: string; destination: string; onAdded: () => Promise<unknown> }) {
  const { t } = useTranslation();
  const input = useRef<HTMLInputElement>(null);
  const [transfer, setTransfer] = useState<{ file: File; percent?: number; error?: string }>();
  const [linking, setLinking] = useState(false);
  const [url, setUrl] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  // A failed upload keeps the same File object, so "Try again" re-sends it (D-05).
  function upload(file: File) {
    setBusy(true);
    setTransfer({ file });
    const body = new FormData();
    body.append("file", file);
    uploadWithProgress(`/api/v1/items/${itemId}/attachments`, body, (progress) => setTransfer((current) => current && { ...current, percent: progress.computable ? progressPercent(progress.loaded, progress.total) : undefined }))
      .then(async () => { setTransfer(undefined); await onAdded(); }, (err: Error) => setTransfer({ file, error: err.message }))
      .finally(() => setBusy(false));
  }
  async function addLink(event: FormEvent) {
    event.preventDefault();
    if (busy || !url.trim()) return;
    setBusy(true); setError("");
    try {
      await api(`/api/v1/items/${itemId}/attachment-links`, { method: "POST", body: JSON.stringify({ url: url.trim() }) });
      setLinking(false); setUrl("");
      await onAdded();
    } catch (err) { setError((err as Error).message); }
    finally { setBusy(false); }
  }

  return <div className="space-y-2">
    <input type="file" ref={input} hidden aria-label={t("views.files.chooseFile")} onChange={(event) => {
      const file = event.currentTarget.files?.[0];
      event.currentTarget.value = "";
      if (file && !busy) upload(file);
    }} />
    <div className="flex justify-end">
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button disabled={busy} />}><Plus />{t("views.files.add")}</DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-72">
          <DropdownMenuItem onClick={() => input.current?.click()}><Upload /><span><span className="block">{t("views.files.upload")}</span><span className="block text-xs text-muted-foreground">{destination}</span></span></DropdownMenuItem>
          <DropdownMenuItem onClick={() => { setError(""); setLinking(true); }}><Link2 /><span><span className="block">{t("views.files.addLinkTitle")}</span><span className="block text-xs text-muted-foreground">{t("views.files.addLinkHint")}</span></span></DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
    {transfer && <div className="space-y-1 text-sm" role="status">
      {transfer.error ? <p role="alert" className="text-destructive">{t("items.upload.failed", { message: transfer.error })}
        <Button variant="link" className="px-1" onClick={() => upload(transfer.file)}>{t("items.upload.retry")}</Button></p>
        : <><p>{t("items.upload.uploading", { name: transfer.file.name })}</p>
          <progress className="h-2 w-full" aria-label={t("items.upload.uploading", { name: transfer.file.name })}
            {...(transfer.percent !== undefined ? { value: transfer.percent, max: 100 } : {})} /></>}
    </div>}
    <Dialog open={linking} onOpenChange={setLinking}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("views.files.addLinkTitle")}</DialogTitle>
          <DialogDescription>{destination}</DialogDescription>
        </DialogHeader>
        <form className="space-y-3" onSubmit={(event) => void addLink(event)}>
          <label className="block text-sm">{t("items.linkUrl")}<Input type="url" required maxLength={2048} value={url} onChange={(event) => setUrl(event.target.value)} className="mt-1" /></label>
          <FormError message={error} />
          <Button type="submit" disabled={busy}>{t("items.addLink")}</Button>
        </form>
      </DialogContent>
    </Dialog>
  </div>;
}
