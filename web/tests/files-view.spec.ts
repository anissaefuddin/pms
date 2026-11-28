import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import type { Item } from "../src/lib/items";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const folderId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const alphaId = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const viewId = "b0b0b0b0-b0b0-4b0b-8b0b-b0b0b0b0b0b0";
const ts = "2026-11-15T10:00:00Z";
const folder = { id: folderId, seq: 1, version: 1, title: "Asset Integrity", type: "folder", locations: [{ space_id: marketing.id }], assignees: [],
  my_capabilities: ALL as Item["my_capabilities"], created_at: ts, updated_at: ts } as Item;
const uploader = { id: "u", name: "Ada Lovelace" };
const file = (id: string, filename: string, path: string[], extra: object = {}) => ({ id, item_id: path.length ? alphaId : folderId,
  item_title: path.at(-1) ?? "Asset Integrity", filename, content_type: "application/pdf", size_bytes: 2048, version: 1, review_status: "not_requested", uploader, created_at: ts, path, ...extra });
const rows = [file("f1", "Standard.pdf", []), file("f2", "Charter.pdf", ["Alpha", "Data Collection"])];

async function open(page: Page, item: Item = folder, over: Record<string, any> = {}) {
  const list = (items: unknown[], total = items.length) => ({ items, total, truncated: false, counts: { all: 128, direct: 12, subitems: 116, projects: 1 }, projects: [{ id: alphaId, title: "Alpha", files: 116 }] });
  const mocked = await mockSpaces(page, { extra: {
    [`GET /items/${folderId}`]: { body: item, headers: { ETag: '"1"' } },
    [`GET /items/${folderId}/children`]: { body: { items: [] } },
    [`GET /items/${folderId}/views`]: { body: { items: [{ id: viewId, name: "Files", type: "files", is_default: true, visibility: "public", position: 0 }] } },
    [`GET /items/${folderId}/files`]: (_body: unknown, req: any) => {
      const query = req.query as URLSearchParams;
      if (query.get("q") === "nothing") return { body: list([], 0) };
      if (query.get("scope") === "direct") return { body: list([rows[0]], 12) };
      return { body: list(query.get("offset") === "50" ? [rows[1]] : rows, 128) };
    },
    [`GET /items/${folderId}/pydio-reference`]: { body: { state: "linked", open_url: "https://dms.example.com/projects/assets" } },
    "GET /workflows": { body: { items: [] } },
    ...over,
  } });
  await page.goto(`/tasks/${folderId}?view=${viewId}`);
  return mocked;
}
const queries = (calls: { path: string; query: URLSearchParams }[]) => calls.filter((c) => c.path === `/items/${folderId}/files`).map((c) => c.query.toString());

