import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import type { CurrentUser } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { ProfileSettingsForm, SelectField, SettingsSection } from "@/components/settings/settings-ui";

export const Route = createFileRoute("/_app/settings/language")({ component: LanguageRoute });

const timezones = ["UTC", ...Intl.supportedValuesOf("timeZone")];
const selects = {
  locale: ["en", "id"],
  date_format: ["dd/mm/yyyy", "mm/dd/yyyy", "yyyy-mm-dd"],
  time_format: ["12h", "24h"],
  week_start: ["monday", "sunday", "saturday"],
} as const;

// Language and formats. Changing the language here is the same setting as the profile menu.
function LanguageRoute() {
  const profile = useProfile();
  if (!profile) return null;
  return <ProfileSettingsForm profile={profile} key={profile.locale ?? "en"}>
    <LanguageSection profile={profile} />
  </ProfileSettingsForm>;
}

function LanguageSection({ profile }: { profile: CurrentUser }) {
  const { t } = useTranslation();
  return <>
    <SettingsSection title={t("settings.sections.language")} description={t("settings.pages.language")}>
      <div className="grid gap-4 sm:grid-cols-2">
        <SelectField name="locale" label={t("settings.profile.fields.locale")} value={profile.locale ?? "en"} options={selects.locale}
          optionLabel={(value) => t(`settings.profile.options.${value}`, { defaultValue: value })} />
        <SelectField name="timezone" label={t("settings.profile.fields.timezone")} value={profile.timezone} options={timezones}
          optionLabel={(zone) => zone} />
      </div>
    </SettingsSection>
    <SettingsSection title={t("settings.sections.formats")}>
      <div className="grid gap-4 sm:grid-cols-2">
        <SelectField name="date_format" label={t("settings.profile.fields.date_format")} value={profile.date_format} options={selects.date_format} optionLabel={(value) => value} />
        <SelectField name="time_format" label={t("settings.profile.fields.time_format")} value={profile.time_format} options={selects.time_format} optionLabel={(value) => t(`settings.profile.options.${value}`, { defaultValue: value })} />
        <SelectField name="week_start" label={t("settings.profile.fields.week_start")} value={profile.week_start} options={selects.week_start} optionLabel={(value) => t(`settings.profile.options.${value}`, { defaultValue: value })} />
      </div>
    </SettingsSection>
  </>;
}
