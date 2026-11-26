# Work Area Contract Audit (FND-08 creation, FND-09 shared state)

## FND-08 Creation context

Entry points found in code (`web/src`):

| Entry point | Where | Location passed to `CreateItemDialog` | Test |
|---|---|---|---|
| Global "Create" menu | `routes/_app.tsx` (topbar) | `spaceId` from the route when one is open, otherwise the user's personal Space | `dr03-create-closes.spec.ts` |
| Sidebar "+" on a Space | `components/sidebar.tsx` event `pms:create-item` with `spaceId` | Space root | `uip4-sidebar.spec.ts` ("Create in Marketing") |
| Space page "New item" | `routes/_app/spaces.$spaceId.tsx` | Space root (`parent_item_id` null) | `creation-context.spec.ts` (new) |
| Project "Add task" | `routes/_app/tasks.$itemId.tsx` (`taskOnly`) | Space of the project, `parent_item_id` = project id | `creation-context.spec.ts` (new) |
| Folder or project view "New" | `components/view-workspace.tsx` (`taskOnly`, `initialStatus`) | Space plus `parent_item_id` = container | not yet |
| Subitem from detail "Add subitem" | `components/item-detail.tsx` `childLocation` | Space plus `parent_item_id` = item | not yet |
| Blueprint "use" | `routes/_app/blueprints.tsx` | Chosen Space, root or parent from the blueprint | not yet |

Findings:

1. **Parent is resolved from the container, not from the page.** Confirmed for the Space and Project entry points by tests.
2. **Folder and Project views create tasks only** (`taskOnly` in `view-workspace.tsx`). The business rule for which types may be children of a folder is not in this audit. DECISION REQUIRED: confirm whether a folder may hold a project or folder child, and whether the subitem path (`item-detail.tsx`, no `taskOnly`) must match the view path.
3. **Global Create with a Space open uses that Space.** Without a Space open, the personal Space is used. This is intended, but not tested.
4. **Permission check** uses `my_capabilities` on the container for each entry point; the server enforces the same rule.

## FND-09 Shared state

| Item | Where it lives | Finding |
|---|---|---|
| Active view | URL `?view=` | Survives reload and navigation. Not in localStorage. |
| Saved views | Server `/api/v1/.../views` | Fields: `visibility` (`public` or `personal`), `position`, `is_default`. |
| View visibility vs sharing | `visibility` on the view; sharing of the parent is a separate ACL | Visibility of a view does not change who can see the parent. The UI label "Public" means "anyone with access to the Space or project", not an anonymous link. Verified in code; a server authorization test for this was not found in this audit (NOT CHECKED). |
| Filters, sort, group | URL search params | Survive reload. |
| Selected item | URL `?task=` | Survives reload. |
| Expanded hierarchy | Not persisted (Table and Gantt collapse state is in component state) | Resets on navigation. Decision needed: persist or not. |
| Selected Space | `localStorage` `pms:last-space:<user>` | Per user, cleared by nothing on sign-out. Minor: sign-out does not clear it. |
| Detail panel | URL `?task=` | Closing removes the param; view state is untouched. |
| Sidebar pins and hidden | Profile fields `sidebar_pinned`, `sidebar_hidden` | Server-side, per user. |

Decisions needed (recorded, not changed here):

- D-FND09-1: persist expanded hierarchy per view (server or local)?
- D-FND09-2: clear the last-Space key on sign-out?
