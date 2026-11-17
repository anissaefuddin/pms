# Batch 5 release report: Custom item types, requests, and Space settings

Date: 17 Nov 2026. Branch `feat/m12c-ui-clean` (PR #18). Commits in scope: `0665875`..`df48d7b`. Deployed to dev-lens: `pms:df48d7b` (previous: `pms:2648c70`).

## 1. Matrix

| Area | Check | Result | Evidence |
|------|-------|--------|----------|
| Requests (RF-08) | Requester without access gets no Item, comments, or attachments; status change reaches the inbox with no Item or Space link | PASS (Go) | `TestMyRequests`, `TestFormSpaceMetadataFollowsSpaceAccess` |
| Requests | Email for a requester status change follows `task_changes` preference | NOT RUN | Code path changed (`mail/notification.go`); no test; not checked on dev-lens |
| Quick Access (CIT-D05) | Menu, More dialog, Settings toggle, Space menu gating | PASS (Go + Playwright) | `TestItemTypeQuickAccessValidationAndMenu`, m8 tests |
| Usage (CIT-D06) | Global counts admin-only; non-admin gets 403 | PASS (Go) | `item_type_usage_integration_test.go` |
| Space item types | Space editors manage their Space's types; Workspace types read-only; create sends `space_id` | PASS (Playwright) | m8 "a Space's item types" |
| Space usage | Space managers see live item counts for their Space; readers and outsiders do not | PASS (Go) | `TestItemTypeUsageCountsLiveItems` |
| Space item types | Same on dev-lens with real data | NOT RUN | Needs a test login on dev-lens |
| Layout builder | Up/down buttons, drag within a section, drag to another section | PASS (Playwright) | m8 "dragging a block", "a Space's manager sees usage..." |
| Layout builder | Keyboard-only drag | NOT BUILT | Arrow buttons are the keyboard path |
| Workspace custom fields | Admin-only create/edit; Space editors cannot | PASS (Go) | `workspace_custom_fields_integration_test.go` |
| OpenAPI | `/custom-fields` documented | PASS (spec and generated TS updated) | `api/openapi.yaml` |
| Accessibility | Registry, type form, Space item types | PASS (axe, 0 violations) | m8 "pass axe" |
| Accessibility | Layout builder preview, keyboard drag | PARTIAL | Axe on the form dialog only |
| Visual | Registry and Space item types at 1280/1440/1920, light and dark | CAPTURED, not compared | `docs/qa/review-1/batch5/` (12 PNG) |
| Visual | Pixel comparison against a baseline | NOT RUN | No baseline exists |
| Performance | Load or timing tests | NOT RUN | No performance harness in the repo |
| Browser collaboration | `TestBrowserCollaboration` | FLAKY, mitigated | Failed in CI at `5258d9e` (mention comment not saved); fixed in `df48d7b` (wait for the popover to close, assert comment POST 201). CI green at `df48d7b`. Root cause not confirmed |
| Full Playwright | `npx playwright test` | 539 passed, 43 skipped, 1 failed | The one failure, `reports.spec.ts:169`, passed 9 of 9 alone; `ownership.spec.ts:51` failed once earlier and passed 3 of 3 alone |
| Full Go | `go test ./...` | PASS | All packages ok |
| Lint and types | eslint, tsc, token check, gofmt, go vet | PASS | |
| CI | PR #18 head `df48d7b` | `test`, `authorization-matrix`, `parity-e2e` PASS | run 44065257853 |
| CI | PR #18 head `5258d9e` | `test` FAIL (pilot mention), fixed in `df48d7b` | run 44062757256 |
| Deploy | dev-lens on `pms:2648c70` | PASS (health 200, migrations 00328–00330 applied, no ERROR logs) | Backup `20261117T094440Z` |
| Deploy | dev-lens on `pms:df48d7b` | PASS (readyz 200, healthz 200, no ERROR logs, no new migrations) | Backup `20261117T110451Z` |
| Live checks on dev-lens | Email, Space item types, request status in inbox | NOT RUN | |

## 2. Files changed in this batch

- Server: `items/item_types.go`, `items/custom_fields.go`, `items/type_fields.go`, `items/notifications.go`, `notifications/record.go`, `httpserver/notifications.go`, `httpserver/requests_integration_test.go`, `mail/notification.go`, `httpserver/create_menu.go`, `httpserver/form_submit.go`, `policy/item.go`.
- Migrations: `00327` (Space item types), `00328` (Workspace custom fields), `00329` (Quick Access), `00330` (notifications with no target).
- API: `api/openapi.yaml`, regenerated `apigen` and `web/src/api/schema.d.ts`.
- Server (later): `httpserver/item_types.go`, `httpserver/spaces.go` (Space usage route), `items/item_types.go` (`SpaceItemTypeUsage`).
- Web (later): `routes/_app/settings/item-types.tsx` (drag across sections, Space usage), `tests/pilot-live.spec.ts`.
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

1. **Flaky tests.** `TestBrowserCollaboration` (mention delivery) was fixed in `df48d7b`, but its root cause is not confirmed. `ownership.spec.ts:51` and `reports.spec.ts:169` each failed once in a full run and passed alone. All three are tracked in `docs/RELEASE_CLEANUP.md`.
2. **Email for requester status** has no automated test and has not been checked on dev-lens.
3. **Keyboard-only drag** is not built. The up/down buttons are the keyboard path.
4. **Formulas on Workspace fields** stay refused. Decided 17 Nov 2026: a Workspace field cannot use a formula, because a formula could read Space fields that a Workspace type does not own.
5. **`scope_test`** is inactive on dev-lens and not deleted. Decision pending (see `docs/RELEASE_CLEANUP.md`).

## 6. Release readiness

**Recommendation: ready for dev-lens, not ready for a wider release yet.**

Before a wider release:
- Run the live checks on dev-lens (email, Space item types, request status in the inbox).
- Fix or explain the two flaky tests.
- Decide on `scope_test` (see `docs/RELEASE_CLEANUP.md`).
