import { useTranslation } from "react-i18next";
import { createFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { AdminEmptyState, AdminPageHeader, InitialsAvatar } from "@/components/admin/admin-ui";

export const Route = createFileRoute("/_app/settings/resources")({ component: Resources });
type Choice = { id: string; name: string };
type Directory = { roles: Choice[]; skills: Choice[]; users: (Choice & { roles: string[]; skills: string[] })[] };
type Capacity = { working_days: number[] | null; daily_hours: number | null; leave_dates: string[] };
type Tab = "capacity" | "roles" | "skills";

// Resource configuration center. Each tab answers one question, so the scope of every setting is visible:
// Capacity = when a person can work, Job roles = what kind of professional they are, Skills = what they can do.
function Resources() {
  const { t } = useTranslation();
  const me = useProfile();
  const [tab, setTab] = useState<Tab>("capacity");
  const [user, setUser] = useState("");
  const cache = useQueryClient();
  const admin = me?.user_type === "admin";
  const id = user || me?.id;
  const query = useQuery({ queryKey: ["resource-directory"], enabled: admin, queryFn: () => api<Directory>("/api/v1/resource-directory") });
  const capacity = useQuery({ queryKey: ["capacity", id], enabled: !!id, queryFn: () => api<Capacity>(`/api/v1/users/${id}/capacity`) });
  const save = useMutation({ mutationFn: ({ path, method, body }: { path: string; method: string; body?: object }) =>
    api(path, { method, body: body && JSON.stringify(body) }),
    onSuccess: async () => {
      await Promise.all(["resource-directory", "capacity", "workload"].map((key) => cache.invalidateQueries({ queryKey: [key] })));
    } });
  const directory = query.data;
  const tabs: { id: Tab; label: string; adminOnly: boolean }[] = [
    { id: "capacity", label: t("settings.resources.tabs.capacity"), adminOnly: false },
    { id: "roles", label: t("settings.resourceSettings.jobRoles"), adminOnly: true },
    { id: "skills", label: t("settings.resourceSettings.skills"), adminOnly: true },
  ];
  const visibleTabs = tabs.filter((item) => admin || !item.adminOnly);
  const selected = directory?.users.find((entry) => entry.id === id);
  return <section className="min-w-0 space-y-6">
    <AdminPageHeader description={t("settings.resources.description")} />
    {(save.error || query.error || capacity.error) && <p role="alert" className="text-sm text-destructive">{(save.error ?? query.error ?? capacity.error)?.message}</p>}

    <nav aria-label={t("settings.resources.sections")} className="flex gap-4 border-b text-sm">
      {visibleTabs.map((item) => <button key={item.id} type="button" aria-current={tab === item.id ? "page" : undefined} onClick={() => setTab(item.id)}
        className={`-mb-px border-b-2 px-1 pb-2 ${tab === item.id ? "border-primary font-medium" : "border-transparent text-muted-foreground hover:text-foreground"}`}>{item.label}</button>)}
    </nav>

    {tab === "capacity" && <section className="space-y-6">
      {admin && <label className="block max-w-sm space-y-1 text-sm">{t("settings.resourceSettings.person")}
        <select value={user} onChange={(event) => setUser(event.target.value)} className="block h-9 w-full rounded-md border border-input bg-background px-2">
          <option value="">{t("settings.resourceSettings.mySchedule")}</option>
          {directory?.users.filter((entry) => entry.id !== me?.id).map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}
        </select>
      </label>}
      <p className="text-sm text-muted-foreground">{t("settings.resources.capacityScope")}</p>
      {capacity.data && <CapacityForm key={JSON.stringify([id, capacity.data])} value={capacity.data} busy={save.isPending} onSave={(body) =>
        save.mutate({ path: `/api/v1/users/${id}/capacity`, method: "PUT", body })} />}
    </section>}

    {tab === "roles" && admin && directory && <AssignmentList kind="role" entries={directory.roles} assigned={selected?.roles ?? []} person={selected} busy={save.isPending}
      onSave={(body) => save.mutate({ path: "/api/v1/resource-directory", method: body.method, body: body.body })} onPerson={setUser} people={directory.users} selectedId={id} />}
    {tab === "skills" && admin && directory && <AssignmentList kind="skill" entries={directory.skills} assigned={selected?.skills ?? []} person={selected} busy={save.isPending}
      onSave={(body) => save.mutate({ path: "/api/v1/resource-directory", method: body.method, body: body.body })} onPerson={setUser} people={directory.users} selectedId={id} />}
  </section>;
}

/** Job roles or skills: a short list with the person's assignments and the rename/remove actions. */
function AssignmentList({ kind, entries, assigned, person, busy, onSave, onPerson, people, selectedId }: {
  kind: "role" | "skill"; entries: Choice[]; assigned: string[]; person?: Choice; busy: boolean;
  onSave: (value: { method: string; body: object }) => void; onPerson: (id: string) => void; people: Choice[]; selectedId?: string;
}) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState<Choice>();
  const title = kind === "role" ? t("settings.resourceSettings.jobRoles") : t("settings.resourceSettings.skills");
  const description = kind === "role" ? t("settings.resources.rolesScope") : t("settings.resources.skillsScope");
  return <section className="space-y-5">
    <p className="text-sm text-muted-foreground">{description}</p>
    <form key={editing?.id ?? "new"} className="flex flex-wrap items-end gap-2" onSubmit={(event) => {
      event.preventDefault();
      const form = event.currentTarget;
      const name = String(new FormData(form).get("name") ?? "").trim();
      onSave({ method: editing ? "PUT" : "POST", body: { kind, name, ...(editing ? { id: editing.id } : {}) } });
      setEditing(undefined);
      form.reset();
    }}>
      <label className="block space-y-1 text-sm">{kind === "role" ? t("settings.resourceSettings.jobRoleName") : t("settings.resourceSettings.skillName")}
        <Input name="name" required maxLength={120} defaultValue={editing?.name} className="w-64" />
      </label>
      <Button type="submit" disabled={busy}>{editing ? t("settings.resourceSettings.saveName") : t("settings.resourceSettings.add")}</Button>
    </form>

    <label className="block max-w-sm space-y-1 text-sm">{t("settings.resourceSettings.person")}
      <select value={selectedId ?? ""} onChange={(event) => onPerson(event.target.value)} className="block h-9 w-full rounded-md border border-input bg-background px-2">
        {people.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}
      </select>
    </label>

    {entries.length ? <ul className="divide-y rounded-lg border">
      {entries.map((entry) => <li key={entry.id} className="flex flex-wrap items-center gap-3 px-3 py-2.5">
        <label className="flex min-w-0 flex-1 items-center gap-2 text-sm">
          <input type="checkbox" disabled={!person || busy} checked={assigned.includes(entry.id)}
            aria-label={t("settings.resources.assignTo", { item: entry.name, name: person?.name ?? "" })}
            onChange={(event) => onSave({ method: "PUT", body: { kind, id: entry.id, user_id: person?.id, assigned: event.target.checked } })} />
          <span className="truncate">{entry.name}</span>
        </label>
        <Button size="sm" variant="ghost" onClick={() => setEditing(entry)}>{t("settings.resourceSettings.rename")}</Button>
        <Button size="sm" variant="outline" disabled={busy} onClick={() => onSave({ method: "DELETE", body: { kind, id: entry.id } })}>{t("settings.resourceSettings.remove")}</Button>
      </li>)}
    </ul> : <AdminEmptyState title={t("settings.resources.emptyTitle", { title: title.toLowerCase() })} description={description} />}
  </section>;
}

