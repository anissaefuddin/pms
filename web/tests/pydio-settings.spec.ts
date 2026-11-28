import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { me, problem } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

const fromEnv = { source: "environment", version: 0, enabled: true, base_url: "https://dms.example.com", roots: ["projects", "common-files/Clients"],
  customer_roots: { ACME: ["projects/acme"] }, max_upload_mb: 100, allowed_extensions: [], listing_enabled: false, listing_token_available: false };
const customers = [
  { id: "c1", code: "ACME", name: "Acme", status: "active" },
  { id: "c2", code: "BETA", name: "Beta Corp", status: "active" },
];

async function open(page: Page, initial: Record<string, unknown>, extra: Record<string, any> = {}) {
  let settings: any = initial;
  const mocked = await mockSpaces(page, { me, extra: {
    "GET /pydio/settings": () => ({ body: settings }),
    "PUT /pydio/settings": (body: any) => { settings = { ...fromEnv, ...body, source: "database", version: body.version + 1 }; return { body: settings }; },
    "DELETE /pydio/settings": () => { settings = fromEnv; return { status: 204, body: null }; },
    "GET /clients": { body: { items: customers } },
    "GET /pydio/storage": { body: { version: 0, enabled: false, base_url: "", root: "", customer_roots: {}, code_field_id: null, token_set: false, key_configured: true } },
    "GET /custom-fields": { body: { items: [] } },
    ...extra,
  } });
  await page.goto("/settings/apps");
  await page.getByRole("tab", { name: "Pydio", exact: true }).click();
  return mocked;
}

test("an administrator saves the settings and returns to the environment", async ({ page }) => {
  const { calls } = await open(page, fromEnv);
  await expect(page.getByText("From the server environment")).toBeVisible();
  await expect(page.getByText("File listing: off")).toBeVisible();
  const form = page.getByRole("form", { name: "Pydio folder links" });
  await expect(form.getByLabel("Pydio address")).toHaveValue("https://dms.example.com");
  await expect(form.getByLabel("Allowed folders")).toHaveValue("projects\ncommon-files/Clients");
  await expect(form.getByLabel("Folders for ACME")).toHaveValue("projects/acme");
  await expect(page.getByRole("button", { name: "Use environment values" })).toHaveCount(0);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  await form.getByLabel("Pydio address").fill("https://files.example.com");
  await form.getByLabel("Add Customer").selectOption("BETA");
  await form.getByLabel("Folders for BETA").fill("projects/beta\n\n projects/beta-2 ");
  await form.getByRole("button", { name: "Remove ACME" }).click();
  await expect(form.getByLabel("Maximum size per file (MB)")).toHaveValue("100");
  await expect(form.getByLabel("Allowed file types")).toHaveValue("");
  await expect(form.getByText("local attachments are not affected")).toBeVisible();
  await form.getByLabel("Maximum size per file (MB)").fill("250");
  await form.getByLabel("Allowed file types").fill("pdf, .DOCX\nxlsx");
  await form.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Saved settings", { exact: true })).toBeVisible();
  expect(calls.find((c) => c.method === "PUT" && c.path === "/pydio/settings")?.body).toEqual({ version: 0, enabled: true, base_url: "https://files.example.com",
    roots: ["projects", "common-files/Clients"], customer_roots: { BETA: ["projects/beta", "projects/beta-2"] },
    max_upload_mb: 250, allowed_extensions: ["pdf", ".DOCX", "xlsx"] });

  await page.getByRole("button", { name: "Use environment values" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Remove saved settings" }).click();
  await expect(page.getByText("From the server environment")).toBeVisible();
  await expect(form.getByLabel("Pydio address")).toHaveValue("https://dms.example.com");
});

test("shows why the server refused and keeps what was typed", async ({ page }) => {
  await open(page, { ...fromEnv, source: "none", enabled: false, base_url: "", roots: [], customer_roots: {} }, {
    "PUT /pydio/settings": problem(422, "base_url must be an https address", "validation_failed"),
  });
  await expect(page.getByText("Not configured")).toBeVisible();
  const form = page.getByRole("form", { name: "Pydio folder links" });
  await form.getByLabel("Pydio address").fill("https://files.example.com/ws");
  await form.getByLabel("Allowed folders").fill("projects");
  await form.getByRole("button", { name: "Save" }).click();
  await expect(form.getByRole("alert")).toHaveText("base_url must be an https address");
  await expect(form.getByLabel("Pydio address")).toHaveValue("https://files.example.com/ws");
});

test("offers to reload when someone else changed the settings", async ({ page }) => {
  let version = 1;
  await open(page, fromEnv, {
    "GET /pydio/settings": () => ({ body: { ...fromEnv, source: "database", version, base_url: `https://v${version}.example.com` } }),
    "PUT /pydio/settings": () => { version = 2; return problem(409, "changed by someone else", "version_conflict"); },
  });
  const form = page.getByRole("form", { name: "Pydio folder links" });
  await form.getByRole("button", { name: "Save" }).click();
  await expect(form.getByRole("alert")).toContainText("Someone else changed these settings");
  await form.getByRole("button", { name: "Load the current settings" }).click();
  await expect(form.getByLabel("Pydio address")).toHaveValue("https://v2.example.com");
});

test("the gallery opens the Pydio settings", async ({ page }) => {
  await mockSpaces(page, { me, extra: { "GET /pydio/settings": { body: fromEnv }, "GET /clients": { body: { items: customers } } } });
  await page.goto("/settings/apps");
  await page.getByRole("button", { name: "Configure Pydio Cells" }).click();
  await expect(page.getByRole("form", { name: "Pydio folder links" })).toBeVisible();
});

test.describe("on a phone", () => {
  test.use({ viewport: { width: 390, height: 844 } });
  test("the Pydio settings do not scroll sideways", async ({ page }) => {
    await open(page, { ...fromEnv, roots: ["projects/" + "very-long-folder-name-".repeat(6)] });
    await expect(page.getByRole("form", { name: "Pydio folder links" })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0);
  });
});
