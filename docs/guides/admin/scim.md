# SCIM provisioning

Settings → Provisioning (admin). Lets Okta, Entra ID or OneLogin create, update and deprovision users and groups (SCIM 2.0).

## Set up
1. New token: Name (e.g. the provider), "User type for new people" (Regular, External, Collaborator or Read-only; administrators are never allowed).
2. Copy the token now. It is shown once and only its SHA-256 hash is stored. Lost token: revoke it and create another.
3. In the IdP set the SCIM base URL shown on the page (`PUBLIC_URL/api/v1/scim/v2`), authentication Bearer token, and map `userName` = email, `name.givenName`, `name.familyName`, `externalId`, `active`, groups (`displayName`, `members`).
4. Users made by SCIM have no password; they sign in through [sso.md](sso.md) (enable it with auto-link so the first sign-in links).

## Behaviour
- Create uses a seat; if seats are full the request gets 409 ([seats.md](seats.md)). Existing users (invited or with a password) are matched by email.
- Deprovision (DELETE or `active=false`): user becomes deactivated, **all sessions end immediately**, the seat is freed, history stays.
- **Administrators cannot be changed, deactivated or deleted through SCIM** (403 `mutability`). Change them in Settings → Users.
- Groups are normal groups; deleting one removes its shares. Filters: only `attribute eq "value"`. Page size max 200.
- Rate limit 600 requests/minute per token (429 with `Retry-After`).
- Every real change is audited with actor `scim:<token name>` (area `scim`, [audit-log.md](audit-log.md)).
- The table shows Last used and Status per token. Revoke is immediate; people already created remain.

## Troubleshooting
- 401 from the IdP: token revoked or mistyped (include no "Bearer " duplicates). **Unverified:** exact error body.
- 409 on create: no free seat, or the email conflicts.
- IP allowlist does not apply to SCIM calls (D-95), so a token must be protected like a password.
