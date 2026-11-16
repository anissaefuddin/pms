import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// WA-D08: review status belongs to the file version. Version 1 was approved; version 2 is new and has no review yet.
const id = "b0000000-0000-4000-8000-000000000003";
const file = "f0000000-0000-4000-8000-000000000003";
const item = { id, seq: 1, type: "task", title: "Spec", version: 1, my_capabilities: ALL, assignees: [], locations: [{ space_id: marketing.id, space_name: "Marketing" }] };
const ann = { id: "a0000000-0000-4000-8000-000000000001", name: "Ann Lee" };
const versions = [
  { attachment_id: file, version: 2, filename: "spec-final.txt", content_type: "text/plain", size_bytes: 18, is_current: true, uploader: ann, created_at: "2026-11-14T11:00:00Z" },
  { attachment_id: file, version: 1, filename: "spec.txt", content_type: "text/plain", size_bytes: 9, is_current: false, uploader: ann, created_at: "2026-11-14T10:00:00Z" },
];
const approval = { id: "p0000000-0000-4000-8000-000000000001", item_id: id, description: "", due_date: null, created_by: ann, version: 1, created_at: "2026-11-14T12:00:00Z",
  status: "approved", policy: "any", attachment_id: file, attachment_version: 1, approved_count: 1, total: 1, approvers: [] };

test("each version shows its own review status; a new version is not inheriting the approval", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET /items/${id}`]: { body: item, headers: { ETag: '"1"' } },
    [`GET /items/${id}/children`]: { body: { items: [] } },
    [`GET /items/${id}/comments`]: { body: { items: [] } },
    [`GET /items/${id}/approvals`]: { body: { items: [approval] } },
    [`GET /items/${id}/attachments`]: { body: { items: [{ ...versions[0], id: file, item_id: id }], max_upload_bytes: 1048576 } },
    [`GET /attachments/${file}/versions`]: { body: { items: versions } },
  } });
  await page.goto(`/tasks/${id}`);
  await page.getByRole("button", { name: "v2" }).click();
  const history = page.getByRole("list", { name: "Version history" });
  await expect(history.locator('[data-review-state="notRequested"]')).toHaveCount(1); // version 2
  await expect(history.locator('[data-review-state="approved"]')).toHaveCount(1); // version 1 keeps its own approval
  await expect(history.getByText("Not requested")).toBeVisible();
  await expect(history.getByText("Approved", { exact: true })).toBeVisible();
});
