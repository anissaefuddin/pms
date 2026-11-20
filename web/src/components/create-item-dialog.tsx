import { useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Folder, FolderKanban, Plus, SquareCheck } from "lucide-react";
import { ApiError, api, type CurrentUser } from "@/lib/api";
import { useSpaces } from "@/lib/space-queries";
import { getItem, itemKeys, listItems, type Item, type ItemCreate, type Location } from "@/lib/items";
import type { Group, Share } from "@/lib/spaces";
import { Search } from "@/components/share-dialog";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { AvatarStack } from "@/components/ui/avatar-stack";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { ItemUserPicker, type PersonOption } from "@/components/item-user-picker";
import { ViewTypePicker } from "@/components/view-type-picker";
import { useItemTypes, type ItemTypeDef } from "@/lib/item-types";
import { useBlueprints } from "@/lib/blueprints";
import { menuAllows, useCreateMenu } from "@/lib/create-menu";
import { TypeIcon } from "@/components/type-icon";
import { ItemDates } from "@/components/item-dates";
import { payloadDates, validateCreate, type CreateKind, type CreateIssue, type SupportedDefaultView } from "@/lib/create-item-validate";
import { newItemId } from "@/lib/create-item-validate";
import { createItemOnce } from "@/lib/create-item-service";
import { cn } from "@/lib/utils";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  me: CurrentUser;
  location?: Location;
  taskOnly?: boolean;
  initialStatus?: string;
  /** Tipe awal saat dibuka dari menu global create; diabaikan bila taskOnly atau ada parent. */
  initialKind?: CreateKind;
  /** A Quick Access or "More item types" choice; the dialog still checks it against the chosen destination. */
  initialTypeKey?: string;
  /** YYYY-MM-DD: prefills start and finish (a one-day Task from a Calendar date). */
  initialDate?: string;
  onCreated: (item: Item) => void;
};
const kinds: CreateKind[] = ["project", "folder", "task"];
const createKinds: CreateKind[] = ["project", "folder", "task"];
const kindIcons = { project: FolderKanban, folder: Folder, task: SquareCheck } as const;
const cardClass = cn(
  "flex cursor-pointer flex-col gap-1 rounded-lg border bg-card p-3 text-left text-sm transition-colors hover:border-ring/60",
  "has-checked:border-primary has-checked:ring-3 has-checked:ring-ring/30",
  "has-focus-visible:ring-3 has-focus-visible:ring-ring/50",
  "has-disabled:cursor-not-allowed has-disabled:opacity-60 has-disabled:hover:border-border"
);
const selectClass = "block w-full rounded border bg-background p-2";

export function CreateItemDialog(props: Props) {
  const { t } = useTranslation();
  return <Dialog open={props.open} onOpenChange={props.onOpenChange}>
    <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-3xl">
      <DialogTitle>{t(props.location?.parent_item_id ? "items.addChild" : "items.create")}</DialogTitle>
      {props.open && <CreateForm {...props} />}
    </DialogContent>
  </Dialog>;
}

type FieldKey = "title" | "location" | "parent" | "due_date";

