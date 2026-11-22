import { expect, test, type Page } from "@playwright/test";
import { me, problem, type Reply } from "./mock-api";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

// B2.2 (Board inline create), B2.3 (Table Add subitem), B2.4 (filter-aware feedback). Mocked API, no backend needed.
const path = `/spaces/${marketing.id}`;
const nw = { id: "88888888-8888-4888-8888-888888888888", name: "New", group: "active", color: "slate" };
const done = { id: "99999999-9999-4999-8999-999999999999", name: "Done", group: "completed", color: "green" };
const existing = {
  id: "b0000000-0000-4000-8000-000000000001", seq: 1, type: "task", title: "Existing work", version: 1, status: nw, my_capabilities: ALL,
  assignees: [], locations: [{ space_id: marketing.id, space_name: "Marketing" }], created_by: { id: "a1", name: "Ada" }, status_changed_at: new Date().toISOString(),
};

const project = { ...existing, id: "b0000000-0000-4000-8000-000000000002", type: "project", title: "Parent project" };

function setup(page: Page, opts: { view?: "board" | "table"; items?: any[]; createReply?: (body: any) => Reply } = {}) {
  const view = opts.view ?? "board";
  let items: any[] = opts.items ?? [existing];
  const views = [{ id: "66666666-6666-4666-8666-666666666666", type: view, name: view === "board" ? "Board" : "Table", visibility: "public", position: 0, is_default: true, config: {} }];
  return mockSpaces(page, {
    extra: {
      [`GET ${path}/views`]: () => ({ body: { items: views } }),
      [`GET ${path}/items`]: () => ({ body: { items } }),
      "GET /workflows": { body: { items: [{ is_default: true, statuses: [nw, done] }] } },
      "POST /items": (body: any) => {
        if (opts.createReply) return opts.createReply(body);
        const created = {
          ...existing, id: `c0000000-0000-4000-8000-00000000000${items.length}`, seq: items.length + 1, title: body.title,
          status: body.status_id === done.id ? done : nw, locations: body.locations.map((l: any) => ({ space_id: l.space_id, parent_item_id: l.parent_item_id ?? undefined, space_name: "Marketing" })),
        };
        items = [...items, created];
        return { status: 201, body: created };
      },
    },
  });
}

const newColumn = (page: Page) => page.getByRole("region", { name: "New", exact: true });

test("Board: Add item opens a name field in the column; Enter creates the Task with that column's status", async ({ page }) => {
  const { calls } = await setup(page);
  await page.goto(path);
  await newColumn(page).getByRole("button", { name: "Add item" }).click();
  const name = page.getByPlaceholder("Task name");
  await expect(name).toBeFocused();
  await name.fill("Inline task");
  await name.press("Enter");
  await expect(newColumn(page).getByRole("button", { name: "Inline task", exact: true })).toBeVisible();
  const post = calls.find((c) => c.method === "POST" && c.path === "/items");
  expect(post?.body).toMatchObject({ type: "task", title: "Inline task", status_id: nw.id, locations: [{ space_id: marketing.id }] });
});

test("Board: Escape cancels the inline name and sends nothing", async ({ page }) => {
  const { calls } = await setup(page);
  await page.goto(path);
  await newColumn(page).getByRole("button", { name: "Add item" }).click();
  await page.getByPlaceholder("Task name").fill("Never sent");
  await page.getByPlaceholder("Task name").press("Escape");
  await expect(page.getByPlaceholder("Task name")).toHaveCount(0);
  expect(calls.filter((c) => c.method === "POST" && c.path === "/items")).toHaveLength(0);
});

test("Board: Enter followed by leaving the field creates the Task once", async ({ page }) => {
  const { calls } = await setup(page);
  await page.goto(path);
  await newColumn(page).getByRole("button", { name: "Add item" }).click();
  const name = page.getByPlaceholder("Task name");
  await name.fill("Once only");
  await name.press("Enter");
  await expect(newColumn(page).getByRole("button", { name: "Once only", exact: true })).toBeVisible();
  expect(calls.filter((c) => c.method === "POST" && c.path === "/items")).toHaveLength(1);
});

test("Board: a failed create keeps the typed name and shows the error, so the user can retry", async ({ page }) => {
  await setup(page, { createReply: () => problem(400, "title is too long", "validation_failed") });
  await page.goto(path);
  await newColumn(page).getByRole("button", { name: "Add item" }).click();
  const name = page.getByPlaceholder("Task name");
  await name.fill("Kept name");
  await name.press("Enter");
  await expect(page.getByRole("alert").filter({ hasText: /./ })).toBeVisible();
  await expect(page.getByPlaceholder("Task name")).toHaveValue("Kept name");
  await expect(page.getByPlaceholder("Task name")).toBeEnabled();
});

