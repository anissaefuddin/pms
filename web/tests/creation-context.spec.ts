import { expect, test } from "@playwright/test";
import type { Item } from "../src/lib/items";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// FND-08: a new item must be created under the container the user is in (Space root or Project), never under an unrelated parent.
const projectId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const timestamp = "2026-11-13T00:00:00Z";
const active = { id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", name: "New", color: "slate", group: "active" } as const;
const common = { seq: 1, version: 1, locations: [{ space_id: marketing.id }], assignees: [],
  my_capabilities: ALL as Item["my_capabilities"], created_at: timestamp, updated_at: timestamp };
const project: Item = { ...common, id: projectId, title: "Campaign", type: "project", progress_method: "task_count",
  progress_percent: null, progress_risk: null, status: active } as Item;
const views = { body: { items: [{ id: "b0b0b0b0-b0b0-4b0b-8b0b-b0b0b0b0b0b0", name: "Table", type: "table", is_default: true, visibility: "public", position: 0 }] } };

test("Space New item creates a top-level item in that Space", async ({ page }) => {
  const posted: Record<string, unknown>[] = [];
  await mockSpaces(page, { extra: {
    [`GET /spaces/${marketing.id}/items`]: { body: { items: [] } },
    [`GET /spaces/${marketing.id}/views`]: views,
    "GET /workflows": { body: { items: [{ is_default: true, statuses: [active] }] } },
    "POST /items": (body: Record<string, unknown>) => {
      posted.push(body);
      return { status: 201, body: { ...common, id: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", title: "New task", type: "task", status: active, ...body } };
    },
  } });
  await page.goto(`/spaces/${marketing.id}`);
  await page.getByRole("main").getByRole("button", { name: "New item" }).click();
  await page.getByRole("menuitem", { name: /^Task/ }).click();
  const dialog = page.getByRole("dialog", { name: "Create item" });
  await dialog.getByRole("radio", { name: /^Task/ }).check({ force: true });
  await dialog.getByLabel("Name").fill("Top level");
  await dialog.getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => posted.length).toBe(1);
  expect(posted[0].locations).toEqual([{ space_id: marketing.id, parent_item_id: null }]);
});

test("Add task on a Project creates the task under that Project", async ({ page }) => {
  const posted: Record<string, unknown>[] = [];
  await mockSpaces(page, { extra: {
    [`GET /items/${projectId}`]: { body: project, headers: { ETag: '"1"' } },
    [`GET /items/${projectId}/children`]: { body: { items: [] } },
    [`GET /items/${projectId}/views`]: views,
    "GET /workflows": { body: { items: [{ id: "wf", is_default: true, statuses: [active] }] } },
    "POST /items": (body: Record<string, unknown>) => {
      posted.push(body);
      return { status: 201, body: { ...common, id: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", title: "Child task", type: "task", status: active, ...body } };
    },
  } });
  await page.goto(`/tasks/${projectId}`);
  await page.getByRole("button", { name: "New task" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name").fill("Child task");
  await dialog.getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => posted.length).toBe(1);
  expect(posted[0].locations).toEqual([{ space_id: marketing.id, parent_item_id: projectId }]);
  expect(posted[0].type).toBe("task");
});
