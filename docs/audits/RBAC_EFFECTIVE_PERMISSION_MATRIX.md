# RBAC effective permission matrix

Audit date: 23 November 2026. Actual code: `v0.10.0` / `82ea670`; reference baseline: `v0.9.0` / `f43f3d6`.

The backend policy engine is authoritative. `policy.Effective` unions every applicable role and intersects that union with the current user-type ceiling. Space and item engines then apply ownership, inheritance, multi-location, archive, lock, Space Admin, form, and custom-field rules. Frontend decisions consume `my_capabilities`; they do not reconstruct roles.

## Built-in roles

| Capability | Read | Limited | Editor | Full |
|---|---:|---:|---:|---:|
| view | yes | yes | yes | yes |
| comment | no | yes | yes | yes |
| change_status | no | yes | yes | yes |
| attach | no | yes | yes | yes |
| edit | no | no | yes | yes |
| create | no | no | yes | yes |
| use_blueprint | no | no | yes | yes |
| create_blueprint | no | no | yes | yes |
| manage_blueprint | no | no | yes | yes |
| delete | no | no | no | yes |
| share | no | no | no | yes |

Evidence: `server/internal/policy/policy.go` (`roleCaps`, `Effective`). This matches the requested matrix.

## User-type ceilings

| User type | Maximum capabilities |
|---|---|
| admin | all eleven capabilities |
| regular | all eleven capabilities |
| external | through Editor, including blueprint capabilities; no delete/share |
| collaborator | view, comment, change_status, attach |
| viewer | view, comment |

Representative intersections therefore resolve as requested: Regular/Read=view; Regular/Limited=limited set; Regular/Editor=editor set; Regular/Full=all; External/Full=editor ceiling; Collaborator/Full=limited ceiling; Viewer/Full=view+comment; no applicable share=no access.

## Resource modifiers

| Modifier | Verified behavior | Evidence |
|---|---|---|
| Direct and group shares | unioned before ceiling | `policy/engine.go`, `spaceRows` |
| Ancestor shares | recursively apply within the Space | `policy/item.go` |
| Multi-location | applicable paths are unioned; destination and source are checked for moves | `policy/item.go`, `items/location.go` |
| Personal Space | owner only; no implicit workspace-admin access | `policy/engine.go:spaceCaps` |
| Workspace admin | no implicit shared-Space content access | D-17.1, `spaceCaps` |
| Explicit Space Admin | separate from Full; only admin/regular may exercise it | `CanAdministerSpace`, governance tests |
| Archived Space | effective capabilities reduced to view | `spaceCaps` |
| Locked Space | mutation restrictions enforced by policy/location guards | `policy/locked.go`, locked-space regression test |
| Custom role | `mask:N`, then intersected with ceiling | `RoleCaps`, `custom_roles_test.go` |
| Custom fields | grants evaluated separately; inaccessible values are omitted/refused | custom-field access integration tests |
| Invisible resource | 404 | `policy.Decide`, handler adapters |
| Visible but forbidden action | 403 | `policy.Decide`, handler adapters |

## Documentation drift

The 0.9.0 reference remains correct for the four built-in share roles. Current 0.10.0 additionally contains custom-role masks, three blueprint capabilities, D-17.1 explicit-access-only, explicit Space Admin eligibility enforcement, locked-Space move protection, and expanded view behavior. Those are additive/current-state differences, not permission regressions.
