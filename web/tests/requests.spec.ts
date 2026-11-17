import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { problem } from "./mock-api";
import { grace, mockSpaces } from "./spaces-mock";

const fid = "f0000000-0000-4000-8000-000000000002";
const definition = { pages: [
  { title: "Basics", elements: [
    { key: "subject", kind: "single_line", label: "Subject", required: true },
    { key: "details", kind: "paragraph", label: "Details" },
    { key: "needed", kind: "date", label: "Needed by" },
    { key: "qty", kind: "number", label: "Quantity", min: 1, max: 5 },
    { key: "kind", kind: "single_choice", label: "Kind", options: ["Laptop", "Phone"] },
    { key: "extras", kind: "multiple_choice", label: "Extras", options: ["Bag", "Mouse"] },
    { key: "urgency", kind: "importance", label: "Urgency" },
    { key: "owner", kind: "assignee", label: "Owner" }] },
  { title: "Files", elements: [{ key: "doc", kind: "attachment", label: "Document", max_files: 2 }] }] };
const noFiles = { pages: [definition.pages[0]] };
const catalogEntry = { id: fid, name: "IT request", description: "Ask IT" };
const fill = (def: any = definition) => ({ id: fid, name: "IT request", description: "Ask IT", definition: def });

async function setup(page: Page, opts: { def?: any; available?: any[]; submit?: any } = {}) {
  const posts: { type: string; text: string }[] = [];
  const { calls } = await mockSpaces(page, { extra: {
    "GET /forms/available": { body: { items: opts.available ?? [{ id: fid, name: "IT request", description: "Ask IT" }] } },
    [`GET /forms/${fid}/definition`]: { body: fill(opts.def) },
    "GET /users": { body: { items: [grace] } },
  } });
  await page.route(`**/api/v1/forms/${fid}/submissions`, async (route) => {
    const req = route.request();
    posts.push({ type: req.headers()["content-type"] ?? "", text: req.postDataBuffer()?.toString("latin1") ?? "" });
    const reply = typeof opts.submit === "function" ? opts.submit(posts.length) : opts.submit ?? { status: 201, body: { item_id: "i1", title: "Laptop please" } };
    await route.fulfill({ status: reply.status, contentType: reply.status >= 400 ? "application/problem+json" : "application/json", body: JSON.stringify(reply.body) });
  });
  return { calls, posts };
}

const err = (status: number, code: string, errors?: any[]) => ({ status, body: { ...problem(422, "bad", code).body, status, code, errors } });

test("list shows available forms with Start, and the empty state", async ({ page }) => {
  await setup(page);
  await page.goto("/requests");
  await expect(page.getByRole("heading", { name: "Request forms" })).toBeVisible();
  await expect(page.getByText("Ask IT")).toBeVisible();
  await page.getByRole("link", { name: "Start IT request" }).click();
  await expect(page).toHaveURL(`/requests/${fid}`);
  await expect(page.getByRole("heading", { name: "IT request" })).toBeVisible();
});

// RF-08: the Space name appears only when the API sends it, which it does only for Spaces the requester can view.
test("catalog shows the Space name only when the requester can view that Space", async ({ page }) => {
  await setup(page, { available: [{ ...catalogEntry, space_name: "Intake" }] });
  await page.goto("/requests");
  await expect(page.getByText("Intake", { exact: true })).toBeVisible();
});

test("catalog hides the Space name from a requester without Space access", async ({ page }) => {
  await setup(page, { available: [catalogEntry] });
  await page.goto("/requests");
  await expect(page.getByRole("link", { name: "Start IT request" })).toBeVisible();
  await expect(page.getByText("Intake")).toHaveCount(0);
});

test("empty list shows the empty state", async ({ page }) => {
  await setup(page, { available: [] });
  await page.goto("/requests");
  await expect(page.getByRole("heading", { name: "No request forms available" })).toBeVisible();
});

test("sidebar links to the request forms", async ({ page }) => {
  await setup(page);
  await page.goto("/");
  await page.getByRole("button", { name: "Request forms" }).click();
  await expect(page).toHaveURL("/requests");
});

