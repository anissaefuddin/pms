import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Browser, type Page } from "@playwright/test";

const e = process.env;
test.skip(!e.PMS_FIXTURE_M3_ADMIN, "Requires the local Go/PostgreSQL M3 fixture");
test.describe.configure({ mode: "serial" });

/** A page in its own browser context, signed in with the given fixture cookie. */
async function as(browser: Browser, cookie: string) {
  const baseURL = test.info().project.use.baseURL!;
  const context = await browser.newContext({ baseURL, colorScheme: "light" });
  await context.addCookies([{ name: "pms_session", value: cookie.split("=")[1], url: baseURL }]);
  return context.newPage();
}
const item = async (page: Page, id: string) => (await (await page.request.get(`/api/v1/items/${id}`)).json());
async function axe(page: Page) {
  await expect(page.locator('[data-starting-style], [data-ending-style]')).toHaveCount(0); // animations settled
  const result = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
  expect(result.violations.map((v) => `${v.id}: ${v.nodes[0]?.html}`)).toEqual([]);
}

let requestId = ""; // the Item created by the form in flow 2, used by flow 3
const subject = "Printer on floor 3 is jammed";

test("flow 1: locked transitions are enforced in the item panel and auto-assign makes the mover an assignee", async ({ browser }) => {
  const admin = await as(browser, e.PMS_FIXTURE_M3_ADMIN!);
  await admin.goto("/settings/workflows");
  await admin.getByRole("combobox", { name: "Workflow", exact: true }).selectOption(e.PMS_FIXTURE_M3_WORKFLOW!);
  await expect(admin.getByLabel("Status name: New")).toBeVisible();
  await admin.getByLabel("Lock transitions: New").click(); // controlled: flips once the server answers
  await expect(admin.getByLabel("Lock transitions: New")).toBeChecked();
  const allowed = admin.getByRole("group", { name: "Allowed next statuses: New" });
  await allowed.getByLabel("In Progress", { exact: true }).click();
  await expect(allowed.getByLabel("In Progress", { exact: true })).toBeChecked();
  await admin.getByLabel("Auto-assign: In Progress").click();
  await expect(admin.getByLabel("Auto-assign: In Progress")).toBeChecked();
  await axe(admin);
  const statuses = async () => ((await (await admin.request.get("/api/v1/workflows")).json()).items as { id: string; statuses: any[] }[])
    .find((w) => w.id === e.PMS_FIXTURE_M3_WORKFLOW)!.statuses;
  const byName = async (name: string) => (await statuses()).find((s) => s.name === name)!;
  await expect.poll(async () => (await byName("New")).allowed_next).toEqual([(await byName("In Progress")).id]);
  expect((await byName("New")).lock_transitions).toBe(true);
  expect((await byName("In Progress")).auto_assign).toBe(true);

  await admin.goto(`/tasks/${e.PMS_FIXTURE_M3_LOCKED}`);
  const status = admin.getByLabel("Status", { exact: true });
  await expect(status).toHaveValue((await byName("New")).id);
  await status.selectOption({ label: "Completed" });
  await expect(admin.getByRole("alert").filter({ hasText: "this workflow does not allow that status change" })).toBeVisible();
  await expect(status).toHaveValue((await byName("New")).id);
  expect((await item(admin, e.PMS_FIXTURE_M3_LOCKED!)).status.name).toBe("New");
  await status.selectOption({ label: "In Progress" });
  await expect.poll(async () => (await item(admin, e.PMS_FIXTURE_M3_LOCKED!)).status.name).toBe("In Progress");
  await expect(admin.getByRole("alert").filter({ hasText: "does not allow" })).toHaveCount(0);

  const editor = await as(browser, e.PMS_FIXTURE_M3_EDITOR!);
  expect((await item(editor, e.PMS_FIXTURE_M3_AUTO!)).assignees).toEqual([]);
  await editor.goto(`/tasks/${e.PMS_FIXTURE_M3_AUTO}`);
  await editor.getByLabel("Status", { exact: true }).selectOption({ label: "In Progress" });
  await expect.poll(async () => (await item(editor, e.PMS_FIXTURE_M3_AUTO!)).status.name).toBe("In Progress");
  expect((await item(editor, e.PMS_FIXTURE_M3_AUTO!)).assignees.map((a: { id: string }) => a.id)).toEqual([e.PMS_FIXTURE_M3_EDITOR_ID]);
  await admin.context().close();
  await editor.context().close();
});

