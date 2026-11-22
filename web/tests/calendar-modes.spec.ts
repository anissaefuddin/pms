import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// C-03: Week mode shows one week of seven days and moves by week; Month mode is unchanged.
const calId = "77777777-7777-4777-8777-777777777773";
const path = `/spaces/${marketing.id}`;

test("Week mode shows seven days and moves by a week; Month returns to the month grid", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET ${path}/views`]: { body: { items: [{ id: calId, type: "calendar", name: "Calendar", visibility: "public", position: 0, is_default: true, config: {} }] } },
    [`GET ${path}/items`]: { body: { items: [] } },
    "GET /workflows": { body: { items: [] } },
    "GET /settings/work-schedule": { body: { working_days: [1, 2, 3, 4, 5], daily_hours: 8, version: 1, exceptions: [] } },
  } });
  await page.goto(path);
  const grid = page.getByRole("grid", { name: "Calendar" });
  await expect(grid.getByRole("gridcell").first()).toBeVisible();
  const monthCells = await grid.getByRole("gridcell").count(); // 28, 35 or 42 depending on the month
  expect([28, 35, 42]).toContain(monthCells);
  await page.getByRole("button", { name: "Week", exact: true }).click();
  await expect(grid.getByRole("gridcell")).toHaveCount(7);
  const headingBefore = await page.getByRole("heading", { level: 2, name: /–|\d{4}/ }).first().textContent();
  await page.getByRole("button", { name: "Next week" }).click();
  await expect(page.getByRole("heading", { level: 2, name: /–|\d{4}/ }).first()).not.toHaveText(headingBefore ?? "");
  await page.getByRole("button", { name: "Month", exact: true }).click();
  await expect(grid.getByRole("gridcell")).toHaveCount(monthCells);
});

test("Quarter and Year modes show their complete planning ranges", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET ${path}/views`]: { body: { items: [{ id: calId, type: "calendar", name: "Calendar", visibility: "public", position: 0, is_default: true, config: {} }] } },
    [`GET ${path}/items`]: { body: { items: [] } },
    "GET /workflows": { body: { items: [] } },
    "GET /settings/work-schedule": { body: { working_days: [1, 2, 3, 4, 5], daily_hours: 8, version: 1, exceptions: [] } },
  } });
  await page.goto(path);
  const grid = page.getByRole("grid", { name: "Calendar" });
  await page.getByRole("button", { name: "Quarter", exact: true }).click();
  expect(await grid.getByRole("gridcell").count()).toBeGreaterThanOrEqual(84);
  await expect(page.getByRole("heading", { level: 2, name: /Q[1-4]/ })).toBeVisible();
  await page.getByRole("button", { name: "Year", exact: true }).click();
  expect(await grid.getByRole("gridcell").count()).toBeGreaterThanOrEqual(364);
  await expect(page.getByRole("heading", { level: 2, name: String(new Date().getFullYear()) })).toBeVisible();
});
