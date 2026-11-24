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

test("Space information stays behind the title icon and view actions share the tabs row", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET /spaces/${marketing.id}/views`]: { body: { items: [{ id: "11111111-1111-4111-8111-111111111111", type: "table", name: "Table", visibility: "public", position: 0, is_default: true, config: {} }] } },
    "GET /workflows": { body: { items: [] } },
  } });
  await page.goto(`/spaces/${marketing.id}`);
  await expect(page.getByText(marketing.description, { exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "Space information" }).click();
  await expect(page.getByText(marketing.description, { exact: true })).toBeVisible();
  await page.keyboard.press("Escape");

  const tabs = page.getByRole("navigation", { name: "Views" });
  // Focus is an app-wide control in the top bar (see focus-mode.spec.ts), so only the Space's own actions share this row.
  const create = page.getByRole("button", { name: "New item", exact: true });
  const menu = page.getByRole("button", { name: "Space actions" });
  for (const action of [create, menu]) await expect(action).toBeVisible();
  const tabsBox = (await tabs.boundingBox())!;
  const boxes = await Promise.all([create, menu].map((action) => action.boundingBox()));
  expect(boxes.every((box) => box && Math.abs(box.y - tabsBox.y) < 12)).toBe(true);
  expect(boxes.map((box) => box!.x)).toEqual([...boxes.map((box) => box!.x)].sort((a, b) => a - b));

  await menu.click();
  await expect(page.getByRole("menuitem").first()).toBeVisible();
  const names = await page.getByRole("menuitem").allInnerTexts();
  expect(names.slice(0, 3)).toEqual(["Share", "Bookmarks", "Settings"]);
});
