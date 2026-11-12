import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const item = { id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", type: "task", title: "Deleted pilot task",
  seq: 1, version: 1, my_capabilities: ALL, locations: [{ space_id: marketing.id, space_name: marketing.name }], assignees: [] };

test("admin sees seats, invites users and opens the policy matrix", async ({ page }) => {
  let invited = false;
  const { calls } = await mockSpaces(page, { extra: {
    "GET /seats": () => ({ body: { used: 1, pending: invited ? 1 : 0, max: 3, available: invited ? 1 : 2 } }),
    "GET /users": { body: { items: [me] } },
    "POST /users/invitations": () => { invited = true; return { status: 201, body: me }; },
    "GET /groups": { body: { items: [] } },
    "GET /users/types": { body: { roles: { read: ["view"] }, user_types: { regular: ["view", "edit"] } } },
  } });
  await page.goto("/settings/users");
  await expect(page.getByText("1 active · 0 invited · 2 available of 3 seats")).toBeVisible();
  await page.getByRole("button", { name: "Invite user" }).click();
  await page.getByRole("textbox", { name: "Email" }).fill("pilot@example.test");
  await page.getByRole("button", { name: "Send invitation" }).click();
  await expect(page.getByText("1 active · 1 invited · 1 available of 3 seats")).toBeVisible();
  expect(calls.find((call) => call.path === "/users/invitations")?.body).toMatchObject({ email: "pilot@example.test", user_type: "regular" });
  await page.getByRole("button", { name: "Groups" }).click();
  await expect(page.getByRole("button", { name: "New group" })).toBeVisible();
  await page.getByRole("link", { name: "User types" }).click();
  await expect(page.getByText("view, edit")).toBeVisible();
});

test("non-admin has no Users link and direct navigation is not found", async ({ page }) => {
  const { calls } = await mockSpaces(page, { me: { ...me, capabilities: { ...me.capabilities, manage_users: false } } });
  await page.goto("/settings");
  await expect(page.getByRole("link", { name: "Users", exact: true })).toHaveCount(0);
  await page.goto("/settings/users");
  await expect(page.getByText("Not Found", { exact: true })).toBeVisible();
  expect(calls.some((call) => call.path === "/seats" || call.path === "/users")).toBe(false);
});

test("profile saves preferences and applies theme and density", async ({ page }) => {
  let profile = { ...me, theme: "system", density: "compact" };
  const { calls } = await mockSpaces(page, { extra: {
    "GET /me": () => ({ body: profile }),
    "PATCH /me": (body) => { profile = { ...profile, ...body }; return { body: profile }; },
    [`GET /items/${item.id}`]: { body: item, headers: { ETag: '"1"' } },
    [`GET /items/${item.id}/children`]: { body: { items: [] } },
    [`GET /items/${item.id}/comments`]: { body: { items: [] } },
    [`POST /items/${item.id}/comments`]: { status: 201, body: {} },
    [`GET /items/${item.id}/attachments`]: { body: { items: [] } },
  } });
  await page.goto("/settings/profile");
  await page.getByLabel("Company").fill("Pilot Ltd");
  await page.getByLabel("Theme").selectOption("dark");
  await page.getByLabel("Density").selectOption("comfortable");
  await page.getByLabel("Send comment shortcut").selectOption("enter");
  await page.getByRole("button", { name: "Save profile" }).click();
  await expect(page.locator("html")).toHaveCSS("color-scheme", "dark");
  await expect(page.locator("html")).toHaveAttribute("data-density", "comfortable");
  expect(calls.find((call) => call.method === "PATCH" && call.path === "/me")?.body)
    .toMatchObject({ company: "Pilot Ltd", theme: "dark", density: "comfortable" });
  await page.reload();
  await expect(page.getByLabel("Company")).toHaveValue("Pilot Ltd");
  await page.goto(`/tasks/${item.id}`);
  const editor = page.getByRole("region", { name: "Comments" }).locator('[contenteditable="true"][aria-label="Comment"]');
  await editor.fill("Keyboard preference");
  await editor.press("Enter");
  await expect.poll(() => calls.some((call) => call.method === "POST" && call.path === `/items/${item.id}/comments`)).toBe(true);
});

test("deleted item offers Undo and Recycle bin explains a blocked restore", async ({ page }) => {
  const blocked = { ...item, title: "Blocked pilot item", restore_blocked_reason: "space_deleted" };
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /items/${item.id}`]: { body: item, headers: { ETag: '"1"' } },
    [`GET /items/${item.id}/children`]: { body: { items: [] } },
    [`GET /items/${item.id}/comments`]: { body: { items: [] } },
    [`GET /items/${item.id}/attachments`]: { body: { items: [], max_upload_bytes: 1048576 } },
    [`DELETE /items/${item.id}`]: { status: 204 },
    "GET /recycle-bin": { body: { items: [blocked] } },
    [`POST /recycle-bin/${item.id}/restore`]: { body: item },
  } });
  await page.goto(`/tasks/${item.id}`);
  await page.getByRole("button", { name: "Item actions" }).click();
  await page.getByRole("menuitem", { name: "Delete item" }).click();
  await page.getByRole("dialog", { name: "Delete item" }).getByRole("button", { name: "Delete item" }).click();
  await page.getByRole("button", { name: "Undo" }).click();
  await expect(page.getByText("Item restored", { exact: true })).toBeVisible();
  expect(calls.some((call) => call.method === "POST" && call.path === `/recycle-bin/${item.id}/restore`)).toBe(true);
  await page.goto("/recycle-bin");
  await expect(page.getByText("The original Space was deleted.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Restore", exact: true })).toBeDisabled();
});
