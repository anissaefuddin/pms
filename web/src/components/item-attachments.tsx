import { useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import type { Item } from "@/lib/items";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";

type Attachment = components["schemas"]["Attachment"];
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

  function upload(files: FileList | null) {
    if (!files?.length || working.current) return;
    if (files.length !== 1) {
      setError(t("items.oneAttachment"));
      return;
    }
    const body = new FormData();
    body.append("file", files[0]);
    void change(() => api(path, { method: "POST", body }));
  }

  return <section className="space-y-2 border-t pt-4">
    <h3 className="font-medium">{t("items.attachments")}</h3>
    {isPending && <p role="status">{t("items.loading")}</p>}
    {isError && <FormError message={t("items.loadError")} />}
    <ul className="divide-y">
      {data?.items.map((file) => {
        const canRemove = item.my_capabilities.includes("delete") || (canAttach && file.uploader.id === userId);
        return <li key={file.id} className="flex items-center justify-between gap-2 py-1">
          <a className="min-w-0 break-all underline" href={attachmentPath(file.id) + "/download"} download>{file.filename}</a>
          {canRemove && <Button variant="ghost" size="sm" disabled={busy} onClick={() => {
            setError("");
            setRemoving(file);
          }}>{t("items.removeAttachment")}</Button>}
        </li>;
      })}
    </ul>
    {canAttach && <div className="rounded border border-dashed p-3" onDragOver={(event) => event.preventDefault()}
      onDrop={(event) => { event.preventDefault(); upload(event.dataTransfer.files); }}>
      <input type="file" ref={input} hidden onChange={(event) => {
        upload(event.currentTarget.files);
        event.currentTarget.value = "";
      }} />
      <Button variant="outline" disabled={busy} onClick={() => input.current?.click()}>{t("items.chooseAttachment")}</Button>
      <p className="mt-1 text-sm text-muted-foreground">{t("items.dropAttachment")}</p>
      {!!data?.max_upload_bytes && <p className="text-sm text-muted-foreground">
        {t("items.attachmentLimit", { limit: data.max_upload_bytes / 2 ** 20 })}
      </p>}
    </div>}
    {!removing && <FormError message={error} />}
    <ConfirmDialog open={!!removing} onOpenChange={(open) => { if (!open) setRemoving(undefined); }}
      title={t("items.removeAttachment")} description={removing?.filename ?? ""}
      confirmLabel={t("items.removeAttachment")} cancelLabel={t("items.cancel")} busy={busy} error={error}
      onConfirm={() => removing ? change(() => api(attachmentPath(removing.id), { method: "DELETE" })) : undefined} />
  </section>;
}
