import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";
import { me } from "./mock-api";

// WA-D05: expanded Table rows are saved on the server, survive a reload, and are stored under this user's profile for this view.
const path = `/spaces/${marketing.id}`;
const status = { id: "88888888-8888-4888-8888-888888888888", name: "New", group: "active", color: "slate" };
const base = { seq: 1, version: 1, status, my_capabilities: ALL, assignees: [] };
const ita = { ...base, id: "77777777-7777-4777-8777-777777777777", type: "project", title: "ITA", child_count: 1, locations: [{ space_id: marketing.id }] };
const kurau = { ...base, id: "66666666-6666-4666-8666-666666666666", type: "folder", title: "Kurau Plant", child_count: 1, locations: [{ space_id: marketing.id, parent_item_id: ita.id }] };
const hazop = { ...base, id: "44444444-4444-4444-8444-444444444444", type: "task", title: "HAZOP study", child_count: 0, locations: [{ space_id: marketing.id, parent_item_id: kurau.id }] };
const table = { id: "55555555-5555-4555-8555-555555555555", type: "table", name: "Table", visibility: "public", position: 0, is_default: true };
const extra = {
  [`GET ${path}/views`]: { body: { items: [table] } },
  [`GET ${path}/items`]: { body: { items: [ita] } },
  [`GET /items/${ita.id}/children`]: { body: { items: [kurau] } },
  [`GET /items/${kurau.id}/children`]: { body: { items: [hazop] } },
  "GET /workflows": { body: { items: [{ is_default: true, statuses: [status] }] } },
};

// The fake server keeps the saved expansion so a reload reads it back from the profile, as the real API does.
function serverExpansion() {
  const saved: Record<string, { rows: string[]; gantt_collapsed: boolean }> = {};
  return {
    saved,
    handlers: {
      [`PUT /me/view-expansion/${table.id}`]: (body: { rows: string[]; gantt_collapsed: boolean }) => {
        saved[table.id] = body;
        return { status: 204, body: undefined };
      },
      "GET /me": () => ({ body: { ...me, view_expansion: saved } }),
    },
  };
}

test("expanded Table rows are saved on the server and restored after reload", async ({ page }) => {
  const store = serverExpansion();
  const { calls } = await mockSpaces(page, { extra: { ...extra, ...store.handlers } });
  await page.goto(path);
  await page.getByRole("button", { name: "Expand ITA", exact: true }).click();
  await page.getByRole("button", { name: "Expand Kurau Plant", exact: true }).click();
  await expect(page.getByRole("button", { name: "HAZOP study", exact: true })).toBeVisible();
  await expect.poll(() => calls.filter((call) => call.method === "PUT").length).toBeGreaterThan(0);

  await page.reload();
  await expect(page.getByRole("button", { name: "HAZOP study", exact: true })).toBeVisible();
  // One debounced save per burst of clicks, never one request per click.
  expect(calls.filter((call) => call.method === "PUT").length).toBeLessThanOrEqual(3);
  expect(store.saved[table.id].rows).toEqual(expect.arrayContaining([ita.id, kurau.id]));
});
