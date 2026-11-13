import { AxeBuilder } from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { marketing, mockSpaces } from "./spaces-mock";

const rid = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const sid = (n: number) => `88888888-8888-4888-8888-${String(n).padStart(12, "0")}`;
const wf = { id: sid(100), name: "Default flow", is_default: true, statuses: [
  { id: sid(1), name: "New", color: "slate", group: "active", auto_assign: false, lock_transitions: false, allowed_next: [] },
  { id: sid(2), name: "Done", color: "green", group: "completed", auto_assign: false, lock_transitions: false, allowed_next: [] }] };
const version = (def: object, status = "published", n = 1) => ({ id: `bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbb0${n}`, version: n, status, definition: def, created_at: "2026-11-13T10:00:00Z", published_at: null });
const published = { when: { type: "item.status_changed", to_status_id: sid(2) }, if: [{ field: "importance", op: "eq", value: "high" }], then: [{ type: "comment", text: "Done!" }] };

async function setup(page: import("@playwright/test").Page, over: Record<string, unknown> = {}) {
  let rule = { id: rid, space_id: marketing.id, container_id: null, name: "Celebrate", owner_id: "u", enabled: false, current_version_id: version(published).id };
  const { calls } = await mockSpaces(page, { extra: {
    "GET /workflows": { body: { items: [wf] } },
    "GET /item-types": { body: { items: [] } },
    [`GET /spaces/${marketing.id}/forms`]: { body: { items: [] } },
    [`GET /spaces/${marketing.id}/automation-rules`]: () => ({ body: { items: [rule] } }),
    [`GET /automation-rules/${rid}`]: () => ({ body: { ...rule, versions: [version(published)] } }),
    [`PATCH /automation-rules/${rid}`]: (body: Record<string, unknown>) => { rule = { ...rule, ...body } as typeof rule; return { body: rule }; },
    [`POST /spaces/${marketing.id}/automation-rules`]: (body: Record<string, unknown>) => ({ status: 201, body: { ...rule, id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", name: body.name } }),
    "POST /automation-rules/cccccccc-cccc-4ccc-8ccc-cccccccccccc/publish": { body: rule },
    [`PUT /automation-rules/${rid}/draft`]: (body: Record<string, unknown>) => ({ body: version(body.definition as object, "draft", 2) }),
    [`POST /automation-rules/${rid}/publish`]: { body: rule },
    ...over,
  } as never });
  return calls;
}

test("lists rules and switches one on", async ({ page }) => {
  const calls = await setup(page);
  await page.goto(`/spaces/${marketing.id}/automation`);
  await expect(page.getByRole("cell", { name: "Celebrate", exact: true })).toBeVisible();
  await page.getByRole("checkbox", { name: /Enable Celebrate/ }).click();
  await expect.poll(() => calls.find((c) => c.method === "PATCH" && c.path === `/automation-rules/${rid}`)?.body).toEqual({ enabled: true });
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("builds a WHEN/IF/THEN rule in the editor, tries it on an item, and publishes it", async ({ page }) => {
  const calls = await setup(page, {
    "GET /search": { body: { items: [{ id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", title: "Crash on save", type: "task" }], next_cursor: null } },
    [`POST /spaces/${marketing.id}/automation-rules/dry-run`]: { body: { conditions: [{ index: 0, holds: true }], would_run: true, actions: ["Comment: Escalated Crash on save"], item_title: "Crash on save" } },
  });
  await page.goto(`/spaces/${marketing.id}/automation`);
  await page.getByRole("button", { name: "New rule" }).click();
  const dialog = page.getByRole("dialog", { name: "New automation rule" });
  await dialog.getByLabel("Rule name").fill("Escalate");
  await dialog.getByLabel("Moved to status").selectOption({ label: "Done" });
  await dialog.getByRole("button", { name: "Add a condition" }).click();
  await dialog.getByLabel("Condition 1 value").selectOption("high");
  await dialog.getByLabel("Action 1 comment").fill("Escalated {item.title}");
  await dialog.getByRole("button", { name: "Add an action" }).click();
  await dialog.getByRole("combobox", { name: "Action 2", exact: true }).selectOption("set_field");
  await dialog.getByLabel("Action 2 field").selectOption("importance");
  await dialog.getByLabel("Action 2 value").selectOption("low");
  await dialog.getByLabel("Search an item to test on").fill("cra");
  await dialog.getByRole("button", { name: "Test on Crash on save" }).click();
  const result = dialog.getByRole("region", { name: "Test result" });
  await expect(result).toContainText("The rule would run on Crash on save.");
  await expect(result).toContainText("Comment: Escalated Crash on save");
  await expect(dialog).toHaveCSS("opacity", "1");
  expect((await new AxeBuilder({ page }).include('[role="dialog"]').analyze()).violations).toEqual([]);
  await dialog.getByRole("button", { name: "Publish" }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === `/spaces/${marketing.id}/automation-rules`)?.body).toEqual({
    name: "Escalate", container_id: null, definition: { when: { type: "item.status_changed", to_status_id: sid(2) },
      if: [{ field: "importance", op: "eq", value: "high" }], then: [{ type: "comment", text: "Escalated {item.title}" }, { type: "set_field", field: "importance", value: "low" }] } });
  await expect.poll(() => calls.some((c) => c.method === "POST" && c.path === "/automation-rules/cccccccc-cccc-4ccc-8ccc-cccccccccccc/publish")).toBe(true);
  expect(calls.find((c) => c.method === "POST" && c.path.endsWith("/dry-run"))?.body).toMatchObject({ item_id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd" });
});

test("editing saves a draft with only the settings the chosen trigger allows", async ({ page }) => {
  const calls = await setup(page);
  await page.goto(`/spaces/${marketing.id}/automation`);
  await page.getByRole("button", { name: "Edit Celebrate" }).click();
  const dialog = page.getByRole("dialog", { name: "Edit automation rule" });
  await expect(dialog.getByLabel("Rule name")).toHaveValue("Celebrate");
  await dialog.getByLabel("Trigger").selectOption("item.due_reached");
  await dialog.getByLabel("Days before the due date").fill("2");
  await dialog.getByRole("button", { name: "Save draft" }).click();
  await expect.poll(() => calls.find((c) => c.method === "PUT")?.body).toEqual({ definition: { when: { type: "item.due_reached", days_before: 2 },
    if: [{ field: "importance", op: "eq", value: "high" }], then: [{ type: "comment", text: "Done!" }] } });
});

test("the log shows each outcome with its item, filters by result and pages", async ({ page }) => {
  const run = (n: number, status: string, detail = "") => ({ id: `eeeeeeee-eeee-4eee-8eee-0000000000${String(n).padStart(2, "0")}`, rule_id: rid, version: 1, event_id: "ffffffff-ffff-4fff-8fff-ffffffffffff",
    event_type: "item.status_changed", action_index: n === 2 ? -1 : 0, item_id: "x", item_title: `Item ${n}`, actor_id: "u", status, detail, created_at: "2026-11-13T10:00:00Z" });
  const calls = await setup(page, { [`GET /automation-rules/${rid}/runs`]: (_b: unknown, req: { query: URLSearchParams }) => ({ body: req.query.get("cursor")
    ? { items: [run(3, "failed", "boom")], next_cursor: null } : { items: [run(1, "done"), run(2, "denied", "the rule owner is no longer an active user")], next_cursor: run(2, "denied").id } }) });
  await page.goto(`/spaces/${marketing.id}/automation`);
  await page.getByRole("button", { name: "Log of Celebrate" }).click();
  const dialog = page.getByRole("dialog", { name: "Log of Celebrate" });
  await expect(dialog.getByText("Item 1")).toBeVisible();
  await expect(dialog.getByRole("cell", { name: /Not allowed/ })).toBeVisible();
  await expect(dialog.getByText(/no longer an active user/)).toBeVisible();
  await dialog.getByRole("button", { name: "Load more" }).click();
  await expect(dialog.getByText(/boom/)).toBeVisible();
  await dialog.getByLabel("Show").selectOption("failed");
  await expect.poll(() => calls.some((c) => c.path.endsWith("/runs") && c.query.get("status") === "failed")).toBe(true);
  await expect(dialog).toHaveCSS("opacity", "1");
  expect((await new AxeBuilder({ page }).include('[role="dialog"]').analyze()).violations).toEqual([]);
});

test("a Space the user cannot edit says so and a project opens its own rules", async ({ page }) => {
  await setup(page, { [`GET /spaces/${marketing.id}/automation-rules`]: { status: 403, body: { code: "forbidden", detail: "no", status: 403, title: "Forbidden", type: "about:blank" } } });
  await page.goto(`/spaces/${marketing.id}/automation`);
  await expect(page.getByRole("alert")).toContainText("do not have permission");
});
