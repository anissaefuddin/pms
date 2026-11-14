import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";
import { chip } from "./view-helpers";
import { problem, type Handler } from "./mock-api";

const tableId = "55555555-5555-4555-8555-555555555555";
const boardId = "66666666-6666-4666-8666-666666666666";
const taskId = "77777777-7777-4777-8777-777777777777";
const newStatus = { id: "88888888-8888-4888-8888-888888888888", name: "New", group: "active", color: "slate" };
const completed = { id: "99999999-9999-4999-8999-999999999999", name: "Completed", group: "completed", color: "green" };
const path = `/spaces/${marketing.id}`;
const task = { id: taskId, seq: 1, type: "task", title: "Plan", version: 1, status: newStatus,
  my_capabilities: ALL, locations: [{ space_id: marketing.id }], assignees: [], child_count: 2 };
const table = { id: tableId, type: "table", name: "Table", visibility: "public", position: 0, is_default: true };
const board = { id: boardId, type: "board", name: "Board", visibility: "public", position: 1, is_default: false };
const workflows = { body: { items: [{ is_default: true, statuses: [newStatus, completed] }] } };

test("Table keyboard edit and reorder, and URL restores the view state", async ({ page }) => {
  let item = task;
  const sibling = { ...task, id: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", title: "Checklist", seq: 2, child_count: 0 };
  let saved = [table, board];
  const { calls } = await mockSpaces(page, { extra: {
    [`GET ${path}/views`]: () => ({ body: { items: saved } }),
    [`GET ${path}/items`]: () => ({ body: { items: [item, sibling] } }),
    [`GET /items/${taskId}/children`]: { body: { items: [{
      ...task, id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", title: "Outline",
      locations: [{ space_id: marketing.id, parent_item_id: taskId }],
    }] } },
    "GET /workflows": workflows,
    [`PATCH /items/${taskId}`]: (body, req) => {
      if (req.headers["if-match"] !== `"${item.version}"`) return problem(409, "Conflict");
      item = { ...item, ...body, version: item.version + 1 };
      return { body: item };
    },
    [`POST /items/${taskId}/reorder`]: { status: 204 },
    [`PATCH /views/${tableId}`]: (body) => {
      saved = saved.map((view) => view.id === tableId ? { ...view, ...body } : view);
      return { body: saved[0] };
    },
  } });
  await page.goto(path);
  const grid = page.getByRole("table", { name: "Table" });
  await expect(grid.getByRole("button", { name: "Plan", exact: true })).toBeVisible();
  await grid.getByRole("button", { name: "Expand Plan" }).click();
  await expect(grid.getByRole("button", { name: "Outline", exact: true })).toBeVisible();
  await grid.getByRole("button", { name: "Collapse Plan" }).click();
  await expect(grid.getByRole("button", { name: "Outline", exact: true })).toHaveCount(0);
  await grid.focus();
  await page.keyboard.press("s");
  await expect(grid.getByRole("combobox", { name: "Status: Plan" })).toBeFocused();
  await grid.focus();
  await page.keyboard.press("a");
  await expect(page.getByRole("dialog", { name: "Assignees" })).toBeVisible();
  await page.getByRole("dialog", { name: "Assignees" }).getByRole("button", { name: "Cancel" }).click();
  await grid.focus();
  await page.keyboard.press("e");
  await grid.getByRole("textbox", { name: "Name", exact: true }).fill("Roadmap");
  await grid.getByRole("textbox", { name: "Name", exact: true }).press("Enter");
  await expect(grid.getByRole("button", { name: "Roadmap", exact: true })).toBeVisible();
  await grid.getByRole("button", { name: "Move Roadmap", exact: true }).click();
  await page.getByRole("menuitem", { name: "Move to top" }).click();
  await expect.poll(() => calls.find((call) => call.path.endsWith("/reorder"))?.body).toEqual({
    space_id: marketing.id, parent_item_id: null, after_item_id: null,
  });
  const source = grid.getByRole("row").filter({ has: page.getByRole("button", { name: "Roadmap", exact: true }) });
  await expect(source).toHaveAttribute("draggable", "true");
  // Drive the HTML5 drag events directly: native dragTo is unreliable on the absolutely positioned, virtualized rows.
  const target = grid.getByRole("row").filter({ has: page.getByRole("button", { name: "Checklist", exact: true }) });
  const data = await page.evaluateHandle(() => new DataTransfer());
  await source.dispatchEvent("dragstart", { dataTransfer: data });
  await target.dispatchEvent("dragover", { dataTransfer: data });
  await target.dispatchEvent("drop", { dataTransfer: data });
  await source.dispatchEvent("dragend", { dataTransfer: data });
  await expect.poll(() => calls.filter((call) => call.path.endsWith("/reorder")).at(-1)?.body.after_item_id).toBe(sibling.id);
  await page.getByRole("button", { name: "Table options" }).click();
  await page.getByRole("menuitem", { name: "Rename view" }).click();
  const rename = page.getByRole("dialog", { name: "Rename view" });
  await rename.getByRole("textbox", { name: "View name" }).fill("Planning");
  await rename.getByRole("button", { name: "Save" }).click();
  await page.getByRole("navigation", { name: "Views" }).getByRole("button", { name: "Planning", exact: true }).click();
  await page.getByLabel("Filter items").fill("Road");
  await chip(page, "Sort");
  await page.getByLabel("Sort", { exact: true }).selectOption("title");
  await chip(page, "Group");
  await page.getByLabel("Group", { exact: true }).selectOption("status");
  await page.reload();
  await expect(page.getByRole("button", { name: "Planning", exact: true })).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByLabel("Filter items")).toHaveValue("Road");
  await chip(page, "Sort");
  await expect(page.getByLabel("Sort", { exact: true })).toHaveValue("title");
  await chip(page, "Group");
  await expect(page.getByLabel("Group", { exact: true })).toHaveValue("status");
});

