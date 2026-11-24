import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const id = "b0000000-0000-4000-8000-000000000001";
const bp = "e0000000-0000-4000-8000-000000000001";
const item = { id, seq: 1, type: "task", title: "Plan", version: 1, my_capabilities: ALL, assignees: [], locations: [{ space_id: marketing.id, space_name: "Marketing" }] };

test("save an item as a blueprint, then create items from it with a start date", async ({ page }) => {
  let saved = false;
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /items/${id}`]: { body: item, headers: { ETag: '"1"' } },
    [`GET /items/${id}/children`]: { body: { items: [] } },
    [`GET /items/${id}/comments`]: { body: { items: [] } },
    [`GET /items/${id}/attachments`]: { body: { items: [] } },
    [`GET /items/${id}/approvals`]: { body: { items: [] } },
    [`GET /spaces/${marketing.id}/blueprints`]: () => ({ body: { items: saved ? [{ id: bp, space_id: marketing.id, name: "Standard plan", item_type: "task", item_count: 3, created_at: "2026-11-13T10:00:00Z" }] : [] } }),
    [`POST /spaces/${marketing.id}/blueprints`]: () => { saved = true; return { status: 201, body: { id: bp } }; },
    [`POST /blueprints/${bp}/items`]: () => ({ status: 201, body: { ...item, id: "c0000000-0000-4000-8000-000000000009", title: "Q2 plan" }, headers: { ETag: '"1"' } }),
  } });
  await page.goto(`/tasks/${id}`);
  await page.getByRole("button", { name: "Item actions" }).click();
  await page.getByRole("menuitem", { name: "Save as blueprint" }).click();
  const dialog = page.getByRole("dialog", { name: "Save as blueprint" });
  await dialog.getByLabel("Blueprint name").fill("Standard plan");
  await dialog.getByLabel("Space").selectOption(marketing.id);
  await dialog.getByRole("button", { name: "Save blueprint" }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path.endsWith("/blueprints"))?.body)
    .toEqual({ name: "Standard plan", source_item_id: id });
  await expect(page.getByText("Blueprint saved")).toBeVisible();

  // Use it: the chosen blueprint is posted with the locations, title and start date.
  await page.getByRole("button", { name: "Add subitem" }).click();
  const create = page.getByRole("dialog", { name: "Add subitem" });
  await create.getByText("Advanced options").click();
  await create.getByRole("radio", { name: /Standard plan/ }).check();
  await create.getByLabel("Name").fill("Q2 plan");
  await create.getByRole("button", { name: "Start date" }).click();
  await page.getByRole("dialog", { name: "Start date" }).getByRole("button", { name: /Thursday.*November 12/ }).click();
  await create.getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path.endsWith(`/blueprints/${bp}/items`))?.body)
    .toEqual({ locations: [{ space_id: marketing.id, parent_item_id: id }], title: "Q2 plan", start_date: "2026-11-12" });
});
