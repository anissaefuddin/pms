import { expect, test } from "@playwright/test";
import type { Item } from "../src/lib/items";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// F-01 / WA-D03 / WA-D02: the project KPIs come from the server tally (whole subtree, milestones excluded).
// Six New items and no In progress items: Work items 6, In progress 0.
const projectId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const view = "b0b0b0b0-b0b0-4b0b-8b0b-b0b0b0b0b0b0";
const timestamp = "2026-11-15T10:00:00Z";
const newStatus = { id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", name: "New", color: "slate", group: "active" } as const;
const inProgress = { id: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", name: "In Progress", color: "blue", group: "active" } as const;
const project = { id: projectId, seq: 1, version: 1, title: "Campaign", type: "project", locations: [{ space_id: marketing.id }],
  assignees: [], status: newStatus, my_capabilities: ALL as Item["my_capabilities"], created_at: timestamp, updated_at: timestamp,
  progress_method: "task_count", progress_percent: null, progress_risk: null } as Item;

test("project KPIs use the server tally: six New gives Work items 6 and In progress 0", async ({ page }) => {
  const bodies: unknown[] = [];
  await mockSpaces(page, { extra: {
    [`GET /items/${projectId}`]: { body: project, headers: { ETag: '"1"' } },
    [`GET /items/${projectId}/children`]: { body: { items: [] } },
    [`GET /items/${projectId}/views`]: { body: { items: [{ id: view, name: "Table", type: "table", is_default: true, visibility: "public", position: 0 }] } },
    "GET /workflows": { body: { items: [{ id: "wf", is_default: true, statuses: [newStatus, inProgress] }] } },
    "POST /analytics/query": (body: unknown) => {
      bodies.push(body);
      return { body: { total: 6, rows: [{ group: { key: newStatus.id, label: "New" }, break: null, value: 6 }] } };
    },
  } });
  await page.goto(`/tasks/${projectId}?view=${view}`);
  await page.getByRole("button", { name: "Summary" }).click();
  const workItems = page.getByRole("region", { name: "Work items" }).or(page.locator("div.rounded-lg").filter({ hasText: "Work items" }).first());
  await expect(workItems.getByText("6", { exact: true })).toBeVisible();
  await expect(page.locator("div.rounded-lg").filter({ hasText: "In progress" }).first().getByText("0", { exact: true })).toBeVisible();
  await expect.poll(() => bodies.length).toBeGreaterThan(0);
  expect(bodies[0]).toMatchObject({ reporting_on: "tasks", metric: "count", group_by: "status_id", container_ids: [projectId], filters: { exclude_milestones: true } });
});