function CreateForm({ me, location, onCreated, onOpenChange, taskOnly, initialStatus, initialKind, initialTypeKey, initialDate }: Props) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const spaces = useSpaces();
  const fixedParent = location?.parent_item_id;
  // WA-D04: only an explicit taskOnly locks the kind. A fixed parent offers every kind its parent allows; the server enforces the rule.
  const lockedKind: CreateKind | undefined = taskOnly ? "task" : undefined;
  const [chosenKind, setKind] = useState<CreateKind>(lockedKind ?? initialKind ?? "project");
  const [spaceID, setSpaceID] = useState(location?.space_id ?? me.personal_space_id);
  // One client id per open form: a retry of this form reuses it (see createItemOnce).
  const [itemId] = useState(newItemId);
  const [parentID, setParentID] = useState(fixedParent ?? "");
  const [trail, setTrail] = useState<Item[]>([]);
  const [owner, setOwner] = useState<PersonOption[]>([{ id: me.id, name: `${me.first_name} ${me.last_name}`.trim() }]);
  const [shares, setShares] = useState<Share[]>([]);
  const [sharingOpen, setSharingOpen] = useState(false);
  const [view, setView] = useState<SupportedDefaultView>("table");
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Partial<Record<FieldKey, string>>>({});
  const [busy, setBusy] = useState(false);
  const [blueprintID, setBlueprintID] = useState("");
  const [typeKeyChoice, setTypeKey] = useState(initialTypeKey ?? "");
  // CIT-D01: Workspace types, and the types of this Space only (a Space type never appears in another Space)
  const typeDefs = (useItemTypes().data ?? []).filter((def) => !def.space_id || def.space_id === spaceID);
  // SPC-14: the Space's "+" menu narrows what is offered; a fixed parent or the board's task-only add is not a menu.
  const menu = useCreateMenu(spaceID, !fixedParent && !taskOnly).data;
  const offered = (def: ItemTypeDef) => fixedParent || taskOnly || menuAllows(menu, def.key);
  const customOf = (base: CreateKind) => typeDefs.filter((def) => !def.is_builtin && def.active && def.base === base && offered(def));
  const kinds = createKinds.filter((base) => menuAllows(menu, base) || customOf(base).length > 0);
  const kind: CreateKind = lockedKind || kinds.includes(chosenKind) ? chosenKind : kinds[0] ?? chosenKind;
  const choices = spaces.data?.filter((space) => space.my_capabilities?.includes("create")) ?? [];
  const selected = choices.find((space) => space.id === spaceID);
  const directory = trail.at(-1);
  const parents = useQuery({
    queryKey: directory ? itemKeys.children(directory.id) : itemKeys.root(spaceID),
    enabled: !!selected && !fixedParent,
    queryFn: () => listItems(directory ? `/api/v1/items/${directory.id}/children` : `/api/v1/spaces/${spaceID}/items`),
  });
  const parent = parentID === directory?.id ? directory : parents.data?.find((item) => item.id === parentID);
  // allowed_children: a parent type that lists child types accepts only those custom types below it (ITM-S8.1.3)
  const parentChildren = typeDefs.find((def) => def.id === parent?.item_type?.id)?.allowed_children ?? [];
  const customTypes = customOf(kind).filter((def) => !parentChildren.length || parentChildren.includes(def.id));
  const baseHidden = !fixedParent && !taskOnly && !menuAllows(menu, kind);
  const typeKey = customTypes.some((def) => def.key === typeKeyChoice) ? typeKeyChoice : baseHidden ? customTypes[0]?.key ?? "" : "";
  // Task boleh di bawah Project atau Task; folder dan project boleh di bawah item non-task (validateCreate dan server menegakkan sisanya).
  const parentOptions = parents.data?.filter((item) => (kind === "task" ? item.type === "project" || item.type === "task" : item.type !== "task")
    && item.my_capabilities.includes("create")
    && item.locations.some((loc) => loc.space_id === spaceID && (loc.parent_item_id ?? "") === (directory?.id ?? "")));
  const canBrowse = !!parent && parent.id !== directory?.id;
  const canShare = kind !== "task" && selected?.kind === "shared" && (parent ?? selected).my_capabilities?.includes("share");
  const groups = useQuery({
    queryKey: ["groups"], enabled: !!canShare,
    queryFn: async () => (await api<{ items: Group[] }>("/api/v1/groups")).items,
  });

  const blueprints = useBlueprints(spaceID, !!selected).data ?? [];
  const blueprint = blueprints.find((entry) => entry.id === blueprintID);
  // F3: with a fixed parent the list above is not loaded, so read that parent's own title (it is the location the item will be created in).
  const fixedParentItem = useQuery({ queryKey: itemKeys.detail(fixedParent ?? ""), enabled: !!fixedParent, queryFn: () => getItem(fixedParent!) });
  const parentLabel = parent?.title ?? (fixedParent ? fixedParentItem.data?.item.title : undefined) ?? directory?.title ?? t("items.root");

  function fail(issue: CreateIssue) {
    setFieldErrors({ [issue.field]: t(`items.${issue.code}`) });
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const form = new FormData(event.currentTarget);
    const title = String(form.get("title") ?? "").trim();
    const start = String(form.get("start_date") ?? "");
    const due = String(form.get("due_date") ?? "");
    setFieldErrors({});
    setError("");
    const issue = validateCreate({
      kind, title, start, due,
      spaceId: selected || fixedParent ? spaceID : "",
      parent: parent ? { type: parent.type as CreateKind, spaceId: spaceID } : undefined,
    });
    if (issue) return fail(issue);
    const dates = payloadDates(kind, start, due);
    const input: ItemCreate = { type: kind, title, locations: [{ space_id: spaceID, parent_item_id: parentID || null }], ...dates };
    if (initialStatus) input.status_id = initialStatus;
    if (typeKey && customTypes.some((def) => def.key === typeKey)) input.type_key = typeKey; // CUST-01: a custom type of this kind
    if (kind !== "task") input.default_view_type = view;
    if (kind === "project" && owner[0]) input.owner_id = owner[0].id;
    if (kind === "task") { // SET-03: the user's defaults for a new task
      if (me.default_task_duration_days != null) input.duration_days = me.default_task_duration_days;
      if (me.default_task_effort_minutes != null) input.effort_minutes = me.default_task_effort_minutes;
    }
    if (canShare && shares.length) input.shares = shares.map(({ principal_type, principal_id, role }) => ({ principal_type, principal_id, role }));
    if (blueprint && start && due) { // one anchor only: the server refuses start_date together with finish_date
      setError(t("blueprints.anchorOne"));
      return;
    }
    setBusy(true);
    try {
      const item = blueprint
        ? await api<Item>(`/api/v1/blueprints/${blueprint.id}/items`, {
          method: "POST", body: JSON.stringify({ locations: input.locations, title, ...(start ? { start_date: start } : due ? { finish_date: due } : {}) }),
        })
        : await createItemOnce(queryClient, { ...input, id: itemId }, me);
      await queryClient.invalidateQueries({ queryKey: itemKeys.all, predicate: ({ queryKey }) => queryKey[1] !== "detail" });
      toast.success(<span>{t("items.created", { name: item.title })}{" "}
        <Link to="/tasks/$itemId" params={{ itemId: item.id }} className="underline">{t("items.openCreated")}</Link></span>);
      onOpenChange(false);
      onCreated(item);
    } catch (err) {
      if (err instanceof ApiError) {
        const problem = err.fields.find(({ field }) => field === "title" || field === "due_date" || field === "locations");
        if (problem) {
          setFieldErrors({ [problem.field === "locations" ? "location" : problem.field]: problem.message });
          return;
        }
      }
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const summary = [
    t(`items.${kind}`),
    selected?.name ?? t("items.chooseSpace"),
    parentLabel,
    kind !== "task" ? t(`views.types.${view}`) : null,
  ].filter(Boolean).join(" · ");

  return <form noValidate onSubmit={submit} className="space-y-6">
    <section className="space-y-3" aria-labelledby="create-general">
      <h3 id="create-general" className="text-sm font-semibold">{t("items.general")}</h3>
      <div className="grid gap-3 sm:grid-cols-2">
        <label className="block space-y-1 sm:col-span-2"><span>{t("items.name")}</span>
          <Input name="title" maxLength={500} required autoFocus aria-invalid={!!fieldErrors.title}
            aria-describedby={fieldErrors.title ? "create-title-error" : undefined} />
        </label>
        {fieldErrors.title && <p id="create-title-error" role="alert" className="text-sm text-destructive sm:col-span-2">{fieldErrors.title}</p>}
        {!fixedParent && <>
          <label className="block space-y-1"><span>{t("items.space")}</span>
            <select value={selected?.id ?? ""} aria-invalid={!!fieldErrors.location} className={selectClass}
              onChange={(e) => { setSpaceID(e.target.value); setParentID(""); setTrail([]); setShares([]); setBlueprintID(""); }}>
              <option value="">{t("items.chooseSpace")}</option>
              {choices.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
            </select>
          </label>
          <div className="space-y-1">
            <label className="block space-y-1"><span>{t("items.parent")}</span>
              <select value={parentID} aria-invalid={!!fieldErrors.parent} aria-describedby={fieldErrors.parent ? "create-parent-error" : undefined} className={selectClass}
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
          </div>
          {fieldErrors.location && <p role="alert" className="text-sm text-destructive sm:col-span-2">{fieldErrors.location}</p>}
          {fieldErrors.parent && <p id="create-parent-error" role="alert" className="text-sm text-destructive sm:col-span-2">{fieldErrors.parent}</p>}
          {!blueprint && (customTypes.length > 1 || (customTypes.length > 0 && !baseHidden)) && <label className="block space-y-1"><span>{t("items.type.label")}</span>
            <select value={typeKey} onChange={(event) => setTypeKey(event.target.value)} className={selectClass}>
              {!baseHidden && <option value="">{t(`items.${kind}`)}</option>}
              {customTypes.map((def) => <option key={def.key} value={def.key}>{def.name}</option>)}
            </select></label>}
          {kind !== "task" && <p className="self-end text-sm text-muted-foreground">{t("items.workflow")}: {t("items.defaultWorkflow")}</p>}
        </>}
        {blueprints.length > 0 && <fieldset className="space-y-1 sm:col-span-2">
          <legend>{t("blueprints.from")}</legend>
          <label className="flex items-center gap-2"><input type="radio" name="blueprint" checked={!blueprint} onChange={() => setBlueprintID("")} />{t("blueprints.none")}</label>
          {blueprints.map((entry) => {
            const def = typeDefs.find((d) => d.key === entry.item_type);
            return <label key={entry.id} className="flex items-center gap-2">
              <input type="radio" name="blueprint" checked={entry.id === blueprintID} onChange={() => setBlueprintID(entry.id)} />
              <TypeIcon name={def?.icon} />{entry.name}
              <small className="text-muted-foreground">{t("blueprints.typeLine", { type: def?.name ?? entry.item_type, count: entry.item_count })}</small>
            </label>;
          })}
          {blueprint && <small className="text-muted-foreground">{t("blueprints.hint")}</small>}
        </fieldset>}
      </div>
    </section>

    <section className="space-y-3" aria-labelledby="create-type">
      <h3 id="create-type" className="text-sm font-semibold">{t("items.typeLegend")}</h3>
      <fieldset disabled={!!lockedKind || !!blueprint} className="grid gap-2 sm:grid-cols-3">
        <legend className="sr-only">{t("items.typeLegend")}</legend>
        {kinds.map((type) => {
          const Icon = kindIcons[type];
          return <label key={type} className={cardClass}>
            <span className="flex items-center gap-2 font-medium text-foreground">
              <Icon className="size-4" aria-hidden="true" />{t(`items.${type}`)}
              {/* F4: disabled on the input too, so the card's has-disabled styling applies and the locked type reads as not selectable. */}
              <input type="radio" name="kind" className="sr-only" checked={kind === type} disabled={!!lockedKind || !!blueprint}
                onChange={() => { setKind(type); setTypeKey(""); }} />
            </span>
            <span className="text-xs text-muted-foreground">{t(`items.typeDesc.${type}`)}</span>
          </label>;
        })}
      </fieldset>
    </section>

    <section className="space-y-4" aria-labelledby="create-settings">
      <h3 id="create-settings" className="text-sm font-semibold">{t("items.settings")}</h3>
      <div className="grid gap-4 sm:grid-cols-2">
        {kind === "project" && <ItemUserPicker label={t("items.owner")} selected={owner} onChange={setOwner} />}
        {kind !== "folder" && <div className="space-y-1 sm:col-span-2">
          <ItemDates initialStart={initialDate} initialDue={initialDate} invalid={!!fieldErrors.due_date} />
          {fieldErrors.due_date && <p id="create-due-error" role="alert" className="text-sm text-destructive">{fieldErrors.due_date}</p>}
        </div>}
        {kind === "folder" && <p className="text-sm text-muted-foreground sm:col-span-2">{t("items.noDates")}</p>}
      </div>
      {kind !== "task" && <div className="space-y-2">
        <p className="text-sm font-medium">{t("items.view")}</p>
        <ViewTypePicker value={view} onChange={setView} />
      </div>}
      {canShare && <div className="space-y-2">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-sm font-medium">{t("items.sharing")}</p>
          <div className="flex items-center gap-2">
            {shares.length > 0 && <AvatarStack people={shares.map((share) => ({ id: `${share.principal_type}:${share.principal_id}`, name: share.principal_name }))} />}
            <Button type="button" variant="outline" size="sm" aria-expanded={sharingOpen} onClick={() => setSharingOpen(!sharingOpen)}>
              <Plus />{t("items.addUser")}
            </Button>
          </div>
        </div>
        {sharingOpen && <Search me={me} groups={groups.data ?? []}
          taken={new Set(shares.map((share) => `${share.principal_type}:${share.principal_id}:direct`))}
          onAdd={(principal, role) => setShares((current) => [...current, { ...principal, role, inherited_from: null }])} />}
        <div className="flex flex-wrap gap-1">
          {shares.map((share) => <Button key={`${share.principal_type}:${share.principal_id}`} type="button" variant="outline" size="sm"
            aria-label={t("items.remove", { name: share.principal_name })}
            onClick={() => setShares((current) => current.filter((entry) => entry !== share))}>{share.principal_name} · {t(`share.role.${share.role}`)}</Button>)}
        </div>
      </div>}
    </section>

    <FormError message={error} />
    <DialogFooter className="items-center sm:justify-between">
      <p className="min-w-0 truncate text-sm text-muted-foreground" title={summary} aria-live="polite">{summary}</p>
      <div className="flex shrink-0 gap-2">
        <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>{t("items.cancel")}</Button>
        <Button type="submit" disabled={busy}>{t("items.create")}</Button>
      </div>
    </DialogFooter>
  </form>;
}