test("submits answers as JSON with the right shape per kind", async ({ page }) => {
  const { posts } = await setup(page, { def: noFiles });
  await page.goto(`/requests/${fid}`);
  await page.getByLabel(/Subject/).fill("Laptop please");
  await page.getByLabel("Details").fill("Fast one");
  await page.getByLabel("Needed by").fill("2026-12-01");
  await page.getByLabel("Quantity").fill("3");
  await page.getByRole("radio", { name: "Phone" }).check();
  await page.getByRole("checkbox", { name: "Mouse" }).check();
  await page.getByLabel("Urgency").selectOption({ label: "High" });
  await expect(page.getByLabel("Urgency").locator("option")).toHaveText(["Choose…", "High", "Normal", "Low"]);
  await page.getByLabel("Owner: Search active users").fill("gr");
  await page.getByRole("button", { name: "Grace Hopper" }).click();
  await page.getByRole("button", { name: "Submit" }).click();
  await expect(page.getByRole("heading", { name: "Request sent" })).toBeFocused();
  await expect(page.getByText("Laptop please", { exact: false }).first()).toBeVisible();
  expect(posts[0].type).toContain("application/json");
  expect(JSON.parse(posts[0].text)).toEqual({ answers: { subject: "Laptop please", details: "Fast one", needed: "2026-12-01", qty: 3,
    kind: "Phone", extras: ["Mouse"], urgency: "high", owner: grace.id } });
  await expect(page.getByRole("link", { name: "Back to request forms" })).toBeVisible();
  await page.getByRole("button", { name: "Submit another request" }).click();
  await expect(page.getByLabel(/Subject/)).toHaveValue("");
});

test("client validation blocks an empty required answer and focuses the summary", async ({ page }) => {
  const { posts } = await setup(page, { def: noFiles });
  await page.goto(`/requests/${fid}`);
  await page.getByRole("button", { name: "Submit" }).click();
  const summary = page.getByRole("alert").filter({ hasText: "problem" });
  await expect(summary).toBeFocused();
  await expect(page.getByText("This question is required.").first()).toBeVisible();
  expect(posts).toHaveLength(0);
  await page.getByLabel(/Subject/).fill("x");
  await page.getByLabel("Quantity").fill("9");
  await page.getByRole("button", { name: "Submit" }).click();
  await expect(page.getByText("Must be at most 5.").first()).toBeVisible();
});

test("submits multipart with answers and file parts", async ({ page }) => {
  const { posts } = await setup(page);
  await page.goto(`/requests/${fid}`);
  await page.getByLabel(/Subject/).fill("With file");
  await page.getByRole("button", { name: "Next" }).click();
  await page.getByLabel("Document").setInputFiles({ name: "spec.txt", mimeType: "text/plain", buffer: Buffer.from("hello") });
  await page.getByLabel("Document").setInputFiles({ name: "gone.txt", mimeType: "text/plain", buffer: Buffer.from("bye") });
  await expect(page.getByText("gone.txt")).toBeVisible();
  await page.getByRole("button", { name: "Remove gone.txt" }).click();
  await expect(page.getByText("gone.txt")).toHaveCount(0);
  await page.getByRole("button", { name: "Submit" }).click();
  await expect(page.getByRole("heading", { name: "Request sent" })).toBeVisible();
  expect(posts[0].type).toMatch(/^multipart\/form-data; boundary=/);
  expect(posts[0].text).toMatch(/name="answers"\r\n\r\n\{"subject":"With file"\}/);
  expect(posts[0].text).toMatch(/name="file:doc"; filename="spec.txt"/);
  expect(posts[0].text).not.toContain("gone.txt");
});

test("422 per-question errors show beside the question, jump to its page and focus the summary", async ({ page }) => {
  await setup(page, { submit: err(422, "validation_failed", [{ field: "answers.doc", message: "Document is not acceptable" }]) });
  await page.goto(`/requests/${fid}`);
  await page.getByLabel(/Subject/).fill("x");
  await page.getByRole("button", { name: "Next" }).click();
  await page.getByRole("button", { name: "Submit" }).click();
  await expect(page.getByText("Page 2 of 2")).toBeVisible();
  const summary = page.getByRole("alert").filter({ hasText: "problem" });
  await expect(summary).toBeFocused();
  await expect(page.getByLabel("Document")).toHaveAttribute("aria-invalid", "true");
  await expect(page.locator("p", { hasText: "Document is not acceptable" })).toBeVisible();
});

