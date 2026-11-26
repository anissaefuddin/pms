import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";

const a = "a0000000-0000-4000-8000-00000000000a";
const b = "a0000000-0000-4000-8000-00000000000b";
// The shape the API sends: the Gantt reads more than the dates.
const item = (id: string, title: string, start: string, due: string) => ({ id, title, start_date: start, due_date: due, type: "project", status: null, importance: "normal",
  assignees: [], locations: [], fields: {}, my_capabilities: ["view"], version: 1, child_count: 0, subtask_count: 0, attachment_count: 0, comment_count: 0, seq: 1 });

test("the portfolio lists tasks of several projects with their cross-project dependency; axe is clean", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: {
    "GET /search": (_, { query }) => ({ body: { items: [
      { id: "p1", title: "Alpha", type: "project" },
      { id: "p2", title: "Beta", type: "project" },
      { id: "task", title: "Alpha task", type: "task" },
    ].filter(item => item.title.toLowerCase().includes(query.get("q")?.toLowerCase() ?? "")) } }),
    "GET /item-types": { body: { items: [] } },
    "POST /dependencies/query": { body: { items: [] } },
    "GET /settings/work-schedule": { body: { working_days: [1, 2, 3, 4, 5], exceptions: [] } },
    "POST /portfolio/timeline": { body: {
      projects: [{ id: "p1", title: "Alpha" }, { id: "p2", title: "Beta" }],
      // The Gantt takes its top rows from the items that are the chosen projects themselves.
      items: [{ project_id: "p1", item: item("p1", "Alpha", "2026-11-02", "2026-11-06") }, { project_id: "p2", item: item("p2", "Beta", "2026-11-09", "2026-11-13") }, { project_id: "p1", item: item(a, "Design", "2026-11-02", "2026-11-06") }, { project_id: "p2", item: item(b, "Build", "2026-11-09", "2026-11-13") }],
      dependencies: [{ id: "d1", predecessor_id: a, successor_id: b, type: "FS", lag: 0 }],
    } },
  } });
  await page.goto("/portfolio");
  const search = page.getByRole("searchbox", { name: "Find projects by name" });
  await search.fill("Al");
  await expect(page.getByRole("button", { name: "Alpha", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Alpha task" })).toHaveCount(0);
  await search.press("Tab");
  await expect(page.getByRole("button", { name: "Alpha", exact: true })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(search).toBeFocused();
  await search.fill("Be");
  await page.getByRole("button", { name: "Beta", exact: true }).click();
  await page.getByRole("button", { name: "Show timeline" }).click();
  await expect(page.getByText("Design → Build (FS)")).toBeVisible();
  // A Gantt row for each project, with its dates.
  for (const date of ["2026-11-06", "2026-11-09"]) await expect(page.getByRole("main").getByText(date, { exact: true }).first()).toBeVisible();

  expect(calls.find((c) => c.path === "/portfolio/timeline")?.body).toEqual({ project_ids: ["p1", "p2"] });
  await page.reload();
  await expect(page.getByRole("button", { name: "Remove Alpha" })).toBeVisible();
  expect((await new AxeBuilder({ page }).include("main").analyze()).violations).toEqual([]);
});
