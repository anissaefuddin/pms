# RBAC final review

Status: **CONDITIONAL PASS for the audited remediation; full master-prompt Definition of Done remains open**.

## Confirmed

- Backend policy matches the requested built-in role matrix and user-type ceilings.
- Effective rights are unioned across applicable shares and intersected with the ceiling.
- Personal Space, explicit-access-only, explicit Space Admin, archived/locked Space, inheritance, multi-location, custom role and custom-field rules exist in the backend.
- 404 versus 403 is modeled centrally and used by scoped handlers.
- Frontend core surfaces generally consume effective capabilities returned by the backend.
- One misleading item action was found and remediated without altering policy.
- Settings now separates personal, workspace-admin, and Space-capability destinations; direct admin-only URLs fail closed.
- Account metadata is enforced as admin-only in both frontend and backend.
- Non-admin Work Schedule exposes only the annual calendar; workspace rules and holidays remain admin-only.
- Personal OAuth grants remain revocable in Security, while API keys and webhooks live under personal Developer Tools.

## Remaining

- Add the named frontend cross-product matrix from `RBAC-TEST-001` if full Definition of Done is required in this workstream.
- Run an authenticated seeded persona matrix in an isolated/dev environment before claiming live-environment acceptance.
- Run the full Go and Playwright suites after the remaining cross-product tests are added.
- Revisit Space deletion authority only through an explicit product/security decision.

## Release impact

The changes are fail-closed. Users without an eligible blueprint destination no longer see that item action, non-admin users do not load admin-only Settings data, and `GET /account` now returns forbidden for non-admin users. The Account authorization change is intentional API hardening; clients that previously read workspace Account metadata as a regular user must stop doing so.

No commit, push, merge, deployment, migration, or production data operation was performed by this audit.
