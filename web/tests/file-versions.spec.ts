import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const id = "b0000000-0000-4000-8000-000000000002";
const file = "f0000000-0000-4000-8000-000000000001";
const item = { id, seq: 1, type: "task", title: "Spec", version: 1, my_capabilities: ALL, assignees: [], locations: [{ space_id: marketing.id, space_name: "Marketing" }] };
const ann = { id: "a0000000-0000-4000-8000-000000000001", name: "Ann Lee" };
const attachment = (version: number, filename: string) => ({ id: file, item_id: id, filename, content_type: "text/plain", size_bytes: 9, version, uploader: ann, created_at: "2026-11-14T10:00:00Z" });

test("file version history lists versions and uploads a new one", async ({ page }) => {
  let version = 1;
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /items/${id}`]: { body: item, headers: { ETag: '"1"' } },
    [`GET /items/${id}/children`]: { body: { items: [] } },
    [`GET /items/${id}/comments`]: { body: { items: [] } },
    [`GET /items/${id}/approvals`]: { body: { items: [] } },
    [`GET /items/${id}/attachments`]: () => ({ body: { items: [attachment(version, version === 1 ? "spec.txt" : "spec-final.txt")], max_upload_bytes: 1048576 } }),
    [`GET /attachments/${file}/versions`]: () => ({ body: { items: [
      ...(version === 2 ? [{ attachment_id: file, version: 2, filename: "spec-final.txt", content_type: "text/plain", size_bytes: 18, is_current: true, uploader: ann, created_at: "2026-11-14T11:00:00Z" }] : []),
      { attachment_id: file, version: 1, filename: "spec.txt", content_type: "text/plain", size_bytes: 9, is_current: version === 1, uploader: ann, created_at: "2026-11-14T10:00:00Z" },
    ] } }),
    [`POST /attachments/${file}/versions`]: () => { version = 2; return { status: 201, body: attachment(2, "spec-final.txt") }; },
  } });
  await page.goto(`/tasks/${id}`);
  await page.getByRole("button", { name: "v1" }).click();
  await expect(page.getByRole("list", { name: "Version history" }).getByRole("link")).toHaveCount(1);
  const chooser = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "Upload new version" }).click();
  await (await chooser).setFiles({ name: "spec-final.txt", mimeType: "text/plain", buffer: Buffer.from("final text, longer") });
  await expect.poll(() => calls.some((c) => c.method === "POST" && c.path.endsWith(`/attachments/${file}/versions`))).toBe(true);
  const history = page.getByRole("list", { name: "Version history" });
  await expect(history.getByRole("link")).toHaveCount(2);
  await expect(history.getByRole("link").first()).toHaveAttribute("href", `/api/v1/attachments/${file}/versions/2/download`);
});

test("a link attachment opens its address and has no version controls", async ({ page }) => {
  let linked = false;
  const link = { ...attachment(1, "Design doc"), content_type: "text/uri-list", link_url: "https://example.com/doc" };
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /items/${id}`]: { body: item, headers: { ETag: '"1"' } },
    [`GET /items/${id}/children`]: { body: { items: [] } },
    [`GET /items/${id}/comments`]: { body: { items: [] } },
    [`GET /items/${id}/approvals`]: { body: { items: [] } },
    [`GET /items/${id}/attachments`]: () => ({ body: { items: linked ? [link] : [], max_upload_bytes: 1048576 } }),
    [`POST /items/${id}/attachment-links`]: () => { linked = true; return { status: 201, body: link }; },
  } });
  await page.goto(`/tasks/${id}`);
  await page.getByLabel("Link address").fill("https://example.com/doc");
  await page.getByRole("button", { name: "Add link" }).click();
  await expect.poll(() => calls.some((c) => c.method === "POST" && c.path.endsWith(`/items/${id}/attachment-links`))).toBe(true);
  const anchor = page.getByRole("link", { name: "Design doc" });
  await expect(anchor).toHaveAttribute("href", "https://example.com/doc");
  await expect(page.getByRole("button", { name: "Upload new version" })).toHaveCount(0);
});
