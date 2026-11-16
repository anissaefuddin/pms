import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import type { CurrentUser } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { Input } from "@/components/ui/input";
import { ProfileSettingsForm, SelectField, SettingsSection } from "@/components/settings/settings-ui";

export const Route = createFileRoute("/_app/settings/work-preferences")({ component: WorkPreferencesRoute });

const shortcuts = ["mod_enter", "enter"] as const;

// Defaults used when a person creates a task, and how comments are sent.
function WorkPreferencesRoute() {
  const profile = useProfile();
  if (!profile) return null;
  return <ProfileSettingsForm profile={profile} numberFields={["default_task_duration_days", "default_task_effort_minutes"]} key={profile.id}>
    <WorkSection profile={profile} />
  </ProfileSettingsForm>;
}

function WorkSection({ profile }: { profile: CurrentUser }) {
  const { t } = useTranslation();
  return <>
    <SettingsSection title={t("settings.sections.tasks")} description={t("settings.pages.work")}>
      <div className="grid gap-4 sm:grid-cols-2">
        <label className="space-y-1 text-sm">
          <span>{t("settings.profile.fields.default_task_duration_days")}</span>
          <Input name="default_task_duration_days" type="number" min={0} max={3650} step={1} defaultValue={profile.default_task_duration_days ?? ""} />
        </label>
        <label className="space-y-1 text-sm">
          <span>{t("settings.profile.fields.default_task_effort_minutes")}</span>
          <Input name="default_task_effort_minutes" type="number" min={0} max={525600} step={1} defaultValue={profile.default_task_effort_minutes ?? ""} />
        </label>
      </div>
    </SettingsSection>
    <SettingsSection title={t("settings.sections.comments")}>
      <div className="grid gap-4 sm:grid-cols-2">
        <SelectField name="comment_shortcut" label={t("settings.profile.fields.comment_shortcut")} value={profile.comment_shortcut ?? "mod_enter"} options={shortcuts}
          optionLabel={(value) => t(`settings.profile.options.${value}`, { defaultValue: value })} />
      </div>
    </SettingsSection>
  </>;
}
