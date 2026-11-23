import { expect, test } from "@playwright/test";
import { bar, design, ganttId, mockGantt, path } from "./gantt-mock";

// C1: "Fit project" brings the earliest scheduled bar into view without changing any date.
test("Fit project scrolls the chart to the earliest bar and changes no dates", async ({ page }) => {
  const { calls } = await mockGantt(page);
  await page.goto(`${path}?view=${ganttId}`);
  await page.getByRole("button", { name: "Fit project" }).click();
  await page.getByRole("menuitem", { name: "Fit all scheduled items" }).click();
  await expect(bar(page, design)).toBeInViewport();
  expect(calls.filter((call) => call.method === "PATCH")).toHaveLength(0);
});

test("date picker jumps directly to a selected month and accepts a typed year", async ({ page }) => {
  await mockGantt(page);
  await page.goto(`${path}?view=${ganttId}`);
  await page.getByRole("button", { name: "Choose timeline month" }).click();
  await page.getByLabel("Years").fill("2027");
  await page.getByRole("button", { name: "Jan", exact: true }).click();
  await expect(page.getByRole("button", { name: "Choose timeline month" })).toHaveText(/Jan 2027/);
});
