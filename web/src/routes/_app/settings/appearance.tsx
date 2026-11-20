import { useState, type ReactNode } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { GripVertical, Plus, X } from "lucide-react";
import { api, type CurrentUser } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { FormError } from "@/components/auth/form-error";
import { ChoiceCards, ProfileSettingsForm, SettingsSection, type ChoiceOption } from "@/components/settings/settings-ui";
import { Button } from "@/components/ui/button";
import { PINNABLE_IDS, SHORTCUT_LABEL_KEY, pinPatch, sanitizePinOrder, togglePin, type ShortcutId } from "@/lib/sidebar-prefs";
import { SIDEBAR_THEMES, setSidebarTheme, useSidebarTheme } from "@/lib/sidebar-theme";

export const Route = createFileRoute("/_app/settings/appearance")({ component: AppearanceRoute });

// Workspace appearance: theme, accent and density are batch-saved; sidebar theme and sidebar entries apply at once.
function AppearanceRoute() {
  const profile = useProfile();
  if (!profile) return null;
  return <>
    <ProfileSettingsForm profile={profile} key={profile.id}>
      <AppearanceSection profile={profile} />
    </ProfileSettingsForm>
    <SidebarThemeSection />
    <SidebarSection profile={profile} />
  </>;
}

function AppearanceSection({ profile }: { profile: CurrentUser }) {
  const { t } = useTranslation();
  const themes: ChoiceOption[] = [
    { value: "light", label: t("settings.profile.options.light"), preview: <Preview kind="light" /> },
    { value: "dark", label: t("settings.profile.options.dark"), preview: <Preview kind="dark" /> },
    { value: "system", label: t("settings.profile.options.system"), preview: <Preview kind="system" /> },
  ];
  const accents: ChoiceOption[] = ["blue", "teal", "violet", "green"].map((value) => ({ value, label: t(`settings.profile.options.${value}`), preview: <span className={`block h-6 w-12 rounded-md ${ACCENT_DOT[value]}`} /> }));
  const densities: ChoiceOption[] = [
    { value: "compact", label: t("settings.profile.options.compact"), hint: t("settings.profile.density.compact"), preview: <Lines gap="gap-1" /> },
    { value: "comfortable", label: t("settings.profile.options.comfortable"), hint: t("settings.profile.density.comfortable"), preview: <Lines gap="gap-3" /> },
  ];
  return <SettingsSection title={t("settings.sections.theme")} description={t("settings.pages.appearance")}>
    <ChoiceCards name="theme" label={t("settings.profile.fields.theme")} options={themes} defaultValue={profile.theme ?? "system"} />
    <ChoiceCards name="accent" label={t("settings.profile.fields.accent")} options={accents} defaultValue={profile.accent ?? "blue"} columns="sm:grid-cols-4" />
    <ChoiceCards name="density" label={t("settings.profile.fields.density")} options={densities} defaultValue={profile.density ?? "compact"} columns="sm:grid-cols-2" />
  </SettingsSection>;
}

const ACCENT_DOT: Record<string, string> = { blue: "bg-blue-600", teal: "bg-teal-600", violet: "bg-violet-600", green: "bg-green-600" };

function Preview({ kind }: { kind: "light" | "dark" | "system" }) {
  // `light` picks the palette of one half of the preview; the system card shows one of each.
  const panel = (light: boolean) => <span className={`flex h-16 flex-1 gap-1.5 p-1.5 ${light ? "bg-slate-100" : "bg-slate-800"}`}>
    <span className={`w-1/4 rounded ${light ? "bg-slate-300" : "bg-slate-600"}`} />
    <span className={`flex-1 rounded ${light ? "bg-white" : "bg-slate-700"}`} />
  </span>;
  return <span className="flex overflow-hidden rounded-md">
    {kind === "system" ? <>{panel(true)}{panel(false)}</> : panel(kind === "light")}
  </span>;
}

function Lines({ gap }: { gap: string }) {
  return <span className={`flex flex-col p-2 ${gap}`}>{[0, 1, 2].map((row) => <span key={row} className="h-2 rounded bg-muted-foreground/40" />)}</span>;
}

/** Sidebar colour themes. Kept in this browser and applied at once. */
function SidebarThemeSection() {
  const { t } = useTranslation();
  const current = useSidebarTheme();
  return <SettingsSection title={t("settings.profile.sidebarTheme.title")} description={t("settings.profile.sidebarTheme.help")}>
    <div role="group" aria-label={t("settings.profile.sidebarTheme.title")} className="grid grid-cols-2 gap-3 sm:grid-cols-3">
      {SIDEBAR_THEMES.map((theme) => <button key={theme.id} type="button" aria-pressed={theme.id === current.id} onClick={() => setSidebarTheme(theme.id)}
        className={`overflow-hidden rounded-lg border text-start ${theme.id === current.id ? "ring-2 ring-ring" : "hover:bg-accent"}`}>
        <span className="flex h-16" aria-hidden="true">
          <span className="w-1/3" style={{ background: theme.nav }} />
          <span className="flex-1 bg-card" />
        </span>
        <span className="block px-2 py-1.5 text-xs font-medium">{theme.name}</span>
      </button>)}
    </div>
  </SettingsSection>;
}

