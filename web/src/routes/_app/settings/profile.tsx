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

function ProfileRoute() {
  const profile = useProfile();
  return profile && <ProfileForm key={profile.id} profile={profile} />;
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
    const values = Object.fromEntries(new FormData(event.currentTarget));
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
