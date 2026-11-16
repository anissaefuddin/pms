import { useState, type FormEvent } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api, type CurrentUser } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PINNABLE_IDS, SHORTCUT_LABEL_KEY, pinPatch, sanitizePinOrder, togglePin, type ShortcutId } from "@/lib/sidebar-prefs";
import { SIDEBAR_THEMES, setSidebarTheme, useSidebarTheme } from "@/lib/sidebar-theme";

export const Route = createFileRoute("/_app/settings/profile")({ component: ProfileRoute });

const personalFields = [
  { key: "first_name", max: 80, required: true },
  { key: "last_name", max: 80, required: true },
  { key: "company", max: 120 },
  { key: "job_title", max: 80 },
  { key: "country", max: 2 },
  { key: "location", max: 120 },
  { key: "department", max: 80 },
  { key: "phone", max: 32 },
] as const;
const choices = {
  locale: ["en", "id"],
  date_format: ["dd/mm/yyyy", "mm/dd/yyyy", "yyyy-mm-dd"],
  time_format: ["12h", "24h"],
  week_start: ["monday", "sunday", "saturday"],
  density: ["compact", "comfortable"],
  theme: ["light", "dark", "system"],
  accent: ["blue", "teal", "violet", "green"],
  comment_shortcut: ["mod_enter", "enter"],
} as const;
const timezones = ["UTC", ...Intl.supportedValuesOf("timeZone")];

// Only entries that exist in this version are offered; the other keys the API accepts stay hidden (NAV-02).
const emailGroups = ["mention", "assigned", "shared", "task_changes", "report"] as const;

/** Density as two visual choices. The value is submitted with the profile form (hidden input named "density"). */
const densities = ["compact", "comfortable"] as const;
function DensityPicker({ defaultValue }: { defaultValue: string }) {
  const { t } = useTranslation();
  const [value, setValue] = useState(defaultValue);
  return <fieldset className="space-y-2 sm:col-span-2">
    <legend className="text-sm font-medium">{t("settings.profile.fields.density")}</legend>
    <input type="hidden" name="density" value={value} />
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      {densities.map((option) => <button key={option} type="button" aria-pressed={value === option} onClick={() => setValue(option)}
        className={`flex items-start gap-3 rounded-lg border p-3 text-left ${value === option ? "border-primary ring-2 ring-ring" : "hover:bg-accent"}`}>
        <span className="mt-1 flex w-10 shrink-0 flex-col gap-1" aria-hidden="true">
          {[0, 1, 2].map((row) => <span key={row} className={`h-1.5 rounded-sm bg-muted-foreground/40 ${option === "compact" ? "" : "mt-1"}`} />)}
        </span>
        <span className="min-w-0">
          <span className="block text-sm font-medium">{t(`settings.profile.options.${option}`, { defaultValue: option })}</span>
          <span className="block text-xs text-muted-foreground">{t(`settings.profile.density.${option}`)}</span>
        </span>
      </button>)}
    </div>
  </fieldset>;
}

/** Sidebar colour themes (Settings V2 prototype). The choice is kept in this browser. */
function SidebarThemePicker() {
  const { t } = useTranslation();
  const current = useSidebarTheme();
  return <section className="space-y-3 border-t pt-6" id="sidebar-theme">
    <h2 className="text-sm font-semibold">{t("settings.profile.sidebarTheme.title")}</h2>
    <p className="text-sm text-muted-foreground">{t("settings.profile.sidebarTheme.help")}</p>
    <div role="group" aria-label={t("settings.profile.sidebarTheme.title")} className="grid grid-cols-2 gap-3 sm:grid-cols-3">
      {SIDEBAR_THEMES.map((theme) => <button key={theme.id} type="button" aria-pressed={theme.id === current.id}
        onClick={() => setSidebarTheme(theme.id)}
        className={`overflow-hidden rounded-lg border text-left ${theme.id === current.id ? "ring-2 ring-ring" : ""}`}>
        <span className="flex h-16" aria-hidden="true">
          <span className="w-1/3" style={{ background: theme.nav }} />
          <span className="flex-1 bg-card" />
        </span>
        <span className="block px-2 py-1.5 text-xs font-medium">{theme.name}</span>
      </button>)}
    </div>
  </section>;
}

function ProfileRoute() {
  const profile = useProfile();
  return profile && <><Identity profile={profile} /><ProfileForm key={profile.id} profile={profile} /><SidebarThemePicker /><SidebarSettings profile={profile} /><EmailSettings profile={profile} /></>;
}

function Identity({ profile }: { profile: CurrentUser }) {
  const initials = `${profile.first_name[0] ?? ""}${profile.last_name[0] ?? ""}`.toUpperCase();
  const role = [profile.job_title, profile.company].filter(Boolean).join(" · ");
  return <div className="mb-6 flex items-center gap-4">
    <span aria-hidden="true" className="grid size-16 place-items-center rounded-xl bg-primary text-xl font-semibold text-primary-foreground">{initials}</span>
    <div><p className="text-lg font-semibold">{profile.first_name} {profile.last_name}</p>{role && <p className="text-sm text-muted-foreground">{role}</p>}</div>
  </div>;
}

