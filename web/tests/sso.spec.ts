import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me, problem } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

const config = (over: Record<string, unknown> = {}) => ({ configured: true, kind: "oidc", name: "Company login", enabled: true, issuer: "https://idp.example.com", client_id: "pms",
  secret_set: true, auto_link: true, allowed_domains: ["example.com"], redirect_url: "https://pms.example.test/api/v1/auth/sso/oidc/callback", key_configured: true, updated_at: null, ...over });

test("the sign-in page offers the provider, keeps next, and explains each failure; axe is clean", async ({ page }) => {
  await page.route("**/api/v1/auth/sso/providers", (route) => route.fulfill({ json: { items: [{ kind: "oidc", name: "Company login" }] } }));
  await page.route("**/api/v1/me", (route) => route.fulfill({ status: 401, json: { code: "unauthenticated", detail: "no session" } }));
  await page.goto("/login?next=%2Fspaces%2Fabc");
  const button = page.getByRole("link", { name: "Sign in with Company login" });
  await expect(button).toHaveAttribute("href", "/api/v1/auth/sso/oidc/start?next=%2Fspaces%2Fabc");
  await expect(page.getByLabel("Email")).toBeVisible(); // passwords keep working
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  for (const [code, text] of [["state", "expired or was started in another browser"], ["unlinked", "not linked to a user here"], ["inactive", "not active"], ["unverified", "not verified your email"], ["whatever", "Single sign-on failed"]]) {
    await page.goto(`/login?sso_error=${code}`);
    await expect(page.getByRole("alert")).toContainText(text);
  }
});

test("no provider means no button", async ({ page }) => {
  await page.route("**/api/v1/auth/sso/providers", (route) => route.fulfill({ json: { items: [] } }));
  await page.route("**/api/v1/me", (route) => route.fulfill({ status: 401, json: { code: "unauthenticated", detail: "no session" } }));
  await page.goto("/login");
  await expect(page.getByLabel("Email")).toBeVisible();
  await expect(page.getByRole("link", { name: /Sign in with/ })).toHaveCount(0);
});

test("admin configures the provider: redirect URI shown, secret kept when left empty, removal asks first", async ({ page }) => {
  let current = config();
  const { calls } = await mockSpaces(page, { extra: {
    "GET /sso/oidc": () => ({ body: current }),
    "PUT /sso/oidc": (body: any) => { current = config({ name: body.name, issuer: body.issuer, client_id: body.client_id, enabled: body.enabled, auto_link: body.auto_link, allowed_domains: body.allowed_domains }); return { body: current }; },
    "DELETE /sso/oidc": () => { current = config({ configured: false, enabled: false, issuer: "", client_id: "", secret_set: false, allowed_domains: [] }); return { status: 204 }; },
    "GET /auth/sso/providers": { body: { items: [] } },
  } });
  await page.goto("/settings/sso");
  await expect(page.getByRole("heading", { name: "Single sign-on", level: 2 })).toBeVisible();
  await expect(page.getByText("https://pms.example.test/api/v1/auth/sso/oidc/callback")).toBeVisible();
  await expect(page.getByLabel("Client secret")).toHaveAttribute("placeholder", /leave empty to keep it/);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  await page.getByLabel("Allowed email domains (optional)").fill("example.com, corp.example");
  await page.getByRole("button", { name: "Save" }).click();
  await expect.poll(() => calls.some((c) => c.method === "PUT")).toBe(true);
  const put = calls.find((c) => c.method === "PUT")!.body;
  expect(put.allowed_domains).toEqual(["example.com", "corp.example"]);
  expect("client_secret" in put).toBe(false); // an empty secret field keeps the stored one

  await page.getByLabel("Client secret").fill("new-secret-value");
  await page.getByRole("button", { name: "Save" }).click();
  await expect.poll(() => calls.filter((c) => c.method === "PUT").length).toBe(2);
  expect(calls.filter((c) => c.method === "PUT")[1].body.client_secret).toBe("new-secret-value");

  await page.getByRole("button", { name: "Remove provider" }).click();
  await expect(page.getByRole("dialog")).toContainText("all linked identities are forgotten");
  expect(calls.some((c) => c.method === "DELETE")).toBe(false);
  await page.getByRole("dialog").getByRole("button", { name: "Remove provider" }).click();
  await expect.poll(() => calls.some((c) => c.method === "DELETE")).toBe(true);
});

test("a server refusal is shown, a missing key is explained, and a regular user is told it is admin only", async ({ page }) => {
  await mockSpaces(page, { extra: {
    "GET /sso/oidc": { body: config({ configured: false, enabled: false, issuer: "", client_id: "", secret_set: false, key_configured: false, allowed_domains: [] }) },
    "PUT /sso/oidc": problem(422, "the issuer did not answer OpenID Connect discovery", "validation_failed"),
  } });
  await page.goto("/settings/sso");
  await expect(page.getByRole("alert").first()).toContainText("PMS_SSO_KEY is not set");
  await page.getByLabel("Issuer URL").fill("https://idp.example.com");
  await page.getByLabel("Client ID").fill("pms");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("alert").last()).toContainText("did not answer OpenID Connect discovery");
  await page.unrouteAll();
  await mockSpaces(page, { me: { ...me, user_type: "regular", capabilities: { manage_users: false, create_space: true } } as typeof me });
  await page.goto("/settings/sso");
  await expect(page.getByRole("alert")).toContainText("Only administrators can configure single sign-on.");
  await expect(page.getByRole("link", { name: "Single sign-on" })).toHaveCount(0);
});

test("Security lists linked identities, links a new one, and unlinks with a clear message when a password is missing", async ({ page }) => {
  let linked = [{ kind: "oidc", provider_name: "Company login", email: "ada@example.test", created_at: "2026-11-01T00:00:00Z", last_login_at: null }];
  let refuse = true;
  const { calls } = await mockSpaces(page, { extra: {
    "GET /auth/sso/providers": { body: { items: [{ kind: "oidc", name: "Company login" }] } },
    "GET /me/identities": () => ({ body: { items: linked } }),
    "DELETE /me/identities/oidc": () => { if (refuse) { refuse = false; return problem(409, "set a password first", "conflict"); } linked = []; return { status: 204 }; },
    "GET /me/2fa": { body: { configured: true, enabled: false } },
  } });
  await page.goto("/settings/security");
  const section = page.getByRole("region", { name: "Single sign-on" });
  await expect(section).toContainText("ada@example.test");
  await section.getByRole("button", { name: "Unlink Company login" }).click();
  await expect(page.getByText("Set a password before unlinking")).toBeVisible();
  await section.getByRole("button", { name: "Unlink Company login" }).click();
  await expect(section.getByRole("link", { name: "Link Company login" })).toHaveAttribute("href", "/api/v1/auth/sso/oidc/start?link=1&next=%2Fsettings%2Fsecurity");
  expect(calls.filter((c) => c.method === "DELETE")).toHaveLength(2);
});
