import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowDown, ArrowUp } from "lucide-react";
import { Button } from "./ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "./ui/dialog";
import {
  DEFAULT_PINNED, FIXED_SHORTCUT, PINNABLE_IDS, SHORTCUT_LABEL_KEY, SIDEBAR_THEME, isShortcutVisible, movePin, togglePin,
  type ShortcutId,
} from "@/lib/sidebar-prefs";

/** Pemanggil lain (mis. tombol create global di Lane B) cukup memanggil fungsi ini untuk membuka modal. */
export function openCustomizeSidebar() {
  window.dispatchEvent(new Event("pms:open-customize-sidebar"));
}

export function CustomizeSidebarDialog({ open, onOpenChange, pinOrder, hidden, onSave }: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  pinOrder: ShortcutId[];
  hidden: string[];
  onSave: (order: ShortcutId[]) => void;
}) {
  return <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent className="sm:max-w-2xl">
      {/* Dipasang ulang setiap dibuka, jadi draf selalu mulai dari urutan yang tersimpan */}
      {open && <CustomizeForm pinOrder={pinOrder} hidden={hidden}
        onCancel={() => onOpenChange(false)}
        onSave={(order) => { onSave(order); onOpenChange(false); }} />}
    </DialogContent>
  </Dialog>;
}

function CustomizeForm({ pinOrder, hidden, onCancel, onSave }: {
  pinOrder: ShortcutId[];
  hidden: string[];
  onCancel: () => void;
  onSave: (order: ShortcutId[]) => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState<ShortcutId[]>(pinOrder);
  const name = (id: ShortcutId) => t(SHORTCUT_LABEL_KEY[id]);
  const visibleDraft = draft.filter((id) => isShortcutVisible(id, hidden));

  // Baris tersemat punya tombol naik/turun; yang belum tersemat hanya checkbox.
  function renderRow(id: ShortcutId, index: number | null) {
    const pinned = index !== null;
    const label = name(id);
    const blocked = !isShortcutVisible(id, hidden);
    return <li key={id} className="flex items-center gap-2 rounded-md px-2 py-1 hover:bg-muted">
      <input type="checkbox" className="size-4 shrink-0" checked={pinned}
        aria-label={t("nav.customize.pin", { name: label })}
        onChange={() => setDraft(togglePin(draft, id))} />
      <span className="min-w-0 flex-1 truncate text-sm">{label}</span>
      {blocked && <span className="text-xs text-muted-foreground">{t("nav.customize.hidden")}</span>}
      {pinned && <>
        <Button variant="ghost" size="icon-sm" disabled={index === 0} aria-label={t("nav.customize.up", { name: label })}
          onClick={() => setDraft(movePin(draft, id, -1))}><ArrowUp /></Button>
        <Button variant="ghost" size="icon-sm" disabled={index === draft.length - 1} aria-label={t("nav.customize.down", { name: label })}
          onClick={() => setDraft(movePin(draft, id, 1))}><ArrowDown /></Button>
      </>}
    </li>;
  }

  return <>
    <DialogHeader>
      <DialogTitle>{t("nav.customize.title")}</DialogTitle>
      <DialogDescription>{t("nav.customize.description")}</DialogDescription>
    </DialogHeader>
    <div className="grid gap-4 md:grid-cols-2">
      <ul className="space-y-1" aria-label={t("nav.customize.list")}>
        {draft.map((id, index) => renderRow(id, index))}
        {PINNABLE_IDS.filter((id) => !draft.includes(id)).map((id) => renderRow(id, null))}
      </ul>
      <div aria-hidden="true" className={`${SIDEBAR_THEME} rounded-lg border border-border bg-nav p-3 text-nav-text`}>
        <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-nav-muted">{t("nav.customize.preview")}</p>
        <ul className="space-y-1 text-sm">
          <li className="px-2 py-1">{name(FIXED_SHORTCUT)}</li>
          <li className="px-2 py-1">{t("nav.inbox.label")}</li>
          <li className="px-2 py-1">{t("nav.search.label")}</li>
          {visibleDraft.map((id) => <li key={id} className="rounded-md px-2 py-1">{name(id)}</li>)}
          <li className="px-2 py-1 text-nav-muted">{t("nav.more.label")}</li>
        </ul>
      </div>
    </div>
    <p className="text-xs text-muted-foreground">{t("nav.customize.fixed")}</p>
    <DialogFooter className="sm:justify-between">
      <Button variant="ghost" onClick={() => setDraft([...DEFAULT_PINNED])}>{t("nav.customize.reset")}</Button>
      <div className="flex gap-2">
        <Button variant="outline" onClick={onCancel}>{t("nav.customize.cancel")}</Button>
        <Button onClick={() => onSave(draft)}>{t("nav.customize.done")}</Button>
      </div>
    </DialogFooter>
  </>;
}
