import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import type { CustomField } from "../src/lib/custom-fields";
import { chip } from "./view-helpers";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const tableId = "55555555-5555-4555-8555-555555555555";
const boardId = "66666666-6666-4666-8666-666666666666";
const totalId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const path = `/spaces/${marketing.id}`;
const todo = { id: "11111111-1111-4111-8111-111111111111", name: "To do", group: "active", position: 0 };
const done = { id: "22222222-2222-4222-8222-222222222222", name: "Completed", group: "completed", position: 1 };
const bugType = { id: "99999999-9999-4999-8999-999999999999", key: "bug", name: "Bug", icon: "bug" };
const taskType = { id: "98989898-9898-4989-8989-989898989898", key: "task", name: "Task", icon: "circle" };
const total: CustomField = { id: totalId, space_id: marketing.id, type: "formula", name: "Total", position: 0, options: [], config: { expression: "[Price]*2" } } as CustomField;
const base = { type: "task", version: 1, my_capabilities: ALL, locations: [{ space_id: marketing.id }], assignees: [], status: todo };
const crash = { ...base, id: "77777777-7777-4777-8777-777777777771", seq: 1, title: "Crash", item_type: bugType, fields: { [totalId]: 1234.5 } };
const plain = { ...base, id: "77777777-7777-4777-8777-777777777772", seq: 2, title: "Plain", item_type: taskType, fields: {} };

function mock(page: import("@playwright/test").Page, columns: string[]) {
  const views = [
    { id: tableId, type: "table", name: "Table", visibility: "public", position: 0, is_default: true, config: { columns } },
    { id: boardId, type: "board", name: "Board", visibility: "public", position: 1, is_default: false, config: { columns } },
  ];
  return mockSpaces(page, { spaces: [marketing], extra: {
    [`GET ${path}/views`]: { body: { items: views } },
    [`GET ${path}/items`]: (_body, req) => ({ body: { items: [crash, plain].filter((i) => !req.query.get("type") || i.item_type.key === req.query.get("type")) } }),
    [`GET /spaces/${marketing.id}/custom-fields`]: { body: { items: [total] } },
    [`GET /spaces/${marketing.id}/fields`]: { body: { items: [total] } },
    "GET /item-types": { body: { items: [taskType, bugType] } },
    "GET /workflows": { body: { items: [{ id: "wf", name: "Default", is_default: true, statuses: [todo, done] }] } },
    [`PATCH /items/${crash.id}`]: () => ({ status: 422, body: { type: "about:blank", title: "Unprocessable Entity", status: 422, code: "transition_blocked",
      detail: "blocked", request_id: "r", errors: [{ field: "due_date", message: "this status needs a due date" }, { field: "assignees", message: "this status needs an assignee" }] } }),
  } });
}

test("Table shows type and formula columns, filters by type through the URL, formula is read-only and sortable", async ({ page }) => {
  const { calls } = await mock(page, ["type", `field:${totalId}`]);
  await page.goto(path);
  const grid = page.getByRole("table", { name: "Table" });
  await expect(grid.getByRole("columnheader", { name: "Type" })).toBeVisible();
  await expect(grid.locator('[data-item-type="bug"]')).toHaveText("Bug");
  await expect(grid.getByText("1,234.5")).toBeVisible();
  await expect(grid.getByText("No result")).toBeVisible(); // Plain has no stored result
  await expect(grid.getByRole("button", { name: /Total:/ })).toHaveCount(0);
  await chip(page, "Sort");
  await page.getByRole("combobox", { name: "Sort" }).selectOption(`field:${totalId}`);
  await expect.poll(() => calls.some((c) => c.path === `${path}/items` && c.query.get("sort") === `field:${totalId}`)).toBe(true);
  await chip(page, "Filter");
  await page.getByRole("combobox", { name: "Filter by type" }).selectOption("bug");
  await expect(page).toHaveURL(/type=bug/);
  await expect.poll(() => calls.some((c) => c.path === `${path}/items` && c.query.get("type") === "bug")).toBe(true);
  await expect(grid.getByText("Plain")).toHaveCount(0);
  await page.reload();
  await chip(page, "Filter");
  await expect(page.getByRole("combobox", { name: "Filter by type" })).toHaveValue("bug");
  expect(await new AxeBuilder({ page }).analyze().then((r) => r.violations)).toEqual([]);
});

test("Board card shows the type and a blocked move shows every problem and keeps the card in place", async ({ page }) => {
  await mock(page, ["type"]);
  await page.goto(`${path}?view=${boardId}`);
  const card = page.getByRole("listitem").filter({ hasText: "Crash" });
  await expect(card.locator('[data-item-type="bug"]')).toHaveText("Bug");
  await card.getByRole("button", { name: /Move to status/ }).click();
  await page.getByRole("menuitem", { name: "Completed", exact: true }).click();
  const alert = page.getByRole("alert").filter({ hasText: "this status needs a due date" });
  await expect(alert).toContainText("this status needs an assignee");
  await expect(alert).toContainText("Crash was not moved");
  await expect(page.getByRole("region", { name: "To do" }).getByText("Crash")).toBeVisible();
  await expect(card.getByRole("button", { name: "Move to status: Crash" })).toBeVisible();
  expect(await new AxeBuilder({ page }).include('[role="alert"]').analyze().then((r) => r.violations)).toEqual([]);
  await alert.getByRole("button", { name: "Dismiss" }).click();
  await expect(alert).toHaveCount(0);
});

test("global search results show the item type", async ({ page }) => {
  await mockSpaces(page, { extra: { "GET /search": { body: {
    items: [{ ...crash, title: "Crash report" }], comments: [], users: [],
  } } } });
  await page.goto("/search?q=crash&scope=everywhere");
  await expect(page.getByRole("link", { name: /Crash report/ }).locator('[data-item-type="bug"]')).toHaveText("Bug");
});
