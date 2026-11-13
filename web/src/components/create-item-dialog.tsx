import { useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { ApiError, api, type CurrentUser } from "@/lib/api";
import { useSpaces } from "@/lib/space-queries";
import { isWorkday, itemKeys, listItems, type Item, type ItemCreate, type ItemType, type Location } from "@/lib/items";
import type { Group, Share } from "@/lib/spaces";
import { Search } from "@/components/share-dialog";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { ItemUserPicker, type PersonOption } from "@/components/item-user-picker";
import { useBlueprints } from "@/lib/blueprints";
import { ItemDates } from "@/components/item-dates";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  me: CurrentUser;
  location?: Location;
  taskOnly?: boolean;
  initialStatus?: string;
  onCreated: (item: Item) => void;
};
const itemTypes = ["project", "folder", "task"] as const;
const viewTypes = ["table", "board"] as const;

export function CreateItemDialog(props: Props) {
  const { t } = useTranslation();
  return <Dialog open={props.open} onOpenChange={props.onOpenChange}>
    <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
      <DialogTitle>{t(props.location?.parent_item_id ? "items.addChild" : "items.create")}</DialogTitle>
      {props.open && <CreateForm {...props} />}
    </DialogContent>
  </Dialog>;
}

function CreateForm({ me, location, onCreated, onOpenChange, taskOnly, initialStatus }: Props) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const spaces = useSpaces();
  const fixedParent = location?.parent_item_id;
  const [kind, setKind] = useState<ItemType>(fixedParent || taskOnly ? "task" : "project");
  const [spaceID, setSpaceID] = useState(location?.space_id ?? me.personal_space_id);
  const [parentID, setParentID] = useState(fixedParent ?? "");
  const [trail, setTrail] = useState<Item[]>([]);
  const [owner, setOwner] = useState<PersonOption[]>([{ id: me.id, name: `${me.first_name} ${me.last_name}`.trim() }]);
  const [shares, setShares] = useState<Share[]>([]);
  const [error, setError] = useState("");
  const [fieldError, setFieldError] = useState<"title" | "due_date" | "locations" | null>(null);
  const [busy, setBusy] = useState(false);
  const [blueprintID, setBlueprintID] = useState("");
  const choices = spaces.data?.filter((space) => space.my_capabilities?.includes("create")) ?? [];
  const selected = choices.find((space) => space.id === spaceID);
  const directory = trail.at(-1);
  const parents = useQuery({
    queryKey: directory ? itemKeys.children(directory.id) : itemKeys.root(spaceID),
    enabled: !!selected && !fixedParent,
    queryFn: () => listItems(directory ? `/api/v1/items/${directory.id}/children` : `/api/v1/spaces/${spaceID}/items`),
  });
  const parent = parentID === directory?.id ? directory : parents.data?.find((item) => item.id === parentID);
  const parentOptions = parents.data?.filter((item) => item.type !== "task" && item.my_capabilities.includes("create")
    && item.locations.some((loc) => loc.space_id === spaceID && (loc.parent_item_id ?? "") === (directory?.id ?? "")));
  const canBrowse = !!parent && parent.id !== directory?.id;
  const canShare = kind !== "task" && selected?.kind === "shared" && (parent ?? selected).my_capabilities?.includes("share");
  const groups = useQuery({
    queryKey: ["groups"], enabled: !!canShare,
    queryFn: async () => (await api<{ items: Group[] }>("/api/v1/groups")).items,
  });

  const blueprints = useBlueprints(spaceID, !!selected).data ?? [];
  const blueprint = blueprints.find((entry) => entry.id === blueprintID);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const form = new FormData(event.currentTarget);
    const title = String(form.get("title") ?? "").trim();
    const start = String(form.get("start_date") ?? "");
    const due = String(form.get("due_date") ?? "");
    setFieldError(null);
    if (!title) { setFieldError("title"); return setError(t("items.nameRequired")); }
    if (!fixedParent && !selected) { setFieldError("locations"); return setError(t("items.locationRequired")); }
    if (start && due && due < start) { setFieldError("due_date"); return setError(t("items.dateOrder")); }
    if ([start, due].some((day) => day && !isWorkday(day))) {
      setFieldError("due_date");
      return setError(t("items.workingDays"));
    }
    const input: ItemCreate = { type: kind, title, locations: [{ space_id: spaceID, parent_item_id: parentID || null }] };
    if (initialStatus) input.status_id = initialStatus;
    if (kind !== "task") input.default_view_type = form.get("default_view_type") === "board" ? "board" : "table";
    if (kind === "project" && owner[0]) input.owner_id = owner[0].id;
    if (kind === "task") { // SET-03: the user's defaults for a new task
      if (me.default_task_duration_days != null) input.duration_days = me.default_task_duration_days;
      if (me.default_task_effort_minutes != null) input.effort_minutes = me.default_task_effort_minutes;
    }
    if (start) input.start_date = start;
    if (due) input.due_date = due;
    if (canShare && shares.length) input.shares = shares.map(({ principal_type, principal_id, role }) => ({ principal_type, principal_id, role }));
    setBusy(true);
    setError("");
    try {
      const item = blueprint
        ? await api<Item>(`/api/v1/blueprints/${blueprint.id}/items`, {
          method: "POST", body: JSON.stringify({ locations: input.locations, title, ...(start && { start_date: start }) }),
        })
        : await api<Item>("/api/v1/items", { method: "POST", body: JSON.stringify(input) });
      await queryClient.invalidateQueries({ queryKey: itemKeys.all, predicate: ({ queryKey }) => queryKey[1] !== "detail" });
      onOpenChange(false);
      onCreated(item);
    } catch (err) {
      if (err instanceof ApiError) {
        const problem = err.fields.find(({ field }) => field === "title" || field === "due_date" || field === "locations");
        if (problem && (problem.field === "title" || problem.field === "due_date" || problem.field === "locations")) {
          setFieldError(problem.field);
          setError(problem.message);
          return;
        }
      }
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return <form noValidate onSubmit={submit} className="space-y-4">
    <div className="grid gap-4 sm:grid-cols-2">
      <div className="space-y-3">
        <label className="block space-y-1"><span>{t("items.name")}</span>
          <Input name="title" maxLength={500} required aria-invalid={fieldError === "title"} /></label>
        {!fixedParent && <>
          <label className="block space-y-1"><span>{t("items.space")}</span>
            <select value={selected?.id ?? ""} aria-invalid={fieldError === "locations"} className="block w-full rounded border bg-background p-2"
              onChange={(e) => { setSpaceID(e.target.value); setParentID(""); setTrail([]); setShares([]); setBlueprintID(""); }}>
              <option value="">{t("items.chooseSpace")}</option>
              {choices.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
            </select>
          </label>
          <label className="block space-y-1"><span>{t("items.parent")}</span>
            <select value={parentID} className="block w-full rounded border bg-background p-2"
              onChange={(e) => { setParentID(e.target.value); setShares([]); }}>
              <option value={directory?.id ?? ""}>{directory?.title ?? t("items.root")}</option>
              {parentOptions?.map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}
            </select>
          </label>
          <div className="flex flex-wrap gap-1">
            {directory && <Button size="sm" type="button" variant="ghost" onClick={() => {
              setParentID(trail.at(-2)?.id ?? "");
              setTrail(trail.slice(0, -1));
              setShares([]);
            }}>{t("items.parentLink")}</Button>}
            {canBrowse && <Button size="sm" type="button" variant="outline" onClick={() => setTrail([...trail, parent])}>
              {t("items.browseChildren")}
            </Button>}
          </div>
          {!taskOnly && !blueprint && <fieldset className="flex gap-3">
            <legend>{t("items.type")}</legend>
            {itemTypes.map((type) => <label className="flex items-center gap-1" key={type}>
              <input type="radio" name="kind" checked={type === kind} onChange={() => setKind(type)} />{t(`items.${type}`)}
            </label>)}
          </fieldset>}
          {kind !== "task" && <p>{t("items.workflow")}: {t("items.defaultWorkflow")}</p>}
        </>}
        {blueprints.length > 0 && <label className="block space-y-1"><span>{t("blueprints.from")}</span>
          <select value={blueprintID} onChange={(e) => setBlueprintID(e.target.value)} className="block w-full rounded border bg-background p-2">
            <option value="">{t("blueprints.none")}</option>
            {blueprints.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}
          </select>
          {blueprint && <small className="text-muted-foreground">{t("blueprints.hint")}</small>}
        </label>}
      </div>
      <div className="space-y-3">
        {kind === "project" && <ItemUserPicker label={t("items.owner")} selected={owner} onChange={setOwner} />}
        {kind !== "folder" && <ItemDates invalid={fieldError === "due_date"} />}
        {kind !== "task" && <fieldset className="flex gap-3">
          <legend>{t("items.view")}</legend>
          {viewTypes.map((view) => <label key={view} className="flex items-center gap-1 rounded border p-2">
            <input type="radio" name="default_view_type" value={view} defaultChecked={view === "table"} />{t(`items.${view}`)}
          </label>)}
        </fieldset>}
      </div>
    </div>
    {canShare && <details>
      <summary>{t("items.sharing")}</summary>
      <Search me={me} groups={groups.data ?? []}
        taken={new Set(shares.map((share) => `${share.principal_type}:${share.principal_id}:direct`))}
        onAdd={(principal, role) => setShares((current) => [...current, { ...principal, role, inherited_from: null }])} />
      {shares.map((share) => <Button key={`${share.principal_type}:${share.principal_id}`} type="button" variant="outline" size="sm"
        aria-label={t("items.remove", { name: share.principal_name })}
        onClick={() => setShares((current) => current.filter((entry) => entry !== share))}>{share.principal_name} · {t(`share.role.${share.role}`)}</Button>)}
    </details>}
    <FormError message={error} />
    <DialogFooter>
      <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>{t("items.cancel")}</Button>
      <Button type="submit" disabled={busy}>{t("items.create")}</Button>
    </DialogFooter>
  </form>;
}