test("ViewTabs duplicates, reorders, changes default, copies and deletes", async ({ page }) => {
  await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
  let saved = [table, board];
  const duplicateId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
  const { calls } = await mockSpaces(page, { extra: {
    [`GET ${path}/views`]: () => ({ body: { items: saved } }),
    [`GET ${path}/items`]: { body: { items: [] } },
    "GET /workflows": workflows,
    [`POST /views/${boardId}/duplicate`]: (body) => {
      const copy = { ...board, ...body, id: duplicateId, position: 0 };
      saved.push(copy);
      return { status: 201, body: copy };
    },
    [`PATCH /views/${boardId}`]: (body) => {
      saved = saved.map((view) => view.id === boardId ? { ...view, ...body }
        : view.id === tableId ? { ...view, position: body.position === 0 ? 1 : view.position, is_default: !body.is_default && view.is_default } : view)
        .sort((a, b) => a.visibility.localeCompare(b.visibility) || a.position - b.position);
      return { body: saved.find((view) => view.id === boardId) };
    },
    [`DELETE /views/${duplicateId}`]: () => {
      saved = saved.filter((view) => view.id !== duplicateId);
      return { status: 204 };
    },
  } });
  await page.goto(path + "?filter=Plan&sort=title");
  const nav = page.getByRole("navigation", { name: "Views" });
  await nav.getByRole("button", { name: "Board", exact: true }).click(); // options exist only on the active segment
  await nav.getByRole("button", { name: "Board options", exact: true }).click();
  await page.getByRole("menuitem", { name: "Move left" }).click();
  await expect.poll(() => calls.find((c) => c.method === "PATCH")?.body).toEqual({ position: 0 });
  await nav.getByRole("button", { name: "Board options", exact: true }).click();
  await page.getByRole("menuitem", { name: "Set as default" }).click();
  await expect(nav.getByRole("button", { name: "Board", exact: true })).toHaveAttribute("aria-pressed", "true");
  await nav.getByRole("button", { name: "Board options", exact: true }).click();
  await expect(page.getByRole("menuitem", { name: "Delete view" })).toHaveCount(0);
  await page.getByRole("menuitem", { name: "Duplicate as personal" }).click();
  await expect(nav.getByRole("button", { name: "Copy of Board", exact: true })).toHaveAttribute("aria-pressed", "true");
  await nav.getByRole("button", { name: "Copy of Board options", exact: true }).click();
  await page.getByRole("menuitem", { name: "Copy link" }).click();
  await expect(page.getByText("Link copied", { exact: true })).toBeVisible();
  const copied = await page.evaluate(() => navigator.clipboard.readText());
  const url = new URL(copied);
  expect(Object.fromEntries(url.searchParams)).toMatchObject({ view: duplicateId, filter: "Plan", sort: "title" });
  await nav.getByRole("button", { name: "Copy of Board options", exact: true }).click();
  await page.getByRole("menuitem", { name: "Delete view" }).click();
  await page.getByRole("dialog", { name: "Delete view" }).getByRole("button", { name: "Delete view" }).click();
  await expect(nav.getByRole("button", { name: "Copy of Board", exact: true })).toHaveCount(0);
});

