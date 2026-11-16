import { expect, test } from "@playwright/test";
import type { Item } from "../src/lib/items";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// Agent E (E2): a link attachment has no file, so the Files view opens its URL; a file keeps its download link.
const projectId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const view = "b0b0b0b0-b0b0-4b0b-8b0b-b0b0b0b0b0b0";
const timestamp = "2026-11-15T10:00:00Z";
const status = { id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", name: "New", color: "slate", group: "active" } as const;
const project = { id: projectId, seq: 1, version: 1, title: "Campaign", type: "project", locations: [{ space_id: marketing.id }],
  assignees: [], status, my_capabilities: ALL as Item["my_capabilities"], created_at: timestamp, updated_at: timestamp,
  progress_method: "task_count", progress_percent: null, progress_risk: null } as Item;
const uploader = { id: "11111111-1111-4111-8111-111111111111", name: "Ada Lovelace" };

test("link attachments open their URL; file attachments keep the download link", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET /items/${projectId}`]: { body: project, headers: { ETag: '"1"' } },
    [`GET /items/${projectId}/children`]: { body: { items: [] } },
    [`GET /items/${projectId}/views`]: { body: { items: [{ id: view, name: "Files", type: "files", is_default: true, visibility: "public", position: 0 }] } },
    [`GET /items/${projectId}/files`]: { body: { items: [
      { id: "f1", item_id: projectId, item_title: "Campaign", filename: "brief.pdf", content_type: "application/pdf", size_bytes: 2048, version: 2, uploader, created_at: timestamp },
      { id: "l1", item_id: projectId, item_title: "Campaign", filename: "Design board", content_type: "text/uri-list", size_bytes: 0, link_url: "https://example.com/board", uploader, created_at: timestamp },
    ] } },
    "GET /workflows": { body: { items: [{ id: "wf", is_default: true, statuses: [status] }] } },
  } });
  await page.goto(`/tasks/${projectId}?view=${view}`);
  const table = page.getByRole("table", { name: "Files" });
  await expect(table.getByRole("link", { name: "Design board" })).toHaveAttribute("href", "https://example.com/board");
  await expect(table.getByRole("link", { name: "Design board" })).toHaveAttribute("target", "_blank");
  await expect(table.getByRole("link", { name: "brief.pdf" })).toHaveAttribute("href", "/api/v1/attachments/f1/download");
  await expect(table.getByRole("row", { name: /brief\.pdf application\/pdf v2/ })).toBeVisible();
});
