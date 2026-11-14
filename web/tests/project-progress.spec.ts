import { expect, test } from "@playwright/test";
import type { Item, ItemUpdate } from "../src/lib/items";
import { problem } from "./mock-api";
import { ALL, marketing, mockSpaces } from "./spaces-mock";

const projectId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const folderId = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const taskId = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const projectPath = `/items/${projectId}`;
const spacePath = `/spaces/${marketing.id}`;
const taskPath = `/items/${taskId}`;
const timestamp = "2026-11-13T00:00:00Z";
const common = { seq: 1, version: 1, locations: [{ space_id: marketing.id }], assignees: [],
  my_capabilities: ALL as Item["my_capabilities"], created_at: timestamp, updated_at: timestamp };
const active = { id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", name: "New", color: "slate", group: "active" } as const;
const completed = { id: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", name: "Completed", color: "green", group: "completed" } as const;
const workflows = { body: { items: [{ is_default: true, statuses: [active, completed] }] } };

test("Project panel and Table refresh when a task changes inside an unloaded folder", async ({ page }) => {
  let project: Item = {
    ...common, id: projectId, title: "Campaign", type: "project",
    progress_method: "task_count", progress_percent: 25, progress_risk: "green",
  };
  await page.addInitScript(() => {
    class MockEventSource extends EventTarget {
      private forward = (event: Event) => {
        this.dispatchEvent(new MessageEvent("item.updated", { data: JSON.stringify((event as CustomEvent).detail) }));
      };
      constructor() {
        super();
        window.addEventListener("pms-test:item-updated", this.forward);
      }
      close() { window.removeEventListener("pms-test:item-updated", this.forward); }
    }
    window.EventSource = MockEventSource as unknown as typeof EventSource;
  });
  const { calls } = await mockSpaces(page, { extra: {
    [`GET ${spacePath}/items`]: () => ({ body: { items: [project] } }),
    [`GET ${spacePath}/views`]: { body: { items: [{ id: "table", name: "Table", type: "table", is_default: true }] } },
    [`GET ${projectPath}`]: () => ({ body: project, headers: { ETag: `"${project.version}"` } }),
    [`GET ${projectPath}/children`]: { body: { items: [] } },
    [`GET ${projectPath}/views`]: { body: { items: [] } },
    [`PATCH ${projectPath}`]: (body: ItemUpdate, req) => {
      if (req.headers["if-match"] !== `"${project.version}"`) return problem(409, "Version conflict");
      project = { ...project, progress_method: body.progress_method, progress_percent: 40, version: project.version + 1 };
      return { body: project, headers: { ETag: `"${project.version}"` } };
    },
    "GET /workflows": workflows,
  } });
  async function descendantUpdated() {
    await page.evaluate((detail) => window.dispatchEvent(new CustomEvent("pms-test:item-updated", { detail })), {
      type: "item.updated", id: taskId, container_ids: [folderId],
    });
  }

  await page.goto(spacePath);
  const row = page.getByRole("row").filter({ has: page.getByRole("button", { name: "Campaign", exact: true }) });
  await expect(row.getByText("25%")).toBeVisible();
  await expect(row.getByText("Green risk")).toBeVisible();
  project = { ...project, progress_percent: 30, progress_risk: "yellow" };
  await descendantUpdated();
  await expect(row.getByText("30%")).toBeVisible();
  await expect(row.getByText("Yellow risk")).toBeVisible();

  await row.getByRole("button", { name: "Campaign", exact: true }).click();
  await page.getByText("More fields", { exact: true }).click();
  const panel = page.getByRole("region", { name: "Project progress" });
  await expect(panel.getByText("30%")).toBeVisible();
  const method = panel.getByRole("combobox", { name: "Progress method" });
  await expect(method).toHaveValue("task_count");
  await method.selectOption("weighted_duration");
  await expect(panel.getByText("40%")).toBeVisible();
  const patch = calls.find((call) => call.method === "PATCH" && call.path === projectPath);
  expect(patch?.body).toEqual({ progress_method: "weighted_duration" });
  expect(patch?.headers["if-match"]).toBe('"1"');

  project = { ...project, progress_percent: 50, progress_risk: "red" };
  await descendantUpdated();
  await expect(panel.getByText("50%")).toBeVisible();
  await expect(panel.getByText("Red risk")).toBeVisible();
  await expect(method).toHaveValue("weighted_duration");
});

test("Task manual progress accepts a bounded integer, can be cleared, and is 100% when completed", async ({ page }) => {
  let task: Item = { ...common, id: taskId, title: "Draft", type: "task", status: active, progress_percent: null };
  const { calls } = await mockSpaces(page, { extra: {
    [`GET ${taskPath}`]: () => ({ body: task, headers: { ETag: `"${task.version}"` } }),
    [`GET ${taskPath}/children`]: { body: { items: [] } },
    [`PATCH ${taskPath}`]: (body: ItemUpdate, req) => {
      if (req.headers["if-match"] !== `"${task.version}"`) return problem(409, "Version conflict");
      task = { ...task, version: task.version + 1,
        status: body.status_id === completed.id ? completed : task.status,
        progress_percent: body.progress_percent === undefined ? task.progress_percent : body.progress_percent,
      };
      return { body: task, headers: { ETag: `"${task.version}"` } };
    },
    "GET /workflows": workflows,
  } });

  await page.goto(`/tasks/${taskId}`);
  await page.getByText("More fields", { exact: true }).click();
  const panel = page.getByRole("region", { name: "Task progress" });
  const percent = panel.getByRole("spinbutton", { name: "Manual progress (%)" });
  const save = panel.getByRole("button", { name: "Save progress" });
  const patches = () => calls.filter((call) => call.method === "PATCH" && call.path === taskPath);
  await expect(percent).toHaveValue("");
  await percent.fill("101");
  await save.click();
  expect(await percent.evaluate((element: HTMLInputElement) => element.validity.rangeOverflow)).toBe(true);
  expect(patches()).toHaveLength(0);
  await percent.fill("45");
  await save.click();
  await expect(save).toBeDisabled();
  await expect(percent).toHaveValue("45");
  await percent.clear();
  await save.click();
  await expect(save).toBeDisabled();
  await expect(percent).toHaveValue("");
  await page.getByRole("combobox", { name: "Status", exact: true }).selectOption(completed.id);
  await expect(panel.getByText("100% complete")).toBeVisible();
  await percent.fill("15");
  await save.click();
  await expect(save).toBeDisabled();
  await expect(panel.getByText("100% complete")).toBeVisible();
  expect(patches().map((call) => [call.headers["if-match"], call.body])).toEqual([
    ['"1"', { progress_percent: 45 }],
    ['"2"', { progress_percent: null }],
    ['"3"', { status_id: completed.id }],
    ['"4"', { progress_percent: 15 }],
  ]);
});
