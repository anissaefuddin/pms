import { expect, test } from "@playwright/test";

// R5 live: a Task created in the Board (status column, parent project) is read back by the Table, and the server data that
// Table, Gantt and Calendar read agrees on status, parent, and dates. Runs only from the Go fixture (TestBrowserCrossView).
const e = process.env;
test.skip(!e.PMS_FIXTURE_XV_ADMIN, "Requires the local Go/PostgreSQL cross-view fixture");

test("A Task created in the Board is the same in the Table and in the server data every view reads", async ({ browser }) => {
  const baseURL = test.info().project.use.baseURL!;
  const context = await browser.newContext({ baseURL, colorScheme: "light" });
  await context.addCookies([{ name: "pms_session", value: e.PMS_FIXTURE_XV_ADMIN!.split("=")[1], url: baseURL }]);
  const page = await context.newPage();
  const project = e.PMS_FIXTURE_XV_PROJECT!;
  const api = (path: string) => page.request.get(`/api/v1/${path}`).then((r) => r.json());

  // 1. Board: quick create in the "In Progress" column of the project.
  await page.goto(`/items/${project}`);
  await page.getByRole("radio", { name: "Board" }).click();
  // The Board hides empty columns by default; "In Progress" has no card yet.
  await page.getByRole("button", { name: /empty/i, pressed: true }).click();
  const column = page.getByRole("region", { name: "In Progress", exact: true });
  await column.getByRole("button", { name: "Add item" }).click();
  await page.getByPlaceholder("Task name").fill("Cross view task");
  await page.getByPlaceholder("Task name").press("Enter");
  await expect(column.getByRole("button", { name: "Cross view task", exact: true })).toBeVisible();

  // 2. The server item: status from the column, parent is the project.
  const statuses = (await api("workflows")).items[0].statuses as { id: string; name: string }[];
  const inProgress = statuses.find((s) => s.name === "In Progress")!;
  const children = (await api(`items/${project}/children`)).items as any[];
  const created = children.find((c) => c.title === "Cross view task");
  expect(created, "the child appears under the project").toBeDefined();
  expect(created.status.id).toBe(inProgress.id);
  expect(created.locations[0].parent_item_id).toBe(project);

  // 3. Dates set on the server (PATCH with the current ETag), then the children data every view reads must agree.
  const current = await page.request.get(`/api/v1/items/${created.id}`);
  const etag = current.headers()["etag"];
  const patched = await page.request.patch(`/api/v1/items/${created.id}`, {
    headers: { "If-Match": etag, "Content-Type": "application/json" },
    data: { start_date: "2026-12-01", due_date: "2026-12-03" },
  });
  expect(patched.status()).toBe(200);
  const after = ((await api(`items/${project}/children`)).items as any[]).find((c) => c.id === created.id);
  expect(after.status.id).toBe(inProgress.id);
  expect(after.locations[0].parent_item_id).toBe(project);
  expect(after.start_date).toBe("2026-12-01");
  expect(after.due_date).toBe("2026-12-03");

  // 4. Table: the same Task, same parent, same status, after reload.
  await page.reload();
  await page.getByRole("radio", { name: "Table" }).click();
  const row = page.getByRole("row", { name: /Cross view task/ });
  await expect(row).toBeVisible();
  await expect(row.getByRole("combobox")).toHaveValue(inProgress.id);
  await context.close();
});
