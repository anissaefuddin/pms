# PMS Administrator Guide

Guides for people who run a PMS instance and administer its account. Everything here is derived from the server code (`server/internal/config`), `api/openapi.yaml`, [DECISIONS.md](../../architecture/DECISIONS.md) D-90..D-101, [ARCHITECTURE.md](../../architecture/ARCHITECTURE.md), `deploy/` and the admin screens under Settings. Where a statement could not be checked against code or docs it is marked **Unverified**.

Two roles appear throughout: the **operator** (has shell access to the server and `deploy/.env`) and the **administrator** (a PMS user of type admin who works in Settings). Items marked "operator" cannot be done from the UI.

| Guide | Who | Topic |
|---|---|---|
| [seats.md](seats.md) | admin, operator | Seat limit, invites, deactivation |
| [backup-restore.md](backup-restore.md) | operator | Backups, off-server copy, restore |
| [updating.md](updating.md) | operator | Update to a new version, rollback |
| [two-factor.md](two-factor.md) | admin, operator | 2FA setup, recovery, admin reset |
| [sso.md](sso.md) | admin | OIDC and SAML sign-in |
| [scim.md](scim.md) | admin | SCIM 2.0 provisioning |
| [roles.md](roles.md) | admin | User types and custom roles |
| [audit-log.md](audit-log.md) | admin | Reading and exporting the audit trail |
| [access-policy.md](access-policy.md) | admin, operator | IP allowlist, locked Spaces |
| [otp-login.md](otp-login.md) | admin | Sign in with an emailed code |
| [ownership.md](ownership.md) | admin | Transferring account ownership |
| [encryption-keys.md](encryption-keys.md) | operator | `PMS_2FA_KEY`, `PMS_SSO_KEY`, `PMS_CONNECTOR_KEY` |
| [connectors-webhooks.md](connectors-webhooks.md) | admin | Connectors and webhooks |
| [email-smtp.md](email-smtp.md) | operator | SMTP settings and email behaviour |

## Settings map

The Settings sidebar (`web/src/routes/_app/settings.tsx`) shows these admin pages only to administrators (Users is shown to anyone with the manage-users capability): Account management → Users, Single sign-on, Provisioning, Account ownership, Access policy, Custom roles, Security audit; Apps & Integrations (connectors, webhooks, API, OAuth apps). Your own Sign-in & security page (password, 2FA) is under Settings too.

## Operator basics

Configuration is environment variables read by `config.Load()`; in a standard install they live in `deploy/.env` and are passed through `deploy/docker-compose.yml`. After changing one, run `docker compose up -d app` from `deploy/`. Never commit `deploy/.env`. See also [PILOT_RUNBOOK.md](../../operations/PILOT_RUNBOOK.md) (Indonesian) for the install and pilot procedure.
