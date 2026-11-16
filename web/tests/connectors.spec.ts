import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

test("connectors tab: list, install, disable and resolve a conflict", async ({ page }) => {
  let items: any[] = [{ id: "c1", kind: "github", name: "Backlog", config: {}, secret_names: ["token"], enabled: true, created_at: "2026-11-14T00:00:00Z", last_sync_at: null, last_sync_ok: null, last_sync_message: null }];
  let conflicts: any[] = [{ id: "x1", external_id: "7", item_title: "Mine", remote_title: "Theirs", differs: "title" }];
  const { calls } = await mockSpaces(page, { me, extra: {
    "GET /connectors": () => ({ body: { kinds: ["github", "google_calendar"], items } }),
    "POST /connectors": (b: any) => { items = [...items, { ...items[0], id: "c2", kind: b.kind, name: b.name }]; return { status: 201, body: items.at(-1) }; },
    "POST /connectors/c1/disable": () => { items = items.map((c) => (c.id === "c1" ? { ...c, enabled: false } : c)); return { body: items[0] }; },
    "POST /connectors/c1/enable": () => { items = items.map((c) => (c.id === "c1" ? { ...c, enabled: true } : c)); return { body: items[0] }; },
    "GET /connectors/c1/conflicts": () => ({ body: { items: conflicts } }),
    "GET /connectors/c2/conflicts": { body: { items: [] } },
    "POST /connector-conflicts/x1/resolve": () => { conflicts = []; return { status: 204, body: null }; },
  } });
  await page.goto("/settings/apps");
  await page.getByRole("tab", { name: "Connectors" }).click();
  await expect(page.getByText("Backlog")).toBeVisible();
  await expect(page.getByText("Enabled", { exact: true })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

  await page.getByRole("button", { name: "Disable" }).click();
  await expect(page.getByText("Disabled", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Sync now" })).toHaveCount(0);

  await page.getByRole("button", { name: "Keep remote" }).click();
  expect(calls.find((c) => c.path === "/connector-conflicts/x1/resolve")?.body).toEqual({ resolution: "remote" });
  await expect(page.getByRole("list", { name: "Sync conflicts" })).toHaveCount(0);

  await page.getByRole("button", { name: "Add connector" }).click();
  const form = page.getByRole("form", { name: "Add a connector" });
  await form.getByLabel("Type").selectOption("google_calendar");
  await form.getByLabel("Name").fill("Cal");
  await form.getByLabel("Calendar ID").fill("primary");
  await form.getByLabel("Space").selectOption({ index: 1 });
  await form.getByLabel("Google access token").fill("ya29");
  await form.getByRole("button", { name: "Install" }).click();
  await expect(page.getByText("Cal", { exact: true })).toBeVisible();
  await expect(page.getByRole("dialog", { name: "Add connector" })).toHaveCount(0);
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});
