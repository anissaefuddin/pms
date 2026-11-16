# Work Area Final Review (Phase 3)

Status: **CHANGES REQUIRED**

The epic is not ready for stakeholder review. Four functional defects were found and fixed locally. Two decisions are open. Several end-to-end scenarios are still not run, and the visual audit is incomplete.

## 1. Scope of this review

- Environment: local only. Database `pms_qa` on the local PostgreSQL 16 server, Go API on `localhost:8080`, Vite on `localhost:5173`. dev-lens was not used.
- Data: `docs/qa/seed/ux_qa_seed.py` creates the isolated dataset: Space "ITA Internal Timeline", project "[UXQA] Lalang Platform — UX QA", five workstream folders, 28 tasks, 3 milestones, 1 subitem. Ids are in `docs/qa/seed/ux_qa_ledger.json`. Every title carries `[UXQA]`.
- Users: the administrator, plus two invited regular users (`qa-editor` with editor share, `qa-reader` with read share) on the project and Space. Their passwords are local-only.
- Browser: built-in pane. Screenshots are scaled to 800×567. Visual checks at 1280, 1440 and 1920 were not run.

## 2. Defects

| ID | Priority | Page | Finding | Root cause | Status |
|---|---|---|---|---|---|
| DEF-P3-01 | P1 | Project → Calendar | Tasks inside folders did not appear. Only folders were listed. | The view read direct children only. The calendar shared a query key with the Table. | Fixed locally. Test QA-P3-01 passes. |
| DEF-P3-02 | P2 | Project KPI | "Upcoming milestones 0" (expected 2). | Read direct children only. | Fixed locally. Test QA-P3-02 passes. Shortcut: one folder level; deeper folders are walked by the calendar and board but not by this KPI. |
| DEF-P3-03 | P1 | Project → Board | Board showed five folder cards and no tasks. "In progress 0" and "Completed 0" while the KPI showed 9 and 7. | Same root cause as DEF-P3-01. Board read direct children only. | Fixed locally. Board now shows executable items from the whole subtree. Board In progress 9, Completed 7. |
| DEF-P3-04 | P2 | Project → Calendar, Board | Subitems under a task (e.g. "Datasheet checklist") did not appear. | The subtree walk expanded only folders and projects. | Fixed locally. The walk now expands any item with `child_count > 0`. Board New is 15, matching the API count. |
| DEF-P3-05 | Decision | Project → Calendar | Folders appear in the calendar's "Unscheduled" tray. | Existing behaviour, unchanged. | DECISION REQUIRED. |
| DEF-P3-06 | Decision | Project → Board | Board shows the three milestones as cards (New shows 15). The KPI excludes milestones (New is 12 tasks). | Board includes milestones; KPI excludes them. | DECISION REQUIRED: hide milestones on Board, or keep them and relabel the KPI. |

Previously known items, checked again:
- **WA-02 (In progress vs New):** KPI In progress 9 equals the Board's In Progress column and the Chart's bar. New (12 tasks) is not in the "In progress" group. Consistent.
- **WA-03 (direct vs subtree counts):** Work items 28 equals the analytics total (subtree, milestones excluded). Caption states the rule.

## 3. Scenario status

| Scenario | Status | Notes |
|---|---|---|
| E2E-01 Workspace navigation | PARTIAL | Table, Board, Gantt, Calendar, Timelog, Files, Chart checked. Layout shift and keyboard view switching not checked. |
| E2E-02 Saved view lifecycle | PARTIAL | Via API: personal view hidden from admin and reader (reader and admin get 404 on GET and PATCH); public view visible to both. Hide and show persist after reload (UI). Rename and All views menu not run (the All views menu did not open in the pane). |
| E2E-03 Contextual creation | NOT RUN (local) | Passed on dev-lens earlier; see `WORK_AREA_TEST_EVIDENCE.md`. |
| E2E-04 Status sync | PASS (API-driven change) | Workshop changed to Completed through the items API. Board, Chart, KPI and Calendar colour updated. Table and detail not re-checked. UI drag not run. |
| E2E-05 Schedule sync | PARTIAL | Valid move (18 Nov – 2 Dec) saved. Calendar bar and detail panel show the same range. Invalid move (due before start) returns 422 and the item is unchanged (version stays 2). Gantt drag not run. |
| E2E-06 Calendar cross-week | PARTIAL | Cross-week segments with continuation markers render. "+N more" overflow visible. Keyboard move and month transition not run locally. |
| E2E-07 Detail panel | PARTIAL | Opened from Calendar with `?task=`. Panel shows the same dates. Closing keeps the URL view. Edit inside panel and filter/scroll checks not run. |
| E2E-08 Approvals | BLOCKED BY ENVIRONMENT | Needs approvers other than the admin. Two extra users exist but approval flows not run. |
| E2E-09 Files | NOT RUN | Files empty state checked; no upload. |
| E2E-10 Timelog | PARTIAL | Empty state checked. No entries seeded. |
| E2E-11 KPI consistency | PASS (checked items) | Work items 28 = analytics 28. In progress 9 = Board 9 = Chart 9. Upcoming milestones 2. Board New 15 includes milestones (DEF-P3-06). |
| E2E-12 Error and recovery | PASS (mocked only) | Mock suite covers load failure, retry, upload retry, star failure. Live network interruption not run. |