test("flow 2: a form built in the UI is shared, filled by a requester, approved, and its status reaches the requester", async ({ browser }) => {
  const admin = await as(browser, e.PMS_FIXTURE_M3_ADMIN!);
  await admin.goto(`/spaces/${e.PMS_FIXTURE_M3_INTAKE}/forms/new`);
  await admin.getByLabel("Form name").fill("Facilities request");
  const addQuestion = async (kind: string) => {
    await admin.getByRole("button", { name: "Add question to page 1" }).click();
    await admin.getByRole("menuitem", { name: kind, exact: true }).click();
  };
  await addQuestion("Single line text");
  await admin.getByLabel("Label", { exact: true }).nth(0).fill("Subject");
  await admin.getByLabel("Required", { exact: true }).check();
  await addQuestion("Importance");
  await addQuestion("Attachment");
  await admin.getByLabel("Maximum files (1-5)").fill("1");
  await admin.getByLabel("Create as").selectOption("task");
  await admin.getByLabel("Destination Space").selectOption({ label: "M3 Work" });
  await admin.getByLabel("Parent project or folder").selectOption({ label: "M3 Platform" });
  await admin.getByLabel("Item title comes from").selectOption({ label: "Subject" });
  await admin.getByLabel("Request approval for each new item").check();
  await admin.getByRole("searchbox", { name: /Approvers/ }).fill("Avery");
  await admin.getByRole("button", { name: "Avery Approver", exact: true }).click();
  await axe(admin);
  await admin.getByRole("button", { name: "Save form" }).click();
  await expect(admin).toHaveURL(new RegExp(`/spaces/${e.PMS_FIXTURE_M3_INTAKE}/forms/[0-9a-f-]{36}$`));
  const formId = admin.url().split("/").pop()!;

  await admin.goto("/forms");
  const row = admin.getByRole("row", { name: /Facilities request/ });
  await expect(row).toContainText("Only form managers");
  await axe(admin);
  await admin.getByRole("button", { name: "Actions for Facilities request" }).click();
  await admin.getByRole("menuitem", { name: "Share / Change access" }).click();
  await admin.getByRole("radio", { name: /Everyone in the workspace/ }).check();
  await admin.getByRole("button", { name: "Save", exact: true }).click();
  await expect(admin.getByText("Form access updated")).toBeVisible();
  await expect(row.getByRole("link", { name: /Open form/ })).toHaveAttribute("href", new RegExp(`/requests/${formId}$`)); // the label replaced the raw URL (UI-05)

  // The requester has no access to the destination Space and works in their own context.
  const requester = await as(browser, e.PMS_FIXTURE_M3_REQUESTER!);
  await requester.goto("/requests");
  await expect(requester.getByRole("link", { name: "Start Facilities request" })).toBeVisible();
  await axe(requester);
  await requester.getByRole("link", { name: "Start Facilities request" }).click();
  await expect(requester).toHaveURL(`/requests/${formId}`);
  await requester.getByLabel(/Subject/).fill(subject);
  await requester.getByLabel("Importance").selectOption({ label: "High" });
  await requester.getByLabel("Attachment").setInputFiles({ name: "jam.txt", mimeType: "text/plain", buffer: Buffer.from("Tray 2 is stuck") });
  await expect(requester.getByText("jam.txt")).toBeVisible();
  await requester.getByRole("button", { name: "Submit" }).click();
  await expect(requester.getByRole("heading", { name: "Request sent" })).toBeVisible();

  await requester.goto("/requests/mine");
  const mine = requester.getByRole("listitem").filter({ hasText: subject });
  await expect(mine.getByText("New", { exact: true })).toBeVisible();
  await expect(mine.getByText("Form: Facilities request")).toBeVisible();
  await axe(requester);
  const link = mine.getByRole("link", { name: subject });
  requestId = (await link.getAttribute("href"))!.split("/").pop()!;
  const created = await item(admin, requestId);
  expect(created.importance).toBe("high");
  expect(created.locations[0]).toMatchObject({ space_id: e.PMS_FIXTURE_M3_WORK, parent_item_id: e.PMS_FIXTURE_M3_PROJECT });
  const files = await (await admin.request.get(`/api/v1/items/${requestId}/attachments`)).json();
  expect(files.items.map((f: { filename: string }) => f.filename)).toEqual(["jam.txt"]);

  await link.click();
  await expect(requester.getByRole("heading", { name: subject })).toBeVisible();
  await expect(requester.getByLabel("Status", { exact: true })).toHaveCount(0); // limited view: no edit controls
  await expect(requester.getByRole("button", { name: /^(Save|Add subitem|Add approval|Edit approval|Delete approval)/ })).toHaveCount(0);
  await requester.getByRole("button", { name: "Actions" }).click();
  await expect(requester.getByRole("menuitem", { name: "Copy link" })).toBeVisible();
  await expect(requester.getByRole("menuitem", { name: "Delete" })).toHaveCount(0);
  await requester.keyboard.press("Escape");
  await expect(requester.getByRole("menuitem")).toHaveCount(0);
  await axe(requester);
  await requester.getByRole("textbox", { name: "Comment" }).fill("Any update on the printer?");
  await requester.getByRole("button", { name: "Post comment" }).click();
  await expect(requester.getByRole("region", { name: "Comments" }).getByText("Any update on the printer?")).toBeVisible();

  const approver = await as(browser, e.PMS_FIXTURE_M3_APPROVER!);
  await approver.goto(`/tasks/${requestId}`);
  const panel = approver.getByRole("region", { name: "Approvals" });
  await expect(panel.getByText("0/1 approved · Pending")).toBeVisible();
  await panel.getByRole("button", { name: "Approve", exact: true }).click();
  await expect(panel.getByText("1/1 approved · Approved")).toBeVisible();
  const approvals = (await (await approver.request.get(`/api/v1/items/${requestId}/approvals`)).json()).items;
  expect(approvals).toHaveLength(1);
  expect(approvals[0]).toMatchObject({ status: "approved", approved_count: 1, total: 1 });

  await admin.goto(`/tasks/${requestId}`);
  await admin.getByLabel("Status", { exact: true }).selectOption({ label: "In Progress" });
  await expect.poll(async () => (await item(admin, requestId)).status.name).toBe("In Progress");

  await requester.goto("/requests/mine");
  await expect(requester.getByRole("listitem").filter({ hasText: subject }).getByText("In Progress", { exact: true })).toBeVisible();
  await expect.poll(async () => ((await (await requester.request.get("/api/v1/notifications")).json()).items as any[])
    .filter((n) => n.kind === "status_changed" && n.item_id === requestId).length).toBe(1);
  await requester.goto("/inbox");
  await expect(requester.getByRole("button", { name: new RegExp(`changed the status of ${subject}`) })).toBeVisible();
  for (const page of [admin, requester, approver]) await page.context().close();
});

