import { expect, test } from "@playwright/test";
import { marketing, mockSpaces } from "./spaces-mock";

// Same Space settings, two entry points: the Workspace registry (/settings/spaces) and the Space work area.
const registryRow = (id: string, name: string) => ({
  id, name, description: null, kind: "shared", version: 1, locked: false, visibility: "private", archived: false, icon_key: null, is_space_admin: false,
  my_capabilities: ["view", "edit", "share"], created_at: "2026-01-02T00:00:00Z", last_activity: null, admins: ["Ada Lovelace"], shares: 2, projects: 3, tools: 1,
});

test("NAV-01/03: opening Settings from the registry stays in the Settings shell and keeps the context on tab switches", async ({ page }) => {
  await mockSpaces(page, { extra: { "GET /spaces/registry": { body: { items: [registryRow(marketing.id, "Marketing")] } } } });
  await page.goto("/settings/spaces");
  await page.getByRole("button", { name: "Actions for Marketing" }).click();
  await page.getByRole("menuitem", { name: "Settings" }).click();
  await expect(page).toHaveURL(new RegExp(`/settings/spaces/${marketing.id}/general$`));
  await expect(page.getByRole("navigation", { name: "Space settings sections" })).toBeVisible();
  await page.getByRole("navigation", { name: "Space settings sections" }).getByRole("link", { name: "Members & access" }).click();
  await expect(page).toHaveURL(new RegExp(`/settings/spaces/${marketing.id}/members$`));
  await expect(page.getByRole("link", { name: "Back to all Spaces" })).toBeVisible();
});

test("NAV-05/06: the back links return to the registry or to the Space", async ({ page }) => {
  await mockSpaces(page, { extra: { "GET /spaces/registry": { body: { items: [registryRow(marketing.id, "Marketing")] } } } });
  await page.goto(`/settings/spaces/${marketing.id}/general`);
  await page.getByRole("link", { name: "Back to all Spaces" }).click();
  await expect(page).toHaveURL(/\/settings\/spaces(\?|$)/);
  await page.goto(`/spaces/${marketing.id}/settings/general`);
  await page.getByRole("link", { name: "Back to Space" }).click();
  await expect(page).toHaveURL(new RegExp(`/spaces/${marketing.id}$`));
});

test("NAV-08/09/10: the old route still works, the new route works, and an unknown tab goes to General in the same context", async ({ page }) => {
  await mockSpaces(page);
  await page.goto(`/spaces/${marketing.id}/settings/members`);
  await expect(page.getByRole("link", { name: "Members & access" })).toHaveAttribute("aria-current", "page");
  await page.goto(`/settings/spaces/${marketing.id}/members`);
  await expect(page.getByRole("link", { name: "Members & access" })).toHaveAttribute("aria-current", "page");
  await page.goto(`/settings/spaces/${marketing.id}/nope`);
  await expect(page).toHaveURL(new RegExp(`/settings/spaces/${marketing.id}/general$`));
  await page.goto(`/spaces/${marketing.id}/settings/nope`);
  await expect(page).toHaveURL(new RegExp(`/spaces/${marketing.id}/settings/general$`));
});

test("registry filters and page live in the URL and survive a reload", async ({ page }) => {
  await mockSpaces(page, { extra: { "GET /spaces/registry": { body: { items: [registryRow("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "Finance"), registryRow("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "Ops")] } } } });
  await page.goto("/settings/spaces?q=fin&sort=projects");
  await expect(page.getByRole("link", { name: "Finance" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Ops" })).toHaveCount(0);
  await page.reload();
  await expect(page.getByRole("link", { name: "Finance" })).toBeVisible();
  await expect(page.getByRole("searchbox").first()).toHaveValue("fin");
});

test("FUN-03/04: unsaved General changes ask first in the Workspace context too; Keep editing stays", async ({ page }) => {
  await mockSpaces(page);
  await page.goto(`/settings/spaces/${marketing.id}/general`);
  await page.getByRole("textbox", { name: "Name" }).fill("Growth");
  await page.getByRole("link", { name: "Members & access" }).click();
  const dialog = page.getByRole("dialog", { name: "Leave without saving?" });
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "Keep editing" }).click();
  await expect(page).toHaveURL(new RegExp(`/settings/spaces/${marketing.id}/general$`));
});

test("NAV-07: browser back and forward move between Settings tabs in the same context", async ({ page }) => {
  await mockSpaces(page);
  await page.goto(`/settings/spaces/${marketing.id}/general`);
  await page.getByRole("navigation", { name: "Space settings sections" }).getByRole("link", { name: "Tools" }).click();
  await expect(page).toHaveURL(new RegExp(`/settings/spaces/${marketing.id}/tools$`));
  await page.goBack();
  await expect(page).toHaveURL(new RegExp(`/settings/spaces/${marketing.id}/general$`));
  await page.goForward();
  await expect(page).toHaveURL(new RegExp(`/settings/spaces/${marketing.id}/tools$`));
});
