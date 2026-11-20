import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { TopbarTitle } from "@/components/topbar-title";

export const Route = createFileRoute("/_app/reviews")({ component: FileReviews });

type Review = components["schemas"]["FileReview"];

/** File approvals addressed to me (FIL-04). An external reviewer sees only the pinned version, never the item. */
function FileReviews() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [error, setError] = useState("");
  const { data, isError } = useQuery({ queryKey: ["file-reviews"], queryFn: () => api<{ items: Review[] }>("/api/v1/file-reviews") });
  async function decide(id: string, decision: "approved" | "rejected", comment: string) {
    setError("");
    try {
      await api(`/api/v1/approvals/${id}/decision`, { method: "PUT", body: JSON.stringify({ decision, comment }) });
      await qc.invalidateQueries({ queryKey: ["file-reviews"] });
    } catch (err) { setError((err as Error).message); }
  }
  return <main className="mx-auto max-w-3xl space-y-4 p-6">
    <TopbarTitle>{t("items.fileReview.title")}</TopbarTitle>
    {isError && <FormError message={t("items.loadError")} />}
    <FormError message={error} />
    {data?.items.length === 0 && <p className="text-muted-foreground">{t("items.fileReview.empty")}</p>}
    <ul className="space-y-3">{data?.items.map((r) => <ReviewCard key={r.approval_id} review={r} onDecide={decide} />)}</ul>
  </main>;
}

function ReviewCard({ review, onDecide }: { review: Review; onDecide: (id: string, d: "approved" | "rejected", comment: string) => void }) {
  const { t } = useTranslation();
  const [comment, setComment] = useState(review.comment);
  return <li className="space-y-2 rounded border p-3">
    <a className="font-medium underline" href={`/api/v1/attachments/${review.attachment_id}/versions/${review.version}/download`} download>
      {review.filename} · {t("items.versionLabel", { version: review.version })}
    </a>
    <p className="text-sm text-muted-foreground">{t("items.approvals.createdBy", { name: review.requested_by })}{review.description && ` — ${review.description}`}</p>
    {review.decision && <p className="text-sm font-medium">{t("items.approvals.yourDecision", { decision: t(`items.approvals.status.${review.decision}`) })}</p>}
    <label className="block space-y-1 text-sm">{t("items.approvals.comment")}
      <Input value={comment} onChange={(event) => setComment(event.target.value)} /></label>
    <div className="flex gap-2">
      <Button size="sm" onClick={() => onDecide(review.approval_id, "approved", comment)}>{t("items.approvals.approve")}</Button>
      <Button size="sm" variant="outline" onClick={() => onDecide(review.approval_id, "rejected", comment)}>{t("items.approvals.reject")}</Button>
    </div>
  </li>;
}
