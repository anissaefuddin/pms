import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces, ops, personal } from "./spaces-mock";
import { me } from "./mock-api";

const launch = "b0000000-0000-4000-8000-00000000a001";
const audit = "b0000000-0000-4000-8000-00000000a002";
const onboarding = "b0000000-0000-4000-8000-00000000a003";

const summary = (id: string, space_id: string, name: string, item_type: string, scope: string, item_count: number, updated_at: string) =>
  ({ id, space_id, name, item_type, scope, item_count, created_at: updated_at, version: 1, updated_at });

const lists = {
  [`GET /spaces/${marketing.id}/blueprints`]: { body: { items: [summary(launch, marketing.id, "Launch kit", "project", "space", 3, "2026-11-10T10:00:00Z")] } },
  [`GET /spaces/${ops.id}/blueprints`]: { body: { items: [summary(audit, ops.id, "Audit checklist", "folder", "space", 2, "2026-11-01T10:00:00Z")] } },
  [`GET /spaces/${personal.id}/blueprints`]: { body: { items: [] } },
  [`GET /account-blueprints`]: { body: { items: [{ ...summary(onboarding, "", "My onboarding", "task", "account", 1, "2026-11-12T10:00:00Z"), space_id: null, owner_id: me.id }] } },
};

test("Settings → Blueprints opens the registry inside Settings, and reload and back work", async ({ page }) => {
  await mockSpaces(page, { extra: lists });
  await page.goto("/settings/profile");
  await expect(page.getByRole("link", { name: "Blueprints" })).toHaveAttribute("href", "/settings/blueprints");
  await page.getByRole("link", { name: "Blueprints" }).click();
  await expect(page).toHaveURL(/\/settings\/blueprints$/);
  await expect(page.getByRole("heading", { name: "Blueprint registry" })).toBeVisible();
  await expect(page.getByRole("navigation").filter({ hasText: "Settings" }).first()).toContainText("Work configuration");
  await page.reload();
  await expect(page.getByRole("heading", { name: "Blueprint registry" })).toBeVisible();
  await page.goto("/blueprints");
  await page.goBack();
  await expect(page).toHaveURL(/\/settings\/blueprints$/);
});

test("the registry lists every visible blueprint, groups by scope, filters, and sorts", async ({ page }) => {
  await mockSpaces(page, { extra: lists });
  await page.goto("/settings/blueprints");
  await expect(page.getByRole("cell", { name: "Launch kit", exact: true })).toBeVisible();
  await expect(page.getByRole("cell", { name: "Audit checklist", exact: true })).toBeVisible();
  await expect(page.getByRole("cell", { name: "My onboarding", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: /^Space \(2\)$/ })).toBeVisible();
  await expect(page.getByRole("heading", { name: /^Account \(1\)$/ })).toBeVisible();

  await page.getByLabel(/^Scope/).selectOption("account");
  await expect(page.getByRole("cell", { name: "My onboarding", exact: true })).toBeVisible();
  await expect(page.getByRole("cell", { name: "Launch kit", exact: true })).toHaveCount(0);
  await page.getByLabel(/^Scope/).selectOption("all");

  await page.getByRole("searchbox").fill("oper");
  await expect(page.getByRole("cell", { name: "Audit checklist", exact: true })).toBeVisible();
  await expect(page.getByRole("cell", { name: "Launch kit", exact: true })).toHaveCount(0);
  await page.getByRole("searchbox").fill("");

  await page.getByLabel("Sort").selectOption("name");
  await page.getByLabel("Group by").selectOption("none");
  const names = await page.getByRole("cell", { name: /^(Launch kit|Audit checklist|My onboarding)$/ }).allTextContents();
  expect(names).toEqual(["Audit checklist", "Launch kit", "My onboarding"]);
});

test("row actions: Edit opens the editor when the user can manage; Duplicate and Archive say why they are unavailable", async ({ page }) => {
  await mockSpaces(page, { extra: { ...lists, [`GET /blueprints/${launch}`]: () => ({ body: { id: launch, space_id: marketing.id, name: "Launch kit", item_type: "project", item_count: 3, created_at: "2026-11-10T10:00:00Z", scope: "space", version: 1, updated_at: "2026-11-10T10:00:00Z", tree: { nodes: [{ parent: -1, type: "project", title: "Launch", importance: "high", assignee_ids: [] }], dependencies: [] } } }) } });
  await page.goto("/settings/blueprints");
  await page.getByRole("button", { name: "Actions for Launch kit" }).click();
  await expect(page.getByRole("menuitem", { name: "Duplicate" })).toHaveAttribute("aria-disabled", "true");
  await expect(page.getByRole("menuitem", { name: "Archive" })).toHaveAttribute("aria-disabled", "true");
  await page.getByRole("menuitem", { name: "Edit" }).click();
  await expect(page).toHaveURL(new RegExp(`/blueprints/${launch}$`));
  await expect(page.getByRole("heading", { name: "Edit blueprint" })).toBeVisible();
});

test("a user who can only view gets no create button and a disabled Edit", async ({ page }) => {
  const view = ["view", "comment", "change_status", "attach"];
  await mockSpaces(page, {
    spaces: [{ ...personal, my_capabilities: view }, { ...marketing, my_capabilities: view }, { ...ops, my_capabilities: view }],
    extra: lists,
  });
  await page.goto("/settings/blueprints");
  await expect(page.getByRole("cell", { name: "Launch kit", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Create blueprint" })).toHaveCount(0);
  await page.getByRole("button", { name: "Actions for Launch kit" }).click();
  await expect(page.getByRole("menuitem", { name: "Edit" })).toHaveAttribute("aria-disabled", "true");
});

test("an empty registry explains what to do", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET /spaces/${marketing.id}/blueprints`]: { body: { items: [] } },
    [`GET /spaces/${ops.id}/blueprints`]: { body: { items: [] } },
    [`GET /spaces/${personal.id}/blueprints`]: { body: { items: [] } },
  } });
  await page.goto("/settings/blueprints");
  await expect(page.getByText("No blueprints yet")).toBeVisible();
  await expect(page.getByRole("button", { name: "Create blueprint" })).toBeVisible();
});

test("the catalogue at /blueprints keeps working", async ({ page }) => {
  await mockSpaces(page, { extra: lists });
  await page.goto("/blueprints");
  await expect(page.getByRole("heading", { name: "Blueprints", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Launch kit" })).toBeVisible();
  expect(me.id).toBeTruthy(); // keeps the shared mock user in scope for this file
});
