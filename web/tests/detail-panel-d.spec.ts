import { expect, test } from "@playwright/test";
import type { Item } from "../src/lib/items";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// Agent D: the detail panel shows readable activity, and a failed comment keeps the draft so nothing is lost.
const projectId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const view = "b0b0b0b0-b0b0-4b0b-8b0b-b0b0b0b0b0b0";
const timestamp = "2026-11-15T10:00:00Z";
const status = { id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", name: "New", color: "slate", group: "active" } as const;
const project = { id: projectId, seq: 1, version: 1, title: "Campaign", type: "project", locations: [{ space_id: marketing.id }],
  assignees: [], status, my_capabilities: ALL as Item["my_capabilities"], created_at: timestamp, updated_at: timestamp,
  progress_method: "task_count", progress_percent: null, progress_risk: null } as Item;
const actor = { id: "11111111-1111-4111-8111-111111111111", name: "Ada Lovelace" };
const activity = {
  items: [
    { id: 2, actor, action: "update", changes: { title: { from: "Old name", to: "New name" } }, created_at: "2026-11-15T11:00:00Z" },
    { id: 1, actor, action: "create", changes: {}, created_at: "2026-11-14T09:00:00Z" },
  ],
  next_cursor: null,
};

test("activity shows changes as from → to, grouped by day", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET /items/${projectId}`]: { body: project, headers: { ETag: '"1"' } },
    [`GET /items/${projectId}/children`]: { body: { items: [] } },
    [`GET /items/${projectId}/views`]: { body: { items: [{ id: view, name: "Table", type: "table", is_default: true, visibility: "public", position: 0 }] } },
    [`GET /items/${projectId}/activity`]: { body: activity },
    [`GET /items/${projectId}/comments`]: { body: { items: [], next_cursor: null } },
    "GET /workflows": { body: { items: [{ id: "wf", is_default: true, statuses: [status] }] } },
  } });
  await page.goto(`/tasks/${projectId}?view=${view}&task=${projectId}`);
  const panel = page.getByRole("dialog");
  await expect(panel.getByText("title: Old name → New name")).toBeVisible();
  await expect(panel.locator("h4")).toHaveCount(2); // two days: 15 and 14 Nov
});

test("a failed comment keeps the draft and can be sent again", async ({ page }) => {
  let attempts = 0;
  const posted: unknown[] = [];
  await mockSpaces(page, { extra: {
    [`GET /items/${projectId}`]: { body: project, headers: { ETag: '"1"' } },
    [`GET /items/${projectId}/children`]: { body: { items: [] } },
    [`GET /items/${projectId}/views`]: { body: { items: [{ id: view, name: "Table", type: "table", is_default: true, visibility: "public", position: 0 }] } },
    [`GET /items/${projectId}/activity`]: { body: { items: [], next_cursor: null } },
    [`GET /items/${projectId}/comments`]: { body: { items: [], next_cursor: null } },
    [`POST /items/${projectId}/comments`]: (body: unknown) => {
      attempts += 1;
      if (attempts === 1) return { status: 500, body: { title: "Server error", status: 500, detail: "temporary" } };
      posted.push(body);
      return { status: 201, body: { id: 1, body, author: actor, created_at: timestamp, version: 1 } };
    },
    "GET /workflows": { body: { items: [{ id: "wf", is_default: true, statuses: [status] }] } },
  } });
  await page.goto(`/tasks/${projectId}?view=${view}&task=${projectId}`);
  const panel = page.getByRole("dialog");
  const editor = panel.getByRole("textbox", { name: "Comment" });
  await editor.click();
  await editor.fill("Keep this text");
  await panel.getByRole("button", { name: "Post comment" }).click();
  await expect(panel.getByRole("alert").first()).toBeVisible();
  await expect(editor).toContainText("Keep this text");
  await panel.getByRole("button", { name: "Post comment" }).click();
  await expect.poll(() => posted.length).toBe(1);
});
