import { createFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/settings/resources")({ component: Resources });
type Choice = { id: string; name: string };
type Directory = { roles: Choice[]; skills: Choice[]; users: (Choice & { roles: string[]; skills: string[] })[] };
type Capacity = { working_days: number[] | null; daily_hours: number | null; leave_dates: string[] };

function Resources() {
  const me = useProfile();
  const [user, setUser] = useState("");
  const [editing, setEditing] = useState<Choice>();
  const cache = useQueryClient();
  const admin = me?.user_type === "admin";
  const id = user || me?.id;
  const query = useQuery({ queryKey: ["resource-directory"], enabled: admin, queryFn: () => api<Directory>("/api/v1/resource-directory") });
  const capacity = useQuery({ queryKey: ["capacity", id], enabled: !!id, queryFn: () => api<Capacity>(`/api/v1/users/${id}/capacity`) });
  const save = useMutation({ mutationFn: ({ path, method, body }: { path: string; method: string; body?: object }) =>
    api(path, { method, body: body && JSON.stringify(body) }),
    onSuccess: async () => {
      setEditing(undefined);
      await Promise.all(["resource-directory", "capacity", "workload"].map(key => cache.invalidateQueries({queryKey:[key]})));
    } });
  const directory = query.data;
  return <section className="space-y-5">
    <h2 className="text-lg font-semibold">Resource settings</h2>
    {(save.error || query.error || capacity.error) && <p role="alert">{(save.error ?? query.error ?? capacity.error)?.message}</p>}
    {admin && <label>Person<select value={user} onChange={e => setUser(e.target.value)} className="block rounded border bg-background p-2">
      <option value="">My schedule</option>{directory?.users.filter(x => x.id !== me.id).map(x => <option key={x.id} value={x.id}>{x.name}</option>)}
    </select></label>}
    {capacity.data && <CapacityForm key={JSON.stringify([id, capacity.data])} value={capacity.data} busy={save.isPending} onSave={body =>
      save.mutate({path:`/api/v1/users/${id}/capacity`,method:"PUT",body})} />}
    {admin && directory && (["role","skill"] as const).map(kind => {
      const entries = kind === "role" ? directory.roles : directory.skills;
      const selected = directory.users.find(x => x.id === id);
      const assigned = kind === "role" ? selected?.roles : selected?.skills;
      const current = entries.find(x => x.id === editing?.id);
      return <section key={kind} className="space-y-2">
        <h3 className="font-semibold">{kind === "role" ? "Job roles" : "Skills"}</h3>
        <form key={current?.id ?? "new"} className="flex flex-wrap items-end gap-2" onSubmit={event => {
          event.preventDefault();
          const form = event.currentTarget;
          save.mutate({path:"/api/v1/resource-directory",method:current ? "PUT" : "POST",
            body:{kind,name:new FormData(form).get("name"),...(current ? {id:current.id} : {})}}, { onSuccess: () => form.reset() });
        }}>
          <label>{kind === "role" ? "Job role name" : "Skill name"}<Input name="name" required maxLength={120} defaultValue={current?.name}/></label>
          <Button type="submit" disabled={save.isPending}>{current ? "Save name" : "Add"}</Button>
        </form>
        <ul>{entries.map(x => <li key={x.id} className="flex flex-wrap items-center gap-2 py-1">
          <label className="flex gap-2"><input type="checkbox" disabled={!selected || save.isPending} checked={assigned?.includes(x.id) ?? false} onChange={event =>
            save.mutate({path:"/api/v1/resource-directory",method:"PUT",body:{kind,id:x.id,user_id:id,assigned:event.target.checked}})}/>{x.name}</label>
          <Button size="sm" variant="ghost" onClick={() => setEditing(x)}>Rename</Button>
          <Button size="sm" variant="outline" disabled={save.isPending} onClick={() => save.mutate({path:`/api/v1/resource-directory?kind=${kind}&id=${x.id}`,method:"DELETE"})}>Remove</Button>
        </li>)}</ul>
      </section>;
    })}
  </section>;
}

function CapacityForm({value,busy,onSave}: {value:Capacity;busy:boolean;onSave:(value:Capacity)=>void}) {
  const [inherit,setInherit] = useState(value.daily_hours == null);
  const [leave,setLeave] = useState(value.leave_dates);
  return <form className="space-y-3" onSubmit={event => {
    event.preventDefault();
    const fields=new FormData(event.currentTarget);
    onSave({working_days:inherit ? null : fields.getAll("day").map(Number),daily_hours:inherit ? null : Number(fields.get("hours")),leave_dates:leave});
  }}>
    <label className="flex gap-2"><input type="checkbox" checked={inherit} onChange={event => setInherit(event.target.checked)}/>Use workspace working hours</label>
    {!inherit && <fieldset className="flex flex-wrap gap-3"><legend>Working days</legend>
      {["Sun","Mon","Tue","Wed","Thu","Fri","Sat"].map((day,i) => <label key={day}><input type="checkbox" name="day" value={i} defaultChecked={value.working_days?.includes(i) ?? (i > 0 && i < 6)} /> {day}</label>)}
      <label>Daily hours<Input name="hours" type="number" min={1} max={24} required defaultValue={value.daily_hours ?? 8}/></label>
    </fieldset>}
    <fieldset className="space-y-2"><legend>Leave dates</legend>
      {leave.map(date => <div key={date} className="flex items-center gap-2">{date}<Button type="button" variant="ghost" onClick={() => setLeave(leave.filter(x => x !== date))}>Remove</Button></div>)}
      <label>Add leave date<Input type="date" value="" onChange={event => { if (event.target.value && !leave.includes(event.target.value)) setLeave([...leave,event.target.value].sort()); }} /></label>
    </fieldset>
    <Button type="submit" disabled={busy}>Save capacity</Button>
  </form>;
}
