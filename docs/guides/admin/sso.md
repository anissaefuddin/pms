# Single sign-on (OIDC and SAML)

Settings → Single sign-on (admin). SSO only **adds** a way in: passwords keep working, there is no "SSO required" switch, and people who are not already users are never created at sign-in (D-92). One provider per kind. Users can link or unlink their own identity under their sign-in page; unlinking needs a password so they cannot lock themselves out.

## Prerequisites
- `PUBLIC_URL` must be the real external address (redirect and ACS addresses are built from it).
- OIDC needs `PMS_SSO_KEY` to store the client secret ([encryption-keys.md](encryption-keys.md)). The page says "PMS_SSO_KEY is not set on this server" otherwise.
- SAML needs HTTPS: `COOKIE_SECURE=true` (the response is a cross-site POST that needs a `SameSite=None; Secure` cookie, D-93).

## OIDC
1. At your provider create an OIDC application with the Redirect URI shown on the page (`PUBLIC_URL/api/v1/auth/sso/oidc/callback`).
2. Fill Button name, Issuer URL, Client ID, Client secret; optionally Allowed email domains. Save without enabling.
3. Tick "Show the sign-in button" and save. Saving as enabled proves discovery works and the secret is stored.
4. Test from another browser window.
Issuer must be `https`, without credentials and not a loopback/private literal address. Needs a verified email (`email_verified`) for the first link.

## SAML 2.0
1. Give your IdP admin the Entity ID, ACS URL (HTTP-POST) and metadata URL (`PUBLIC_URL/api/v1/auth/sso/saml/metadata`) from the page.
2. Paste the IdP's whole EntityDescriptor XML (max 256 KiB). The stored copy is never shown again; leave the field empty to keep it. To rotate the IdP certificate, paste new metadata.
3. IdP requirements: signed assertion or response, NameID `persistent` or `emailAddress` (not `transient`), an `AudienceRestriction` with our Entity ID, an email attribute (`email`, `mail`, or the name entered in "Email attribute"), clock skew within about 3 minutes.
Not supported: encrypted assertions, signed requests, IdP-initiated sign-in (start from the button on the login page).

## Linking behaviour
"Link first sign-ins by email" ties a first sign-in to the active user with the same verified email. Invited, deactivated and unknown people are refused. Removing the provider forgets all linked identities.

## Troubleshooting
Failures redirect to `/login?sso_error=<reason>`: `state`, `denied`, `failed`, `unavailable`, `unlinked`, `unverified`, `inactive`, `domain`, `session`, `already_linked`, and `ip` (blocked by [access-policy.md](access-policy.md)). Each is logged as `sso.login_failed` in [audit-log.md](audit-log.md); for SAML the detailed reason is only in the server log.
- `unlinked`: the person has no linked identity and auto-link is off, or the email is not a user.
- `unverified`: IdP did not mark the email verified. `inactive`: user is deactivated or still invited.
- SAML fails behind a proxy over HTTP: ensure HTTPS end to end for the browser and `COOKIE_SECURE=true`.

Related: [scim.md](scim.md), [two-factor.md](two-factor.md).
