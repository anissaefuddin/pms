import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { ALL, marketing, mockSpaces } from "./spaces-mock";
import { bar, design, ganttId, kids, milestone, mockGantt, path as ganttPath, project } from "./gantt-mock";
import { chip } from "./view-helpers";

const path = `/spaces/${marketing.id}`;
const tableId = "55555555-5555-4555-8555-555555555555";
const boardId = "66666666-6666-4666-8666-666666666666";
const todo = { id: "88888888-8888-4888-8888-888888888888", name: "To do", group: "active", color: "amber" };
const done = { id: "99999999-9999-4999-8999-999999999999", name: "Completed", group: "completed", color: "green" };
const ann = { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", name: "Ann Lee" };
const card = { id: "77777777-7777-4777-8777-777777777777", seq: 1, type: "task", title: "Plan", version: 1, status: todo, my_capabilities: ALL,
  locations: [{ space_id: marketing.id }], assignees: [ann], child_count: 2, start_date: "2026-11-02", due_date: "2026-11-04",
  item_type: { id: "t1", key: "task", name: "Task", icon: "check-square" } };
const views = [
  { id: tableId, type: "table", name: "Table", visibility: "public", position: 0, is_default: true },
  { id: boardId, type: "board", name: "Board", visibility: "public", position: 1, is_default: false },
];

function mock(page: import("@playwright/test").Page) {
  let item: any = card;
  const patches: any[] = [];
  return mockSpaces(page, { extra: {
    // The board walks items with subitems (child_count > 0); this card has none in the mock.
    [`GET /items/${card.id}/children`]: { body: { items: [] } },
    [`GET ${path}/views`]: { body: { items: views } },
    [`GET ${path}/items`]: () => ({ body: { items: [item] } }),
    "GET /workflows": { body: { items: [{ is_default: true, statuses: [todo, done] }] } },
    [`PATCH /items/${card.id}`]: (body: any) => { patches.push(body); item = { ...item, version: 2, status: body.status_id === done.id ? done : todo }; return { body: item }; },
  } }).then(() => patches);
}

test("toolbar is a single row of chips with accessible popovers; state stays in the URL", async ({ page }) => {
  await mock(page);
  await page.goto(path);
  const toolbar = page.getByRole("toolbar", { name: "View toolbar" });
  const boxes = await Promise.all([toolbar.getByRole("button", { name: "Filter", exact: true }), toolbar.getByRole("button", { name: "Sort", exact: true }),
    toolbar.getByRole("button", { name: "Group", exact: true }), toolbar.getByRole("textbox", { name: "Filter items" })].map((l) => l.boundingBox()));
  const mids = boxes.map((b) => b!.y + b!.height / 2);
  expect(Math.max(...mids) - Math.min(...mids)).toBeLessThan(8); // one row
  expect((await toolbar.boundingBox())!.height).toBeLessThan(60);
  await chip(page, "Sort");
  expect(await new AxeBuilder({ page }).include('[role="dialog"]').analyze().then((r) => r.violations)).toEqual([]);
  await page.getByLabel("Sort", { exact: true }).selectOption("title");
  await expect(page).toHaveURL(/sort=title/);
  await toolbar.getByRole("textbox", { name: "Filter items" }).fill("Pl");
  await expect(page).toHaveURL(/filter=Pl/);
});

test("view tabs are segmented: options only on the active tab, Add view is a button", async ({ page }) => {
  await mock(page);
  await page.goto(path);
  const nav = page.getByRole("navigation", { name: "Views" });
  await expect(nav.getByRole("button", { name: /options$/ })).toHaveCount(1);
  await expect(nav.getByRole("button", { name: "Table options" })).toBeVisible();
  await nav.getByRole("radio", { name: "Board", exact: true }).click();
  await expect(nav.getByRole("button", { name: "Board options" })).toBeVisible();
  await expect(nav.getByRole("button", { name: /options$/ })).toHaveCount(1);
  await nav.getByRole("button", { name: "Add view" }).click();
  await expect(page.getByRole("dialog", { name: "Choose a view type" }).getByRole("button", { name: /Gantt/ })).toBeEnabled();
  await page.keyboard.press("Escape");
  expect(await new AxeBuilder({ page }).include("nav").analyze().then((r) => r.violations)).toEqual([]);
});

test("Board card shows chip, avatars, duration; status changes by drag and by the card menu", async ({ page }) => {
  const patches = await mock(page);
  await page.goto(`${path}?view=${boardId}`);
  const todoCol = page.getByRole("region", { name: "To do", exact: true });
  const doneCol = page.getByRole("region", { name: "Completed", exact: true });
  await expect(todoCol.locator("[data-status-chip]")).toHaveText("To do");
  await expect(todoCol.getByRole("img", { name: "Ann Lee" })).toBeVisible();
  await expect(todoCol).toContainText("(3d)");
  await expect(todoCol.getByRole("combobox")).toHaveCount(0);
  expect(await new AxeBuilder({ page }).analyze().then((r) => r.violations.filter((v) => v.impact === "serious" || v.impact === "critical"))).toEqual([]);
  await todoCol.getByRole("button", { name: /Move to status/ }).click();
  await page.getByRole("menuitem", { name: "Completed" }).click();
  await expect.poll(() => patches).toEqual([{ status_id: done.id }]);
  await expect(doneCol.getByRole("listitem")).toHaveCount(1);
  await doneCol.getByRole("listitem").dragTo(todoCol);
  await expect.poll(() => patches.at(-1)).toEqual({ status_id: todo.id });
});

test("Table shows the status as a coloured chip and assignees as avatars", async ({ page }) => {
  await mock(page);
  await page.goto(path);
  await expect(page.getByRole("img", { name: "Ann Lee" })).toBeVisible();
  await expect(page.getByRole("combobox", { name: "Status: Plan" })).toHaveClass(/bg-amber-100/);
});

test("Gantt: selectable Status/Start/Due columns, bars coloured by status with assignee label, zoom buttons, axe contrast", async ({ page }) => {
  await page.setViewportSize({ width: 1800, height: 900 });
  const status = { id: todo.id, name: "To do", color: "amber", group: "active" };
  const withStatus = (item: any) => ({ ...item, status, assignees: [ann] });
  await mockGantt(page, { root: [withStatus(project)], children: [...kids, milestone].map(withStatus) });
  await page.goto(`${ganttPath}?view=${ganttId}`);
  await expect(bar(page, design)).toBeVisible();
  await expect(page.locator('.wx-grid [data-id], .wx-table-box').getByText("To do").first()).toBeVisible(); // Status column
  await expect(bar(page, design).locator('[data-bar-status="amber"]')).toContainText("Design • Ann Lee");
  const before = (await bar(page, design).boundingBox())!.width;
  await page.getByRole("button", { name: "Zoom out" }).click();
  await expect.poll(async () => (await bar(page, design).boundingBox())!.width).toBeLessThan(before);
  await page.getByRole("button", { name: "Zoom in" }).click();
  await expect.poll(async () => (await bar(page, design).boundingBox())!.width).toBe(before);
  await expect(page.getByRole("button", { name: "Keyboard help" })).toHaveAttribute("title", /Arrow up and down/);
  await page.getByRole("button", { name: "Columns" }).click();
  const dialog = page.getByRole("dialog", { name: "Choose columns" });
  await expect(dialog.getByLabel("Status", { exact: true })).toBeChecked();
  await expect(dialog.getByLabel("Start date")).toBeChecked();
  expect(await new AxeBuilder({ page }).analyze().then((r) => r.violations.filter((v) => v.impact === "serious" || v.impact === "critical"))).toEqual([]);
});
