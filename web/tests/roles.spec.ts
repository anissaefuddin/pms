import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me, problem } from "./mock-api";
import { extra, grace, marketing, mockSpaces } from "./spaces-mock";

const r1 = "c0000000-0000-4000-8000-000000000001";
const role = (over: Record<string, unknown> = {}) => ({
  id: r1, name: "Reviewer", description: "Reads and comments", capabilities: ["view", "comment"], version: 3, in_use: 2,
  created_at: "2026-11-01T00:00:00Z", updated_at: "2026-11-02T00:00:00Z", ...over });

test("admin lists, creates, edits and deletes roles with If-Match, and axe is clean", async ({ page }) => {
  let roles = [role()];
  const { calls } = await mockSpaces(page, { extra: {
    "GET /custom-roles": () => ({ body: { items: roles } }),
    "POST /custom-roles": (body: any) => { roles = [...roles, role({ id: "c0000000-0000-4000-8000-000000000002", name: body.name, capabilities: body.capabilities, in_use: 0, version: 1 })]; return { status: 201, body: roles.at(-1) }; },
    [`PUT /custom-roles/${r1}`]: (body: any) => { roles = roles.map((r) => r.id === r1 ? role({ name: body.name, capabilities: body.capabilities, version: 4 }) : r); return { body: roles[0] }; },
    "DELETE /custom-roles/c0000000-0000-4000-8000-000000000002": () => { roles = roles.filter((r) => r.id !== "c0000000-0000-4000-8000-000000000002"); return { status: 204 }; },
  } });
  await page.goto("/settings/roles");
  await expect(page.getByRole("heading", { name: "Custom roles", level: 1 })).toBeVisible();
  const row = page.getByRole("row", { name: /Reviewer/ });
  await expect(row).toContainText("View, Comment");
  await expect(row).toContainText("2");
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  // create: view is locked on and cannot be removed
  await page.getByRole("button", { name: "New role" }).click();
  const form = page.getByRole("form", { name: "New role" });
  await expect(form.getByRole("checkbox", { name: /^View/ })).toBeChecked();
  await expect(form.getByRole("checkbox", { name: /^View/ })).toBeDisabled();
  await form.getByLabel("Name").fill("Triager");
  await form.getByRole("checkbox", { name: /^Change status/ }).check();
  await form.getByRole("checkbox", { name: /^Edit/ }).check();
  await form.getByRole("button", { name: "Save role" }).click();
  await expect(page.getByRole("row", { name: /Triager/ })).toContainText("View, Change status, Edit");
  expect(calls.find((c) => c.method === "POST" && c.path === "/custom-roles")!.body).toEqual({ name: "Triager", description: "", capabilities: ["view", "change_status", "edit"] });

  // edit sends the version it loaded
  await page.getByRole("button", { name: "Edit Reviewer" }).click();
  await page.getByRole("form", { name: "Edit role" }).getByRole("checkbox", { name: /^Comment/ }).uncheck();
  await page.getByRole("form", { name: "Edit role" }).getByRole("button", { name: "Save role" }).click();
  await expect.poll(() => calls.some((c) => c.method === "PUT")).toBe(true);
  const put = calls.find((c) => c.method === "PUT")!;
  expect(put.headers["if-match"]).toBe('"3"');
  expect(put.body.capabilities).toEqual(["view"]);

  // delete asks first
  await page.getByRole("button", { name: "Delete Triager" }).click();
  await expect(page.getByRole("dialog")).toContainText("only possible when no share uses it");
  await page.getByRole("button", { name: "Delete role" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.getByRole("row", { name: /Triager/ })).toHaveCount(0);
});

test("a role in use cannot be deleted, a stale edit says so, and a regular user is told it is admin only", async ({ page }) => {
  await mockSpaces(page, { extra: {
    "GET /custom-roles": { body: { items: [role()] } },
    [`DELETE /custom-roles/${r1}`]: problem(409, "the role is used by shares", "conflict"),
    [`PUT /custom-roles/${r1}`]: problem(409, "role changed", "version_conflict"),
  } });
  await page.goto("/settings/roles");
  await page.getByRole("button", { name: "Delete Reviewer" }).click();
  await page.getByRole("button", { name: "Delete role" }).click();
  await expect(page.getByRole("dialog").getByRole("alert")).toContainText("Some shares still use this role.");
  await page.getByRole("button", { name: "Cancel" }).click();
  await page.getByRole("button", { name: "Edit Reviewer" }).click();
  await page.getByRole("button", { name: "Save role" }).click();
  await expect(page.getByRole("alert")).toContainText("Someone changed this role.");
  await page.unrouteAll();
  await mockSpaces(page, { me: { ...me, user_type: "regular", capabilities: { manage_users: false, create_space: true } } as typeof me });
  await page.goto("/settings/roles");
  await expect(page.getByRole("alert")).toContainText("Only administrators can manage roles.");
  await expect(page.getByRole("link", { name: "Custom roles" })).toHaveCount(0);
});

test("the Share dialog offers custom roles, shows their names, and saves custom:<id>", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: extra({ "GET /custom-roles": { body: { items: [role()] } } }) });
  await page.goto(`/spaces/${marketing.id}`);
  await page.getByRole("button", { name: "Share" }).click();
  const dialog = page.getByRole("dialog", { name: "Share “Marketing”" });
  await dialog.getByLabel("Add people or groups").fill("gra");
  await expect(dialog.getByText("grace@example.test")).toBeVisible();
  await expect(dialog.getByLabel("Role for Grace Hopper").getByRole("option", { name: "Reviewer" })).toHaveCount(1);
  await dialog.getByLabel("Role for Grace Hopper").selectOption(`custom:${r1}`);
  await dialog.getByRole("button", { name: "Add Grace Hopper" }).click();
  await expect(dialog.getByLabel("Role for Grace Hopper")).toHaveValue(`custom:${r1}`);
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog).toBeHidden();
  const put = calls.find((c) => c.method === "PUT")!;
  expect(put.body.shares).toContainEqual({ principal_type: "user", principal_id: grace.id, role: `custom:${r1}` });
});
