import { test, expect } from "@playwright/test";
import { marketing, mockSpaces } from "./spaces-mock";

const members = [
  { user_id: "u1", name: "Grace Hopper", role: "editor", capped_by_user_type: false, sources: [
    { kind: "direct", role: "editor" },
    { kind: "group", name: "Design team", role: "read" },
  ] },
  { user_id: "u2", name: "Ada Lovelace", role: "full", capped_by_user_type: true, sources: [{ kind: "direct", role: "full" }] },
];

test.describe("Space settings", () => {
  test("Members & access lists who has access and why", async ({ page }) => {
    await mockSpaces(page, { extra: { [`GET /spaces/${marketing.id}/members`]: { body: { items: members } } } });
    await page.goto(`/spaces/${marketing.id}/settings/members`);
    const table = page.getByRole("table");
    await expect(table.getByRole("row", { name: /Grace Hopper/ })).toContainText("Via group Design team");
    await expect(table.getByRole("row", { name: /Ada Lovelace/ })).toContainText("Limited by user type");
    await expect(page.getByRole("button", { name: "Manage members and access" })).toBeVisible();
  });

  test("an unknown tab goes back to General", async ({ page }) => {
    await mockSpaces(page);
    await page.goto(`/spaces/${marketing.id}/settings/nope`);
    await expect(page).toHaveURL(new RegExp(`/spaces/${marketing.id}/settings/general$`));
    await expect(page.getByRole("textbox", { name: "Name" })).toHaveValue("Marketing");
  });

  test("editing the name shows unsaved changes and Cancel restores the saved name", async ({ page }) => {
    await mockSpaces(page);
    await page.goto(`/spaces/${marketing.id}/settings/general`);
    const name = page.getByRole("textbox", { name: "Name" });
    await name.fill("Growth");
    await expect(page.getByText("Unsaved changes")).toBeVisible();
    await page.getByRole("button", { name: "Cancel" }).click();
    await expect(name).toHaveValue("Marketing");
    await expect(page.getByText("All changes saved")).toBeVisible();
  });

  test("leaving with unsaved changes asks first, and Keep editing stays", async ({ page }) => {
    await mockSpaces(page);
    await page.goto(`/spaces/${marketing.id}/settings/general`);
    await page.getByRole("textbox", { name: "Name" }).fill("Growth");
    await page.getByRole("link", { name: "Members & access" }).click();
    const dialog = page.getByRole("dialog", { name: "Leave without saving?" });
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: "Keep editing" }).click();
    await expect(page).toHaveURL(new RegExp(`/settings/general$`));
  });

  test("archive asks first and sends the Space version", async ({ page }) => {
    const { calls } = await mockSpaces(page, { extra: { [`POST /spaces/${marketing.id}/archive`]: () => ({ body: { space: { ...marketing, archived: true, version: 5 }, warnings: [] } }) } });
    await page.goto(`/spaces/${marketing.id}/settings/general`);
    await page.getByRole("button", { name: "Archive Space", exact: true }).click();
    const dialog = page.getByRole("alertdialog", { name: "Archive “Marketing”?" }).or(page.getByRole("dialog", { name: "Archive “Marketing”?" }));
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: "Archive Space" }).click();
    const call = calls.find((c) => c.method === "POST" && c.path.endsWith("/archive"));
    expect(call?.headers["if-match"]).toBe('"4"');
  });

  test("the icon picker saves the chosen icon with the form", async ({ page }) => {
    const { calls } = await mockSpaces(page);
    await page.goto(`/spaces/${marketing.id}/settings/general`);
    await page.getByRole("button", { name: "Rocket" }).click();
    await page.getByRole("button", { name: "Save changes" }).click();
    const patch = calls.find((c) => c.method === "PATCH");
    expect(patch?.body).toEqual({ name: "Marketing", description: "Campaigns and launches", icon_key: "rocket" });
  });

  test("discovery lists public Spaces and Join sends the request", async ({ page }) => {
    const town = { id: "77777777-7777-4777-8777-777777777777", name: "Town hall", description: null, icon_key: null };
    const { calls } = await mockSpaces(page, { extra: {
      "GET /spaces/discover": () => ({ body: { items: [town] } }),
      [`POST /spaces/${town.id}/join`]: () => ({ body: { ...town, kind: "shared", version: 1, visibility: "public", archived: false, my_capabilities: ["view"] } }),
    } });
    await page.goto("/settings/spaces");
    await expect(page.getByText("Town hall")).toBeVisible();
    await page.getByRole("button", { name: "Join Town hall" }).click();
    expect(calls.some((c) => c.method === "POST" && c.path.endsWith(`/spaces/${town.id}/join`))).toBe(true);
  });

  test("registry filters by name and archive state, and an empty match says so", async ({ page }) => {
    const row = (id: string, name: string, archived: boolean, visibility: string) => ({
      id, name, description: null, kind: "shared", version: 1, locked: false, visibility, archived, icon_key: null, is_space_admin: false,
      my_capabilities: ["view", "edit"], created_at: "2026-01-02T00:00:00Z", last_activity: null, admins: ["Ada Lovelace"], shares: 2, projects: 3, tools: 1,
    });
    await mockSpaces(page, { extra: { "GET /spaces/registry": { body: { items: [
      row("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "Finance", false, "private"),
      row("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "Old Campaigns", true, "public"),
    ] } } } });
    await page.goto("/settings/spaces");
    await expect(page.getByRole("link", { name: "Finance" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Old Campaigns" })).toBeVisible();
    await page.getByRole("combobox", { name: "Status" }).selectOption("archived");
    await expect(page.getByRole("link", { name: "Finance" })).toHaveCount(0);
    await expect(page.getByRole("link", { name: "Old Campaigns" })).toBeVisible();
    await page.getByRole("searchbox").first().fill("zzz");
    await expect(page.getByText("No Spaces match these filters.")).toBeVisible();
    await page.getByRole("button", { name: "Clear filters" }).click();
    await expect(page.getByRole("link", { name: "Finance" })).toBeVisible();
  });

  test("Members lists direct access and removal names the group that keeps access", async ({ page }) => {
    const members = [{ user_id: "u9", name: "Grace Hopper", role: "editor", capped_by_user_type: false, sources: [
      { kind: "direct", role: "editor" }, { kind: "group", name: "Design team", role: "read" }] }];
    const shares = [{ principal_type: "user", principal_id: "u9", principal_name: "Grace Hopper", role: "editor", inherited_from: null }];
    const { calls } = await mockSpaces(page, { extra: {
      [`GET /spaces/${marketing.id}/members`]: { body: { items: members } },
      [`GET /spaces/${marketing.id}/shares`]: { body: { items: shares }, headers: { ETag: '"4"' } },
      [`PUT /spaces/${marketing.id}/shares`]: { body: { items: [] } },
      "GET /groups": { body: { items: [] } },
    } });
    await page.goto(`/spaces/${marketing.id}/settings/members`);
    await page.getByRole("button", { name: "Remove Grace Hopper" }).click();
    const dialog = page.getByRole("alertdialog", { name: "Remove Grace Hopper?" }).or(page.getByRole("dialog", { name: "Remove Grace Hopper?" }));
    await expect(dialog).toContainText("Access through Design team stays");
    await dialog.getByRole("button", { name: "Remove access" }).click();
    const put = calls.find((c) => c.method === "PUT" && c.path.endsWith("/shares"));
    expect(put?.headers["if-match"]).toBe('"4"');
    expect(put?.body).toEqual({ shares: [] });
  });

  test("workflows tab saves the Space default for tasks", async ({ page }) => {
    const { calls } = await mockSpaces(page, { extra: {
      "GET /workflows": { body: { items: [{ id: "a1111111-1111-4111-8111-111111111111", name: "Alpha flow" }] } },
      [`GET /spaces/${marketing.id}/default-workflows`]: { body: { task: null, project: null } },
      [`PUT /spaces/${marketing.id}/default-workflows`]: { body: { task: "a1111111-1111-4111-8111-111111111111", project: null } },
    } });
    await page.goto(`/spaces/${marketing.id}/settings/workflows`);
    await page.getByRole("combobox", { name: "Root task workflow" }).selectOption({ label: "Alpha flow" });
    const put = calls.find((c) => c.method === "PUT" && c.path.endsWith("/default-workflows"));
    expect(put?.body).toEqual({ task: "a1111111-1111-4111-8111-111111111111", project: null });
  });
});
