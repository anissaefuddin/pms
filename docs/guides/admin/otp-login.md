# Sign in with an emailed code

Settings → Access policy → "Sign in with a code" → tick "Allow signing in with an emailed code" and Save policy. **Off by default**, because the inbox becomes a way in (D-97).

## Behaviour
- A person asks for a six-digit code; it is emailed, valid 10 minutes, single use, five attempts (the fifth wrong guess burns it), one request per minute per person; a new code cancels the old.
- It **replaces the password, not 2FA**: a user with 2FA still gets the second step.
- Always answers 202, so it does not reveal who has an account; unknown or deactivated addresses get nothing.
- Failures share the password-login rate limits (per IP and email, 429) and the IP allowlist applies.
- Only a hash is stored; the code itself travels in the email job (jobs kept about 1 hour). **With `SMTP_HOST` empty the code is written to the server log** — do not enable OTP without working SMTP on a shared log system. Code emails use `SMTP_FROM_AUTH` if set ([email-smtp.md](email-smtp.md)).
- Audit: `auth.otp_requested`, `auth.login` (method `otp`), `auth.login_failed` (reason `otp`).

## Troubleshooting
- No email arrives: check SMTP, spam, and the outbox/log; or the one-per-minute limit.
- Code rejected: expired, already used, or five attempts spent: request a new one.
