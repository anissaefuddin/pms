import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { marketing, mockSpaces } from "./spaces-mock";

// FND-01/FND-04: the shell must not overflow sideways, and the sidebar must fill the viewport height at the sizes the team uses.
const sizes = [
  { width: 1280, height: 800 },
  { width: 1440, height: 900 },
  { width: 1920, height: 1080 },
];

for (const size of sizes) {
  for (const theme of ["light", "dark"] as const) {
    test(`shell fits ${size.width}x${size.height} ${theme} without sideways scroll`, async ({ page }) => {
      await page.setViewportSize(size);
      await mockSpaces(page, { me: { ...me, theme } });
      await page.goto(`/spaces/${marketing.id}`);
      await expect(page.getByRole("heading", { name: "Marketing", level: 1 })).toBeVisible();
      const metrics = await page.evaluate(() => {
        const aside = document.querySelector("aside");
        return {
          pageOverflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
          sidebarHeight: aside?.getBoundingClientRect().height ?? 0,
          viewportHeight: window.innerHeight,
        };
      });
      expect(metrics.pageOverflow).toBeLessThanOrEqual(0);
      expect(Math.round(metrics.sidebarHeight)).toBe(metrics.viewportHeight);
    });
  }
}
