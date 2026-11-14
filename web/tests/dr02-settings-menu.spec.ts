import { expect, test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";

// DR-02: the experimental Gantt spike must not be linked from the production Settings menu.
test("Settings menu has no Gantt spike link", async ({ page }) => {
  await mockSpaces(page, {});
  await page.goto("/settings/apps");
  await expect(page.getByRole("navigation", { name: /settings/i }).first()).toBeVisible();
  await expect(page.getByRole("link", { name: /gantt spike/i })).toHaveCount(0);
});