function SidebarSettings({ profile }: { profile: CurrentUser }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [error, setError] = useState("");
  // Same data as the sidebar: checked = pinned, so the checklist and the sidebar cannot disagree.
  const pins = sanitizePinOrder(profile.sidebar_pinned);
  async function toggle(id: ShortcutId) {
    setError("");
    const order = togglePin(pins, id);
    const body = pinPatch(order, profile.sidebar_hidden ?? []);
    client.setQueryData(["me"], { ...profile, ...body }); // optimistic, the sidebar follows at once
    try {
      client.setQueryData(["me"], await api<CurrentUser>("/api/v1/me", { method: "PATCH", body: JSON.stringify(body) }));
    } catch (err) {
      client.setQueryData(["me"], profile);
      setError((err as Error).message);
    }
  }
  return <fieldset className="mt-6 max-w-3xl space-y-2">
    <legend className="mb-1 text-lg font-semibold">{t("settings.profile.sidebar.title")}</legend>
    <p className="text-sm text-muted-foreground">{t("settings.profile.sidebar.hint")}</p>
    {PINNABLE_IDS.map((id) => <label key={id} className="flex items-center gap-2 text-sm">
      <input type="checkbox" checked={pins.includes(id)} onChange={() => void toggle(id)} />
      {t(SHORTCUT_LABEL_KEY[id])}
    </label>)}
    <FormError message={error} />
  </fieldset>;
}

function EmailSettings({ profile }: { profile: CurrentUser }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [error, setError] = useState("");
  const [muted, setMuted] = useState(profile.email_muted ?? []);
  async function toggle(key: (typeof emailGroups)[number], on: boolean) {
    setError("");
    const previous = muted;
    const next = on ? muted.filter((entry) => entry !== key) : [...muted, key];
    setMuted(next);
    try {
      client.setQueryData(["me"], await api<CurrentUser>("/api/v1/me", { method: "PATCH", body: JSON.stringify({ email_muted: next }) }));
    } catch (err) {
      setMuted(previous);
      setError((err as Error).message);
    }
  }
  return <fieldset id="email" className="mt-6 max-w-3xl scroll-mt-4 space-y-2">
    <legend className="mb-1 text-lg font-semibold">{t("settings.profile.email.title")}</legend>
    <p className="text-sm text-muted-foreground">{t("settings.profile.email.hint")}</p>
    {emailGroups.map((key) => <label key={key} className="flex items-center gap-2 text-sm">
      <input type="checkbox" checked={!muted.includes(key)} onChange={(event) => void toggle(key, event.target.checked)} />
      {t(`settings.profile.email.${key}`)}
    </label>)}
    <FormError message={error} />
  </fieldset>;
}

function ProfileForm({ profile }: { profile: CurrentUser }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const values: Record<string, unknown> = Object.fromEntries(new FormData(event.currentTarget));
    for (const key of ["default_task_duration_days", "default_task_effort_minutes"]) values[key] = values[key] === "" ? null : Number(values[key]);
    setBusy(true);
    setError("");
    try {
      const updated = await api<CurrentUser>("/api/v1/me", { method: "PATCH", body: JSON.stringify(values) });
      client.setQueryData(["me"], updated);
      setRevision((current) => current + 1);
      toast.success(t("settings.profile.saved"));
    } catch (err) {
      setError((err as Error).message);
    } finally { setBusy(false); }
  }

  return <form key={revision} className="max-w-3xl space-y-6" onSubmit={save}>
    <fieldset className="grid gap-4 sm:grid-cols-2" disabled={busy}>
      <legend className="mb-3 text-lg font-semibold">{t("settings.profile.title")}</legend>
      {personalFields.map((field) => <label key={field.key} className="space-y-1 text-sm">
        <span>{t(`settings.profile.fields.${field.key}`)}</span>
        <Input name={field.key} defaultValue={profile[field.key] ?? ""} maxLength={field.max} required={"required" in field} />
      </label>)}
    </fieldset>
    <fieldset id="appearance" className="grid scroll-mt-4 gap-4 sm:grid-cols-2" disabled={busy}>
      <legend className="mb-3 text-lg font-semibold">{t("settings.profile.workspace")}</legend>
      <label className="space-y-1 text-sm">
        <span>{t("settings.profile.fields.timezone")}</span>
        <Input name="timezone" defaultValue={profile.timezone} list="profile-timezones" required />
      </label>
      <datalist id="profile-timezones">
        {timezones.map((zone) => <option value={zone} key={zone} />)}
      </datalist>
      <label className="space-y-1 text-sm">
        <span>{t("settings.profile.fields.default_task_duration_days")}</span>
        <Input name="default_task_duration_days" type="number" min={0} max={3650} step={1} defaultValue={profile.default_task_duration_days ?? ""} />
      </label>
      <label className="space-y-1 text-sm">
        <span>{t("settings.profile.fields.default_task_effort_minutes")}</span>
        <Input name="default_task_effort_minutes" type="number" min={0} max={525600} step={1} defaultValue={profile.default_task_effort_minutes ?? ""} />
      </label>
      <DensityPicker defaultValue={profile.density ?? "compact"} />
      {Object.entries(choices).filter(([key]) => key !== "density").map(([key, options]) => <label key={key} className="space-y-1 text-sm">
        <span>{t(`settings.profile.fields.${key}`)}</span>
        <select className="block h-8 w-full rounded-md border border-input bg-background px-2 focus-visible:outline-ring"
          name={key} defaultValue={profile[key as keyof typeof choices] ?? (key === "theme" ? "system" : options[0])}>
          {options.map((value) => <option value={value} key={value}>
            {t(`settings.profile.options.${value}`, { defaultValue: value })}
          </option>)}
        </select>
      </label>)}
    </fieldset>
    <FormError message={error} />
    <Button type="submit" disabled={busy}>{t("settings.profile.save")}</Button>
  </form>;
}
