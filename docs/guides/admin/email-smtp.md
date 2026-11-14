# Email and SMTP configuration

Environment variables (operator, in `deploy/.env`, restart the app after changes):

| Variable | Default | Notes |
|---|---|---|
| `SMTP_HOST` | empty | Empty means **log mode** (below) |
| `SMTP_PORT` | `587` | |
| `SMTP_USER`, `SMTP_PASSWORD` | empty | PLAIN auth is used only if `SMTP_USER` is set |
| `SMTP_FROM` | `PMS <noreply@localhost>` | Sender for notifications and invitations |
| `SMTP_FROM_AUTH` | = `SMTP_FROM` | Sender for activation, password reset and sign-in codes |
| `SMTP_REPLY_TO` | empty | |
| `PUBLIC_URL` | `http://localhost:8080` | Used for links inside emails |

## Behaviour
- **STARTTLS is required** unless the host is `localhost` or `127.0.0.1`; a server without it fails with "does not offer STARTTLS". There is no implicit-TLS (port 465) mode in the code. **Unverified:** port 465 providers.
- One delivery has a 30 s limit. Failures retry with exponential backoff, up to 8 times, then go to a visible failed queue (ARCHITECTURE §email).
- Email jobs are kept about 1 hour, so links and codes in them are short-lived.
- **Log mode** (no `SMTP_HOST`): the email, including activation/reset links and OTP codes, is written to the log as a warning. Only acceptable on a trusted log; production needs SMTP.
- Used by: invitations, password reset, [OTP codes](otp-login.md), notifications, form confirmations.

## Docs mismatch
ARCHITECTURE.md lists `PMS_SMTP_HOST/PORT/USER/PASS/FROM`; the code and `docker-compose.yml` use the `SMTP_*` names above. Trust the code.

## Troubleshooting
- No mail: check `docker compose logs app` for "email logged instead of sent" (SMTP unset) or SMTP errors; check spam and SPF/DKIM for the `SMTP_FROM` domain (outside PMS).
- Links point at localhost: set `PUBLIC_URL`.
