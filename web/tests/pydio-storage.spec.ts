import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { me, problem } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

const links = { source: "none", version: 0, enabled: false, base_url: "https://files.example.com", roots: [], customer_roots: {}, max_upload_mb: 100, allowed_extensions: [], listing_enabled: false, listing_token_available: false };
const empty = { version: 0, enabled: false, base_url: "", root: "", customer_roots: {}, code_field_id: null, token_set: false, key_configured: true };
const customers = [{ id: "c1", code: "ACME", name: "Acme", status: "active" }];
const codeField = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
const fields = [{ id: codeField, name: "Project Code", type: "text", archived_at: null }, { id: "n1", name: "Budget", type: "number", archived_at: null }];

async function open(page: Page, initial: Record<string, unknown>, extra: Record<string, any> = {}) {
  let storage: any = initial;
  const mocked = await mockSpaces(page, { me, extra: {
    "GET /pydio/settings": { body: links }, "GET /clients": { body: { items: customers } }, "GET /custom-fields": { body: { items: fields } },
    "GET /pydio/storage": () => ({ body: storage }),
    "PUT /pydio/storage": (body: any) => { const { token, ...rest } = body; storage = { ...storage, ...rest, version: body.version + 1, token_set: storage.token_set || !!token }; return { body: storage }; },
    ...extra,
  } });
  await page.goto("/settings/apps");
  await page.getByRole("tab", { name: "Pydio", exact: true }).click();
  return mocked;
}
const storageForm = (page: Page) => page.getByRole("form", { name: "Pydio file storage" });

test("an administrator sets up file storage; the token is write only", async ({ page }) => {
  const { calls } = await open(page, empty);
  await expect(page.getByText("Storage off", { exact: true })).toBeVisible();
  const form = storageForm(page);
  await expect(form.getByLabel("Storage address")).toHaveValue("https://files.example.com"); // taken over from the folder links
  await expect(form.getByRole("button", { name: "Test connection" })).toHaveCount(0);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  await form.getByLabel("Turn on file storage").check();
  await form.getByLabel("Main folder").fill("pms-files");
  await form.getByLabel("Access token").fill("secret-token");
  await form.getByLabel("Project code field").selectOption(codeField);
  await expect(form.getByLabel("Project code field").getByRole("option", { name: "Budget" })).toHaveCount(0);
  await form.getByLabel("Add a Customer folder").selectOption("ACME");
  await form.getByLabel("Storage folder for ACME").fill(" cust-acme/files ");
  await form.getByRole("button", { name: "Save storage" }).click();
  await expect(page.getByText("Storage on", { exact: true })).toBeVisible();
  expect(calls.find((c) => c.method === "PUT" && c.path === "/pydio/storage")?.body).toEqual({ version: 0, enabled: true, base_url: "https://files.example.com",
    root: "pms-files", customer_roots: { ACME: "cust-acme/files" }, code_field_id: codeField, token: "secret-token" });
  await expect(form.getByLabel("Access token")).toHaveValue("");
  await expect(form.getByLabel("Access token")).toHaveAttribute("placeholder", "Saved. Leave empty to keep it.");
});

test("explains a failed check, asks for the token again for a new address and tests the connection", async ({ page }) => {
  const saved = { ...empty, version: 3, enabled: true, base_url: "https://files.example.com", root: "pms-files", token_set: true };
  let tests = 0;
  await open(page, saved, {
    "PUT /pydio/storage": problem(422, "Pydio did not accept a test file", "pydio_storage_check_failed"),
    "POST /pydio/storage/test": () => (++tests === 1 ? { status: 204, body: null } : problem(422, "no", "pydio_storage_check_failed")),
  });
  const form = storageForm(page);
  await form.getByRole("button", { name: "Test connection" }).click();
  await expect(page.getByText("Pydio accepted a test file in every folder.")).toBeVisible();
  await form.getByRole("button", { name: "Test connection" }).click();
  await expect(form.getByRole("alert")).toContainText("Pydio did not accept a test file");
  await expect(form.getByLabel("Access token")).not.toHaveAttribute("required", "");
  await form.getByLabel("Storage address").fill("https://other.example.com");
  await expect(form.getByLabel("Access token")).toHaveAttribute("required", "");
  await expect(form.getByText("Files already stored are looked for at the new address.")).toBeVisible();
});

