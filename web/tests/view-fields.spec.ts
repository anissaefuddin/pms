import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import type { CustomField } from "../src/lib/custom-fields";
import { chip } from "./view-helpers";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const tableId = "55555555-5555-4555-8555-555555555555";
const boardId = "66666666-6666-4666-8666-666666666666";
const costId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const levelId = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const high = { id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", label: "High", color: "#ff0000" };
const low = { id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", label: "Low", color: "#00aa00" };
const cost: CustomField = { id: costId, space_id: marketing.id, type: "number", name: "Cost", position: 0, options: [] };
const level: CustomField = { id: levelId, space_id: marketing.id, type: "single_select", name: "Level", position: 1, options: [high, low] };
const tags: CustomField = { id: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", space_id: marketing.id, type: "multi_select", name: "Tags", position: 2, options: [high] };
const path = `/spaces/${marketing.id}`;
const task = { id: "77777777-7777-4777-8777-777777777777", seq: 1, type: "task", title: "Plan", version: 1,
  my_capabilities: ALL, locations: [{ space_id: marketing.id }], assignees: [], fields: { [costId]: 12, [levelId]: high.id } };

function mock(page: import("@playwright/test").Page, capabilities = ALL, spaceCaps = ALL) {
  let views = [
    { id: tableId, type: "table", name: "Table", visibility: "public", position: 0, is_default: true, config: {} },
    { id: boardId, type: "board", name: "Board", visibility: "public", position: 1, is_default: false, config: { columns: [`field:${costId}`] } },
  ];
  let item: any = { ...task, my_capabilities: capabilities };
  return mockSpaces(page, { spaces: [{ ...marketing, my_capabilities: spaceCaps }], extra: {
    [`GET ${path}/views`]: () => ({ body: { items: views } }),
    [`GET ${path}/items`]: () => ({ body: { items: [item] } }),
    [`GET /spaces/${marketing.id}/custom-fields`]: { body: { items: [cost, level, tags] } },
    "GET /workflows": { body: { items: [] } },
    [`PATCH /views/${tableId}`]: (body) => {
      views = views.map((view) => view.id === tableId ? { ...view, ...body } : view);
      return { body: views[0] };
    },
    [`PATCH /items/${task.id}`]: (body) => {
      item = { ...item, ...body, fields: { ...item.fields, ...body.fields }, version: item.version + 1 };
      return { body: item };
    },
  } });
}

test("column picker saves custom field columns to the view and the Table edits a cell inline", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto(path);
  const grid = page.getByRole("table", { name: "Table" });
  await expect(grid.getByRole("columnheader", { name: "Cost" })).toHaveCount(0);
  await page.getByRole("button", { name: "Columns" }).click();
  const dialog = page.getByRole("dialog", { name: "Choose columns" });
  await expect(dialog.getByRole("group", { name: /Baselines/ })).toHaveAttribute("disabled", "");
  expect(await new AxeBuilder({ page }).include('[role="dialog"]').analyze().then((r) => r.violations)).toEqual([]);
  await dialog.getByLabel("Cost").check();
  await dialog.getByLabel("Level").check();
  await dialog.getByRole("button", { name: "Save columns" }).click();
  expect(calls.find((call) => call.method === "PATCH" && call.path === `/views/${tableId}`)?.body.config.columns)
    .toEqual(["status", "assignee", "start_date", "due_date", `field:${costId}`, `field:${levelId}`]);
  await expect(grid.getByRole("columnheader", { name: "Cost" })).toBeVisible();
  await expect(grid.getByRole("button", { name: "Cost: Plan" })).toHaveText("12");
  await expect(grid.getByRole("button", { name: "Level: Plan" })).toContainText("High");
  await page.reload();
  await expect(grid.getByRole("columnheader", { name: "Level" })).toBeVisible();

  await grid.getByRole("button", { name: "Cost: Plan" }).click();
  const editor = page.getByRole("dialog").getByRole("form", { name: "Cost" });
  await editor.getByRole("textbox", { name: "Cost" }).fill("40");
  await editor.getByRole("button", { name: "Save" }).click();
  await expect(grid.getByRole("button", { name: "Cost: Plan" })).toHaveText("40");
  expect(calls.find((call) => call.method === "PATCH" && call.path === `/items/${task.id}`)?.body).toEqual({ fields: { [costId]: 40 } });
});

test("field sort and filter live in the URL and reach the server", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto(path);
  await chip(page, "Sort");
  await expect(page.getByRole("combobox", { name: "Sort" }).locator("option", { hasText: "Field: Cost" })).toHaveCount(1);
  await expect(page.getByRole("combobox", { name: "Sort" }).locator("option", { hasText: "Field: Tags" })).toHaveCount(0);
  await page.getByRole("combobox", { name: "Sort" }).selectOption(`field:${costId}`);
  await expect.poll(() => calls.some((call) => call.path === `${path}/items` && call.query.get("sort") === `field:${costId}`)).toBe(true);
  await chip(page, "Filter");
  await page.getByRole("combobox", { name: "Filter by field" }).selectOption(levelId);
  await page.getByRole("combobox", { name: "Field value" }).selectOption(high.id);
  await expect(page).toHaveURL(new RegExp(`field=${levelId}%3A${high.id}`));
  await expect.poll(() => calls.some((call) => call.path === `${path}/items` && call.query.get("field") === `${levelId}:${high.id}`)).toBe(true);
  await page.reload();
  await chip(page, "Filter");
  await expect(page.getByRole("combobox", { name: "Field value" })).toHaveValue(high.id);
});

test("Board cards show only the chosen fields, and a viewer cannot edit cells", async ({ page }) => {
  await mock(page, ["view"], ["view"]);
  await page.goto(`${path}?view=${boardId}`);
  const card = page.getByRole("listitem").filter({ hasText: "Plan" });
  await expect(card).toContainText(/Cost:\s*12/);
  await expect(card).not.toContainText("Level");
  await page.getByRole("button", { name: "Card fields" }).click();
  await expect(page.getByRole("button", { name: "Save columns" })).toHaveCount(0);
  await expect(page.getByRole("checkbox", { name: "Cost" })).toBeDisabled();
});
