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
    nodes: [
      { id: project, title: "Launch plan", type: "project", due_date: "2026-11-25", status_group: "active", in_range: true,
        children: [{ id: task, title: "Write brief", type: "task", due_date: "2026-11-24", status_group: "active", in_range: true, children: [] }] },
      { id: loose, title: "Book venue", type: "task", due_date: "2026-12-01", status_group: "active", in_range: true, children: [] },
    ],
  }],
};
const me = "11111111-1111-4111-8111-111111111111";
const tasksBody = {
  timezone: "Asia/Jakarta", today: "2026-11-22", truncated: false,
  items: [
    { id: task, title: "Write brief", due_date: "2026-11-24", start_date: null, status_group: "active", status_name: "In progress", completed_at: null, space_id: marketing.id, space_name: "Marketing", overdue: false, assignees: [{ id: me, name: "Ada" }] },
    { id: loose, title: "Book venue", due_date: "2026-11-01", start_date: null, status_group: "active", status_name: "New", completed_at: null, space_id: ops.id, space_name: "Operations", overdue: true, assignees: [] },
  ],
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
    ...record("/overview/tasks", () => ({ body: tasksBody })),
  } });
  return { calls };
}

test("summary shows KPIs, the attention list and a read-only Space link", async ({ page }) => {
  await mock(page);
  await page.goto("/dashboards");
  await expect(page.getByRole("radio", { name: "Summary" })).toBeChecked();
  await expect(page.getByText("Active projects").locator("..").getByText("3")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Needs attention" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Marketing" }).first()).toHaveAttribute("href", `/spaces/${marketing.id}`);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("spaces tab sorts and loads the next page with the server cursor", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto("/dashboards");
  await page.getByRole("radio", { name: "Spaces" }).click();
  await expect(page.getByRole("row", { name: /Operations/ })).toBeVisible();
  await page.getByRole("button", { name: "Load more" }).click();
  await expect(page.getByRole("rowheader", { name: "Finance" })).toBeVisible();
  await expect.poll(() => calls.some((c) => c.query.get("cursor") === "cursor-2")).toBe(true);
});

test("timeline groups by Space, expands a Project, and moves by one period", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto("/dashboards");
  await page.getByRole("radio", { name: "Timeline" }).click();
  await expect(page.getByRole("button", { name: /Launch plan/ })).toHaveCount(0);
  // Collapsed Space and Project rows still show where their work falls, before anything is expanded.
  await expect(page.locator("[data-summary]").first()).toBeVisible();
  await expect(page.locator("[data-summary]")).toHaveCount(3); // Nov 24 and Nov 25 on the Space row, Dec 1 for the unassigned Task
  await page.getByRole("button", { name: "Marketing", exact: true }).click();
  const row = page.getByRole("button", { name: /Launch plan/ });
  await expect(row).toHaveAttribute("aria-expanded", "false");
  await row.click();
  await expect(row).toHaveAttribute("aria-expanded", "true");
  await expect(page.getByText("Write brief")).toBeVisible();
  // Items sit under their real parents: "Book venue" is a top-level Task of the Space, not a separate group.
  await expect(page.getByText("Book venue")).toBeVisible();
  await page.getByRole("button", { name: "Next period" }).click();
  await expect.poll(() => calls.some((c) => c.query.get("from") === "2027-01-21")).toBe(true);
});

test("agenda groups by date, flags overdue work, and filters by type", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto("/dashboards");
  await page.getByRole("radio", { name: "Agenda" }).click();
  await expect(page.getByText("3 overdue tasks in Spaces you can see")).toBeVisible();
  await expect(page.getByRole("heading", { name: "2026-11-22" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "2026-11-25" })).toBeVisible();
  await page.getByRole("radio", { name: "Tasks", exact: true }).last().click(); // the Agenda filter; the view tab has the same name
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
      await page.getByRole("radio", { name: view }).click();
      await page.waitForLoadState("networkidle");
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
      expect(overflow).toBeLessThanOrEqual(0);
    });
  }
});

