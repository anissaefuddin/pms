import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

/** Product feedback (NAV-09): subject, optional rating and comment; only the current app path is sent along. */
export function FeedbackDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { t } = useTranslation();
  const [sent, setSent] = useState(false);
  const send = useMutation({
    mutationFn: (body: object) => api("/api/v1/feedback", { method: "POST", body: JSON.stringify(body) }),
    onSuccess: () => setSent(true),
  });
  return <Dialog open={open} onOpenChange={(next) => { if (!next) { setSent(false); send.reset(); } onOpenChange(next); }}>
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{t("nav.feedback.title")}</DialogTitle>
        <DialogDescription>{t("nav.feedback.hint")}</DialogDescription>
      </DialogHeader>
      {sent ? <p role="status">{t("nav.feedback.thanks")}</p> : <form className="space-y-3" onSubmit={(event) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        const rating = Number(data.get("rating"));
        send.mutate({ subject: String(data.get("subject")), rating: rating || null, comment: String(data.get("comment")), page: location.pathname });
      }}>
        {send.error && <p role="alert">{send.error.message}</p>}
        <label className="block text-sm">{t("nav.feedback.subject")}<Input name="subject" required maxLength={200} /></label>
        <label className="block text-sm">{t("nav.feedback.rating")}
          <select name="rating" defaultValue="" className="block rounded border bg-background p-2">
            <option value="">{t("nav.feedback.noRating")}</option>
            {[1, 2, 3, 4, 5].map((n) => <option key={n} value={n}>{n}</option>)}
          </select></label>
        <label className="block text-sm">{t("nav.feedback.comment")}<textarea name="comment" maxLength={4000} rows={4} className="block w-full rounded border bg-background p-2" /></label>
        <Button type="submit" disabled={send.isPending}>{t("nav.feedback.send")}</Button>
      </form>}
    </DialogContent>
  </Dialog>;
}
