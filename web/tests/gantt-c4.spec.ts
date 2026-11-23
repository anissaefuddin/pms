import { expect, test } from "@playwright/test";
import { bar, design, ganttId, mockGantt, path } from "./gantt-mock";

// C-04: weekends and holidays are shaded from the work schedule; Fit project frames the schedule.
test("weekend and configured holiday columns are distinct and Fit project frames the schedule", async ({ page }) => {
  await mockGantt(page, { extra: {
    "GET /settings/work-schedule": { body: { working_days: [1, 2, 3, 4, 5], daily_hours: 8, version: 1, exceptions: [{ date: "2026-11-05", name: "Founders Day" }] } },
  } });
  await page.goto(`${path}?view=${ganttId}`);
  await expect(page.locator(".gantt-weekend").first()).toBeVisible();
  await expect(page.locator(".gantt-holiday").first()).toBeVisible();
  await bar(page, design).click();
  await page.getByRole("button", { name: "Fit project" }).click();
  await expect(bar(page, design)).toBeInViewport();
});
