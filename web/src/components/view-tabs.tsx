import { useState, type FormEvent } from "react";
import { CalendarDays, ChartColumn, ChartGantt, Clock, Folder, Kanban, MoreHorizontal, Plus, Table2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { SavedView } from "@/lib/items";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { FormError } from "@/components/auth/form-error";

const viewIcons = { table: Table2, board: Kanban, gantt: ChartGantt, calendar: CalendarDays, timelog: Clock, files: Folder, chart: ChartColumn } as const;
function ViewIcon({ type }: { type: SavedView["type"] }) {
  const Icon = viewIcons[type];
  return <Icon aria-hidden="true" />;
}

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
  const selectedIndex = views.findIndex((view) => view.id === selected.id);
  const selectedSiblings = views.filter((candidate) => candidate.visibility === selected.visibility);
  const i = selectedSiblings.findIndex((candidate) => candidate.id === selected.id);
  const previous = selectedSiblings[i - 1], next = selectedSiblings[i + 1];
  const editable = selected.visibility === "personal" || canEdit;
  return <div className="space-y-2">
    <nav aria-label={t("views.views")} className="flex flex-wrap items-center gap-2 border-b pb-2">
      <SegmentedControl aria-label={t("views.views")} value={selected.id} onChange={onSelect}
        items={views.map((view) => ({ value: view.id, label: view.name, icon: <ViewIcon type={view.type} /> }))} />
      {selectedIndex >= 0 && <DropdownMenu>
        <DropdownMenuTrigger render={<Button size="icon-sm" variant="ghost" aria-label={t("views.options", { name: selected.name })} />}><MoreHorizontal /></DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <DropdownMenuItem onClick={() => void copy(selected)}>{t("items.copyLink")}</DropdownMenuItem>
          {editable && <DropdownMenuItem disabled={busy} onClick={() => { setError(""); setRenaming(selected); }}>{t("views.rename")}</DropdownMenuItem>}
          {canEdit && <DropdownMenuItem disabled={busy} onClick={() => void duplicate(selected, "public")}>{t("views.duplicatePublic")}</DropdownMenuItem>}
          <DropdownMenuItem disabled={busy} onClick={() => void duplicate(selected, "personal")}>{t("views.duplicatePersonal")}</DropdownMenuItem>
          {canEdit && selected.visibility === "public" && !selected.is_default && <DropdownMenuItem disabled={busy}
            onClick={() => void update(selected.id, { is_default: true })}>{t("views.setDefault")}</DropdownMenuItem>}
          {editable && previous && <DropdownMenuItem disabled={busy} onClick={() => void update(selected.id, { position: previous.position })}>{t("views.moveLeft")}</DropdownMenuItem>}
          {editable && next && <DropdownMenuItem disabled={busy} onClick={() => void update(selected.id, { position: next.position })}>{t("views.moveRight")}</DropdownMenuItem>}
          {editable && !selected.is_default && <DropdownMenuItem variant="destructive" disabled={busy}
            onClick={() => { setError(""); setDeleting(selected); }}>{t("views.delete")}</DropdownMenuItem>}
        </DropdownMenuContent>
      </DropdownMenu>}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button size="icon-sm" variant="ghost" disabled={busy} aria-label={t("views.addView")} title={t("views.addView")} />}>
          <Plus />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          {viewTypes.filter((type) => (type !== "timelog" && type !== "files" && type !== "chart") || project).map((type) => <DropdownMenuItem key={type} onClick={() => { setError(""); setAdding(type); }}>
            <ViewIcon type={type} />{t(`views.types.${type}`)}
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
