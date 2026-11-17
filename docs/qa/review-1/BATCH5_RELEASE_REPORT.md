# Batch 5 release report: Custom item types, requests, and Space settings

Date: 17 Nov 2026. Branch `feat/m12c-ui-clean` (PR #18). Commits in scope: `0665875`..`2648c70`. Deployed to dev-lens: `pms:2648c70`.

## 1. Matrix

| Area | Check | Result | Evidence |
|------|-------|--------|----------|
| Requests (RF-08) | Requester without access gets no Item, comments, or attachments; status change reaches the inbox with no Item or Space link | PASS (Go) | `TestMyRequests`, `TestFormSpaceMetadataFollowsSpaceAccess` |
| Requests | Email for a requester status change follows `task_changes` preference | NOT RUN | Code path changed (`mail/notification.go`); no test; not checked on dev-lens |
| Quick Access (CIT-D05) | Menu, More dialog, Settings toggle, Space menu gating | PASS (Go + Playwright) | `TestItemTypeQuickAccessValidationAndMenu`, m8 tests |
| Usage (CIT-D06) | Global counts admin-only; non-admin gets 403 | PASS (Go) | `item_type_usage_integration_test.go` |
| Space item types | Space editors manage their Space's types; Workspace types read-only; create sends `space_id` | PASS (Playwright) | m8 "a Space's item types" |
| Space item types | Same on dev-lens with real data | NOT RUN | Needs a test login on dev-lens |
| Layout builder | Up/down buttons and drag within a section | PASS (Playwright) | m8 "dragging a block" |
| Layout builder | Drag between sections; keyboard-only drag | NOT RUN | Not built (drag is within one section only) |
| Workspace custom fields | Admin-only create/edit; Space editors cannot | PASS (Go) | `workspace_custom_fields_integration_test.go` |
| OpenAPI | `/custom-fields` documented | PASS (spec and generated TS updated) | `api/openapi.yaml` |
| Accessibility | Registry, type form, Space item types | PASS (axe, 0 violations) | m8 "pass axe" |
| Accessibility | Layout builder preview, keyboard drag | PARTIAL | Axe on the form dialog only |
| Visual | Registry and Space item types at 1280/1440/1920, light and dark | CAPTURED, not compared | `docs/qa/review-1/batch5/` (12 PNG) |
| Visual | Pixel comparison against a baseline | NOT RUN | No baseline exists |
| Performance | Load or timing tests | NOT RUN | No performance harness in the repo |
| Browser collaboration | `TestBrowserCollaboration` | FLAKY | Passed 10 of 11 local runs; failure output not captured; CI passed on the last 3 heads |
| Full Playwright | `npx playwright test` | 538 passed, 43 skipped, 1 failed | The one failure, `ownership.spec.ts:51`, passed 3 of 3 alone |
| Full Go | `go test ./...` | PASS | All packages ok |
| Lint and types | eslint, tsc, token check, gofmt, go vet | PASS | |
| CI | PR #18 head `2648c70` | `test`, `authorization-matrix`, `parity-e2e` PASS | run 44047559481 |
| Deploy | dev-lens on `pms:2648c70` | PASS (health 200, migrations 00328–00330 applied, no ERROR logs) | Backup `20261117T094440Z` |
| Live checks on dev-lens | Email, Space item types, request status in inbox | NOT RUN | |

## 2. Files changed in this batch

- Server: `items/item_types.go`, `items/custom_fields.go`, `items/type_fields.go`, `items/notifications.go`, `notifications/record.go`, `httpserver/notifications.go`, `httpserver/requests_integration_test.go`, `mail/notification.go`, `httpserver/create_menu.go`, `httpserver/form_submit.go`, `policy/item.go`.
- Migrations: `00327` (Space item types), `00328` (Workspace custom fields), `00329` (Quick Access), `00330` (notifications with no target).
- API: `api/openapi.yaml`, regenerated `apigen` and `web/src/api/schema.d.ts`.
- Web: `routes/_app.tsx`, `routes/_app/settings/item-types.tsx`, `routes/_app/spaces_.$spaceId.item-types.tsx` (new), `routes/_app/spaces.$spaceId.tsx`, `components/topbar.tsx`, `components/create-item-dialog.tsx`, `i18n/{en,id}/settings.json`, `routeTree.gen.ts`.
- Tests: `tests/m8.spec.ts`, `tests/custom-fields.spec.ts`, `tests/batch5-visual.spec.ts` (new).
- Docs: `docs/RELEASE_CLEANUP.md` (new), this report.

## 3. Security findings

- **Requester access.** Submitting a form no longer grants access to the created Item. Verified by tests (404 for requester, viewer, and other users).
- **Notifications with no target.** Rows with no Item or Space are shown only to their recipient (inbox filter and mark-read checks). `notifications.CanView` returns true for an empty target. Any new caller must check the recipient itself.
- **Usage counts.** Global counts are admin-only, enforced on the server (`GET /api/v1/item-types/usage`).
- **Workspace custom fields.** Admin-only on write; Space editors cannot create or edit them.
- **Space types.** Created and edited by Space editors only; the server checks that the type's Space matches.
- No secrets were found in the diff (checked before each commit).

## 4. Performance findings

- The inbox still checks visibility by loading Item caps for each page (existing shortcut, comment in `httpserver/notifications.go`). Fine at current volume; revisit when inbox volume grows.
- Requester status notifications add one query per status change, only when the requester has no access.
- No load test was run.

## 5. Outstanding defects

1. **Flaky tests.** `TestBrowserCollaboration` (mention delivery) and `ownership.spec.ts:51` fail sometimes. Neither root cause is confirmed.
2. **Email for requester status** has no automated test and has not been checked on dev-lens.
3. **Layout builder:** drag works only within one section; no drag between sections.
4. **Formulas on Workspace fields** are refused. Not decided.
5. **Space usage counts** are not shown to Space admins (planned as a later enhancement).

## 6. Release readiness

**Recommendation: ready for dev-lens, not ready for a wider release yet.**

Before a wider release:
- Run the live checks on dev-lens (email, Space item types, request status in the inbox).
- Fix or explain the two flaky tests.
- Decide on `scope_test` (see `docs/RELEASE_CLEANUP.md`).
- Decide whether Workspace fields may use formulas.
