import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me, problem } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

const samlConfig = (over: Record<string, unknown> = {}) => ({ configured: false, kind: "saml", name: "Single sign-on", enabled: false, issuer: "", client_id: "", secret_set: false, auto_link: true, allowed_domains: [],
  redirect_url: "https://pms.example.test/api/v1/auth/sso/saml/acs", key_configured: true, updated_at: null, entity_id: "https://pms.example.test/api/v1/auth/sso/saml/metadata",
  acs_url: "https://pms.example.test/api/v1/auth/sso/saml/acs", metadata_url: "https://pms.example.test/api/v1/auth/sso/saml/metadata", has_metadata: false, ...over });
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
    "GET /sso/saml": { body: samlConfig() },
  } });
  await page.goto("/settings/sso");
  await expect(page.getByRole("heading", { name: "Single sign-on", level: 1 })).toBeVisible();
  await expect(page.getByText("https://pms.example.test/api/v1/auth/sso/oidc/callback")).toBeVisible();
  await expect(page.getByLabel("Client secret")).toHaveAttribute("placeholder", /leave empty to keep it/);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  const oidc = page.getByRole("form", { name: "OpenID Connect settings" });
  await oidc.getByLabel("Allowed email domains (optional)").fill("example.com, corp.example");
  await oidc.getByRole("button", { name: "Save" }).click();
  await expect.poll(() => calls.some((c) => c.method === "PUT")).toBe(true);
  const put = calls.find((c) => c.method === "PUT")!.body;
  expect(put.allowed_domains).toEqual(["example.com", "corp.example"]);
  expect("client_secret" in put).toBe(false); // an empty secret field keeps the stored one

  // The save handler clears the secret field after the response; wait for it to finish (button re-enabled) before typing,
  // or a slow runner clears the new value (QA-D-lane3-007 flake).
  await expect(oidc.getByRole("button", { name: "Save" })).toBeEnabled();
  await page.getByLabel("Client secret").fill("new-secret-value");
  await oidc.getByRole("button", { name: "Save" }).click();
  await expect.poll(() => calls.filter((c) => c.method === "PUT").length).toBe(2);
  expect(calls.filter((c) => c.method === "PUT")[1].body.client_secret).toBe("new-secret-value");

  await oidc.getByRole("button", { name: "Remove provider" }).click();
  await expect(page.getByRole("dialog")).toContainText("all linked identities are forgotten");
  expect(calls.some((c) => c.method === "DELETE")).toBe(false);
  await page.getByRole("dialog").getByRole("button", { name: "Remove provider" }).click();
  await expect.poll(() => calls.some((c) => c.method === "DELETE")).toBe(true);
});

test("a server refusal is shown, a missing key is explained, and a regular user is told it is admin only", async ({ page }) => {
  await mockSpaces(page, { extra: {
    "GET /sso/oidc": { body: config({ configured: false, enabled: false, issuer: "", client_id: "", secret_set: false, key_configured: false, allowed_domains: [] }) },
    "PUT /sso/oidc": problem(422, "the issuer did not answer OpenID Connect discovery", "validation_failed"),
    "GET /sso/saml": { body: samlConfig() },
  } });
  await page.goto("/settings/sso");
  await expect(page.getByRole("alert").first()).toContainText("PMS_SSO_KEY is not set");
  await page.getByLabel("Issuer URL").fill("https://idp.example.com");
  await page.getByLabel("Client ID").fill("pms");
  await page.getByRole("form", { name: "OpenID Connect settings" }).getByRole("button", { name: "Save" }).click();
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

test("SAML: service provider values are shown, metadata is optional once stored, and errors are explained", async ({ page }) => {
  let current = samlConfig();
  const { calls } = await mockSpaces(page, { extra: {
    "GET /sso/oidc": { body: config({ configured: false, enabled: false, issuer: "", client_id: "", secret_set: false, allowed_domains: [] }) },
    "GET /sso/saml": () => ({ body: current }),
    "PUT /sso/saml": (body: any) => {
      if (!body.idp_metadata_xml && !current.has_metadata) return problem(422, "the identity provider metadata must be 1 byte to 256 KiB of XML", "validation_failed");
      current = samlConfig({ configured: true, name: body.name, enabled: body.enabled, has_metadata: true, idp_entity_id: "https://idp.example.com/entity", idp_sign_on_url: "https://idp.example.com/sso", allowed_domains: body.allowed_domains });
      return { body: current };
    },
    "GET /auth/sso/providers": { body: { items: [] } },
  } });
  await page.goto("/settings/sso");
  const saml = page.getByRole("region", { name: "SAML 2.0" });
  await expect(saml.getByText("https://pms.example.test/api/v1/auth/sso/saml/acs").first()).toBeVisible();
  await expect(saml.getByRole("link", { name: "https://pms.example.test/api/v1/auth/sso/saml/metadata" })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  await saml.getByRole("button", { name: "Save" }).click(); // nothing on file and nothing pasted: the browser asks for the metadata first
  expect(calls.some((c) => c.method === "PUT")).toBe(false);
  await saml.getByLabel("Identity provider metadata (XML)").fill("<EntityDescriptor/>");
  await saml.getByLabel("Show the sign-in button").check();
  await saml.getByRole("button", { name: "Save" }).click();
  await expect(saml.getByText("Provider on file: https://idp.example.com/entity")).toBeVisible();
  expect(calls.find((c) => c.method === "PUT")!.body).toMatchObject({ idp_metadata_xml: "<EntityDescriptor/>", enabled: true, auto_link: true });

  // Now the metadata is on file: the box may stay empty.
  await saml.getByLabel("Allowed email domains (optional)").fill("corp.example");
  await saml.getByRole("button", { name: "Save" }).click();
  await expect.poll(() => calls.filter((c) => c.method === "PUT").length).toBe(2);
  expect(calls.filter((c) => c.method === "PUT")[1].body.idp_metadata_xml).toBe("");
});
