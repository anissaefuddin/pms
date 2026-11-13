import assert from "node:assert/strict";
import test from "node:test";
import { answerErrors, buildAnswers, buildInput, buildSubmission, validateAnswers, draftFromForm, emptyDraft, errorTarget, mapErrors, moveAt, newElement, slugify, uniqueKey, type Form } from "./forms.ts";

test("slugify yields keys matching the server pattern", () => {
  assert.equal(slugify("Your full name"), "your_full_name");
  assert.equal(slugify("  Café -- Menu! "), "cafe_menu");
  assert.equal(slugify("2nd choice"), "q_2nd_choice");
  assert.equal(slugify("???"), "question");
  const long = slugify("a".repeat(80));
  assert.equal(long.length, 40);
  for (const s of ["Name", "9 lives", "x".repeat(60), "Ünïcode"]) assert.match(slugify(s), /^[a-z][a-z0-9_]{0,39}$/);
});

test("uniqueKey adds a numeric suffix and stays within 40 chars", () => {
  assert.equal(uniqueKey("Name", []), "name");
  assert.equal(uniqueKey("Name", ["name"]), "name_2");
  assert.equal(uniqueKey("Name", ["name", "name_2"]), "name_3");
  const long = "a".repeat(40);
  const key = uniqueKey(long, [long]);
  assert.equal(key.length, 40);
  assert.ok(key.endsWith("_2"));
});

test("moveAt swaps neighbours and stops at the edges", () => {
  assert.deepEqual(moveAt(["a", "b", "c"], 1, -1), ["b", "a", "c"]);
  assert.deepEqual(moveAt(["a", "b", "c"], 1, 1), ["a", "c", "b"]);
  const list = ["a", "b"];
  assert.equal(moveAt(list, 0, -1), list);
  assert.equal(moveAt(list, 1, 1), list);
});

test("buildInput sends only what each kind understands", () => {
  const d = emptyDraft("s1");
  d.name = " Intake ";
  const choice = newElement("single_choice", "Pick", []);
  choice.options = ["A", " ", "B"];
  choice.required = true;
  const section = { ...newElement("section", "Part", ["pick"]), required: true, help: "Intro" };
  const num = { ...newElement("number", "Qty", ["pick", "part"]), min: 1, max: 9, options: ["stray"] };
  d.pages[0].elements = [choice, section, num, newElement("attachment", "File", [])];
  d.title_question = "";
  d.approval = true; d.approvers = [{ id: "u1", name: "U" }];
  const body = buildInput(d);
  assert.equal(body.name, "Intake");
  assert.deepEqual(body.definition.pages[0].elements, [
    { key: "pick", kind: "single_choice", label: "Pick", required: true, options: ["A", "B"] },
    { key: "part", kind: "section", label: "Part", help: "Intro" },
    { key: "qty", kind: "number", label: "Qty", min: 1, max: 9 },
    { key: "file", kind: "attachment", label: "File", max_files: 1 },
  ]);
  assert.deepEqual(body.response, { item_type: "task", status_id: null, location: { space_id: "s1", parent_item_id: null },
    assignee_ids: [], title_question: null, approval: { approver_ids: ["u1"], description: undefined } });
});

test("draftFromForm keeps element keys and round-trips through buildInput", () => {
  const form = { name: "F", description: "d", definition: { pages: [{ title: "P", elements: [{ key: "old_key", kind: "single_line", label: "Renamed label" }] }] },
    response: { item_type: "project", status_id: "st", location: { space_id: "s", parent_item_id: "p" }, assignee_ids: ["u"], title_question: "old_key", approval: null } } as unknown as Form;
  const body = buildInput(draftFromForm(form));
  assert.equal(body.definition.pages[0].elements[0].key, "old_key");
  assert.equal(body.response.status_id, "st");
  assert.equal(body.response.location.parent_item_id, "p");
  assert.deepEqual(body.response.assignee_ids, ["u"]);
});

test("errorTarget and mapErrors route server paths to controls", () => {
  assert.deepEqual(errorTarget("definition.pages[0].elements[1].options"), { area: "element", page: 0, element: 1, prop: "options" });
  assert.deepEqual(errorTarget("definition.pages[2].title"), { area: "page", page: 2, prop: "title" });
  assert.deepEqual(errorTarget("response.location.space_id"), { area: "field", name: "response.location.space_id" });
  const { byControl, summary } = mapErrors([
    { field: "definition.pages[0].elements[1].options", message: "need 2" },
    { field: "definition.pages[0].elements[1].options", message: "dup" },
    { field: "definition.pages", message: "too few" },
    { field: "name", message: "required" },
  ]);
  assert.deepEqual(byControl, { "e0-1-options": "need 2", name: "required" });
  assert.deepEqual(summary, ["dup", "too few"]);
});

const def: Form["definition"] = { pages: [{ title: "", elements: [
  { key: "name", kind: "single_line", label: "Name", required: true },
  { key: "qty", kind: "number", label: "Qty", min: 1, max: 5 },
  { key: "who", kind: "assignee", label: "Who" },
  { key: "crew", kind: "assignee", label: "Crew", multiple: true },
  { key: "tags", kind: "multiple_choice", label: "Tags", options: ["a", "b"] },
  { key: "doc", kind: "attachment", label: "Doc", max_files: 2 },
  { key: "part", kind: "section", label: "Part" }] }] };

test("buildAnswers shapes each kind and omits blanks", () => {
  assert.deepEqual(buildAnswers(def, { name: " Ada ", qty: "3", who: ["u1"], crew: ["u1", "u2"], tags: [], doc: ["x.pdf"] }),
    { name: "Ada", qty: 3, who: "u1", crew: ["u1", "u2"] });
});

test("buildSubmission is JSON without files and multipart with them", async () => {
  const none = buildSubmission(def, { name: "Ada" }, {});
  assert.equal(none.body, JSON.stringify({ answers: { name: "Ada" } }));
  const file = new File(["hi"], "a.txt", { type: "text/plain" });
  const multi = buildSubmission(def, { name: "Ada" }, { doc: [file, file], other: [file] });
  assert.ok(multi.body instanceof FormData);
  assert.deepEqual(JSON.parse(String(multi.body.get("answers"))), { name: "Ada" });
  assert.equal(multi.body.getAll("file:doc").length, 2);
  assert.equal(multi.body.has("file:other"), false);
});

test("validateAnswers flags required, non-numeric and out-of-range", () => {
  assert.deepEqual(validateAnswers(def, {}, {}), { name: { code: "required" } });
  assert.deepEqual(validateAnswers(def, { name: "x", qty: "9" }, {}), { qty: { code: "max", limit: 5 } });
  assert.deepEqual(validateAnswers(def, { name: "x", qty: "0" }, {}), { qty: { code: "min", limit: 1 } });
  assert.deepEqual(validateAnswers(def, { name: "x", qty: "abc" }, {}), { qty: { code: "number" } });
  assert.deepEqual(validateAnswers(def, { name: "x", qty: "2" }, {}), {});
});

test("answerErrors maps answers.<key> back to question keys", () => {
  assert.deepEqual(answerErrors([{ field: "answers.name", message: "needed" }, { field: "answers.name", message: "dup" }, { field: "other", message: "x" }]),
    { byKey: { name: "needed" }, rest: ["dup", "x"] });
});
