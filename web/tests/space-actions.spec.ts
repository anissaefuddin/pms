import { expect, test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";
import { marketing, ops } from "./spaces-mock";

// The Space row menu follows effective capabilities (D-17.1, D-SA1): what a role can do, not its name.
// A Full member without delete and without Space Admin: everything a member can do on content and members.
const MEMBER_CAPS = ["view", "comment", "change_status", "attach", "edit", "create", "share"];

async function openMenu(page: import("@playwright/test").Page, name: string) {
  await page.goto("/");
  await page.getByRole("link", { name, exact: true }).hover();
  await page.getByRole("button", { name: `Actions for ${name}` }).click();
}

test("a read-only member sees Open, Copy link and Settings, and nothing that changes the Space", async ({ page }) => {
  await mockSpaces(page, { spaces: [{ ...marketing, my_capabilities: ["view"] }] });
  await openMenu(page, "Marketing");
  const menu = page.getByRole("menu");
  await expect(menu.getByRole("menuitem", { name: "Open Space" })).toBeVisible();
  await expect(menu.getByRole("menuitem", { name: "Copy link" })).toBeVisible();
  await expect(menu.getByRole("menuitem", { name: "Rename" })).toHaveCount(0);
  await expect(menu.getByRole("menuitem", { name: "Members & access" })).toHaveCount(0);
  await expect(menu.getByRole("menuitem", { name: "Archive Space" })).toHaveCount(0);
});

test("an editor can rename but not manage members; a Full member can manage members but is not offered archive", async ({ page }) => {
  await mockSpaces(page, { spaces: [{ ...marketing, my_capabilities: ["view", "edit"] }, { ...ops, my_capabilities: MEMBER_CAPS }] });
  await openMenu(page, "Marketing");
  await expect(page.getByRole("menuitem", { name: "Rename" })).toBeVisible();
  await expect(page.getByRole("menuitem", { name: "Members & access" })).toHaveCount(0);
  await page.keyboard.press("Escape");
  await openMenu(page, "Operations");
  await expect(page.getByRole("menuitem", { name: "Members & access" })).toBeVisible();
  await expect(page.getByRole("menuitem", { name: "Archive Space" })).toHaveCount(0);
});

test("only an explicit Space Admin is offered Archive", async ({ page }) => {
  await mockSpaces(page, { spaces: [{ ...ops, my_capabilities: MEMBER_CAPS, is_space_admin: true }] });
  await openMenu(page, "Operations");
  await expect(page.getByRole("menuitem", { name: "Archive Space" })).toBeVisible();
});
