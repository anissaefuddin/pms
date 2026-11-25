import { expect, test } from "@playwright/test";
import { ALL, mockSpaces } from "./spaces-mock";

test("pins a task and saves a weekly duration and comment", async ({ page }) => {
  const task = { id: "11111111-1111-4111-8111-111111111111", type: "task", title: "Delivery", my_capabilities: ALL };
  let entries: object[] = [];
  const { calls } = await mockSpaces(page, { extra: {
    "GET /search": { body: { items: [task], comments: [], users: [] } },
    "GET /time-entries": () => ({ body: { items: entries, next_offset: null } }),
    "POST /time-entries": (body) => {
      entries = [{ ...body, id: "entry", version: 1, status: "draft", review_note: "", item_title: task.title }];
      return { status: 201, body: entries[0] };
    },
  } });
  await page.goto("/timesheets");
  await page.getByRole("searchbox", { name: "Find a task to pin" }).fill("Delivery");
  await page.getByRole("button", { name: "Pin Delivery", exact: true }).click();
  const durations = page.getByRole("textbox", { name: /Duration for Delivery/ });
  await expect(durations).toHaveCount(7);
  await durations.first().fill("1:30");
  await durations.first().press("Alt+ArrowRight");
  await expect(durations.nth(1)).toBeFocused();
  await page.getByRole("textbox", { name: /Comment for Delivery/ }).first().fill("Reviewed plan");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect.poll(() => calls.find((call) => call.method === "POST" && call.path === "/time-entries")?.body)
    .toMatchObject({ item_id: task.id, minutes: 90, comment: "Reviewed plan" });
  await expect(page.getByRole("cell", { name: "1:30", exact: true }).last()).toBeVisible();
  await page.reload();
  await expect(page.getByRole("rowheader", { name: /Delivery/ })).toBeVisible();
});

test("keeps a conflicted cell draft until the user discards it", async ({ page }) => {
  const task = "11111111-1111-4111-8111-111111111111";
  let version = 1;
  const { calls } = await mockSpaces(page, { extra: {
    "GET /time-entries": (_, request) => ({ body: { items: [{
      id: "entry", item_id: task, item_title: "Delivery", date: request.query.get("from"),
      minutes: version === 1 ? 90 : 120, comment: "", locked: false, version, status: "draft", review_note: "",
    }], next_offset: null } }),
    "PATCH /time-entries/entry": () => {
      version = 2;
      return { status: 409, body: { detail: "Time entry version changed" } };
    },
  } });
  await page.goto("/timesheets");
  const duration = page.getByRole("textbox", { name: /Duration for Delivery/ }).first();
  await expect(duration).toHaveValue("1:30");
  await duration.fill("3:00");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("This cell changed");
  await expect(duration).toHaveValue("3:00");
  expect(calls.find((call) => call.method === "PATCH")?.headers["if-match"]).toBe('"1"');
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(duration).toHaveValue("2:00");
});

test("submits the week, shows the decision and its history, and reviews other people's time", async ({ page }) => {
  const task = "11111111-1111-4111-8111-111111111111";
  let status = "draft";
  let waiting = true;
  const { calls } = await mockSpaces(page, { extra: {
    "GET /time-entries": (_, request) => ({ body: { items: [{ id: "entry", item_id: task, item_title: "Delivery", date: request.query.get("from"),
      minutes: 90, comment: "", locked: false, version: 1, status, review_note: status === "rejected" ? "Wrong task" : "" }], next_offset: null } }),
    "POST /time-entries/submit": () => { status = "rejected"; return { body: { submitted: 1 } }; },
    "GET /time-entries/entry/history": { body: { items: [{ action: "submitted", actor_name: "Ada Lovelace", note: "", minutes: 90, at: "2026-11-23T08:00:00Z" },
      { action: "rejected", actor_name: "Grace Hopper", note: "Wrong task", minutes: 90, at: "2026-11-23T09:00:00Z" }] } },
    "GET /time-entries/review": () => ({ body: { items: waiting ? [{ id: "theirs", item_id: task, item_title: "Delivery", user_name: "Grace Hopper", date: "2026-11-23",
      minutes: 60, comment: "Design", locked: false, version: 4, status: "submitted", review_note: "" }] : [], next_offset: null } }),
    "POST /time-entries/theirs/review": () => { waiting = false; return { body: {} }; },
  } });
  await page.goto("/timesheets");
  await page.getByRole("button", { name: "Submit 1 entry" }).click();
  await expect(page.getByText("Rejected: Wrong task")).toBeVisible();
  expect(Object.keys(calls.find((call) => call.path === "/time-entries/submit")?.body as object).sort()).toEqual(["from", "to"]);
  await page.getByRole("button", { name: /^History of Delivery/ }).click();
  await expect(page.getByRole("dialog").getByText(/Grace Hopper · Rejected · 1:30 · Wrong task/)).toBeVisible();
  await page.keyboard.press("Escape");
  const review = page.getByRole("region", { name: "Time to review" });
  await review.getByRole("textbox").fill("Thanks");
  await review.getByRole("button", { name: /^Approve Grace Hopper/ }).click();
  await expect(review).toBeHidden();
  const sent = calls.find((call) => call.path === "/time-entries/theirs/review");
  expect(sent?.headers["if-match"]).toBe('"4"');
  expect(sent?.body).toEqual({ decision: "approved", note: "Thanks" });
});