for (const { type, label, visibility } of [
  { type: "gantt", label: "Gantt", visibility: "public" },
  { type: "calendar", label: "Calendar", visibility: "personal" },
]) {
  test(`Add view creates ${label} and restores its placeholder on reload`, async ({ page }) => {
    const saved = [table, board];
    const viewId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
    const { calls } = await mockSpaces(page, { extra: {
      [`GET ${path}/views`]: () => ({ body: { items: saved } }),
      "GET /workflows": workflows,
      "POST /views": (body) => {
        const view = { ...body, id: viewId, position: saved.filter((entry) => entry.visibility === body.visibility).length, is_default: false };
        saved.push(view);
        return { status: 201, body: view };
      },
    } });
    await page.goto(path);
    const nav = page.getByRole("navigation", { name: "Views" });
    await nav.getByRole("button", { name: "All views" }).click();
    await page.getByRole("menuitem", { name: `Add ${label} view`, exact: true }).click();
    const form = page.getByRole("dialog", { name: "Add view" });
    const name = `${label} plan`;
    const input = form.getByRole("textbox", { name: "View name" });
    await expect(input).toHaveValue(label);
    await input.fill(name);
    const visibilitySelect = form.getByRole("combobox", { name: "Visibility" });
    await expect(visibilitySelect).toHaveValue("public");
    if (visibility === "personal") await visibilitySelect.selectOption("personal");
    await form.getByRole("button", { name: "Save" }).click();
    await expect.poll(() => calls.find((call) => call.method === "POST" && call.path === "/views")?.body)
      .toEqual({ scope_space_id: marketing.id, type, name, visibility });

    const tab = nav.getByRole("button", { name, exact: true });
    // Gantt and Calendar are real views now (S2.4.2, S2.5.2).
    const heading = type === "gantt" ? page.getByRole("application", { name: "Gantt chart" }) : page.getByRole("grid", { name: "Calendar" });
    const grid = page.getByRole("table", { name: "Table" });
    await expect(tab).toHaveAttribute("aria-pressed", "true");
    await expect(heading).toBeVisible();
    await nav.getByRole("button", { name: "Table", exact: true }).click();
    await expect(grid).toBeVisible();
    await tab.click();
    await expect(page).toHaveURL(`${path}?view=${viewId}`);
    await page.reload();
    await expect(tab).toHaveAttribute("aria-pressed", "true");
    await expect(heading).toBeVisible();
    await expect(grid).toHaveCount(0);
    if (type === "gantt") await expect(page.getByLabel("Filter items")).toHaveCount(0);
  });
}

test("Board rolls back conflict, supports drag and creates in the target column", async ({ page }) => {
  let item = task;
  let reject = true;
  const { calls } = await mockSpaces(page, { extra: {
    [`GET ${path}/views`]: { body: { items: [table, board] } },
    [`GET ${path}/items`]: () => ({ body: { items: [item] } }),
    [`GET /items/${taskId}`]: () => ({ body: item, headers: { ETag: `"${item.version}"` } }),
    [`GET /items/${taskId}/children`]: { body: { items: [] } },
    "GET /workflows": workflows,
    [`PATCH /items/${taskId}`]: (body) => {
      if (reject) { reject = false; return problem(409, "Version conflict"); }
      item = { ...item, version: 2, status: body.status_id === completed.id ? completed : newStatus };
      return { body: item };
    },
    "POST /items": (body) => ({ status: 201, body: { ...item, ...body } }),
  } });
  await page.goto(path + "?view=" + boardId);
  const before = page.getByRole("region", { name: "New", exact: true });
  const after = page.getByRole("region", { name: "Completed", exact: true });
  await expect(before.getByText("2 subitems")).toBeVisible();
  await before.getByRole("button", { name: "Move to status: Plan" }).click();
  await page.getByRole("menuitem", { name: "Completed", exact: true }).click();
  await expect(page.getByText("Item was updated by someone else. Latest version loaded.", { exact: true })).toBeVisible();
  await expect(before.getByRole("button", { name: "Plan", exact: true })).toBeVisible();
  await expect(before.getByRole("button", { name: "Move to status: Plan" })).toBeEnabled();
  await before.getByRole("listitem").dragTo(after);
  await expect(after.getByRole("button", { name: "Plan", exact: true })).toBeVisible();
  await expect(after.getByRole("button", { name: "Move to status: Plan" })).toBeEnabled();
  await after.getByRole("button", { name: "Add item", exact: true }).click();
  const form = page.getByRole("dialog", { name: "Create item" });
  await form.getByRole("textbox", { name: "Name", exact: true }).fill("Checklist");
  await form.getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => calls.find((call) => call.method === "POST" && call.path === "/items")?.body)
    .toMatchObject({ type: "task", status_id: completed.id, title: "Checklist" });
});

test("Table virtualizes a 500-item list", async ({ page }) => {
  const items = Array.from({ length: 500 }, (_, index) => ({
    ...task, id: `77777777-7777-4777-8777-${String(index).padStart(12, "0")}`, title: `Task ${index}`, seq: index,
  }));
  await mockSpaces(page, { extra: {
    [`GET ${path}/views`]: { body: { items: [table, board] } },
    [`GET ${path}/items`]: { body: { items } },
    "GET /workflows": workflows,
  } });
  const times: number[] = [];
  for (let index = 0; index < 10; index++) {
    const start = performance.now();
    await page.goto(path, { waitUntil: "domcontentloaded" });
    const grid = page.getByRole("table", { name: "Table" });
    await expect(grid.getByRole("button", { name: "Task 0", exact: true })).toBeVisible();
    times.push(performance.now() - start);
    expect(await grid.getByRole("row").count()).toBeLessThan(60);
  }
  times.sort((a, b) => a - b);
  console.log(`500 items, navigation to first row, local mocked API, p95=${times[9].toFixed(0)}ms`);
  await page.getByRole("table", { name: "Table" }).focus();
  await page.keyboard.press("j");
  await page.keyboard.press("j");
  await page.keyboard.press("k");
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(new RegExp(`task=${items[1].id}`));
});
