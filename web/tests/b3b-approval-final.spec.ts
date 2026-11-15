import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { ALL, grace, marketing, mockSpaces } from "./spaces-mock";

// QA-D-laneD-003: once an approval is final the decision buttons must not stay active.
const id = "b0000000-0000-4000-8000-000000000001";
const aid = "d0000000-0000-4000-8000-000000000001";
const ada = { id: me.id, name: "Ada Lovelace" };
const graceRef = { id: grace.id, name: "Grace Hopper" };
const row = (user: typeof ada, decision: string | null) => ({ user, decision, comment: "", decided_at: decision ? "2026-11-02T10:00:00Z" : null });
const make = (status: string, policy: string, approvers: any[]) => ({ id: aid, item_id: id, description: "Sign off", policy, due_date: null, created_by: graceRef, version: 1, created_at: "2026-11-01T10:00:00Z",
  status, approved_count: approvers.filter((a) => a.decision === "approved").length, total: approvers.length, approvers });

async function open(page: any, approval: any) {
  await mockSpaces(page, { extra: {
    [`GET /items/${id}`]: { body: { id, seq: 1, type: "task", title: "Plan", version: 1, my_capabilities: ALL, assignees: [], locations: [{ space_id: marketing.id, space_name: "Marketing" }] }, headers: { ETag: '"1"' } },
    [`GET /items/${id}/children`]: { body: { items: [] } }, [`GET /items/${id}/comments`]: { body: { items: [] } }, [`GET /items/${id}/attachments`]: { body: { items: [] } },
    "GET /users": { body: { items: [grace] } },
    [`GET /items/${id}/approvals`]: { body: { items: [approval] } }, [`GET /items/${id}/approval-events`]: { body: { items: [] } },
  } });
  await page.goto(`/tasks/${id}`);
  await expect(page.getByRole("region", { name: "Approvals" })).toBeVisible();
}
const buttons = (page: any) => page.getByRole("region", { name: "Approvals" }).getByRole("button", { name: /^(Approve|Reject)$/ });

test("final approval (Any policy, I voted): no Approve/Reject, my decision shown", async ({ page }) => {
  await open(page, make("approved", "any", [row(ada, "approved"), row(graceRef, null)]));
  await expect(page.getByRole("region", { name: "Approvals" }).getByText(/Your decision/)).toBeVisible();
  await expect(buttons(page)).toHaveCount(0);
});
test("final approval (Any policy): an approver who has not voted cannot vote either", async ({ page }) => {
  await open(page, make("approved", "any", [row(ada, null), row(graceRef, "approved")]));
  await expect(buttons(page)).toHaveCount(0);
});
test("rejected (final) hides the buttons; a pending approval keeps them", async ({ page }) => {
  await open(page, make("rejected", "all", [row(ada, "approved"), row(graceRef, "rejected")]));
  await expect(buttons(page)).toHaveCount(0);
});
test("pending approval keeps Approve and Reject for an approver who has not voted", async ({ page }) => {
  await open(page, make("pending", "all", [row(ada, null), row(graceRef, "approved")]));
  await expect(buttons(page)).toHaveCount(2);
});
