import { ALL, marketing, mockSpaces } from "./spaces-mock";
import { problem } from "./mock-api";

export const ganttId = "55555555-5555-4555-8555-555555555555";
export const path = `/spaces/${marketing.id}`;
export const loc = (parent?: string) => [{ space_id: marketing.id, parent_item_id: parent ?? null }];
export const base = { version: 1, my_capabilities: ALL, assignees: [] };
export const project = { ...base, id: "a0000000-0000-4000-8000-000000000001", seq: 1, type: "project", title: "Launch", start_date: "2026-11-02", due_date: "2026-11-13", child_count: 3, locations: loc(), progress_percent: 40 };
export const kids = ["Design", "Build", "Ship"].map((title, i) => ({ ...base, id: `b0000000-0000-4000-8000-00000000000${i + 1}`, seq: i + 2, type: "task", title,
  start_date: `2026-11-0${i * 2 + 2}`, due_date: `2026-11-0${i * 2 + 3}`, progress_percent: 25 * i, child_count: 0, locations: loc(project.id) }));
export const milestone = { ...base, id: "c0000000-0000-4000-8000-000000000001", seq: 9, type: "task", title: "Go live", is_milestone: true, start_date: "2026-11-13", due_date: "2026-11-13", child_count: 0, locations: loc(project.id) };

export function mockGantt(page: import("@playwright/test").Page, opts: { root?: object[]; children?: object[]; space?: string[]; conflict?: boolean; deps?: any[]; fields?: any[]; config?: object; extra?: Record<string, any> } = {}) {
  let views = [{ id: "44444444-4444-4444-8444-444444444441", type: "table", name: "Table", visibility: "public", position: 0, is_default: true, config: {} },
    { id: ganttId, type: "gantt", name: "Gantt", visibility: "public", position: 1, is_default: false, config: opts.config ?? {} }];
  const deps: any[] = [...(opts.deps ?? [])];
  const store = new Map<string, any>([project, ...kids, milestone].map((item) => [item.id, { ...item }]));
  return mockSpaces(page, { spaces: [{ ...marketing, my_capabilities: opts.space ?? ALL }], extra: {
    [`GET ${path}/views`]: () => ({ body: { items: views } }),
    [`GET ${path}/items`]: () => ({ body: { items: opts.root ?? [store.get(project.id)] } }),
    [`GET /items/${project.id}/children`]: () => ({ body: { items: opts.children ?? [...kids.map((k) => store.get(k.id)), store.get(milestone.id)] } }),
    "GET /workflows": { body: { items: [] } },
    [`GET /spaces/${marketing.id}/custom-fields`]: { body: { items: opts.fields ?? [] } },
    [`GET /spaces/${marketing.id}/fields`]: { body: { items: opts.fields ?? [] } },
    "POST /dependencies/query": () => ({ body: { items: deps } }),
    ...Object.fromEntries([...store.keys()].map((id) => [`POST /items/${id}/dependencies`, (body: any) => {
      if (body.predecessor_id === milestone.id) return problem(422, "This link would create a cycle", "validation_failed");
      const dep = { id: `e0000000-0000-4000-8000-00000000000${deps.length + 1}`, successor_id: id, lag: 0, ...body };
      deps.push(dep);
      const next = store.get(id); // the server pushes the successor after its predecessor
      store.set(id, { ...next, start_date: "2026-11-09", due_date: "2026-11-10", version: next.version + 1 });
      return { status: 201, body: dep };
    }])),
    "PATCH /dependencies/e0000000-0000-4000-8000-000000000001": (body: any) => { Object.assign(deps[0], body); return { body: deps[0] }; },
    "DELETE /dependencies/e0000000-0000-4000-8000-000000000001": () => { deps.splice(0, 1); return { status: 204 }; },
    ...Object.fromEntries([...store.keys()].flatMap((id) => [
      [`GET /items/${id}/children`, { body: { items: [] } }],
      [`PATCH /items/${id}`, (body: any) => { if (opts.conflict) return problem(409, "Version conflict"); const item = { ...store.get(id), ...body, version: store.get(id).version + 1 }; store.set(id, item); return { body: item }; }],
    ]).filter(([key]) => key !== `GET /items/${project.id}/children`)),
    ...(opts.extra ?? {}),
  } });
}



export const bar = (page: import("@playwright/test").Page, id: string) => page.locator(`.wx-bar[data-id=":${id}"]`);

export const design = kids[0].id;

/** The Gantt scale is chosen from the floating menu at the top right of the chart. */
export async function pickScale(page: import("@playwright/test").Page, name: string) {
  await page.getByRole("button", { name: "Time scale" }).click();
  await page.getByRole("menuitem", { name, exact: true }).click();
}