/** Pinned and available sidebar entries, side by side. Pinned entries can be reordered by dragging or Alt+Arrow. Changes apply at once. */
function SidebarSection({ profile }: { profile: CurrentUser }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [error, setError] = useState("");
  const [dragging, setDragging] = useState<ShortcutId>();
  const pins = sanitizePinOrder(profile.sidebar_pinned);
  const available = PINNABLE_IDS.filter((id) => !pins.includes(id));
  async function save(order: ShortcutId[]) {
    setError("");
    const body = pinPatch(order, profile.sidebar_hidden ?? []);
    client.setQueryData(["me"], { ...profile, ...body });
    try {
      client.setQueryData(["me"], await api<CurrentUser>("/api/v1/me", { method: "PATCH", body: JSON.stringify(body) }));
    } catch (err) {
      client.setQueryData(["me"], profile);
      setError((err as Error).message);
    }
  }
  const toggle = (id: ShortcutId) => void save(togglePin(pins, id));
  // Moves `id` so it lands at `index` in the pinned order.
  const moveTo = (id: ShortcutId, index: number) => {
    const from = pins.indexOf(id);
    if (from < 0 || index < 0 || index >= pins.length || from === index) return;
    const order = pins.filter((entry) => entry !== id);
    order.splice(index, 0, id);
    void save(order);
  };
  const name = (id: ShortcutId) => t(SHORTCUT_LABEL_KEY[id]);
  return <SettingsSection title={t("settings.profile.sidebar.title")} description={t("settings.profile.sidebar.hint")}>
    <div className="grid gap-4 sm:grid-cols-2">
      <EntryList title={t("settings.sidebar.pinned")} hint={t("settings.sidebar.pinnedHint")} empty={t("settings.sidebar.emptyPinned")}>
        {pins.map((id, index) => <EntryRow key={id} label={name(id)} action="remove" onClick={() => toggle(id)}
          actionLabel={t("settings.sidebar.remove", { name: name(id) })} reorder={{
            hint: t("settings.sidebar.reorderHint"),
            dragging: dragging === id,
            onDragStart: () => setDragging(id),
            onDragEnd: () => setDragging(undefined),
            onDrop: () => { if (dragging) moveTo(dragging, index); setDragging(undefined); },
            onKey: (key) => {
              if (key === ARROW_UP) moveTo(id, index - 1);
              if (key === ARROW_DOWN) moveTo(id, index + 1);
            },
            position: t("settings.sidebar.position", { index: index + 1, count: pins.length }),
          }} />)}
      </EntryList>
      <EntryList title={t("settings.sidebar.available")} hint={t("settings.sidebar.availableHint")} empty={t("settings.sidebar.emptyAvailable")}>
        {available.map((id) => <EntryRow key={id} label={name(id)} action="add" onClick={() => toggle(id)} actionLabel={t("settings.sidebar.add", { name: name(id) })} />)}
      </EntryList>
    </div>
    <FormError message={error} />
  </SettingsSection>;
}

const ARROW_UP = "ArrowUp" as const;
const ARROW_DOWN = "ArrowDown" as const;

function EntryList({ title, hint, empty, children }: { title: string; hint: string; empty: string; children: ReactNode }) {
  const items = Array.isArray(children) ? children : [children];
  return <div className="rounded-lg border p-3">
    <p className="text-sm font-semibold">{title}</p>
    <p className="mb-2 text-xs text-muted-foreground">{hint}</p>
    {items.length ? <ul className="space-y-1">{children}</ul> : <p className="py-2 text-xs text-muted-foreground">{empty}</p>}
  </div>;
}

type Reorder = { hint: string; dragging: boolean; position: string; onDragStart: () => void; onDragEnd: () => void; onDrop: () => void; onKey: (key: string) => void };

function EntryRow({ label, action, actionLabel, onClick, reorder }: { label: string; action: "add" | "remove"; actionLabel: string; onClick: () => void; reorder?: Reorder }) {
  return <li draggable={!!reorder} aria-label={reorder ? `${label}, ${reorder.position}. ${reorder.hint}` : undefined}
    tabIndex={reorder ? 0 : undefined}
    onDragStart={reorder?.onDragStart} onDragEnd={reorder?.onDragEnd}
    onDragOver={reorder ? (event) => event.preventDefault() : undefined} onDrop={reorder ? (event) => { event.preventDefault(); reorder.onDrop(); } : undefined}
    onKeyDown={reorder ? (event) => { if (event.altKey) { event.preventDefault(); reorder.onKey(event.key); } } : undefined}
    className={`flex items-center justify-between gap-2 rounded-md px-2 py-1 text-sm hover:bg-accent ${reorder?.dragging ? "opacity-50" : ""} ${reorder ? "cursor-grab" : ""}`}>
    {reorder && <GripVertical aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />}
    <span className="min-w-0 flex-1 truncate">{label}</span>
    <Button type="button" variant="ghost" size="icon-sm" aria-label={actionLabel} onClick={onClick}>{action === "add" ? <Plus /> : <X />}</Button>
  </li>;
}