test("Board: with an active filter that hides the new Task, the feedback says it was saved and is hidden", async ({ page }) => {
  await setup(page);
  await page.goto(path);
  const filter = page.getByRole("textbox", { name: "Filter items" });
  await filter.fill("Existing");
  await newColumn(page).getByRole("button", { name: "Add item" }).click();
  await page.getByPlaceholder("Task name").fill("Hidden by filter");
  await page.getByPlaceholder("Task name").press("Enter");
  await expect(page.getByText("Task saved, but the active filter hides it.")).toBeVisible();
  await expect(filter).toHaveValue("Existing");
});

test("Table: the row menu's Add subitem opens the create dialog under that row and sends its id as parent", async ({ page }) => {
  // A Task's parent must be a Project in the client rule (create-item-validate.ts:39), so the parent here is a Project.
  const { calls } = await setup(page, { view: "table", items: [project] });
  await page.goto(path);
  // The row menu is labelled "Move <title>" when the row can also be reordered (manual sort, edit permission).
  await page.getByRole("button", { name: `Move ${project.title}`, exact: true }).click();
  await page.getByRole("menuitem", { name: "Add subtask" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await dialog.locator('input[name="title"]').fill("Child task");
  await dialog.getByRole("button", { name: "Create item" }).click();
  // The request is recorded when the mock answers, so poll rather than read the list once.
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === "/items")?.body).toMatchObject({
    title: "Child task", locations: [{ space_id: marketing.id, parent_item_id: project.id }],
  });
});

test("Table: Add subitem under a Task is allowed (a Task may sit under a Task)", async ({ page }) => {
  const { calls } = await setup(page, { view: "table" });
  await page.goto(path);
  await page.getByRole("button", { name: `Move ${existing.title}`, exact: true }).click();
  await page.getByRole("menuitem", { name: "Add subtask" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await dialog.locator('input[name="title"]').fill("Nested task");
  await dialog.getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === "/items")?.body).toMatchObject({
    title: "Nested task", locations: [{ space_id: marketing.id, parent_item_id: existing.id }],
  });
});

test("Table: a row without create or edit permission offers no row menu, so Add subitem is hidden", async ({ page }) => {
  const viewOnly = { ...existing, my_capabilities: ["view", "comment"] };
  await setup(page, { view: "table", items: [viewOnly] });
  await page.goto(path);
  const row = page.getByRole("row", { name: /Existing work/ });
  await expect(row).toBeVisible();
  await expect(row.getByRole("button", { name: /^(Move|Actions)/ })).toHaveCount(0);
});

const folder = { ...existing, id: "b0000000-0000-4000-8000-000000000003", type: "folder", title: "Team folder" };

test("Table: a Folder row offers Add subitem, the same rule as the Item Detail panel", async ({ page }) => {
  const { calls } = await setup(page, { view: "table", items: [folder] });
  await page.goto(path);
  await page.getByRole("row", { name: /Team folder/ }).getByRole("button", { name: /^(Move|Actions)/ }).click();
  await page.getByRole("menuitem", { name: "Add subtask" }).click();
  await page.getByRole("dialog", { name: "Add subtask" }).locator('input[name="title"]').fill("Folder child");
  await page.getByRole("dialog", { name: "Add subtask" }).getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === "/items")?.body).toMatchObject({
    title: "Folder child", locations: [{ space_id: marketing.id, parent_item_id: folder.id }],
  });
});

// A Folder has no item panel: its page is the container view, whose "New item" opens the same dialog with the folder as parent.
test("Folder page (Item Detail equivalent): New item creates under the folder, like the Table row's Add subitem", async ({ page }) => {
  const { calls } = await mockSpaces(page, {
    extra: {
      [`GET /items/${folder.id}`]: () => ({ headers: { ETag: '"1"' }, body: folder }),
      [`GET /items/${folder.id}/children`]: () => ({ body: { items: [] } }),
      [`GET /items/${folder.id}/activity`]: () => ({ body: { items: [], next_cursor: null } }),
      "GET /workflows": { body: { items: [{ is_default: true, statuses: [nw, done] }] } },
      "POST /items": (body: any) => ({ status: 201, body: { ...folder, id: "c0000000-0000-4000-8000-000000000009", title: body.title, type: body.type } }),
    },
  });
  await page.goto(`/items/${folder.id}`);
  await page.getByRole("button", { name: "New item", exact: true }).click();
  await page.getByRole("dialog", { name: "Add subtask" }).locator('input[name="title"]').fill("Folder detail child");
  await page.getByRole("dialog", { name: "Add subtask" }).getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === "/items")?.body).toMatchObject({
    title: "Folder detail child", locations: [{ space_id: marketing.id, parent_item_id: folder.id }],
  });
});

