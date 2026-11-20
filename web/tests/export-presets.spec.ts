import { expect, test, type Page } from "@playwright/test";
import { ALL, marketing, mockSpaces, ops } from "./spaces-mock";

const project = "88888888-8888-4888-8888-888888888888";
const otherProject = "77777777-7777-4777-8777-777777777777";

// A project whose first location is `space`, so its presets live under that Space's namespace.
async function mockProjects(page: Page, space: string) {
  return mockSpaces(page, { extra: {
    [`GET /items/${project}`]: { body: { id: project, type: "project", title: "Delivery",
      locations: [{ space_id: space }], my_capabilities: ALL, assignees: [] }, headers: { ETag: '"1"' } },
    [`GET /items/${project}/children`]: { body: { items: [] } },
    [`GET /items/${otherProject}`]: { body: { id: otherProject, type: "project", title: "Other",
      locations: [{ space_id: ops.id }], my_capabilities: ALL, assignees: [] }, headers: { ETag: '"1"' } },
    [`GET /items/${otherProject}/children`]: { body: { items: [] } },
  } });
}

async function openExport(page: Page, id: string) {
  await page.goto(`/tasks/${id}`);
  await page.getByRole("button", { name: "Info" }).click();
  await page.getByRole("button", { name: "Item actions" }).click();
  await page.getByRole("menuitem", { name: "Export to Excel" }).click();
  return page.getByRole("dialog", { name: "Export to Excel" });
}

test("a preset is saved, survives reload, updates by name, and can be deleted", async ({ page }) => {
  await mockProjects(page, marketing.id);
  let dialog = await openExport(page, project);
  await dialog.getByLabel("Milestone").uncheck();
  await dialog.getByRole("textbox", { name: "Preset name" }).fill("Lean");
  await dialog.getByRole("button", { name: "Save preset" }).click();
  await expect(dialog.getByRole("combobox", { name: "Preset" })).toHaveValue("Lean");

  // Reload: the preset is still there and applies its columns.
  await page.reload();
  dialog = await openExport(page, project);
  await dialog.getByRole("combobox", { name: "Preset" }).selectOption("Lean");
  await expect(dialog.getByLabel("Milestone")).not.toBeChecked();

  // Update: same name, new scope; the old one is replaced, not duplicated.
  await dialog.getByRole("combobox", { name: "Scope" }).selectOption("active");
  await dialog.getByRole("button", { name: "Save preset" }).click();
  await page.reload();
  dialog = await openExport(page, project);
  const names = await dialog.getByRole("combobox", { name: "Preset" }).locator("option").allTextContents();
  expect(names.filter((name) => name === "Lean")).toHaveLength(1);
  await dialog.getByRole("combobox", { name: "Preset" }).selectOption("Lean");
  await expect(dialog.getByRole("combobox", { name: "Scope" })).toHaveValue("active");

  // Delete: gone after reload.
  await dialog.getByRole("button", { name: "Delete preset" }).click();
  await page.reload();
  dialog = await openExport(page, project);
  await expect(dialog.getByRole("combobox", { name: "Preset" }).locator("option", { hasText: "Lean" })).toHaveCount(0);
});

test("presets stay in their own Space: a preset saved for one workspace does not show for another", async ({ page }) => {
  await mockProjects(page, marketing.id);
  let dialog = await openExport(page, project);
  await dialog.getByRole("textbox", { name: "Preset name" }).fill("Marketing only");
  await dialog.getByRole("button", { name: "Save preset" }).click();
  await page.reload();
  dialog = await openExport(page, otherProject);
  await expect(dialog.getByRole("combobox", { name: "Preset" }).locator("option", { hasText: "Marketing only" })).toHaveCount(0);
  await page.reload();
  dialog = await openExport(page, project);
  await expect(dialog.getByRole("combobox", { name: "Preset" }).locator("option", { hasText: "Marketing only" })).toHaveCount(1);
});
