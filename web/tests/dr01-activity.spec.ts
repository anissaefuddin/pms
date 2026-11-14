import { expect, test } from "@playwright/test";
import { ALL, extra, marketing, mockSpaces } from "./spaces-mock";

// DR-01: "created this item" must not be followed by a dump of internal column names; edits still name what changed.
const id = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaa01";
const status = { id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", name: "New", color: "slate", group: "active" };

test("create entry shows a plain sentence, update entry still lists changed fields", async ({ page }) => {
  const item = { id, type: "task", title: "T", seq: 1, version: 1, locations: [{ space_id: marketing.id, parent_item_id: null }], assignees: [], status, my_capabilities: ALL };
  const actor = { id: "11111111-1111-4111-8111-111111111111", name: "Ada Lovelace" };
  await mockSpaces(page, { extra: extra({
    [`GET /items/${id}`]: { headers: { ETag: '"1"' }, body: item },
    [`GET /items/${id}/children`]: { body: { items: [] } },
    [`GET /items/${id}/activity`]: { body: { items: [
      { id: "a2", action: "update", actor, created_at: "2026-11-12T10:00:00Z", changes: { title: { from: "A", to: "B" } } },
      { id: "a1", action: "create", actor, created_at: "2026-11-12T09:00:00Z", changes: { assignee_ids: {}, completed_at: {}, type_id: {} } },
    ] } },
  }) });
  await page.goto(`/tasks/${id}`);
  const log = page.getByRole("region", { name: "Activity" });
  await expect(log.getByText("created this item")).toBeVisible();
  await expect(log).not.toContainText("assignee_ids");
  await expect(log).toContainText("title");
});
