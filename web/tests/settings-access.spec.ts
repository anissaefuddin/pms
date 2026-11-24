import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { marketing, mockSpaces } from "./spaces-mock";

test("regular settings navigation keeps personal pages and hides workspace administration", async ({ page }) => {
  const regular = { ...me, user_type: "regular", capabilities: { ...me.capabilities, manage_users: false } };
  await mockSpaces(page, { me: regular as typeof me, spaces: [{ ...marketing, my_capabilities: ["view"] }] as any });
  await page.goto("/settings/security");
  const nav = page.getByRole("navigation", { name: "Settings navigation" });
  await expect(nav.getByRole("link", { name: "Work preferences" })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Developer tools" })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Apps" })).toHaveCount(0);
  await expect(nav.getByRole("link", { name: "Resources" })).toHaveCount(0);
  await expect(nav.getByRole("link", { name: "Account" })).toHaveCount(0);
  await expect(nav.getByText("Work configuration")).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Authorized apps" })).toBeVisible();
});

test("admin sees workspace administration and configuration destinations", async ({ page }) => {
  await mockSpaces(page);
  await page.goto("/settings/security");
  const nav = page.getByRole("navigation", { name: "Settings navigation" });
  await expect(nav.getByRole("link", { name: "Apps" })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Resources" })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Workspace", exact: true })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Item types" })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Workflows" })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Custom fields" })).toBeVisible();
});

test("direct admin-only settings URLs fail closed for a regular user", async ({ page }) => {
  const regular = { ...me, user_type: "regular", capabilities: { ...me.capabilities, manage_users: false } };
  const { calls } = await mockSpaces(page, { me: regular as typeof me, spaces: [{ ...marketing, my_capabilities: ["view"] }] as any });
  for (const path of ["/settings/apps", "/settings/resources", "/settings/account", "/settings/item-types", "/settings/workflows", "/settings/custom-fields"]) {
    await page.goto(path);
    await expect(page.getByRole("alert")).toHaveText("Administrator access is required.");
  }
  expect(calls.some((call) => ["/account", "/resource-directory", "/workflows", "/custom-fields"].includes(call.path))).toBe(false);
});
