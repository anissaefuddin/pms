import { useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { FormManagePanel } from "@/components/form-manage-panel";
import { AdminPageHeader } from "@/components/admin/admin-ui";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useSpaces } from "@/lib/space-queries";
import { can } from "@/lib/spaces";

export const Route = createFileRoute("/_app/settings/request-forms")({ component: RequestFormsSettings });

// Admin side of request forms: every form you can manage, and the way to start a new one. Requesters use /requests.
function RequestFormsSettings() {
  const { t } = useTranslation();
  return <section className="space-y-5">
    <AdminPageHeader description={t("forms.manage.description")} action={<NewFormButton />} />
    <FormManagePanel />
  </section>;
}

/** A form belongs to one Space, so creating one starts by choosing the Space; the builder opens on that Space. */
function NewFormButton() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const spaces = useSpaces();
  const editable = (spaces.data ?? []).filter((space) => can(space, "edit"));
  const [open, setOpen] = useState(false);
  const [spaceId, setSpaceId] = useState("");
  const chosen = spaceId || editable[0]?.id || "";

  return <>
    <Button onClick={() => setOpen(true)}>{t("forms.manage.createForm")}</Button>
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("forms.manage.createForm")}</DialogTitle>
          <DialogDescription>{t("forms.manage.pickSpaceHint")}</DialogDescription>
        </DialogHeader>
        {editable.length === 0 ? <p role="status" className="text-sm text-muted-foreground">{t("forms.manage.noSpaces")}</p> :
        <label className="block space-y-1 text-sm">{t("forms.manage.pickSpace")}
          <select value={chosen} onChange={(event) => setSpaceId(event.target.value)} className="mt-1 block h-9 w-full rounded-md border border-input bg-background px-2">
            {editable.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
          </select></label>}
        <DialogFooter>
          <Button disabled={!chosen} onClick={() => { setOpen(false); void navigate({ to: "/spaces/$spaceId/forms/$formId", params: { spaceId: chosen, formId: "new" } }); }}>
            {t("forms.manage.createForm")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </>;
}
