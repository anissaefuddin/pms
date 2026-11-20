import { createFileRoute } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api, type CurrentUser } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { UserAvatar } from "@/components/user-avatar";
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

const PHOTO_TYPES = ["image/png", "image/jpeg", "image/webp"];
const PHOTO_MAX_BYTES = 2 << 20;

function Identity({ profile }: { profile: CurrentUser }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const role = [profile.job_title, profile.company].filter(Boolean).join(" · ");
  async function save(request: Promise<CurrentUser>) {
    setBusy(true);
    try {
      queryClient.setQueryData(["me"], await request);
    } catch (err) {
      toast.error((err as Error).message);
    } finally {
      setBusy(false);
    }
  }
  function choose(file: File | undefined) {
    if (!file) return;
    if (!PHOTO_TYPES.includes(file.type)) return void toast.error(t("settings.profile.photo.type"));
    if (file.size > PHOTO_MAX_BYTES) return void toast.error(t("settings.profile.photo.size"));
    void save(api<CurrentUser>("/api/v1/me/avatar", { method: "PUT", headers: { "Content-Type": file.type }, body: file }));
  }
  return <div className="mb-6 flex items-center gap-4">
    <UserAvatar profile={profile} className="size-16 rounded-xl text-xl" />
    <div className="min-w-0 flex-1"><p className="text-lg font-semibold">{profile.first_name} {profile.last_name}</p>{role && <p className="text-sm text-muted-foreground">{role}</p>}</div>
    <div className="flex shrink-0 flex-col items-end gap-1">
      <input ref={input} type="file" accept={PHOTO_TYPES.join(",")} className="sr-only" aria-label={t("settings.profile.photo.label")}
        onChange={(event) => { choose(event.target.files?.[0]); event.target.value = ""; }} />
      <Button variant="outline" size="sm" disabled={busy} onClick={() => input.current?.click()}>{t("settings.profile.photo.change")}</Button>
      {profile.avatar_version > 0 && <Button variant="ghost" size="sm" disabled={busy} onClick={() => void save(api<CurrentUser>("/api/v1/me/avatar", { method: "DELETE" }))}>{t("settings.profile.photo.remove")}</Button>}
      <p className="text-xs text-muted-foreground">{t("settings.profile.photo.hint")}</p>
    </div>
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
