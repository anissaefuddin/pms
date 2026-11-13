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
  locale: ["en"],
  date_format: ["dd/mm/yyyy", "mm/dd/yyyy", "yyyy-mm-dd"],
  time_format: ["12h", "24h"],
  week_start: ["monday", "sunday", "saturday"],
  density: ["compact", "comfortable"],
  theme: ["light", "dark", "system"],
  comment_shortcut: ["mod_enter", "enter"],
} as const;
const timezones = ["UTC", ...Intl.supportedValuesOf("timeZone")];

// Only entries that exist in this version are offered; the other keys the API accepts stay hidden (NAV-02).
const emailGroups = ["mention", "assigned", "shared", "task_changes", "report"] as const;
const sidebarChoices = ["my_todo", "request_forms", "stream", "timesheets"] as const;

function ProfileRoute() {
  const profile = useProfile();
  return profile && <><ProfileForm key={profile.id} profile={profile} /><SidebarSettings profile={profile} /><EmailSettings profile={profile} /></>;
}

function SidebarSettings({ profile }: { profile: CurrentUser }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [error, setError] = useState("");
  const [hidden, setHidden] = useState(profile.sidebar_hidden ?? []); // local, so the checkbox flips at once
  async function toggle(key: (typeof sidebarChoices)[number], visible: boolean) {
    setError("");
    const previous = hidden;
    const next = visible ? hidden.filter((entry) => entry !== key) : [...hidden, key];
    setHidden(next);
    try {
      client.setQueryData(["me"], await api<CurrentUser>("/api/v1/me", { method: "PATCH", body: JSON.stringify({ sidebar_hidden: next }) }));
    } catch (err) {
      setHidden(previous);
      setError((err as Error).message);
    }
  }
  return <fieldset className="mt-6 max-w-3xl space-y-2">
    <legend className="mb-1 text-lg font-semibold">{t("settings.profile.sidebar.title")}</legend>
    <p className="text-sm text-muted-foreground">{t("settings.profile.sidebar.hint")}</p>
    {sidebarChoices.map((key) => <label key={key} className="flex items-center gap-2 text-sm">
      <input type="checkbox" checked={!hidden.includes(key)} onChange={(event) => void toggle(key, event.target.checked)} />
      {t(`settings.profile.sidebar.${key}`)}
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
  return <fieldset className="mt-6 max-w-3xl space-y-2">
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
    <fieldset className="grid gap-4 sm:grid-cols-2" disabled={busy}>
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
      {Object.entries(choices).map(([key, options]) => <label key={key} className="space-y-1 text-sm">
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
