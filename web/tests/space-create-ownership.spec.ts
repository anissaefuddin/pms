import { expect, test } from "@playwright/test";
import { problem } from "./mock-api";
import { extra, mockSpaces } from "./spaces-mock";

// B3-SC: Create Space asks for ownership explicitly. The Customer picker is the B2-08 membership lookup; the backend
// revalidates membership and Customer status on every create. Mocked API, the real app (mocked UI verification).

const ACME = { id: "77777777-7777-4777-8777-777777777777", code: "ACME", name: "Acme Co" };

async function openCreate(page: import("@playwright/test").Page) {
  await page.goto("/");
  await page.getByRole("button", { name: "New Space" }).click();
  return page.getByRole("dialog", { name: "New Space" });
}

test("ownership has no default and is required before the create request", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: extra() });
  const dialog = await openCreate(page);
  await expect(dialog.getByRole("radio", { name: /^Internal/ })).not.toBeChecked();
  await expect(dialog.getByRole("radio", { name: /^Customer/ })).not.toBeChecked();
  await dialog.getByLabel("Name").fill("Research");
  await dialog.getByRole("button", { name: "Create Space" }).click();
  await expect(dialog.getByRole("alert")).toHaveText("Choose whether this Space is internal or belongs to a Customer");
  expect(calls.some((c) => c.method === "POST")).toBe(false);
});

test("Customer ownership requires a selection from the membership lookup", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: { ...extra(), "GET /clients/eligible": { body: { items: [ACME] } } } });
  const dialog = await openCreate(page);
  await dialog.getByLabel("Name").fill("Launch");
  await dialog.getByRole("radio", { name: /^Customer/ }).check();
  await dialog.getByRole("button", { name: "Create Space" }).click();
  await expect(dialog.getByRole("alert")).toHaveText("Choose the Customer for this Space");
  expect(calls.some((c) => c.method === "POST")).toBe(false);

  const lookups = calls.filter((c) => c.path === "/clients/eligible");
  expect(lookups.length).toBeGreaterThan(0);
  expect(lookups.every((c) => c.query.get("scope") === "membership")).toBe(true);
  await dialog.getByRole("button", { name: /Acme Co/ }).click();
  await dialog.getByRole("button", { name: "Create Space" }).click();
  await expect(dialog).toBeHidden();
  expect(calls.find((c) => c.method === "POST" && c.path === "/spaces")?.body).toEqual({ name: "Launch", scope_state: "client", client_id: ACME.id });
});

test("switching back to Internal clears the Customer", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: { ...extra(), "GET /clients/eligible": { body: { items: [ACME] } } } });
  const dialog = await openCreate(page);
  await dialog.getByLabel("Name").fill("Ops notes");
  await dialog.getByRole("radio", { name: /^Customer/ }).check();
  await dialog.getByRole("button", { name: /Acme Co/ }).click();
  await dialog.getByRole("radio", { name: /^Internal/ }).check();
  await expect(dialog.getByRole("searchbox", { name: "Search by name or code" })).toHaveCount(0);
  await dialog.getByRole("button", { name: "Create Space" }).click();
  await expect(dialog).toBeHidden();
  const body = calls.find((c) => c.method === "POST" && c.path === "/spaces")?.body as Record<string, unknown>;
  expect(body.scope_state).toBe("internal");
  expect(body).not.toHaveProperty("client_id");
});

test("a refused Customer shows the server message and keeps the dialog open", async ({ page }) => {
  await mockSpaces(page, { extra: { ...extra(), "GET /clients/eligible": { body: { items: [ACME] } },
    "POST /spaces": problem(403, "you must be an active member of this Customer to create its Space", "forbidden") } });
  const dialog = await openCreate(page);
  await dialog.getByLabel("Name").fill("Revoked");
  await dialog.getByRole("radio", { name: /^Customer/ }).check();
  await dialog.getByRole("button", { name: /Acme Co/ }).click();
  await dialog.getByRole("button", { name: "Create Space" }).click();
  await expect(dialog.getByRole("alert")).toHaveText("you must be an active member of this Customer to create its Space");
  await expect(dialog).toBeVisible();
});

test("ownership labels render in English", async ({ page }) => {
  await mockSpaces(page, { extra: extra() });
  const dialog = await openCreate(page);
  await expect(dialog.getByRole("radiogroup", { name: "Ownership" })).toBeVisible();
  await expect(dialog.getByText("Shared inside the workspace, not tied to a Customer.")).toBeVisible();
});
