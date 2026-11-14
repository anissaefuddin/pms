import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { me, problem } from "./mock-api";
import { marketing, mockSpaces } from "./spaces-mock";

const d1 = "d0000000-0000-4000-8000-000000000001";
const d2 = "d0000000-0000-4000-8000-000000000002";
const w = (n: number) => `e0000000-0000-4000-8000-00000000000${n}`;
const query = (extra: Record<string, unknown> = {}) => ({ reporting_on: "tasks", metric: "count", group_by: "status_group", filters: {}, ...extra });
const widget = (n: number, name: string, type: string, config: Record<string, unknown>, position = n) => ({ id: w(n), name, type, position, config });
const rows = (labels: [string, number][]) => ({ total: labels.reduce((s, [, v]) => s + v, 0), rows: labels.map(([key, value]) => ({ group: { key, label: key }, break: null, value })) });
const many = rows(Array.from({ length: 11 }, (_, i) => [`item${i}`, 11 - i] as [string, number]));

function board(over: Record<string, unknown> = {}) {
  return { id: d1, owner_id: me.id, space_id: null, name: "Team board", layout: "columns", version: 3, widgets: [
    widget(1, "This week", "task_list", { list: "todo_this_week" }),
    widget(2, "Open tasks", "indicator", { query: query({ group_by: undefined }) }),
    widget(3, "By status", "pie", { show_legend: true, show_labels: true, pie_style: "donut", query: query() }),
    widget(4, "By importance", "column", { show_legend: false, query: query({ group_by: "importance" }) }),
    widget(5, "By assignee", "bar", { query: query({ group_by: "assignee" }) }),
    widget(6, "Trend", "line", { query: query({ group_by: "due_month" }) }),
    widget(7, "Table", "table", { query: query() }),
  ], ...over };
}
const series = (result: unknown) => ({ body: { kind: "series", result } });

async function mock(page: Page, over: Record<string, any> = {}, dash = board()) {
  const list = [{ id: d1, owner_id: me.id, space_id: null, name: "Team board", version: 3 }, { id: d2, owner_id: me.id, space_id: null, name: "Second", version: 1 }];
  const { calls } = await mockSpaces(page, { extra: {
    "GET /dashboards": () => ({ body: { items: list } }),
    [`GET /dashboards/${d1}`]: () => ({ body: dash }),
    [`GET /dashboards/${d1}/widgets/${w(1)}/data`]: { body: { kind: "tasks", items: [{ id: "f0000000-0000-4000-8000-000000000001", title: "Write the plan", status: "Open", status_group: "active", due_date: "2026-11-13", importance: "high" }] } },
    [`GET /dashboards/${d1}/widgets/${w(2)}/data`]: series(rows([["", 12]])),
    [`GET /dashboards/${d1}/widgets/${w(3)}/data`]: series(rows([["active", 7], ["completed", 4], ["deferred", 1]])),
    [`GET /dashboards/${d1}/widgets/${w(4)}/data`]: series(rows([["high", 3], ["normal", 6], ["low", 3]])),
    [`GET /dashboards/${d1}/widgets/${w(5)}/data`]: series(many),
    [`GET /dashboards/${d1}/widgets/${w(6)}/data`]: series(rows([["2026-09", 2], ["2026-10", 5], ["2026-11", 4]])),
    [`GET /dashboards/${d1}/widgets/${w(7)}/data`]: series(rows([["active", 7], ["completed", 4]])),
    "POST /analytics/query": { body: rows([["active", 7], ["completed", 4]]) },
    ...over,
  } });
  return calls;
}

