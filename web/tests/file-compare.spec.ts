import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const id = "b0000000-0000-4000-8000-000000000002";
const file = "f0000000-0000-4000-8000-000000000001";
const item = { id, seq: 1, type: "task", title: "Logo", version: 1, my_capabilities: ALL, assignees: [], locations: [{ space_id: marketing.id, space_name: "Marketing" }] };
const ann = { id: "a0000000-0000-4000-8000-000000000001", name: "Ann Lee" };
const png = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==", "base64");
const preview = (version: number, status: string) => ({ attachment_id: file, version, status, note: "", pages: status === "ready" ? [{ page: 1, width: 400, height: 300 }] : [] });

test("compare shows two versions of one file side by side", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /items/${id}`]: { body: item, headers: { ETag: '"1"' } },
    [`GET /items/${id}/children`]: { body: { items: [] } },
    [`GET /items/${id}/comments`]: { body: { items: [] } },
    [`GET /items/${id}/approvals`]: { body: { items: [] } },
    [`GET /items/${id}/attachments`]: { body: { items: [{ id: file, item_id: id, filename: "logo2.png", content_type: "image/png", size_bytes: 9, version: 2, uploader: ann, created_at: "2026-11-14T10:00:00Z" }], max_upload_bytes: 1048576 } },
    [`GET /attachments/${file}/versions`]: { body: { items: [2, 1].map((v) => ({ attachment_id: file, version: v, filename: `logo${v}.png`, content_type: "image/png", size_bytes: 9, is_current: v === 2, uploader: ann, created_at: "2026-11-14T10:00:00Z" })) } },
    [`GET /attachment-compare`]: { body: { left: preview(1, "ready"), right: preview(2, "skipped") } },
  } });
  for (const v of [1, 2]) await page.route(`**/api/v1/attachments/${file}/versions/${v}/preview/1`, (route) => route.fulfill({ contentType: "image/png", body: png }));
  await page.goto(`/tasks/${id}`);
  await page.getByRole("button", { name: "v2" }).click();
  await page.getByRole("button", { name: "Compare versions" }).click();
  const sides = page.getByTestId("compare-side");
  await expect(sides).toHaveCount(2);
  await expect(sides.nth(0).locator("img")).toBeVisible();
  await expect(sides.nth(1)).toContainText("No preview is available for this file");
  const query = calls.filter((c) => c.path.includes("attachment-compare")).at(-1)!.query;
  expect([query.get("left_version"), query.get("right_version")]).toEqual(["1", "2"]);
});
