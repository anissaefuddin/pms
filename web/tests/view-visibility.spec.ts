import { expect, test } from "@playwright/test";
import { me } from "./mock-api";
import { marketing, mockSpaces } from "./spaces-mock";

// WA-D07: hiding a view is personal and reversible; the default view stays in the tabs; the choice survives a reload.
const path = `/spaces/${marketing.id}`;
const names = ["Table", "Board", "Gantt", "Calendar", "Sprint", "Backlog", "Overdue"];
const views = names.map((name, index) => ({
  id: `0000000${index}-0000-4000-8000-000000000000`, type: index % 2 ? "board" : "table", name,
  visibility: "public", position: index, is_default: index === 0,
}));
const board = views[1].id;

function stored() {
  const prefs: Record<string, { hidden: string[]; order: string[] }> = {};
  return {
    prefs,
    handlers: {
      [`PUT /me/view-preferences/${marketing.id}`]: (body: { hidden: string[]; order: string[] }) => {
        prefs[marketing.id] = body;
        return { status: 204, body: undefined };
      },
      "GET /me": () => ({ body: { ...me, view_prefs: prefs } }),
    },
  };
}

test("hide a view from the tabs, find it under Hidden views, restore it; the default cannot be hidden", async ({ page }) => {
  const store = stored();
  const { calls } = await mockSpaces(page, { extra: { [`GET ${path}/views`]: { body: { items: views } }, [`GET ${path}/items`]: { body: { items: [] } }, ...store.handlers } });
  await page.goto(path);
  const bar = page.getByRole("radiogroup", { name: "Views" });
  await expect(bar.getByRole("radio", { name: "Board", exact: true })).toBeVisible();
  await bar.getByRole("radio", { name: "Board", exact: true }).click();
  await page.getByRole("button", { name: "Board options", exact: true }).click();
  await page.getByRole("menuitem", { name: "Hide from tabs" }).click();
  // The active view stays as a tab, marked Hidden, so the user can see where they are; the saved preference is unchanged for it.
  await expect(page.getByText("Hidden", { exact: true })).toBeVisible();
  await expect.poll(() => calls.find((c) => c.method === "PUT")?.body).toEqual({ hidden: [board], order: expect.any(Array) });
  await bar.getByRole("radio", { name: "Table", exact: true }).click();
  await expect(bar.getByRole("radio", { name: "Board", exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "All views" }).click();
  await expect(page.getByText("Hidden views")).toBeVisible();
  await page.getByRole("menuitem", { name: /Board/ }).getByRole("button", { name: "Restore" }).or(page.getByText("Restore", { exact: true })).first().click();
  await expect.poll(() => calls.filter((c) => c.method === "PUT").at(-1)?.body).toEqual({ hidden: [], order: expect.any(Array) });
  await expect(bar.getByRole("radio", { name: "Board", exact: true })).toBeVisible();
});

test("the default view cannot be hidden, and hidden views survive a reload", async ({ page }) => {
  const store = stored();
  await mockSpaces(page, { extra: { [`GET ${path}/views`]: { body: { items: views } }, [`GET ${path}/items`]: { body: { items: [] } }, ...store.handlers } });
  await page.goto(path);
  await page.getByRole("button", { name: "Table options", exact: true }).click();
  await expect(page.getByRole("menuitem", { name: "Default view stays in tabs" })).toBeDisabled();
  await page.keyboard.press("Escape");
  await page.getByRole("radiogroup", { name: "Views" }).getByRole("radio", { name: "Board", exact: true }).click();
  await page.getByRole("button", { name: "Board options", exact: true }).click();
  await page.getByRole("menuitem", { name: "Hide from tabs" }).click();
  await expect.poll(() => store.prefs[marketing.id]?.hidden).toEqual([board]);
  await page.reload();
  await expect(page.getByRole("radiogroup", { name: "Views" }).getByRole("radio", { name: "Board", exact: true })).toHaveCount(0);
});

test("dragging a tab onto another saves a personal order; the shared view position is untouched", async ({ page }) => {
  const store = stored();
  const { calls } = await mockSpaces(page, { extra: { [`GET ${path}/views`]: { body: { items: views } }, [`GET ${path}/items`]: { body: { items: [] } }, ...store.handlers } });
  await page.goto(path);
  const bar = page.getByRole("radiogroup", { name: "Views" });
  await bar.getByRole("radio", { name: "Gantt", exact: true }).dragTo(bar.getByRole("radio", { name: "Table", exact: true }));
  await expect.poll(() => store.prefs[marketing.id]?.order?.[0]).toBe(views[2].id);
  expect(calls.filter((c) => c.method === "PATCH" && c.path.startsWith("/views/"))).toHaveLength(0);
});