test("flow 3: the requester reaches nothing around the request, and private forms stay hidden", async ({ browser }) => {
  expect(requestId).not.toBe("");
  const requester = await as(browser, e.PMS_FIXTURE_M3_REQUESTER!);
  for (const path of [`/api/v1/spaces/${e.PMS_FIXTURE_M3_WORK}`, `/api/v1/items/${e.PMS_FIXTURE_M3_PROJECT}`, `/api/v1/items/${e.PMS_FIXTURE_M3_SIBLING}`]) {
    expect((await requester.request.get(path)).status(), path).toBe(404);
  }
  expect((await requester.request.get(`/api/v1/items/${requestId}`)).status()).toBe(200);
  await requester.goto(`/tasks/${e.PMS_FIXTURE_M3_SIBLING}`);
  await expect(requester.getByRole("alert")).toBeVisible();
  await expect(requester.getByText("Sibling secret")).toHaveCount(0);
  await requester.goto(`/tasks/${requestId}`);
  await expect(requester.getByRole("heading", { name: subject })).toBeVisible();
  await expect(requester.getByText("Sibling secret")).toHaveCount(0);
  await expect(requester.getByText("M3 Platform")).toHaveCount(0);

  await requester.goto("/requests");
  await expect(requester.getByRole("link", { name: "Start Facilities request" })).toBeVisible();
  await expect(requester.getByText("Private M3 form")).toHaveCount(0);
  await requester.context().close();
});

