import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import type { CurrentUser } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { Input } from "@/components/ui/input";
import { ProfileSettingsForm, SettingsSection } from "@/components/settings/settings-ui";

export const Route = createFileRoute("/_app/settings/profile")({ component: ProfileRoute });

// Profile is about who the person is. Language, work defaults, appearance and email have their own pages.
const identityFields = [
  { key: "first_name", max: 80, required: true },
  { key: "last_name", max: 80, required: true },
  { key: "job_title", max: 80 },
  { key: "company", max: 120 },
  { key: "department", max: 80 },
  { key: "location", max: 120 },
] as const;

function ProfileRoute() {
  const profile = useProfile();
  if (!profile) return null;
  return <>
    <Identity profile={profile} />
    <ProfileSettingsForm profile={profile} key={profile.id}>
      <IdentitySection profile={profile} />
    </ProfileSettingsForm>
  </>;
}

function Identity({ profile }: { profile: CurrentUser }) {
  const initials = `${profile.first_name[0] ?? ""}${profile.last_name[0] ?? ""}`.toUpperCase();
  const role = [profile.job_title, profile.company].filter(Boolean).join(" · ");
  return <div className="mb-6 flex items-center gap-4">
    <span aria-hidden="true" className="grid size-16 place-items-center rounded-xl bg-primary text-xl font-semibold text-primary-foreground">{initials}</span>
    <div><p className="text-lg font-semibold">{profile.first_name} {profile.last_name}</p>{role && <p className="text-sm text-muted-foreground">{role}</p>}</div>
  </div>;
}

function IdentitySection({ profile }: { profile: CurrentUser }) {
  const { t } = useTranslation();
  return <SettingsSection title={t("settings.sections.personal")} description={t("settings.pages.profile")}>
    <div className="grid gap-4 sm:grid-cols-2">
      {identityFields.map((field) => <label key={field.key} className="space-y-1 text-sm">
        <span>{t(`settings.profile.fields.${field.key}`)}</span>
        <Input name={field.key} defaultValue={profile[field.key] ?? ""} maxLength={field.max} required={"required" in field} />
      </label>)}
      <label className="space-y-1 text-sm">
        <span>{t("settings.profile.fields.country")}</span>
        <Input name="country" defaultValue={profile.country ?? ""} maxLength={2} />
      </label>
      <label className="space-y-1 text-sm sm:col-span-2">
        <span>{t("settings.profile.fields.phone")}</span>
        <Input name="phone" defaultValue={profile.phone ?? ""} maxLength={32} />
      </label>
    </div>
  </SettingsSection>;
}
