// Status review per versi file (WA-D08). Murni: tanpa React, tanpa mengarang status dari data yang tidak ada.
// Review terikat ke attachment_id + attachment_version. Versi baru tidak mewarisi keputusan versi lama.

export type ReviewState = "loading" | "unavailable" | "notRequested" | "inReview" | "approved" | "rejected";

export type ApprovalLike = {
  id: string;
  status: "pending" | "approved" | "rejected";
  attachment_id?: string | null;
  attachment_version?: number | null;
  created_at: string;
};

/**
 * Status review untuk satu versi.
 * - loading / unavailable: daftar approval belum ada atau gagal dimuat. Ini tidak sama dengan "belum diminta".
 * - notRequested: tidak ada approval yang terikat ke versi ini.
 * - inReview: ada approval pending untuk versi ini (yang terbaru).
 * - approved / rejected: tidak ada yang pending; diambil dari keputusan terbaru untuk versi ini.
 * Approved tidak pernah disimpulkan dari tidak adanya review pending.
 */
export function reviewStateFor(
  approvals: ApprovalLike[] | undefined,
  attachmentId: string,
  version: number,
  load: "loading" | "error" | "ok",
): { state: ReviewState; approvalId?: string } {
  if (load === "error") return { state: "unavailable" };
  if (load === "loading" || !approvals) return { state: "loading" };
  const matching = approvals
    .filter((approval) => approval.attachment_id === attachmentId && approval.attachment_version === version)
    .sort((a, b) => b.created_at.localeCompare(a.created_at));
  if (!matching.length) return { state: "notRequested" };
  const pending = matching.find((approval) => approval.status === "pending");
  if (pending) return { state: "inReview", approvalId: pending.id };
  const latest = matching[0];
  return latest.status === "approved" ? { state: "approved", approvalId: latest.id } : { state: "rejected", approvalId: latest.id };
}
