import { expect, test } from "@playwright/test";
import type { Item } from "../src/lib/items";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// FND-03: the project header shows a compact strip by default; "Summary" opens the KPI cards without leaving the view.
const projectId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const timestamp = "2026-11-13T00:00:00Z";
const common = { seq: 1, version: 1, locations: [{ space_id: marketing.id }], assignees: [],
  my_capabilities: ALL as Item["my_capabilities"], created_at: timestamp, updated_at: timestamp };
const active = { id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", name: "In Progress", color: "blue", group: "active" } as const;
const completed = { id: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", name: "Completed", color: "green", group: "completed" } as const;
const project: Item = { ...common, id: projectId, title: "Campaign", type: "project", progress_method: "task_count", progress_percent: null, progress_risk: null,
  status: active } as Item;
const children = [
  { ...common, id: "c1c1c1c1-c1c1-4c1c-8c1c-c1c1c1c1c1c1", title: "Brief", type: "task", status: completed },
  { ...common, id: "c2c2c2c2-c2c2-4c2c-8c2c-c2c2c2c2c2c2", title: "Launch", type: "task", status: active },
];

test("project header: compact strip by default, Summary expands KPI cards and keeps the view", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET /items/${projectId}`]: { body: project, headers: { ETag: `"1"` } },
    [`GET /items/${projectId}/children`]: { body: { items: children } },
    [`GET /items/${projectId}/views`]: { body: { items: [{ id: 'b0b0b0b0-b0b0-4b0b-8b0b-b0b0b0b0b0b0', name: 'Table', type: 'table', is_default: true }] } },
    "GET /workflows": { body: { items: [{ id: "wf", is_default: true, statuses: [active, completed] }] } },
  } });
  await page.goto(`/tasks/${projectId}?view=b0b0b0b0-b0b0-4b0b-8b0b-b0b0b0b0b0b0`);
  const summary = page.getByRole("button", { name: "Summary" });
  await expect(summary).toHaveAttribute("aria-expanded", "false");
  await expect(page.getByText("Work items", { exact: true })).toBeVisible();
  await summary.click();
  await expect(summary).toHaveAttribute("aria-expanded", "true");
  await expect(page.locator("#" + (await summary.getAttribute("aria-controls"))!).getByText("Work items", { exact: true })).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`view=b0b0b0b0-b0b0-4b0b-8b0b-b0b0b0b0b0b0`));
});
