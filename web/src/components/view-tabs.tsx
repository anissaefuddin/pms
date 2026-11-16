import { useState, type FormEvent } from "react";
import { Check, CalendarDays, ChartColumn, ChartGantt, Clock, Folder, Kanban, LayoutList, MoreHorizontal, Plus, Table2 } from "lucide-react";
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
import { useViewPreferences } from "@/lib/use-view-preferences";
import { arrangeViews, hideView, moveView, moveViewBefore, restoreView } from "@/lib/view-preferences";

const viewIcons = { table: Table2, board: Kanban, gantt: ChartGantt, calendar: CalendarDays, timelog: Clock, files: Folder, chart: ChartColumn } as const;
function ViewIcon({ type }: { type: SavedView["type"] }) {
  const Icon = viewIcons[type];
  return <Icon aria-hidden="true" />;
}

const viewTypes = ["table", "board", "gantt", "calendar", "timelog", "files", "chart"] as const satisfies readonly SavedView["type"][];
const projectOnlyTypes: readonly SavedView["type"][] = ["timelog", "files", "chart"];
/** Tab terlihat sebelum sisanya masuk menu "All views". Tetap sederhana: pengukuran lebar layar belum dipakai. shortcut: tetap 6 tab, naik ke pengukuran lebar bila perlu. */
const VISIBLE_TABS = 6;

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
  const [choosing, setChoosing] = useState(false);
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
  // WA-D07: order and hidden views are personal, saved per workspace; the shared view configuration is not changed.
  const scopeId = "scope_space_id" in scope ? scope.scope_space_id : scope.scope_item_id;
  const { prefs, save: savePrefs } = useViewPreferences(scopeId);
  const arranged = arrangeViews(views, prefs, selected.id);
  // A hidden active view is still shown (marked hidden) so the user can see where they are; the saved preference is not changed.
  const visible = arranged.hidden.some((view) => view.id === selected.id)
    ? [...arranged.visible.slice(0, VISIBLE_TABS - 1), selected] : arranged.visible;
  const selectedHidden = arranged.hidden.some((view) => view.id === selected.id);
  // A-02: drag a visible tab onto another to place it before that tab (personal order only).
  const [dragging, setDragging] = useState<string>();
  const editable = selected.visibility === "personal" || canEdit;
  return <div className="space-y-2">
    <nav aria-label={t("views.views")} className="flex flex-wrap items-center gap-2 border-b pb-2">
      <SegmentedControl aria-label={t("views.views")} value={selected.id} onChange={onSelect}
        itemDrag={(id) => ({
          draggable: true,
          onDragStart: (event) => { event.dataTransfer.setData("text/plain", id); setDragging(id); },
          onDragOver: (event) => { if (dragging && dragging !== id) event.preventDefault(); },
          onDrop: (event) => { event.preventDefault(); if (dragging) savePrefs(moveViewBefore(prefs, views, dragging, id)); setDragging(undefined); },
          onDragEnd: () => setDragging(undefined),
        })}
        items={visible.map((view) => ({ value: view.id, label: view.name, icon: <ViewIcon type={view.type} /> }))} />
      {selectedHidden && <span className="rounded bg-muted px-2 py-0.5 text-xs text-muted-foreground">{t("views.hiddenNow")}</span>}
      {views.length > VISIBLE_TABS && <DropdownMenu>
        <DropdownMenuTrigger render={<Button size="sm" variant="ghost" aria-label={t("views.allViews")} />}><LayoutList />{t("views.allViews")}</DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="max-h-80 w-64 overflow-y-auto">
          <DropdownMenuGroup>
            <DropdownMenuLabel>{t("views.allViews")}</DropdownMenuLabel>
            {arranged.visible.map((view) => <DropdownMenuItem key={view.id} aria-current={view.id === selected.id ? "true" : undefined} onClick={() => onSelect(view.id)}>
              <ViewIcon type={view.type} />
              <span className="min-w-0 flex-1 truncate">{view.name}</span>
              <span className="text-xs text-muted-foreground">{[view.is_default && t("views.defaultView"), t(view.visibility === "personal" ? "views.personal" : "views.public")].filter(Boolean).join(" · ")}</span>
              {view.id === selected.id && <Check aria-label={t("views.currentView")} />}
            </DropdownMenuItem>)}
          </DropdownMenuGroup>
          {arranged.overflow.length > 0 && <DropdownMenuGroup>
            <DropdownMenuLabel>{t("views.overflowViews")}</DropdownMenuLabel>
            {arranged.overflow.map((view) => <DropdownMenuItem key={view.id} aria-current={view.id === selected.id ? "true" : undefined} onClick={() => onSelect(view.id)}>
              <ViewIcon type={view.type} /><span className="min-w-0 flex-1 truncate">{view.name}</span>
            </DropdownMenuItem>)}
          </DropdownMenuGroup>}
          {arranged.hidden.length > 0 && <DropdownMenuGroup>
            <DropdownMenuLabel>{t("views.hiddenViews")}</DropdownMenuLabel>
            {arranged.hidden.map((view) => <DropdownMenuItem key={view.id} onClick={() => onSelect(view.id)}>
              <ViewIcon type={view.type} /><span className="min-w-0 flex-1 truncate">{view.name}</span>
              <span role="button" tabIndex={0} className="text-xs underline" onClick={(event) => { event.stopPropagation(); savePrefs(restoreView(prefs, view.id)); }}
                onKeyDown={(event) => { if (event.key === "Enter") { event.stopPropagation(); savePrefs(restoreView(prefs, view.id)); } }}>{t("views.restoreView")}</span>
            </DropdownMenuItem>)}
          </DropdownMenuGroup>}
        </DropdownMenuContent>
      </DropdownMenu>}
      {selectedIndex >= 0 && <DropdownMenu>
        <DropdownMenuTrigger render={<Button size="icon-sm" variant="ghost" aria-label={t("views.options", { name: selected.name })} />}><MoreHorizontal /></DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <DropdownMenuItem onClick={() => void copy(selected)}>{t("items.copyLink")}</DropdownMenuItem>
          {editable && <DropdownMenuItem disabled={busy} onClick={() => { setError(""); setRenaming(selected); }}>{t("views.rename")}</DropdownMenuItem>}
          {canEdit && <DropdownMenuItem disabled={busy} onClick={() => void duplicate(selected, "public")}>{t("views.duplicatePublic")}</DropdownMenuItem>}
          <DropdownMenuItem disabled={busy} onClick={() => void duplicate(selected, "personal")}>{t("views.duplicatePersonal")}</DropdownMenuItem>
          {canEdit && selected.visibility === "public" && !selected.is_default && <DropdownMenuItem disabled={busy}
            onClick={() => void update(selected.id, { is_default: true })}>{t("views.setDefault")}</DropdownMenuItem>}
          <DropdownMenuItem disabled={busy || selected.is_default || !(arranged.visible.some((view) => view.id === selected.id) || arranged.overflow.some((view) => view.id === selected.id))}
            onClick={() => savePrefs(hideView(prefs, selected.id, views))}>{selected.is_default ? t("views.cannotHideDefault") : t("views.hideView")}</DropdownMenuItem>
          <DropdownMenuItem disabled={busy || arranged.order.indexOf(selected.id) <= 0} onClick={() => savePrefs(moveView(prefs, views, selected.id, -1))}>{t("views.moveLeft")}</DropdownMenuItem>
          <DropdownMenuItem disabled={busy || arranged.order.indexOf(selected.id) < 0 || arranged.order.indexOf(selected.id) >= arranged.order.length - 1} onClick={() => savePrefs(moveView(prefs, views, selected.id, 1))}>{t("views.moveRight")}</DropdownMenuItem>
          {editable && !selected.is_default && <DropdownMenuItem variant="destructive" disabled={busy}
            onClick={() => { setError(""); setDeleting(selected); }}>{t("views.delete")}</DropdownMenuItem>}
        </DropdownMenuContent>
      </DropdownMenu>}
      <Button size="icon-sm" variant="ghost" disabled={busy} aria-label={t("views.addView")} title={t("views.addView")} onClick={() => { setError(""); setChoosing(true); }}>
        <Plus />
      </Button>
    </nav>
    {!adding && !renaming && !deleting && <FormError message={error} />}
    <Dialog open={choosing} onOpenChange={(open) => { if (!open) setChoosing(false); }}>
      <DialogContent>
        <DialogTitle>{t("views.chooseType")}</DialogTitle>
        <div className="grid gap-2 sm:grid-cols-2" role="group" aria-label={t("views.chooseType")}>
          {viewTypes.map((type) => {
            const available = !projectOnlyTypes.includes(type) || project;
            return <button key={type} type="button" disabled={!available} aria-disabled={!available || undefined}
              onClick={() => { setChoosing(false); setAdding(type); }}
              className="flex items-start gap-3 rounded-lg border bg-card p-3 text-left hover:bg-accent disabled:cursor-not-allowed disabled:opacity-60">
              <span className="mt-0.5"><ViewIcon type={type} /></span>
              <span className="min-w-0 space-y-0.5">
                <span className="block font-medium">{t(`views.types.${type}`)}</span>
                <span className="block text-xs text-muted-foreground">{t(`views.typeDescriptions.${type}`)}</span>
                <span className="block text-xs text-muted-foreground">{available ? t("views.availableHere") : t("views.projectOnly")}</span>
              </span>
            </button>;
          })}
        </div>
      </DialogContent>
    </Dialog>
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
