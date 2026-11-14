# Audit log

Settings → Security audit (admin only). Also `GET /audit-events`.

## Reading
Filter by Area (Sign-in and security, Users, Sharing, Roles, Single sign-on, Provisioning, Access policy, API keys, Webhooks, Connected apps, Audit) and a From/To date; Load more pages through results. Columns: Time, Who, Action, Target, IP address, Details. Actor is empty for SCIM calls and shows `scim:<token name>`.

## Export
Export CSV downloads at most **10,000 rows** of the current filter. The export itself is logged (`audit.exported`) with the row count. Cells from attacker-controlled text (e.g. the email in a failed sign-in) are quoted.

## What is recorded
Sign-in, failed sign-in (with attempted email, never the password), sign-out, password change/reset, 2FA events, OTP request, user invite/type change/deactivate/reactivate, sharing changes, role, SSO, SCIM and access-policy changes, locked-space changes, ownership transfer, API keys, webhooks and OAuth apps (see action list in `web/src/i18n/en/audit.json`). Changes made inside a transaction are logged atomically with it. The IP respects `TRUST_PROXY`. Changes that alter nothing are not logged.

## Limits (D-90)
- No retention or pruning: the table grows. **Unverified:** whether a later change added pruning.
- Not tamper-proof at database level; a database administrator can delete rows.
- Login/logout/failed-login logging failures are only written to the server log and do not fail the request.

## Troubleshooting
- Expected event missing: confirm the area filter and date range (dates are UTC day bounds).
- Page shows "Only administrators can read the audit trail": sign in as an admin.
