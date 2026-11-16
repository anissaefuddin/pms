import { expect, test } from "@playwright/test";
import type { Item } from "../src/lib/items";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// Regression tests for the findings from the dev-lens QA pass (E2E-03). Not yet checked against the pre-fix code.
const timestamp = "2026-11-15T10:00:00Z";
const view = "b0b0b0b0-b0b0-4b0b-8b0b-b0b0b0b0b0b0";
const status = { id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", name: "New", color: "slate", group: "active" } as const;
const base = { version: 1, my_capabilities: ALL as Item["my_capabilities"], assignees: [], created_at: timestamp, updated_at: timestamp, locations: [{ space_id: marketing.id }] };
const folderId = "ffffffff-ffff-4fff-8fff-fffffffffff1";
const taskId = "ffffffff-ffff-4fff-8fff-fffffffffff2";
const folder = { ...base, id: folderId, seq: 1, title: "QA-E2E03 Folder", type: "folder" } as Item;
const task = { ...base, id: taskId, seq: 2, title: "QA-E2E03 Task", type: "task", status, locations: [{ space_id: marketing.id, parent_item_id: folderId }] } as Item;
const folderViews = { body: { items: [{ id: view, name: "Table", type: "table", is_default: true, visibility: "public", position: 0 }] } };

test("F1/F2/F3: folder shows its Space breadcrumb; the create footer names the folder; the KPI scope is visible without Summary", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET /items/${folderId}`]: { body: folder, headers: { ETag: '"1"' } },
    [`GET /items/${folderId}/children`]: { body: { items: [] } },
    [`GET /items/${folderId}/views`]: folderViews,
    "GET /workflows": { body: { items: [{ id: "wf", is_default: true, statuses: [status] }] } },
  } });
  await page.goto(`/tasks/${folderId}?view=${view}`);
  await expect(page.getByRole("navigation", { name: "Breadcrumb" }).getByRole("link", { name: "Marketing" })).toBeVisible(); // F2
  await page.getByRole("button", { name: "New item" }).first().click();
  await expect(page.getByRole("dialog").getByText(/Folder/).first()).toBeVisible();
  await expect(page.getByRole("dialog").getByText("Space root")).toHaveCount(0); // F3: the folder's own title is shown
  await expect(page.getByRole("dialog").getByText("QA-E2E03 Folder")).toBeVisible();
});

test("F4: with a task as parent, the Project and Folder types cannot be picked", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET /items/${taskId}`]: { body: task, headers: { ETag: '"1"' } },
    [`GET /items/${taskId}/children`]: { body: { items: [] } },
    [`GET /items/${taskId}/comments`]: { body: { items: [], next_cursor: null } },
    [`GET /items/${taskId}/approvals`]: { body: { items: [] } },
    [`GET /items/${taskId}/attachments`]: { body: { items: [], max_upload_bytes: 1048576 } },
    [`GET /items/${taskId}/activity`]: { body: { items: [], next_cursor: null } },
    [`GET /items/${taskId}/views`]: { body: { items: [] } },
    "GET /workflows": { body: { items: [{ id: "wf", is_default: true, statuses: [status] }] } },
  } });
  await page.goto(`/tasks/${taskId}`);
  await page.getByRole("button", { name: "Add subitem" }).click();
  const dialog = page.getByRole("dialog", { name: "Add subitem" });
  await expect(dialog.getByRole("radio", { name: /^Project/ })).toBeDisabled();
  await expect(dialog.getByRole("radio", { name: /^Folder/ })).toBeDisabled();
  await expect(dialog.getByRole("radio", { name: /^Task/ })).toBeChecked();
});
