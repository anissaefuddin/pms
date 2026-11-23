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
  await page.setViewportSize({ width: 1440, height: 900 });
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
  await expect.poll(async () => { const box = await grid.boundingBox(); return box!.y + box!.height; }).toBeLessThanOrEqual(900);
  await page.getByRole("button", { name: "Year", exact: true }).click();
  expect(await grid.getByRole("gridcell").count()).toBeGreaterThanOrEqual(364);
  await expect(page.getByRole("heading", { level: 2, name: String(new Date().getFullYear()) })).toBeVisible();
  await expect.poll(async () => { const box = await grid.boundingBox(); return box!.y + box!.height; }).toBeLessThanOrEqual(900);
});

test("period picker navigates directly and browser Back restores the previous period", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET ${path}/views`]: { body: { items: [{ id: calId, type: "calendar", name: "Calendar", visibility: "public", position: 0, is_default: true, config: {} }] } },
    [`GET ${path}/items`]: { body: { items: [] } },
    "GET /workflows": { body: { items: [] } },
    "GET /settings/work-schedule": { body: { working_days: [1, 2, 3, 4, 5], daily_hours: 8, version: 1, exceptions: [] } },
  } });
  await page.goto(path);
  const periodHeading = page.locator("h2[aria-live]");
  const original = await periodHeading.textContent();
  await page.getByRole("button", { name: "Choose calendar period" }).click();
  await page.getByLabel("Year").fill("2029");
  await page.getByRole("button", { name: "Mar", exact: true }).click();
  await expect(periodHeading).toHaveText(/March 2029/);
  await expect.poll(() => new URL(page.url()).searchParams.get("calDate")).toBe("2029-03-01");
  await page.goBack();
  await expect(periodHeading).toHaveText(original ?? "");
});

test("clicking a date opens Day details and Year month headings drill down", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET ${path}/views`]: { body: { items: [{ id: calId, type: "calendar", name: "Calendar", visibility: "public", position: 0, is_default: true, config: {} }] } },
    [`GET ${path}/items`]: { body: { items: [] } },
    "GET /workflows": { body: { items: [] } },
    "GET /settings/work-schedule": { body: { working_days: [1, 2, 3, 4, 5], daily_hours: 8, version: 1, exceptions: [] } },
  } });
  await page.goto(path);
  const firstCell = page.getByRole("grid", { name: "Calendar" }).getByRole("gridcell").first();
  await firstCell.click();
  await expect(page.getByRole("dialog").getByText("Day details")).toBeVisible();
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Year", exact: true }).click();
  const month = new Date().toLocaleDateString("en-US", { month: "long", year: "numeric" });
  await page.getByRole("button", { name: month, exact: true }).click();
  await expect(page.getByRole("button", { name: "Month", exact: true })).toHaveAttribute("aria-pressed", "true");
});