test("warns when the server cannot keep a token", async ({ page }) => {
  await open(page, { ...empty, key_configured: false });
  await expect(page.getByText("PMS_CONNECTOR_KEY", { exact: false })).toBeVisible();
  await expect(storageForm(page).getByRole("button", { name: "Save storage" })).toBeDisabled();
});

test.describe("on a phone", () => {
  test.use({ viewport: { width: 390, height: 844 } });
  test("the storage form does not scroll sideways", async ({ page }) => {
    await open(page, { ...empty, root: "pms-files/" + "very-long-folder-name-".repeat(6) });
    await expect(storageForm(page)).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0);
  });
});

const projectId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const project = { id: projectId, seq: 7, version: 1, title: "Refinery study", type: "project", locations: [{ space_id: "33333333-3333-4333-8333-333333333333" }], assignees: [],
  my_capabilities: ["view", "edit"], created_at: "2026-11-15T10:00:00Z", updated_at: "2026-11-15T10:00:00Z" };
const automatic = { project_id: projectId, folder: "pms-files/ACME/PRJ_7", source: "default", enabled: true };

async function openProject(page: Page, extra: Record<string, any> = {}, user: typeof me = me) {
  const mocked = await mockSpaces(page, { me: user, extra: {
    [`GET /items/${projectId}`]: { body: project, headers: { ETag: '"1"' } }, [`GET /items/${projectId}/children`]: { body: { items: [] } },
    [`GET /items/${projectId}/views`]: { body: { items: [] } }, [`GET /items/${projectId}/pydio-reference`]: { body: { state: "not_configured" } },
    "GET /workflows": { body: { items: [] } }, ...extra,
  } });
  await page.goto(`/tasks/${projectId}`);
  await expect(page.getByRole("region", { name: "Documents" })).toBeVisible();
  return mocked;
}

test("an administrator sees and changes the storage folder of a project", async ({ page }) => {
  let place: any = automatic;
  const { calls } = await openProject(page, {
    [`GET /items/${projectId}/pydio-folder`]: () => ({ body: place }),
    [`PUT /items/${projectId}/pydio-folder`]: (body: any) => { if (body.folder === "taken") return problem(422, "another project already uses this folder or one inside it", "validation_failed"); place = { ...automatic, folder: body.folder, source: "project" }; return { body: place }; },
    [`DELETE /items/${projectId}/pydio-folder`]: () => { place = automatic; return { status: 204, body: null }; },
  });
  const documents = page.getByRole("region", { name: "Documents" });
  await expect(documents).toContainText("pms-files/ACME/PRJ_7");
  await expect(documents).toContainText("Automatic (main folder)");
  await documents.getByRole("button", { name: "Change storage folder" }).click();
  const dialog = page.getByRole("dialog", { name: "Storage folder" });
  await dialog.getByLabel("Folder for new files").fill("taken");
  await dialog.getByRole("button", { name: "Save folder" }).click();
  await expect(dialog.getByRole("alert")).toContainText("another project already uses this folder");
  await dialog.getByLabel("Folder for new files").fill("special/refinery");
  await dialog.getByRole("button", { name: "Save folder" }).click();
  await expect(dialog).toBeHidden();
  await expect(documents).toContainText("special/refinery");
  await expect(documents).toContainText("Set for this project");
  await documents.getByRole("button", { name: "Change storage folder" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Use the automatic folder" }).click();
  await expect(documents).toContainText("pms-files/ACME/PRJ_7");
  expect(calls.filter((c) => c.path.endsWith("/pydio-folder") && c.method !== "GET").map((c) => c.method)).toEqual(["PUT", "PUT", "DELETE"]);
});

test("a project shows no storage folder without storage settings or to other roles", async ({ page }) => {
  await openProject(page, { [`GET /items/${projectId}/pydio-folder`]: problem(404, "Pydio storage is not configured", "pydio_storage_not_configured") });
  await expect(page.getByRole("button", { name: "Change storage folder" })).toHaveCount(0);
  await expect(page.getByText("Storage folder", { exact: false })).toHaveCount(0);
});

test("a regular user never asks for the storage folder", async ({ page }) => {
  const { calls } = await openProject(page, {}, { ...me, user_type: "regular" } as typeof me);
  await expect(page.getByRole("button", { name: "Change storage folder" })).toHaveCount(0);
  expect(calls.some((c) => c.path.endsWith("/pydio-folder"))).toBe(false);
});
