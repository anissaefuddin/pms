import { useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { blueprintKey } from "@/lib/blueprints";
import { listItems, type Item } from "@/lib/items";
import type { Space } from "@/lib/spaces";
import { type SuggestedBlueprint } from "@/lib/workspace-metrics";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "@/components/ui/dialog";

export function CreateBlueprintDialog({ initial, spaces, defaultSpaceId, onClose }: {
  initial: { name: string; type: SuggestedBlueprint["itemType"] };
  spaces: Space[];
  defaultSpaceId: string;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [spaceId, setSpaceId] = useState(defaultSpaceId);
  const [type, setType] = useState(initial.type);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const sources = useQuery({
    queryKey: ["blueprint-sources", spaceId], enabled: !!spaceId,
    queryFn: () => listItems(`/api/v1/spaces/${spaceId}/items`),
  });
  const candidates = (sources.data ?? []).filter((item: Item) => item.type === type);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const name = String(form.get("name") ?? "").trim();
    const source = String(form.get("source") ?? "");
    if (!name || !source) return;
    setBusy(true); setError("");
    try {
      await api(`/api/v1/spaces/${spaceId}/blueprints`, { method: "POST", body: JSON.stringify({ name, source_item_id: source }) });
      await qc.invalidateQueries({ queryKey: blueprintKey(spaceId) });
      toast.success(t("blueprints.saved"));
      onClose();
    } catch (err) {
      setError((err as Error).message || t("blueprints.saveError"));
    } finally {
      setBusy(false);
    }
  }

  return <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
    <DialogContent>
      <DialogTitle>{t("blueprints.createTitle")}</DialogTitle>
      <form className="space-y-3" onSubmit={(event) => void submit(event)}>
        <label className="block space-y-1"><span>{t("blueprints.name")}</span>
          <Input name="name" defaultValue={initial.name} maxLength={120} required autoFocus />
        </label>
        <label className="block space-y-1"><span>{t("blueprints.space")}</span>
          <select value={spaceId} onChange={(event) => setSpaceId(event.target.value)} className="block w-full rounded border bg-background p-2">
            {spaces.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
          </select>
        </label>
        <label className="block space-y-1"><span>{t("blueprints.kind")}</span>
          <select value={type} onChange={(event) => setType(event.target.value as SuggestedBlueprint["itemType"])} className="block w-full rounded border bg-background p-2">
            <option value="project">{t("blueprints.type.project")}</option>
            <option value="folder">{t("blueprints.type.folder")}</option>
          </select>
        </label>
        <label className="block space-y-1"><span>{t("blueprints.source")}</span>
          <select name="source" required defaultValue="" className="block w-full rounded border bg-background p-2">
            <option value="" disabled>{candidates.length ? t("blueprints.pickSource") : t("blueprints.noSource")}</option>
            {candidates.map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}
          </select>
        </label>
        <FormError message={error} />
        <DialogFooter><Button type="submit" disabled={busy || !candidates.length}>{t("blueprints.save")}</Button></DialogFooter>
      </form>
    </DialogContent>
  </Dialog>;
}
