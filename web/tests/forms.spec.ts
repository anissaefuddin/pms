import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { me, problem } from "./mock-api";
import { grace, marketing, mockSpaces, ops, personal } from "./spaces-mock";

const fid = "f0000000-0000-4000-8000-000000000001";
const sid = (n: number) => `88888888-8888-4888-8888-${String(n).padStart(12, "0")}`;
const ada = { id: me.id, name: "Ada Lovelace" };
const wf = { id: sid(100), name: "Default flow", is_default: true, statuses: [
  { id: sid(1), name: "New", color: "slate", group: "active", auto_assign: false, lock_transitions: false, allowed_next: [] },
  { id: sid(2), name: "In Progress", color: "blue", group: "active", auto_assign: false, lock_transitions: false, allowed_next: [] }] };

const stored = (version = 3, status = "draft") => ({
  id: fid, space_id: marketing.id, name: "IT request", description: "Ask IT", version, status, created_by: ada, updated_by: ada,
  created_at: "2026-11-01T10:00:00Z", updated_at: "2026-11-02T10:00:00Z",
  definition: { pages: [{ title: "Details", elements: [
    { key: "your_name", kind: "single_line", label: "Your name", required: true },
    { key: "kind", kind: "single_choice", label: "Kind", options: ["Laptop", "Phone"] }] },
  { title: "More", elements: [{ key: "notes", kind: "paragraph", label: "Notes" }] }] },
  response: { item_type: "task", status_id: null, location: { space_id: marketing.id, parent_item_id: null }, assignee_ids: [], title_question: "your_name", approval: null },
});

async function mock(page: Page, opts: { spaces?: any[]; extra?: Record<string, any>; list?: any[] } = {}) {
  let current = stored();
  const { calls } = await mockSpaces(page, { spaces: opts.spaces, extra: {
    "GET /workflows": { body: { items: [wf] } },
    "GET /users": { body: { items: [grace] } },
    [`GET /users/${grace.id}`]: { body: grace },
    [`GET /spaces/${marketing.id}/forms`]: () => ({ body: { items: opts.list ?? [{ id: fid, space_id: marketing.id, name: "IT request", description: "Ask IT", question_count: 3, updated_by: ada, updated_at: "2026-11-02T10:00:00Z" }] } }),
    [`POST /spaces/${marketing.id}/forms`]: (body: any) => {
      current = { ...stored(1), ...body };
      return { status: 201, body: current, headers: { ETag: '"1"' } };
    },
    [`GET /forms/${fid}`]: () => ({ body: current, headers: { ETag: `"${current.version}"` } }),
    [`PUT /forms/${fid}`]: (body: any) => {
      current = { ...current, ...body, version: current.version + 1 };
      return { body: current, headers: { ETag: `"${current.version}"` } };
    },
    [`DELETE /forms/${fid}`]: () => ({ status: 204 }),
    [`POST /forms/${fid}/publish`]: () => { current = { ...current, status: "published", version: current.version + 1 }; return { body: current, headers: { ETag: `"${current.version}"` } }; },
    [`POST /forms/${fid}/unpublish`]: () => { current = { ...current, status: "draft", version: current.version + 1 }; return { body: current, headers: { ETag: `"${current.version}"` } }; },
    ...opts.extra,
  } });
  return calls;
}

const addQuestion = async (page: Page, kind: string) => {
  await page.getByRole("button", { name: "Add question to page 1" }).click();
  await page.getByRole("menuitem", { name: kind }).click();
};

test("list shows forms and the Space page links to it for editors", async ({ page }) => {
  await mock(page);
  await page.goto(`/spaces/${marketing.id}`);
  await page.getByRole("link", { name: "Forms" }).click();
  await expect(page).toHaveURL(`/spaces/${marketing.id}/forms`);
  const row = page.getByRole("row", { name: /IT request/ });
  await expect(row).toContainText("Ask IT");
  await expect(row.getByRole("cell").first()).toHaveText("3");
  await expect(page.getByRole("link", { name: "New form" })).toBeVisible();
});

test("empty list shows the empty state", async ({ page }) => {
  await mock(page, { list: [] });
  await page.goto(`/spaces/${marketing.id}/forms`);
  await expect(page.getByRole("heading", { name: "No forms yet" })).toBeVisible();
});

