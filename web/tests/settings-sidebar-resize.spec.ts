import { expect, test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";

// The Settings sidebar resizes like the Workspace sidebar and keeps the width after a reload.
test("settings sidebar width changes from its edge and persists", async ({ page }) => {
  await mockSpaces(page);
  await page.goto("/settings");
  const handle = page.getByRole("separator", { name: "Resize sidebar" });
  const before = Number(await handle.getAttribute("aria-valuenow"));
  await handle.focus();
  await page.keyboard.press("ArrowRight");
  await page.keyboard.press("ArrowRight");
  const after = Number(await handle.getAttribute("aria-valuenow"));
  expect(after).toBe(Math.min(400, before + 16));
  await page.reload();
  await expect(page.getByRole("separator", { name: "Resize sidebar" })).toHaveAttribute("aria-valuenow", String(after));
});
