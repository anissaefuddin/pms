import { AxeBuilder } from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { marketing, mockSpaces } from "./spaces-mock";

// A2: the registry shows each rule's state, owner, trigger and history, and its lifecycle actions post to the right endpoints.
const sid = (n: number) => `88888888-8888-4888-8888-${String(n).padStart(12, "0")}`;
const ids = { live: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", paused: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaab1", archived: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaab2" };
const base = { space_id: marketing.id, container_id: null, owner_id: "u", owner_name: "Grace Hopper", container_title: null, trigger: "item.created", action_count: 2, has_draft: false, error_count: 0, last_run_at: null, last_run_status: null, current_version_id: sid(9), created_at: "2026-11-13T10:00:00Z" };
const rules = [
  { ...base, id: ids.live, name: "Celebrate", enabled: true, health: "ok", health_reason: "", archived_at: null, last_run_at: "2026-11-13T11:00:00Z", last_run_status: "done" },
  { ...base, id: ids.paused, name: "Nightly sweep", enabled: true, health: "paused", health_reason: "owner_no_access", archived_at: null, error_count: 2, last_run_status: "denied", last_run_at: "2026-11-13T12:00:00Z", has_draft: true },
  { ...base, id: ids.archived, name: "Old rule", enabled: false, health: "ok", health_reason: "", archived_at: "2026-11-01T09:00:00Z", current_version_id: null },
];

async function setup(page: Page, over: Record<string, unknown> = {}) {
  const { calls } = await mockSpaces(page, { extra: {
    "GET /workflows": { body: { items: [] } },
    "GET /item-types": { body: { items: [] } },
    [`GET /spaces/${marketing.id}/forms`]: { body: { items: [] } },
    [`GET /spaces/${marketing.id}/automation-rules`]: { body: { items: rules } },
    "GET /automation/dead-events": { body: { items: [{ event_id: "ffffffff-ffff-4fff-8fff-ffffffffffff", type: "item.created", item_id: null, attempts: 8, last_error: "database is busy", created_at: "2026-11-13T09:00:00Z" }] } },
    [`POST /automation/events/ffffffff-ffff-4fff-8fff-ffffffffffff/replay`]: { body: { requeued: true, reopened_runs: 1 } },
    [`POST /automation-rules/${ids.live}/duplicate`]: { status: 201, body: { ...rules[0], id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", name: "Copy of Celebrate", enabled: false, current_version_id: null } },
    [`POST /automation-rules/${ids.archived}/restore`]: { body: { ...rules[2], archived_at: null } },
    [`POST /automation-rules/${ids.live}/archive`]: { body: { ...rules[0], archived_at: "2026-11-13T13:00:00Z", enabled: false } },
    ...over,
  } as never });
  return calls;
}

test("the registry shows state, owner, trigger and history; the filter narrows the list", async ({ page }) => {
  await setup(page);
  await page.goto(`/spaces/${marketing.id}/automation`);
  const row = (name: string) => page.getByRole("row").filter({ hasText: name });
  await expect(row("Nightly sweep")).toContainText("Paused: owner cannot edit this Space");
  await expect(row("Nightly sweep")).toContainText("Unpublished changes");
  await expect(row("Nightly sweep")).toContainText("Grace Hopper");
  await expect(row("Celebrate")).toContainText("Whole Space");
  await expect(row("Old rule")).toContainText("Archived");
  await page.getByLabel("Show").selectOption("attention");
  await expect(page.getByRole("cell", { name: "Nightly sweep", exact: true })).toBeVisible();
  await expect(page.getByRole("cell", { name: "Celebrate", exact: true })).toHaveCount(0);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("duplicate, archive and restore post to their endpoints", async ({ page }) => {
  const calls = await setup(page);
  await page.goto(`/spaces/${marketing.id}/automation`);
  await page.getByRole("button", { name: "Duplicate Celebrate" }).click();
  await expect.poll(() => calls.some((c) => c.method === "POST" && c.path === `/automation-rules/${ids.live}/duplicate`)).toBe(true);
  await page.getByRole("button", { name: "Archive Celebrate" }).click();
  await page.getByRole("dialog", { name: "Archive this rule?" }).getByRole("button", { name: "Archive" }).click();
  await expect.poll(() => calls.some((c) => c.method === "POST" && c.path === `/automation-rules/${ids.live}/archive`)).toBe(true);
  await page.getByRole("button", { name: "Restore Old rule" }).click();
  await expect.poll(() => calls.some((c) => c.method === "POST" && c.path === `/automation-rules/${ids.archived}/restore`)).toBe(true);
});

test("an administrator sees failed events and can replay one", async ({ page }) => {
  const calls = await setup(page);
  await page.goto(`/spaces/${marketing.id}/automation`);
  await expect(page.getByRole("cell", { name: "database is busy" })).toBeVisible();
  await page.getByRole("button", { name: /Replay event ffffffff/ }).click();
  await expect.poll(() => calls.some((c) => c.method === "POST" && c.path === "/automation/events/ffffffff-ffff-4fff-8fff-ffffffffffff/replay")).toBe(true);
});

// A3 / AUT-17: a template fills the builder, and closing a changed builder asks before it discards the work.
const template = { id: "thank-approver", name: "Thank the approver", description: "Comment on the item when it is approved.", definition: { when: { type: "approval.decided", decision: "approved" }, if: [], then: [{ type: "comment", text: "Approved. Thank you." }] } };

test("a template fills the builder and closing a changed builder asks before discarding", async ({ page }) => {
  await setup(page, { "GET /automation/templates": { body: { items: [template] } } });
  await page.goto(`/spaces/${marketing.id}/automation`);
  await page.getByRole("button", { name: "New rule" }).click();
  const dialog = page.getByRole("dialog", { name: "New automation rule" });
  await dialog.getByLabel("Start from a template").selectOption("thank-approver");
  await expect(dialog.getByLabel("Rule name")).toHaveValue("Thank the approver");
  await dialog.getByRole("button", { name: "When", exact: true }).click();
  await expect(dialog.getByLabel("Decision")).toHaveValue("approved");
  await dialog.getByRole("button", { name: "Apply to", exact: true }).click();
  await dialog.getByLabel("Rule name").fill("Thanks");
  await dialog.getByRole("button", { name: "Cancel" }).click();
  const discard = page.getByRole("dialog", { name: "Discard changes?" });
  await expect(discard).toBeVisible();
  await discard.getByRole("button", { name: "Keep editing" }).click();
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await page.getByRole("dialog", { name: "Discard changes?" }).getByRole("button", { name: "Discard" }).click();
  await expect(dialog).toHaveCount(0);
});
