# Encryption keys

Four separate keys, each **32 random bytes, base64** (`openssl rand -base64 32`). A malformed value stops the server at start-up. All but `PMS_OTP_KEY` use AES-256-GCM. Set them in `deploy/.env`; `docker-compose.yml` passes them through. Each has a `_PREVIOUS` twin for rotation; `_PREVIOUS` without the main key is a start-up error.

| Variable | Protects | Missing key means |
|---|---|---|
| `PMS_2FA_KEY` | TOTP secrets | Nobody can enable 2FA (log: `PMS_2FA_KEY is not set`) |
| `PMS_SSO_KEY` | OIDC client secret | Settings → Single sign-on cannot store a secret |
| `PMS_CONNECTOR_KEY` | Connector secrets and webhook signing secrets | Saving them is refused with 422 `key_not_configured`; nothing is stored in plain text (D-98) |
| `PMS_OTP_KEY` | HMAC over one-time sign-in codes (not encryption; no `_PREVIOUS`) | Settings → Access policy cannot turn code sign-in on (422 `key_not_configured`); the code endpoints answer 503 |

## Rules
- **Never** store keys in git or in the backup folder; the database dump holds only ciphertext. Keep them in a secret store and record where in your hand-over notes.
- On restore, the target must use the same keys ([backup-restore.md](backup-restore.md)). Check logs for `... is not set` after every update and restore.

## Rotation
1. Put the old key in `X_PREVIOUS`, a new one in `X`; `docker compose up -d app`.
2. **2FA:** each secret moves to the new key when its owner signs in with 2FA. Clear `_PREVIOUS` only after all 2FA users have signed in; unmigrated secrets then become unreadable and need an admin reset.
3. **SSO:** re-save the client secret in Settings → Single sign-on.
4. **Connector/webhook:** re-save every connector (moves it to the new key). Webhook signing secrets cannot be re-encrypted: create a new webhook, give the receiver the new secret, delete the old one, before dropping `_PREVIOUS`.

## If a key is lost
Unrecoverable by design. 2FA: reset each user ([two-factor.md](two-factor.md)). SSO: set a new key and re-enter the client secret. Connectors: reinstall. Webhooks: recreate (receivers get new secrets). A wrong key never sends unsigned requests.

Existing plain-text rows from before D-98 stay readable. **Unverified:** whether SSO/2FA have any such legacy rows.

## One-time sign-in code key (`PMS_OTP_KEY`)
Set it in `deploy/.env`, then `docker compose up -d app`; the log line `PMS_OTP_KEY is not set` disappears. Then turn the feature on in Settings → Access policy. Changing or losing the key only invalidates codes in flight (they live minutes); nothing stored becomes unreadable, so no rotation procedure is needed.
