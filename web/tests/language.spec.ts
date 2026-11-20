import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

// ID/EN switching: the profile menu switches live and remembers the choice; a saved profile language wins after sign-in.
test("the profile menu switches the language live and remembers it", async ({ page }) => {
  await mockSpaces(page);
  await page.goto("/");
  await page.getByRole("button", { name: "Account menu" }).click();
  await page.getByRole("menuitemradio", { name: "Bahasa Indonesia" }).click();
  await expect(page.locator("html")).toHaveAttribute("lang", "id");
  // The menu may close itself after the language choice; open it again only while its items are not showing.
  await expect(async () => {
    if (!(await page.getByRole("menuitem", { name: "Keluar" }).isVisible())) await page.getByRole("button", { name: "Menu akun" }).click();
    await expect(page.getByRole("menuitem", { name: "Keluar" })).toBeVisible({ timeout: 1000 });
  }).toPass({ timeout: 15000 });
  expect(await page.evaluate(() => localStorage.getItem("pms-language"))).toBe("id");
});

test("a saved profile language is used after sign-in", async ({ page }) => {
  await mockSpaces(page, { me: { ...me, locale: "id" } });
  await page.goto("/");
  await expect(page.locator("html")).toHaveAttribute("lang", "id");
});
