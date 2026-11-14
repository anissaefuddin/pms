import type { Page } from "@playwright/test";

/** Opens one toolbar chip popover (Filter, Sort, Group); the previous popover is closed first. */
export async function chip(page: Page, name: "Filter" | "Sort" | "Group") {
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name, exact: true }).click();
}
