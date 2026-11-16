import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";
import { problem } from "./mock-api";

// B-02: bulk status change reports what was updated and what was not, and never repeats an update that succeeded.
const path = `/spaces/${marketing.id}`;
const tableId = "55555555-5555-4555-8555-555555555551";
const active = { id: "88888888-8888-4888-8888-888888888881", name: "New", color: "slate", group: "active" };
const done = { id: "88888888-8888-4888-8888-888888888882", name: "Completed", color: "green", group: "completed" };
const base = { seq: 1, version: 1, my_capabilities: ALL, assignees: [], locations: [{ space_id: marketing.id }], type: "task", status: active };
const items = [
  { ...base, id: "d1000000-0000-4000-8000-000000000001", title: "Alpha" },
  { ...base, id: "d1000000-0000-4000-8000-000000000002", title: "Beta" },
  { ...base, id: "d1000000-0000-4000-8000-000000000003", title: "Gamma" },
];

test("bulk status: two rows selected, one updated and one refused; the updated row is not sent again", async ({ page }) => {
  const patched: string[] = [];
  const { calls } = await mockSpaces(page, { extra: {
    [`GET ${path}/views`]: { body: { items: [{ id: tableId, type: "table", name: "Table", visibility: "public", position: 0, is_default: true, config: {} }] } },
    [`GET ${path}/items`]: { body: { items } },
    "GET /workflows": { body: { items: [{ id: "wf", is_default: true, statuses: [active, done] }] } },
    [`PATCH /items/${items[0].id}`]: () => { patched.push(items[0].id); return { body: { ...items[0], status: done, version: 2 } }; },
    [`PATCH /items/${items[1].id}`]: () => { patched.push(items[1].id); return problem(409, "version conflict"); },
  } });
  await page.goto(`${path}?view=${tableId}`);
  await page.getByRole("checkbox", { name: "Select Alpha" }).check();
  await page.getByRole("checkbox", { name: "Select Beta" }).check();
  await expect(page.getByRole("toolbar", { name: "Bulk actions" })).toContainText("2 selected");
  await page.getByRole("combobox", { name: "Set status" }).selectOption(done.id);
  await expect(page.getByRole("status").filter({ hasText: "updated" })).toContainText("1 updated, 1 not updated");
  expect(calls.filter((c) => c.method === "PATCH")).toHaveLength(2);
  expect(patched).toEqual([items[0].id, items[1].id]);
});
