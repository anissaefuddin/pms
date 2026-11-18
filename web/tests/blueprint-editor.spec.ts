import { expect, test } from "@playwright/test";
import { marketing, mockSpaces } from "./spaces-mock";
import { problem } from "./mock-api";

const bp = "e0000000-0000-4000-8000-0000000000aa";

const detail = (version: number, name = "Launch kit") => ({
  id: bp, space_id: marketing.id, name, item_type: "project", item_count: 2, created_at: "2026-11-13T10:00:00Z",
  scope: "space", version, updated_at: "2026-11-13T10:00:00Z",
  tree: {
    nodes: [
      { parent: -1, type: "project", title: "Launch", importance: "high", assignee_ids: [], is_milestone: false },
      { parent: 0, type: "task", title: "Plan", importance: "normal", assignee_ids: [], is_milestone: false, start_offset: 0, due_offset: 2 },
    ],
    dependencies: [],
  },
});

test("edit a blueprint, save with its version, and see the saved name", async ({ page }) => {
  let version = 1;
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /blueprints/${bp}`]: () => ({ body: detail(version) }),
    [`PATCH /blueprints/${bp}`]: (body) => {
      version += 1;
      return { body: { ...detail(version, body.name), version } };
    },
  } });
  await page.goto(`/blueprints/${bp}`);
  await expect(page.getByRole("heading", { name: "Edit blueprint" })).toBeVisible();
  await page.getByLabel("Name").fill("Launch kit v2");
  await page.getByLabel("Item title").nth(1).fill("Plan the launch");
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect.poll(() => calls.find((c) => c.method === "PATCH")?.body).toMatchObject({
    version: 1, name: "Launch kit v2", tree: { nodes: [{ title: "Launch" }, { title: "Plan the launch" }] },
  });
  await expect(page.getByText("Blueprint saved")).toBeVisible();
  await expect(page.getByText("v2")).toBeVisible();
});

test("a stale version is refused with a clear message and a reload option", async ({ page }) => {
  await mockSpaces(page, { extra: {
    [`GET /blueprints/${bp}`]: () => ({ body: detail(1) }),
    [`PATCH /blueprints/${bp}`]: () => problem(409, "the Item changed; reload and try again", "version_conflict"),
  } });
  await page.goto(`/blueprints/${bp}`);
  await page.getByLabel("Name").fill("Stale edit");
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(page.getByText("Someone changed this blueprint")).toBeVisible();
  await expect(page.getByRole("button", { name: "Reload" })).toBeVisible();
});

test("add a subitem, remove it again, and preview the dates without saving", async ({ page }) => {
  const { calls } = await mockSpaces(page, { extra: {
    [`GET /blueprints/${bp}`]: () => ({ body: detail(1) }),
    [`POST /blueprints/${bp}/preview`]: () => ({ body: { items: [
      { title: "Launch", type: "project", parent: -1, start_date: "2026-06-01", due_date: "2026-06-03" },
      { title: "Plan", type: "task", parent: 0, start_date: "2026-06-01", due_date: "2026-06-03" },
    ] } }),
  } });
  await page.goto(`/blueprints/${bp}`);
  await page.getByRole("button", { name: "Add subitem" }).first().click();
  await expect(page.getByLabel("Item title")).toHaveCount(3);
  await page.getByRole("button", { name: "Remove" }).last().click();
  await expect(page.getByLabel("Item title")).toHaveCount(2);

  // Preview needs a saved blueprint, so the editor asks for a save first when there are changes.
  await page.getByLabel("Name").fill("Changed");
  await expect(page.getByRole("button", { name: "Preview" })).toBeDisabled();
  await page.getByLabel("Name").fill("Launch kit");
  await page.getByLabel("Start date").fill("2026-06-01");
  await page.getByRole("button", { name: "Preview" }).click();
  await expect(page.getByRole("table", { name: "Preview" }).getByRole("row")).toHaveCount(3);
  expect(calls.some((c) => c.method === "PATCH")).toBe(false);
  expect(calls.find((c) => c.method === "POST" && c.path.endsWith("/preview"))?.body).toEqual({ start_date: "2026-06-01" });
});
