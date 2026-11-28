import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api, ApiError } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";

type Place = components["schemas"]["PydioFolder"];

// Where new files of a project are stored in Pydio, for administrators. Nothing is shown while storage was never configured.
export function PydioProjectFolder({ itemId, version }: { itemId: string; version: number }) {
  const { t } = useTranslation();
  const admin = useProfile()?.user_type === "admin";
  const cache = useQueryClient();
  // The item version is part of the key: a new project code changes the automatic folder.
  const key = ["pydio-folder", itemId, version];
  const path = `/api/v1/items/${itemId}/pydio-folder`;
  const place = useQuery({ queryKey: key, queryFn: () => api<Place>(path), enabled: admin, retry: false });
  const [open, setOpen] = useState(false);
  const [folder, setFolder] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  if (!admin || !place.data) return place.isError && !(place.error instanceof ApiError && place.error.status === 404)
    ? <p role="alert" className="text-sm text-destructive">{t("apps.pydio.folder.loadError")}</p> : null;
  const data = place.data;

  async function change(request: () => Promise<unknown>) {
    setBusy(true); setError("");
    try {
      await request();
      await cache.invalidateQueries({ queryKey: key });
      setOpen(false);
    } catch (err) {
      setError(!(err instanceof ApiError) ? t("apps.pydio.saveFailed")
        : err.code === "pydio_storage_check_failed" ? t("apps.pydio.storage.checkFailed") : err.message);
    } finally { setBusy(false); }
  }
  function save(event: FormEvent) {
    event.preventDefault();
    void change(() => api(path, { method: "PUT", body: JSON.stringify({ folder: folder.trim() }) }));
  }

  return <div className="flex flex-wrap items-center gap-x-3 gap-y-2 border-t pt-3 text-sm">
    <span className="font-medium">{t("apps.pydio.folder.title")}</span>
    <code className="min-w-0 break-all rounded bg-muted px-1.5 py-0.5 text-xs">{data.folder}</code>
    <span className="text-muted-foreground">{t(`apps.pydio.folder.source.${data.source}`)}{!data.enabled && ` · ${t("apps.pydio.folder.off")}`}</span>
    <Button variant="outline" size="sm" onClick={() => { setFolder(data.source === "project" ? data.folder : ""); setError(""); setOpen(true); }}>{t("apps.pydio.folder.change")}</Button>
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("apps.pydio.folder.title")}</DialogTitle>
          <DialogDescription>{t("apps.pydio.folder.hint")}</DialogDescription>
        </DialogHeader>
        <form onSubmit={save} className="space-y-4">
          <label className="block text-sm">{t("apps.pydio.folder.label")}
            <Input required maxLength={400} spellCheck={false} value={folder} onChange={(e) => setFolder(e.target.value)} placeholder={data.folder} className="mt-1 font-mono" /></label>
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
          <DialogFooter>
            {data.source === "project" && <Button type="button" variant="outline" disabled={busy} onClick={() => void change(() => api(path, { method: "DELETE" }))}>{t("apps.pydio.folder.automatic")}</Button>}
            <Button type="submit" disabled={busy}>{t("apps.pydio.folder.save")}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  </div>;
}
