import type { components } from "@/api/schema";

export type Form = components["schemas"]["Form"];
export type FormInput = components["schemas"]["FormInput"];
export type FormSummary = components["schemas"]["FormSummary"];
export type FormElement = components["schemas"]["FormElement"];
export type FormPage = components["schemas"]["FormPage"];
export type FormDefinition = components["schemas"]["FormDefinition"];
export type ElementKind = FormElement["kind"];
/** Requester answers by element key: text (numbers stay text until sent), or string[] (choices, user ids). Files live apart. */
export type Answers = Record<string, string | string[]>;

export const ELEMENT_KINDS: ElementKind[] = ["single_line", "paragraph", "single_choice", "multiple_choice", "date", "number", "attachment", "assignee", "importance", "section"];
export const IMPORTANCE = ["high", "normal", "low"] as const;
export const MAX_PAGES = 10;
export const MAX_ELEMENTS = 100;

export const formKeys = { list: (spaceId: string) => ["forms", spaceId] as const, one: (id: string) => ["forms", "one", id] as const };

/** Lowercase, `_`-separated, starts with a letter, at most 40 chars: matches ^[a-z][a-z0-9_]{0,39}$. */
export function slugify(label: string): string {
  let slug = label.normalize("NFKD").replace(/[̀-ͯ]/g, "").toLowerCase().replace(/[^a-z0-9]+/g, "_").replace(/^_+|_+$/g, "");
  if (!slug) slug = "question";
  if (!/^[a-z]/.test(slug)) slug = `q_${slug}`;
  return slug.slice(0, 40).replace(/_+$/, "");
}

/** Slug of `label`, suffixed `_2`, `_3`... until it is not in `taken`; the suffix never pushes it past 40 chars. */
export function uniqueKey(label: string, taken: Iterable<string>): string {
  const used = new Set(taken);
  const base = slugify(label);
  let key = base;
  for (let n = 2; used.has(key); n++) key = `${base.slice(0, 40 - String(n).length - 1)}_${n}`;
  return key;
}

export const allKeys = (pages: FormPage[]) => pages.flatMap((page) => page.elements.map((element) => element.key));

/** Moves `index` one step (-1 up, +1 down); unchanged at the edges. */
export function moveAt<T>(list: T[], index: number, delta: -1 | 1): T[] {
  const to = index + delta;
  if (index < 0 || index >= list.length || to < 0 || to >= list.length) return list;
  const next = [...list];
  next.splice(to, 0, ...next.splice(index, 1));
  return next;
}

export const isChoice = (kind: ElementKind) => kind === "single_choice" || kind === "multiple_choice";

/** A fresh element: key derived from the label once, then never touched again. */
export function newElement(kind: ElementKind, label: string, taken: Iterable<string>): FormElement {
  const element: FormElement = { key: uniqueKey(label, taken), kind, label };
  if (isChoice(kind)) element.options = ["", ""];
  if (kind === "attachment") element.max_files = 1;
  return element;
}

export type FormDraft = {
  name: string;
  description: string;
  pages: FormPage[];
  item_type: "task" | "project";
  status_id: string;
  space_id: string;
  parent_item_id: string;
  assignees: { id: string; name: string }[];
  title_question: string;
  approval: boolean;
  approvers: { id: string; name: string }[];
  approval_description: string;
};

export const emptyDraft = (spaceId: string): FormDraft => ({
  name: "", description: "", pages: [{ title: "", elements: [] }], item_type: "task", status_id: "", space_id: spaceId, parent_item_id: "",
  assignees: [], title_question: "", approval: false, approvers: [], approval_description: "",
});

/** Only the properties a kind understands are sent, so switching nothing lingers (the server rejects strays). */
function cleanElement(e: FormElement): FormElement {
  const out: FormElement = { key: e.key, kind: e.kind, label: e.label };
  if (e.help) out.help = e.help;
  if (e.kind !== "section" && e.required) out.required = true;
  if (isChoice(e.kind)) out.options = (e.options ?? []).map((o) => o.trim()).filter(Boolean);
  if (e.kind === "number") {
    if (e.min !== undefined) out.min = e.min;
    if (e.max !== undefined) out.max = e.max;
  }
  if (e.kind === "attachment") out.max_files = e.max_files;
  if (e.kind === "assignee" && e.multiple) out.multiple = true;
  return out;
}

export function buildInput(d: FormDraft): FormInput {
  const response: FormInput["response"] = {
    item_type: d.item_type,
    status_id: d.status_id || null,
    location: { space_id: d.space_id, parent_item_id: d.parent_item_id || null },
    assignee_ids: d.assignees.map((a) => a.id),
    title_question: d.title_question || null,
    approval: d.approval ? { approver_ids: d.approvers.map((a) => a.id), description: d.approval_description || undefined } : null,
  };
  return {
    name: d.name.trim(), description: d.description,
    definition: { pages: d.pages.map((p) => ({ title: p.title, elements: p.elements.map(cleanElement) })) },
    response,
  };
}

