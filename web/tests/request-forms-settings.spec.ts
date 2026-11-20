import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { marketing, mockSpaces } from "./spaces-mock";

const ada = { id: me.id, name: "Ada Lovelace" };
const f1 = "f0000000-0000-4000-8000-000000000001";

test("admin reaches request forms from Settings, creates one by choosing a Space, and axe is clean", async ({ page }) => {
  const items = [{ id: f1, space_id: marketing.id, space_name: marketing.name, name: "IT request", description: "", access: "private", question_count: 2, updated_by: ada, updated_at: "2026-11-02T10:00:00Z" }];
  await mockSpaces(page, { me: { ...me, user_type: "admin" }, extra: { "GET /forms": () => ({ body: { items } }) } });
  await page.goto("/settings/request-forms");
  await expect(page.getByRole("row", { name: /IT request/ })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  await page.getByRole("button", { name: "Create form" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Space").selectOption({ label: marketing.name });
  await dialog.getByRole("button", { name: "Create form" }).click();
  await expect(page).toHaveURL(new RegExp(`/spaces/${marketing.id}/forms/new$`));
});

test("requesters see request forms as cards", async ({ page }) => {
  await mockSpaces(page, { extra: { "GET /forms/available": () => ({ body: { items: [{ id: f1, name: "IT request", description: "Laptop or access", space_name: marketing.name }] } }) } });
  await page.goto("/requests");
  const card = page.getByRole("listitem").filter({ hasText: "IT request" });
  await expect(card.getByText(marketing.name)).toBeVisible();
  await expect(card.getByRole("link", { name: "Start IT request" })).toBeVisible();
});

test("Configure emails edits one form's confirmation wording with a live preview and saves it as a form change", async ({ page }) => {
  const formFixture = (greeting = "") => ({
    id: f1, space_id: marketing.id, name: "IT request", description: "", access: "private", status: "published", version: 2, public_token: null,
    created_by: ada, updated_by: ada, created_at: "2026-11-01T10:00:00Z", updated_at: "2026-11-02T10:00:00Z",
    definition: { pages: [{ title: "Details", elements: [{ key: "subject", kind: "single_line", label: "Subject" }] }] },
    response: { item_type: "task", status_id: null, location: { space_id: marketing.id, parent_item_id: null }, assignee_ids: [], title_question: "subject", approval: null,
      email: { sender_name: "", logo_url: "", greeting, message: "", closing: "" } },
  });
  const summary = { id: f1, space_id: marketing.id, space_name: marketing.name, name: "IT request", description: "", access: "private", status: "published",
    question_count: 1, updated_by: ada, updated_at: "2026-11-02T10:00:00Z" };
  let current = formFixture();
  const { calls } = await mockSpaces(page, { me: { ...me, user_type: "admin" }, extra: {
    "GET /forms": { body: { items: [summary] } },
    [`GET /forms/${f1}`]: () => ({ body: current, headers: { ETag: `"${current.version}"` } }),
    [`PUT /forms/${f1}`]: (body: any) => { current = { ...current, response: body.response, version: current.version + 1 }; return { body: current, headers: { ETag: `"${current.version}"` } }; },
  } });
  await page.goto("/settings/request-forms");
  await page.getByRole("button", { name: "Configure emails" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Greeting").fill("Hi {name}");
  await expect(dialog.getByRole("region", { name: "Email preview" })).toContainText("Hi Sam Visitor");
  await dialog.getByRole("button", { name: "Save emails" }).click();
  await expect.poll(() => calls.find((c) => c.method === "PUT")?.body?.response?.email?.greeting).toBe("Hi {name}");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("row actions publish a draft, duplicate it as a draft copy, and open Configure emails for that form", async ({ page }) => {
  const row = { id: f1, space_id: marketing.id, space_name: marketing.name, name: "IT request", description: "", access: "private", status: "draft", question_count: 1, updated_by: { id: me.id, name: "Ada Lovelace" }, updated_at: "2026-11-02T10:00:00Z" };
  const full = { id: f1, space_id: marketing.id, name: "IT request", description: "", access: "private", status: "draft", version: 1, public_token: null,
    created_by: { id: me.id, name: "Ada Lovelace" }, updated_by: { id: me.id, name: "Ada Lovelace" }, created_at: "2026-11-01T10:00:00Z", updated_at: "2026-11-02T10:00:00Z",
    definition: { pages: [{ title: "Details", elements: [{ key: "subject", kind: "single_line", label: "Subject" }] }] },
    response: { item_type: "task", status_id: null, location: { space_id: marketing.id, parent_item_id: null }, assignee_ids: [], title_question: "subject", approval: null,
      email: { sender_name: "", logo_url: "", greeting: "", message: "", closing: "" } } };
  const { calls } = await mockSpaces(page, { me: { ...me, user_type: "admin" }, extra: {
    "GET /forms": { body: { items: [row] } },
    [`GET /forms/${f1}`]: { body: full, headers: { ETag: '"1"' } },
    [`POST /forms/${f1}/publish`]: { body: { ...full, status: "published", version: 2 }, headers: { ETag: '"2"' } },
    [`POST /spaces/${marketing.id}/forms`]: { status: 201, body: { ...full, id: "f0000000-0000-4000-8000-000000000009", name: "Copy of IT request", status: "draft" } },
  } });
  await page.goto("/settings/request-forms");
  await page.getByRole("button", { name: "Actions for IT request" }).click();
  await page.getByRole("menuitem", { name: "Publish", exact: true }).click();
  await expect.poll(() => calls.some((c) => c.method === "POST" && c.path === `/forms/${f1}/publish`)).toBe(true);

  await page.getByRole("button", { name: "Actions for IT request" }).click();
  await page.getByRole("menuitem", { name: "Duplicate", exact: true }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === `/spaces/${marketing.id}/forms`)?.body?.name).toBe("Copy of IT request");

  await page.getByRole("button", { name: "Actions for IT request" }).click();
  await page.getByRole("menuitem", { name: "Configure emails", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "Configure confirmation emails" })).toBeVisible();
});
