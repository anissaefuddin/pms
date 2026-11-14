# Two-factor authentication (2FA)

TOTP via an authenticator app. Secrets are stored encrypted with `PMS_2FA_KEY` (AES-256-GCM).

## Enable for the instance (operator)
Set `PMS_2FA_KEY` (`openssl rand -base64 32`) in `deploy/.env` and run `docker compose up -d app`. Without it the server runs but Settings → Sign-in & security shows "not available on this server yet" and nobody can turn 2FA on. Details and rotation: [encryption-keys.md](encryption-keys.md).

## User setup
Settings → Sign-in & security → Set up two-factor authentication: scan the QR code (or type the key), enter the 6-digit code, Turn on. Backup codes are shown once (copy or Download .txt); each works once. Turning 2FA on signs out the user's other sessions. Users can Generate new backup codes (needs password plus an app or backup code). Admins cannot force 2FA for everyone: **Unverified:** no "require 2FA" setting was found in the code or docs.

## Lost phone and backup codes: admin reset
An administrator calls `POST /users/{id}/2fa/reset`. It deletes the secret and backup codes, signs the user out everywhere and records `auth.2fa_reset_by_admin` in the audit log. As of the runbook there is **no button in the UI** (a grep of `web/src` finds only the generated API schema). Confirm the person's identity outside the app first. They then sign in with their password and can enrol again.

## Interactions
- SSO sign-in does not ask for local 2FA (D-92). OTP email sign-in still asks for 2FA (D-97, [otp-login.md](otp-login.md)).
- Audit actions: `auth.2fa_enabled`, `auth.2fa_disabled`, `auth.2fa_backup_codes_renewed`, `auth.2fa_reset_by_admin` ([audit-log.md](audit-log.md)).

## Troubleshooting
- "That code is not valid": the phone clock is off; fix time sync.
- Everyone with 2FA fails after a restore or update: `PMS_2FA_KEY` differs from the original or is missing; look for the `PMS_2FA_KEY is not set` log line. Restore the right key, or reset each user.
