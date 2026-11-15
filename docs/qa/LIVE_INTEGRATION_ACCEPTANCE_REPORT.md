# Live integration acceptance report — Batch 5

Mocks and local fixtures are not counted as live acceptance.

| Service | Available evidence | Live status | Prerequisite |
|---|---|---|---|
| GitHub connector | fixture/local paths pass | BLOCKED | disposable repository and token |
| Google Calendar | fixture/local paths pass | BLOCKED | disposable calendar/account |
| SMTP | mail job tests exist | BLOCKED | isolated mail sink and feature-on OTP run |
| Webhook | signed/replay local tests pass | NOT RUN live | disposable HTTPS receiver |
| OIDC/SAML | local contract tests | BLOCKED | sandbox IdP |
| SCIM | local API tests pass | NOT RUN live | sandbox IdP/client |
| Pydio Cells | adapter/source coverage | NOT RUN | isolated pilot endpoint |
| MinIO/S3 | adapter/source coverage | NOT RUN | disposable bucket and credentials |

Invalid/revoked credentials, expiry, scope, duplicate events, retry, reconnect and audit must be exercised per service once prerequisites exist. No production credential or resource was used.