## 4. Readiness gate

| Gate item | Result |
|---|---|
| No unresolved P0 or P1 | Met for the four P1/P2 functional defects (fixed locally, uncommitted). |
| Required regression tests pass | Met locally. Mock suite 500 passed. Lint, typecheck, token lint and 85 unit tests pass. |
| Existing capabilities preserved | Partly verified. Two old board tests needed a mock for subitems (see section 5). |
| No unauthorized data exposure | Verified for personal views (admin and reader cannot read or change the editor's personal view). |
| No incorrect hierarchy mutation | Not verified beyond the invalid-date check. |
| No critical scheduling regression | Invalid schedule rejected with no change. Valid schedule shows consistently in calendar, detail and API. |
| Cross-view state consistent | Yes for status (Board, Chart, Calendar, KPI). Milestones counting differs by design (DEF-P3-06). |
| No unexpected database changes | The seed and tests used only the public API, plus the invitation activation on the local database. |
| Known limitations documented | Yes (section 5). |
| Before and after evidence for critical pages | Partial. After-fix screenshots only. |

Result: **CHANGES REQUIRED**. Blockers: decisions DEF-P3-05 and DEF-P3-06, scenarios not run, and the fixes are uncommitted.

## 5. Known limitations and changes to tests

- The subtree walk makes one `children` request per container. A project with five folders and one subitem makes six requests to load the calendar or board. No timing was measured.
- Folder cards are hidden from the Board. Owner decision pending (DEF-P3-05 covers the Calendar equivalent).
- Two old mock tests (`uip-lane2.spec.ts`, `uip2-b.spec.ts`) were updated with an empty `children` mock for a card that has `child_count: 2`, because the board now walks subitems. They were not weakened: their assertions are unchanged.
- The milestone KPI still reads only one folder level down (shortcut). The calendar and board walk any depth.
- The seed has only the administrator as assignee.

## 6. Next steps

1. Decide DEF-P3-05 (folders in calendar tray) and DEF-P3-06 (milestones on the Board).
2. Run E2E-02 rename and the All views menu, E2E-07 edit and filter checks, E2E-01 Board/Chart layout checks, and the remaining E2E-06 keyboard move.
3. Visual audit at 1280, 1440 and 1920, light and dark, compact and comfortable.
4. Commit the fixes, the regression spec and the updated docs only after approval.

## 7. Phase 3.1 status (this pass)

Execution method: sequential, single operator (no parallel agents were run). Local environment only: `pms_qa`, Go on localhost:8080, Vite on localhost:5173.

| ID | Phase | Owner | Priority | Decision status | Implementation | Verification | Evidence | Remaining limitation |
|---|---|---|---|---|---|---|---|---|
| DEF-P3-05 | 3 | A | P1 | APPROVED (DECISIONS §8) | DONE (uncommitted) | PASS (mock QA-P3-04; live calendar check) | `item-calendar`, `view-workspace` | none |
| DEF-P3-06 | 3 | A | P2 | APPROVED (DECISIONS §8) | DONE (uncommitted): milestone label, column split, chart scope label | PASS (mock QA-P3-05; live board check) | `item-board`, `project-files` | Chart still shows status counts only, not a task/milestone split |
| WA-03 / WA-D03 | 2 / 2.1 | A | P2 | APPROVED (DECISIONS §8) | DONE (server analytics, unchanged) | PASS (live: Work items 28 = analytics) | analytics query | none |
| WA-02 / WA-D02 | 2 | A | P2 | APPROVED (DECISIONS §8) | DONE (group-based, unchanged) | PASS (live: In progress 9 = Board = Chart) | header, board | none |
| A2 Upcoming milestones | 3.1 | A | P1 | — | DONE (uncommitted): full subtree walk | PASS (mock QA-P3-06, three folder levels) | `project-header.tsx` | client walk, not server-side; one request per container |
| A3 traversal audit | 3.1 | A | P1 | — | PARTIAL: subtree walk now covers subitems; filter-during-traversal not audited | NOT RUN for filters, pagination, duplicates | `lib/items.ts` | filters apply to the top list only |
| A5 performance (50/200/500) | 3.1 | A | P2 | — | NOT RUN | NOT RUN | — | — |
| FND-02 Folder header | 1 | B | P2 | — | NOT VERIFIED in this pass (earlier decision register marks DONE) | NOT RUN | DECISIONS | — |
| FND-06 duplicate components | 1 | B | P2 | — | OPEN | NOT RUN | — | — |
| FND-07 states | 1 | B | P2 | — | NOT VERIFIED | NOT RUN | — | — |
| FND-08 / WA-D04 creation policy | 1/2 | B | P2 | — | NOT VERIFIED | NOT RUN | — | — |
| FND-09 / WA-D05/06 session lifecycle | 1/2 | B | P2 | — | NOT VERIFIED | NOT RUN | — | — |
| C1 drag tab reorder (WA-D07) | 2.1 | C | P1 | — | NOT VERIFIED in this pass | NOT RUN | — | — |
| C2 Table and Board interactions | 2 | C | P1 | — | NOT VERIFIED | NOT RUN | — | — |
| C3 Gantt and Calendar | 2 | C | P1 | — | NOT VERIFIED | NOT RUN | — | — |
| C4 ItemDetail | 2 | C | P1 | — | NOT VERIFIED | NOT RUN | — | — |
| D1–D12 live E2E | 3.1 | D | P1 | — | see section 3 above | PARTIAL | `WORK_AREA_TEST_EVIDENCE.md` | multi-user and approvals not run with real users |
| E visual audit and a11y | 3.1 | E | P2 | — | NOT RUN | NOT RUN | — | — |

Automated checks on the current working tree (not committed): lint PASS, token lint PASS, typecheck PASS, unit 85/85 PASS, production build PASS, mock Playwright 503 passed, Go `gofmt`/`go vet`/`go test ./server/...` PASS. Browser Go list: 15 pass; `TestBrowserCollaboration` failed once in `pilot-live.spec.ts` (mention notification not visible within 3 s) and passed on two reruns, so it is intermittent.

Remote: last reviewed `bffe6b9` (pushed). Local uncommitted changes are listed in `git status`.

Overall status: **CHANGES REQUIRED** (Wave 1 agents B, C, D (except as noted), and E not executed; A5 not measured; DEF-P3-06 chart split not done).

### CI on bffe6b9 (failed)

- Job `test`, step "Generated code is up to date" failed. Cause: `server/internal/dbgen/models.go` was stale. The `ApprovalEvent` struct lacked `attachment_id` and `attachment_version` from migration 00325 (commit 3867409). The other checks passed (`authorization-matrix`, `parity-e2e`).
- Fix applied locally, not committed: regenerated with `sqlc generate` (v1.31.1). Only `models.go` changed. `gofmt`, `go vet`, and approval and file-review tests pass.
- Lesson recorded for the next pass: run the generated-code check before pushing.

### Phase 3.1 update (this pass, uncommitted)

- A5 performance at 500 items: measured. Board children requests reduced from 22 to 12 by a shared cached child fetch. Filter and mutation timing NOT RUN. 50 and 200 item fixtures NOT RUN.
- E visual audit: automated capture of 7 views at three viewports, light and dark (42 screenshots). No horizontal overflow. Three findings recorded (VIS-01 P3 open). Density, Stream, All views, ItemDetail, dialogs, and approval card NOT RUN.
- Overall status unchanged: **CHANGES REQUIRED**.

### Wave 1 closure log (this pass)

| ID | Status | Evidence |
|---|---|---|
| FND-06 | DONE | `git ls-files`: one StatusChip (`components/ui/status-chip.tsx`, moved from `components/status-chip.tsx`), one AvatarStack (`components/ui/avatar-stack.tsx`). Four importers updated. Lint, typecheck, and 508 mock tests pass. |
| FND-07, FND-08, FND-09, B2–B6 | OPEN | Not verified in this pass. |
| C1–C4 | OPEN | Not verified in this pass. |
| D1–D12 (E2E) | PARTIAL, see section 3 | |
| Agent A traversal and performance | PARTIAL | See regression report section 7. |
| Agent E visual and accessibility | PARTIAL | See visual audit section 4. |
