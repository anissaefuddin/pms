# Account ownership

Settings → Account ownership (admin). The first administrator created at bootstrap is the owner; on older instances the oldest admin was set as owner by migration (D-97).

## Rules
- The owner cannot be demoted or deactivated by other admins (409 `account_owner`).
- Only the owner can hand ownership over, and only to another **active administrator** (make someone admin first in Users).
- Proof is required: the owner's password, or, if they have none (SSO/SCIM), a session created in the last 15 minutes ("Sign in again, then transfer within 15 minutes"). A wrong password counts against the failed-login budget.
- Transfer is immediate and direct: no acceptance step and no notification email to the new owner. The former owner stays a normal admin, whose role other admins can then change. Audited `account.ownership_transferred`.

## Not covered
If the owner is deactivated through the database, nothing hands ownership over (D-97). Recovery requires database work by the operator. **Unverified:** a documented procedure for that.

## Troubleshooting
- "There is no other active administrator": promote someone first.
- "Only the owner can hand ownership over": sign in as the owner.
