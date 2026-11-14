import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { ALL, marketing, mockSpaces } from "./spaces-mock";
import { bar, design, ganttId, kids, milestone, mockGantt, path as ganttPath, project } from "./gantt-mock";

const path = `/spaces/${marketing.id}`;
const tableId = "55555555-5555-4555-8555-555555555555";
const boardId = "66666666-6666-4666-8666-666666666666";
const calId = "77777777-7777-4777-8777-777777777771";
const todo = { id: "88888888-8888-4888-8888-888888888888", name: "To do", group: "active", color: "amber" };
const ann = { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", name: "Ann Lee" };
const card = { id: "77777777-7777-4777-8777-777777777777", seq: 1, type: "task", title: "Plan", version: 1, status: todo, my_capabilities: ALL,
  locations: [{ space_id: marketing.id }], assignees: [ann], child_count: 2, start_date: "2026-11-02", due_date: "2026-11-04",
  item_type: { id: "t1", key: "task", name: "Task", icon: "check-square" } };
const views = [
  { id: tableId, type: "table", name: "Table", visibility: "public", position: 0, is_default: true },
  { id: boardId, type: "board", name: "Board", visibility: "public", position: 1, is_default: false },
  { id: calId, type: "calendar", name: "Calendar", visibility: "public", position: 2, is_default: false },
];
const mock = (page: import("@playwright/test").Page) => mockSpaces(page, { extra: {
  [`GET ${path}/views`]: { body: { items: views } },
  [`GET ${path}/items`]: { body: { items: [card] } },
  "GET /workflows": { body: { items: [{ is_default: true, statuses: [todo] }] } },
} });
const serious = (r: { violations: { impact?: string | null }[] }) => r.violations.filter((v) => v.impact === "serious" || v.impact === "critical");

test("Board card: type icon beside the title, avatars and dates on one row, subitem count in a footer", async ({ page }) => {
  await mock(page);
  await page.goto(`${path}?view=${boardId}`);
  const item = page.getByRole("region", { name: "To do", exact: true }).getByRole("listitem");
  const title = await item.getByRole("button", { name: "Plan", exact: true }).boundingBox();
  const glyph = await item.locator("svg").first().boundingBox();
  expect(Math.abs((glyph!.y + glyph!.height / 2) - (title!.y + title!.height / 2))).toBeLessThan(20); // icon sits on the title row
  const avatars = await item.getByRole("img", { name: "Ann Lee" }).boundingBox();
  const dates = await item.getByText("(3d)").boundingBox();
  expect(Math.abs(avatars!.y - dates!.y)).toBeLessThan(14); // same row
  const count = await item.getByTitle("2 subitems").boundingBox();
  expect(count!.y).toBeGreaterThan(dates!.y + dates!.height); // footer below
  expect(await new AxeBuilder({ page }).analyze().then(serious)).toEqual([]);
});

test("Table: type icon in the name cell, Add item is a full-width row; view tabs carry icons", async ({ page }) => {
  await mock(page);
  await page.goto(path);
  const row = page.getByRole("row", { name: /Plan/ }).first();
  await expect(row.locator("svg").first()).toBeVisible();
  const add = page.getByRole("button", { name: "Add item" });
  const table = await page.getByRole("table", { name: "Table", exact: true }).boundingBox();
  expect((await add.boundingBox())!.width).toBeGreaterThan(table!.width - 4);
  await expect(page.getByRole("navigation", { name: "Views" }).getByRole("button", { name: "Board", exact: true }).locator("svg")).toHaveCount(1);
});

test("Calendar: chips are tinted by status and keep an accessible name", async ({ page }) => {
  await mock(page);
  await page.goto(`${path}?view=${calId}`);
  const chip = page.getByRole("button", { name: /Plan/ }).first();
  await expect(chip).toHaveClass(/bg-amber-100/);
  expect(await new AxeBuilder({ page }).analyze().then(serious)).toEqual([]);
});

test("Gantt: filter chip toolbar, status colour square, and + Add item opens the create dialog", async ({ page }) => {
  await page.setViewportSize({ width: 1800, height: 900 });
  const status = { id: todo.id, name: "To do", color: "amber", group: "active" };
  await mockGantt(page, { root: [{ ...project, status }], children: [...kids, milestone].map((item) => ({ ...item, status })) });
  await page.goto(`${ganttPath}?view=${ganttId}`);
  await expect(bar(page, design)).toBeVisible();
  const toolbar = page.getByRole("toolbar", { name: "View toolbar" });
  await expect(toolbar.getByRole("button", { name: "Filter", exact: true })).toBeVisible();
  await expect(toolbar.getByRole("button", { name: "Columns" })).toBeVisible();
  await expect(page.locator('.wx-table-box [class*="bg-amber-500"]').first()).toBeVisible();
  await page.getByRole("button", { name: "Add item" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
});
