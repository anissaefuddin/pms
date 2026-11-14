# Roles and custom roles

## User types vs. sharing roles
Each person has a **user type** (e.g. admin, regular, external, collaborator, read-only). The type sets a ceiling. When you share a Space or item you pick a sharing role: the built-ins Read only, Limited, Editor, Full, or a **custom role**. Effective access is the union of a person's roles, cut to their user type's ceiling, so a custom role never exceeds it (D-91). Nobody can grant more than they have themselves.

## Custom roles: Settings → Custom roles (admin)
Click New role. Name, optional description, and tick capabilities: View (always included), Comment, Change status, Attach files, Edit, Create, Delete, Share. The table shows how many shares use each role.
- Edits apply immediately to every share that uses the role. Concurrent edits give "Someone changed this role. Reload".
- A role in use cannot be deleted (409): change or remove those shares first.
- All signed-in users can **read** the role list (so the Share dialog can offer them, D-100); only admins create, change or delete.
- Audited as `role.created|updated|deleted` with before/after capabilities.

## Limits
Not supported: custom roles on the initial share when creating an item, on invitations (rejected), as a user type, and per-field conditions. Role bits are order-sensitive in the database; do not rely on direct DB edits.

## Troubleshooting
- Person cannot do something their custom role allows: check their user type ceiling.
- 403 when saving a role: you are not an administrator.

Related: [access-policy.md](access-policy.md) (locked Spaces), [audit-log.md](audit-log.md).
