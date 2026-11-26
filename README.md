# PMS

PMS is a self-hosted, single-tenant project and work management system. Milestone M0 provides the secure account, authentication, UI shell, jobs, observability, and deployment foundation described in [`docs/product/PRD.md`](docs/product/PRD.md).

## Local setup

Requirements: Go 1.26+, Node 24+, PostgreSQL 16+, `sqlc`, and Docker.

```sh
cp deploy/.env.example deploy/.env
cd web && npm ci && npm run generate && cd ..
DATABASE_URL='postgres://pms:pms@localhost:5432/pms?sslmode=disable' PUBLIC_URL='http://localhost:5173' go run ./server/cmd/pms bootstrap --account 'Acme' --admin-email admin@example.com
DATABASE_URL='postgres://pms:pms@localhost:5432/pms?sslmode=disable' PUBLIC_URL='http://localhost:5173' go run ./server/cmd/pms serve
```

Run `npm run dev` from `web/` in another terminal. SMTP delivery is queued in PostgreSQL; configure the `SMTP_*` variables for invitation and password emails.

## Client deployment

1. Copy `deploy/` for the client and fill `.env` with an immutable image tag and secrets.
2. Ensure the external Traefik network exists.
3. Start with `docker compose up -d`.
4. Bootstrap once:

```sh
docker compose exec app pms bootstrap --account 'Client name' --admin-email admin@example.com
```

Schedule `deploy/backup.sh` daily. Test a backup in an isolated client stack with `restore.sh TIMESTAMP --force` before relying on it. Update an instance with `update.sh ghcr.io/anissaefuddin/pms:vX.Y.Z`; it creates a pre-deploy backup first.

## Commands

- `make generate` regenerates sqlc Go code and OpenAPI TypeScript types.
- `make test` runs Go tests and TypeScript checks.
- `pms bootstrap` creates the workspace, the first administrator (state `invited`), the default workflow and the General space exactly once, and prints or emails an activation link (`/invitations/<token>`, valid 7 days). The Personal space is created when the administrator accepts the invitation. If the link expires, run the same command again with the same email to get a new one.
- `pms migrate` applies pending database migrations (they also run on every start).
