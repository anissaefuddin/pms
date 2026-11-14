# Connectors and webhooks

Settings → Apps & Integrations (tabs: API, Webhooks, Authorized apps, OAuth clients, Connectors). Needs `PMS_CONNECTOR_KEY` to store secrets ([encryption-keys.md](encryption-keys.md)).

## Connectors (admin to install)
Add a connector: Type, Name, then settings.
- **GitHub Issues** (two-way: title, text, open/closed): Repository `owner/name`, Space, Access token, optional webhook secret, options to import new open issues as tasks and create issues for new tasks. For faster sync add a GitHub webhook to `PUBLIC_URL/api/v1/connectors/<id>/github-webhook`.
- **Google Calendar** (one-way: items with a due date become all-day events; only items you can see): Calendar ID, Space, and either an access token (expires in about 1 hour) or a refresh token with your own Client ID and secret.
Per connector: Enable/Disable, Sync now, last-sync status, and Sync conflicts (Keep item / Keep remote).
Troubleshooting: 422 `key_not_configured` = set `PMS_CONNECTOR_KEY`; sync fails after a key change = "cannot be decrypted", reinstall; Google stops after an hour = use a refresh token.

## Webhooks
Each webhook belongs to its creator and only sends events that creator may see; per-Space webhooks are a known gap (D-98). Create: Name, HTTPS endpoint URL, Events. The signing secret is shown once.
- Verify header `X-PMS-Signature` = `v1=HMAC-SHA256(secret, timestamp + "." + body)` and reject `X-PMS-Timestamp` more than 5 minutes off.
- Endpoints are SSRF-guarded: private, loopback, link-local and other reserved addresses are refused, and redirects are not followed.
- Per webhook: Pause/Enable again (it can be disabled with a reason), Delivery log (attempts), Delete. Changes are audited under area `webhook`.
Troubleshooting: "Disabled: <reason>" after failures, fix the endpoint then Enable again; endpoint rejected = it resolves to a reserved address or is not HTTPS. **Unverified:** retry schedule and auto-disable threshold.
