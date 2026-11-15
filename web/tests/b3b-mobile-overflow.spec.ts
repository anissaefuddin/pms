import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { ALL, grace, marketing, mockSpaces } from "./spaces-mock";

// Batch 3 UI audit (390 px): no sideways page scroll on /settings/audit, /settings/apps and the approvals buttons.
test.use({ viewport: { width: 390, height: 844 } });
const hscroll = (page: any) => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
const event = (id: number) => ({ id, at: "2026-11-14T03:00:00Z", actor_id: me.id, actor_name: "ada.lovelace.long.address@example.test", action: "share.space_changed", target_type: "space", target_id: "s1", ip: "2001:db8:85a3::8a2e:370:7334", details: { reason: "x".repeat(80) } });

test("/settings/audit table scrolls inside its region", async ({ page }) => {
  await mockSpaces(page, { extra: { "GET /audit-events": { body: { items: [event(1), event(2)], next_cursor: null } } } });
  await page.goto("/settings/audit");
  await expect(page.getByRole("row").nth(1)).toBeVisible();
  expect(await hscroll(page)).toBeLessThanOrEqual(0);
});
test("/settings/apps tabs wrap instead of widening the page", async ({ page }) => {
  await mockSpaces(page, { me, extra: { "GET /api-keys": { body: { items: [] } } } });
  await page.goto("/settings/apps");
  await expect(page.getByRole("tab", { name: "API" })).toBeVisible();
  expect(await hscroll(page)).toBeLessThanOrEqual(0);
});
test("approval Edit/Reassign/Delete buttons wrap on a phone", async ({ page }) => {
  const id = "b0000000-0000-4000-8000-000000000001", aid = "d0000000-0000-4000-8000-000000000001";
  const ada = { id: me.id, name: "Ada Lovelace" };
  const approval = { id: aid, item_id: id, description: "Sign off", policy: "all", due_date: null, created_by: ada, version: 1, created_at: "2026-11-01T10:00:00Z", status: "pending", approved_count: 0, total: 1, approvers: [{ user: ada, decision: null, comment: "", decided_at: null }] };
  await mockSpaces(page, { extra: {
    [`GET /items/${id}`]: { body: { id, seq: 1, type: "task", title: "Plan", version: 1, my_capabilities: ALL, assignees: [], locations: [{ space_id: marketing.id, space_name: "Marketing" }] }, headers: { ETag: '"1"' } },
    [`GET /items/${id}/children`]: { body: { items: [] } }, [`GET /items/${id}/comments`]: { body: { items: [] } }, [`GET /items/${id}/attachments`]: { body: { items: [] } },
    "GET /users": { body: { items: [grace] } }, [`GET /items/${id}/approvals`]: { body: { items: [approval] } }, [`GET /items/${id}/approval-events`]: { body: { items: [] } },
  } });
  await page.goto(`/tasks/${id}`);
  await expect(page.getByRole("button", { name: "Delete" })).toBeVisible();
  expect(await hscroll(page)).toBeLessThanOrEqual(0);
});
