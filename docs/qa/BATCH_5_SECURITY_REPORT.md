# Batch 5 security report

Status: **LOCAL_PASS with external acceptance blockers**.

## OTP login

- Replaced database-copy-guessable SHA-256 OTP verification with HMAC-SHA-256 using a dedicated 32-byte base64 `PMS_OTP_KEY`.
- The key is not stored in the database. OTP enablement without it returns `422 key_not_configured`.
- An upgraded instance that already has OTP enabled but lacks the key fails closed with HTTP 503.
- Rotating the key invalidates outstanding codes; maximum exposure is the existing ten-minute lifetime.
- Database-only guessing regression passes.
- Five concurrent verification requests produce exactly one HTTP 200 and one session; repeated ten times successfully.
- Existing expiry, attempt limit, enumeration protection, TOTP continuation and rate limits remain covered.

Deployment prerequisite: generate and configure `PMS_OTP_KEY` before enabling OTP. Mail-sink lifecycle evidence remains BLOCKED.

## Authorization and approval

- Relation hidden-target and no-name-leak tests pass repeatedly; no policy change was required.
- Full Go authorization/security suites pass locally.
- Auto-approval still drops inaccessible configured reviewers; the existing test encodes this behavior. RT-14 is not approved, so runtime policy was not changed.

## QA clone sanitization

Required before clone use: revoke all sessions; rotate/reset API, OAuth, webhook and connector credentials; disable production connectors; redirect all mail to a sink; separate database/storage/secrets/IdP; sanitize personal data; block production notifications. This procedure is documented but not executed against an environment.

## Not run

Manual OWASP WSTG sampling, exact-candidate image/dependency scan, live provider scope/revocation tests and sanitized-clone destructive verification remain NOT RUN.

