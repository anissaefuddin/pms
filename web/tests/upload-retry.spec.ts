import { expect, test } from "@playwright/test";
import type { Item } from "../src/lib/items";
import { ALL, marketing, mockSpaces } from "./spaces-mock";
import { problem } from "./mock-api";

// D-05: a failed upload shows the error and "Try again" sends the same file again.
const id = "b0000000-0000-4000-8000-000000000009";
const item = { id, seq: 1, type: "task", title: "Spec", version: 1, my_capabilities: ALL, assignees: [], locations: [{ space_id: marketing.id, space_name: "Marketing" }] } as Item;

test("a failed upload can be retried with the same file", async ({ page }) => {
  let attempts = 0;
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /items/${id}`]: { body: item, headers: { ETag: '"1"' } },
    [`GET /items/${id}/children`]: { body: { items: [] } },
    [`GET /items/${id}/comments`]: { body: { items: [] } },
    [`GET /items/${id}/approvals`]: { body: { items: [] } },
    [`GET /items/${id}/attachments`]: { body: { items: [], max_upload_bytes: 1048576 } },
    [`POST /items/${id}/attachments`]: () => {
      attempts += 1;
      return attempts === 1 ? problem(500, "temporary failure") : { status: 201, body: { id: "att", item_id: id, filename: "brief.txt", content_type: "text/plain", size_bytes: 4, version: 1, uploader: { id: "u", name: "Ann" }, created_at: "2026-11-15T10:00:00Z" } };
    },
  } });
  await page.goto(`/tasks/${id}`);
  const chooser = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "Choose file" }).click();
  await (await chooser).setFiles({ name: "brief.txt", mimeType: "text/plain", buffer: Buffer.from("test") });
  await expect(page.getByRole("alert").filter({ hasText: "Upload failed" })).toBeVisible();
  await page.getByRole("button", { name: "Try again" }).click();
  await expect.poll(() => calls.filter((c) => c.method === "POST" && c.path.endsWith("/attachments")).length).toBe(2);
});
