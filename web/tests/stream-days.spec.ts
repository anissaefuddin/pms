import { expect, test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";

// Agent F (F2): Stream entries are grouped under their day, newest day first; one card per item inside a day.
const actor = { id: "11111111-1111-4111-8111-111111111111", name: "Ada Lovelace" };
const item = { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", title: "Launch prep", type: "task" };
const entry = (id: number, created_at: string) => ({ kind: "activity", id, action: "update", item, actor, changes: { title: { from: "a", to: "b" } }, snippet: null, created_at });

test("Stream groups entries by day and keeps one card per item within a day", async ({ page }) => {
  await mockSpaces(page, { extra: {
    "GET /stream": { body: { items: [entry(3, "2026-11-16T09:00:00Z"), entry(2, "2026-11-16T08:00:00Z"), entry(1, "2026-11-14T10:00:00Z")], next_cursor: null } },
  } });
  await page.goto("/stream");
  await expect(page.getByRole("heading", { name: "Stream", level: 1 })).toBeVisible();
  await expect(page.locator("section[aria-label]").filter({ has: page.getByRole("heading", { level: 2 }) })).toHaveCount(2);
  await expect(page.locator("section.divide-y")).toHaveCount(2); // one card for the item on 16 Nov, one on 14 Nov
});
