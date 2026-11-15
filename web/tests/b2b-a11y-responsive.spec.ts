import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { me } from "./mock-api";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// QA batch 2, lane B: RT-05/06/24/25/26 (a11y + 390 px layout). Mock API, no backend.
const rid = "a0000000-0000-4000-8000-000000000001";
const did = "d0000000-0000-4000-8000-000000000001";
const wid = (n: number) => `e0000000-0000-4000-8000-00000000000${n}`;
const long = "A very long report name that keeps going and going so a narrow screen has to cope with it";
const summary = { id: rid, name: long, owner: { id: me.id, name: "Ada Lovelace" }, space: { id: "c0000000-0000-4000-8000-000000000001", name: "QA | Engineering" }, sharing: "space", created_at: "2026-11-01T09:00:00Z", updated_at: "2026-11-02T10:00:00Z", version: 1, subscription: { weekday: 1, hour: 8 } };
const notifications = { items: Array.from({ length: 3 }, (_, i) => ({ id: `bbbbbbbb-bbbb-4bbb-8bbb-00000000000${i}`, kind: "mention", item_id: "b0000000-0000-4000-8000-000000000001", item_title: `${i}-${"QA-L2-unbroken-title-".repeat(6)}`, actor: { id: me.id, name: "Ada Lovelace" }, snippet: "x".repeat(300), created_at: "2026-11-12T12:00:00Z" })), unread_count: 11 };
const taskRow = (i: number) => ({ id: `f0000000-0000-4000-8000-00000000000${i}`, title: `${long} ${i}`, status: "In Progress", status_group: "active", due_date: "", importance: "normal" });
const series = (rows: [string, number][]) => ({ body: { kind: "series", result: { total: 1, rows: rows.map(([key, value]) => ({ group: { key, label: key }, break: null, value })) } } });
const query = { reporting_on: "tasks", metric: "count", group_by: "status", filters: {} };

async function mock(page: Page, extra: Record<string, any> = {}) {
  await mockSpaces(page, { extra: {
    "GET /notifications": { body: notifications },
    "GET /reports": { body: { items: [summary] } },
    "GET /dashboards": { body: { items: [{ id: did, owner_id: me.id, space_id: null, name: "Board", version: 1 }] } },
    [`GET /dashboards/${did}`]: { body: { id: did, owner_id: me.id, space_id: null, name: "Board", layout: "columns", version: 1, widgets: [
      { id: wid(1), name: "Backlog", type: "task_list", position: 0, config: { template: "backlog" } },
      { id: wid(2), name: "Column", type: "column", position: 1, config: { query } },
      { id: wid(3), name: "Bar", type: "bar", position: 2, config: { query: { ...query, group_by: "assignee" } } }] } },
    [`GET /dashboards/${did}/widgets/${wid(1)}/data`]: { body: { kind: "tasks", items: [taskRow(1), taskRow(2)] } },
    [`GET /dashboards/${did}/widgets/${wid(2)}/data`]: series([["Completed", 42], ["New", 100]]),
    [`GET /dashboards/${did}/widgets/${wid(3)}/data`]: series([["HAZID Engineer", 30], ["(none)", 12]]),
    "GET /workload": { body: { bookings: [], role_demand: [], tasks: [], people: [{ id: me.id, name: "Ada", capacity: [480, 480, 480], planned: [0, 0, 0], booked: [0, 0, 0] }] } },
    "GET /utilization": { body: { items: [] } },
    ...extra,
  } });
}
const hscroll = (page: Page) => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);

test.describe("390 px", () => {
  test.use({ viewport: { width: 390, height: 844 } });
  for (const route of ["/reports", "/inbox", `/dashboards/${did}`]) {
    test(`${route} does not scroll the page sideways`, async ({ page }) => {
      await mock(page);
      await page.goto(route);
      await page.waitForLoadState("networkidle");
      await expect.poll(() => hscroll(page)).toBeLessThanOrEqual(0);
    });
  }
  test("a wide report table scrolls inside a keyboard-focusable region", async ({ page }) => {
    await mock(page);
    await page.goto("/reports");
    await expect(page.getByRole("region", { name: "Reports" })).toBeVisible();
    expect((await new AxeBuilder({ page }).analyze()).violations.filter((v) => v.impact === "serious" || v.impact === "critical")).toEqual([]);
  });
});

test("workload scroll regions are keyboard focusable (axe)", async ({ page }) => {
  await mock(page);
  await page.goto("/workload");
  await expect(page.getByRole("table").first()).toBeVisible();
  const serious = (await new AxeBuilder({ page }).analyze()).violations.filter((v) => v.impact === "serious" || v.impact === "critical");
  expect(serious).toEqual([]);
});

test.describe("dark", () => {
  test.use({ colorScheme: "dark" });
  test("unread badge keeps AA contrast", async ({ page }) => {
    await mock(page);
    await page.goto("/reports");
    await expect(page.getByText("11").first()).toBeAttached();
    const v = (await new AxeBuilder({ page }).withRules(["color-contrast"]).analyze()).violations;
    expect(v).toEqual([]);
  });
});

test("RT-05: table assignee button name contains its visible text (axe label-content-name-mismatch)", async ({ page }) => {
  const item = { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", title: "Pilot task", type: "task", seq: 1, version: 1, my_capabilities: ALL, locations: [{ space_id: marketing.id }], assignees: [{ id: me.id, name: "Ada Lovelace" }] };
  const table = { id: "55555555-5555-4555-8555-555555555555", type: "table", name: "Table", visibility: "public", position: 0, is_default: true };
  await mockSpaces(page, { extra: { [`GET /spaces/${marketing.id}/views`]: { body: { items: [table] } }, [`GET /spaces/${marketing.id}/items`]: { body: { items: [item] } } } });
  await page.goto(`/spaces/${marketing.id}`);
  await expect(page.getByRole("button", { name: /^Assignees: Pilot task/ })).toBeVisible();
  expect((await new AxeBuilder({ page }).withRules(["label-content-name-mismatch"]).analyze()).violations).toEqual([]);
});

test("RT-28: chart axis titles stay inside the chart", async ({ page }) => {
  await mock(page);
  await page.goto(`/dashboards/${did}`);
  await expect(page.locator("svg text").first()).toBeAttached();
  await page.waitForTimeout(1500);
  const clipped = await page.evaluate(() => [...document.querySelectorAll("svg")].flatMap((svg) => {
    const box = svg.getBoundingClientRect();
    return [...svg.querySelectorAll("text")].filter((t) => { const r = t.getBoundingClientRect(); return r.width > 0 && (r.left < box.left - 0.5 || r.right > box.right + 0.5); }).map((t) => t.textContent);
  }));
  expect(clipped).toEqual([]);
});
