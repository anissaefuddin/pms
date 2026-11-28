import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { mockApi, problem } from "./mock-api";

// Customer Picker (B2-08) through the test-only harness in tests/fixtures/customer-picker (not a product route).
// The API is mocked; the picker shows only what the mocked lookup returns. Eligibility is checked server-side (Go tests).

const ALPHA = { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", code: "ALPHA", name: "Alpha Holding", short_name: null };
const BETA = { id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", code: "BETA", name: "Beta Works", short_name: "BW" };
const OMEGA = { id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", code: "OMEGA", name: "Omega Trust", short_name: null };
const UNAVAILABLE = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
const MEMBERSHIP = [ALPHA, BETA];
const ADMINISTRATIVE = [ALPHA, BETA, OMEGA];

function lookup(scope: string, q: string) {
  const base = scope === "administrative" ? ADMINISTRATIVE : MEMBERSHIP;
  const needle = q.trim().toLowerCase();
  return base.filter((o) => !needle || [o.name, o.code, o.short_name ?? ""].join(" ").toLowerCase().includes(needle));
}

async function open(page: Page, query = "") {
  await page.goto(`/tests/fixtures/customer-picker/index.html${query}`);
  await expect(page.getByRole("group", { name: "Customer" }).first()).toBeVisible();
}

function changes(page: Page) {
  return page.evaluate(() => window.__changes);
}

test("PK-01 the picker shows only the options the lookup returned, for membership scope", async ({ page }) => {
  const calls = await mockApi(page, { "GET /clients/eligible": (_b, req) => ({ body: { items: lookup(req.query.get("scope") ?? "membership", req.query.get("q") ?? "") } }) });
  await open(page);
  const group = page.getByRole("group", { name: "Customer" });
  await expect(group.getByRole("button", { name: /Alpha Holding/ })).toBeVisible();
  await expect(group.getByRole("button", { name: /Beta Works/ })).toBeVisible();
  await expect(page.getByText("Omega Trust")).toHaveCount(0);
  const lookups = calls.filter((c) => c.path === "/clients/eligible");
  expect(lookups).toHaveLength(1);
  expect(lookups[0].query.get("scope")).toBe("membership");
  expect(lookups[0].query.get("limit")).toBe("50");
  expect(lookups[0].query.has("q")).toBe(false);
});

test("PK-02 search sends the query and shows matches by name or code", async ({ page }) => {
  const calls = await mockApi(page, { "GET /clients/eligible": (_b, req) => ({ body: { items: lookup("membership", req.query.get("q") ?? "") } }) });
  await open(page);
  await page.getByRole("searchbox", { name: "Search by name or code" }).fill("beta");
  const group = page.getByRole("group", { name: "Customer" });
  await expect(group.getByRole("button", { name: /Beta Works/ })).toBeVisible();
  await expect(group.getByRole("button", { name: /Alpha Holding/ })).toHaveCount(0);
  expect(calls.some((c) => c.path === "/clients/eligible" && c.query.get("q") === "beta")).toBe(true);
});

test("PK-03 selecting a customer returns its canonical UUID and marks it selected", async ({ page }) => {
  await mockApi(page, { "GET /clients/eligible": (_b, req) => ({ body: { items: lookup("membership", req.query.get("q") ?? "") } }) });
  await open(page);
  await page.getByRole("button", { name: /Beta Works/ }).click();
  expect(await changes(page)).toEqual([BETA.id]);
  await expect(page.getByRole("button", { name: /Beta Works/ })).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByText(`Selected: ${BETA.name} (${BETA.code})`)).toBeVisible();
});

test("PK-04 loading, empty, and error states, with retry", async ({ page }) => {
  // Loading: hold the lookup until the test releases it.
  let release!: () => void;
  const gate = new Promise<void>((resolve) => { release = resolve; });
  await page.route(/\/api\/v1\/clients\/eligible/, async (route) => {
    await gate;
    await route.fulfill({ contentType: "application/json", body: JSON.stringify({ items: MEMBERSHIP }) });
  });
  await page.goto("/tests/fixtures/customer-picker/index.html");
  await expect(page.getByText("Loading customers…")).toBeVisible();
  release();
  await expect(page.getByRole("button", { name: /Alpha Holding/ })).toBeVisible();
  await page.unrouteAll();

  // Empty: no eligible customers, and a search with no match.
  await mockApi(page, { "GET /clients/eligible": { body: { items: [] } } });
  await open(page);
  await expect(page.getByText("No customers are available to you yet.")).toBeVisible();
  await page.getByRole("searchbox", { name: "Search by name or code" }).fill("zzz");
  await expect(page.getByText("No customers match your search.")).toBeVisible();
  await page.unrouteAll();

  // Error: a server failure is shown safely, and retry fetches again.
  let failing = true;
  await mockApi(page, { "GET /clients/eligible": () => (failing ? problem(500, "database down") : { body: { items: MEMBERSHIP } }) });
  await open(page);
  await expect(page.getByRole("alert")).toContainText("Something went wrong");
  await expect(page.getByText("database down")).toHaveCount(0);
  failing = false;
  await page.getByRole("button", { name: "Try again" }).click();
  await expect(page.getByRole("button", { name: /Alpha Holding/ })).toBeVisible();
});

test("PK-05 disabled hides the choices; clear returns null without a request", async ({ page }) => {
  await mockApi(page, { "GET /clients/eligible": { body: { items: MEMBERSHIP } } });
  await open(page, "?disabled=1");
  await expect(page.getByRole("searchbox", { name: "Search by name or code" })).toBeDisabled();
  await expect(page.getByRole("button", { name: /Alpha Holding/ })).toHaveCount(0);

  await open(page, `?value=${ALPHA.id}`);
  await expect(page.getByText(`Selected: ${ALPHA.name} (${ALPHA.code})`)).toBeVisible();
  await page.getByRole("button", { name: "Clear selection" }).click();
  expect(await changes(page)).toEqual([null]);
  await expect(page.getByText(`Selected: ${ALPHA.name}`)).toHaveCount(0);
});

test("PK-06 membership and administrative lookups are separate, and only administrators can request the latter", async ({ page }) => {
  const calls = await mockApi(page, { "GET /clients/eligible": (_b, req) => ({ body: { items: lookup(req.query.get("scope") ?? "membership", "") } }) });
  await open(page);
  await expect(page.getByRole("button", { name: /Omega Trust/ })).toHaveCount(0);
  await open(page, "?scope=administrative");
  await expect(page.getByRole("button", { name: /Omega Trust/ })).toBeVisible();
  const scopes = calls.filter((c) => c.path === "/clients/eligible").map((c) => c.query.get("scope"));
  expect(scopes).toEqual(["membership", "administrative"]);

  const nonAdmin = await mockApi(page, { "GET /clients/eligible": { body: { items: ADMINISTRATIVE } } });
  await open(page, "?scope=administrative&user_type=regular");
  await expect(page.getByText("Administrator access is required to browse all customers.")).toBeVisible();
  expect(nonAdmin.filter((c) => c.path === "/clients/eligible")).toHaveLength(0);
});

test("PK-07 a previously selected customer that is no longer eligible is unavailable and never replaced", async ({ page }) => {
  await mockApi(page, { "GET /clients/eligible": { body: { items: MEMBERSHIP } } });
  await open(page, `?value=${UNAVAILABLE}`);
  await expect(page.getByRole("alert")).toContainText("not available to you");
  await expect(page.getByText("Selected:")).toHaveCount(0);
  await expect(page.getByRole("button", { name: /Alpha Holding/ })).toHaveAttribute("aria-pressed", "false");
  expect(await changes(page)).toEqual([]);
});

test("PK-08 another signed-in user triggers a fresh lookup and never reuses the earlier user's options", async ({ page }) => {
  const calls = await mockApi(page, { "GET /clients/eligible": { body: { items: MEMBERSHIP } } });
  await open(page);
  await expect(page.getByRole("button", { name: /Alpha Holding/ })).toBeVisible();
  expect(calls.filter((c) => c.path === "/clients/eligible")).toHaveLength(1);
  await page.getByRole("button", { name: "Switch user" }).click();
  await expect.poll(() => calls.filter((c) => c.path === "/clients/eligible").length).toBe(2);
});

test("PK-09 labels exist in English and Indonesian", async ({ page }) => {
  await mockApi(page, { "GET /clients/eligible": { body: { items: MEMBERSHIP } } });
  await open(page);
  await expect(page.getByRole("searchbox", { name: "Search by name or code" })).toBeVisible();
  await open(page, "?lang=id");
  await expect(page.getByRole("searchbox", { name: "Cari berdasarkan nama atau kode" })).toBeVisible();
});

test("PK-10 keyboard selection works and the picker passes axe in each state", async ({ page }) => {
  await mockApi(page, { "GET /clients/eligible": (_b, req) => ({ body: { items: lookup("membership", req.query.get("q") ?? "") } }) });
  await open(page);
  await expect(page.getByRole("button", { name: /Alpha Holding/ })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  await page.getByRole("searchbox", { name: "Search by name or code" }).focus();
  await page.keyboard.press("Tab");
  await expect(page.getByRole("button", { name: /Alpha Holding/ })).toBeFocused();
  await page.keyboard.press("Enter");
  expect(await changes(page)).toEqual([ALPHA.id]);
  await expect(page.getByRole("button", { name: /Alpha Holding/ })).toHaveAttribute("aria-pressed", "true");
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  await open(page, `?value=${UNAVAILABLE}`);
  await expect(page.getByRole("alert")).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});
