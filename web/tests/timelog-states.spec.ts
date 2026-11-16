import { expect, test } from "@playwright/test";
import type { Item } from "../src/lib/items";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// Agent E (E1): the project Timelog tells apart "nothing logged yet", "no match for the filters", and "could not load".
const projectId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const view = "b0b0b0b0-b0b0-4b0b-8b0b-b0b0b0b0b0b0";
const timestamp = "2026-11-15T10:00:00Z";
const status = { id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", name: "New", color: "slate", group: "active" } as const;
const project = { id: projectId, seq: 1, version: 1, title: "Campaign", type: "project", locations: [{ space_id: marketing.id }],
  assignees: [], status, my_capabilities: ALL as Item["my_capabilities"], created_at: timestamp, updated_at: timestamp,
  progress_method: "task_count", progress_percent: null, progress_risk: null } as Item;
const timelogView = { id: view, name: "Timelog", type: "timelog", is_default: true, visibility: "public", position: 0 };

async function open(page: import("@playwright/test").Page, entriesResponse: () => { status?: number; body: unknown }) {
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /items/${projectId}`]: { body: project, headers: { ETag: '"1"' } },
    [`GET /items/${projectId}/children`]: { body: { items: [] } },
    [`GET /items/${projectId}/views`]: { body: { items: [timelogView] } },
    [`GET /items/${projectId}/time-entries`]: entriesResponse,
    "GET /workflows": { body: { items: [{ id: "wf", is_default: true, statuses: [status] }] } },
  } });
  await page.goto(`/tasks/${projectId}?view=${view}`);
  return calls;
}

test("no entries yet says so; with a filter active it says none match and offers Clear filters", async ({ page }) => {
  await open(page, () => ({ body: { items: [], next_offset: null } }));
  await expect(page.getByText("No time entries yet.", { exact: false })).toBeVisible();
  await page.getByRole("button", { name: "Filter" }).click();
  await page.getByRole("textbox", { name: "Category" }).fill("Design");
  await expect(page.getByText("No time entries match the filters.")).toBeVisible();
  await page.getByRole("button", { name: "Clear filters" }).click();
  await expect(page.getByText("No time entries yet.", { exact: false })).toBeVisible();
});

test("a failed load shows an alert with retry, and retry loads the entries", async ({ page }) => {
  let failing = true; // the query retries on its own, so the failure stays until the user presses Retry
  await open(page, () => failing
    ? { status: 500, body: { title: "Server error", status: 500, detail: "temporary" } }
    : { body: { items: [{ id: "e1", item_title: "Campaign", user_name: "Ada", date: "2026-11-15", minutes: 90, category: "Design", comment: "", billing_type: "billable", locked: false }], next_offset: null } });
  await expect(page.getByText("Time entries could not be loaded.")).toBeVisible();
  failing = false;
  await page.getByRole("button", { name: "Reload" }).click();
  await expect(page.getByRole("table", { name: "Timelog" })).toBeVisible();
});

test("E-01: the Timelog shows the server's total for all filtered entries, not only this page", async ({ page }) => {
  const row = { id: "e1", item_title: "Campaign", user_name: "Ada", date: "2026-11-15", minutes: 60, category: "Design", comment: "", billing_type: "billable", locked: false };
  await open(page, () => ({ body: { items: [row], next_offset: 1, total_minutes: 180, billable_minutes: 150, non_billable_minutes: 30 } }));
  await expect(page.getByText("Total for the filtered entries: 3:00", { exact: false })).toBeVisible();
  await expect(page.getByText("Billable: 2:30", { exact: false })).toBeVisible();
  await expect(page.getByText("On this page only", { exact: false })).toHaveCount(0);
});