function CapacityForm({ value, busy, onSave }: { value: Capacity; busy: boolean; onSave: (value: Capacity) => void }) {
  const { t } = useTranslation();
  const [inherit, setInherit] = useState(value.daily_hours == null);
  const [leave, setLeave] = useState(value.leave_dates);
  return <form className="space-y-6" onSubmit={(event) => {
    event.preventDefault();
    const fields = new FormData(event.currentTarget);
    onSave({ working_days: inherit ? null : fields.getAll("day").map(Number), daily_hours: inherit ? null : Number(fields.get("hours")), leave_dates: leave });
  }}>
    <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={inherit} onChange={(event) => setInherit(event.target.checked)} />{t("settings.resourceSettings.useWorkspaceWorkingHours")}</label>
    {!inherit && <fieldset className="space-y-3">
      <legend className="mb-2 text-sm font-medium">{t("settings.resourceSettings.workingDays")}</legend>
      <div className="flex flex-wrap gap-2">
        {[0, 1, 2, 3, 4, 5, 6].map((i) => <label key={i} className="inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1 text-sm has-[:checked]:border-primary has-[:checked]:bg-accent">
          <input type="checkbox" name="day" value={i} defaultChecked={value.working_days?.includes(i) ?? (i > 0 && i < 6)} />
          {t(`settings.resourceSettings.weekday${i}`)}
        </label>)}
      </div>
      <label className="block max-w-xs space-y-1 text-sm">{t("settings.resourceSettings.dailyHours")}
        <Input name="hours" type="number" min={1} max={24} required defaultValue={value.daily_hours ?? 8} className="w-28" />
      </label>
    </fieldset>}
    <fieldset className="space-y-2">
      <legend className="mb-2 text-sm font-medium">{t("settings.resourceSettings.leaveDates")}</legend>
      {leave.length ? <ul className="divide-y rounded-md border">
        {leave.map((date) => <li key={date} className="flex items-center justify-between px-3 py-2 text-sm">
          <span>{date}</span>
          <Button type="button" variant="ghost" size="sm" aria-label={t("settings.resources.removeLeave", { date })} onClick={() => setLeave(leave.filter((entry) => entry !== date))}>{t("settings.resourceSettings.remove")}</Button>
        </li>)}
      </ul> : <p className="text-sm text-muted-foreground">{t("settings.resources.noLeave")}</p>}
      <label className="block max-w-xs space-y-1 text-sm">{t("settings.resourceSettings.addLeaveDate")}
        <Input type="date" value="" className="w-48" onChange={(event) => { if (event.target.value && !leave.includes(event.target.value)) setLeave([...leave, event.target.value].sort()); }} />
      </label>
    </fieldset>
    <Button type="submit" disabled={busy}>{t("settings.resourceSettings.saveCapacity")}</Button>
  </form>;
}
