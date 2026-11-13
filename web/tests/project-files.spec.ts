import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import type { Item } from "../src/lib/items";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const projectId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const taskId = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const ts = "2026-11-13T00:00:00Z";
const common = { seq: 1, version: 1, locations: [{ space_id: marketing.id }], assignees: [], my_capabilities: ALL as Item["my_capabilities"], created_at: ts, updated_at: ts };
const project = { ...common, id: projectId, title: "Campaign", type: "project" } as Item;
const task = { ...common, id: taskId, title: "Banner", type: "task", status: { id: "s1", name: "In review", color: "slate", group: "active" } } as Item;
const views = [{ id: "11111111-1111-4111-8111-111111111111", name: "Table", type: "table", is_default: true }, { id: "22222222-2222-4222-8222-222222222222", name: "Files", type: "files", is_default: false }, { id: "33333333-3333-4333-8333-333333333333", name: "Chart", type: "chart", is_default: false }];

test("project Files lists files across items and Chart counts items by status; passes axe", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET /spaces/${marketing.id}/items`]: { body: { items: [project] } },
    [`GET /spaces/${marketing.id}/views`]: { body: { items: [views[0]] } },
    [`GET /items/${projectId}`]: { body: project, headers: { ETag: '"1"' } },
    [`GET /items/${projectId}/children`]: { body: { items: [task] } },
    [`GET /items/${projectId}/views`]: { body: { items: views } },
    [`GET /items/${projectId}/files`]: { body: { items: [{ id: "f1", item_id: taskId, item_title: "Banner", filename: "banner.png", content_type: "image/png", size_bytes: 2048, uploader: { id: "u", name: "Ada Lovelace" }, created_at: ts }] } },
  } });
  await page.goto(`/spaces/${marketing.id}`);
  await page.getByRole("button", { name: "Campaign", exact: true }).click();
  await page.getByRole("button", { name: "Files", exact: true }).click();
  await expect(page.getByRole("link", { name: "banner.png" })).toHaveAttribute("href", "/api/v1/attachments/f1/download");
  await expect(page.getByRole("row", { name: /banner\.png Banner 2 KB Ada Lovelace/ })).toBeVisible();
  expect((await new AxeBuilder({ page }).include("table").analyze()).violations).toEqual([]);
  await page.getByRole("button", { name: "Chart", exact: true }).click();
  await expect(page.getByRole("img", { name: "In review: 1" })).toBeVisible();
  expect((await new AxeBuilder({ page }).include("figure").analyze()).violations).toEqual([]);
});
