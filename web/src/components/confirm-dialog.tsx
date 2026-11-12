import { useRef, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: ReactNode;
  confirmLabel: string;
  cancelLabel: string;
  onConfirm: () => void;
  busy?: boolean;
  error?: string;
};

/** Destructive confirmation: initial focus lands on Cancel (DESIGN_SYSTEM ConfirmDialog). */
export function ConfirmDialog({ open, onOpenChange, title, description, confirmLabel, cancelLabel, onConfirm, busy, error }: Props) {
  const { t } = useTranslation();
  const cancel = useRef<HTMLButtonElement>(null);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent initialFocus={cancel}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <FormError message={error ?? ""} />
        <DialogFooter>
          <Button ref={cancel} variant="outline" onClick={() => onOpenChange(false)}>{cancelLabel}</Button>
          <Button variant="destructive" disabled={busy} onClick={onConfirm}>{busy ? t("common.feedback.loading") : confirmLabel}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
