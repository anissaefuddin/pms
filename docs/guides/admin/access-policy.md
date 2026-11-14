# IP allowlist and locked Spaces

## IP allowlist: Settings → Access policy (admin)
Tick "Only allow the addresses below" and list one address or CIDR per line (IPv4 or IPv6, max 50). The page shows "Your address as the server sees it" and an "Add my address" button.
- Enforced at sign-in (before the password is checked), when sessions are created, in SSO (`sso_error=ip`) and on every request with a session.
- **Not** enforced for public forms, public dashboard links, SCIM, webhooks and health checks (D-95).
- Saving a list that does not contain your own address is refused (`self_lockout`); an empty list cannot be enabled. Saves use a version check ("Someone changed the policy").
- Client address comes from `X-Forwarded-For` only if `TRUST_PROXY=true`. The shipped `docker-compose.yml` sets it true behind Traefik. Without it every user appears as the proxy's address, so the list would be wrong.
- If the policy table is unreachable the request is allowed (fail-open).

### Locked out? (operator)
Set `PMS_DISABLE_IP_ALLOWLIST=true` in `deploy/.env`, `docker compose up -d app`, fix the list, then remove the variable and restart. While it is set the page says the list is not enforced. **Unverified:** the variable is not listed in `docker-compose.yml`; add it under the app `environment:` for it to reach the container.

## Locked Spaces
Open a Space's menu and lock it (admin only; audited as `policy.space_locked/unlocked`). While locked: only admins change the Space's share list; items can only be shared with people who already reach the Space or groups on its list (applies to admins too until unlock); non-admins cannot move its items to another Space. Renaming and daily work are unchanged. **Unverified:** exact menu label (set from each space's menu per the page text).

Related: [otp-login.md](otp-login.md) (same page), [audit-log.md](audit-log.md).
