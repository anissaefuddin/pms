import { expect, test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";

// The account menu is reachable and usable from the keyboard: open, move, choose, close with focus returned.
test("account menu opens from the keyboard, moves between items, and closes back to its trigger", async ({ page }) => {
  await mockSpaces(page);
  await page.goto("/");
  const trigger = page.getByRole("button", { name: "Account menu" });
  await trigger.focus();
  await page.keyboard.press("Enter");
  const settings = page.getByRole("menuitem", { name: "Settings" });
  await expect(settings).toBeVisible();
  await expect(settings).toBeFocused();
  await page.keyboard.press("ArrowDown");
  await expect(page.getByRole("menuitem", { name: "Appearance" })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("menuitem", { name: "Settings" })).toHaveCount(0);
  await expect(trigger).toBeFocused();
});

test("Language & region in the account menu opens the full Language page", async ({ page }) => {
  await mockSpaces(page);
  await page.goto("/");
  await page.getByRole("button", { name: "Account menu" }).click();
  await page.getByRole("menuitem", { name: "Language & region" }).click();
  await expect(page).toHaveURL(/\/settings\/language$/);
});
