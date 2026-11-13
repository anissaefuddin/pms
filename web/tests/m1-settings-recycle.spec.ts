import { AxeBuilder } from "@axe-core/playwright";
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

test("emptying the Recycle bin asks for confirmation and deletes permanently", async ({ page }) => {
  let emptied = false;
  const { calls } = await mockSpaces(page, { extra: {
    "GET /recycle-bin": () => ({ body: { items: emptied ? [] : [item] } }),
    "DELETE /recycle-bin": () => { emptied = true; return { body: { deleted: 1 } }; },
  } });
  await page.goto("/recycle-bin");
  await page.getByRole("button", { name: "Empty Recycle bin" }).click();
  const dialog = page.getByRole("dialog", { name: "Empty the Recycle bin?" });
  await expect(dialog.getByRole("button", { name: "Cancel" })).toBeFocused();
  expect(calls.some((call) => call.method === "DELETE")).toBe(false);
  await expect(dialog).toHaveCSS("opacity", "1"); // axe must not measure contrast mid fade-in
  expect((await new AxeBuilder({ page }).include('[role="dialog"]').analyze()).violations).toEqual([]);
  await dialog.getByRole("button", { name: "Delete permanently" }).click();
  await expect(page.getByText("Recycle bin is empty")).toBeVisible();
  expect(calls.filter((call) => call.method === "DELETE" && call.path === "/recycle-bin")).toHaveLength(1);
});

test("account page lists seats and administrators; email and task defaults save", async ({ page }) => {
  let profile = { ...me, email_muted: [] as string[], default_task_duration_days: null as number | null };
  const { calls } = await mockSpaces(page, { extra: {
    "GET /me": () => ({ body: profile }),
    "PATCH /me": (body) => { profile = { ...profile, ...body }; return { body: profile }; },
    "GET /account": { body: { name: "Pilot Co", created_at: "2026-01-02T00:00:00Z", role: "admin", joined_at: "2026-01-03T00:00:00Z",
      administrators: [{ id: me.id, name: "Pilot Admin", email: "admin@example.test" }], seats: { used: 2, pending: 1, max: 5, available: 2 } } },
  } });
  await page.goto("/settings/account");
  await expect(page.getByText("Pilot Co")).toBeVisible();
  await expect(page.getByText("2 active · 1 invited · 2 available of 5 seats")).toBeVisible();
  await expect(page.getByRole("link", { name: "admin@example.test" })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.goto("/settings/profile");
  await page.getByRole("checkbox", { name: "Scheduled report emails" }).uncheck();
  await expect.poll(() => calls.find((call) => call.method === "PATCH" && call.path === "/me")?.body).toMatchObject({ email_muted: ["report"] });
  await page.getByLabel("Default task duration (days)").fill("3");
  await page.getByRole("button", { name: "Save profile" }).click();
  await expect.poll(() => calls.filter((call) => call.method === "PATCH").at(-1)?.body).toMatchObject({ default_task_duration_days: 3 });
});

test("item detail lists the activity log", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET /items/${item.id}`]: { body: item, headers: { ETag: '"1"' } },
    [`GET /items/${item.id}/children`]: { body: { items: [] } },
    [`GET /items/${item.id}/comments`]: { body: { items: [] } },
    [`GET /items/${item.id}/attachments`]: { body: { items: [], max_upload_bytes: 1048576 } },
    [`GET /items/${item.id}/activity`]: { body: { next_cursor: null, items: [
      { id: 2, actor: { id: me.id, name: "Pilot Admin" }, action: "update", changes: { title: { from: "a", to: "b" } }, created_at: "2026-11-13T08:00:00Z" },
      { id: 1, actor: { id: me.id, name: "Pilot Admin" }, action: "create", changes: {}, created_at: "2026-11-13T07:00:00Z" },
    ] } },
  } });
  await page.goto(`/tasks/${item.id}`);
  const log = page.getByRole("region", { name: "Activity" });
  await expect(log.getByText("Pilot Admin updated this item")).toBeVisible();
  await expect(log.getByText("Pilot Admin created this item")).toBeVisible();
  expect((await new AxeBuilder({ page }).include("main").analyze()).violations).toEqual([]);
});
