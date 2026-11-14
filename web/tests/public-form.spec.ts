import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me, mockApi, problem } from "./mock-api";
import { marketing, mockSpaces } from "./spaces-mock";

const definition = { pages: [{ title: "Details", elements: [
  { key: "subject", kind: "single_line", label: "Subject", required: true },
  { key: "notes", kind: "paragraph", label: "Notes" }] }] };
const view = { name: "Public help", description: "Tell us what is wrong", definition };
const ada = { id: me.id, name: "Ada Lovelace" };
const f1 = "f0000000-0000-4000-8000-000000000001";
const tokenA = "a".repeat(43), tokenB = "b".repeat(43);

test("public form works without a session, sends requester details, and shows a receipt", async ({ page }) => {
  let sessionCalls = 0;
  const calls = await mockApi(page, {
    [`GET /public/forms/${tokenA}`]: { body: view },
    [`POST /public/forms/${tokenA}/submissions`]: { status: 201, body: { received: true } },
    "GET /me": () => { sessionCalls++; return problem(401, "no session", "unauthenticated"); },
  });
  await page.goto(`/f/${tokenA}`);
  await expect(page.getByRole("heading", { name: "Public help" })).toBeVisible();
  await expect(page).toHaveURL(`/f/${tokenA}`); // no redirect to login
  const results = await new AxeBuilder({ page }).analyze();
  expect(results.violations).toEqual([]);

  await page.getByLabel("Subject").fill("Printer jammed");
  await page.getByRole("button", { name: "Submit" }).click();
  await expect(page.getByRole("alert")).toContainText("Enter your name and a valid email address first.");
  await page.getByLabel("Your name").fill("Pat Visitor");
  await page.getByLabel("Your email").fill("pat@example.com");
  await page.getByRole("button", { name: "Submit" }).click();
  await expect(page.getByRole("heading", { name: "Request received" })).toBeVisible();
  await expect(page.getByText("pat@example.com")).toBeVisible();
  const post = calls.find((call) => call.method === "POST")!;
  expect(post.body).toEqual({ answers: { subject: "Printer jammed" }, requester: { name: "Pat Visitor", email: "pat@example.com" }, website: "" });
  expect(sessionCalls).toBe(0);
});

test("the honeypot field is invisible to people and assistive tech", async ({ page }) => {
  await mockApi(page, { [`GET /public/forms/${tokenA}`]: { body: view } });
  await page.goto(`/f/${tokenA}`);
  await expect(page.getByRole("heading", { name: "Public help" })).toBeVisible();
  await expect(page.getByRole("textbox", { name: "Website" })).toHaveCount(0); // aria-hidden: not in the accessibility tree
  await expect(page.locator('[aria-hidden="true"] input')).toHaveCount(1);
});

test("a revoked link shows the unavailable page, and rate limiting shows a message", async ({ page }) => {
  await mockApi(page, {
    [`GET /public/forms/${tokenA}`]: problem(404, "form not found", "not_found"),
    [`GET /public/forms/${tokenB}`]: { body: view },
    [`POST /public/forms/${tokenB}/submissions`]: problem(429, "too many", "rate_limited"),
  });
  await page.goto(`/f/${tokenA}`);
  await expect(page.getByRole("heading", { name: "This form link is not available" })).toBeVisible();
  await page.goto(`/f/${tokenB}`);
  await page.getByLabel("Subject").fill("x");
  await page.getByLabel("Your name").fill("Pat");
  await page.getByLabel("Your email").fill("pat@example.com");
  await page.getByRole("button", { name: "Submit" }).click();
  await expect(page.getByRole("alert")).toContainText("Too many requests");
});

test("Share dialog creates, replaces and turns off the public link", async ({ page }) => {
  let token: string | null = null;
  const row = () => ({ id: f1, space_id: marketing.id, space_name: marketing.name, name: "IT request", description: "", access: "private",
    public_token: token, question_count: 2, updated_by: ada, updated_at: "2026-11-02T10:00:00Z" });
  const { calls } = await mockSpaces(page, { extra: {
    "GET /forms": () => ({ body: { items: [row()] } }),
    [`POST /forms/${f1}/public-link`]: () => { token = token ? tokenB : tokenA; return { body: row() }; },
    [`DELETE /forms/${f1}/public-link`]: () => { token = null; return { body: row() }; },
  } });
  await page.goto("/forms");
  await page.getByRole("button", { name: "Actions for IT request" }).click();
  await page.getByRole("menuitem", { name: "Share / Change access" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("No public link yet.")).toBeVisible();
  await dialog.getByRole("button", { name: "Create public link" }).click();
  await expect(dialog.getByLabel("Public link address")).toHaveValue(new RegExp(`/f/${tokenA}$`));
  await dialog.getByRole("button", { name: "Replace link" }).click();
  await expect(dialog.getByRole("alert")).toContainText("stops working immediately");
  await dialog.getByRole("button", { name: "Replace link" }).click();
  await expect(dialog.getByLabel("Public link address")).toHaveValue(new RegExp(`/f/${tokenB}$`));
  await dialog.getByRole("button", { name: "Turn off link" }).click();
  await dialog.getByRole("button", { name: "Turn off link" }).click();
  await expect(dialog.getByText("No public link yet.")).toBeVisible();
  expect(calls.filter((call) => call.path === `/forms/${f1}/public-link`).map((call) => call.method)).toEqual(["POST", "POST", "DELETE"]);
  // A button that has just finished its request is still fading from its pending colour; axe would read the in-between contrast.
  await expect(dialog.getByRole("button", { name: "Create public link" })).toBeEnabled();
  await page.waitForFunction(() => document.getAnimations().length === 0);
  const results = await new AxeBuilder({ page }).include('[role="dialog"]').analyze();
  expect(results.violations).toEqual([]);
});