test("list shows dashboards, switches scope, creates and opens one", async ({ page }) => {
  const calls = await mock(page, { "POST /dashboards": (body: any) => ({ status: 201, body: { ...board(), name: body.name } }) });
  await page.goto("/dashboards");
  await expect(page.getByRole("heading", { name: "Dashboards", level: 1 })).toBeVisible();
  await expect(page.getByRole("link", { name: "Team board" })).toHaveAttribute("href", `/dashboards/${d1}`);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByLabel("Show dashboards in").selectOption(marketing.id);
  await expect.poll(() => calls.some((c) => c.path === "/dashboards" && c.query.get("space_id") === marketing.id)).toBe(true);
  await page.getByLabel("Name", { exact: true }).fill("Weekly");
  await page.getByRole("button", { name: "Create dashboard" }).click();
  await expect(page).toHaveURL(`/dashboards/${d1}`);
  expect(calls.find((c) => c.method === "POST" && c.path === "/dashboards")!.body).toEqual({ name: "Weekly", space_id: marketing.id });
});

test("rename, duplicate, move and delete send the right requests and ask before deleting", async ({ page }) => {
  const calls = await mock(page, {
    [`PATCH /dashboards/${d1}`]: (body: any) => ({ body: { ...board(), name: body.name, version: 4 } }),
    [`POST /dashboards/${d1}/duplicate`]: { status: 201, body: board() },
    [`POST /dashboards/${d1}/move`]: { body: board() },
    [`DELETE /dashboards/${d2}`]: { status: 204 },
  });
  await page.goto("/dashboards");
  const open = (name: string, item: string) => async () => { await page.getByRole("button", { name: `Actions for ${name}` }).click(); await page.getByRole("menuitem", { name: item }).click(); };
  await open("Team board", "Rename")();
  await page.getByLabel("Name", { exact: true }).last().fill("Renamed");
  await page.getByRole("button", { name: "Rename", exact: true }).click();
  await expect.poll(() => calls.some((c) => c.method === "PATCH")).toBe(true);
  expect(calls.find((c) => c.method === "PATCH")!.headers["if-match"]).toBe('"3"');
  expect(calls.find((c) => c.method === "PATCH")!.body).toEqual({ name: "Renamed" });

  await open("Team board", "Duplicate")();
  await page.getByLabel("Name of the copy").fill("Copy of board");
  await page.getByLabel("Copy into").selectOption(marketing.id);
  await page.getByRole("button", { name: "Duplicate", exact: true }).click();
  await expect.poll(() => calls.some((c) => c.path.endsWith("/duplicate"))).toBe(true);
  expect(calls.find((c) => c.path.endsWith("/duplicate"))!.body).toEqual({ name: "Copy of board", space_id: marketing.id });

  await open("Team board", "Move")();
  await page.getByLabel("Move to").selectOption(marketing.id);
  await page.getByRole("button", { name: "Move", exact: true }).click();
  await expect.poll(() => calls.some((c) => c.path.endsWith("/move"))).toBe(true);
  expect(calls.find((c) => c.path.endsWith("/move"))!.headers["if-match"]).toBe('"3"');
  expect(calls.find((c) => c.path.endsWith("/move"))!.body).toEqual({ space_id: marketing.id });

  await open("Second", "Delete")();
  await expect(page.getByRole("dialog")).toContainText("Tasks and other data are not affected.");
  expect(calls.some((c) => c.method === "DELETE")).toBe(false);
  await page.getByRole("button", { name: "Delete dashboard" }).click();
  await expect.poll(() => calls.some((c) => c.method === "DELETE")).toBe(true);
  expect(calls.find((c) => c.method === "DELETE")!.headers["if-match"]).toBe('"1"');
});

test("a stale rename shows the conflict message", async ({ page }) => {
  await mock(page, { [`PATCH /dashboards/${d1}`]: problem(409, "dashboard changed", "version_conflict") });
  await page.goto("/dashboards");
  await page.getByRole("button", { name: "Actions for Team board" }).click();
  await page.getByRole("menuitem", { name: "Rename" }).click();
  await page.getByRole("button", { name: "Rename", exact: true }).click();
  await expect(page.getByRole("dialog").getByRole("alert")).toContainText("Someone changed this dashboard.");
});

