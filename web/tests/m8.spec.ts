import { AxeBuilder } from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const builtin = (key: string, base: string, name: string, id: string) => ({ id, key, name, icon: "circle", base, is_builtin: true, active: true, allowed_children: [], defaults: {}, layout: {}, version: 1 });
const taskType = builtin("task", "task", "Task", "11111111-1111-4111-8111-111111111111");
const projectType = builtin("project", "project", "Project", "22222222-2222-4222-8222-222222222222");
const bugType = { ...builtin("bug", "task", "Bug", "33333333-3333-4333-8333-333333333333"), is_builtin: false, icon: "bug" };

test("admin creates and deactivates item types with a layout; the dialog passes axe", async ({ page }) => {
  let types = [taskType, projectType, bugType];
  const { calls } = await mockSpaces(page, { extra: {
    "GET /item-types": () => ({ body: { items: types } }),
    "POST /item-types": (body: Record<string, unknown>) => { const created = { ...bugType, ...body, id: "44444444-4444-4444-8444-444444444444", is_builtin: false, active: true, version: 1 }; types = [...types, created]; return { status: 201, body: created, headers: { ETag: '"1"' } }; },
    [`PATCH /item-types/${bugType.id}`]: (body: Record<string, unknown>) => { types = types.map((t) => t.id === bugType.id ? { ...t, ...body, version: t.version + 1 } : t); return { body: types.find((t) => t.id === bugType.id), headers: { ETag: '"2"' } }; },
  } });
  await page.goto("/settings/item-types");
  await expect(page.getByText("Bug", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Create a custom item type" }).first().click();
  const dialog = page.getByRole("dialog", { name: "Create a custom item type" });
  await dialog.getByLabel("Name", { exact: true }).fill("Epic");
  await dialog.getByLabel("Key (lowercase letters, digits, _)").fill("epic");
  await dialog.getByLabel("Kind").selectOption("project");
  await dialog.locator('select[name="importance"]').selectOption("high");
  await dialog.getByRole("button", { name: "Add a section" }).click();
  await dialog.getByLabel("Section 1 title").fill("Triage");
  await dialog.getByRole("checkbox", { name: "Status" }).check();
  await dialog.getByRole("checkbox", { name: "Importance", exact: true }).check();
  await expect(dialog).toHaveCSS("opacity", "1");
  expect((await new AxeBuilder({ page }).include('[role="dialog"]').analyze()).violations).toEqual([]);
  await dialog.getByRole("button", { name: "Save" }).click();
  expect(calls.find((c) => c.method === "POST" && c.path === "/item-types")?.body).toMatchObject({
    key: "epic", name: "Epic", base: "project", defaults: { importance: "high" }, layout: { sections: [{ title: "Triage", fields: ["status", "importance"] }] } });
  await page.getByRole("button", { name: "Edit: Bug" }).click();
  const edit = page.getByRole("dialog", { name: "Edit Bug" });
  await edit.getByLabel("Available for new items").uncheck();
  await edit.getByRole("button", { name: "Save" }).click();
  await expect.poll(() => calls.find((c) => c.method === "PATCH" && c.path === `/item-types/${bugType.id}`)?.headers["if-match"]).toBe('"1"');
  expect(calls.find((c) => c.method === "PATCH")?.body).toMatchObject({ active: false });
});

test("only administrators see edit controls for item types", async ({ page }) => {
  await mockSpaces(page, { me: { ...me, user_type: "regular", capabilities: { ...me.capabilities, manage_users: false } } as typeof me, extra: { "GET /item-types": { body: { items: [taskType, bugType] } } } });
  await page.goto("/settings/item-types");
  await expect(page.getByText("Only administrators can change item types.")).toBeVisible();
  await expect(page.getByRole("button", { name: /Create a custom item type|Edit:/ })).toHaveCount(0);
});

test("a custom type appears in the create dialog and is sent as type_key", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: {
    "GET /item-types": { body: { items: [taskType, projectType, bugType] } },
    "POST /items": (body: Record<string, unknown>) => ({ status: 201, body: { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", type: "task", title: body.title, seq: 1, version: 1, assignees: [], locations: [{ space_id: marketing.id, parent_item_id: null }], my_capabilities: ALL } }),
    "GET /items/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa": { body: { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", type: "task", title: "Crash", seq: 1, version: 1, assignees: [], locations: [], my_capabilities: ALL }, headers: { ETag: '"1"' } },
  } });
  await page.goto(`/spaces/${marketing.id}`);
  await page.locator("header").first().getByRole("button", { name: "Create" }).click();
  await page.getByRole("menuitem", { name: /^Project/ }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name").fill("Crash");
  await dialog.getByRole("radio", { name: /^Task/ }).check({ force: true });
  await dialog.locator("select").filter({ has: page.locator('option[value="bug"]') }).selectOption("bug");
  await dialog.getByRole("button", { name: "Create item" }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === "/items")?.body).toMatchObject({ type: "task", type_key: "bug", title: "Crash" });
});

const sid = (n: number) => `88888888-8888-4888-8888-${String(n).padStart(12, "0")}`;
const status = (n: number, name: string, extra: object = {}) => ({ id: sid(n), name, color: "slate", group: "active", auto_assign: false, lock_transitions: false, allowed_next: [],
  required_fields: [], actor_rule: "anyone", require_children_done: false, approval_approver_ids: [], approved_status_id: null, rejected_status_id: null, ...extra });

test("the workflow editor saves entry rules, the actor rule and the approval outcome per status", async ({ page }) => {
  const wf = { id: sid(100), name: "Default flow", is_default: true, statuses: [status(1, "New"), status(2, "Review"), status(3, "Done", { group: "completed" })] };
  const { calls } = await mockSpaces(page, { extra: {
    "GET /workflows": { body: { items: [wf] } },
    [`PATCH /statuses/${sid(2)}`]: (body: Record<string, unknown>) => { wf.statuses[1] = { ...wf.statuses[1], ...body } as typeof wf.statuses[1]; return { body: wf }; },
  } });
  await page.goto("/settings/workflows");
  const patches = () => calls.filter((c) => c.method === "PATCH" && c.path === `/statuses/${sid(2)}`).map((c) => c.body);
  await page.getByRole("button", { name: "Review", exact: true }).click();
  await page.getByRole("checkbox", { name: /Needs a due date\s*:\s*Review/ }).click();
  await expect.poll(patches).toContainEqual({ required_fields: ["due_date"] });
  await page.getByLabel("Who can move items here: Review").selectOption("assignee");
  await expect.poll(patches).toContainEqual({ actor_rule: "assignee" });
  await page.getByRole("checkbox", { name: /All subitems must be completed first\s*:\s*Review/ }).click();
  await expect.poll(patches).toContainEqual({ require_children_done: true });
  await page.getByLabel("If approved, move to: Review").selectOption({ label: "Done" });
  await expect.poll(patches).toContainEqual({ approved_status_id: sid(3) });
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

const fid = "f0000000-0000-4000-8000-000000000008";
const branching = { pages: [{ title: "Report", elements: [
  { key: "kind", kind: "single_choice", label: "Kind", required: true, options: ["Bug", "Idea"] },
  { key: "steps", kind: "paragraph", label: "Steps", required: true, show_if: { key: "kind", op: "equals", value: "Bug" } },
  { key: "ref", kind: "single_line", label: "Reference" }] }] };

test("a conditional question appears only on its branch, is not required elsewhere, and stale answers are not sent", async ({ page }) => {
  const posts: string[] = [];
  await mockSpaces(page, { extra: {
    "GET /forms/available": { body: { items: [{ id: fid, name: "Report", description: "" }] } },
    [`GET /forms/${fid}/definition`]: { body: { id: fid, name: "Report", description: "", definition: branching } },
  } });
  await page.route(`**/api/v1/forms/${fid}/submissions`, async (route) => {
    posts.push(route.request().postData() ?? "");
    await route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify({ item_id: "i1", title: "x" }) });
  });
  await page.goto(`/requests/${fid}`);
  await expect(page.getByLabel(/Steps/)).toHaveCount(0);
  await page.getByRole("radio", { name: "Bug" }).check();
  await page.getByLabel(/Steps/).fill("click save");
  await page.getByRole("radio", { name: "Idea" }).check();
  await expect(page.getByLabel(/Steps/)).toHaveCount(0);
  await page.getByRole("button", { name: "Submit" }).click();
  await expect(page.getByRole("heading", { name: "Request sent" })).toBeFocused();
  expect(JSON.parse(posts[0]).answers).toMatchObject({ kind: "Idea" });
  await page.getByRole("button", { name: "Submit another request" }).click();
  await page.getByRole("radio", { name: "Bug" }).check();
  await page.getByRole("button", { name: "Submit" }).click();
  await expect(page.getByRole("alert")).toContainText("Steps");
});

const itemId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const baseItem = (extra: object = {}) => ({ id: itemId, type: "task", title: "Crash on save", seq: 1, version: 1, assignees: [], importance: "normal",
  locations: [{ space_id: marketing.id, parent_item_id: null }], my_capabilities: ALL, status: { id: "s1", name: "New", color: "slate", group: "active" },
  item_type: { id: bugType.id, key: "bug", name: "Bug", icon: "bug" }, ...extra });

async function openItem(page: import("@playwright/test").Page, item: object, extra: Record<string, unknown> = {}) {
  const calls = await mockSpaces(page, { extra: {
    [`GET /items/${itemId}`]: { body: item, headers: { ETag: '"1"' } },
    [`GET /items/${itemId}/children`]: { body: { items: [] } },
    [`GET /items/${itemId}/comments`]: { body: { items: [] } },
    [`GET /items/${itemId}/attachments`]: { body: { items: [], max_upload_bytes: 1048576 } },
    [`GET /items/${itemId}/approvals`]: { body: { items: [] } },
    [`GET /items/${itemId}/activity`]: { body: { items: [], next_cursor: null } },
    ...extra,
  } as never });
  await page.goto(`/tasks/${itemId}`);
  return calls;
}

test("the item panel groups its blocks as the item type's layout says, and keeps unlisted blocks after them", async ({ page }) => {
  await openItem(page, baseItem({ item_type: { id: bugType.id, key: "bug", name: "Bug", icon: "bug", layout: { sections: [{ title: "Triage", fields: ["importance", "status", "effort"] }, { title: "Discussion", fields: ["comments"] }] } } }));
  const triage = page.getByRole("region", { name: "Triage" });
  await expect(page.getByLabel("Status", { exact: true })).toBeVisible(); // status and importance are chips above the blocks
  await expect(page.getByLabel("Importance")).toBeVisible();
  await expect(triage.getByText("More fields")).toBeVisible();
  await expect(page.getByRole("region", { name: "Discussion" })).toBeVisible();
  await expect(page.getByRole("region", { name: "More" })).toBeVisible(); // everything the layout left out
  // Triage comes before Discussion, which comes before the leftovers.
  const order = await page.locator("section[aria-label]").evaluateAll((nodes) => nodes.map((n) => n.getAttribute("aria-label")).filter((l) => ["Triage", "Discussion", "More"].includes(l ?? "")));
  expect(order).toEqual(["Triage", "Discussion", "More"]);
  expect((await new AxeBuilder({ page }).include("main, article").analyze()).violations).toEqual([]);
});

test("an item without a layout keeps the standard panel", async ({ page }) => {
  await openItem(page, baseItem());
  await expect(page.getByLabel("Status")).toBeVisible();
  await expect(page.getByRole("region", { name: "More" })).toHaveCount(0);
});

const relField = { id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", space_id: marketing.id, type: "relation", name: "Blocks", position: 0, options: [], config: {} };

test("a relation field names the targets you can see and only counts the ones you cannot; editing is closed then", async ({ page }) => {
  await openItem(page, baseItem({ fields: { [relField.id]: ["x"] } }), {
    [`GET /spaces/${marketing.id}/custom-fields`]: { body: { items: [relField] } },
    [`GET /items/${itemId}/relations`]: { body: { items: [{ field_id: relField.id, name: "Blocks", hidden: 2, items: [{ id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", title: "Release notes", type: "task", type_key: "task" }] }] } },
  });
  const section = page.getByRole("region", { name: "Custom fields" });
  await expect(section.getByRole("link", { name: "Release notes" })).toBeVisible();
  await expect(section.getByText("2 items you cannot access")).toBeVisible();
  await expect(section.getByText("Top secret")).toHaveCount(0);
  await section.getByRole("button", { name: "Edit Blocks" }).click();
  await expect(section.getByText(/cannot be edited here/)).toBeVisible();
});

test("a relation field with only visible targets can be edited by searching", async ({ page }) => {
  const calls = await openItem(page, baseItem({ fields: {} }), {
    [`GET /spaces/${marketing.id}/custom-fields`]: { body: { items: [relField] } },
    [`GET /items/${itemId}/relations`]: { body: { items: [] } },
    "GET /search": { body: { items: [{ id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", title: "Release notes", type: "task" }], next_cursor: null } },
    [`PATCH /items/${itemId}`]: { body: baseItem({ version: 2 }), headers: { ETag: '"2"' } },
  });
  const section = page.getByRole("region", { name: "Custom fields" });
  await section.getByRole("button", { name: "Edit Blocks" }).click();
  await section.getByLabel("Search items to link").fill("rel");
  await section.getByRole("button", { name: "Link Release notes" }).click();
  await section.getByRole("button", { name: "Save" }).click();
  await expect.poll(() => calls.calls.find((c) => c.method === "PATCH")?.body).toEqual({ fields: { [relField.id]: ["cccccccc-cccc-4ccc-8ccc-cccccccccccc"] } });
});

test("an item type is a Workspace type by default, or belongs to one Space when a Space is picked", async ({ page }) => {
  let types = [taskType, projectType, bugType];
  const { calls } = await mockSpaces(page, { extra: {
    "GET /item-types": () => ({ body: { items: types } }),
    "POST /item-types": (body: Record<string, unknown>) => { const created = { ...bugType, ...body, id: `5555555${types.length}-5555-4555-8555-555555555555`, is_builtin: false, active: true, version: 1 }; types = [...types, created]; return { status: 201, body: created, headers: { ETag: '"1"' } }; },
  } });
  await page.goto("/settings/item-types");
  await page.getByRole("button", { name: "Create a custom item type" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name", { exact: true }).fill("Campaign brief");
  await dialog.getByLabel(/Key/).fill("campaign_brief");
  await dialog.getByLabel("Scope").selectOption({ label: marketing.name });
  await dialog.getByRole("button", { name: "Save", exact: true }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST")?.body?.space_id).toBe(marketing.id);
  await expect(page.getByRole("dialog")).toHaveCount(0);

  await page.getByRole("button", { name: "Create a custom item type" }).click();
  await page.getByRole("dialog").getByLabel("Name", { exact: true }).fill("Risk");
  await page.getByRole("dialog").getByLabel(/Key/).fill("risk_item");
  await page.getByRole("dialog").getByRole("button", { name: "Save", exact: true }).click();
  await expect.poll(() => calls.filter((c) => c.method === "POST").at(-1)?.body?.space_id).toBeNull();
});
