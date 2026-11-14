import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { createMenuKey, useCreateMenu, type CreateMenu } from "@/lib/create-menu";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { TypeIcon } from "@/components/type-icon";

/** Which item types the "+" menu of a Space offers (SPC-14). Hiding a type removes no items and does not stop the API. */
export function CreateMenuDialog({ spaceId, open, onOpenChange }: { spaceId: string; open: boolean; onOpenChange: (open: boolean) => void }) {
  const { t } = useTranslation();
  const menu = useCreateMenu(spaceId, open);
  return <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{t("spaces.createMenu.title")}</DialogTitle>
        <DialogDescription>{t("spaces.createMenu.hint")}</DialogDescription>
      </DialogHeader>
      {menu.isError && <p role="alert">{t("spaces.createMenu.loadError")}</p>}
      {menu.data && <Form key={menu.data.version} spaceId={spaceId} menu={menu.data} onDone={() => onOpenChange(false)} />}
    </DialogContent>
  </Dialog>;
}

function Form({ spaceId, menu, onDone }: { spaceId: string; menu: CreateMenu; onDone: () => void }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [on, setOn] = useState(() => new Set(menu.items.filter((entry) => entry.enabled).map((entry) => entry.key)));
  const save = useMutation({
    mutationFn: () => api<CreateMenu>(`/api/v1/spaces/${spaceId}/create-menu`, {
      method: "PUT", headers: { "If-Match": `"${menu.version}"` },
      body: JSON.stringify({ enabled: menu.items.filter((entry) => on.has(entry.key)).map((entry) => entry.key) }),
    }),
    onSuccess: async () => { await qc.invalidateQueries({ queryKey: createMenuKey(spaceId) }); onDone(); },
  });
  const toggle = (key: string) => setOn((current) => { const next = new Set(current); if (!next.delete(key)) next.add(key); return next; });
  return <form className="space-y-3" onSubmit={(event) => { event.preventDefault(); save.mutate(); }}>
    <fieldset className="space-y-2">
      <legend className="sr-only">{t("spaces.createMenu.legend")}</legend>
      {menu.items.map((entry) => <label key={entry.key} className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={on.has(entry.key)} disabled={on.size === 1 && on.has(entry.key)} onChange={() => toggle(entry.key)} />
        <TypeIcon name={entry.icon} />{entry.name}
      </label>)}
    </fieldset>
    {save.error && <p role="alert">{save.error.message}</p>}
    <Button type="submit" disabled={save.isPending}>{t("spaces.createMenu.save")}</Button>
  </form>;
}
