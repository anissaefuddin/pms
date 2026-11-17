import { useState } from "react";
import { useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, ApiError } from "@/lib/api";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";

type Field = { id: string; name: string; type: string; archived_at?: string | null };
type Assignment = { field_id: string; scope_kind: "space" | "project" | "folder"; scope_id: string; scope_name: string };
type Choice = { id: string; name: string; type: string; assigned: boolean };

/**
 * Project or folder: which fields apply to this container and the items inside it (DR-13). A field with no container
 * assignment in the Space applies to the whole Space; once one is set, only the assigned subtrees see it.
 * Only people with the Full role in the Space can change it; the server checks each write.
 */
export function ContainerFieldAssignments({ open, onOpenChange, spaceId, item }: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  spaceId: string;
  item: { id: string; title: string; type: "project" | "folder" };
}) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<string>();
  const fields = useQuery({ queryKey: ["container-fields", spaceId], enabled: open, retry: false,
    queryFn: async () => (await api<{ items: Field[] }>(`/api/v1/spaces/${spaceId}/custom-fields`)).items });
  const choices = useQuery({ queryKey: ["container-choices", spaceId], enabled: open, retry: false,
    queryFn: async () => (await api<{ items: Choice[] }>(`/api/v1/spaces/${spaceId}/field-choices`)).items });
  // A Space field is in its Space already; a Workspace field appears here only once the Space is assigned to it.
  const available = [
    ...(fields.data ?? []).filter((field) => !field.archived_at),
    ...(choices.data ?? []).filter((choice) => choice.assigned).map((choice) => ({ id: choice.id, name: choice.name, type: choice.type })),
  ];
  const assignments = useQueries({
    queries: available.map((field) => ({
      queryKey: ["field-assignments", spaceId, field.id], enabled: open,
      queryFn: async () => (await api<{ items: Assignment[] }>(`/api/v1/spaces/${spaceId}/custom-fields/${field.id}/assignments`)).items,
    })),
  });

  async function toggle(fieldId: string, here: boolean) {
    setBusy(fieldId);
    setError("");
    try {
      const base = `/api/v1/spaces/${spaceId}/custom-fields/${fieldId}/assignments`;
      if (here) await api(`${base}/${item.type}/${item.id}`, { method: "DELETE" });
      else await api(base, { method: "POST", body: JSON.stringify({ scope_kind: item.type, scope_id: item.id }) });
      await client.invalidateQueries({ queryKey: ["field-assignments", spaceId, fieldId] });
    } catch (err) {
      setError(err instanceof ApiError && err.status === 403 ? t("items.fieldAssignments.forbidden") : (err as Error).message);
    } finally {
      setBusy(undefined);
    }
  }

  return <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-xl">
      <div className="space-y-1.5 pr-8">
        <DialogTitle>{t("items.fieldAssignments.title", { name: item.title })}</DialogTitle>
        <DialogDescription>{t(`items.fieldAssignments.help.${item.type}`)}</DialogDescription>
      </div>
      {fields.isPending && <p role="status" className="text-sm text-muted-foreground">{t("items.fieldAssignments.loading")}</p>}
      {(fields.isError || choices.isError) && <p role="alert" className="text-sm">{t("items.fieldAssignments.forbidden")}</p>}
      {fields.isSuccess && choices.isSuccess && available.length === 0 && <p className="text-sm text-muted-foreground">{t("items.fieldAssignments.empty")}</p>}
      <ul className="divide-y rounded-md border">
        {available.map((field, index) => {
          const list = assignments[index]?.data ?? [];
          const here = list.some((a) => a.scope_kind === item.type && a.scope_id === item.id);
          const restricted = list.some((a) => a.scope_kind !== "space");
          const status = here ? t("items.fieldAssignments.here")
            : restricted ? t("items.fieldAssignments.elsewhere") : t("items.fieldAssignments.wholeSpace");
          return <li key={field.id} className="flex items-center justify-between gap-3 px-3 py-2">
            <label className="flex min-w-0 items-center gap-2 text-sm">
              <input type="checkbox" checked={here} disabled={busy === field.id || !assignments[index]?.data}
                onChange={() => void toggle(field.id, here)} />
              <span className="min-w-0 truncate">{field.name}</span>
            </label>
            <span className="shrink-0 text-xs text-muted-foreground">{status}</span>
          </li>;
        })}
      </ul>
      <FormError message={error} />
      <div className="flex justify-end">
        <Button variant="outline" onClick={() => onOpenChange(false)}>{t("items.cancel")}</Button>
      </div>
    </DialogContent>
  </Dialog>;
}
