# RBAC gap report

Baseline compared: reference `f43f3d6` (0.9.0) against current `82ea670` (0.10.0). No P0 or P1 defect was found in this pass. Existing security closure evidence covers prior locked-Space move, Space Admin downgrade, item-share delegation, and move-history defects.

| Finding ID | Severity | Affected role/user type | Scope | Screen/endpoint | Expected | Actual/root cause | Proposed fix | Evidence | Status |
|---|---|---|---|---|---|---|---|---|---|
| RBAC-UI-001 | P2 | Any actor without `create_blueprint` destination | Item action menu | Item detail / blueprint POST | Do not advertise unusable mutation | Menu item was unconditional; dialog later had no eligible Space and disabled submit | Gate menu from server-provided Space capabilities | `item-detail.tsx`; Playwright regression | FIXED, uncommitted |
| RBAC-DOC-001 | P3 | All | Documentation | 0.9.0 reference | Describe current contract | Reference predates custom roles, blueprint caps, D-17.1 and security closure | Record drift; retain 0.9.0 as historical baseline | permission matrix document | CLOSED |
| RBAC-TEST-001 | P2 test gap | Four roles × five user types | Frontend all views | E2E suite | One explicit cross-product journey validates UI and server together | Backend matrix is strong; frontend tests use capability fixtures per feature but do not expose one named 20-intersection E2E suite across every view | Add a compact capability-driven view matrix; do not duplicate backend policy calculation | repository test search | OPEN |
| RBAC-UX-002 | P3 | Limited and above | Time tracking | time-entry APIs/UI | Mapping is understandable | Time entry create/edit/delete uses `change_status`, not `edit`; correct but unintuitive | Keep semantics; document in UI/help/test names | `time_entries.go:141,222` | ACCEPTED |
| RBAC-PROD-001 | P2 operational | All | Deployed environment | dev-lens | Audit build matches audited commit | Repository audit is current 0.10.0; no authenticated live role-persona matrix was executed in this audit | Run persona E2E against isolated seeded environment before release claim | test evidence | BLOCKED: no persona credentials/seed requested |
| RBAC-SET-001 | P2 | Non-admin | Settings navigation | `settings-sidebar.tsx` | Workspace-admin destinations are absent | Apps, Resources, Account, and global configuration are admin-only; aggregate Space tools are capability-scoped; personal developer credentials have their own destination | Split personal, admin-only and capability-scoped destinations | code review; `settings-access.spec.ts` | FIXED, uncommitted |
| RBAC-SET-002 | P1 privacy | Non-admin | Account metadata | `GET /account`, `/settings/account` | Seat/admin directory is workspace-admin-only | UI hides and guards Account; backend GET now applies `requireAdmin` | Add `requireAdmin`, hide link, add direct-URL/API tests | `server.go`; profile integration test; Settings E2E | FIXED, uncommitted |
| RBAC-SET-003 | P2 | Non-admin | Work schedule | `/settings/work-schedule` | Non-admin sees calendar only; admin manages workspace rules/holidays | Non-admin now receives the annual calendar without rule, timezone, hours, holiday-editor, or save controls | Render only annual calendar/legend for non-admin; keep PUT admin-only | route, `worksched.go`, `work-schedule.spec.ts` | FIXED, uncommitted |
| RBAC-SET-004 | P1 if broadened incorrectly | Full Space member | Workspace holidays | `PUT /settings/work-schedule` | Workspace-global mutation requires workspace authority | Backend correctly requires user type Admin; a Space Full share does not qualify | Preserve admin-only backend; do not map Space Full to workspace schedule | `worksched.go:putWorkSchedule` | CONFIRMED |
| RBAC-SET-005 | P2 | Non-admin | Direct settings routes | Apps/Resources/Account/global Work configuration | Hidden navigation and direct route communicate no access without loading admin data | Admin-only pages now fail closed and suppress their protected queries; backend checks remain authoritative | Add page guards for genuinely admin-only pages; backend remains mandatory | `settings-access.spec.ts` | FIXED, uncommitted |
| RBAC-SET-006 | P3/security UX | All authenticated users | Authorized Apps | Security | Users can inspect/revoke their own OAuth grants | Personal grants now live under Security; workspace connectors and OAuth clients remain in admin-only Apps | Keep personal Authorized Apps. Moving it to admin-only would prevent users revoking their own grants | OAuth E2E | ACCEPTED, clarified |

## Security decisions not changed

- Workspace administrators receive no implicit shared-Space content access.
- Full share does not imply explicit Space Admin.
- Full currently permits deleting a Space through the `delete` capability; changing that to SA is a product/security decision.
- Comment owners may edit/delete their own comments; item Full authority may moderate.
- Attachment uploaders with `attach` may remove their own attachment; item `delete` may remove any.
- Time entries remain tied to `change_status` and ownership.

## Escalations

No policy-semantic change is proposed. A product decision is still needed only if Space deletion should move from `delete` to explicit SA. This audit does not make that change.

The Settings ownership decision is resolved: `/settings/account` and `GET /account` are workspace-admin-only. Personal API keys and webhooks remain self-service under Developer Tools; personal OAuth grants remain under Security.
