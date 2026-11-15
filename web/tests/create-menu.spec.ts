import { AxeBuilder } from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const def = (key: string, base: string, name: string, icon: string, n: number, builtin = true) =>
  ({ id: `${n}${n}${n}${n}${n}${n}${n}${n}-1111-4111-8111-111111111111`, key, name, icon, base, is_builtin: builtin, active: true, allowed_children: [], defaults: {}, layout: {}, version: 1 });
const types = [def("task", "task", "Task", "check-square", 1), def("project", "project", "Project", "folder-kanban", 2), def("folder", "folder", "Folder", "folder", 3), def("bug", "task", "Bug", "bug", 4, false)];
const entry = (t: (typeof types)[number], enabled: boolean) => ({ key: t.key, name: t.name, icon: t.icon, base: t.base, enabled });
const bp = "e0000000-0000-4000-8000-000000000001";

test("admin toggles item types of the '+' menu; the dialog passes axe; the create dialog offers only enabled types", async ({ page }) => {
  let hidden = ["folder"];
  let version = 4;
  const body = () => ({ version, items: types.map((t) => entry(t, !hidden.includes(t.key))) });
  const { calls } = await mockSpaces(page, { extra: {
    "GET /item-types": { body: { items: types } },
    [`GET /spaces/${marketing.id}/create-menu`]: () => ({ body: body() }),
    [`PUT /spaces/${marketing.id}/create-menu`]: (b: { enabled: string[] }) => { hidden = types.map((t) => t.key).filter((k) => !b.enabled.includes(k)); version += 1; return { body: body() }; },
    [`GET /spaces/${marketing.id}/blueprints`]: { body: { items: [{ id: bp, space_id: marketing.id, name: "Release plan", item_type: "project", item_count: 3, created_at: "2026-11-13T10:00:00Z" }] } },
  } });
  await page.goto(`/spaces/${marketing.id}`);
  // The hidden type is not offered; the enabled ones are, and a blueprint shows its name, icon and type.
  await page.getByRole("main").getByRole("button", { name: "New item" }).click();
  const create = page.getByRole("dialog", { name: "Create item" });
  await expect(create.getByRole("radio", { name: "Project", exact: true })).toBeVisible();
  await expect(create.getByRole("radio", { name: "Task", exact: true })).toBeVisible();
  await expect(create.getByRole("radio", { name: "Folder", exact: true })).toHaveCount(0);
  await expect(create.getByRole("radio", { name: /Release plan/ })).toBeVisible();
  await expect(create.getByText("Project · 3 items")).toBeVisible();
  await create.getByRole("button", { name: "Cancel" }).click();

  await page.getByRole("button", { name: "Space actions" }).click();
  await page.getByRole("menuitem", { name: "“+” menu" }).click();
  const dialog = page.getByRole("dialog", { name: "“+” menu of this Space" });
  await expect(dialog.getByRole("checkbox", { name: "Folder" })).not.toBeChecked();
  await expect(dialog.getByRole("checkbox", { name: "Bug" })).toBeChecked();
  await expect(dialog).toHaveCSS("opacity", "1");
  expect((await new AxeBuilder({ page }).include('[role="dialog"]').analyze()).violations).toEqual([]);
  await dialog.getByRole("checkbox", { name: "Folder" }).check();
  await dialog.getByRole("checkbox", { name: "Bug" }).uncheck();
  await dialog.getByRole("button", { name: "Save menu" }).click();
  const isPut = (c: { method: string; path: string }) => c.method === "PUT" && c.path.endsWith("/create-menu");
  await expect.poll(() => calls.some(isPut)).toBe(true); // the click returns before the request is recorded on a slow runner
  const put = calls.find(isPut);
  expect(put?.body).toEqual({ enabled: ["task", "project", "folder"] });
  expect(put?.headers["if-match"]).toBe('"4"');
  await page.getByRole("main").getByRole("button", { name: "New item" }).click();
  await expect(page.getByRole("dialog", { name: "Create item" }).getByRole("radio", { name: "Folder", exact: true })).toBeVisible();
});