test("every widget type renders, with accessible names, legends, labels and an Others slice, and axe passes", async ({ page }) => {
  await mock(page);
  await page.goto(`/dashboards/${d1}`);
  await expect(page.getByRole("heading", { name: "Team board", level: 1 })).toBeVisible();
  // task list, indicator and table
  await expect(page.getByRole("link", { name: "Write the plan" })).toHaveAttribute("href", /\/tasks\//);
  await expect(page.getByRole("region", { name: "Open tasks" })).toContainText("12");
  await expect(page.getByRole("region", { name: "Table" }).getByRole("row", { name: /Active/ })).toContainText("7");
  // charts are labelled images drawn as SVG
  for (const [type, name] of [["pie", "By status"], ["column", "By importance"], ["bar", "By assignee"], ["line", "Trend"]] as const) {
    const chart = page.getByRole("region", { name }).locator(`[data-chart="${type}"]`);
    await expect(chart).toHaveAttribute("aria-label", /Chart of Number of items by/);
    await expect(chart.locator("svg")).toBeVisible();
    expect(await chart.locator("svg path, svg rect").count()).toBeGreaterThan(2);
  }
  const pie = page.getByRole("region", { name: "By status" }).locator('[data-chart="pie"]');
  await expect(pie).toContainText("Active"); // legend and label text
  await expect(pie).toContainText("Deferred");
  const column = page.getByRole("region", { name: "By importance" }).locator('[data-chart="column"]');
  await expect(column).toContainText("Number of items"); // value axis name; the legend is switched off
  // 11 categories fold into 7 slices plus Others in a pie, and every chart has a table behind it
  const bar = page.getByRole("region", { name: "By assignee" });
  await bar.getByText("Show data as a table").click();
  await expect(bar.getByRole("row")).toHaveCount(1 + 11 + 1);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("a pie folds the smallest values into Others", async ({ page }) => {
  await mock(page, { [`GET /dashboards/${d1}/widgets/${w(3)}/data`]: series(many) });
  await page.goto(`/dashboards/${d1}`);
  const pie = page.getByRole("region", { name: "By status" }).locator('[data-chart="pie"]');
  await expect(pie).toContainText("Others");
  await expect(pie).not.toContainText("item10"); // folded away
  await expect(pie).toContainText("item0");
});

test("chart styles are drawn differently: classic, donut and ring give different sector shapes", async ({ page }) => {
  const shapes: Record<string, string> = {};
  for (const style of ["classic", "donut", "ring"]) {
    const dash = board();
    dash.widgets = [widget(3, "Style", "pie", { pie_style: style, show_labels: false, query: query() })];
    await page.unrouteAll();
    await mock(page, { [`GET /dashboards/${d1}/widgets/${w(3)}/data`]: series(rows([["active", 5], ["completed", 5]])) }, dash);
    await page.goto(`/dashboards/${d1}`);
    const chart = page.locator('[data-chart="pie"] svg');
    await expect(chart).toBeVisible();
    shapes[style] = await chart.evaluate((svg) => [...svg.querySelectorAll("path")].map((p) => p.getAttribute("d")).filter((d) => d && d.includes("A")).join("|"));
    expect(shapes[style]).not.toBe("");
  }
  expect(new Set(Object.values(shapes)).size).toBe(3);
});

test("adding a widget previews the draft and saves the config with If-Match", async ({ page }) => {
  const calls = await mock(page, { [`POST /dashboards/${d1}/widgets`]: (body: any) => ({ status: 201, body: { ...board(), version: 4, widgets: [...board().widgets, { id: w(8), position: 7, ...body }] } }) });
  await page.goto(`/dashboards/${d1}`);
  await page.getByRole("button", { name: "Add widget" }).click();
  await page.getByRole("dialog", { name: "Choose widget" }).getByRole("button", { name: /^Task list/ }).click();
  const dialog = page.getByRole("dialog", { name: "Add widget" });
  await dialog.getByLabel("Widget name").fill("My donut");
  await dialog.getByLabel(/^Widget type/).selectOption("pie");
  await dialog.getByLabel(/^Group by/).selectOption("importance");
  await dialog.getByRole("tab", { name: "Design" }).click();
  await dialog.getByLabel(/^Pie style/).selectOption("ring");
  await dialog.getByLabel("Show labels").check();
  await expect.poll(() => calls.filter((c) => c.path === "/analytics/query").at(-1)?.body?.group_by).toBe("importance");
  await expect(dialog.locator('[data-chart="pie"] svg')).toBeVisible(); // preview
  expect((await new AxeBuilder({ page }).include('[role="dialog"]').analyze()).violations).toEqual([]);
  await dialog.getByRole("button", { name: "Save widget" }).click();
  await expect.poll(() => calls.some((c) => c.method === "POST" && c.path.endsWith("/widgets"))).toBe(true);
  const post = calls.find((c) => c.method === "POST" && c.path.endsWith("/widgets"))!;
  expect(post.headers["if-match"]).toBe('"3"');
  expect(post.body).toEqual({ name: "My donut", type: "pie", position: 7,
    config: { query: { reporting_on: "tasks", metric: "count", group_by: "importance", filters: {} }, show_legend: true, show_labels: true, pie_style: "ring" } });
});

test("a task list widget saves a list or a template, not a query", async ({ page }) => {
  const calls = await mock(page, { [`PUT /dashboards/${d1}/widgets/${w(1)}`]: { body: board() } });
  await page.goto(`/dashboards/${d1}`);
  await page.getByRole("button", { name: "Actions for This week" }).click();
  await page.getByRole("menuitem", { name: "Edit" }).click();
  await page.getByLabel(/^List or template/).selectOption("template:overdue");
  await page.getByRole("button", { name: "Save widget" }).click();
  await expect.poll(() => calls.some((c) => c.method === "PUT")).toBe(true);
  expect(calls.find((c) => c.method === "PUT")!.body).toEqual({ name: "This week", type: "task_list", position: 1, config: { template: "overdue" } });
});

test("moving a widget later swaps positions, and deleting asks first", async ({ page }) => {
  const calls = await mock(page, {
    [`PUT /dashboards/${d1}/widgets/${w(1)}`]: { body: board({ version: 4 }) },
    [`PUT /dashboards/${d1}/widgets/${w(2)}`]: { body: board({ version: 5 }) },
    [`DELETE /dashboards/${d1}/widgets/${w(7)}`]: { body: board({ version: 4 }) },
  });
  await page.goto(`/dashboards/${d1}`);
  await page.getByRole("button", { name: "Actions for This week" }).click();
  await page.getByRole("menuitem", { name: "Move later" }).click();
  await expect.poll(() => calls.filter((c) => c.method === "PUT").length).toBe(2);
  const puts = calls.filter((c) => c.method === "PUT");
  expect(puts.map((c) => [c.path.slice(-1), c.body.position, c.headers["if-match"]])).toEqual([["1", 1, '"3"'], ["2", 0, '"4"']]);

  await page.getByRole("button", { name: "Actions for Table" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await expect(page.getByRole("dialog")).toContainText("will be removed from this dashboard");
  expect(calls.some((c) => c.method === "DELETE")).toBe(false);
  await page.getByRole("button", { name: "Delete widget" }).click();
  await expect.poll(() => calls.some((c) => c.method === "DELETE")).toBe(true);
});

test("a widget without a data definition explains itself, and errors stay in their card", async ({ page }) => {
  const dash = board();
  dash.widgets = [widget(3, "Empty chart", "pie", {}), widget(4, "Broken", "column", { query: query() })];
  await mock(page, {
    [`GET /dashboards/${d1}/widgets/${w(3)}/data`]: problem(422, "the widget has no data definition", "validation_failed"),
    [`GET /dashboards/${d1}/widgets/${w(4)}/data`]: problem(503, "the query took too long", "unavailable"),
  }, dash);
  await page.goto(`/dashboards/${d1}`);
  await expect(page.getByRole("region", { name: "Empty chart" })).toContainText("the widget has no data definition");
  await expect(page.getByRole("region", { name: "Broken" }).getByRole("alert")).toContainText("This widget could not be loaded.");
});

test("an unknown dashboard shows not found, and a read-only viewer sees no edit controls", async ({ page }) => {
  await mock(page, {}, board({ space_id: marketing.id }));
  await page.goto(`/dashboards/${d1}`);
  await expect(page.getByRole("heading", { name: "Team board", level: 1 })).toBeVisible();
  await page.unrouteAll();
  await mockSpaces(page, { spaces: [{ ...marketing, my_capabilities: ["view"] }] as any, extra: { [`GET /dashboards/${d1}`]: { body: board({ space_id: marketing.id }) }, ...Object.fromEntries([1, 2, 3, 4, 5, 6, 7].map((n) => [`GET /dashboards/${d1}/widgets/${w(n)}/data`, series(rows([["active", 1]]))])) } });
  await page.goto(`/dashboards/${d1}`);
  await expect(page.getByText("You can view this dashboard but not change it.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Add widget" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: /^Actions for/ })).toHaveCount(0);
  await page.unrouteAll();
  await mock(page, { [`GET /dashboards/${d1}`]: problem(404, "not found", "not_found") });
  await page.goto(`/dashboards/${d1}`);
  await expect(page.getByRole("alert")).toContainText("This dashboard does not exist or you cannot see it.");
});

test("area, pivot, completion and burndown widgets render from their data, and the picker groups the kinds", async ({ page }) => {
  const q = { reporting_on: "tasks", metric: "count", group_by: "importance", break_by: "status_group", filters: {} };
  const dash = board({ widgets: [
    widget(1, "Area", "area", { query: query({ group_by: "due_month" }) }),
    widget(2, "Pivot", "pivot", { query: q }),
    widget(3, "Done", "completion", { query: { reporting_on: "tasks", metric: "completion_percent", filters: {} } }),
    widget(4, "Burn", "burndown", { query: { reporting_on: "tasks", metric: "count", filters: {} } }),
  ] });
  const cell = (g: [string, string], b: [string, string], value: number) => ({ group: { key: g[0], label: g[1] }, break: { key: b[0], label: b[1] }, value });
  await mock(page, {
    [`GET /dashboards/${d1}/widgets/${w(1)}/data`]: series(rows([["2026-10", 2], ["2026-11", 5]])),
    [`GET /dashboards/${d1}/widgets/${w(2)}/data`]: series({ total: 10, rows: [cell(["high", "high"], ["active", "Active"], 3), cell(["high", "high"], ["completed", "Completed"], 2), cell(["low", "low"], ["active", "Active"], 5)] }),
    [`GET /dashboards/${d1}/widgets/${w(3)}/data`]: series({ total: 40, rows: [] }),
    [`GET /dashboards/${d1}/widgets/${w(4)}/data`]: series(rows([["2026-11-01", 9], ["2026-11-02", 7], ["2026-11-03", 4]])),
  }, dash);
  await page.goto(`/dashboards/${d1}`);
  await expect(page.getByRole("region", { name: "Area" }).locator('[data-chart="area"] svg')).toBeVisible();
  const pivot = page.getByRole("region", { name: "Pivot" });
  await expect(pivot.getByRole("columnheader", { name: "Active" })).toBeVisible();
  await expect(pivot.getByRole("row", { name: /High/ })).toContainText("5"); // 3 + 2 across the row
  await expect(pivot.getByRole("row", { name: /^Total/ })).toContainText("10");
  await expect(page.getByRole("region", { name: "Done" }).getByRole("progressbar")).toHaveAttribute("aria-valuenow", "40");
  await expect(page.getByRole("region", { name: "Burn" }).locator('[data-chart="burndown"] svg')).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByRole("button", { name: "Add widget" }).click();
  const picker = page.getByRole("dialog", { name: "Choose widget" });
  for (const category of ["Lists", "Numbers", "Charts", "Progress over time", "Tables"]) await expect(picker.getByRole("region", { name: category })).toBeVisible();
  await expect(picker.getByRole("region", { name: "Tables" }).getByRole("button", { name: /^Pivot table/ })).toBeVisible();
});

test("a Space gets a ready-made analytics board of five widgets", async ({ page }) => {
  let version = 1;
  const calls = await mock(page, {
    "POST /dashboards": (body: any) => ({ status: 201, body: { ...board({ widgets: [] }), name: body.name, version } }),
    [`POST /dashboards/${d1}/widgets`]: () => ({ status: 201, body: { ...board({ widgets: [] }), version: ++version } }),
  });
  await page.goto("/");
  await expect(page.getByRole("button", { name: "Create analytics board" })).toHaveCount(0); // Tools only show once a Space is open
  await page.getByRole("button", { name: "Expand Marketing" }).click();
  await page.getByRole("button", { name: "Create analytics board" }).click();
  await expect(page).toHaveURL(`/dashboards/${d1}`);
  const posts = calls.filter((c) => c.method === "POST" && c.path === `/dashboards/${d1}/widgets`);
  expect(posts.map((c) => c.body.type)).toEqual(["completion", "pie", "burndown", "pivot", "bar"]);
  expect(posts.map((c) => c.headers["if-match"])).toEqual(['"1"', '"2"', '"3"', '"4"', '"5"']);
});

test("a widget shows its description and the layout can switch to one column", async ({ page }) => {
  const dash = board({ widgets: [widget(1, "Open", "indicator", { description: "Everything not done yet", query: query({ group_by: undefined }) })] });
  const calls = await mock(page, {
    [`GET /dashboards/${d1}/widgets/${w(1)}/data`]: series(rows([["", 12]])),
    [`PATCH /dashboards/${d1}`]: (body: any) => ({ body: { ...dash, ...body, version: 4 } }),
  }, dash);
  await page.goto(`/dashboards/${d1}`);
  await expect(page.getByRole("region", { name: "Open" })).toContainText("Everything not done yet");
  await page.getByRole("button", { name: "Layout" }).click();
  await page.getByRole("button", { name: "Vertical" }).click();
  await expect.poll(() => calls.find((c) => c.method === "PATCH")?.body).toEqual({ layout: "stack" });
  expect(calls.find((c) => c.method === "PATCH")?.headers["if-match"]).toBe('"3"');
  await expect(page.locator("main > div.grid").first()).not.toHaveClass(/lg:grid-cols-2/);
});

test("the title menu switches dashboard, the layout popover changes it, and the editor keeps preview beside Data, Design and Filters tabs", async ({ page }) => {
  const calls = await mock(page, { [`PATCH /dashboards/${d1}`]: (body: any) => ({ body: { ...board(), ...body, version: 4 } }) });
  await page.goto(`/dashboards/${d1}`);
  await page.getByRole("button", { name: "Team board: switch dashboard" }).click();
  await expect(page.getByRole("menuitem", { name: "Second" })).toHaveAttribute("href", `/dashboards/${d2}`);
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Layout" }).click();
  await expect(page.getByRole("button", { name: "Horizontal" })).toHaveAttribute("aria-pressed", "true");
  await page.getByRole("button", { name: "Vertical" }).click();
  await expect.poll(() => calls.find((c) => c.method === "PATCH")?.body).toEqual({ layout: "stack" });
  await page.keyboard.press("Escape");
  await expect(page.getByRole("button", { name: "Share" })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByRole("button", { name: "Actions for By status" }).click();
  await page.getByRole("menuitem", { name: "Edit" }).click();
  const dialog = page.getByRole("dialog", { name: "Edit widget" });
  await expect(dialog.getByRole("region", { name: "Preview" })).toBeVisible();
  await dialog.getByRole("tab", { name: "Filters" }).click();
  await expect(dialog.getByRole("tabpanel")).toContainText("Filters");
  await dialog.getByRole("tab", { name: "Design" }).click();
  await dialog.getByLabel("Show labels").uncheck();
  await expect(dialog.getByRole("region", { name: "Preview" })).toBeVisible();
  expect((await new AxeBuilder({ page }).include('[role="dialog"]').analyze()).violations).toEqual([]);
});