/** Inverse of buildInput; person names are filled in later (the API returns ids only). */
export function draftFromForm(f: Form): FormDraft {
  const r = f.response;
  return {
    name: f.name, description: f.description, pages: structuredClone(f.definition.pages),
    item_type: r.item_type, status_id: r.status_id ?? "", space_id: r.location.space_id, parent_item_id: r.location.parent_item_id ?? "",
    assignees: (r.assignee_ids ?? []).map((id) => ({ id, name: id })), title_question: r.title_question ?? "",
    approval: !!r.approval, approvers: (r.approval?.approver_ids ?? []).map((id) => ({ id, name: id })), approval_description: r.approval?.description ?? "",
  };
}

export type ErrorTarget =
  | { area: "element"; page: number; element: number; prop: string }
  | { area: "page"; page: number; prop: string }
  | { area: "field"; name: string };

/** Maps a server error path such as `definition.pages[0].elements[1].options` to the control that owns it. */
export function errorTarget(field: string): ErrorTarget {
  const el = /^definition\.pages\[(\d+)\]\.elements\[(\d+)\](?:\.(\w+))?/.exec(field);
  if (el) return { area: "element", page: +el[1], element: +el[2], prop: el[3] ?? "" };
  const pg = /^definition\.pages\[(\d+)\](?:\.(\w+))?/.exec(field);
  if (pg) return { area: "page", page: +pg[1], prop: pg[2] ?? "" };
  return { area: "field", name: field };
}

const ELEMENT_PROPS = new Set(["label", "help", "options", "min", "max", "max_files"]);
const FIELD_IDS = new Set(["name", "description", "response.item_type", "response.status_id", "response.location.space_id", "response.location.parent_item_id",
  "response.assignee_ids", "response.title_question", "response.approval.approver_ids", "response.approval.description"]);

/**
 * Splits errors into those shown beside a control (keyed by control id: `e<page>-<element>-<prop>`, `p<page>-title`
 * or the field path) and the rest for the form-level summary. Only ids the builder renders count as controls.
 */
export function mapErrors(errors: { field: string; message: string }[]) {
  const byControl: Record<string, string> = {};
  const summary: string[] = [];
  for (const { field, message } of errors) {
    const target = errorTarget(field.replace(/^(response\.\w+)\[\d+\]/, "$1"));
    const id = target.area === "element" ? (ELEMENT_PROPS.has(target.prop) ? `e${target.page}-${target.element}-${target.prop}` : "")
      : target.area === "page" ? (target.prop === "title" ? `p${target.page}-title` : "")
      : FIELD_IDS.has(target.name) ? target.name : "";
    if (id && !(id in byControl)) byControl[id] = message; else summary.push(message);
  }
  return { byControl, summary };
}

export type Files = Record<string, File[]>;
export type FieldProblem = { code: "required" | "number" | "min" | "max"; limit?: number };

const isEmpty = (value: string | string[] | undefined) => value === undefined || (Array.isArray(value) ? value.length === 0 : value.trim() === "");

/** The answers object the server expects: unanswered and attachment questions omitted, numbers as numbers, single assignee as one id. */
export function buildAnswers(definition: FormDefinition, answers: Answers): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const element of definition.pages.flatMap((page) => page.elements)) {
    const value = answers[element.key];
    if (element.kind === "section" || element.kind === "attachment" || isEmpty(value)) continue;
    if (element.kind === "number") out[element.key] = Number(value);
    else if (element.kind === "assignee") out[element.key] = element.multiple ? value : [value].flat()[0];
    else out[element.key] = Array.isArray(value) ? value : value!.trim();
  }
  return out;
}

/** JSON without files; otherwise multipart with `answers` plus `file:<key>` parts (no Content-Type: the browser adds the boundary). */
export function buildSubmission(definition: FormDefinition, answers: Answers, files: Files): { method: "POST"; body: string | FormData } {
  const built = buildAnswers(definition, answers);
  const attached = definition.pages.flatMap((page) => page.elements).filter((e) => e.kind === "attachment" && files[e.key]?.length);
  if (!attached.length) return { method: "POST", body: JSON.stringify({ answers: built }) };
  const body = new FormData();
  body.append("answers", JSON.stringify(built));
  for (const element of attached) for (const file of files[element.key]) body.append(`file:${element.key}`, file);
  return { method: "POST", body };
}

/** Cheap checks before sending; the server stays authoritative. */
export function validateAnswers(definition: FormDefinition, answers: Answers, files: Files): Record<string, FieldProblem> {
  const out: Record<string, FieldProblem> = {};
  for (const e of definition.pages.flatMap((page) => page.elements)) {
    if (e.kind === "section") continue;
    const value = answers[e.key];
    const empty = e.kind === "attachment" ? !files[e.key]?.length : isEmpty(value);
    if (empty) { if (e.required) out[e.key] = { code: "required" }; continue; }
    if (e.kind !== "number") continue;
    const n = Number(value);
    if (!Number.isFinite(n)) out[e.key] = { code: "number" };
    else if (e.min !== undefined && n < e.min) out[e.key] = { code: "min", limit: e.min };
    else if (e.max !== undefined && n > e.max) out[e.key] = { code: "max", limit: e.max };
  }
  return out;
}

/** Splits server errors into per-question messages (`answers.<key>`) and the rest. */
export function answerErrors(errors: { field: string; message: string }[]) {
  const byKey: Record<string, string> = {};
  const rest: string[] = [];
  for (const { field, message } of errors) {
    const key = /^answers\.(.+)$/.exec(field)?.[1];
    if (key && !(key in byKey)) byKey[key] = message; else rest.push(message);
  }
  return { byKey, rest };
}
