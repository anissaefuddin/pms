import { AxeBuilder } from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { me } from "./mock-api";
import { marketing, mockSpaces, ops, personal } from "./spaces-mock";

// Settings > Automation: one sidebar entry, Workspace and Space tabs, Space selector limited to editable Spaces, search and sort.
const wsRule = { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaacc", space_id: "", scope: "workspace", container_id: null, name: "Close everywhere", owner_id: "u", owner_name: "Grace Hopper", enabled: true,
  health: "ok", health_reason: "", archived_at: null, current_version_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbb01", target_space_ids: [marketing.id], container_title: null,
  trigger: "item.created", action_count: 1, has_draft: false, error_count: 0, last_run_at: null, last_run_status: null };
const spaceRule = (id: string, name: string) => ({ ...wsRule, id, space_id: marketing.id, scope: "space", name, owner_name: "Ada Lovelace", target_space_ids: [] });

async function setup(page: Page, opts: { user?: Record<string, unknown>; opsCapabilities?: string[] } = {}) {
  const spaces = opts.opsCapabilities ? [personal, marketing, { ...ops, my_capabilities: opts.opsCapabilities }] : [personal, marketing, ops];
  return mockSpaces(page, { me: opts.user ? { ...me, ...opts.user } as typeof me : me, spaces, extra: {
    "GET /automation/workspace-rules": { body: { items: [wsRule] } },
    "GET /automation/limits": { body: { rule_per_hour: 50, workspace_per_hour: 500, space_per_hour: 500 } },
    "GET /automation/usage": { body: { workspace: { used: 3, limit: 500, state: "ok" }, space: { used: 1, limit: 500, state: "ok" }, window: "hour" } },
    [`GET /spaces/${marketing.id}/automation-rules`]: { body: { items: [spaceRule("cccccccc-cccc-4ccc-8ccc-cccccccc0001", "Alpha escalation"), spaceRule("cccccccc-cccc-4ccc-8ccc-cccccccc0002", "Zeta notify")] } },
    [`GET /spaces/${marketing.id}/workspace-automation`]: { body: { items: [] } },
    [`GET /spaces/${ops.id}/automation-rules`]: { body: { items: [] } },
    [`GET /spaces/${personal.id}/automation-rules`]: { body: { items: [] } },
    "GET /automation/dead-events": { body: { items: [] } },
  } as never });
}

test("one Automation entry in the settings menu; the workspace-only entry is gone", async ({ page }) => {
  await setup(page);
  await page.goto("/settings/automation");
  await expect(page.getByRole("link", { name: "Automation", exact: true })).toHaveCount(1);
  await expect(page.getByRole("link", { name: "Workspace automation" })).toHaveCount(0);
});

test("the Workspace and Space tabs switch, and the page passes axe", async ({ page }) => {
  await setup(page);
  await page.goto("/settings/automation");
  const scope = page.getByRole("radiogroup", { name: "Automation scope" });
  await expect(scope.getByRole("radio", { name: "Workspace", exact: true })).toHaveAttribute("aria-checked", "true");
  await expect(page.getByRole("button", { name: "New workspace rule" })).toBeVisible();
  await scope.getByRole("radio", { name: "Space", exact: true }).click();
  await page.getByRole("combobox", { name: "Space" }).selectOption({ label: "Marketing" });
  await expect(page.getByRole("button", { name: "New workspace rule" })).toHaveCount(0);
  await expect(page.getByRole("cell", { name: "Alpha escalation", exact: true })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
});

test("the Space selector lists only Spaces the person may edit", async ({ page }) => {
  await setup(page, { opsCapabilities: ["view"] });
  await page.goto("/settings/automation");
  await page.getByRole("radio", { name: "Space", exact: true }).click();
  await page.getByRole("combobox", { name: "Space" }).selectOption({ label: "Marketing" });
  const selector = page.getByRole("combobox", { name: "Space" });
  await expect(selector.locator("option", { hasText: "Marketing" })).toHaveCount(1);
  await expect(selector.locator("option", { hasText: "Operations" })).toHaveCount(0);
});

test("a regular user does not see workspace rules and starts on the Space tab", async ({ page }) => {
  await setup(page, { user: { user_type: "regular" } });
  await page.goto("/settings/automation");
  await page.getByRole("combobox", { name: "Space" }).selectOption({ label: "Marketing" });
  await expect(page.getByRole("cell", { name: "Alpha escalation", exact: true })).toBeVisible();
  await page.getByRole("radio", { name: "Workspace", exact: true }).click();
  await expect(page.getByText("Only a workspace administrator can manage workspace automation.")).toBeVisible();
});

test("search narrows the Space rules and the name column sorts them", async ({ page }) => {
  await setup(page);
  await page.goto("/settings/automation");
  await page.getByRole("radio", { name: "Space", exact: true }).click();
  await page.getByRole("combobox", { name: "Space" }).selectOption({ label: "Marketing" });
  const names = page.getByRole("columnheader", { name: "Rule", exact: true });
  await expect(names).toHaveAttribute("aria-sort", "ascending");
  await names.getByRole("button").click();
  await expect(names).toHaveAttribute("aria-sort", "descending");
  await page.getByRole("searchbox", { name: "Search rules" }).fill("zeta");
  await expect(page.getByRole("cell", { name: "Zeta notify", exact: true })).toBeVisible();
  await expect(page.getByRole("cell", { name: "Alpha escalation", exact: true })).toHaveCount(0);
});

test("the old Space deep link still opens the same Space rules", async ({ page }) => {
  await setup(page);
  await page.goto(`/spaces/${marketing.id}/automation`);
  await expect(page.getByRole("heading", { name: "Automation", exact: true })).toBeVisible();
  await expect(page.getByRole("cell", { name: "Alpha escalation", exact: true })).toBeVisible();
});

test("New rule opens the builder from both tabs, scoped to the selected tab", async ({ page }) => {
  await setup(page);
  await page.goto("/settings/automation");
  await page.getByRole("button", { name: "New workspace rule" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await page.getByRole("radio", { name: "Space", exact: true }).click();
  await page.getByRole("combobox", { name: "Space" }).selectOption({ label: "Marketing" });
  await page.getByRole("button", { name: "New rule", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
});
