import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, type CurrentUser } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { FormError } from "@/components/auth/form-error";
import { SettingsRow, SettingsSection, SettingsSwitch } from "@/components/settings/settings-ui";

export const Route = createFileRoute("/_app/settings/email")({ component: EmailRoute });

const emailGroups = ["mention", "assigned", "shared", "task_changes", "report"] as const;

// Email preferences are on/off switches that save at once, so there is no Save button on this page.
function EmailRoute() {
  const profile = useProfile();
  if (!profile) return null;
  return <EmailSettings profile={profile} key={profile.id} />;
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
  return <SettingsSection title={t("settings.email.activity")} description={t("settings.pages.email")}>
    <div>
      {emailGroups.map((key) => <SettingsRow key={key} label={t(`settings.profile.email.${key}`)} hint={t(`settings.email.hint.${key}`)}>
        <SettingsSwitch label={t(`settings.profile.email.${key}`)} checked={!muted.includes(key)} onChange={(on) => void toggle(key, on)} />
      </SettingsRow>)}
    </div>
    <p className="rounded-md bg-muted px-3 py-2 text-xs text-muted-foreground">{t("settings.email.note")}</p>
    <FormError message={error} />
  </SettingsSection>;
}
