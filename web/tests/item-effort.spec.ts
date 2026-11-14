import { expect, test } from "@playwright/test";
import type { Item, ItemUpdate } from "../src/lib/items";
import { ALL, extra, grace, marketing, mockSpaces } from "./spaces-mock";
import { me, problem } from "./mock-api";

const id = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const itemPath = `/items/${id}`;
const ada = { id: me.id, name: "Ada Lovelace" };
const assignee = { id: grace.id, name: "Grace Hopper" };
const status = { id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", name: "New", color: "slate", group: "active" };

test("effort and per-assignee allocation save without rescheduling until requested", async ({ page }) => {
  let item = {
    id, type: "task", title: "Design review", seq: 1, version: 1,
    locations: [{ space_id: marketing.id, parent_item_id: null }],
    my_capabilities: ALL, status,
    start_date: "2026-11-12", due_date: "2026-11-12", duration_days: 1,
    effort_minutes: null as number | null,
    assignees: [ada, { ...assignee, allocation: { unit: "percent", value: 50 } }] as Item["assignees"],
  };
  const { calls } = await mockSpaces(page, { extra: extra({
    [`GET ${itemPath}`]: () => ({ headers: { ETag: `"${item.version}"` }, body: item }),
    [`GET ${itemPath}/children`]: { body: { items: [] } },
    "GET /settings/work-schedule": { body: { daily_hours: 6, working_days: [1, 2, 3, 4, 5], exceptions: [], version: 1 } },
    [`PATCH ${itemPath}`]: (body: ItemUpdate, req) => {
      if (req.headers["if-match"] !== `"${item.version}"`) return problem(409, "Version conflict");
      const changes = new Map(body.assignee_allocations?.map((change) => [change.assignee_id, change.allocation]));
      item = {
        ...item,
        version: item.version + 1,
        effort_minutes: body.effort_minutes === undefined ? item.effort_minutes : body.effort_minutes,
        assignees: item.assignees
          .filter((person) => !body.assignee_ids || body.assignee_ids.includes(person.id))
          .map((person) => changes.has(person.id) ? { ...person, allocation: changes.get(person.id) ?? undefined } : person),
        ...(body.recalculate_duration ? { duration_days: 2, due_date: "2026-11-13" } : {}),
      };
      return { headers: { ETag: `"${item.version}"` }, body: item };
    },
  }) });

  await page.goto(`/tasks/${id}`);
  await page.getByText("More fields", { exact: true }).click();
  const section = page.getByRole("region", { name: "Effort and allocation" });
  const effort = section.getByRole("spinbutton", { name: "Effort (hours)" });
  const adaAllocation = section.getByRole("spinbutton", { name: "Allocation for Ada Lovelace" });
  const graceAllocation = section.getByRole("spinbutton", { name: "Allocation for Grace Hopper" });
  const warning = section.getByText("Total allocation exceeds 100% of a workspace workday.");
  const save = section.getByRole("button", { name: "Save effort and allocation" });
  const recalculate = section.getByRole("button", { name: "Recalculate duration" });
  const patches = () => calls.filter((call) => call.method === "PATCH" && call.path === itemPath);

  await expect(recalculate).toBeDisabled();
  await expect(graceAllocation).toHaveValue("50");
  await effort.fill("7.25");
  await section.getByRole("combobox", { name: "Allocation unit for Ada Lovelace" }).selectOption("minutes_per_day");
  await adaAllocation.fill("3.5");
  await expect(warning).toBeVisible();
  await save.click();
  await expect.poll(() => patches().at(-1)?.body).toEqual({
    effort_minutes: 435, assignee_allocations: [{ assignee_id: ada.id, allocation: { unit: "minutes_per_day", value: 210 } }],
  });
  await expect(section.getByText("Duration: 1 workday")).toBeVisible();
  expect(item.due_date).toBe("2026-11-12");

  await effort.fill("10");
  await graceAllocation.fill("25");
  await expect(warning).toHaveCount(0);
  await recalculate.click();
  await expect(section.getByText("Duration: 2 workdays")).toBeVisible();
  expect(patches().at(-1)?.body).toEqual({
    effort_minutes: 600,
    assignee_allocations: [{ assignee_id: assignee.id, allocation: { unit: "percent", value: 25 } }],
    recalculate_duration: true,
  });
  expect(patches().map((call) => call.headers["if-match"])).toEqual(['"1"', '"2"']);

  await effort.clear();
  await adaAllocation.clear();
  await save.click();
  await expect.poll(() => patches().at(-1)?.body).toEqual({
    effort_minutes: null, assignee_allocations: [{ assignee_id: ada.id, allocation: null }],
  });

  await page.getByRole("button", { name: /^Assignees:/ }).click();
  await page.getByRole("button", { name: "Remove Grace Hopper" }).click();
  await expect(graceAllocation).toHaveCount(0);
  expect(patches().at(-1)?.body).toEqual({ assignee_ids: [ada.id] });
});
