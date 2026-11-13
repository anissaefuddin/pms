import { expect, test } from "@playwright/test";
import { ALL, grace, marketing, mockSpaces } from "./spaces-mock";

const id = "b0000000-0000-4000-8000-000000000003";
const file = "f0000000-0000-4000-8000-000000000003";
const aid = "d0000000-0000-4000-8000-000000000003";
const ann = { id: "a0000000-0000-4000-8000-000000000001", name: "Ann Lee" };

test("requesting a file review pins the current version", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /items/${id}`]: { body: { id, seq: 1, type: "task", title: "Logo", version: 1, my_capabilities: ALL, assignees: [], locations: [{ space_id: marketing.id, space_name: "Marketing" }] }, headers: { ETag: '"1"' } },
    [`GET /items/${id}/children`]: { body: { items: [] } },
    [`GET /items/${id}/comments`]: { body: { items: [] } },
    [`GET /items/${id}/approvals`]: { body: { items: [] } },
    [`GET /items/${id}/approval-events`]: { body: { items: [] } },
    [`GET /items/${id}/attachments`]: { body: { items: [{ id: file, item_id: id, filename: "logo.png", content_type: "image/png", size_bytes: 9, version: 2, uploader: ann, created_at: "2026-11-14T10:00:00Z" }], max_upload_bytes: 1048576 } },
    "GET /users": { body: { items: [grace] } },
    [`POST /items/${id}/approvals`]: { status: 201, body: {} },
  } });
  await page.goto(`/tasks/${id}`);
  const section = page.locator("section[aria-labelledby]", { has: page.getByRole("heading", { name: "Approvals" }) });
  await section.getByRole("button", { name: "Add approval" }).click();
  await section.getByLabel("File to review (optional)").selectOption(file);
  await section.getByRole("searchbox", { name: /Approvers/ }).fill("gr");
  await section.getByRole("button", { name: "Grace Hopper" }).click();
  await section.getByRole("button", { name: "Add approval" }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path.endsWith("/approvals"))?.body)
    .toMatchObject({ approver_ids: [grace.id], attachment_id: file, attachment_version: 2 });
});

test("reviewer decides on a file version without item access", async ({ page }) => {
  let decision: string | null = null;
  const { calls } = await mockSpaces(page, { extra: {
    "GET /file-reviews": () => ({ body: { items: [{ approval_id: aid, attachment_id: file, version: 1, filename: "logo.png", size_bytes: 9, description: "Check colours", due_date: null, decision, comment: "", requested_by: "Ann Lee" }] } }),
    [`PUT /approvals/${aid}/decision`]: () => { decision = "approved"; return { body: {} }; },
  } });
  await page.goto("/reviews");
  await expect(page.getByRole("link", { name: /logo\.png/ })).toHaveAttribute("href", `/api/v1/attachments/${file}/versions/1/download`);
  await page.getByRole("button", { name: "Approve" }).click();
  await expect.poll(() => calls.some((c) => c.method === "PUT" && c.path.endsWith(`/approvals/${aid}/decision`))).toBe(true);
  await expect(page.getByText("Your decision: Approved")).toBeVisible();
});
