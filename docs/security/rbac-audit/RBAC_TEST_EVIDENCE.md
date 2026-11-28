# RBAC test evidence

Audit date: 23 November 2026.

## Existing evidence inspected

| Layer | Evidence | Result represented by repository |
|---|---|---|
| Policy unit | `policy_test.go`, `custom_roles_test.go` | built-in matrix, all ceilings, custom masks, malformed masks |
| Space integration | `spaces_integration_test.go` | four roles × five user types, capability response |
| Explicit access | `authorization_final_regression_integration_test.go` | admin without share=404; read share exact; personal isolation; revoke |
| Space Admin | eligibility/downgrade/governance tests | Full is not SA; ineligible types cannot exercise SA |
| Items | item/location/share integration tests | inheritance, multi-location, move, cycle, delegation, 403/404 |
| Locked/archive | locked-space and policy tests | mutation blocked/read-only behavior |
| Custom fields | custom-field access regression tests | field grants and non-leakage |
| Attachments/comments/approvals/time | dedicated integration tests | resource-specific rules and ownership |
| Frontend | Space actions, views, files, approvals, blueprints, settings specs | capability-driven visibility and requests by feature |

## Tests executed in this audit

| Command | Result |
|---|---|
| `go test ./server/internal/policy -count=1` | PASS, package `policy` in 0.524s |
| `go test -count=1 ./server/internal/httpserver ./server/internal/policy -run 'Authorization\|AccessRules\|Matrix\|Capabilit\|SpaceAdmin\|ItemShare\|Move\|ConcurrentMoves\|ReplaceItemLocations'` | PASS; `httpserver` 13.257s; `policy` reported no matching named test (the full policy package was run separately) |
| `npm run typecheck` | PASS |
| `npm run lint` | PASS |
| `PMS_TEST_BROWSER_PORT=4183 npx playwright test tests/blueprints.spec.ts` | PASS, 2/2; existing save path and new view-only regression |
| `go test ./server/internal/httpserver -run TestProfileEmailAndTaskDefaultsAndAccountInfo -count=1` | PASS; regular Account GET denied and admin GET accepted |
| `PMS_TEST_BROWSER_PORT=4186 npx playwright test tests/settings-access.spec.ts tests/work-schedule.spec.ts tests/oauth.spec.ts tests/api-keys.spec.ts tests/webhooks.spec.ts tests/m1-settings-recycle.spec.ts tests/connectors.spec.ts tests/settings-blueprints.spec.ts` | PASS, 25/25; navigation, direct routes, Account, Apps, developer credentials, OAuth grants and calendar-only schedule |
| `npm run typecheck` and `npm run lint` after Settings remediation | PASS |
| `git diff --check` | PASS |

The integration command used the existing isolated `pms-test-pg` PostgreSQL container at port 55432. It did not target production.

## Not run / blocked

- Authenticated live persona matrix on dev-lens: NOT RUN; this pass has no supplied persona credentials or seed reset authorization.
- Full Go and full Playwright suites: NOT RUN in this audit turn; the focused security and changed-surface gates above passed.
- External integrations: NOT RUN; no external provider behavior changed.
- Production mutation probes: NOT RUN by design.

## Required evidence format

Every executed command must record exit code and counts. Skipped tests remain explicit. A focused pass is evidence only for its scope and does not substitute for the full release suite.
