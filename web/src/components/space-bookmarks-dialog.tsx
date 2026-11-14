import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

type Bookmark = components["schemas"]["SpaceBookmark"];

/** External URL bookmarks of a Space (NAV-08). Anyone who can view lists them; Edit adds, changes and removes. */
export function SpaceBookmarksDialog({ spaceId, canEdit, open, onOpenChange }: { spaceId: string; canEdit: boolean; open: boolean; onOpenChange: (open: boolean) => void }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const key = ["space-bookmarks", spaceId];
  const base = `/api/v1/spaces/${spaceId}/bookmarks`;
  const [editing, setEditing] = useState<Bookmark | null>(null);
  const list = useQuery({ queryKey: key, enabled: open, queryFn: () => api<{ items: Bookmark[] }>(base) });
  const save = useMutation({
    mutationFn: (body: object) => api(editing ? `${base}/${editing.id}` : base, { method: editing ? "PATCH" : "POST", body: JSON.stringify(body) }),
    onSuccess: async () => { setEditing(null); await qc.invalidateQueries({ queryKey: key }); },
  });
  const remove = useMutation({
    mutationFn: (id: string) => api(`${base}/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
  });
  const error = save.error ?? remove.error ?? list.error;
  return <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{t("spaces.bookmarks.title")}</DialogTitle>
        <DialogDescription>{t("spaces.bookmarks.hint")}</DialogDescription>
      </DialogHeader>
      {error && <p role="alert">{error.message}</p>}
      {list.data && !list.data.items.length && <p className="text-sm text-muted-foreground">{t("spaces.bookmarks.empty")}</p>}
      <ul className="space-y-2">
        {list.data?.items.map((b) => <li key={b.id} className="flex flex-wrap items-center gap-2 text-sm">
          <a href={b.url} target="_blank" rel="noopener noreferrer" className="font-medium underline">{b.title}<span className="sr-only"> {t("spaces.bookmarks.newTab")}</span></a>
          {b.description && <span className="text-muted-foreground">{b.description}</span>}
          {canEdit && <>
            <Button size="sm" variant="outline" aria-label={t("spaces.bookmarks.editNamed", { name: b.title })} onClick={() => { save.reset(); setEditing(b); }}>{t("spaces.bookmarks.edit")}</Button>
            <Button size="sm" variant="outline" disabled={remove.isPending} aria-label={t("spaces.bookmarks.deleteNamed", { name: b.title })} onClick={() => remove.mutate(b.id)}>{t("spaces.bookmarks.delete")}</Button>
          </>}
        </li>)}
      </ul>
      {canEdit && <form key={editing?.id ?? "new"} className="space-y-3" onSubmit={(event) => {
        event.preventDefault();
        const form = event.currentTarget;
        const data = new FormData(form);
        save.mutate({ url: String(data.get("url")), title: String(data.get("title")), description: String(data.get("description")) }, { onSuccess: () => form.reset() });
      }}>
        <h3 className="font-medium">{editing ? t("spaces.bookmarks.editTitle") : t("spaces.bookmarks.add")}</h3>
        <label className="block text-sm">{t("spaces.bookmarks.url")}<Input name="url" type="url" required maxLength={2000} defaultValue={editing?.url} /></label>
        <label className="block text-sm">{t("spaces.bookmarks.name")}<Input name="title" required maxLength={200} defaultValue={editing?.title} /></label>
        <label className="block text-sm">{t("spaces.bookmarks.description")}<Input name="description" maxLength={1000} defaultValue={editing?.description} /></label>
        <div className="flex gap-2">
          <Button type="submit" disabled={save.isPending}>{editing ? t("spaces.bookmarks.save") : t("spaces.bookmarks.addButton")}</Button>
          {editing && <Button type="button" variant="outline" onClick={() => setEditing(null)}>{t("spaces.bookmarks.cancel")}</Button>}
        </div>
      </form>}
    </DialogContent>
  </Dialog>;
}
