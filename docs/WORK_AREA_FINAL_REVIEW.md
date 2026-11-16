# Work Area Final Review (Phase 3)

Status: **CHANGES REQUIRED**

The epic is not ready for stakeholder review. Two P1/P2 defects were found and fixed locally, but most end-to-end scenarios were not run, and nothing is committed or deployed yet.

## 1. Scope of this review

- Environment: local only. A fresh database `pms_qa` on the local PostgreSQL 16 server, the Go API on `localhost:8080`, and Vite on `localhost:5173`. dev-lens was not touched.
- Data: `docs/qa/seed/ux_qa_seed.py` creates the isolated dataset (Space "ITA Internal Timeline", project "[UXQA] Lalang Platform — UX QA", five workstream folders, 28 tasks, 3 milestones, 1 subitem). Ids are in `docs/qa/seed/ux_qa_ledger.json`. Every title carries the `[UXQA]` marker.
- Users: one administrator only. Multi-user, permission, approval-by-another-person and second-user view scenarios are blocked by the environment.
- Browser: the built-in browser pane, one viewport (screenshots are 800×567 scaled). Visual checks at 1280, 1440 and 1920 were not run.

## 2. Defects

| ID | Priority | Page | Finding | Root cause | Status |
|---|---|---|---|---|---|
| DEF-P3-01 | P1 | Project → Calendar | The calendar showed no tasks inside folders. Only the five folders appeared, in the unscheduled tray. | The calendar read the project's direct children only. The Table view used the same query key, so the calendar reused the Table's folder-only result. | Fixed locally (`view-workspace.tsx`, `lib/items.ts` `listSubtree`). Verified in browser. Not committed. |
| DEF-P3-02 | P2 | Project header KPI | "Upcoming milestones 0". The seed has two milestones due after today (1 Dec and 14 Dec 2026). | The KPI read direct children only, so milestones inside folders were missed. | Fixed locally (`project-header.tsx`, one folder level). Verified: 0 → 2. Shortcut: folders nested deeper than one level are not walked. |
| DEF-P3-03 | Decision | Project → Calendar tray | Folders appear in the calendar's "Unscheduled" tray. They have no dates and add noise. | Existing behaviour, kept unchanged. | DECISION REQUIRED: hide folders from the calendar, or keep them. |

Previously known items, checked again this pass:
- **WA-02 (In progress vs New):** Project KPI "In progress 10" equals the count of tasks with status In Progress (10). New (12) is not counted. Consistent with the chosen definition. Label still reads "In progress". Owner decision recorded earlier.
- **WA-03 (direct vs subtree counts):** Work items 28 equals the analytics total (28), which covers the whole subtree and excludes milestones. The summary caption states this.

## 3. Scenario status

| Scenario | Status | Notes |
|---|---|---|
| E2E-01 Workspace navigation | PARTIAL | Gantt, Calendar (after fix), Timelog and Files checked. Board and Chart not run this pass. |
| E2E-02 Saved view lifecycle | NOT RUN | Needs a second user for the visibility check. |
| E2E-03 Contextual creation | NOT RUN (local) | Passed on dev-lens earlier; see `WORK_AREA_TEST_EVIDENCE.md`. Not re-run on this database. |
| E2E-04 Status sync | NOT RUN | |
| E2E-05 Schedule sync | NOT RUN | |
| E2E-06 Calendar cross-week | PARTIAL | Cross-week segments render (e.g. "Late review" across two weeks). Keyboard move and month transition not run locally. |
| E2E-07 Detail panel | NOT RUN | |
| E2E-08 Approval lifecycle | BLOCKED BY ENVIRONMENT | Needs approvers other than the administrator. |
| E2E-09 File lifecycle | NOT RUN | Files view empty state checked; no file uploaded. |
| E2E-10 Timelog lifecycle | PARTIAL | Empty state checked. No entries seeded; totals, filters and locked entries not run. |
| E2E-11 KPI consistency | PASS (for the checked items) | Work items 28, In progress 10, Upcoming milestones 2 (after fix). |
| E2E-12 Error and recovery | PASS (mocked only) | Covered by the mock suite (load failure, retry, upload retry, star failure). Live network interruption not run. |

## 4. Readiness gate

| Gate item | Result |
|---|---|
| No unresolved P0 or P1 | Met. DEF-P3-01 is fixed locally, not committed or deployed. |
| Required regression tests pass | Met locally. Mock suite 499 passed. Go suites from the previous CI work pass. |
| Existing capabilities preserved | Not fully verified. Most E2E scenarios not run. |
| No unauthorized data exposure | Not re-verified in this pass. |
| No incorrect hierarchy mutation | Not verified in this pass. |
| No critical scheduling regression | Not verified in this pass. |
| Cross-view state consistent | Partly: KPI counts match analytics. Calendar and Gantt checked on the same project. |
| No unexpected database changes | Seed used only the public API on the local database. |
| Known limitations documented | Yes (see section 5). |
| Before and after evidence for critical pages | No. Only screenshots from this pass; no before-fix screenshots for the calendar were kept. |

Result: **CHANGES REQUIRED**. Blockers: most scenarios not run, the decision on DEF-P3-03, and the fixes still uncommitted.

## 5. Known limitations

- One folder level only for the milestone count and the calendar subtree (shortcut in code).
- Calendar type and field filters are applied to the top level list only; nested items are fetched with the same filters, so a type filter that excludes folders hides nested items. Not verified.
- The seed has no assignees other than the administrator and no time entries, approvals or files.

## 6. Next steps

1. Decide DEF-P3-03.
2. Run E2E-02, E2E-04, E2E-05, E2E-07 and the remaining E2E-06 steps locally. Add multi-user seed for E2E-02 and E2E-08 if the owner approves a second local user.
3. Visual audit at 1280, 1440 and 1920, light and dark, compact and comfortable.
4. Commit the three fixes and the regression spec only after approval.