test("the Tasks tab lists Tasks across Spaces; filters combine; a Summary figure opens it already filtered", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto("/dashboards");
  await page.getByRole("button", { name: /Overdue tasks/ }).click();
  await expect(page.getByRole("radio", { name: "Tasks", exact: true }).first()).toBeChecked();
  await expect(page.getByRole("radio", { name: "Overdue", exact: true })).toBeChecked();
  await expect(page.getByRole("row", { name: /Book venue/ })).toBeVisible();
  await expect(page.getByRole("link", { name: "Operations" }).last()).toHaveAttribute("href", `/spaces/${ops.id}`);
  expect(calls.at(-1)?.query.get("preset")).toBe("overdue");

  await page.getByRole("checkbox", { name: "Completed" }).check();
  await expect.poll(() => calls.some((c) => c.query.getAll("status").includes("completed"))).toBe(true);
  await page.getByLabel("Due from").fill("2026-11-01");
  await expect.poll(() => calls.some((c) => c.query.get("due_from") === "2026-11-01")).toBe(true);
  await page.getByRole("searchbox", { name: "Search title" }).fill("venue");
  await expect.poll(() => calls.some((c) => c.query.get("q") === "venue")).toBe(true);
  await page.locator("details summary").click();
  await page.getByRole("checkbox", { name: "Marketing" }).check();
  await page.getByRole("checkbox", { name: "Operations" }).check();
  await expect.poll(() => calls.some((c) => c.query.getAll("space_id").length === 2 && c.query.getAll("space_id").includes(marketing.id))).toBe(true);
  await expect(page.getByRole("status").filter({ hasText: "2 tasks" })).toBeVisible();
});

test("the overview tabs end with Custom dashboard, after Tasks", async ({ page }) => {
  await mock(page);
  await page.goto("/dashboards");
  const sections = page.getByRole("radiogroup", { name: "Overview section" }).getByRole("radio");
  await expect(sections).toHaveText(["Summary", "Spaces", "Timeline", "Agenda", "Tasks", "Custom dashboard"]);
  await expect(page.getByRole("radio", { name: "My dashboards" })).toHaveCount(0);
  await sections.last().click();
  await expect(page.getByRole("radio", { name: "Custom dashboard" })).toBeChecked();
});

test("timeline ranges ask for calendar periods and scales change the grid", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto("/dashboards");
  await page.getByRole("radio", { name: "Timeline" }).click();
  await page.getByRole("radio", { name: "Quarter", exact: true }).click();
  await expect.poll(() => calls.at(-1)?.query.get("days")).toMatch(/^(89|90|91|92)$/);
  const quarter = calls.at(-1)!.query;
  expect(quarter.get("from")).toMatch(/-(01|04|07|10)-01$/);
  await page.getByRole("radio", { name: "Year", exact: true }).click();
  await expect.poll(() => calls.at(-1)?.query.get("days")).toMatch(/^(365|366)$/);
  expect(calls.at(-1)!.query.get("from")).toMatch(/-01-01$/);
  await page.getByRole("radio", { name: "Months", exact: true }).click();
  await expect(page.getByRole("radio", { name: "Months", exact: true })).toBeChecked();
});

test("timeline bars never cover the label column when the grid is scrolled", async ({ page }) => {
  await mock(page);
  await page.setViewportSize({ width: 1400, height: 900 });
  await page.goto("/dashboards");
  await page.getByRole("radio", { name: "Timeline" }).click();
  await page.getByRole("button", { name: "Marketing", exact: true }).click();
  await page.getByRole("region", { name: /timeline/i }).evaluate((el) => { el.scrollLeft = 200; });
  const onTop = await page.evaluate(() => {
    const label = document.querySelector('[role="region"] button[aria-expanded]') as HTMLElement;
    const row = label.closest(".flex.border-b") as HTMLElement;
    const r = row.getBoundingClientRect();
    return (document.elementFromPoint(r.x + 40, r.y + r.height / 2) as HTMLElement).closest(".sticky") !== null;
  });
  expect(onTop).toBe(true);
});
