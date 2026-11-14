import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { me, problem } from "./mock-api";
import { marketing, mockSpaces } from "./spaces-mock";

const ada = { id: me.id, name: "Ada Lovelace" };
const r1 = "a0000000-0000-4000-8000-000000000001";
const r2 = "a0000000-0000-4000-8000-000000000002";
const proj = "b0000000-0000-4000-8000-000000000001";
const summary = (id: string, name: string, over: Record<string, unknown> = {}) => ({
  id, name, owner: ada, space: null, sharing: "private", created_at: "2026-11-01T09:00:00Z", updated_at: "2026-11-02T10:00:00Z", version: 1, subscription: null, ...over });
const definition = { layout: "column", query: { reporting_on: "tasks", metric: "count", group_by: "status_group", filters: {} } };
const result = { total: 5, rows: [
  { group: { key: "active", label: "active" }, break: null, value: 3 },
  { group: { key: "completed", label: "completed" }, break: null, value: 2 }] };

async function mock(page: Page, over: Record<string, any> = {}) {
  const rows = [summary(r1, "Team status", { space: { id: marketing.id, name: "Marketing" }, sharing: "space", subscription: { weekday: 1, hour: 8 } }), summary(r2, "My numbers")];
  const { calls } = await mockSpaces(page, { extra: {
    "GET /reports": () => ({ body: { items: rows } }),
    [`GET /reports/${r1}`]: () => ({ body: { ...rows[0], definition }, headers: { ETag: '"1"' } }),
    "POST /analytics/query": { body: result },
    "GET /report-templates": { body: { items: ["active_tasks_by_assignee", "overdue", "utilization"].map((key) => ({ key, layout: "column", query: { reporting_on: "tasks", metric: "count" } })) } },
    "GET /search": { body: { items: [{ id: proj, title: "Website launch", type: "project" }, { id: "b0000000-0000-4000-8000-0000000000aa", title: "Launch task", type: "task" }], comments: [], users: [] } },
    [`GET /items/${proj}`]: { body: { id: proj, title: "Website launch", type: "project" } },
    ...over,
  } });
  return { calls, rows };
}

test("list shows RPT-03 columns, filters by name, and has no axe violations", async ({ page }) => {
  await mock(page);
  await page.goto("/reports");
  await expect(page.getByRole("heading", { name: "Reports", level: 1 })).toBeVisible();
  await expect(page.getByRole("columnheader")).toHaveText(["Name", "Created by", "Space", "Sharing", "Created", "Updated", "Subscription", "Actions"]);
  const team = page.getByRole("row", { name: /Team status/ });
  await expect(team.getByRole("link", { name: "Team status" })).toHaveAttribute("href", `/reports/${r1}`);
  await expect(team).toContainText("Marketing");
  await expect(team).toContainText("Everyone in the Space");
  await expect(team).toContainText("Monday at 08:00");
  await expect(team).toContainText("Ada Lovelace");
  const mine = page.getByRole("row", { name: /My numbers/ });
  await expect(mine).toContainText("Personal");
  await expect(mine).toContainText("Not subscribed");
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByRole("searchbox", { name: "Filter reports" }).fill("zzz");
  await expect(page.getByText("No report matches that name.")).toBeVisible();
});

test("creating a report posts the default definition and opens the editor", async ({ page }) => {
  const { calls } = await mock(page, {
    "POST /reports": (body: any) => ({ status: 201, body: { ...summary(r1, body.name), definition: body.definition }, headers: { ETag: '"1"' } }),
  });
  await page.goto("/reports");
  await page.getByRole("button", { name: "New report", exact: true }).click();
  await page.getByLabel("Name", { exact: true }).fill("Weekly");
  await page.getByRole("button", { name: "Create report" }).click();
  await expect(page).toHaveURL(`/reports/${r1}`);
  const post = calls.find((c) => c.method === "POST" && c.path === "/reports")!;
  expect(post.body).toEqual({ name: "Weekly", space_id: null, sharing: "private", definition: { layout: "table", query: { reporting_on: "tasks", metric: "count", filters: {} } } });
});

