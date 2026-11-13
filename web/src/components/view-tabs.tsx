import { useState, type FormEvent } from "react";
import { MoreHorizontal, Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { SavedView } from "@/lib/items";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { FormError } from "@/components/auth/form-error";

const viewTypes = ["table", "board", "gantt", "calendar", "timelog", "files", "chart"] as const satisfies readonly SavedView["type"][];

export function ViewTabs({ views, selected, scope, canEdit, project = false, onSelect, onRefresh }: {
  views: SavedView[];
  selected: SavedView;
  scope: { scope_space_id: string } | { scope_item_id: string };
  canEdit: boolean;
  project?: boolean;
  onSelect: (id: string) => void;
  onRefresh: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const [adding, setAdding] = useState<SavedView["type"]>();
  const [renaming, setRenaming] = useState<SavedView>();
  const [deleting, setDeleting] = useState<SavedView>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function run<T>(work: () => Promise<T>, after?: (result: T) => void) {
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      const value = await work();
      await onRefresh();
      after?.(value);
    } catch (err) { setError((err as Error).message); }
    finally { setBusy(false); }
  }
  function update(id: string, body: object, after?: () => void) {
    return run(() => api<SavedView>(`/api/v1/views/${id}`, { method: "PATCH", body: JSON.stringify(body) }), after);
  }
  function duplicate(view: SavedView, visibility: "personal" | "public") {
    const name = Array.from(t("views.duplicateName", { name: view.name })).slice(0, 80).join("");
    return run(() => api<SavedView>(`/api/v1/views/${view.id}/duplicate`, { method: "POST", body: JSON.stringify({ name, visibility }) }),
      (created) => onSelect(created.id));
  }
  async function copy(view: SavedView) {
    const url = new URL(window.location.href);
    url.searchParams.set("view", view.id);
    try {
      await navigator.clipboard.writeText(url.href);
      toast.success(t("items.copied"));
    } catch (err) { setError((err as Error).message); }
  }
  function rename(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!renaming) return;
    const name = String(new FormData(event.currentTarget).get("name") ?? "").trim();
    if (name) void update(renaming.id, { name }, () => setRenaming(undefined));
  }
  function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!adding) return;
    const form = new FormData(event.currentTarget);
    const name = String(form.get("name") ?? "").trim();
    if (!name) return;
    const visibility = canEdit && form.get("visibility") === "public" ? "public" : "personal";
    void run(() => api<SavedView>("/api/v1/views", {
      method: "POST", body: JSON.stringify({ ...scope, type: adding, name, visibility }),
    }), (created) => {
      setAdding(undefined);
      onSelect(created.id);
    });
  }
  return <div className="space-y-2">
    <nav aria-label={t("views.views")} className="flex flex-wrap gap-2 border-b pb-2">
      {views.map((view) => {
        const editable = view.visibility === "personal" || canEdit;
        const siblings = views.filter((candidate) => candidate.visibility === view.visibility);
        const i = siblings.findIndex((candidate) => candidate.id === view.id);
        const previous = siblings[i - 1], next = siblings[i + 1];
        return <div key={view.id} className="flex items-center gap-0.5">
          <Button size="sm" variant={view.id === selected.id ? "secondary" : "ghost"} aria-pressed={view.id === selected.id}
            onClick={() => onSelect(view.id)}>{view.name}</Button>
          <DropdownMenu>
            <DropdownMenuTrigger render={<Button size="icon-sm" variant="ghost" aria-label={t("views.options", { name: view.name })} />}><MoreHorizontal /></DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              <DropdownMenuItem onClick={() => void copy(view)}>{t("items.copyLink")}</DropdownMenuItem>
              {editable && <DropdownMenuItem disabled={busy} onClick={() => { setError(""); setRenaming(view); }}>{t("views.rename")}</DropdownMenuItem>}
              {canEdit && <DropdownMenuItem disabled={busy} onClick={() => void duplicate(view, "public")}>{t("views.duplicatePublic")}</DropdownMenuItem>}
              <DropdownMenuItem disabled={busy} onClick={() => void duplicate(view, "personal")}>{t("views.duplicatePersonal")}</DropdownMenuItem>
              {canEdit && view.visibility === "public" && !view.is_default && <DropdownMenuItem disabled={busy}
                onClick={() => void update(view.id, { is_default: true })}>{t("views.setDefault")}</DropdownMenuItem>}
              {editable && previous && <DropdownMenuItem disabled={busy} onClick={() => void update(view.id, { position: previous.position })}>{t("views.moveLeft")}</DropdownMenuItem>}
              {editable && next && <DropdownMenuItem disabled={busy} onClick={() => void update(view.id, { position: next.position })}>{t("views.moveRight")}</DropdownMenuItem>}
              {editable && !view.is_default && <DropdownMenuItem variant="destructive" disabled={busy}
                onClick={() => { setError(""); setDeleting(view); }}>{t("views.delete")}</DropdownMenuItem>}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>;
      })}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button size="sm" variant="ghost" disabled={busy} />}>
          <Plus /> {t("views.addView")}
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          {viewTypes.filter((type) => (type !== "timelog" && type !== "files" && type !== "chart") || project).map((type) => <DropdownMenuItem key={type} onClick={() => { setError(""); setAdding(type); }}>
            {t(`views.types.${type}`)}
          </DropdownMenuItem>)}
        </DropdownMenuContent>
      </DropdownMenu>
    </nav>
    {!adding && !renaming && !deleting && <FormError message={error} />}
    <Dialog open={!!adding} onOpenChange={(open) => { if (!open) setAdding(undefined); }}>
      <DialogContent>
        <DialogTitle>{t("views.addView")}</DialogTitle>
        {adding && <form className="space-y-3" onSubmit={create}>
          <label className="block space-y-1"><span>{t("views.name")}</span>
            <Input name="name" defaultValue={t(`views.types.${adding}`)} maxLength={80} required autoFocus />
          </label>
          {canEdit && <label className="block space-y-1"><span>{t("views.visibility")}</span>
            <select name="visibility" className="block rounded border bg-background p-2">
              <option value="public">{t("views.public")}</option>
              <option value="personal">{t("views.personal")}</option>
            </select>
          </label>}
          <FormError message={error} />
          <DialogFooter><Button type="submit" disabled={busy}>{t("items.save")}</Button></DialogFooter>
        </form>}
      </DialogContent>
    </Dialog>
    <Dialog open={!!renaming} onOpenChange={(open) => { if (!open) setRenaming(undefined); }}>
      <DialogContent>
        <DialogTitle>{t("views.rename")}</DialogTitle>
        {renaming && <form className="space-y-3" onSubmit={rename}>
          <label className="block space-y-1"><span>{t("views.name")}</span>
            <Input name="name" defaultValue={renaming.name} maxLength={80} required autoFocus />
          </label>
          <FormError message={error} />
          <DialogFooter><Button type="submit" disabled={busy}>{t("items.save")}</Button></DialogFooter>
        </form>}
      </DialogContent>
    </Dialog>
    <ConfirmDialog open={!!deleting} onOpenChange={(open) => { if (!open) setDeleting(undefined); }}
      title={t("views.delete")} description={t("views.deleteConfirm", { name: deleting?.name })}
      confirmLabel={t("views.delete")} cancelLabel={t("items.cancel")} busy={busy} error={error}
      onConfirm={async () => {
        if (!deleting) return;
        await run(() => api(`/api/v1/views/${deleting.id}`, { method: "DELETE" }), () => {
          if (deleting.id === selected.id) onSelect(views.find((view) => view.is_default)?.id ?? "");
          setDeleting(undefined);
        });
      }} />
  </div>;
}
