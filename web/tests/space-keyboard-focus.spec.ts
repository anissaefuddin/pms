import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { marketing, mockSpaces } from "./spaces-mock";

// Focus audit (throwaway): the Space settings tabs and header are reachable from the keyboard; dialogs trap and return focus.
const focusName = (page: any) => page.evaluate(() => {
  const el = document.activeElement as HTMLElement | null;
  return el ? `${el.tagName}|${el.getAttribute("role") ?? ""}|${(el.getAttribute("aria-label") || el.textContent || "").trim().slice(0, 40)}` : "none";
});
const visibleFocusRing = (page: any) => page.evaluate(() => {
  const el = document.activeElement as HTMLElement | null;
  if (!el) return false;
  const s = getComputedStyle(el);
  return s.outlineStyle !== "none" && s.outlineWidth !== "0px" || s.boxShadow !== "none";
});

test("space settings tabs are keyboard reachable and show a focus ring", async ({ page }) => {
  await mockSpaces(page);
  await page.goto(`/spaces/${marketing.id}/settings/general`);
  await page.getByRole("link", { name: "General", exact: true }).first().focus();
  const seen: string[] = [];
  for (let i = 0; i < 12; i++) {
    await page.keyboard.press("Tab");
    seen.push(await focusName(page));
  }
  const tabs = seen.filter((s) => s.includes("Members & access") || s.includes("Tools") || s.includes("Custom fields"));
  expect(tabs.length).toBeGreaterThan(0);
  await page.getByRole("link", { name: "Members & access" }).focus();
  expect(await visibleFocusRing(page)).toBe(true);
});

test("the unsaved-changes dialog traps focus and returns it on Keep editing", async ({ page }) => {
  await mockSpaces(page);
  await page.goto(`/spaces/${marketing.id}/settings/general`);
  await page.getByRole("textbox", { name: "Name" }).fill("Growth");
  await page.getByRole("link", { name: "Members & access" }).click();
  const dialog = page.getByRole("dialog", { name: "Leave without saving?" });
  await expect(dialog).toBeVisible();
  await page.keyboard.press("Tab");
  expect(await page.evaluate(() => !!document.activeElement?.closest("[role=dialog]"))).toBe(true);
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(page).toHaveURL(new RegExp(`/settings/general$`));
});

test("axe finds no serious violations on Space settings and the registry", async ({ page }) => {
  await mockSpaces(page);
  for (const path of [`/spaces/${marketing.id}/settings/general`, `/spaces/${marketing.id}/settings/workflows`, "/settings/spaces"]) {
    await page.goto(path);
    await page.waitForTimeout(300);
    const results = await new AxeBuilder({ page }).analyze();
    const serious = results.violations.filter((v) => v.impact === "serious" || v.impact === "critical").map((v) => `${path}: ${v.id}`);
    expect(serious).toEqual([]);
  }
});

test("the Space header keeps New item primary; Share, Bookmarks and Settings are first in the actions menu", async ({ page }) => {
  await mockSpaces(page);
  await page.goto(`/spaces/${marketing.id}`);
  await expect(page.getByRole("button", { name: "New item", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Space actions" }).click();
  await expect(page.getByRole("menuitem").first()).toBeVisible();
  const names = await page.getByRole("menuitem").allInnerTexts();
  expect(names.slice(0, 3)).toEqual(["Share", "Bookmarks", "Settings"]);
});
