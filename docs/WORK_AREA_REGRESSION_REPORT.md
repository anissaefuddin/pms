# Work Area Regression Report (Phase 3)

Date: 16 Nov 2026. Environment: local (macOS, Go and PostgreSQL 16 on localhost). No dev-lens calls in this pass.

## 1. Automated suites

| Suite | Command | Result | Notes |
|---|---|---|---|
| Web lint | `npm run lint` | PASS | |
| Web typecheck | `npm run typecheck` | PASS | |
| Token lint | `npm run lint:tokens` | PASS | |
| Web unit tests | `npm test` | PASS, 85/85 | |
| Mock e2e (Playwright) | `npx playwright test --workers=2` | PASS, 499 passed, 43 skipped | Includes 2 new regression tests in `work-area-p3-qa.spec.ts`. The 43 skipped are the live tests that need a Go fixture. |
| Targeted mock run | calendar, project header, view navigation, gantt, views | PASS, 23 passed | Run before the regression spec was added. |
| Go vet | `go vet ./server/...` | PASS | |
| Go unit and integration (`./server/...`) | `PMS_TEST_DATABASE_URL=… go test -count=1 ./server/...` | PASS | Includes the authorization sweep. |
| Parity E2E groups | `scripts/parity-e2e.sh 01-04 / 05-08 / 09-12` | PASS | |
| Browser Go tests (CI list) | `PMS_TEST_BROWSER=1 go test -run …` | PASS, 16/16 | |

## 2. Regression tests added

| Test | Covers | Result |
|---|---|---|
| QA-P3-01 `a project calendar shows tasks that sit inside its folders` | DEF-P3-01 | PASS |
| QA-P3-02 `upcoming milestones count milestones inside folders` | DEF-P3-02 | PASS |

Both tests were written from the defects seen in the live app. They use mocked API responses that mirror the seeded structure.

## 3. Live browser checks on the local seed

| Check | Before fix | After fix |
|---|---|---|
| Project calendar shows tasks in folders | FAIL (only folders listed) | PASS (bars, "+N more", tray) |
| Upcoming milestones KPI | FAIL (0; expected 2) | PASS (2) |
| Work items KPI equals analytics total | PASS (28 = 28) | PASS |
| In progress KPI equals In Progress count | PASS (10 = 10) | PASS |
| Gantt tree shows statuses and bars | PASS | PASS (not changed) |
| Timelog empty state | PASS | PASS (not changed) |
| Files empty state | PASS | PASS (not changed) |

## 4. Performance (measured)

Only request counts were measured, from the browser network log. No timing comparison was made.

- Project calendar load, after fix: one `children` request for the project, then one per folder (5 folders), so 6 requests for the item list. Before the fix there was 1.
- Project header with milestones, after fix: 5 additional `children` requests, which share the cache with the Table view's folder children.

No frame timing, render counts or memory measurements were taken. Performance claims are not made.

## 5. Not run

- Go integration suites that need the browser fixture and were not part of the CI list.
- Visual regression at three resolutions and dark/light/density combinations (see `WORK_AREA_VISUAL_AUDIT.md`).
- Live network interruption, conflict response and multi-user permission scenarios.

## 6. Second pass (same day)

| Check | Result |
|---|---|
| Web lint, typecheck, token lint | PASS |
| Unit tests | PASS, 85/85 |
| Mock e2e (`npx playwright test --workers=2`) | PASS, 500 passed, 43 skipped. Two old board tests needed an empty `children` mock for a card with subitems (`uip-lane2.spec.ts`, `uip2-b.spec.ts`). Their assertions are unchanged. |
| New regression tests | QA-P3-01, QA-P3-02, QA-P3-03 (board shows tasks inside folders; no folder cards). All PASS. |
| Live: project Board | Before: 0 tasks, folder cards only. After: New 15, In Progress 9, Completed 7. |
| Live: project Calendar with subitems | Before: subitem missing. After: subitem listed in the tray. |
| Live: invalid schedule (due before start) | 422, no change, version unchanged. |
| Live: personal view isolation | Admin and reader get 404 on the editor's personal view (GET and PATCH). Public view visible to all three. |
| Live: view hide persistence | Hidden through preferences API, visible as hidden after reload, restored. |

## 7. Performance at 500 items (measured, local)

Fixture: project "[UXQA-PERF] 500 items" on the local `pms_qa` database. Five folders with 495 tasks (about 99 per folder), plus five folders, so 500 items. Only the 50-item and 200-item fixtures were not created in this pass; 500 is the measured size.

Measured with Playwright (Chromium, 1440×900, light), requests counted from the network log. Times are to network idle on one run, not averaged.

| View | Children requests before shared cache | Children requests after | Time to network idle (after) |
|---|---|---|---|
| Table | 11 | 11 | 947 ms |
| Board | 22 (7+5 page-1 + page-2 across two walks) | 12 | 736 ms |
| Calendar | 11 | 11 | 804 ms |
| Gantt | 11 | 11 | 796 ms |

Notes:
- The before-numbers for Board include a second walk from the project header milestone KPI. Fix: one cached fetch per folder (`subtreeChildren` in `lib/items.ts`), shared by the header and the flat views.
- "Page 2+" requests are real pagination (about 99 items per folder, 50 per page). They are not duplicates.
- Filter response time: not measured reliably in this pass. The table filter test found 0 `tbody` rows, which points to the selector, not the filter. NOT RUN.
- View-switch time and mutation behaviour: NOT RUN.
- 50- and 200-item measurements: NOT RUN.
- No timing comparison against the pre-change build was made, so no performance improvement is claimed.
