import { expect, test, type Page } from "@playwright/test";
import type { CustomField, CustomFieldValue } from "../src/lib/custom-fields";
import type { Item, ItemUpdate } from "../src/lib/items";
import { me, problem } from "./mock-api";
import { ALL, grace, marketing, mockSpaces, ops } from "./spaces-mock";

const id = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const currencyId = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const peopleId = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const inactive = { id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", name: "Grace Murray", status: "deactivated" } as const;
const otherInactive = { ...grace, id: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", last_name: "Jones", status: "deactivated" };
const itemPath = `/items/${id}`;

async function mockItem(page: Page, field: CustomField, value: CustomFieldValue, locale = "en-US") {
  const item = {
    id, type: "task", title: "Design review", seq: 1, version: 1, assignees: [],
    locations: [{ space_id: marketing.id, parent_item_id: null }], my_capabilities: ALL,
    fields: { [field.id]: value } as NonNullable<Item["fields"]>,
    field_people: field.type === "people" ? { [inactive.id]: inactive } as NonNullable<Item["field_people"]> : undefined,
  };
  const response = () => ({ headers: { ETag: `"${item.version}"` }, body: item });
  const { calls } = await mockSpaces(page, { me: { ...me, locale }, extra: {
    [`GET ${itemPath}`]: response,
    [`GET ${itemPath}/children`]: { body: { items: [] } },
    [`GET /spaces/${marketing.id}/custom-fields`]: { body: { items: [field] } },
    "GET /users": { body: { items: [grace, otherInactive] } },
    [`PATCH ${itemPath}`]: (body: ItemUpdate, req) => {
      if (req.headers["if-match"] !== `"${item.version}"`) return problem(409, "Version conflict");
      for (const [id, next] of Object.entries(body.fields ?? {})) {
        if (next === null) delete item.fields[id];
        else item.fields[id] = next;
      }
      if (item.field_people) {
        const ids = item.fields[peopleId];
        item.field_people = Array.isArray(ids) ? Object.fromEntries([
          inactive, { id: grace.id, name: "Grace Hopper", status: "active" as const },
        ].filter(({ id }) => ids.includes(id)).map((person) => [person.id, person])) : {};
      }
      item.version++;
      return response();
    },
  } });
  await page.goto(`/tasks/${id}`);
  return { calls, item, patches: () => calls.filter((call) => call.method === "PATCH" && call.path === itemPath) };
}

for (const fixture of [
  { locale: "en-US", currency: "USD", initial: "$1,234.50", next: "$2,000.25", invalid: "2.000,25" },
  { locale: "id-ID", currency: "IDR", initial: "Rp\u00a01.234,50", next: "Rp 2.000,25", invalid: "2,000.25" },
]) {
  test(`edits Currency with ${fixture.locale} separators and updates the marker`, async ({ page }) => {
    const { calls, patches } = await mockItem(page, {
      id: currencyId, space_id: marketing.id, type: "currency", name: "Budget", position: 0, options: [],
      config: { currency: fixture.currency, markers: [{ operator: "gte", value: 2000, color: "#ff0000" }] },
    }, 1234.5, fixture.locale);
    const section = page.getByRole("region", { name: "Custom fields" });
    await expect(section.getByText(fixture.initial)).toBeVisible();
    await section.getByRole("button", { name: "Edit Budget" }).click();
    const editor = section.getByRole("form", { name: "Budget" });
    const input = editor.getByRole("textbox", { name: "Budget" });
    await expect(input).toHaveValue(fixture.initial);
    await expect(editor.getByRole("button", { name: "Save" })).toBeDisabled();
    await input.fill(fixture.invalid);
    await editor.getByRole("button", { name: "Save" }).click();
    await expect(editor.getByRole("alert")).toBeVisible();
    expect(patches()).toHaveLength(0);

    await input.fill(fixture.next);
    await editor.getByRole("button", { name: "Save" }).click();
    await expect(section.getByText(fixture.next)).toBeVisible();
    await expect(section.getByRole("button", { name: "Edit Budget" })).toBeFocused();
    await expect(section.getByRole("img", { name: /^At least / })).toHaveCSS("background-color", "rgb(255, 0, 0)");
    expect(patches()[0]).toMatchObject({ headers: { "if-match": '"1"' }, body: { fields: { [currencyId]: 2000.25 } } });
    expect(calls.some((call) => call.path === `/spaces/${ops.id}/custom-fields`)).toBe(false);

    await section.getByRole("button", { name: "Edit Budget" }).click();
    await editor.getByRole("button", { name: "Clear value" }).click();
    await editor.getByRole("button", { name: "Save" }).click();
    await expect(section.getByText("Not set")).toBeVisible();
    expect(patches()[1]).toMatchObject({ headers: { "if-match": '"2"' }, body: { fields: { [currencyId]: null } } });
  });
}

test("a stale Currency draft uses the item conflict controls", async ({ page }) => {
  const { item, patches } = await mockItem(page, {
    id: currencyId, space_id: marketing.id, type: "currency", name: "Budget", position: 0, options: [],
    config: { currency: "USD" },
  }, 10);
  const section = page.getByRole("region", { name: "Custom fields" });
  await section.getByRole("button", { name: "Edit Budget" }).click();
  const editor = section.getByRole("form", { name: "Budget" });
  await editor.getByRole("textbox", { name: "Budget" }).fill("15");
  item.fields[currencyId] = 20;
  item.version++;

  await editor.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Someone else changed this item. Review the latest version.")).toBeVisible();
  await expect(editor.getByRole("textbox", { name: "Budget" })).toHaveValue("15");
  await expect(editor.getByRole("button", { name: "Save" })).toBeDisabled();
  await page.getByRole("button", { name: "Reapply my changes" }).click();
  await expect(section.getByText("$15.00")).toBeVisible();
  expect(patches().map((call) => call.headers["if-match"])).toEqual(['"1"', '"2"']);
});

test("People keeps an inactive selection and only offers active users", async ({ page }) => {
  const { calls, patches } = await mockItem(page, {
    id: peopleId, space_id: marketing.id, type: "people", name: "Reviewers", position: 0, options: [],
  }, [inactive.id]);
  const section = page.getByRole("region", { name: "Custom fields" });
  await expect(section.getByText("Grace Murray (inactive)")).toBeVisible();
  await section.getByRole("button", { name: "Edit Reviewers" }).click();
  const editor = section.getByRole("form", { name: "Reviewers" });
  await expect(editor.getByRole("button", { name: "Remove Grace Murray (inactive)" })).toBeVisible();
  await editor.getByRole("searchbox").fill("Grace");
  await expect(editor.getByRole("button", { name: "Grace Hopper", exact: true })).toBeVisible();
  await expect(editor.getByRole("button", { name: "Grace Jones", exact: true })).toHaveCount(0);
  const search = calls.find((call) => call.path === "/users");
  expect(search?.query.get("status")).toBe("active");

  await editor.getByRole("button", { name: "Grace Hopper", exact: true }).click();
  await editor.getByRole("button", { name: "Save" }).click();
  await expect(section.getByText("Grace Murray (inactive), Grace Hopper")).toBeVisible();
  expect(patches()[0]).toMatchObject({ headers: { "if-match": '"1"' }, body: { fields: { [peopleId]: [inactive.id, grace.id] } } });

  await section.getByRole("button", { name: "Edit Reviewers" }).click();
  await editor.getByRole("button", { name: "Remove Grace Murray (inactive)" }).click();
  await editor.getByRole("button", { name: "Save" }).click();
  await expect(section.getByText("Grace Hopper")).toBeVisible();
  expect(patches()[1]?.body).toEqual({ fields: { [peopleId]: [grace.id] } });
});

test("a field with a view grant only is shown without an edit control, and a field without one is not shown (DEF-016)", async ({ page }) => {
  const field = { id: currencyId, space_id: marketing.id, name: "Budget line", type: "text", position: 0, options: [], config: {}, access: "view" } as unknown as CustomField;
  await mockItem(page, field, "Confidential");
  await expect(page.getByText("Budget line")).toBeVisible();
  await expect(page.getByRole("button", { name: "Edit Budget line" })).toHaveCount(0);
  await page.route(`**/api/v1/spaces/${marketing.id}/custom-fields`, (route) => route.fulfill({ json: { items: [{ ...field, access: "none" }] } }));
  await page.reload();
  await expect(page.getByRole("textbox", { name: "Name", exact: true })).toBeVisible();
  await expect(page.getByText("Budget line")).toHaveCount(0);
});
