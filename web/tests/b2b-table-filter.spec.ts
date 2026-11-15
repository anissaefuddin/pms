import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// RT-37 (QA-D-lane4-008): the Table quick filter must also match rows nested under an expanded parent.
const path = `/spaces/${marketing.id}`;
const status = { id: "88888888-8888-4888-8888-888888888888", name: "New", group: "active", color: "slate" };
const parent = { id: "77777777-7777-4777-8777-777777777777", seq: 1, type: "project", title: "ITA", version: 1, status, my_capabilities: ALL, locations: [{ space_id: marketing.id }], assignees: [], child_count: 2 };
const other = { ...parent, id: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", seq: 2, title: "Checklist", child_count: 0 };
const child = (id: string, title: string) => ({ ...parent, id, title, type: "task", child_count: 0, locations: [{ space_id: marketing.id, parent_item_id: parent.id }] });
const table = { id: "55555555-5555-4555-8555-555555555555", type: "table", name: "Table", visibility: "public", position: 0, is_default: true };

test("filter keeps nested matches and their parents, and hides the rest", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET ${path}/views`]: { body: { items: [table] } },
    [`GET ${path}/items`]: { body: { items: [parent, other] } },
    [`GET /items/${parent.id}/children`]: { body: { items: [child("c1", "HAZOP study"), child("c2", "Kick off")] } },
    "GET /workflows": { body: { items: [{ is_default: true, statuses: [status] }] } },
  } });
  await page.goto(path);
  await page.getByRole("button", { name: "Expand ITA", exact: true }).click();
  await expect(page.getByRole("button", { name: "HAZOP study", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Filter", exact: true }).click();
  await page.getByRole("searchbox").or(page.getByPlaceholder(/filter|search/i)).first().fill("HAZOP");
  await expect(page.getByRole("button", { name: "HAZOP study", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "ITA", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Kick off", exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Checklist", exact: true })).toHaveCount(0);
  await expect(page.getByText("No matching items")).toHaveCount(0);
});