// R4: Table "+ Add" (the dialog) reports a create the active filter hides, and stays quiet when nothing is hidden.
const hiddenNotice = "Task saved, but the active filter hides it.";

test("Table: creating from + Add with an active filter that hides the Task says it was saved and hidden", async ({ page }) => {
  const { calls } = await setup(page, { view: "table" });
  await page.goto(path);
  await page.getByRole("textbox", { name: "Filter items" }).fill("Existing");
  await page.getByRole("button", { name: "Add item", exact: true }).click();
  await page.locator('input[name="title"]').fill("Hidden in table");
  await page.getByRole("dialog").getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === "/items")).toBeDefined();
  await expect(page.getByText(hiddenNotice)).toBeVisible();
  // The item panel covers the table, so the filter is read with includeHidden: it must still hold the user's text.
  await expect(page.getByRole("textbox", { name: "Filter items", includeHidden: true })).toHaveValue("Existing");
});

test("Table: with no filter that hides the Task, no hidden notice is shown", async ({ page }) => {
  const { calls } = await setup(page, { view: "table" });
  await page.goto(path);
  await page.getByRole("button", { name: "Add item", exact: true }).click();
  await page.locator('input[name="title"]').fill("Visible in table");
  await page.getByRole("dialog").getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === "/items")).toBeDefined();
  await expect(page.getByText("Visible in table").first()).toBeVisible();
  await expect(page.getByText(hiddenNotice)).toHaveCount(0);
});

// R6 / Option B: a retry after a lost response reuses the client id, so the item is stored once and the form succeeds.
// The first POST stores the item but answers 504 (the response is lost); the retry sends the same id and gets 409, then
// the client reads the item back, sees it is this user's with the same title, and treats it as created.
function retrySetup(page: Page, view: "board" | "table") {
  const stored: any[] = [];
  const ids: string[] = [];
  const views = [{ id: "66666666-6666-4666-8666-666666666666", type: view, name: view === "board" ? "Board" : "Table", visibility: "public", position: 0, is_default: true, config: {} }];
  const mocked = mockSpaces(page, {
    extra: {
      [`GET ${path}/views`]: () => ({ body: { items: views } }),
      [`GET ${path}/items`]: () => ({ body: { items: [existing, ...stored] } }),
      "GET /workflows": { body: { items: [{ is_default: true, statuses: [nw, done] }] } },
      "POST /items": (body: any) => {
        ids.push(body.id);
        if (stored.some((item) => item.id === body.id)) return problem(409, "item already exists", "conflict");
        stored.push({ ...existing, id: body.id, title: body.title, status: nw, created_by: { id: me.id, name: "Ada" },
          locations: body.locations.map((l: any) => ({ space_id: l.space_id, space_name: "Marketing" })) });
        return problem(504, "gateway timeout", "timeout");
      },
    },
  });
  // Route registered after mockSpaces is matched first: serves the stored item for GET /items/{id}, otherwise falls through.
  page.route(/\/api\/v1\/items\/[0-9a-f-]{36}$/, async (route) => {
    const id = route.request().url().split("/").pop()!;
    const item = stored.find((entry) => entry.id === id);
    if (route.request().method() === "GET" && item) return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(item) });
    return route.fallback();
  });
  return { ids, stored, mocked };
}

test("Retry after a lost response (dialog): the same client id is sent again, the item is stored once, the form succeeds", async ({ page }) => {
  const { ids, stored } = retrySetup(page, "table");
  await page.goto(path);
  await page.getByRole("button", { name: "Add item", exact: true }).click();
  await page.locator('input[name="title"]').fill("Retry dialog");
  await page.getByRole("dialog").getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => ids.length).toBe(1);
  await page.getByRole("dialog").getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => ids.length).toBe(2);
  expect(ids[1]).toBe(ids[0]);
  expect(stored).toHaveLength(1);
  await expect(page.locator('input[name="title"]')).toHaveCount(0);
});

test("Retry after a lost response (Board quick create): the same client id is sent again, the item is stored once", async ({ page }) => {
  const { ids, stored } = retrySetup(page, "board");
  await page.goto(path);
  await newColumn(page).getByRole("button", { name: "Add item" }).click();
  const name = page.getByPlaceholder("Task name");
  await name.fill("Retry board");
  await name.press("Enter");
  await expect.poll(() => ids.length).toBe(1);
  // The failed request finishes first: the user retries after the error shows (a second Enter during the request is ignored).
  await expect(page.getByRole("alert")).toBeVisible();
  await expect(name).toHaveValue("Retry board");
  await name.press("Enter");
  await expect.poll(() => ids.length).toBe(2);
  expect(ids[1]).toBe(ids[0]);
  expect(stored).toHaveLength(1);
});
