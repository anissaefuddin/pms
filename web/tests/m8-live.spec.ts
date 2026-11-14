import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Browser, type Page } from "@playwright/test";

const e = process.env;
test.skip(!e.PMS_FIXTURE_M8_ADMIN, "Requires the local Go/PostgreSQL M8 fixture");
test.describe.configure({ mode: "serial" });

async function as(browser: Browser, cookie: string) {
  const baseURL = test.info().project.use.baseURL!;
  const context = await browser.newContext({ baseURL, colorScheme: "light" });
  await context.addCookies([{ name: "pms_session", value: cookie.split("=")[1], url: baseURL }]);
  return context.newPage();
}
async function axe(page: Page) {
  await expect(page.locator("[data-starting-style], [data-ending-style]")).toHaveCount(0);
  const result = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
  expect(result.violations.map((v) => `${v.id}: ${v.nodes[0]?.html}`)).toEqual([]);
}

test("E2E-PAR-01/02: an admin defines a Bug type in the UI, and a Bug made from the create dialog carries its defaults", async ({ browser }) => {
  const admin = await as(browser, e.PMS_FIXTURE_M8_ADMIN!);
  await admin.goto("/settings/item-types");
  await admin.getByRole("button", { name: "Create a custom item type" }).first().click();
  const dialog = admin.getByRole("dialog", { name: "Create a custom item type" });
  await dialog.getByLabel("Name", { exact: true }).fill("Bug");
  await dialog.getByLabel("Key (lowercase letters, digits, _)").fill("bug");
  await dialog.locator('select[name="importance"]').selectOption("high");
  await axe(admin);
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(admin.getByText("Bug", { exact: true })).toBeVisible();
  const types = (await (await admin.request.get("/api/v1/item-types")).json()).items as { key: string; defaults: Record<string, unknown> }[];
  expect(types.find((t) => t.key === "bug")?.defaults).toEqual({ importance: "high" });

  await admin.goto(`/spaces/${e.PMS_FIXTURE_M8_WORK}`);
  await admin.locator("header").first().getByRole("button", { name: "New item" }).click();
  const create = admin.getByRole("dialog");
  await create.getByLabel("Name").fill("Crash on save");
  await create.getByRole("radio", { name: "Task" }).check();
  await create.locator("select").filter({ has: admin.locator('option[value="bug"]') }).selectOption("bug");
  await create.getByRole("button", { name: "Create item" }).click();
  await expect(admin.getByLabel("Importance", { exact: true })).toHaveValue("high");
  const list = (await (await admin.request.get(`/api/v1/spaces/${e.PMS_FIXTURE_M8_WORK}/items`)).json()).items as { id: string; title: string; item_type: { key: string } }[];
  expect(list.find((i) => i.title === "Crash on save")?.item_type.key).toBe("bug");
});

test("E2E-PAR-03: a status guard blocks the move the same way in the UI and over REST, and passes when the item is ready", async ({ browser }) => {
  const admin = await as(browser, e.PMS_FIXTURE_M8_ADMIN!);
  const editor = await as(browser, e.PMS_FIXTURE_M8_EDITOR!);
  await admin.goto("/settings/workflows");
  await admin.getByRole("combobox", { name: "Workflow", exact: true }).selectOption(e.PMS_FIXTURE_M8_WORKFLOW!);
  await admin.getByRole("button", { name: "Completed", exact: true }).click();
  await admin.getByRole("checkbox", { name: /Needs a due date\s*:\s*Completed/ }).click();
  await expect(admin.getByRole("checkbox", { name: /Needs a due date\s*:\s*Completed/ })).toBeChecked();
  await admin.getByRole("checkbox", { name: /Needs an assignee\s*:\s*Completed/ }).click();
  await expect(admin.getByRole("checkbox", { name: /Needs an assignee\s*:\s*Completed/ })).toBeChecked();
  await axe(admin);
  const completed = ((await (await admin.request.get("/api/v1/workflows")).json()).items as { id: string; statuses: { id: string; name: string; required_fields: string[] }[] }[])
    .find((w) => w.id === e.PMS_FIXTURE_M8_WORKFLOW)!.statuses.find((s) => s.name === "Completed")!;
  expect(completed.required_fields.sort()).toEqual(["assignees", "due_date"]);

  // UI: the refusal lists every missing piece and the item keeps its status.
  await editor.goto(`/tasks/${e.PMS_FIXTURE_M8_TASK}`);
  const status = editor.getByLabel("Status", { exact: true });
  const before = await status.inputValue();
  await status.selectOption({ label: "Completed" });
  const alert = editor.getByRole("alert").filter({ hasText: "this status needs a due date" });
  await expect(alert).toContainText("this status needs an assignee");
  await expect(status).toHaveValue(before);

  // REST: same refusal, structured, nothing written.
  const version = async () => (await (await editor.request.get(`/api/v1/items/${e.PMS_FIXTURE_M8_TASK}`)).json()).version as number;
  const v1 = await version();
  const refused = await editor.request.patch(`/api/v1/items/${e.PMS_FIXTURE_M8_TASK}`, { headers: { "If-Match": `"${v1}"`, Origin: test.info().project.use.baseURL! }, data: { status_id: completed.id } });
  expect(refused.status()).toBe(422);
  expect((await refused.json()).code).toBe("transition_blocked");
  expect(await version()).toBe(v1);

  // Ready: one change that supplies the date, the assignee and the status together is accepted.
  const ok = await editor.request.patch(`/api/v1/items/${e.PMS_FIXTURE_M8_TASK}`, { headers: { "If-Match": `"${v1}"`, Origin: test.info().project.use.baseURL! },
    data: { due_date: "2026-12-01", assignee_ids: [e.PMS_FIXTURE_M8_EDITOR_ID], status_id: completed.id } });
  expect(ok.status()).toBe(200);
  expect((await ok.json()).status.name).toBe("Completed");
});
