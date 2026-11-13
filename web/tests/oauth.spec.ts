import { expect, test } from "@playwright/test";
import { mockSpaces } from "./spaces-mock";

test("authorized apps tab lists and revokes", async ({ page }) => {
  let revoked = false;
  const { calls } = await mockSpaces(page, { extra: {
    "GET /oauth/grants": () => ({ body: { items: revoked ? [] : [{ id: "g1", app_name: "Sync Bot", scopes: ["read"], created_at: "2026-11-01T00:00:00Z" }] } }),
    "DELETE /oauth/grants/g1": () => { revoked = true; return { status: 204, body: null }; },
  } });
  await page.goto("/settings/apps");
  await page.getByRole("tab", { name: "Authorized apps" }).click(); // API keys is the first tab
  await expect(page.getByText("Sync Bot")).toBeVisible();
  await page.getByRole("button", { name: "Revoke Sync Bot?" }).click();
  await expect(page.getByText("No app has access to your account.")).toBeVisible();
  expect(calls.some((c) => c.method === "DELETE" && c.path === "/oauth/grants/g1")).toBe(true);
});

test("consent screen approves and rejects invalid requests", async ({ page }) => {
  await mockSpaces(page, { extra: {
    "GET /oauth/authorize": { body: { app_name: "Sync Bot", scopes: ["read", "write"] } },
    "POST /oauth/authorize": { body: { redirect_to: "http://localhost:1/cb?code=c&state=s" } },
  } });
  await page.route("http://localhost:1/**", (route) => route.fulfill({ body: "ok" }));
  await page.goto("/oauth/authorize?client_id=c&redirect_uri=http://localhost:1/cb&scope=read+write&state=s&code_challenge=x&code_challenge_method=S256");
  await expect(page.getByRole("heading", { name: "Authorize Sync Bot" })).toBeVisible();
  await page.getByRole("button", { name: "Allow" }).click();
  await page.waitForURL("http://localhost:1/cb**");
});
