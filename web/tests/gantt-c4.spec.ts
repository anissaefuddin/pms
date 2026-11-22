import { expect, test } from "@playwright/test";
import { bar, design, ganttId, mockGantt, path } from "./gantt-mock";

// C-04: weekends and holidays are shaded from the work schedule; Fit project frames the schedule.
test("weekend columns are shaded and Fit project frames the schedule", async ({ page }) => {
  await mockGantt(page);
  await page.goto(`${path}?view=${ganttId}`);
  await expect(page.locator(".gantt-off").first()).toBeVisible();
  await bar(page, design).click();
  await page.getByRole("button", { name: "Fit project" }).click();
  await expect(bar(page, design)).toBeInViewport();
});
