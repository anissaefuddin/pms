# Work Area UX Audit (Phase 0)

Status: audit of branch `feat/m12c-ui-clean` at commit `4d5356d` (base `main` at `4a81055`). Read-only except for the foundation changes listed in `WORK_AREA_IMPLEMENTATION_PLAN.md`.

## 1. Repository and stack

- Branch `feat/m12c-ui-clean`, pushed to PR #16 (`origin` up to `4d5356d`). Untracked, not part of any commit: `docs/qa/evidence/`, `docs/qa/results/`.
- Backend: Go `net/http` (`server/internal/httpserver`), pgx and sqlc (`server/internal/dbgen`), goose migrations (`server/migrations`), OpenAPI contract (`api/openapi.yaml`, generated to `server/internal/apigen` and `web/src/api/schema.d.ts`).
- Frontend: React, TanStack Router (file routes in `web/src/routes`), TanStack Query, react-i18next (English only, `web/src/i18n/en`), Tailwind v4 with `@theme inline` tokens in `web/src/index.css`, base-ui Dialog and DropdownMenu, lucide-react icons.
- Architecture boundary: the web app only talks to `/api/v1/*`. Permissions come from the server as `my_capabilities` on each Space and item. Client code only hides or shows controls.

## 2. Routes that make up the work area

| Route | Screen | Layout |
|---|---|---|
| `/spaces/$spaceId` | Space (workspace of projects and folders) | `spaces.$spaceId.tsx`, `ViewWorkspace` with no parent |
| `/tasks/$itemId` | Project or Folder page; for a Task it shows the detail | `tasks.$itemId.tsx`, `ProjectHeader` for projects, `ViewWorkspace` with parent |
| `/tasks/$itemId?task=` | Task detail in a slide-right panel | `_app.tsx` Dialog with `ItemDetail` |
| `/spaces/$spaceId/forms`, `/automation` | Space admin | separate pages |
| `/stream`, `/inbox`, `/my-todo`, `/workload`, `/timesheets`, `/portfolio`, `/calendars`, `/reports`, `/dashboards`, `/reviews`, `/starred`, `/shared-with-me`, `/blueprints`, `/recycle-bin`, `/search` | Global pages | own routes |

Views are chosen with `?view=<uuid>`. Grouping, sort, filter, field filter, preset and type filter are URL search params validated in `_app.tsx`, so the state survives reload and can be shared.

## 3. Shared layout components (as they exist)

| Concept in epic | Existing component | Where | Note |
|---|---|---|---|
| Application shell | `AppShell` in `routes/_app.tsx`; `Sidebar`; `Topbar` | `components/sidebar.tsx`, `components/topbar.tsx` | Sidebar is `h-dvh` and sticky. Mobile uses `MobileSidebar`. |
| Workspace header | none shared. Space header is inline in `spaces.$spaceId.tsx`; project header is `ProjectHeader` | `components/project-header.tsx` | Two separate headers, no breadcrumb on Space, breadcrumb only on project. |
| Project summary / KPI | `ProjectHeader` KPI cards, `KpiCard` | `components/ui/kpi-card.tsx` | Now a compact strip with a Summary toggle (FND-03). |
| View navigation | `ViewTabs` | `components/view-tabs.tsx` | Role "radio" tabs. |
| View toolbar | `ViewToolbar`, `ColumnPicker` | `components/view-toolbar.tsx`, `components/view-columns.tsx` | Toolbar is not the same as `ui/toolbar.tsx`. |
| Views | `ItemTable`, `ItemBoard`, `ItemGantt` (lazy), `ItemCalendar` (lazy), `Timelog`, `ProjectFiles`, `ProjectChart` | `components/*.tsx` | All orchestrated by `ViewWorkspace`. |
| Detail panel | `ItemDetail` inside a Dialog | `components/item-detail.tsx`, `_app.tsx` | Dialog is `inset-y-0 right-0`, `overflow-y-auto`. |
| Create | `CreateItemDialog`, `CreateMenuDialog`, `pms:create-item` event | `components/create-item-dialog.tsx` | Location (space and parent) is passed in. |
| Empty / loading / error | `EmptyState`, inline `role="status"` and `role="alert"` text | `components/ui/empty-state.tsx` | No shared LoadingState or ErrorState component. |
| Confirm | `ConfirmDialog` | `components/confirm-dialog.tsx` | |
| Status | `StatusChip` in two places | `components/status-chip.tsx` and `components/ui/status-chip.tsx` | Duplicate, see gap WA-07. |
| Avatars | `AvatarStack` in two places | `components/avatar-stack.tsx` and `components/ui/avatar-stack.tsx` | Duplicate, see gap WA-07. |
| Type icon | `TypeIcon`, `TypeBadge` | `components/type-icon.tsx`, `components/type-badge.tsx` | |
| Toasts | `sonner` toaster | `components/ui/toaster.tsx` | |

