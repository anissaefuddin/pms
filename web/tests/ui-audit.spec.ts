import { AxeBuilder } from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";

// S7.1.4: the re-audit sweep behind the new UI score. Every main screen at the four audit viewports must fit the
// viewport (no horizontal page scroll) and have no axe violations, and the navigation must hold no dead link.
const viewports = [{ width: 1440, height: 900 }, { width: 1280, height: 800 }, { width: 768, height: 1024 }, { width: 390, height: 844 }];
const routes = ["/", "/my-todo", "/inbox", "/stream", "/starred", "/forms", "/requests", "/requests/mine", "/dashboards", "/reports", "/timesheets",
  "/recycle-bin", "/shared-with-me", "/settings/profile", "/settings/account", "/settings/security", "/settings/work-schedule", "/settings/workflows", "/settings/users"];

for (const viewport of viewports) {
  test.describe(`${viewport.width}px`, () => {
    test.use({ viewport });
    for (const route of routes) {
      test(`${route} fits the viewport and passes axe`, async ({ page }) => {
        await mockSpaces(page);
        await page.goto(route);
        await page.waitForLoadState("networkidle");
        const widths = await page.evaluate(() => ({ doc: document.documentElement.scrollWidth, view: window.innerWidth }));
        expect(widths.doc, `document ${widths.doc}px wider than ${widths.view}px`).toBeLessThanOrEqual(widths.view);
        expect((await new AxeBuilder({ page }).analyze()).violations.map((v) => v.id)).toEqual([]);
      });
    }
  });
}

test("every entry in the navigation opens a real page", async ({ page }) => {
  await mockSpaces(page);
  await page.goto("/");
  const nav = page.getByRole("navigation", { name: "Main navigation" });
  const targets: string[] = [];
  await nav.getByRole("button", { name: "More" }).click();
  await expect(page.getByRole("menuitem").first()).toBeVisible();
  let entries = await page.getByRole("menuitem").count();
  expect(entries).toBeGreaterThan(3);
  await page.keyboard.press("Escape");
  for (let i = 0; i < entries; i++) {
    await nav.getByRole("button", { name: "More" }).click();
    await expect(page.getByRole("menuitem").first()).toBeVisible();
    const entry = page.getByRole("menuitem").nth(i);
    if ((await entry.innerText()).trim() === "Customize sidebar") { // S-UIP.2.1: opens a dialog, not a page
      await entry.click();
      await expect(page.getByRole("dialog")).toBeVisible();
      await page.keyboard.press("Escape");
      entries--;
      continue;
    }
    await entry.click();
    await page.waitForLoadState("networkidle");
    targets.push(new URL(page.url()).pathname);
    await expect(page.getByText("Not Found", { exact: true }), `dead entry ${i} -> ${page.url()}`).toHaveCount(0);
  }
  for (const link of await nav.getByRole("button").all()) {
    expect(await link.isVisible()).toBe(true);
  }
  expect(new Set(targets).size).toBe(entries);
});
