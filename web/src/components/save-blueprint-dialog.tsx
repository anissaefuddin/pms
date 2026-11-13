import { useState, type FormEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import { blueprintKey } from "@/lib/blueprints";
import { useSpaces } from "@/lib/space-queries";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "@/components/ui/dialog";

type Props = { open: boolean; onOpenChange: (open: boolean) => void; itemID: string; title: string };

export function SaveBlueprintDialog({ open, onOpenChange, itemID, title }: Props) {
  const { t } = useTranslation();
  return <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent>
      <DialogTitle>{t("blueprints.saveTitle")}</DialogTitle>
      {open && <SaveForm itemID={itemID} title={title} onOpenChange={onOpenChange} />}
    </DialogContent>
  </Dialog>;
}

function SaveForm({ itemID, title, onOpenChange }: Omit<Props, "open">) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  // Only shared Spaces where the user can edit may hold blueprints.
  const spaces = (useSpaces().data ?? []).filter((space) => space.kind === "shared" && space.my_capabilities?.includes("edit"));
  const [spaceID, setSpaceID] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const target = spaceID || spaces[0]?.id || "";

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy || !target) return;
    setBusy(true);
    setError("");
    try {
      await api(`/api/v1/spaces/${target}/blueprints`, {
        method: "POST",
        body: JSON.stringify({ name: String(new FormData(event.currentTarget).get("name") ?? "").trim(), source_item_id: itemID }),
      });
      await queryClient.invalidateQueries({ queryKey: blueprintKey(target) });
      toast.success(t("blueprints.saved"));
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("blueprints.saveError"));
    } finally {
      setBusy(false);
    }
  }

  return <form noValidate onSubmit={submit} className="space-y-4">
    <p className="text-sm text-muted-foreground">{t("blueprints.saveHint")}</p>
    <label className="block space-y-1"><span>{t("blueprints.name")}</span>
      <Input name="name" maxLength={120} defaultValue={title} required /></label>
    <label className="block space-y-1"><span>{t("items.space")}</span>
      <select value={target} onChange={(e) => setSpaceID(e.target.value)} className="block w-full rounded border bg-background p-2">
        {spaces.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
      </select>
    </label>
    {!spaces.length && <p role="status">{t("blueprints.noSpace")}</p>}
    <FormError message={error} />
    <DialogFooter>
      <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>{t("items.cancel")}</Button>
      <Button type="submit" disabled={busy || !target}>{t("blueprints.save")}</Button>
    </DialogFooter>
  </form>;
}
