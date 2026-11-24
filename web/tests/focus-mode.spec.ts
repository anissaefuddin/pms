import { expect, test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";

// Focus mode is app-wide like the sidebar toggle: the same top-bar control works on any page and survives a reload.
test("focus mode works on a page without views and stays on after a reload", async ({ page }) => {
  await mockSpaces(page);
  await page.goto("/inbox");
  const sidebar = page.locator("aside").first();
  const focus = page.getByRole("button", { name: "Focus", exact: true });
  await expect(focus).toHaveAttribute("aria-pressed", "false");
  const open = (await sidebar.boundingBox())!.width;
  await focus.click();
  await expect(page.getByRole("button", { name: "Exit focus", exact: true })).toHaveAttribute("aria-pressed", "true");
  await expect(page.locator("html")).toHaveAttribute("data-view-focus", "true");
  await expect.poll(async () => (await sidebar.boundingBox())!.width).toBeLessThan(open);

  await page.reload();
  await expect(page.getByRole("button", { name: "Exit focus", exact: true })).toHaveAttribute("aria-pressed", "true");
  await expect.poll(async () => (await sidebar.boundingBox())!.width).toBeLessThan(open);

  await page.getByRole("button", { name: "Exit focus", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-view-focus", "false");
  await expect.poll(async () => (await sidebar.boundingBox())!.width).toBe(open);
});