test("a folder shows counts, locations and the DMS link", async ({ page }) => {
  await open(page);
  const summary = page.getByRole("group", { name: "File summary" });
  await expect(summary).toContainText("Total documents128");
  await expect(summary).toContainText("Direct files12");
  await expect(summary).toContainText("Files in subitems116");
  await expect(summary).toContainText("Projects with files1");
  const table = page.getByRole("table", { name: "Files" });
  await expect(table.getByRole("row", { name: /Standard\.pdf/ })).toContainText("This folder");
  await expect(table.getByRole("button", { name: "Alpha › Data Collection" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Open folder in Pydio" })).toHaveAttribute("href", "https://dms.example.com/projects/assets");
  expect((await new AxeBuilder({ page }).include("[data-files-view]").analyze()).violations).toEqual([]);
});

test("search, tabs, filters and pages are asked of the server", async ({ page }) => {
  const { calls } = await open(page);
  await expect(page.getByText("Showing 1–2 of 128")).toBeVisible();
  await page.getByRole("button", { name: "Next page" }).click();
  await expect(page.getByText("Showing 51–51 of 128")).toBeVisible();
  await page.getByRole("button", { name: "Direct (12)" }).click();
  await expect(page.getByRole("button", { name: "Direct (12)" })).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByText("Showing 1–1 of 12")).toBeVisible();
  await page.getByRole("combobox", { name: "Project" }).selectOption(alphaId);
  await page.getByRole("combobox", { name: "Type" }).selectOption("file");
  await page.getByRole("combobox", { name: "Review" }).selectOption("approved");
  await page.getByRole("searchbox", { name: "Search files" }).fill("nothing");
  await expect(page.getByText("No files match.")).toBeVisible();
  expect(queries(calls)[0]).toBe("limit=50&offset=0");
  expect(queries(calls)).toContain("limit=50&offset=50");
  expect(queries(calls).at(-1)).toBe(`limit=50&offset=0&q=nothing&scope=direct&kind=file&review=approved&project=${alphaId}`);
  await page.getByRole("button", { name: "Clear filters" }).click();
  await expect(page.getByText("Showing 1–2 of 128")).toBeVisible();
});

test("adds a link and uploads a file to this folder", async ({ page }) => {
  const { calls } = await open(page, folder, {
    [`POST /items/${folderId}/attachment-links`]: { status: 201, body: {} },
    [`POST /items/${folderId}/attachments`]: { status: 413, body: { type: "about:blank", title: "too large", status: 413, detail: "attachment exceeds upload limit" } },
  });
  await page.getByRole("button", { name: "Add file" }).click();
  await page.getByRole("menuitem", { name: /Add external link/ }).click();
  const dialog = page.getByRole("dialog", { name: "Add external link" });
  await expect(dialog).toContainText("Added to this folder");
  await dialog.getByLabel("Link address").fill("https://example.com/spec");
  await dialog.getByRole("button", { name: "Add link" }).click();
  await expect(dialog).toBeHidden();
  expect(calls.find((c) => c.path === `/items/${folderId}/attachment-links`)?.body).toEqual({ url: "https://example.com/spec" });
  await page.getByLabel("Choose a file to add").setInputFiles({ name: "big.pdf", mimeType: "application/pdf", buffer: Buffer.from("x") });
  await expect(page.getByRole("alert")).toContainText("attachment exceeds upload limit");
});

test("a reader gets no Add file, and an empty folder says so", async ({ page }) => {
  await open(page, { ...folder, my_capabilities: ["view"] } as Item, {
    [`GET /items/${folderId}/files`]: { body: { items: [], total: 0, truncated: false, counts: { all: 0, direct: 0, subitems: 0, projects: 0 }, projects: [] } },
    [`GET /items/${folderId}/pydio-reference`]: { body: { state: "not_configured" } },
  });
  await expect(page.getByRole("heading", { name: "No files here yet" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Add file" })).toHaveCount(0);
  await expect(page.getByRole("link", { name: "Open folder in Pydio" })).toHaveCount(0);
});

test("a Space lists the files of its items without Add file or direct files", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /spaces/${marketing.id}/views`]: { body: { items: [{ id: viewId, name: "Files", type: "files", is_default: true, visibility: "public", position: 0 }] } },
    [`GET /spaces/${marketing.id}/items`]: { body: { items: [folder] } },
    [`GET /spaces/${marketing.id}/files`]: { body: { items: [file("f2", "Charter.pdf", ["Asset Integrity", "Alpha"])], total: 1, truncated: false,
      counts: { all: 1, direct: 0, subitems: 1, projects: 1 }, projects: [{ id: alphaId, title: "Alpha", files: 1 }] } },
    "GET /workflows": { body: { items: [] } },
  } });
  await page.goto(`/spaces/${marketing.id}?view=${viewId}`);
  const summary = page.getByRole("group", { name: "File summary" });
  await expect(summary).toContainText("Total documents1");
  await expect(summary).toContainText("Projects with files1");
  await expect(summary).not.toContainText("Direct files");
  await expect(page.getByRole("button", { name: "Asset Integrity › Alpha" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Add file" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: /^Direct/ })).toHaveCount(0);
  expect(calls.some((c) => c.path.endsWith("/pydio-reference"))).toBe(false);
  expect((await new AxeBuilder({ page }).include("[data-files-view]").analyze()).violations).toEqual([]);
});

test("a Space offers the Files view when adding one", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET /spaces/${marketing.id}/views`]: { body: { items: [{ id: viewId, name: "Table", type: "table", is_default: true, visibility: "public", position: 0 }] } },
  } });
  await page.goto(`/spaces/${marketing.id}`);
  await page.getByRole("button", { name: "Add view" }).click();
  const types = page.getByRole("group", { name: /view type/i });
  await expect(types.getByRole("button", { name: /^Files/ })).toBeEnabled();
  await expect(types.getByRole("button", { name: /^Chart/ })).toBeDisabled();
});

test.describe("on a phone", () => {
  test.use({ viewport: { width: 390, height: 844 } });
  test("the files view does not scroll the page sideways", async ({ page }) => {
    await open(page);
    await expect(page.getByRole("table", { name: "Files" })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0);
  });
});
