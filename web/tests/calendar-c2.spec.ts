import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// C2: overflow lists every hidden event, and a bar shows its details on hover or keyboard focus.
const calId = "77777777-7777-4777-8777-777777777772";
const path = `/spaces/${marketing.id}`;
const now = new Date();
const ym = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
const status = { id: "88888888-8888-4888-8888-888888888888", name: "In Progress", color: "blue", group: "active" };
const done = { id: "99999999-9999-4999-8999-999999999999", name: "Done", color: "green", group: "completed" };
const base = { version: 1, my_capabilities: ALL, assignees: [{ id: "a1", name: "Ann Lee" }], locations: [{ space_id: marketing.id }], type: "task", status };
const titles = ["Design", "Build", "Test", "Review"];
const items = titles.map((title, index) => ({ ...base, id: `d000000${index}-0000-4000-8000-000000000001`, seq: index + 1, title, start_date: `${ym}-10`, due_date: `${ym}-11` }));

test.beforeEach(async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET ${path}/views`]: { body: { items: [{ id: calId, type: "calendar", name: "Calendar", visibility: "public", position: 0, is_default: true, config: {} }] } },
    [`GET ${path}/items`]: { body: { items } },
    "GET /workflows": { body: { items: [{ is_default: true, statuses: [status, done] }] } },
    "GET /settings/work-schedule": { body: { working_days: [1, 2, 3, 4, 5], daily_hours: 8, version: 1, exceptions: [] } },
  } });
});

test("more events than lanes: +N opens an anchored list with the hidden event", async ({ page }) => {
  await page.setViewportSize({ width: 1400, height: 1200 });
  await page.goto(path);
  // Three lanes are drawn; the fourth event is hidden on each day it spans and is listed under "+1 more".
  await page.getByRole("button", { name: "+1 more" }).first().click();
  // Order is deterministic (start, length, id), so the fourth item by id is the hidden one.
  await expect(page.getByRole("list").getByText("Review", { exact: true })).toBeVisible();
});

test("a bar shows its details on keyboard focus, and the bar is described by that card", async ({ page }) => {
  await page.setViewportSize({ width: 1400, height: 1200 });
  await page.goto(path);
  const bar = page.getByRole("grid", { name: "Calendar" }).getByRole("button", { name: "Design", exact: true });
  await bar.focus();
  const card = page.getByRole("tooltip").filter({ hasText: "Design" });
  await expect(card).toBeVisible();
  await expect(card).toContainText("Status: In Progress");
  await expect(card).toContainText("Assignees: Ann Lee");
  await expect(bar).toHaveAttribute("aria-describedby", /.+/);
});

test("E2E-06: calendar labels never expose a raw translation key", async ({ page }) => {
  await page.setViewportSize({ width: 1400, height: 1200 });
  await page.goto(path);
  await expect(page.getByRole("grid", { name: "Calendar" }).getByRole("button", { name: /Design/ }).first()).toBeVisible();
  await expect(page.locator("[aria-label*='views.calendar']")).toHaveCount(0);
});

test("a calendar bar menu exposes direct item actions", async ({ page }) => {
  await page.goto(path);
  await page.getByRole("button", { name: "Item actions: Design" }).click();
  await expect(page.getByRole("menuitem", { name: "Full details" })).toBeVisible();
  await expect(page.getByRole("menuitem", { name: "Copy link" })).toBeVisible();
  await page.getByRole("menuitem", { name: "Status" }).hover();
  await expect(page.getByRole("menuitem", { name: "Done" })).toBeVisible();
  await expect(page.getByRole("menuitem", { name: "Delete item" })).toBeVisible();
});
