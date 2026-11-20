import { expect, test } from "@playwright/test";
import { mockSpaces, marketing } from "./spaces-mock";

// Batch 5: captures the item type pages at desktop widths in light and dark, for review. Images are not compared to a baseline.
const bug = { id: "33333333-3333-4333-8333-333333333333", key: "bug", name: "Bug", icon: "bug", base: "task", is_builtin: false, active: true, allowed_children: [], defaults: {}, layout: {}, version: 1 };
const out = "../docs/qa/review-1/batch5";
for (const width of [1280, 1440, 1920]) for (const scheme of ["light", "dark"] as const) {
  test(`item types pages at ${width}px ${scheme}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.emulateMedia({ colorScheme: scheme });
    await mockSpaces(page, { extra: { "GET /item-types": { body: { items: [{ id: "11111111-1111-4111-8111-111111111111", key: "task", name: "Task", icon: "circle", base: "task", is_builtin: true, active: true, allowed_children: [], defaults: {}, layout: {}, version: 1 }, bug] } } } });
    await page.goto("/settings/item-types");
    await expect(page.getByRole("button", { name: `Edit: ${bug.name}` })).toBeVisible();
    await page.screenshot({ path: `${out}/registry_${width}_${scheme}.png`, fullPage: true });
    await page.goto(`/spaces/${marketing.id}/item-types`);
    await expect(page.getByRole("heading", { name: "Item types in this Space" })).toBeVisible();
    await page.screenshot({ path: `${out}/space-item-types_${width}_${scheme}.png`, fullPage: true });
  });
}
