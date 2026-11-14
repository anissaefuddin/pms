import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { marketing, mockSpaces } from "./spaces-mock";

const install = (over: object) => ({ id: "c1", kind: "github", name: "Repo", config: {}, secret_names: ["token"], enabled: true, created_at: "2026-11-14T00:00:00Z", last_sync_at: null, last_sync_ok: null, last_sync_message: null, ...over });

test("an admin installs the GitHub and Google connectors and sees the last sync status", async ({ page }) => {
  let items: any[] = [];
  const { calls } = await mockSpaces(page, { me, extra: {
    "GET /connectors": () => ({ body: { kinds: ["github", "google_calendar"], items } }),
    "POST /connectors": (body: any) => { items = [...items, install({ id: `c${items.length + 1}`, kind: body.kind, name: body.name, last_sync_at: "2026-11-14T01:00:00Z", last_sync_ok: items.length === 0, last_sync_message: items.length === 0 ? "pushed 1, pulled 2, conflicts 0" : "GitHub rejected the token (401): replace it" })]; return { status: 201, body: items.at(-1) }; },
    "GET /connectors/c1/conflicts": { body: { items: [] } },
    "GET /connectors/c2/conflicts": { body: { items: [] } },
  } });
  await page.goto("/settings/apps");
  await page.getByRole("tab", { name: "Connectors" }).click();
  const form = page.getByRole("form", { name: "Add a connector" });
  await expect(form).toBeVisible();
  await form.getByLabel("Name").fill("Backlog");
  await form.getByLabel("Repository").fill("acme/backlog");
  await form.getByLabel("Space").selectOption({ label: marketing.name });
  await form.getByLabel("Access token").fill("ghp_test");
  await form.getByLabel("Create issues for new tasks").check();
  await form.getByRole("button", { name: "Install" }).click();
  await expect(page.getByText(/Last sync .*: pushed 1, pulled 2, conflicts 0/)).toBeVisible();
  const post = calls.find((c) => c.method === "POST" && c.path === "/connectors")?.body as any;
  expect(post).toMatchObject({ kind: "github", name: "Backlog", config: { repo: "acme/backlog", space_id: marketing.id, import: true, push_new: true }, secrets: { token: "ghp_test" } });

  await form.getByLabel("Type").selectOption("google_calendar");
  await expect(form.getByLabel("Calendar ID")).toBeVisible();
  await form.getByLabel("Name").fill("Team calendar");
  await form.getByLabel("Calendar ID").fill("team@group.calendar.google.com");
  await form.getByLabel("Space").selectOption({ label: marketing.name });
  await form.getByLabel("Google access token").fill("ya29.test");
  await form.getByRole("button", { name: "Install" }).click();
  await expect(page.getByText(/Last sync failed .*: GitHub rejected the token/)).toBeVisible();
  const google = calls.filter((c) => c.method === "POST" && c.path === "/connectors")[1].body as any;
  expect(google).toMatchObject({ kind: "google_calendar", config: { calendar_id: "team@group.calendar.google.com", space_id: marketing.id }, secrets: { token: "ya29.test" } });
  expect(google.config.import).toBeUndefined();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});
