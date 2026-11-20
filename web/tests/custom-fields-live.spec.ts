import { expect, test } from "@playwright/test";
import type { components } from "../src/api/schema";

const { PMS_FIXTURE_CUSTOM_FIELDS_SPACE: space, PMS_FIXTURE_CUSTOM_FIELDS_ITEM: item,
  PMS_FIXTURE_CUSTOM_FIELDS_COOKIE: cookie, PMS_FIXTURE_ORIGIN: origin } = process.env;
const { PMS_FIXTURE_TYPED_FIELDS_ITEM: typedItem, PMS_FIXTURE_TYPED_FIELDS_AMOUNT: amount,
  PMS_FIXTURE_TYPED_FIELDS_PEOPLE: people, PMS_FIXTURE_TYPED_FIELDS_INACTIVE: inactive,
  PMS_FIXTURE_TYPED_FIELDS_ACTIVE: active } = process.env;

test.skip(!space || !item || !cookie || !origin, "Requires the local Go/PostgreSQL custom fields fixture");

test("Space owner manages custom fields while the server preserves an option in use", async ({ page, baseURL }) => {
  await page.context().addCookies([{ name: "pms_session", value: cookie!.split("=")[1], url: baseURL! }]);
  await page.goto(`/spaces/${space}`);
  async function openFields() {
    await page.getByRole("button", { name: "Space actions" }).click();
    await page.getByRole("menuitem", { name: "Custom fields" }).click();
    return page.getByRole("dialog", { name: "Custom fields" });
  }

  const dialog = await openFields();
  async function addField(name: string, labels: string[]) {
    await dialog.getByRole("button", { name: "Add custom field" }).click();
    await dialog.getByRole("textbox", { name: "Field name" }).fill(name);
    for (const [index, label] of labels.entries()) {
      if (index > 0) await dialog.getByRole("button", { name: "Add option" }).click();
      await dialog.getByRole("textbox", { name: `Option ${index + 1} label` }).fill(label);
    }
    await dialog.getByLabel("Option 1 color").fill("#cc3333");
    await dialog.getByRole("button", { name: "Save", exact: true }).click();
    await expect(dialog.getByRole("listitem").filter({ hasText: name })).toBeVisible();
  }
  await addField("Priority", ["High", "Low"]);
  await addField("Impact", ["High", "Low"]);
  const impact = dialog.getByRole("listitem").filter({ hasText: "Impact" });
  await impact.getByRole("button", { name: "Move Impact up" }).click();
  await expect(dialog.getByRole("listitem").first()).toContainText("Impact");

  const list = await page.request.get(`/api/v1/spaces/${space}/custom-fields`);
  await expect(list).toBeOK();
  const { items }: { items: components["schemas"]["CustomField"][] } = await list.json();
  const [field] = items;
  expect(field.name).toBe("Impact");
  const [option] = field.options;
  expect(option.color).toBe("#cc3333");

  const itemURL = `/api/v1/items/${item}`;
  const detail = await page.request.get(itemURL);
  await expect(detail).toBeOK();
  const etag = detail.headers().etag;
  expect(etag).toBeTruthy();
  const saved = await page.request.patch(itemURL, {
    headers: { Origin: origin!, "If-Match": etag },
    data: { fields: { [field.id]: option.id } },
  });
  await expect(saved).toBeOK();

  await impact.getByRole("button", { name: "Edit", exact: true }).click();
  await dialog.getByRole("button", { name: "Remove option 1" }).click();
  await dialog.getByRole("button", { name: "Save", exact: true }).click();
  await expect(dialog.getByRole("alert")).toHaveText("an option still in use cannot be removed");
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();

  await impact.getByRole("button", { name: "Archive", exact: true }).click();
  await dialog.getByRole("alertdialog").getByRole("button", { name: "Archive field", exact: true }).click();
  await expect(impact.getByText("Archived", { exact: true })).toBeVisible();
  await page.reload();
  const reloaded = await openFields();
  await expect(reloaded.getByRole("listitem").filter({ hasText: "Impact" }).getByText("Archived", { exact: true })).toBeVisible();
  const persisted = await page.request.get(itemURL);
  await expect(persisted).toBeOK();
  expect((await persisted.json()).fields).toEqual({ [field.id]: option.id });
});

test("Currency and People persist through the Item editor and real API", async ({ page, baseURL }) => {
  test.skip(!typedItem || !amount || !people || !inactive || !active, "Requires the typed fields fixture");
  await page.context().addCookies([{ name: "pms_session", value: cookie!.split("=")[1], url: baseURL! }]);
  await page.goto(`/tasks/${typedItem}`);
  const section = page.getByRole("region", { name: "Custom fields" });
  await expect(section.getByText("$1,234.50")).toBeVisible();
  await section.getByRole("button", { name: "Edit Budget" }).click();
  const budget = section.getByRole("form", { name: "Budget" });
  await budget.getByRole("textbox", { name: "Budget" }).fill("$9,876.54");
  await budget.getByRole("button", { name: "Save" }).click();
  await expect(section.getByText("$9,876.54")).toBeVisible();

  await expect(section.getByText("Review Former (inactive)")).toBeVisible();
  await section.getByRole("button", { name: "Edit Reviewers" }).click();
  const reviewers = section.getByRole("form", { name: "Reviewers" });
  await reviewers.getByRole("searchbox").fill("Review");
  const candidate = reviewers.getByRole("button", { name: "Review Current", exact: true });
  await expect(candidate).toBeVisible();
  await expect(reviewers.getByRole("button", { name: "Review Former", exact: true })).toHaveCount(0);
  await candidate.click();
  await reviewers.getByRole("button", { name: "Save" }).click();
  await expect(section.getByText("Review Former (inactive), Review Current")).toBeVisible();
  await page.reload();
  await expect(section.getByText("$9,876.54")).toBeVisible();
  await expect(section.getByText("Review Former (inactive), Review Current")).toBeVisible();
  const response = await page.request.get(`/api/v1/items/${typedItem}`);
  await expect(response).toBeOK();
  expect((await response.json()).fields).toEqual({ [amount!]: 9876.54, [people!]: [inactive, active] });
});
