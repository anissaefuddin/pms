import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const id = "b0000000-0000-4000-8000-000000000002";
const file = "f0000000-0000-4000-8000-000000000001";
const item = { id, seq: 1, type: "task", title: "Logo", version: 1, my_capabilities: ALL, assignees: [], locations: [{ space_id: marketing.id, space_name: "Marketing" }] };
const ann = { id: "a0000000-0000-4000-8000-000000000001", name: "Ann Lee" };
const png = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==", "base64");

test("proofing viewer pins a comment to a position and keeps versions apart", async ({ page }) => {
  const notes: Record<number, unknown[]> = { 1: [], 2: [] };
  const posted: unknown[] = [];
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /items/${id}`]: { body: item, headers: { ETag: '"1"' } },
    [`GET /items/${id}/children`]: { body: { items: [] } },
    [`GET /items/${id}/comments`]: { body: { items: [] } },
    [`GET /items/${id}/approvals`]: { body: { items: [] } },
    [`GET /items/${id}/attachments`]: { body: { items: [{ id: file, item_id: id, filename: "logo2.png", content_type: "image/png", size_bytes: 9, version: 2, uploader: ann, created_at: "2026-11-14T10:00:00Z" }], max_upload_bytes: 1048576 } },
    [`GET /attachments/${file}/versions`]: { body: { items: [2, 1].map((v) => ({ attachment_id: file, version: v, filename: `logo${v}.png`, content_type: "image/png", size_bytes: 9, is_current: v === 2, uploader: ann, created_at: "2026-11-14T10:00:00Z" })) } },
    [`GET /attachments/${file}/versions/1/preview`]: { body: { attachment_id: file, version: 1, status: "ready", note: "", pages: [{ page: 1, width: 400, height: 300 }] } },
    [`GET /attachments/${file}/versions/2/preview`]: { body: { attachment_id: file, version: 2, status: "skipped", note: "", pages: [] } },
    [`GET /attachments/${file}/versions/1/annotations`]: () => ({ body: { items: notes[1] } }),
    [`GET /attachments/${file}/versions/2/annotations`]: () => ({ body: { items: notes[2] } }),
    [`POST /attachments/${file}/versions/1/annotations`]: (input: object) => {
      posted.push(input);
      const made = { id: "c0000000-0000-4000-8000-000000000001", attachment_id: file, version: 1, resolved: false, author: ann, created_at: "2026-11-14T10:00:00Z", ...input };
      notes[1].push(made);
      return { status: 201, body: made };
    },
  } });
  await page.route(`**/api/v1/attachments/${file}/versions/1/preview/1`, (route) => route.fulfill({ contentType: "image/png", body: png }));
  await page.goto(`/tasks/${id}`);
  await page.getByRole("button", { name: "v2" }).click();
  await page.getByRole("button", { name: "Proof logo1.png" }).click();
  const surface = page.getByTestId("proof-surface").locator("img");
  await expect(surface).toBeVisible();
  const box = (await surface.boundingBox())!;
  await surface.click({ position: { x: box.width / 4, y: box.height / 2 } });
  await page.getByLabel("Pin note").fill("Move the logo left");
  await page.getByRole("button", { name: "Pin comment" }).click();
  await expect(page.getByTestId("proof-pin")).toHaveCount(1);
  await expect(page.getByRole("list", { name: "Pinned comments" })).toContainText("Move the logo left");
  expect(posted).toHaveLength(1);
  expect((posted[0] as { x: number }).x).toBeCloseTo(0.25, 1);
  expect(calls.some((c) => c.path.includes("/versions/2/annotations") && c.method === "POST")).toBe(false);
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Proof logo2.png" }).click();
  await expect(page.getByText("No preview is available for this file")).toBeVisible();
  await expect(page.getByTestId("proof-pin")).toHaveCount(0);
});
