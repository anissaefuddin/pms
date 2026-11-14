# Seat management

A seat is one **active or invited** user. The limit is `MAX_SEATS` (default `25`, must be a positive integer, otherwise the server refuses to start).

## What you see
Settings → Users shows: "N active · N invited · N available of MAX seats". The same numbers come from `GET /seats`.

## Rules (from code, DECISIONS and the runbook)
- Pending invitations use a seat. A pending invite plus later reprovisioning does not use a second seat (D-94).
- Deactivating a user (Users → Deactivate) frees the seat at once, revokes their sessions and keeps their history. Reactivate uses a seat again.
- SCIM-created users use a seat; SCIM reprovisioning returns 409 when no seat is available (D-94).
- The placeholder account `external-requester@system.invalid` (public forms) does not appear in Users and uses no seat. Never activate or delete it.
- Raising the limit is an operator action: edit `MAX_SEATS` in `deploy/.env`, then `docker compose up -d app`. There is no UI for it.

## Troubleshooting
- Invite or reactivation fails with a seat message: check the counts; deactivate unused accounts or raise `MAX_SEATS`. **Unverified:** the exact error text/status for an invite when seats are full (only the SCIM 409 is documented).
- Counts look too high: invited-but-never-accepted people still hold a seat; deactivate them.
- Server refuses to start with `MAX_SEATS must be a positive integer`: fix the value.

Related: [scim.md](scim.md), [roles.md](roles.md).
