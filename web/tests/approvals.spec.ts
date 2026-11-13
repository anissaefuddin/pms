import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { me } from "./mock-api";
import { ALL, grace, marketing, mockSpaces } from "./spaces-mock";

const id = "b0000000-0000-4000-8000-000000000001";
const aid = "d0000000-0000-4000-8000-000000000001";
const ada = { id: me.id, name: "Ada Lovelace" };
const graceRef = { id: grace.id, name: "Grace Hopper" };
const other = { id: "99999999-9999-4999-8999-999999999999", name: "Linus T" };

function approval(version: number, approvers: { user: typeof ada; decision: string | null; comment: string }[], description = "Sign off the plan", policy = "all") {
  const count = approvers.filter((a) => a.decision === "approved").length;
  const status = approvers.some((a) => a.decision === "rejected") ? "rejected" : count === approvers.length ? "approved" : "pending";
  return { id: aid, item_id: id, description, policy, due_date: "2026-12-01", created_by: other, version, created_at: "2026-11-01T10:00:00Z",
    status, approved_count: count, total: approvers.length, approvers: approvers.map((a) => ({ ...a, decided_at: a.decision ? "2026-11-02T10:00:00Z" : null })) };
}

async function mock(page: Page, caps: string[], seed = true) {
  let current: any = seed ? approval(1, [{ user: ada, decision: null, comment: "" }, { user: graceRef, decision: null, comment: "" }]) : undefined;
  const item = { id, seq: 1, type: "task", title: "Plan", version: 1, my_capabilities: caps, assignees: [], locations: [{ space_id: marketing.id, space_name: "Marketing" }] };
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /items/${id}`]: { body: item, headers: { ETag: '"1"' } },
    [`GET /items/${id}/children`]: { body: { items: [] } },
    [`GET /items/${id}/comments`]: { body: { items: [] } },
    [`GET /items/${id}/attachments`]: { body: { items: [] } },
    "GET /users": { body: { items: [grace] } },
    [`GET /items/${id}/approvals`]: () => ({ body: { items: current ? [current] : [] } }),
    [`GET /items/${id}/approval-events`]: { body: { items: [
      { id: 2, approval_id: aid, kind: "decided", actor: graceRef, subject: graceRef, decision: "rejected", comment: "Needs work", item_version: 7, created_at: "2026-03-02T10:00:00Z" },
      { id: 1, approval_id: aid, kind: "approver_requested", actor: ada, subject: graceRef, decision: null, comment: "", item_version: 6, created_at: "2026-03-01T10:00:00Z" },
    ] } },
    [`POST /items/${id}/approvals`]: (body) => {
      current = approval(1, body.approver_ids.map((uid: string) => ({ user: uid === grace.id ? graceRef : ada, decision: null, comment: "" })), body.description, body.policy);
      return { status: 201, body: current, headers: { ETag: '"1"' } };
    },
    [`PUT /approvals/${aid}/decision`]: (body) => {
      current = approval(current.version, current.approvers.map((a: any) => a.user.id === me.id ? { user: ada, decision: body.decision, comment: body.comment } : a));
      return { body: current };
    },
    [`PATCH /approvals/${aid}`]: (body) => {
      current = approval(current.version + 1, current.approvers, body.description);
      return { body: current, headers: { ETag: `"${current.version}"` } };
    },
    [`DELETE /approvals/${aid}`]: () => { current = undefined; return { status: 204 }; },
  } });
  return calls;
}

const panel = (page: Page) => page.getByRole("region", { name: "Approvals" });

test("an editor creates an approval with an approver", async ({ page }) => {
  const calls = await mock(page, ALL, false);
  await page.goto(`/tasks/${id}`);
  const section = panel(page);
  await section.getByRole("button", { name: "Add approval" }).click();
  await expect(section.getByRole("button", { name: "Add approval" })).toBeDisabled(); // no approver yet
  await section.getByLabel("Description").fill("Launch copy");
  await section.getByLabel("Approval policy").selectOption("any");
  await section.getByRole("searchbox", { name: /Approvers/ }).fill("gr");
  await section.getByRole("button", { name: "Grace Hopper" }).click();
  await section.getByRole("button", { name: "Add approval" }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path.endsWith("/approvals"))?.body)
    .toEqual({ policy: "any", description: "Launch copy", due_date: null, approver_ids: [grace.id] });
  await expect(section.getByText("Launch copy")).toBeVisible();
  await expect(section.getByText("0/1 approved · Pending")).toBeVisible();
});

test("a named approver approves and the counter moves from 0/2 to 1/2", async ({ page }) => {
  const calls = await mock(page, ALL);
  await page.goto(`/tasks/${id}`);
  const section = panel(page);
  await expect(section.getByText("0/2 approved · Pending")).toBeVisible();
  await section.getByLabel("Comment (optional)").fill("Looks good");
  await section.getByRole("button", { name: "Approve", exact: true }).click();
  await expect(section.getByText("1/2 approved · Pending")).toBeVisible();
  await expect(section.getByText("Your decision: Approved")).toBeVisible();
  await expect(section.getByText("Looks good").first()).toBeVisible();
  expect(calls.find((c) => c.method === "PUT")?.body).toEqual({ decision: "approved", comment: "Looks good" });
  await section.getByRole("button", { name: "Reject", exact: true }).click(); // a decision can be changed
  await expect(section.getByText("0/2 approved · Rejected")).toBeVisible();
});

test("edit sends If-Match with the approval version; delete asks for confirmation", async ({ page }) => {
  const calls = await mock(page, ALL);
  await page.goto(`/tasks/${id}`);
  const section = panel(page);
  await section.getByRole("button", { name: "Edit approval" }).click();
  await section.getByLabel("Description").fill("Revised");
  await section.getByRole("button", { name: "Save approval" }).click();
  await expect(section.getByText("Revised")).toBeVisible();
  const patch = calls.find((c) => c.method === "PATCH");
  expect(patch?.headers["if-match"]).toBe('"1"');
  expect(patch?.body.description).toBe("Revised");
  await section.getByRole("button", { name: "Delete approval" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete approval" }).click();
  await expect(section.getByText("No approvals yet.")).toBeVisible();
  expect(calls.some((c) => c.method === "DELETE")).toBe(true);
});

test("without edit capability there is no Add, Edit or Delete", async ({ page }) => {
  await mock(page, ["view", "comment"]);
  await page.goto(`/tasks/${id}`);
  const section = panel(page);
  await expect(section.getByText("0/2 approved · Pending")).toBeVisible();
  await expect(section.getByRole("button", { name: /Add approval|Edit approval|Delete approval/ })).toHaveCount(0);
  await expect(section.getByRole("button", { name: "Approve", exact: true })).toBeVisible(); // approver, not editor
});

for (const scheme of ["light", "dark"] as const) {
  test(`approvals panel has no axe violations (${scheme})`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: scheme });
    await mock(page, ALL);
    await page.goto(`/tasks/${id}`);
    await expect(panel(page).getByText("0/2 approved · Pending")).toBeVisible();
    const result = await new AxeBuilder({ page }).include('section[aria-labelledby]').analyze();
    expect(result.violations).toEqual([]);
  });
}

test("the decision history lists who decided what, pinned to the item version", async ({ page }) => {
  await mock(page, ALL);
  await page.goto(`/tasks/${id}`);
  const section = panel(page);
  await section.getByText("Decision history").click();
  const log = section.getByRole("list", { name: "Decision history" });
  await expect(log.getByRole("listitem")).toHaveCount(2);
  await expect(log.getByRole("listitem").first()).toContainText("Needs work");
  await expect(log.getByRole("listitem").first()).toContainText("item version 7");
});

test("reassign uses the current version and sends only the reviewer list", async ({ page }) => {
  const calls = await mock(page, ALL);
  await page.goto(`/tasks/${id}`);
  const section = panel(page);
  await section.getByRole("button", { name: "Reassign approvers" }).click();
  await expect(section.getByLabel("Approval policy")).toHaveCount(0);
  await section.getByRole("button", { name: "Save approval" }).click();
  await expect.poll(() => calls.find((c) => c.method === "PATCH")?.body).toEqual({ approver_ids: [me.id, grace.id] });
  expect(calls.find((c) => c.method === "PATCH")?.headers["if-match"]).toBe('"1"');
});
