import { expect, test } from "@playwright/test";
import { problem } from "./mock-api";
import { ALL, marketing, mockSpaces, ops } from "./spaces-mock";

const id = "b0000000-0000-4000-8000-000000000001";
const project = { id: "c0000000-0000-4000-8000-000000000001", seq: 5, type: "project", title: "Rollout", version: 1, my_capabilities: ALL, locations: [{ space_id: ops.id }], assignees: [] };

function mock(page: import("@playwright/test").Page, refuse = false) {
  let item: any = { id, seq: 1, type: "task", title: "Plan", version: 1, my_capabilities: ALL, assignees: [], locations: [{ space_id: marketing.id, space_name: "Marketing" }] };
  return mockSpaces(page, { spaces: [{ ...marketing, my_capabilities: ALL }, { ...ops, my_capabilities: ALL }], extra: {
    [`GET /items/${id}`]: () => ({ body: item, headers: { ETag: `"${item.version}"` } }),
    [`GET /items/${id}/children`]: { body: { items: [] } },
    [`GET /items/${id}/comments`]: { body: { items: [] } },
    [`GET /items/${id}/attachments`]: { body: { items: [] } },
    [`GET /items/${project.id}`]: { body: project, headers: { ETag: '"1"' } },
    [`GET /spaces/${ops.id}/items`]: { body: { items: [project] } },
    [`PUT /items/${id}/locations`]: (body) => {
      if (refuse) return problem(422, "this location would put the item inside itself", "validation_failed");
      item = { ...item, locations: body.locations.map((loc: any) => ({ ...loc, space_name: loc.space_id === ops.id ? "Operations" : "Marketing" })) };
      return { body: item };
    },
  } });
}

test("lists locations, adds a project in another Space, and removes one but never the last", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto(`/tasks/${id}`);
  await page.getByRole("button", { name: /^Locations:/ }).click();
  const section = page.getByRole("region", { name: "Locations" });
  await expect(section.getByText("Marketing")).toBeVisible();
  await expect(section.getByRole("button", { name: /Remove location/ })).toBeDisabled();
  await section.getByRole("button", { name: "Add location" }).click();
  await section.getByRole("combobox", { name: "Space", exact: true }).selectOption(ops.id);
  await section.getByRole("combobox", { name: "Project", exact: true }).selectOption(project.id);
  await section.getByRole("button", { name: "Add location" }).last().click();
  await expect.poll(() => calls.find((call) => call.method === "PUT")?.body).toEqual({ locations: [
    { space_id: marketing.id, parent_item_id: null }, { space_id: ops.id, parent_item_id: project.id },
  ] });
  await expect(section.getByText("Operations / Rollout")).toBeVisible();
  await section.getByRole("button", { name: "Remove location Marketing" }).click();
  await expect.poll(() => calls.filter((call) => call.method === "PUT").at(-1)?.body).toEqual({ locations: [{ space_id: ops.id, parent_item_id: project.id }] });
  await expect(section.getByRole("listitem")).toHaveCount(1);
  await expect(section.getByRole("button", { name: /Remove location/ })).toBeDisabled();
});

test("a refused location (cycle) shows the server's message and changes nothing", async ({ page }) => {
  await mock(page, true);
  await page.goto(`/tasks/${id}`);
  await page.getByRole("button", { name: /^Locations:/ }).click();
  const section = page.getByRole("region", { name: "Locations" });
  await section.getByRole("button", { name: "Add location" }).click();
  await section.getByRole("combobox", { name: "Space", exact: true }).selectOption(ops.id);
  await section.getByRole("button", { name: "Add location" }).last().click();
  await expect(section.getByRole("alert")).toContainText("inside itself");
  await expect(section.getByRole("listitem")).toHaveCount(1);
});
