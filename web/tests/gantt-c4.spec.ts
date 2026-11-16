import { expect, test } from "@playwright/test";
import { bar, design, ganttId, mockGantt, path } from "./gantt-mock";

// C-04: weekends and holidays are shaded from the work schedule; "Show selected" needs a selected bar.
test("weekend columns are shaded and Show selected is enabled only with a selected bar", async ({ page }) => {
  await mockGantt(page);
  await page.goto(`${path}?view=${ganttId}`);
  await expect(page.locator(".gantt-off").first()).toBeVisible();
  await expect(page.getByRole("button", { name: "Show selected" })).toBeDisabled();
  await bar(page, design).click();
  await expect(page.getByRole("button", { name: "Show selected" })).toBeEnabled();
  await page.getByRole("button", { name: "Show selected" }).click();
  await expect(bar(page, design)).toBeInViewport();
});
