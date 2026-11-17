import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { grace, marketing, mockSpaces } from "./spaces-mock";

const risk = { id: "11111111-2222-4333-8444-555555555555", space_id: "", name: "Risk", type: "single_select", position: 0, archived_at: null,
  options: [{ id: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", label: "Low", color: "#22c55e" }], config: {} };
const legacy = { id: "99999999-2222-4333-8444-555555555555", space_id: "", name: "Legacy cost", type: "number", position: 1,
  archived_at: "2026-11-10T00:00:00Z", options: [], config: {} };

test("admin lists Workspace fields, filters them, and opens the drawer with unsaved-change protection", async ({ page }) => {
  await mockSpaces(page, { me, extra: {
    "GET /custom-fields": () => ({ body: { items: [risk, legacy] } }),
    "GET /item-types": () => ({ body: { items: [] } }),
    "GET /groups": () => ({ body: { items: [] } }),
    "GET /users": () => ({ body: { items: [] } }),
    [`GET /custom-fields/${risk.id}/grants`]: () => ({ body: { items: [] } }),
  } });
  await page.goto("/settings/custom-fields");
  const table = page.getByRole("table");
  await expect(table.getByRole("rowheader", { name: "Risk" })).toBeVisible();
  await expect(table.getByRole("rowheader", { name: "Legacy cost" })).toBeVisible();

  await page.getByRole("searchbox", { name: "Search fields" }).fill("ris");
  await expect(table.getByRole("rowheader", { name: "Legacy cost" })).toHaveCount(0);
  await page.getByRole("searchbox", { name: "Search fields" }).fill("");
  await page.getByRole("combobox", { name: "Status" }).selectOption("archived");
  await expect(table.getByRole("rowheader", { name: "Risk" })).toHaveCount(0);

  await page.getByRole("combobox", { name: "Status" }).selectOption("all");
  await page.getByRole("button", { name: "Actions for Risk" }).click();
  await page.getByRole("menuitem", { name: "Edit" }).click();
  const drawer = page.getByRole("dialog", { name: "Edit Risk" });
  await expect(drawer.getByRole("heading", { name: "Permissions" })).toBeVisible();
  await expect(drawer.getByRole("heading", { name: "Preview" })).toBeVisible();
  await drawer.getByRole("textbox", { name: "Field name" }).fill("Risk level");
  await drawer.getByRole("button", { name: "Cancel" }).click();
  await expect(page.getByRole("alertdialog", { name: "Discard unsaved changes?" }).or(page.getByRole("dialog", { name: "Discard unsaved changes?" }))).toBeVisible();
  await page.getByRole("button", { name: "Keep editing" }).click();
  await expect(drawer).toBeVisible();
  await drawer.getByRole("button", { name: "Cancel" }).click();
  await page.getByRole("button", { name: "Discard" }).click();
  await expect(drawer).toHaveCount(0);

  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("a person without administrator rights sees their Space's fields and a clear message for Workspace fields", async ({ page }) => {
  const spaceField = { ...risk, space_id: marketing.id, name: "Budget band" };
  await mockSpaces(page, { me: { ...grace, capabilities: { manage_users: false } } as typeof me, extra: {
    [`GET /spaces/${marketing.id}/custom-fields`]: () => ({ body: { items: [spaceField] } }),
    "GET /item-types": () => ({ body: { items: [] } }),
  } });
  await page.goto("/settings/custom-fields");
  await page.getByRole("combobox", { name: "Space" }).selectOption(marketing.id);
  await expect(page.getByRole("table").getByRole("rowheader", { name: "Budget band" })).toBeVisible();
  await page.getByRole("radio", { name: "Workspace" }).click();
  await expect(page.getByRole("status").filter({ hasText: "Only administrators manage Workspace fields." })).toBeVisible();
});

test("an administrator assigns a Workspace field to a Space from the drawer; the Space shows it only when assigned", async ({ page }) => {
  let assigned = false;
  const { calls } = await mockSpaces(page, { me, extra: {
    "GET /custom-fields": () => ({ body: { items: [risk] } }),
    "GET /item-types": () => ({ body: { items: [] } }),
    "GET /groups": () => ({ body: { items: [] } }),
    [`GET /custom-fields/${risk.id}/grants`]: () => ({ body: { items: [] } }),
    [`GET /spaces/${marketing.id}/field-choices`]: () => ({ body: { items: [{ id: risk.id, name: "Risk", type: "single_select", assigned }] } }),
    [`POST /spaces/${marketing.id}/custom-fields/${risk.id}/assignments`]: () => { assigned = true; return { status: 201, body: { field_id: risk.id, space_id: marketing.id, scope_kind: "space", scope_id: marketing.id, scope_name: "Marketing" } }; },
  } });
  await page.goto("/settings/custom-fields");
  await page.getByRole("button", { name: "Actions for Risk" }).click();
  await page.getByRole("menuitem", { name: "Edit" }).click();
  const drawer = page.getByRole("dialog", { name: "Edit Risk" });
  const box = drawer.getByRole("checkbox", { name: "Marketing" });
  await expect(box).not.toBeChecked();
  await box.click();
  await expect(box).toBeChecked();
  expect(calls.find((c) => c.method === "POST" && c.path.endsWith("/assignments"))?.body).toMatchObject({ scope_kind: "space", scope_id: marketing.id });
});

test("a Space manager sets which fields apply to a project from the item menu", async ({ page }) => {
  const project = "88888888-8888-4888-8888-888888888888";
  const phase = { id: "12121212-3434-4545-8686-787878787878", space_id: marketing.id, name: "Phase", type: "text", position: 0, archived_at: null, options: [], config: {} };
  let assigned = false;
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /items/${project}`]: { body: { id: project, type: "project", title: "Delivery",
      locations: [{ space_id: marketing.id }], my_capabilities: ["view", "edit"], assignees: [] }, headers: { ETag: '"1"' } },
    [`GET /items/${project}/children`]: { body: { items: [] } },
    [`GET /spaces/${marketing.id}/custom-fields`]: () => ({ body: { items: [phase] } }),
    [`GET /spaces/${marketing.id}/field-choices`]: () => ({ body: { items: [] } }),
    [`GET /spaces/${marketing.id}/custom-fields/${phase.id}/assignments`]: () => ({ body: { items: assigned ? [{ field_id: phase.id, space_id: marketing.id, scope_kind: "project", scope_id: project, scope_name: "Delivery" }] : [] } }),
    [`POST /spaces/${marketing.id}/custom-fields/${phase.id}/assignments`]: () => { assigned = true; return { status: 201, body: {} }; },
  } });
  await page.goto(`/tasks/${project}`);
  await page.getByRole("button", { name: "Info" }).click();
  await page.getByRole("button", { name: "Item actions" }).click();
  await page.getByRole("menuitem", { name: "Custom fields…" }).click();
  const dialog = page.getByRole("dialog", { name: "Custom fields for Delivery" });
  const box = dialog.getByRole("checkbox", { name: "Phase" });
  await expect(box).not.toBeChecked();
  await box.click();
  await expect(box).toBeChecked();
  expect(calls.find((c) => c.method === "POST" && c.path.endsWith("/assignments"))?.body).toEqual({ scope_kind: "project", scope_id: project });
});

test("a Space manager adds a Workspace field to the Space from the table's Columns menu", async ({ page }) => {
  const { path, project } = await import("./gantt-mock");
  const table = { id: "44444444-4444-4444-8444-444444444441", type: "table", name: "Table", visibility: "public", position: 0, is_default: true, config: {} };
  const region = { id: "13131313-1414-4515-8616-171717171717", space_id: "", name: "Region", type: "text", position: 0, archived_at: null, options: [], config: {} };
  let assigned = false;
  const { calls } = await mockSpaces(page, { extra: {
    [`GET ${path}/views`]: () => ({ body: { items: [table] } }),
    [`GET ${path}/items`]: () => ({ body: { items: [project] } }),
    [`GET /spaces/${marketing.id}/fields`]: () => ({ body: { items: assigned ? [region] : [] } }),
    [`GET /spaces/${marketing.id}/field-choices`]: () => ({ body: { items: [{ id: region.id, name: "Region", type: "text", assigned }] } }),
    [`POST /spaces/${marketing.id}/custom-fields/${region.id}/assignments`]: () => { assigned = true; return { status: 201, body: {} }; },
  } });
  await page.goto(path);
  await page.getByRole("button", { name: "Columns", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Fields in this Space")).toBeVisible();
  await dialog.getByRole("listitem").filter({ hasText: "Region" }).getByRole("button", { name: "Add to Space" }).click();
  await expect(dialog.getByRole("listitem").filter({ hasText: "Region" }).getByRole("button", { name: "Remove from Space" })).toBeVisible();
  expect(calls.find((c) => c.method === "POST" && c.path.endsWith("/assignments"))?.body).toEqual({ scope_kind: "space", scope_id: marketing.id });
});
