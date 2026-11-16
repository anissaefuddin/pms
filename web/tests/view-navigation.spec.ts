import { expect, test } from "@playwright/test";
import { marketing, mockSpaces } from "./spaces-mock";

// Agent A: a compact view bar that stays usable with many views, plus a richer add-view chooser.
const path = `/spaces/${marketing.id}`;
const names = ["Table", "Board", "Gantt", "Calendar", "Sprint", "Backlog", "Overdue"];
const views = names.map((name, index) => ({
  id: `0000000${index}-0000-4000-8000-000000000000`, type: index % 2 ? "board" : "table", name,
  visibility: index === 5 ? "personal" : "public", position: index, is_default: index === 0,
}));

test("with seven views, six stay as tabs and the rest open from All views", async ({ page }) => {
  await mockSpaces(page, { extra: { [`GET ${path}/views`]: { body: { items: views } }, [`GET ${path}/items`]: { body: { items: [] } } } });
  await page.goto(path);
  const bar = page.getByRole("radiogroup", { name: "Views" });
  await expect(bar.getByRole("radio")).toHaveCount(6);
  await page.getByRole("button", { name: "All views" }).click();
  await expect(page.getByRole("menuitem")).toHaveCount(7);
  await expect(page.getByRole("menuitem", { name: /Backlog/ })).toContainText("Personal");
  await expect(page.getByRole("menuitem", { name: /Table/ })).toContainText("Default");
  await page.getByRole("menuitem", { name: /Overdue/ }).click();
  await expect(page).toHaveURL(new RegExp(`view=${views[6].id}`));
  // The selected view is kept visible in the bar, so the active view is always identifiable.
  await expect(bar.getByRole("radio", { name: "Overdue" })).toBeChecked();
});

test("Add view shows every view type with its availability before the name form", async ({ page }) => {
  await mockSpaces(page, { extra: { [`GET ${path}/views`]: { body: { items: views.slice(0, 2) } }, [`GET ${path}/items`]: { body: { items: [] } } } });
  await page.goto(path);
  await page.getByRole("button", { name: "Add view" }).click();
  const chooser = page.getByRole("dialog", { name: "Choose a view type" });
  const types = chooser.getByRole("group", { name: "Choose a view type" }).getByRole("button");
  await expect(types).toHaveCount(7);
  await expect(chooser.getByRole("button", { name: /Timelog/ })).toBeDisabled();
  await expect(chooser.getByRole("button", { name: /Calendar/ })).toBeEnabled();
  await chooser.getByRole("button", { name: /Calendar/ }).click();
  await expect(page.getByRole("dialog", { name: "Add view" }).getByRole("textbox", { name: "Name" })).toHaveValue("Calendar");
});