test("a Space without edit has no Forms link and the page says no permission", async ({ page }) => {
  const readOnly = { ...marketing, my_capabilities: ["view", "comment"] };
  await mock(page, { spaces: [personal, readOnly, ops] });
  await page.goto(`/spaces/${marketing.id}`);
  await expect(page.getByRole("heading", { name: "Marketing" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Forms" })).toHaveCount(0);
  await page.goto(`/spaces/${marketing.id}/forms`);
  await expect(page.getByRole("alert")).toContainText("do not have permission");
});

test("a 403 from the server shows the no-permission message", async ({ page }) => {
  await mock(page, { extra: { [`GET /spaces/${marketing.id}/forms`]: problem(403, "forbidden", "forbidden") } });
  await page.goto(`/spaces/${marketing.id}/forms`);
  await expect(page.getByRole("alert")).toContainText("do not have permission");
});

test("create: builds the request body with kinds, options, keys and response settings", async ({ page }) => {
  const calls = await mock(page);
  await page.goto(`/spaces/${marketing.id}/forms/new`);
  await page.getByLabel("Form name").fill("Laptop request");
  await addQuestion(page, "Single line text");
  await page.getByLabel("Label", { exact: true }).fill("Your full name");
  await expect(page.getByText("Key: your_full_name")).toBeVisible();
  await page.getByLabel("Required").check();
  await addQuestion(page, "Single choice");
  await page.getByLabel("Label", { exact: true }).nth(1).fill("Model");
  await page.getByLabel("Option 1", { exact: true }).fill("Air");
  await page.getByLabel("Option 2", { exact: true }).fill("Pro");
  await page.getByRole("button", { name: "Add option" }).click();
  await page.getByLabel("Option 3", { exact: true }).fill("Max");
  await page.getByRole("button", { name: "Move option 3 up" }).click(); // Air, Max, Pro
  await addQuestion(page, "Attachment");
  await page.getByLabel("Maximum files (1-5)").fill("3");
  // response settings
  await page.getByLabel("Create as").selectOption("project");
  await page.getByLabel("Destination Space").selectOption(ops.id);
  await page.getByLabel("Initial status").selectOption(sid(2));
  await page.getByLabel("Item title comes from").selectOption("your_full_name");
  await page.getByRole("searchbox", { name: /Assignees/ }).fill("gr");
  await page.getByRole("button", { name: "Grace Hopper", exact: true }).click();
  await page.getByLabel("Request approval for each new item").check();
  await page.getByRole("searchbox", { name: /Approvers/ }).fill("gr");
  await page.getByRole("button", { name: "Grace Hopper", exact: true }).click();
  await page.getByLabel("Approval description").fill("Budget check");
  await page.getByRole("button", { name: "Save form" }).click();
  await expect(page).toHaveURL(`/spaces/${marketing.id}/forms/${fid}`);
  const post = calls.find((c) => c.method === "POST" && c.path.endsWith("/forms"))!;
  expect(post.body).toEqual({
    name: "Laptop request", description: "",
    definition: { pages: [{ title: "", elements: [
      { key: "your_full_name", kind: "single_line", label: "Your full name", required: true },
      { key: "model", kind: "single_choice", label: "Model", options: ["Air", "Max", "Pro"] },
      { key: "attachment", kind: "attachment", label: "Attachment", max_files: 3 },
    ] }] },
    response: { item_type: "project", status_id: sid(2), location: { space_id: ops.id, parent_item_id: null }, assignee_ids: [grace.id],
      title_question: "your_full_name", approval: { approver_ids: [grace.id], description: "Budget check" } },
  });
});

test("edit sends If-Match and keeps the key of a saved question when its label changes", async ({ page }) => {
  const calls = await mock(page);
  await page.goto(`/spaces/${marketing.id}/forms/${fid}`);
  await expect(page.getByLabel("Form name")).toHaveValue("IT request");
  await page.getByLabel("Label", { exact: true }).first().fill("Full name of requester");
  await expect(page.getByText("Key: your_name")).toBeVisible();
  await page.getByRole("button", { name: "Move Kind up" }).click();
  await page.getByRole("button", { name: "Save form" }).click();
  await expect.poll(() => calls.find((c) => c.method === "PUT")?.headers["if-match"]).toBe('"3"');
  const put = calls.find((c) => c.method === "PUT")!;
  expect(put.body.definition.pages[0].elements.map((e: any) => e.key)).toEqual(["kind", "your_name"]);
  expect(put.body.definition.pages[0].elements[1].label).toBe("Full name of requester");
  expect(put.body.response.title_question).toBe("your_name");
});

test("a 422 shows the message next to the control and unmapped ones in a summary", async ({ page }) => {
  await mock(page, { extra: { [`PUT /forms/${fid}`]: () => ({ status: 422, body: { type: "about:blank", title: "invalid", status: 422, code: "validation", detail: "invalid", errors: [
    { field: "definition.pages[0].elements[1].options", message: "Choices need at least 2 options" },
    { field: "response.location.space_id", message: "You cannot create in this Space" },
    { field: "definition.pages", message: "At least one question is required" },
  ] } }) } });
  await page.goto(`/spaces/${marketing.id}/forms/${fid}`);
  await expect(page.getByLabel("Form name")).toHaveValue("IT request");
  await page.getByRole("button", { name: "Save form" }).click();
  await expect(page.getByText("Choices need at least 2 options")).toBeVisible();
  await expect(page.getByLabel("Destination Space")).toHaveAttribute("aria-invalid", "true");
  await expect(page.getByText("You cannot create in this Space")).toBeVisible();
  await expect(page.getByRole("alert").filter({ hasText: "Fix these problems" })).toContainText("At least one question is required");
});

test("a duplicate name and a stale version are explained", async ({ page }) => {
  await mock(page, { extra: { [`PUT /forms/${fid}`]: problem(409, "duplicate", "name_taken") } });
  await page.goto(`/spaces/${marketing.id}/forms/${fid}`);
  await page.getByLabel("Form name").fill("Other");
  await page.getByRole("button", { name: "Save form" }).click();
  await expect(page.getByText("A form with this name already exists")).toBeVisible();
});

test("a 409 conflict offers to reload the latest version", async ({ page }) => {
  await mock(page, { extra: { [`PUT /forms/${fid}`]: problem(409, "stale", "version_conflict") } });
  await page.goto(`/spaces/${marketing.id}/forms/${fid}`);
  await page.getByLabel("Form name").fill("Mine");
  await page.getByRole("button", { name: "Save form" }).click();
  await expect(page.getByText("changed by someone else")).toBeVisible();
  await page.getByRole("button", { name: "Reload latest version" }).click();
  await expect(page.getByLabel("Form name")).toHaveValue("IT request");
  await expect(page.getByText("changed by someone else")).toHaveCount(0);
});

test("delete asks for confirmation then removes the form", async ({ page }) => {
  let items: any[] | undefined;
  const calls = await mock(page, { extra: { [`GET /spaces/${marketing.id}/forms`]: () => ({ body: { items: items ?? [{ id: fid, space_id: marketing.id, name: "IT request", description: "", question_count: 3, updated_by: ada, updated_at: "2026-11-02T10:00:00Z" }] } }),
    [`DELETE /forms/${fid}`]: () => { items = []; return { status: 204 }; } } });
  await page.goto(`/spaces/${marketing.id}/forms`);
  await page.getByRole("button", { name: "Delete IT request" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete form" }).click();
  await expect(page.getByRole("heading", { name: "No forms yet" })).toBeVisible();
  expect(calls.some((c) => c.method === "DELETE")).toBe(true);
});

test("preview renders the questions page by page and cannot submit", async ({ page }) => {
  await mock(page);
  await page.goto(`/spaces/${marketing.id}/forms/${fid}`);
  const preview = page.getByRole("region", { name: "Preview" });
  await expect(preview.getByLabel(/Your name/)).toBeVisible();
  await expect(preview.getByText("Page 1 of 2")).toBeVisible();
  await expect(preview.getByRole("radio", { name: "Laptop" })).toBeVisible();
  await expect(preview.getByRole("button", { name: "Submit" })).toHaveCount(0);
  // Next checks the required answer on this page first, so the preview fills "Your name" before moving on.
  await preview.getByRole("button", { name: "Next" }).click();
  await expect(preview.getByText("Your name", { exact: false }).first()).toBeVisible();
  await preview.getByLabel(/Your name/).fill("Sam");
  await preview.getByRole("button", { name: "Next" }).click();
  await expect(preview.getByLabel("Notes")).toBeVisible();
  await expect(preview.getByRole("button", { name: "Submit" })).toBeDisabled();
  await preview.getByRole("button", { name: "Back" }).click();
  await expect(preview.getByText("Page 1 of 2")).toBeVisible();
});

for (const scheme of ["light", "dark"] as const) {
  test(`list and builder have no axe violations (${scheme})`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: scheme });
    await mock(page);
    await page.goto(`/spaces/${marketing.id}/forms`);
    await expect(page.getByRole("row", { name: /IT request/ })).toBeVisible();
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
    await page.goto(`/spaces/${marketing.id}/forms/${fid}`);
    await expect(page.getByLabel("Form name")).toHaveValue("IT request");
    await expect(page.getByLabel("Initial status")).toBeVisible();
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  });
}

test("a question can be shown only for an earlier answer; the condition is saved and the confirmation email previews escaped text", async ({ page }) => {
  const calls = await mock(page, { extra: { "GET /item-types": { body: { items: [] } } } });
  await page.goto(`/spaces/${marketing.id}/forms/${fid}`);
  await expect(page.getByLabel("Form name")).toHaveValue("IT request");
  // "Notes" (page 2) can depend on "Kind" (page 1); the first question has no earlier question to depend on.
  await page.getByLabel(/Show "Notes" conditionally/).check();
  await page.getByLabel('Earlier question for "Notes"').selectOption("kind");
  await page.getByLabel('Value for "Notes"').selectOption("Laptop");
  await expect(page.getByLabel(/Show "Your name" conditionally/)).toHaveCount(0);
  // The confirmation wording is previewed as text: markup typed by an author never becomes elements.
  await page.getByLabel("Greeting").fill("Hi <b>{name}</b>");
  await page.getByLabel("Message").fill("Got {title}\n<script>alert(1)</script>");
  const preview = page.getByRole("region", { name: "Email preview" });
  await expect(preview).toContainText("Hi <b>Sam Visitor</b>");
  await expect(preview).toContainText("Got Broken login");
  await expect(preview.locator("script, b")).toHaveCount(0);
  await page.getByRole("button", { name: "Save form" }).click();
  await expect.poll(() => calls.find((c) => c.method === "PUT")?.body).toBeTruthy();
  const put = calls.find((c) => c.method === "PUT")!;
  expect(put.body.definition.pages[1].elements[0].show_if).toEqual({ key: "kind", op: "equals", value: "Laptop" });
  expect(put.body.response.email).toMatchObject({ greeting: "Hi <b>{name}</b>" });
});

test("a route picks a Blueprint of the destination Space, shows it in its summary, and is saved as blueprint_id", async ({ page }) => {
  const bp = "b0000000-0000-4000-8000-000000000001";
  const calls = await mock(page, { extra: {
    "GET /item-types": { body: { items: [{ id: sid(50), key: "bug", name: "Bug", base: "task", is_builtin: false, active: true, defaults: {}, layout: {} }] } },
    [`GET /spaces/${marketing.id}/blueprints`]: { body: { items: [{ id: bp, name: "Bug triage", item_type: "task", item_count: 3 }] } },
  } });
  await page.goto(`/spaces/${marketing.id}/forms/${fid}`);
  await page.getByRole("button", { name: "Add a route" }).click();
  await page.getByLabel("Route 1: when").selectOption("kind");
  await page.getByLabel("Route 1: is").selectOption("Laptop");
  await page.getByLabel("Route 1: create").selectOption("bug");
  await page.getByLabel("Route 1: blueprint").selectOption(bp);
  await expect(page.getByText("Route 1: when Kind is Laptop, create Bug with blueprint: Bug triage")).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByRole("button", { name: "Save form" }).click();
  await expect.poll(() => calls.find((c) => c.method === "PUT")?.body).toBeTruthy();
  expect(calls.find((c) => c.method === "PUT")!.body.response.routes).toEqual([
    { when: { key: "kind", op: "equals", value: "Laptop" }, type_key: "bug", blueprint_id: bp }]);
});

test("a draft is published from the builder and can be moved back to draft", async ({ page }) => {
  const calls = await mock(page);
  await page.goto(`/spaces/${marketing.id}/forms/${fid}`);
  await expect(page.getByText("Draft", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Publish" }).click();
  await expect(page.getByText("Published", { exact: true })).toBeVisible();
  await expect.poll(() => calls.some((c) => c.method === "POST" && c.path === `/forms/${fid}/publish`)).toBe(true);
  await page.getByRole("button", { name: "Move to draft" }).click();
  await expect(page.getByText("Draft", { exact: true })).toBeVisible();
  await expect.poll(() => calls.some((c) => c.method === "POST" && c.path === `/forms/${fid}/unpublish`)).toBe(true);
});

test("the builder has four sections; access is chosen in Access & sharing and saved on its own", async ({ page }) => {
  const calls = await mock(page, { extra: {
    [`PUT /forms/${fid}/access`]: (body: any) => ({ body: { ...stored(4), access: body.access }, headers: { ETag: '"4"' } }),
  } });
  await page.goto(`/spaces/${marketing.id}/forms/${fid}`);
  const nav = page.getByRole("navigation", { name: "Form sections" });
  for (const label of ["Questions", "Submission settings", "Access & sharing", "Confirmation"]) {
    await expect(nav.getByRole("link", { name: label })).toBeVisible();
  }
  await nav.getByRole("link", { name: "Access & sharing" }).click();
  await expect(page.getByRole("radio", { name: /Everyone in the workspace/ })).not.toBeChecked();
  await page.getByRole("radio", { name: /Everyone in the workspace/ }).check();
  await page.locator("#sec-access").getByRole("button", { name: "Save", exact: true }).click();
  await expect.poll(() => calls.find((c) => c.method === "PUT" && c.path === `/forms/${fid}/access`)?.body).toEqual({ access: "internal" });
});

test("a new form asks for a save before access can be set", async ({ page }) => {
  await mock(page);
  await page.goto(`/spaces/${marketing.id}/forms/new`);
  await expect(page.getByText("Save the form first. Then you can choose who can fill it and create a public link.")).toBeVisible();
  await expect(page.locator("#sec-access").getByRole("button", { name: "Save", exact: true })).toHaveCount(0);
});