test("a template fills the name and is created by key, without sending a definition", async ({ page }) => {
  const { calls } = await mock(page, {
    "POST /reports": (body: any) => ({ status: 201, body: { ...summary(r1, body.name), definition } , headers: { ETag: '"1"' } }),
  });
  await page.goto("/reports");
  await page.getByRole("button", { name: "New report", exact: true }).click();
  await page.getByLabel("Start from").selectOption("overdue");
  await expect(page.getByLabel("Name", { exact: true })).toHaveValue("Overdue tasks");
  await page.getByRole("button", { name: "Create report" }).click();
  await expect(page).toHaveURL(`/reports/${r1}`);
  expect(calls.find((c) => c.method === "POST" && c.path === "/reports")!.body).toEqual({ name: "Overdue tasks", space_id: null, sharing: "private", template: "overdue" });
});

test("deleting asks first, then sends If-Match", async ({ page }) => {
  const { calls, rows } = await mock(page, { [`DELETE /reports/${r2}`]: () => ({ status: 204 }) });
  await page.goto("/reports");
  await page.getByRole("button", { name: "Actions for My numbers" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await expect(page.getByRole("dialog")).toContainText("Tasks and time entries are not affected.");
  await page.getByRole("button", { name: "Delete report" }).click();
  await expect.poll(() => calls.some((c) => c.method === "DELETE")).toBe(true);
  expect(calls.find((c) => c.method === "DELETE")!.headers["if-match"]).toBe(`"${rows[1].version}"`);
});

test("editor runs the draft, saves with If-Match, and exposes export and subscription", async ({ page }) => {
  const { calls } = await mock(page, {
    [`PATCH /reports/${r1}`]: (body: any) => ({ body: { ...summary(r1, body.name), definition: body.definition, version: 2 }, headers: { ETag: '"2"' } }),
    [`PUT /reports/${r1}/subscription`]: (body: any) => ({ body }),
  });
  await page.goto(`/reports/${r1}`);
  await expect(page.getByRole("heading", { name: "Team status", level: 1 })).toBeVisible();
  // result: table plus a labelled chart for the column layout
  await expect(page.getByRole("img", { name: "Chart of Number of items by Status group" })).toBeVisible();
  await expect(page.getByRole("row", { name: /Active/ })).toContainText("3");
  await expect(page.getByText("Total: 5")).toBeVisible();
  await expect(page.getByRole("link", { name: "Export CSV" })).toHaveAttribute("href", `/api/v1/reports/${r1}/export`);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  // Switching the source drops choices the new source does not know.
  await page.getByLabel(/^Based on/).selectOption("time_entries");
  await expect(page.getByLabel(/^Metric/)).toHaveValue("minutes");
  await expect(page.getByLabel(/^Group by/)).toHaveValue("");
  await expect(page.getByRole("group", { name: "Status group" })).toHaveCount(0);
  await page.getByLabel(/^Group by/).selectOption("user");
  await page.getByRole("checkbox", { name: "Marketing" }).check();
  await page.getByLabel(/^Billing/).selectOption("billable");
  await page.getByRole("searchbox", { name: "Find a Project or Folder" }).fill("launch");
  await expect(page.getByRole("button", { name: "Launch task" })).toHaveCount(0); // only Projects and Folders can limit the source
  await page.getByRole("button", { name: "Website launch" }).click();
  await expect(page.getByRole("button", { name: "Remove Website launch" })).toBeVisible();
  await expect(page.getByText("You have unsaved changes.")).toBeVisible();
  await page.getByRole("button", { name: "Run report" }).click();
  await expect.poll(() => calls.filter((c) => c.path === "/analytics/query").length).toBe(2);
  expect(calls.filter((c) => c.path === "/analytics/query").at(-1)!.body).toEqual({
    reporting_on: "time_entries", metric: "minutes", group_by: "user", space_ids: [marketing.id], container_ids: [proj], filters: { billing: "billable" } });

  await page.getByLabel("Title").fill("Team time");
  await page.getByRole("button", { name: "Save report" }).click();
  await expect.poll(() => calls.some((c) => c.method === "PATCH")).toBe(true);
  const patch = calls.find((c) => c.method === "PATCH")!;
  expect(patch.headers["if-match"]).toBe('"1"');
  expect(patch.body.name).toBe("Team time");
  expect(patch.body.definition.query.reporting_on).toBe("time_entries");

  await page.getByLabel(/^Day/).selectOption("5");
  await page.getByLabel(/^Hour/).selectOption("17");
  await page.getByRole("button", { name: "Subscribe", exact: true }).click();
  await expect.poll(() => calls.some((c) => c.method === "PUT")).toBe(true);
  expect(calls.find((c) => c.method === "PUT")!.body).toEqual({ weekday: 5, hour: 17, timezone: null });
  // every day, in a zone of the user's choosing
  await page.getByLabel(/^Day/).selectOption("daily");
  await page.getByLabel(/^Time zone/).selectOption("Asia/Jakarta");
  await page.getByRole("button", { name: "Subscribe", exact: true }).click();
  await expect.poll(() => calls.filter((c) => c.method === "PUT").length).toBe(2);
  expect(calls.filter((c) => c.method === "PUT")[1].body).toEqual({ weekday: null, hour: 17, timezone: "Asia/Jakarta" });
});

test("choosing utilization sets the person grouping and the period it needs", async ({ page }) => {
  const { calls } = await mock(page);
  await page.goto(`/reports/${r1}`);
  await page.getByLabel(/^Based on/).selectOption("time_entries");
  await page.getByLabel(/^Metric/).selectOption("utilization");
  await expect(page.getByLabel(/^Group by/)).toHaveValue("user");
  await expect(page.getByRole("checkbox", { name: "This month" })).toBeChecked();
  await page.getByRole("button", { name: "Run report" }).click();
  await expect.poll(() => calls.filter((c) => c.path === "/analytics/query").length).toBe(2);
  expect(calls.filter((c) => c.path === "/analytics/query").at(-1)!.body).toEqual({
    reporting_on: "time_entries", metric: "utilization", group_by: "user", filters: { entry_this_month: true } });
});

test("a stale save shows the conflict message, and an unknown report shows not found", async ({ page }) => {
  await mock(page, { [`PATCH /reports/${r1}`]: problem(409, "report changed; reload and try again", "version_conflict") });
  await page.goto(`/reports/${r1}`);
  await page.getByLabel("Title").fill("Renamed");
  await page.getByRole("button", { name: "Save report" }).click();
  await expect(page.getByRole("alert")).toContainText("Someone changed this report. Reload it and try again.");
  await page.goto("/reports/a0000000-0000-4000-8000-0000000000ff");
  await expect(page.getByRole("alert")).toContainText("This report does not exist or you cannot see it.");
});

test("a source that is too large explains what to change", async ({ page }) => {
  await mock(page, { "POST /analytics/query": problem(422, "the data source is too large; narrow the Spaces or filters", "validation_failed") });
  await page.goto(`/reports/${r1}`);
  await expect(page.getByRole("alert")).toContainText("The data source is too large.");
});

test("the actual cost metric is offered and shown as money to an admin only; axe is clean", async ({ page }) => {
  const cost = { reporting_on: "time_entries", metric: "actual_cost_minor", group_by: "user", filters: {} };
  const def = { layout: "table", query: cost };
  const money = { total: 123456, rows: [{ group: { key: "u1", label: "Grace Hopper" }, break: null, value: 123456 }] };
  const open = async (user_type: string) => {
    await mock(page, { "GET /me": { body: { ...me, user_type } },
      [`GET /reports/${r1}`]: () => ({ body: { ...summary(r1, "Cost"), definition: def }, headers: { ETag: '"1"' } }), "POST /analytics/query": { body: money } });
    await page.goto(`/reports/${r1}`);
  };
  await open("admin");
  await expect(page.getByRole("columnheader", { name: "Actual cost" })).toBeVisible();
  await expect(page.getByRole("row", { name: /Grace Hopper 1,234\.56/ })).toBeVisible();
  await expect(page.getByText("Total: 1,234.56")).toBeVisible();
  await expect(page.getByLabel("Metric").getByRole("option", { name: "Actual cost" })).toHaveCount(1);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.unrouteAll();
  await open("regular");
  await expect(page.getByLabel("Metric").getByRole("option", { name: "Actual cost" })).toHaveCount(0);
});
