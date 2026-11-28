import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import type { Item } from "../src/lib/items";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const projectId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const taskId = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const view = "b0b0b0b0-b0b0-4b0b-8b0b-b0b0b0b0b0b0";
const ts = "2026-11-15T10:00:00Z";
const project = { id: projectId, seq: 1, version: 1, title: "Campaign", type: "project", locations: [{ space_id: marketing.id }], assignees: [],
  my_capabilities: ALL as Item["my_capabilities"], created_at: ts, updated_at: ts } as Item;
const uploader = { id: "u", name: "Ada Lovelace" };
const row = (id: string, filename: string, extra: object) => ({ id, item_id: taskId, item_title: "Banner", filename, content_type: "text/plain; charset=utf-8", size_bytes: 10240, version: 1,
  uploader, created_at: ts, path: ["Banner"], review_status: "not_requested", can_delete: true, can_upload_version: true, ...extra });

async function open(page: Page, extra: Record<string, any> = {}) {
  let rows = [row("f1", "summary.md", {}), row("l1", "Design board", { link_url: "https://example.com/board", content_type: "text/uri-list", can_upload_version: false }),
    row("r1", "locked.pdf", { can_delete: false, can_upload_version: false, item_id: projectId, path: [] })];
  const mocked = await mockSpaces(page, { extra: {
    [`GET /items/${projectId}`]: { body: project, headers: { ETag: '"1"' } }, [`GET /items/${projectId}/children`]: { body: { items: [] } },
    [`GET /items/${projectId}/views`]: { body: { items: [{ id: view, name: "Files", type: "files", is_default: true, visibility: "public", position: 0 }] } },
    [`GET /items/${projectId}/files`]: () => ({ body: { items: rows, total: rows.length, truncated: false, counts: { all: rows.length, direct: 1, subitems: rows.length - 1, projects: 0 }, projects: [] } }),
    "DELETE /attachments/f1": () => { rows = rows.filter((r) => r.id !== "f1"); return { status: 204, body: null }; },
    "POST /attachments/f1/versions": { status: 201, body: {} },
    "GET /workflows": { body: { items: [] } }, ...extra,
  } });
  await page.goto(`/tasks/${projectId}?view=${view}`);
  return mocked;
}
const table = (page: Page) => page.getByRole("table", { name: "Files" });

test("five columns, with type, size and version under the name", async ({ page }) => {
  await open(page);
  await expect(table(page).getByRole("columnheader")).toHaveText(["File", "Location", "Review", "Added", "Actions"]);
  const summary = table(page).getByRole("row", { name: /summary\.md/ });
  await expect(summary).toContainText("MD · 10 KB · v1");
  await expect(summary).toContainText("Ada Lovelace");
  await expect(summary.getByRole("link", { name: "Download summary.md" })).toHaveAttribute("href", "/api/v1/attachments/f1/download");
  await expect(table(page).getByRole("link", { name: "Open Design board" })).toHaveAttribute("href", "https://example.com/board");
  expect((await new AxeBuilder({ page }).include("[data-files-view]").analyze()).violations).toEqual([]);
});

test("the menu offers only what the row allows", async ({ page }) => {
  await open(page);
  const items = async (name: string) => {
    await table(page).getByRole("button", { name: `Actions for ${name}` }).click();
    await expect(page.getByRole("menu")).toBeVisible();
    const texts = await page.getByRole("menuitem").allInnerTexts();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("menu")).toHaveCount(0);
    return texts.map((text) => text.trim());
  };
  expect(await items("summary.md")).toEqual(["Open details", "Upload new version", "Copy link", "Delete"]);
  expect(await items("Design board")).toEqual(["Open details", "Copy link", "Delete"]);
  expect(await items("locked.pdf")).toEqual(["Copy link"]); // attached to the project being viewed, and read only
});

test("copies the address, uploads a new version and deletes after confirming", async ({ page, context }) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  const { calls } = await open(page);
  const menu = async (item: string) => { await table(page).getByRole("button", { name: "Actions for summary.md" }).click(); await page.getByRole("menuitem", { name: item }).click(); };
  await menu("Copy link");
  expect(await page.evaluate(() => navigator.clipboard.readText())).toMatch(/\/api\/v1\/attachments\/f1\/download$/);
  await page.getByLabel("New version of summary.md").setInputFiles({ name: "summary.md", mimeType: "text/markdown", buffer: Buffer.from("two") });
  await expect(page.getByText("New version uploaded")).toBeVisible();
  await menu("Delete");
  const dialog = page.getByRole("dialog", { name: "Delete summary.md?" });
  expect(calls.some((call) => call.method === "DELETE")).toBe(false);
  await dialog.getByRole("button", { name: "Delete" }).click();
  await expect(table(page).getByRole("row", { name: /summary\.md/ })).toHaveCount(0);
});

test("a refused delete is explained and the file stays", async ({ page }) => {
  await open(page, { "DELETE /attachments/f1": { status: 403, body: { type: "about:blank", title: "forbidden", status: 403, detail: "you may not delete this file" } } });
  await table(page).getByRole("button", { name: "Actions for summary.md" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Delete" }).click();
  await expect(dialog.getByRole("alert")).toContainText("you may not delete this file");
  await dialog.getByRole("button", { name: "Cancel" }).click(); // the page behind a modal is hidden from the accessibility tree
  await expect(table(page).locator("tbody tr")).toHaveCount(3);
});

test.describe("on a phone", () => {
  test.use({ viewport: { width: 390, height: 844 } });
  test("location moves under the name and nothing scrolls sideways", async ({ page }) => {
    await open(page);
    await expect(table(page).getByRole("columnheader")).toHaveText(["File", "Review", "Actions"]);
    await expect(table(page).getByRole("row", { name: /summary\.md/ }).getByRole("button", { name: "Banner" })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0);
  });
});
