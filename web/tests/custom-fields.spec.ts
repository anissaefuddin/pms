import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import type { components } from "../src/api/schema";
import { problem, type Handler } from "./mock-api";
import { marketing, mockSpaces } from "./spaces-mock";

const path = `/spaces/${marketing.id}/custom-fields`;
const firstId = "11111111-2222-4333-8444-555555555555";
const addedId = "66666666-7777-4888-8999-aaaaaaaaaaaa";

test("Space owner can create, edit, reorder and archive colored custom fields", async ({ page }) => {
  const first = {
    id: firstId, space_id: marketing.id, name: "Risk", type: "single_select", position: 0,
    options: [{ id: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", label: "Low", color: "#22c55e" }],
  };
  let items: (typeof first & { archived_at?: string })[] = [first];
  const extra: Record<string, Handler> = {
    [`GET ${path}`]: () => ({ body: { items } }),
    [`POST ${path}`]: (body) => {
      const field = { ...body, id: addedId, space_id: marketing.id, position: items.length };
      items.push(field);
      return { status: 201, body: field };
    },
    [`PATCH ${path}/${addedId}`]: (body) => {
      if (body.options?.length === 1) return problem(422, "An option is still in use.");
      const field = items.find((entry) => entry.id === addedId)!;
      Object.assign(field, body);
      if (body.position === 0) {
        first.position = 1;
        items = [field, first];
      }
      return { body: field };
    },
    [`DELETE ${path}/${addedId}`]: () => {
      items.find((entry) => entry.id === addedId)!.archived_at = "2026-11-13T00:00:00Z";
      return { status: 204 };
    },
  };
  const { calls } = await mockSpaces(page, { extra });
  await page.goto(`/spaces/${marketing.id}`);
  await page.getByRole("button", { name: "Space actions" }).click();
  await page.getByRole("menuitem", { name: "Custom fields" }).click();
  const dialog = page.getByRole("dialog", { name: "Custom fields" });

  await dialog.getByRole("button", { name: "Add custom field" }).click();
  await dialog.getByRole("textbox", { name: "Field name" }).fill("Impact");
  await dialog.getByRole("combobox", { name: "Field type" }).selectOption("multi_select");
  await dialog.getByRole("textbox", { name: "Option 1 label" }).fill("High");
  await dialog.getByLabel("Option 1 color").fill("#cc3333");
  await dialog.getByRole("button", { name: "Add option" }).click();
  await dialog.getByRole("textbox", { name: "Option 2 label" }).fill("Medium");
  await dialog.getByRole("button", { name: "Save", exact: true }).click();

  let row = dialog.getByRole("listitem").filter({ hasText: "Impact" });
  await expect(row.getByText("High")).toBeVisible();
  expect(calls.find((entry) => entry.method === "POST" && entry.path === path)?.body).toMatchObject({
    name: "Impact", type: "multi_select", options: [{ id: expect.any(String), label: "High", color: "#cc3333" }, { label: "Medium" }],
  });
  await row.getByRole("button", { name: "Edit", exact: true }).click();
  await dialog.getByRole("textbox", { name: "Field name" }).fill("Business impact");
  await dialog.getByRole("button", { name: "Remove option 2" }).click();
  await dialog.getByRole("button", { name: "Save", exact: true }).click();
  await expect(dialog.getByRole("alert")).toHaveText("An option is still in use.");
  await expect(dialog.getByRole("textbox", { name: "Field name" })).toHaveValue("Business impact");
  await dialog.getByRole("button", { name: "Add option" }).click();
  await dialog.getByRole("textbox", { name: "Option 2 label" }).fill("Medium");
  await dialog.getByRole("button", { name: "Save", exact: true }).click();

  row = dialog.getByRole("listitem").filter({ hasText: "Business impact" });
  await row.getByRole("button", { name: "Move Business impact up" }).click();
  await expect(dialog.getByRole("listitem").first()).toContainText("Business impact");
  await row.getByRole("button", { name: "Archive", exact: true }).click();
  await expect(row.getByText("Archived", { exact: true })).toBeVisible();
  await expect(row.getByRole("button", { name: "Edit", exact: true })).toHaveCount(0);
});

test("Space editor does not get custom field management controls", async ({ page }) => {
  const { calls } = await mockSpaces(page, { spaces: [{ ...marketing, my_capabilities: ["view", "edit", "create"] }] });
  await page.goto(`/spaces/${marketing.id}`);
  await page.getByRole("button", { name: "Space actions" }).click();
  await expect(page.getByRole("menuitem", { name: "Custom fields" })).toHaveCount(0);
  expect(calls.some((call) => call.path === path)).toBe(false);
});

test("Space owner can save and replace a currency field's numeric settings", async ({ page }) => {
  type Field = components["schemas"]["CustomField"];
  let items: Field[] = [];
  const { calls } = await mockSpaces(page, {
    extra: {
      [`GET ${path}`]: () => ({ body: { items } }),
      [`POST ${path}`]: (body) => {
        items = [{ ...body, id: addedId, space_id: marketing.id, position: 0, options: [] }];
        return { status: 201, body: items[0] };
      },
      [`PATCH ${path}/${addedId}`]: (body) => {
        items = [{ ...items[0], ...body }];
        return { body: items[0] };
      },
    },
  });
  await page.goto(`/spaces/${marketing.id}`);
  await page.getByRole("button", { name: "Space actions" }).click();
  await page.getByRole("menuitem", { name: "Custom fields" }).click();
  const dialog = page.getByRole("dialog", { name: "Custom fields" });
  const save = dialog.getByRole("button", { name: "Save", exact: true });
  const addMarker = dialog.getByRole("button", { name: "Add marker" });
  await dialog.getByRole("button", { name: "Add custom field" }).click();
  await dialog.getByRole("textbox", { name: "Field name" }).fill("Budget");
  const fieldType = dialog.getByRole("combobox", { name: "Field type" });
  await fieldType.selectOption("percent");
  await dialog.getByRole("checkbox", { name: "Allow values over 100%" }).check();
  await fieldType.selectOption("currency");
  await expect(dialog.getByRole("textbox", { name: "Option 1 label" })).toHaveCount(0);
  await expect(dialog.getByRole("textbox", { name: "Currency code" })).toHaveValue("USD");
  await dialog.getByRole("textbox", { name: "Currency code" }).fill("idr");
  await expect(dialog.getByRole("spinbutton", { name: "Decimal places" })).toHaveValue("2");
  await dialog.getByRole("spinbutton", { name: "Decimal places" }).fill("0");
  await dialog.getByRole("combobox", { name: "Default aggregation" }).selectOption("sum");
  await addMarker.click();
  await dialog.getByRole("spinbutton", { name: "Marker 1 value" }).fill("1000000");
  await addMarker.click();
  await dialog.getByRole("spinbutton", { name: "Marker 2 value" }).fill("2000000");
  await dialog.getByLabel("Marker 2 color").fill("#cc3333");
  await dialog.getByRole("button", { name: "Move marker 2 up" }).click();
  await save.click();

  const row = dialog.getByRole("listitem").filter({ hasText: "Budget" });
  await expect(row.getByText("Currency", { exact: true })).toBeVisible();
  expect(calls.find((entry) => entry.method === "POST" && entry.path === path)?.body).toEqual({
    name: "Budget",
    type: "currency",
    config: {
      currency: "IDR",
      decimals: 0,
      aggregation: "sum",
      markers: [
        { operator: "gte", value: 2000000, color: "#cc3333" },
        { operator: "gte", value: 1000000, color: "#2563eb" },
      ],
    },
  });
  await row.getByRole("button", { name: "Edit", exact: true }).click();
  await expect(fieldType).toHaveCount(0);
  await expect(dialog.getByRole("spinbutton", { name: "Marker 1 value" })).toHaveValue("2000000");
  await dialog.getByRole("textbox", { name: "Field name" }).fill("Quarterly budget");
  await save.click();
  const renamed = dialog.getByRole("listitem").filter({ hasText: "Quarterly budget" });
  await renamed.getByRole("button", { name: "Edit", exact: true }).click();
  await expect(dialog.getByRole("spinbutton", { name: "Marker 1 value" })).toHaveValue("2000000");
  await dialog.getByRole("button", { name: "Remove marker 2" }).click();
  await dialog.getByRole("button", { name: "Remove marker 1" }).click();
  await dialog.getByRole("combobox", { name: "Default aggregation" }).selectOption("");
  await save.click();
  await expect(renamed.getByRole("button", { name: "Edit", exact: true })).toBeEnabled();
  expect(calls.filter((entry) => entry.method === "PATCH" && entry.path === `${path}/${addedId}`)
    .map((entry) => entry.body)).toEqual([
    { name: "Quarterly budget" },
    { name: "Quarterly budget", config: { currency: "IDR", decimals: 0, markers: [] } },
  ]);
});

for (const colorScheme of ["light", "dark"] as const) {
  test.describe(colorScheme, () => {
    test.use({ colorScheme });

    test("custom field form has no serious accessibility violations", async ({ page }) => {
      await mockSpaces(page, { extra: { [`GET ${path}`]: { body: { items: [] } } } });
      await page.goto(`/spaces/${marketing.id}`);
      await page.getByRole("button", { name: "Space actions" }).click();
      await page.getByRole("menuitem", { name: "Custom fields" }).click();
      const dialog = page.getByRole("dialog", { name: "Custom fields" });
      await dialog.getByRole("button", { name: "Add custom field" }).click();
      await expect(dialog.getByRole("textbox", { name: "Field name" })).toBeFocused();
      await page.evaluate(() => Promise.all(document.getAnimations().map((animation) => animation.finished.catch(() => undefined))));
      const { violations } = await new AxeBuilder({ page }).include('[data-slot="dialog-content"]').analyze();
      expect(violations.filter(({ impact }) => impact === "serious" || impact === "critical")).toEqual([]);
    });
  });
}
