import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

const t1 = "d0000000-0000-4000-8000-000000000001";
const token = (over: Record<string, unknown> = {}) => ({
  id: t1, name: "Okta", prefix: "pms_scim_AbCd", default_user_type: "regular", created_at: "2026-11-01T00:00:00Z", last_used_at: null, revoked_at: null, ...over });
const BASE = "https://pms.example.test/api/v1/scim/v2";

test("admin creates a token that is shown once, then revokes it after a confirmation, and axe is clean", async ({ page }) => {
  let tokens = [token()];
  const secret = "pms_scim_SECRET-VALUE-SHOWN-ONCE";
  const { calls } = await mockSpaces(page, { extra: {
    "GET /scim-tokens": () => ({ body: { items: tokens, base_url: BASE } }),
    "POST /scim-tokens": (body: any) => { const made = token({ id: "d0000000-0000-4000-8000-000000000002", name: body.name, default_user_type: body.default_user_type, prefix: "pms_scim_ZzZz" }); tokens = [made, ...tokens]; return { status: 201, body: { ...made, token: secret } }; },
    "DELETE /scim-tokens/d0000000-0000-4000-8000-000000000002": () => { tokens = tokens.map((x) => x.id.endsWith("2") ? { ...x, revoked_at: "2026-11-14T00:00:00Z" } : x); return { status: 204 }; },
  } });
  await page.goto("/settings/provisioning");
  await expect(page.getByRole("heading", { name: "Provisioning", level: 2 })).toBeVisible();
  await expect(page.getByLabel("SCIM base URL")).toHaveValue(BASE);
  const row = page.getByRole("row", { name: /Okta/ });
  await expect(row).toContainText("Never");
  await expect(row).toContainText("Active");
  await expect(row).toContainText("pms_scim_AbCd…");
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  await page.getByRole("button", { name: "New token" }).click();
  const form = page.getByRole("form", { name: "New SCIM token" });
  await expect(form.getByLabel("User type for new people").getByRole("option", { name: "Administrator" })).toHaveCount(0);
  await form.getByLabel("Name").fill("Entra ID");
  await form.getByLabel("User type for new people").selectOption("viewer");
  await form.getByRole("button", { name: "Create token" }).click();
  const shown = page.getByRole("alert").filter({ hasText: "Copy this token now" });
  await expect(shown).toBeVisible();
  await expect(shown.getByLabel("Token for Entra ID")).toHaveValue(secret);
  expect(calls.find((c) => c.method === "POST" && c.path === "/scim-tokens")!.body).toEqual({ name: "Entra ID", default_user_type: "viewer" });
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  // once dismissed the secret is gone from the page, and the list only has the prefix
  await shown.getByRole("button", { name: "I have copied it" }).click();
  await expect(page.getByText(secret)).toHaveCount(0);
  await expect(page.getByRole("row", { name: /Entra ID/ })).toContainText("pms_scim_ZzZz…");

  await page.getByRole("button", { name: "Revoke Entra ID" }).click();
  await expect(page.getByRole("dialog")).toContainText("stops working immediately");
  await page.getByRole("button", { name: "Revoke token" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.getByRole("row", { name: /Entra ID/ })).toContainText("Revoked");
  await expect(page.getByRole("button", { name: "Revoke Entra ID" })).toHaveCount(0);
});

test("a regular user is told provisioning is admin only and gets no menu entry", async ({ page }) => {
  await mockSpaces(page, { me: { ...me, user_type: "regular", capabilities: { manage_users: false, create_space: true } } as typeof me });
  await page.goto("/settings/provisioning");
  await expect(page.getByRole("alert")).toContainText("Only administrators can manage provisioning.");
  await expect(page.getByRole("link", { name: "Provisioning" })).toHaveCount(0);
});
