import { expect, test } from "@playwright/test";
import type { Item } from "../src/lib/items";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// WA-D01: the folder page shows its title once (in the WorkspaceHeader). Info opens the detail panel, which keeps the view and the title editor.
const folderId = "ffffffff-ffff-4fff-8fff-ffffffffffff";
const timestamp = "2026-11-13T00:00:00Z";
const view = "b0b0b0b0-b0b0-4b0b-8b0b-b0b0b0b0b0b0";
const folder: Item = { id: folderId, seq: 1, version: 1, title: "Workstream A", type: "folder",
  locations: [{ space_id: marketing.id }], assignees: [], my_capabilities: ALL as Item["my_capabilities"],
  created_at: timestamp, updated_at: timestamp } as Item;

test("folder page: one primary title, Info opens the panel and keeps the view", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET /items/${folderId}`]: { body: folder, headers: { ETag: '"1"' } },
    [`GET /items/${folderId}/children`]: { body: { items: [] } },
    [`GET /items/${folderId}/views`]: { body: { items: [{ id: view, name: "Table", type: "table", is_default: true, visibility: "public", position: 0 }] } },
    [`GET /items/${folderId}/shares`]: { body: { items: [] }, headers: { ETag: '"1"' } },
  } });
  await page.goto(`/tasks/${folderId}?view=${view}`);
  await expect(page.getByRole("heading", { name: "Workstream A", level: 1 })).toHaveCount(1);
  await expect(page.getByRole("heading", { name: "Workstream A", level: 2 })).toHaveCount(0);
  await page.getByRole("button", { name: "Info" }).click();
  await expect(page).toHaveURL(new RegExp(`task=${folderId}`));
  await expect(page).toHaveURL(new RegExp(`view=${view}`));
  const panel = page.getByRole("dialog");
  await expect(panel.getByRole("textbox", { name: "Name" })).toHaveValue("Workstream A");
  await page.keyboard.press("Escape");
  await expect(panel).toHaveCount(0);
  await expect(page).not.toHaveURL(new RegExp(`task=${folderId}`));
  await expect(page).toHaveURL(new RegExp(`view=${view}`));
});
