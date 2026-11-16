import { expect, test } from "@playwright/test";
import { bar, design, ganttId, mockGantt, path } from "./gantt-mock";

// C1: "Fit project" brings the earliest scheduled bar into view without changing any date.
test("Fit project scrolls the chart to the earliest bar and changes no dates", async ({ page }) => {
  const { calls } = await mockGantt(page);
  await page.goto(`${path}?view=${ganttId}`);
  await page.getByRole("button", { name: "Fit project" }).click();
  await expect(bar(page, design)).toBeInViewport();
  expect(calls.filter((call) => call.method === "PATCH")).toHaveLength(0);
});
