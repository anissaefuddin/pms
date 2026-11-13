import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { me, problem } from "./mock-api";
import { mockSpaces } from "./spaces-mock";

const id = (n: number) => `88888888-8888-4888-8888-${String(n).padStart(12, "0")}`;
const status = (n: number, name: string, group: string, color: string) =>
  ({ id: id(n), name, color, group, auto_assign: false, lock_transitions: false, allowed_next: [] as string[] });
const seed = () => [
  { id: id(100), name: "Default flow", is_default: true, statuses: [status(1, "New", "active", "slate"), status(2, "In Progress", "active", "blue"), status(3, "Completed", "completed", "green")] },
];

/** Stateful fake of the workflow endpoints. `inUse` makes DELETE of that status 409 until move_to is sent. */
function workflowApi(state: { items: ReturnType<typeof seed>; inUse?: string }) {
  const find = (sid: string) => state.items.find((wf) => wf.statuses.some((s) => s.id === sid))!;
  return {
    "GET /workflows": () => ({ body: { items: state.items } }),
    "POST /workflows": (body: any) => {
      const wf = { id: id(200), name: body.name, is_default: false, statuses: [status(11, "New", "active", "slate"), status(12, "In Progress", "active", "blue"), status(13, "Completed", "completed", "green")] };
      state.items.push(wf);
      return { status: 201, body: wf };
    },
    [`POST /workflows/${id(100)}/statuses`]: (body: any) => {
      state.items[0].statuses.push({ ...status(4, body.name, body.group, body.color), auto_assign: false });
      return { status: 201, body: state.items[0] };
    },
    [`PUT /workflows/${id(100)}/status-order`]: (body: any) => {
      state.items[0].statuses = body.status_ids.map((sid: string) => state.items[0].statuses.find((s) => s.id === sid)!);
      return { body: state.items[0] };
    },
    ...Object.fromEntries([1, 2, 3, 4].flatMap((n) => [
      [`PATCH /statuses/${id(n)}`, (body: any) => { Object.assign(find(id(n)).statuses.find((s) => s.id === id(n))!, body); return { body: find(id(n)) }; }],
      [`DELETE /statuses/${id(n)}`, (_: any, req: any) => {
        if (state.inUse === id(n) && !req.query.get("move_to")) return problem(409, "items still use this status", "status_in_use");
        const wf = find(id(n));
        wf.statuses = wf.statuses.filter((s) => s.id !== id(n));
        return { body: wf };
      }],
    ])),
  };
}

test("admin creates a workflow, adds and reorders statuses, and locks transitions", async ({ page }) => {
  const state = { items: seed() };
  const { calls } = await mockSpaces(page, { extra: workflowApi(state) });
  await page.goto("/settings/workflows");

  await page.getByLabel("Workflow name").fill("Support");
  await page.getByRole("button", { name: "Create workflow" }).click();
  await expect(page.getByRole("combobox", { name: "Workflow", exact: true })).toHaveValue(id(200));
  await expect(page.getByLabel("Status name: In Progress")).toBeVisible();

  await page.getByRole("combobox", { name: "Workflow", exact: true }).selectOption(id(100));
  await page.getByLabel("Status name", { exact: true }).fill("Review");
  await page.getByRole("button", { name: "Add status" }).click();
  await expect(page.getByLabel("Status name: Review")).toBeVisible();
  expect(calls.find((c) => c.method === "POST" && c.path.endsWith("/statuses"))?.body).toEqual({ name: "Review", group: "active", color: "slate" });

  await page.getByRole("button", { name: "Move up: Review" }).click();
  await expect(page.getByRole("button", { name: "Move down: Review" })).toBeEnabled();
  expect(calls.find((c) => c.method === "PUT")?.body).toEqual({ status_ids: [id(1), id(2), id(4), id(3)] });

  await expect(page.getByText("Allowed next statuses")).toHaveCount(0);
  await page.getByLabel("Lock transitions: New").click(); // controlled: flips once the server answers
  await expect(page.getByText("Allowed next statuses: New")).toBeVisible();
  await page.getByLabel("In Progress", { exact: true }).click();
  await expect(page.getByLabel("In Progress", { exact: true })).toBeChecked();
  const patches = calls.filter((c) => c.method === "PATCH").map((c) => c.body);
  expect(patches).toEqual([{ lock_transitions: true }, { allowed_next: [id(2)] }]);
});

test("deleting a status in use asks where to move its items, then retries with move_to", async ({ page }) => {
  const state = { items: seed(), inUse: id(2) };
  const { calls } = await mockSpaces(page, { extra: workflowApi(state) });
  await page.goto("/settings/workflows");

  await page.getByRole("button", { name: "Delete status: In Progress" }).click();
  await page.getByRole("button", { name: "Delete", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Move items to another status" });
  await expect(dialog).toBeVisible();
  await dialog.getByLabel("Move items to").selectOption({ label: "Completed" });
  await dialog.getByRole("button", { name: "Move and delete" }).click();
  await expect(page.getByLabel("Status name: In Progress")).toHaveCount(0);
  const deletes = calls.filter((c) => c.method === "DELETE");
  expect(deletes.map((c) => c.query.get("move_to"))).toEqual([null, id(3)]);
});

test("server rule errors are shown inline", async ({ page }) => {
  const state = { items: seed() };
  await mockSpaces(page, { extra: { ...workflowApi(state), [`PATCH /statuses/${id(3)}`]: problem(422, "the workflow needs at least one Completed status", "validation_failed") } });
  await page.goto("/settings/workflows");
  await page.getByLabel("Group: Completed").selectOption("active");
  await expect(page.getByRole("alert")).toHaveText("the workflow needs at least one Completed status");
});

test("a regular user sees the workflows without edit controls", async ({ page }) => {
  await mockSpaces(page, { me: { ...me, user_type: "regular" }, extra: workflowApi({ items: seed() }) });
  await page.goto("/settings/workflows");
  await expect(page.getByText("Only administrators can change workflows.")).toBeVisible();
  await expect(page.getByLabel("Status name: New")).toBeDisabled();
  for (const name of ["Create workflow", "Add status", "Delete workflow", "Rename workflow"]) await expect(page.getByRole("button", { name })).toHaveCount(0);
});

for (const scheme of ["light", "dark"] as const) {
  test(`workflows have no axe violations (${scheme})`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: scheme });
    const items = seed();
    items[0].statuses[1].lock_transitions = true;
    await mockSpaces(page, { extra: workflowApi({ items }) });
    await page.goto("/settings/workflows");
    await expect(page.getByLabel("Status name: New")).toBeVisible();
    const result = await new AxeBuilder({ page }).analyze();
    expect(result.violations.map((v) => `${v.id}: ${v.nodes[0]?.html}`)).toEqual([]);
  });
}