test("flow 4: a visitor without an account files a request through a public link, and the link can be replaced", async ({ browser }) => {
  const baseURL = test.info().project.use.baseURL!;
  const context = await browser.newContext({ baseURL, colorScheme: "light" });
  const visitor = await context.newPage();
  await visitor.goto(`/f/${e.PMS_FIXTURE_M3_PUBLIC_TOKEN}`);
  await expect(visitor.getByRole("heading", { name: "Public M3 form" })).toBeVisible();
  await expect(visitor).toHaveURL(`/f/${e.PMS_FIXTURE_M3_PUBLIC_TOKEN}`);
  await axe(visitor);
  await visitor.getByLabel("Subject").fill("Badge reader is broken");
  await visitor.getByLabel("Your name").fill("Pat Visitor");
  await visitor.getByLabel("Your email").fill("pat.visitor@example.com");
  await visitor.getByRole("button", { name: "Submit" }).click();
  await expect(visitor.getByRole("heading", { name: "Request received" })).toBeVisible();
  await expect(visitor.getByText("pat.visitor@example.com")).toBeVisible();
  // Without a session the visitor reaches nothing else.
  for (const path of ["/api/v1/me", "/api/v1/spaces", `/api/v1/spaces/${e.PMS_FIXTURE_M3_WORK}`, `/api/v1/items/${e.PMS_FIXTURE_M3_PROJECT}`]) {
    expect((await visitor.request.get(path)).status(), path).toBe(401);
  }
  await context.close();

  const admin = await as(browser, e.PMS_FIXTURE_M3_ADMIN!);
  const found = await (await admin.request.get("/api/v1/search?q=Badge")).json();
  const created = (found.items as any[]).find((entry) => entry.title === "Badge reader is broken");
  expect(created, "the item exists in the destination Space").toBeTruthy();
  expect(created.created_by.name).toBe("External requester");
  expect(JSON.stringify(created)).not.toContain("pat.visitor@example.com"); // the email stays on the submission, not on the item

  // Replacing the link in the UI kills the old one.
  await admin.goto("/forms");
  await admin.getByRole("button", { name: "Actions for Public M3 form" }).click();
  await admin.getByRole("menuitem", { name: "Share / Change access" }).click();
  const dialog = admin.getByRole("dialog");
  await dialog.getByRole("button", { name: "Replace link" }).click();
  await dialog.getByRole("button", { name: "Replace link" }).click();
  await expect(dialog.getByLabel("Public link address")).not.toHaveValue(new RegExp(`/f/${e.PMS_FIXTURE_M3_PUBLIC_TOKEN}$`));
  await expect.poll(async () => (await admin.request.get(`/api/v1/public/forms/${e.PMS_FIXTURE_M3_PUBLIC_TOKEN}`)).status()).toBe(404);
  await admin.context().close();
});
