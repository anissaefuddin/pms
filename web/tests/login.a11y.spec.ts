import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

for (const colorScheme of ["light", "dark"] as const) {
  test.describe(colorScheme, () => {
    test.use({ colorScheme });

    test("login page has no WCAG A or AA violations", async ({ page }) => {
      await page.goto("/login");
      await expect(page.getByRole("heading", { name: "Sign in to your workspace" })).toBeVisible();
      await expect(page.getByRole("textbox", { name: "Email" })).toBeVisible();
      await expect(page.getByLabel("Password")).toBeVisible();

      const root = page.locator("html");
      await expect(root).toHaveAttribute("data-brand", "default");
      await expect(root).toHaveAttribute("data-density", "compact");
      await expect(root).toHaveCSS("color-scheme", colorScheme);
      await expect(page.locator("body")).toHaveCSS("font-size", "13px");

      const { violations } = await new AxeBuilder({ page })
        .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
        .analyze();
      expect(violations).toEqual([]);
    });
  });
}
