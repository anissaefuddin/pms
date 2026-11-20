import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

// The key list is the main content; create, rotate and revoke each happen in a dialog that names the key.
test("a person creates, rotates and revokes an API key; the secret is shown once", async ({ page }) => {
  let keys: any[] = [];
  const key = (n: string, scope: string) => ({ id: "k1", name: n, scope, prefix: "pmsk_abcdef", expires_at: null, last_used_at: null, created_at: "2026-11-14T00:00:00Z" });
  const { calls } = await mockSpaces(page, { me, extra: {
    "GET /api-keys": () => ({ body: { items: keys } }),
    "POST /api-keys": (body: any) => { keys = [key(body.name, body.scope)]; return { status: 201, body: { key: keys[0], secret: "pmsk_SECRET_ONE" } }; },
    "POST /api-keys/k1/rotate": () => ({ body: { key: keys[0], secret: "pmsk_SECRET_TWO" } }),
    "DELETE /api-keys/k1": () => { keys = []; return { status: 204, body: null }; },
  } });
  await page.goto("/settings/apps");
  await expect(page.getByRole("tab", { name: "API keys" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "API keys" })).toBeVisible();
  await page.getByRole("button", { name: "Create API key" }).click();
  await page.getByLabel("Key name").fill("CI");
  await page.getByRole("button", { name: "Create key" }).click();
  await expect(page.getByText("pmsk_SECRET_ONE")).toBeVisible();
  expect(calls.find((c) => c.method === "POST" && c.path === "/api-keys")?.body).toMatchObject({ name: "CI", scope: "read" });
  await page.getByRole("button", { name: "I have copied it" }).click();
  await expect(page.getByText("pmsk_SECRET_ONE")).toHaveCount(0);
  await expect(page.getByRole("row", { name: /CI pmsk_abcdef… Read only No expiry Never/ })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByRole("button", { name: "Actions for CI" }).click();
  await page.getByRole("menuitem", { name: "Rotate" }).click();
  await expect(page.getByText("pmsk_SECRET_TWO")).toBeVisible();
  await page.getByRole("button", { name: "I have copied it" }).click();
  await page.getByRole("button", { name: "Actions for CI" }).click();
  await page.getByRole("menuitem", { name: "Revoke" }).click();
  await expect(page.getByRole("dialog", { name: "Revoke CI?" })).toBeVisible();
  await page.getByRole("button", { name: "Revoke", exact: true }).click();
  await expect(page.getByText("No API keys yet.")).toBeVisible();
});
