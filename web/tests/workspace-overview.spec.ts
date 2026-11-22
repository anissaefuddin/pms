import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import type { Reply, Req } from "./mock-api";
import { marketing, mockSpaces, ops } from "./spaces-mock";

// Workspace Overview (Batch B and C). Mock API only: the server rules are covered by the Go integration tests.
const project = "p0000000-0000-4000-8000-000000000001";
const task = "t0000000-0000-4000-8000-000000000001";
const loose = "t0000000-0000-4000-8000-000000000002";

const summary = {
  generated_at: "2026-11-22T04:00:00Z", timezone: "Asia/Jakarta",
  kpis: { accessible_spaces: 2, active_projects: 3, overdue_tasks: 4, completed_this_week: 5, at_risk_projects: { status: "NOT_AVAILABLE" } },
  spaces: [
    { id: marketing.id, name: "Marketing", archived: false, project_count: 2, task_count: 8, done_task_count: 4, progress_percent: 50, overdue_count: 3, health: "attention", last_activity: "2026-11-21T10:00:00Z", effective_access: "full" },
    { id: ops.id, name: "Operations", archived: false, project_count: 1, task_count: 0, done_task_count: 0, progress_percent: null, overdue_count: 0, health: "empty", last_activity: null, effective_access: "read" },
  ],
  next_cursor: "cursor-2", total: 3,
};
const secondPage = { ...summary, spaces: [{ ...summary.spaces[1], id: "99999999-9999-4999-8999-999999999999", name: "Finance" }], next_cursor: null };

const timeline = {
  timezone: "Asia/Jakarta", from: "2026-11-22", to: "2027-01-20", truncated: false,
  spaces: [{
    id: marketing.id, name: "Marketing", effective_access: "full",
    projects: [{ id: project, title: "Launch plan", due_date: "2026-11-25", status_group: "active", tasks: [{ id: task, title: "Write brief", due_date: "2026-11-24", status_group: "active" }] }],
    unassigned_tasks: [{ id: loose, title: "Book venue", due_date: "2026-12-01", status_group: "active" }],
  }],
};

const agenda = {
  timezone: "Asia/Jakarta", today: "2026-11-22", from: "2026-11-22", to: "2026-12-05", truncated: false, overdue_count: 3,
  items: [
    { id: task, title: "Write brief", type: "task", due_date: "2026-11-22", status_group: "active", space_id: marketing.id, space_name: "Marketing", overdue: false },
    { id: project, title: "Launch plan", type: "project", due_date: "2026-11-25", status_group: "active", space_id: marketing.id, space_name: "Marketing", overdue: false },
  ],
};

async function mock(page: Page) {
  const calls: Req[] = [];
  const record = (path: string, reply: (req: Req) => Reply) => ({ [`GET ${path}`]: (_b: unknown, req: Req) => { calls.push(req); return reply(req); } });
  await mockSpaces(page, { extra: {
    "GET /dashboards": { body: { items: [] } },
    ...record("/overview", (req) => ({ body: req.query.get("limit") === "100" ? summary : req.query.get("cursor") ? secondPage : summary })),
    ...record("/overview/timeline", () => ({ body: timeline })),
    ...record("/overview/agenda", () => ({ body: agenda })),
  } });
  return { calls };
}

test("summary shows KPIs, the attention list and a read-only Space link", async ({ page }) => {
  await mock(page);
  await page.goto("/dashboards");
  await page.getByRole("radio", { name: "Workspace overview" }).click();
  await expect(page.getByRole("radio", { name: "Summary" })).toBeChecked();
  await expect(page.getByText("Active projects").locator("..").getByText("3")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Needs attention" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Marketing" }).first()).toHaveAttribute("href", `/spaces/${marketing.id}`);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("spaces tab sorts and loads the next page with the server cursor", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto("/dashboards");
  await page.getByRole("radio", { name: "Workspace overview" }).click();
  await page.getByRole("radio", { name: "Spaces" }).click();
  await expect(page.getByRole("row", { name: /Operations/ })).toBeVisible();
  await page.getByRole("button", { name: "Load more" }).click();
  await expect(page.getByRole("rowheader", { name: "Finance" })).toBeVisible();
  await expect.poll(() => calls.some((c) => c.query.get("cursor") === "cursor-2")).toBe(true);
});

test("timeline groups by Space, expands a Project, and moves by one period", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto("/dashboards");
  await page.getByRole("radio", { name: "Workspace overview" }).click();
  await page.getByRole("radio", { name: "Timeline" }).click();
  await expect(page.getByRole("button", { name: /Launch plan/ })).toHaveCount(0);
  await page.getByRole("button", { name: "Marketing", exact: true }).click();
  const row = page.getByRole("button", { name: /Launch plan/ });
  await expect(row).toHaveAttribute("aria-expanded", "false");
  await row.click();
  await expect(row).toHaveAttribute("aria-expanded", "true");
  await expect(page.getByText("Write brief")).toBeVisible();
  await expect(page.getByText("Book venue")).toHaveCount(0);
  await page.getByRole("button", { name: /Tasks without a project/ }).click();
  await expect(page.getByText("Book venue")).toBeVisible();
  await page.getByRole("button", { name: "Next period" }).click();
  await expect.poll(() => calls.some((c) => c.query.get("from") === "2027-01-21")).toBe(true);
});

test("agenda groups by date, flags overdue work, and filters by type", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto("/dashboards");
  await page.getByRole("radio", { name: "Workspace overview" }).click();
  await page.getByRole("radio", { name: "Agenda" }).click();
  await expect(page.getByText("3 overdue tasks in Spaces you can see")).toBeVisible();
  await expect(page.getByRole("heading", { name: "2026-11-22" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "2026-11-25" })).toBeVisible();
  await page.getByRole("radio", { name: "Tasks" }).click();
  await expect.poll(() => calls.some((c) => c.query.get("type") === "task")).toBe(true);
  await page.getByRole("combobox", { name: /^Space/ }).selectOption(ops.id);
  await expect.poll(() => calls.some((c) => c.query.get("space_id") === ops.id)).toBe(true);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test.describe("390 px", () => {
  test.use({ viewport: { width: 390, height: 844 } });
  for (const view of ["Summary", "Spaces", "Timeline", "Agenda"]) {
    test(`${view} does not scroll the page sideways`, async ({ page }) => {
      await mock(page);
      await page.goto("/dashboards");
      await page.getByRole("radio", { name: "Workspace overview" }).click();
      await page.getByRole("radio", { name: view }).click();
      await page.waitForLoadState("networkidle");
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
      expect(overflow).toBeLessThanOrEqual(0);
    });
  }
});
