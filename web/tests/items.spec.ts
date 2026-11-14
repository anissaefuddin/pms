import { expect, test } from "@playwright/test";
import { ALL, extra, grace, marketing, mockSpaces } from "./spaces-mock";
import { me, problem, type Handler } from "./mock-api";

const parentId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const childId = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const first = { id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", name: "New", color: "slate", group: "active" };
const done = { id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", name: "Completed", color: "green", group: "completed" };
const root = `/spaces/${marketing.id}`;
const itemPath = `/items/${parentId}`;
const location = { space_id: marketing.id, parent_item_id: null };
const baseItem = {
  id: parentId, type: "project", title: "Design review", seq: 1, version: 1,
  locations: [location], assignees: [], status: first, my_capabilities: ALL,
};

test("global create opens the Item panel and submits project options", async ({ page }) => {
  let item: typeof baseItem | undefined;
  let rejectOnce = true;
  const { calls } = await mockSpaces(page, { extra: extra({
    [`GET ${root}/items`]: () => ({ body: { items: item ? [item] : [] } }),
    [`GET ${itemPath}`]: () => ({ headers: { ETag: '"1"' }, body: item }),
    [`GET ${itemPath}/children`]: { body: { items: [] } },
    "POST /items": (body: Record<string, unknown>) => {
      if (rejectOnce) {
        rejectOnce = false;
        return { status: 422, body: { detail: "Invalid title", errors: [{ field: "title", message: "Choose another name" }] } };
      }
      item = { ...baseItem, ...body };
      return { status: 201, body: item };
    },
  }) });
  await page.clock.setFixedTime(new Date(2026, 10, 12, 12));
  await page.goto(root);
  await page.locator("header").first().getByRole("button", { name: "New item" }).click();
  const dialog = page.getByRole("dialog", { name: "Create item" });
  await dialog.getByRole("button", { name: "Create item" }).click();
  await expect(dialog.getByRole("alert")).toContainText("Enter an item name");
  await dialog.getByLabel("Name").fill("Plan");
  await dialog.getByRole("radio", { name: "Board" }).check();
  await dialog.getByRole("button", { name: "Start date" }).click();
  const startPicker = page.getByRole("dialog", { name: "Start date" });
  await expect(startPicker.getByRole("button", { name: /Saturday.*November 14/ })).toBeDisabled();
  await startPicker.getByRole("button", { name: /Thursday.*November 12/ }).click();
  await dialog.getByRole("button", { name: "Finish date" }).click();
  await page.getByRole("dialog", { name: "Finish date" }).getByRole("button", { name: /Friday.*November 13/ }).click();
  await dialog.getByText("Sharing", { exact: true }).click();
  await dialog.getByLabel("Add people or groups").fill("gra");
  await dialog.getByRole("button", { name: "Add Grace Hopper" }).click();
  await dialog.getByRole("button", { name: "Create item" }).click();
  await expect(dialog.getByRole("alert")).toContainText("Choose another name");
  await expect(dialog.getByLabel("Name")).toHaveAttribute("aria-invalid", "true");
  await dialog.getByRole("button", { name: "Create item" }).click();
  await expect(page).toHaveURL(`/tasks/${parentId}`);
  await expect(page.getByRole("heading", { name: "Plan" })).toBeVisible();
  expect(calls.find((call) => call.method === "POST" && call.path === "/items")?.body).toEqual({
    type: "project", title: "Plan", locations: [location], owner_id: me.id,
    start_date: "2026-11-12", due_date: "2026-11-13", default_view_type: "board",
    shares: [{ principal_type: "user", principal_id: grace.id, role: "read" }],
  });
});

test("detail actions use latest ETag and add a subitem with a parent link", async ({ page }) => {
  let parent = { ...baseItem, type: "task" };
  let child: typeof baseItem | undefined;
  const handlers: Record<string, Handler> = {
    ...extra(),
    [`GET ${itemPath}`]: () => ({ headers: { ETag: `"${parent.version}"` }, body: parent }),
    [`GET ${itemPath}/children`]: () => ({ body: { items: child ? [child] : [] } }),
    [`GET /items/${childId}`]: () => ({ headers: { ETag: '"1"' }, body: child }),
    [`GET /items/${childId}/children`]: { body: { items: [] } },
    "GET /workflows": { body: { items: [{ id: "wf", is_default: true, statuses: [first, done] }] } },
    [`PATCH ${itemPath}`]: (body, req) => {
      if (req.headers["if-match"] !== `"${parent.version}"`) return problem(409, "version conflict");
      parent = { ...parent, ...body, status: body.status_id === done.id ? done : parent.status, version: parent.version + 1 };
      return { headers: { ETag: `"${parent.version}"` }, body: parent };
    },
    "POST /items": (body) => {
      child = { ...baseItem, ...body, id: childId };
      return { status: 201, body: child };
    },
  };
  const { calls } = await mockSpaces(page, { extra: handlers });
  await page.goto(`/tasks/${parentId}`);
  await expect(page.getByRole("heading", { name: "Design review" })).toBeVisible();
  await page.getByLabel("Status", { exact: true }).selectOption(done.id);
  await expect.poll(() => parent.status.id).toBe(done.id);
  await page.getByRole("button", { name: "Item actions" }).click();
  await page.getByRole("menuitem", { name: "Convert to milestone" }).click();
  await expect.poll(() => calls.filter((c) => c.method === "PATCH").map((c) => c.headers["if-match"])).toEqual(['"1"', '"2"']);
  const description = page.locator('[contenteditable="true"][aria-label="Description"]');
  await description.fill("Agenda for the review");
  await page.getByRole("heading", { name: "Comments" }).click();
  await expect.poll(() => calls.filter((c) => c.method === "PATCH").at(-1)?.body.description?.content[0]?.content[0]?.text).toBe("Agenda for the review");
  await page.getByRole("button", { name: /^Assignees:/ }).click();
  const assignees = page.getByRole("group", { name: "Assignees", exact: true });
  await assignees.getByRole("searchbox").fill("gra");
  await assignees.getByRole("button", { name: "Grace Hopper" }).click();
  await expect.poll(() => calls.filter((c) => c.method === "PATCH").at(-1)?.body.assignee_ids).toEqual([grace.id]);
  await page.getByRole("button", { name: "Add subitem" }).click();
  const form = page.getByRole("dialog", { name: "Add subitem" });
  await expect(form.getByRole("combobox", { name: "Parent" })).toHaveCount(0);
  await form.getByLabel("Name").fill("Outline");
  await form.getByRole("button", { name: "Create item" }).click();
  await expect(page).toHaveURL(new RegExp(`/tasks/${childId}$`));
  await expect(page.getByRole("heading", { name: "Outline" })).toBeVisible();
  expect(calls.find((c) => c.method === "POST" && c.path === "/items")?.body.locations).toEqual([
    { space_id: marketing.id, parent_item_id: parentId },
  ]);
  await page.getByRole("button", { name: "Design review", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/tasks/${parentId}$`));
  await expect(page.getByRole("button", { name: "Outline" })).toBeVisible();
});

test("a stale title edit can be discarded and Delete opens a confirmation", async ({ page }) => {
  await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
  let item = baseItem;
  let rejectPatch = true;
  const { calls } = await mockSpaces(page, { extra: {
    [`GET ${itemPath}`]: () => ({ headers: { ETag: `"${item.version}"` }, body: item }),
    [`GET ${itemPath}/children`]: { body: { items: [] } },
    [`GET ${itemPath}/comments`]: { body: { items: [], next_cursor: null } },
    [`GET ${itemPath}/attachments`]: { body: { items: [], max_upload_bytes: 104857600 } },
    [`PATCH ${itemPath}`]: (body, req) => {
      if (rejectPatch) {
        rejectPatch = false;
        item = { ...item, version: 2, title: "Updated by teammate" };
        return problem(409, "version conflict", "version_conflict");
      }
      if (req.headers["if-match"] !== '"2"') return problem(409, "wrong version");
      item = { ...item, title: body.title, version: 3 };
      return { body: item, headers: { ETag: '"3"' } };
    },
    [`DELETE ${itemPath}`]: { status: 204 },
  } });
  await page.goto(`/tasks/${parentId}`);
  await page.getByRole("textbox", { name: "Name", exact: true }).fill("My draft");
  await page.getByRole("textbox", { name: "Name", exact: true }).press("Enter");
  await expect(page.getByRole("alert").filter({ hasText: "Review the latest version" })).toBeVisible();
  await page.getByRole("button", { name: "Discard my changes" }).click();
  await expect(page.getByRole("textbox", { name: "Name", exact: true })).toHaveValue("Updated by teammate");
  await page.getByRole("textbox", { name: "Name", exact: true }).fill("Reviewed");
  await page.getByRole("textbox", { name: "Name", exact: true }).press("Enter");
  await expect(page.getByRole("heading", { name: "Reviewed" })).toBeVisible();
  await page.getByRole("button", { name: "Item actions" }).click();
  await page.getByRole("menuitem", { name: "Copy link" }).click();
  await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toMatch(new RegExp(`/tasks/${parentId}$`));
  await page.getByRole("button", { name: "Item actions" }).click();
  await page.getByRole("menuitem", { name: "Delete item" }).click();
  const confirmation = page.getByRole("dialog", { name: "Delete item" });
  await expect(confirmation).toBeVisible();
  expect(calls.some((call) => call.method === "DELETE")).toBe(false);
  await confirmation.getByRole("button", { name: "Delete item" }).click();
  await expect(page).toHaveURL(`/spaces/${me.personal_space_id}`);
  expect(calls.filter((call) => call.method === "DELETE")).toHaveLength(1);
});

test("conflicted draft reapplies against the current ETag", async ({ page }) => {
  let item = baseItem;
  const { calls } = await mockSpaces(page, { extra: {
    [`GET ${itemPath}`]: () => ({ headers: { ETag: `"${item.version}"` }, body: item }),
    [`GET ${itemPath}/children`]: { body: { items: [] } },
    [`PATCH ${itemPath}`]: (body, req) => {
      if (item.version === 1) {
        item = { ...item, version: 2, title: "Teammate's edit" };
        return problem(409, "version conflict", "version_conflict");
      }
      if (req.headers["if-match"] !== '"2"') return problem(409, "wrong version");
      item = { ...item, title: body.title, version: 3 };
      return { body: item, headers: { ETag: '"3"' } };
    },
  } });
  await page.goto(`/tasks/${parentId}`);
  const title = page.getByRole("textbox", { name: "Name", exact: true });
  await title.fill("My draft");
  await title.press("Enter");
  await page.getByRole("button", { name: "Reapply my changes" }).click();
  await expect(page.getByRole("heading", { name: "My draft" })).toBeVisible();
  expect(calls.filter((call) => call.method === "PATCH").map((call) => [call.headers["if-match"], call.body.title]))
    .toEqual([['"1"', "My draft"], ['"2"', "My draft"]]);
});