## 4. View and preference persistence

- Saved views are server records (`/api/v1/spaces/{id}/views`, `/api/v1/items/{id}/views`, `/api/v1/views/{id}`, duplicate endpoint). Visibility is a field on the view and is not the same as sharing of the parent.
- The active view is the `view` URL param. If none is given, the default view (`is_default`) or the first one is used. Nothing about the active view is stored in `localStorage`.
- Browser `localStorage` is used only for: sidebar collapse, sidebar width and open Spaces (`lib/sidebar-prefs.ts`), last Space (`pms:last-space:<user>`), and timesheet rows.
- Sidebar pins and hidden shortcuts are server profile fields (`sidebar_pinned`, `sidebar_hidden`).

## 5. Permissions

- Space: `can(space, "create" | "edit" | "share" | "delete")` from `my_capabilities`. Admin-only lock is also checked with `me.user_type`.
- Item: `container.my_capabilities.includes("create" | "edit" | "share")`.
- Client gating is only a UI control. The server enforces the same rules (existing integration tests under `server/internal/httpserver`). This audit did not re-verify each server check.

## 6. Test coverage and status

- Unit: `npm test` runs `node --test src/lib/*.test.ts`: 52 of 52 pass.
- Lint: `npm run lint` passes. Typecheck passes.
- Token rule: `npm run lint:tokens` (no hex colors outside tokens). It flagged three intentional Gantt bar shades. They are marked `allow-hex` (see gap WA-08).
- Browser: Playwright mock suite in `web/tests` (about 100 spec files). Run status is in the final report.
- Go integration tests need `PMS_TEST_DATABASE_URL` and are skipped locally without it. They run in CI.

## 7. Findings

Classification and priority are in `WORK_AREA_GAP_MATRIX.md`. Summary:

1. The project header used four large KPI cards that took most of the first screen above the view. Changed to a compact strip with a Summary toggle.
2. The "In progress" KPI counts every item whose status group is `active`. The default workflow puts both `New` and `In Progress` in group `active` (`server/internal/bootstrap/bootstrap.go`). So the KPI shows 6 while the Board "In Progress" column shows 0. This is a definition question, not a bug fix. Recorded as DECISION REQUIRED.
3. Work item counts use direct children only (`/items/{id}/children`). Folder children and deeper descendants are not counted. Recorded as DECISION REQUIRED.
4. The sidebar and document do not overflow sideways at 1280, 1440 and 1920 widths, in light and dark. Now covered by a test.
5. Space page and project page use different page padding (`p-4 sm:p-8` against `p-6`) and different header structure.
6. Duplicate components exist for status chip and avatar stack.

## 8. Not checked in this phase

- Visual comparison with the approved mockup and the screenshots (`LENS_UI_UX_Improvement.html`, screenshots). Not re-reviewed here.
- Font sizes and row heights against the recommended scale (body 13–14px, compact row 32px). Not measured.
- Dark theme on every view. Only the shell was measured.
- Board and Gantt horizontal scroll isolation. Not measured.
- Context menus, keyboard navigation and inline creation contract (FND-07, FND-08). Not audited in depth.
- Saved view visibility against public sharing (FND-09). Not audited in depth.
