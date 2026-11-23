# RBAC remediation plan

## Completed low-risk workstream

1. Consume `my_capabilities`/Space capabilities already returned by the backend.
2. Hide “Save as blueprint” when no shared Space grants `create_blueprint`.
3. Add a Playwright regression proving the action is absent for a view-only actor while the existing success path remains.

No backend semantics, schema, dependency, route, API contract, or production data changed.

## Next workstream (P2 test coverage)

Create one data-driven E2E suite that feeds capability sets for Read, Limited, Editor and Full into Table, Board, Calendar, Gantt, Files and Timelog. Assertions must pair UI visibility/interaction with captured API calls. Use separate cases for External/Full, Collaborator/Full and Viewer/Full ceilings; backend integration remains the authority for the actual intersection.

Minimum cases:

- Read: no mutation control and no mutation request.
- Limited: status/comment/attachment allowed; dates/title/create/delete/share denied.
- Editor: edit/create allowed; delete/share denied.
- Full: all baseline actions offered.
- Mixed-authority bulk selection: unauthorized row is not silently mutated.
- Optimistic status update rolls back on 403/409.

## Release gate

Before declaring the larger audit complete:

1. Run policy unit tests and authorization-matrix integration tests against PostgreSQL.
2. Run focused web tests for navigation, Space actions, item detail, all views, attachments, approvals, blueprints and settings.
3. Run full frontend typecheck/lint/build and full Playwright suite.
4. Run authenticated seeded personas if environment access is available; otherwise mark NOT RUN.
5. Re-review OpenAPI response capability fields and documentation drift.

No commit, push, merge, deploy, or production mutation is part of this plan without separate authorization.

## Settings navigation workstream

Recommended target grouping:

| Group | Visibility |
|---|---|
| Personal: Profile, Language, Email, Appearance, Security, Work preferences | every authenticated user; each edits self only |
| Authorized Apps | every authenticated user, inside Security; owner-scoped revoke |
| Account | admin-only if the product confirms seat/admin contact data is confidential |
| Apps / Integration administration | admin-only route; personal API keys/webhooks need a separate personal Developer destination if retained |
| Resources | admin-only workspace directory; move personal capacity/leave to a personal schedule page if self-service remains |
| Work schedule | visible to all as an annual calendar; only admin sees working-day, timezone, hours, holiday mutation controls |
| Workspace Item types, Workflows, Workspace custom fields | admin-only |
| Automation, Request forms, Blueprints | admin or users with an applicable editable/manageable Space; otherwise hidden |
| Space-specific configuration | accessed through Space Settings and effective Space capabilities, not the global admin group |

Implementation order:

1. Decide Account visibility and whether personal API keys/webhooks remain self-service.
2. Centralize settings destination metadata (`personal`, `admin`, or capability predicate) and use it for sidebar, breadcrumb and topbar links.
3. Add route-level fail-closed pages for admin-only destinations.
4. Keep backend admin checks; add `requireAdmin` to Account only if approved.
5. Simplify non-admin Work schedule to the annual calendar; do not grant Space Full authority over workspace holidays.
6. Add navigation and direct-URL tests for admin, regular, external, collaborator and viewer.
