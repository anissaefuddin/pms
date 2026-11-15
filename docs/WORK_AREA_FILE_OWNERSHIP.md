# Work Area File Ownership

For parallel execution. Two work streams must not edit the same file. Shared files are listed with one owner; others coordinate through that owner.

| Area | Files | Owner stream | Shared with |
|---|---|---|---|
| Shell and layout | `web/src/routes/_app.tsx`, `web/src/components/sidebar.tsx`, `web/src/components/topbar.tsx`, `web/src/index.css` | Foundation (shell) | Every page stream reads, does not edit |
| Space header | `web/src/routes/_app/spaces.$spaceId.tsx` | Foundation (FND-02) | Space page stream after FND-02 |
| Project header and summary | `web/src/components/project-header.tsx`, `web/src/components/ui/kpi-card.tsx` | Foundation (FND-02, FND-03) | Project stream after FND-02 |
| Task page and folder page | `web/src/routes/_app/tasks.$itemId.tsx` | Project stream | - |
| View navigation and toolbar | `web/src/components/view-tabs.tsx`, `web/src/components/view-toolbar.tsx`, `web/src/components/view-columns.tsx`, `web/src/components/view-workspace.tsx` | View stream | Foundation for `view-workspace.tsx` layout slot only |
| Table | `web/src/components/item-table.tsx` | Table stream | - |
| Board | `web/src/components/item-board.tsx` | Board stream | - |
| Gantt | `web/src/components/item-gantt.tsx`, `web/src/lib/workspace-metrics.ts` (gantt helpers) | Gantt stream | Foundation (`gantt` color helper only) |
| Calendar | `web/src/components/item-calendar.tsx`, `web/src/lib/calendar-days.ts` | Calendar stream | - |
| Timelog | `web/src/components/timelog.tsx`, `web/src/routes/_app/timesheets.tsx` | Timelog stream | - |
| Files and chart | `web/src/components/project-files.tsx`, `web/src/components/file-*.tsx` | Files stream | - |
| Detail panel and activity | `web/src/components/item-detail.tsx`, `item-activity.tsx`, `item-comments.tsx` | Detail stream | `_app.tsx` Dialog wrapper owned by Foundation |
| Creation | `web/src/components/create-item-dialog.tsx`, `create-menu-dialog.tsx`, `web/src/lib/create-item-validate.ts` | Creation stream | - |
| Shared components | `web/src/components/ui/*`, `web/src/components/status-chip.tsx`, `web/src/components/avatar-stack.tsx` | Foundation (FND-06) | All streams |
| Tokens and tests | `web/src/index.css`, `web/scripts/check-no-hex.mjs`, `web/tests/work-area-*.spec.ts`, `web/tests/project-header-summary.spec.ts` | Foundation | All streams add tests in their own spec files |
| Server contracts | `api/openapi.yaml`, `server/internal/httpserver/*` | Not part of this epic unless a decision needs it | - |
| Docs | `docs/WORK_AREA_*.md` | Foundation | Streams append their own section |

Rules:

1. Each stream creates its own spec file under `web/tests/` and does not edit another stream's spec file.
2. Foundation is merged before page streams start, so the stream branches build on the shared layout.
3. Any change to `api/openapi.yaml` is a contract change and needs an owner decision first.
