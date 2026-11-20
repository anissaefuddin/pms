import { AxeBuilder } from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { marketing, mockSpaces } from "./spaces-mock";

// AUT-02 UI: Workspace Settings > Automation (admin), the targets dialog with Select all, and the read-only list on a Space.
const rid = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaacc";
const rule = { id: rid, space_id: "", scope: "workspace", container_id: null, name: "Close everywhere", owner_id: "u", owner_name: "Grace Hopper", enabled: true,
  health: "ok", health_reason: "", archived_at: null, current_version_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbb01", target_space_ids: [marketing.id], container_title: null,
  trigger: "item.created", action_count: 1, has_draft: false, error_count: 0, last_run_at: null, last_run_status: null };

async function setup(page: Page, over: Record<string, unknown> = {}) {
  let limits = { rule_per_hour: 50, workspace_per_hour: 500, space_per_hour: 500 };
  const { calls } = await mockSpaces(page, { extra: {
    "GET /automation/workspace-rules": { body: { items: [rule] } },
    "GET /automation/limits": () => ({ body: limits }),
    "PUT /automation/limits": (body: Record<string, number>) => { limits = body as typeof limits; return { body: limits }; },
    [`POST /automation-rules/${rid}/targets`]: { body: rule },
    [`GET /spaces/${marketing.id}/workspace-automation`]: { body: { items: [{ id: rid, name: "Close everywhere", enabled: true, health: "ok" }] } },
    [`GET /spaces/${marketing.id}/automation-rules`]: { body: { items: [] } },
    "GET /workflows": { body: { items: [] } },
    "GET /item-types": { body: { items: [] } },
    ...over,
  } as never });
  return calls;
}

test("the workspace automation page lists rules and saves the execution limits", async ({ page }) => {
  const calls = await setup(page);
  await page.goto("/settings/automation");
  await expect(page.getByRole("heading", { name: "Workspace", exact: true })).toBeVisible();
  await page.getByText("Usage & limits").click();
  await expect(page.getByRole("cell", { name: "Close everywhere", exact: true })).toBeVisible();
  await expect(page.getByLabel("Per workspace rule")).toHaveValue("50");
  await page.getByLabel("Per workspace rule").fill("80");
  await page.getByRole("button", { name: "Save limits" }).click();
  await expect.poll(() => calls.find((c) => c.method === "PUT" && c.path === "/automation/limits")?.body).toEqual({ rule_per_hour: 80, workspace_per_hour: 500, space_per_hour: 500 });
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("targets: select all takes a snapshot of the visible Spaces and saves it", async ({ page }) => {
  const calls = await setup(page);
  await page.goto("/settings/automation");
  await page.getByRole("button", { name: "Targets of Close everywhere" }).click();
  const dialog = page.getByRole("dialog", { name: "Spaces for Close everywhere" });
  await dialog.getByRole("button", { name: "Select all" }).click();
  await dialog.getByRole("button", { name: "Save targets" }).click();
  await expect.poll(() => calls.find((c) => c.method === "POST" && c.path === `/automation-rules/${rid}/targets`)?.body).toMatchObject({ space_ids: expect.arrayContaining([marketing.id]) });
});

test("a Space shows the workspace rules that run in it, by name and state only", async ({ page }) => {
  await setup(page);
  await page.goto(`/spaces/${marketing.id}/automation`);
  await expect(page.getByRole("heading", { name: "Workspace rules that run here" })).toBeVisible();
  await expect(page.getByText("Close everywhere")).toBeVisible();
});
