# Batch 4 execution report

Date: 15 November 2026. No commit, push, merge or deploy was performed.

| Requirement | Actual result | Status |
|---|---|---|
| AUTH-08 | D-97 implements generic request response, queued SMTP, expiry/single use, cooldown/rate limits, attempt limit, TOTP continuation, session and audit. Focused backend tests pass. Live feature-on/mail-sink lifecycle was not run. SHA-256 treatment of a six-digit code conflicts with offline-guess-resistance wording; expired/invalid share a generic UI message and no visible resend countdown exists. | BLOCKED for live acceptance; conflict recorded |
| SPC-14 | D-99 implements per-Space create-menu, custom item-type name/icon, visibility, permission, locking, persistence and Blueprint section. D-99 uses the base item-type icon; this is no longer a pending decision. | LOCAL_PASS; deployed evidence PARTIAL |
| VIEW-04 | Existing configuration now also controls assignees, start date, finish date and priority. Cover, ID, type, location, author, days-in-status and custom fields remain. Picker contrast was corrected. | LOCAL_PASS; deployed evidence PARTIAL |

## Verification

- Focused backend OTP/create-menu/card-field command: PASS on `qa/integration-b3`.
- `npm run typecheck`: PASS.
- Board/view-field Playwright: 6/6 PASS.
- Latest completed candidate CI at preflight, run 43682109614, failed only in PostgreSQL browser workflows; parity-e2e and authorization-matrix passed. Run for `828c2ef` was still in progress.

## Not claimed

No full 208-scenario rerun, isolated OTP mail-sink run, multi-browser run, deployed verification, UAT acceptance or feature-freeze approval was completed.
