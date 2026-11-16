# Work Area Implementation Plan

Scope: foundation (Phase 0 and Phase 1) only. The page-specific redesign (Table, Board, Gantt, Calendar, Timelog, Files, Analytics, detail panel, activity, creation) is a later phase and is not started here.

## 1. Done in this phase

| Item | Change | Files | Verified by |
|---|---|---|---|
| FND-03 | Project KPI cards collapsed into a compact strip with a Summary toggle | `web/src/components/project-header.tsx`, `web/src/i18n/en/views.json` | `web/tests/project-header-summary.spec.ts` |
| FND-01 / FND-04 | Shell overflow and sidebar height regression test at three sizes and two themes (no layout code change was needed) | `web/tests/work-area-layout.spec.ts` | Playwright |
| FND-05 | Gantt bar shades marked as intentional `allow-hex` so token lint passes | `web/src/components/item-gantt.tsx` | `npm run lint:tokens` |
| FND-02 | `WorkspaceHeader` (breadcrumb, icon, title with Star, meta, actions, summary slot) used by Space and Project | `web/src/components/workspace-header.tsx`, `spaces.$spaceId.tsx`, `project-header.tsx` | Full Playwright suite |
| FND-04 | `WorkspaceLayout` with one page padding (`p-4 sm:p-6`), used by Space and Task pages | `web/src/components/workspace-layout.tsx`, `tasks.$itemId.tsx` | Full Playwright suite; shell layout test |
| Audit | Phase 0 audit and this plan | `docs/WORK_AREA_*.md` | - |

## 2. Remaining foundation work (ordered)

| Order | Item | Depends on | Owner area (see file ownership) | Notes |
|---|---|---|---|---|
| 1 | Decide WA-02 and WA-03 (KPI semantics) | owner | - | Blocks final KPI labels. Do not change code before the decision |
| 2 | FND-02 for Folder: header with title, star, folder icon. Not applied yet because `ItemDetail` already shows the folder title; needs a decision on which title to keep | 1 | `tasks.$itemId.tsx`, `item-detail.tsx` | Open |
| 3 | FND-04 extension: slots for nav and toolbar inside `WorkspaceLayout` (currently header and content only) | 2 | `workspace-layout.tsx` | Open |
| 4 | FND-06 consolidate `StatusChip` and `AvatarStack` into `components/ui` | none | shared components only | Needs call-site count; WA-07 |
| 5 | FND-07 loading and error state components | none | `components/ui` | Replaces inline text |
| 6 | FND-08 creation contract audit | none | `create-item-dialog.tsx`, callers | Tests for each entry point |
| 7 | FND-09 saved view visibility vs sharing audit | none | server `views.go` plus tests | Server-side check |
| 8 | Measure typography and row heights (WA-15), dark theme for each view (WA-14), Board and Gantt horizontal scroll (WA-13) | 3 | tests and CSS | Needs browser screenshots |

## 3. Dependency map

```
WA-02, WA-03 (owner decision) ──> KPI labels in WorkspaceHeader/ProjectSummary
FND-02 WorkspaceHeader ──> FND-04 WorkspaceLayout ──> page-specific phase
FND-06 shared components ──> page-specific phase
FND-07 states, FND-08 create, FND-09 views: independent of each other
```

## 4. Risks

- Changing KPI semantics without the owner decision would change what users see as "in progress". Blocked by WA-02.
- The Gantt uses fixed hex shades for contrast. Replacing them with tokens must keep white-label contrast above 4.5:1 in both themes.
- Playwright mock suite was written for the current DOM. Header changes can break selectors; the full suite is run before any push.
