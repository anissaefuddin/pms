import { expect, test } from "@playwright/test";
import type { Item } from "../src/lib/items";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// Phase 3 regression (QA-P3-01, QA-P3-02): the project's calendar and milestone KPI must read items inside folders.
const projectId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const folderId = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const calendarView = "b1b1b1b1-b1b1-4b1b-8b1b-b1b1b1b1b1b1";
const boardView = "b2b2b2b2-b2b2-4b2b-8b2b-b2b2b2b2b2b2";
const tableView = "b0b0b0b0-b0b0-4b0b-8b0b-b0b0b0b0b0b0";
const timestamp = "2026-11-15T10:00:00Z";
const newStatus = { id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", name: "New", color: "slate", group: "active" } as const;
const task = { id: "11111111-1111-4111-8111-111111111111", seq: 2, version: 1, title: "Calendar task inside folder", type: "task",
  locations: [{ space_id: marketing.id, parent_item_id: folderId }], assignees: [], status: newStatus, my_capabilities: ALL as Item["my_capabilities"],
  start_date: "2026-11-16", due_date: "2026-11-18", created_at: timestamp, updated_at: timestamp } as Item;
const milestone = { id: "22222222-2222-4222-8222-222222222222", seq: 3, version: 1, title: "Nested milestone", type: "task", is_milestone: true,
  locations: [{ space_id: marketing.id, parent_item_id: folderId }], assignees: [], status: newStatus, my_capabilities: ALL as Item["my_capabilities"],
  due_date: "2026-12-01", created_at: timestamp, updated_at: timestamp } as Item;
const folder = { id: folderId, seq: 1, version: 1, title: "Workstream", type: "folder", locations: [{ space_id: marketing.id, parent_item_id: projectId }],
  assignees: [], status: newStatus, my_capabilities: ALL as Item["my_capabilities"], created_at: timestamp, updated_at: timestamp } as Item;
const project = { id: projectId, seq: 1, version: 1, title: "Campaign", type: "project", locations: [{ space_id: marketing.id }],
  assignees: [], status: newStatus, my_capabilities: ALL as Item["my_capabilities"], created_at: timestamp, updated_at: timestamp,
  progress_method: "task_count", progress_percent: null, progress_risk: null } as Item;

function mocks() {
  return {
    [`GET /items/${projectId}`]: { body: project, headers: { ETag: '"1"' } },
    [`GET /items/${projectId}/children`]: { body: { items: [folder] } },
    [`GET /items/${folderId}/children`]: { body: { items: [task, milestone] } },
    [`GET /items/${projectId}/views`]: { body: { items: [
      { id: tableView, name: "Table", type: "table", is_default: true, visibility: "public", position: 0 },
      { id: calendarView, name: "Calendar", type: "calendar", is_default: false, visibility: "public", position: 1 },
      { id: boardView, name: "Board", type: "board", is_default: false, visibility: "public", position: 2 },
    ] } },
    "GET /workflows": { body: { items: [{ id: "wf", is_default: true, statuses: [newStatus] }] } },
    "POST /analytics/query": { body: { total: 1, rows: [{ group: { key: newStatus.id, label: "New" }, break: null, value: 1 }] } },
  };
}

test("QA-P3-01: a project calendar shows tasks that sit inside its folders", async ({ page }) => {
  await mockSpaces(page, { extra: mocks() });
  await page.goto(`/tasks/${projectId}?view=${calendarView}`);
  await expect(page.getByText("Calendar task inside folder").first()).toBeVisible();
});

test("QA-P3-02: upcoming milestones count milestones inside folders", async ({ page }) => {
  await mockSpaces(page, { extra: mocks() });
  await page.goto(`/tasks/${projectId}?view=${tableView}`);
  await expect(page.locator("span.tabular-nums").filter({ hasText: "Upcoming milestones" })).toContainText("1");
});

test("QA-P3-03: a project board shows tasks inside folders, and folders are not cards", async ({ page }) => {
  await mockSpaces(page, { extra: mocks() });
  await page.goto(`/tasks/${projectId}?view=${boardView}`);
  await expect(page.getByText("Calendar task inside folder").first()).toBeVisible();
  await expect(page.getByText("Workstream", { exact: true })).toHaveCount(0);
});
