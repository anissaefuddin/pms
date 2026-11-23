import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, type CurrentUser } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { FormError } from "@/components/auth/form-error";
import { SettingsRow, SettingsSection, SettingsSwitch } from "@/components/settings/settings-ui";

export const Route = createFileRoute("/_app/settings/email")({ component: EmailRoute });
const activityGroups = ["mention", "assigned", "shared", "task_changes"] as const;
type DuePreferences = { enabled:boolean; days_before:number[]; overdue:boolean; overdue_every_days:number; delivery:"digest"|"individual"; hour:number };
const defaults:DuePreferences={enabled:false,days_before:[3,1,0],overdue:true,overdue_every_days:3,delivery:"digest",hour:8};

function EmailRoute(){const profile=useProfile();return profile?<EmailSettings profile={profile} key={profile.id}/>:null}
function EmailSettings({profile}:{profile:CurrentUser}){
  const {t}=useTranslation();const client=useQueryClient();const [error,setError]=useState("");const [muted,setMuted]=useState(profile.email_muted??[]);const [due,setDue]=useState<DuePreferences>((profile.due_reminders as DuePreferences|undefined)??defaults);
  async function patch(body:Record<string,unknown>){client.setQueryData(["me"],await api<CurrentUser>("/api/v1/me",{method:"PATCH",body:JSON.stringify(body)}))}
  async function toggle(key:(typeof activityGroups)[number]|"report",on:boolean){setError("");const previous=muted;const next=on?muted.filter(x=>x!==key):[...muted,key];setMuted(next);try{await patch({email_muted:next})}catch(e){setMuted(previous);setError((e as Error).message)}}
  async function saveDue(next:DuePreferences){setError("");const previous=due;setDue(next);try{await patch({due_reminders:next})}catch(e){setDue(previous);setError((e as Error).message)}}
  const day=(value:number,label:string)=><label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={due.days_before.includes(value)} disabled={!due.enabled} onChange={e=>void saveDue({...due,days_before:e.target.checked?[...due.days_before,value]:due.days_before.filter(x=>x!==value)})}/>{label}</label>;
  return <div className="max-w-3xl space-y-6">
    <SettingsSection title={t("settings.email.activity")} description={t("settings.pages.email")}><div>{activityGroups.map(key=><SettingsRow key={key} label={t(`settings.profile.email.${key}`)} hint={t(`settings.email.hint.${key}`)}><SettingsSwitch label={t(`settings.profile.email.${key}`)} checked={!muted.includes(key)} onChange={on=>void toggle(key,on)}/></SettingsRow>)}</div></SettingsSection>
    <SettingsSection title={t("settings.email.due.title")} description={t("settings.email.due.description")}>
      <SettingsRow label={t("settings.email.due.enable")} hint={t("settings.email.due.enableHint")}><SettingsSwitch label={t("settings.email.due.enable")} checked={due.enabled} onChange={enabled=>void saveDue({...due,enabled})}/></SettingsRow>
      <div className="space-y-3 rounded-lg border p-4"><p className="text-sm font-medium">{t("settings.email.due.before")}</p><div className="flex flex-wrap gap-5">{day(3,t("settings.email.due.h3"))}{day(1,t("settings.email.due.h1"))}{day(0,t("settings.email.due.today"))}</div></div>
      <SettingsRow label={t("settings.email.due.overdue")} hint={t("settings.email.due.overdueHint")}><SettingsSwitch label={t("settings.email.due.overdue")} checked={due.overdue} disabled={!due.enabled} onChange={overdue=>void saveDue({...due,overdue})}/></SettingsRow>
      <div className="grid gap-4 sm:grid-cols-3">
        <label className="space-y-1 text-sm"><span>{t("settings.email.due.repeat")}</span><select className="h-9 w-full rounded-md border bg-background px-2" disabled={!due.enabled||!due.overdue} value={due.overdue_every_days} onChange={e=>void saveDue({...due,overdue_every_days:Number(e.target.value)})}>{[1,3,7].map(n=><option key={n} value={n}>{t("settings.email.due.everyDays",{count:n})}</option>)}</select></label>
        <label className="space-y-1 text-sm"><span>{t("settings.email.due.delivery")}</span><select className="h-9 w-full rounded-md border bg-background px-2" disabled={!due.enabled} value={due.delivery} onChange={e=>void saveDue({...due,delivery:e.target.value as DuePreferences["delivery"]})}><option value="digest">{t("settings.email.due.digest")}</option><option value="individual">{t("settings.email.due.individual")}</option></select></label>
        <label className="space-y-1 text-sm"><span>{t("settings.email.due.sendAt")}</span><select className="h-9 w-full rounded-md border bg-background px-2" disabled={!due.enabled} value={due.hour} onChange={e=>void saveDue({...due,hour:Number(e.target.value)})}>{Array.from({length:24},(_,h)=><option key={h} value={h}>{String(h).padStart(2,"0")}:00</option>)}</select></label>
      </div><p className="text-xs text-muted-foreground">{t("settings.email.due.timezone",{timezone:profile.timezone})}</p>
    </SettingsSection>
    <SettingsSection title={t("settings.email.reports")}><SettingsRow label={t("settings.profile.email.report")} hint={t("settings.email.hint.report")}><SettingsSwitch label={t("settings.profile.email.report")} checked={!muted.includes("report")} onChange={on=>void toggle("report",on)}/></SettingsRow></SettingsSection>
    <p className="rounded-md bg-muted px-3 py-2 text-xs text-muted-foreground">{t("settings.email.note")}</p><FormError message={error}/>
  </div>
}