test("server error on an earlier page jumps back to it", async ({ page }) => {
  await setup(page, { submit: err(422, "validation_failed", [{ field: "answers.subject", message: "Too vague" }]) });
  await page.goto(`/requests/${fid}`);
  await page.getByLabel(/Subject/).fill("x");
  await page.getByRole("button", { name: "Next" }).click();
  await page.getByRole("button", { name: "Submit" }).click();
  await expect(page.getByText("Page 1 of 2")).toBeVisible();
  await expect(page.getByLabel(/Subject/)).toHaveAttribute("aria-invalid", "true");
});

for (const code of ["form_unavailable", "file_type_blocked"]) {
  test(`${code} shows a banner`, async ({ page }) => {
    await setup(page, { def: noFiles, submit: err(422, code) });
    await page.goto(`/requests/${fid}`);
    await page.getByLabel(/Subject/).fill("x");
    await page.getByRole("button", { name: "Submit" }).click();
    await expect(page.getByRole("alert").filter({ hasText: code === "form_unavailable" ? "Contact the form owner" : "not allowed" })).toBeFocused();
  });
}

test("file_too_large (413) shows a banner", async ({ page }) => {
  await setup(page, { def: noFiles, submit: err(413, "file_too_large") });
  await page.goto(`/requests/${fid}`);
  await page.getByLabel(/Subject/).fill("x");
  await page.getByRole("button", { name: "Submit" }).click();
  await expect(page.getByRole("alert").filter({ hasText: "too large" })).toBeVisible();
});

test("a 403 from the user directory shows a friendly note in the assignee picker", async ({ page }) => {
  await setup(page, { def: noFiles });
  await page.route("**/api/v1/users?*", (route) => route.fulfill({ status: 403, contentType: "application/problem+json", body: JSON.stringify({ title: "forbidden", status: 403, code: "forbidden" }) }));
  await page.goto(`/requests/${fid}`);
  await page.getByLabel("Owner: Search active users").fill("gr");
  await expect(page.getByText("The people directory is not available to you.")).toBeVisible();
});

test("unknown form shows not found", async ({ page }) => {
  await mockSpaces(page, { extra: { "GET /forms/nope/definition": problem(404, "not found", "not_found") } });
  await page.goto("/requests/nope");
  await expect(page.getByRole("heading", { name: "Form not found" })).toBeVisible();
});

for (const scheme of ["light", "dark"] as const) {
  test(`list and fill screens have no axe violations (${scheme})`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: scheme });
    await setup(page);
    await page.goto("/requests");
    await expect(page.getByText("Ask IT")).toBeVisible();
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
    await page.goto(`/requests/${fid}`);
    // Next checks the page first: the empty required answer is reported here and the summary takes focus.
    await page.getByRole("button", { name: "Next" }).click();
    await expect(page.getByRole("alert").filter({ hasText: "problem" })).toBeFocused();
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  });
}

test("progress lists each page; Next checks only the required answers on this page", async ({ page }) => {
  await setup(page);
  await page.goto(`/requests/${fid}`);
  const progress = page.getByRole("navigation", { name: "Form progress" });
  await expect(progress.getByText("1. Basics")).toBeVisible();
  await expect(progress.getByText("2. Files")).toBeVisible();
  await expect(progress.locator("[aria-current=step]")).toHaveText("1. Basics");
  await page.getByRole("button", { name: "Next" }).click();
  await expect(page.getByRole("alert")).toContainText("Subject");
  await expect(progress.locator("[aria-current=step]")).toHaveText("1. Basics");
  await page.getByLabel(/Subject/).fill("Laptop please");
  await page.getByRole("button", { name: "Next" }).click();
  await expect(progress.locator("[aria-current=step]")).toHaveText("2. Files");
});
