# Batch 5 file ownership

| Lane | Owner | Allowed files | Excluded shared files |
|---|---|---|---|
| B backend | `batch5_backend` | public-form/relation implementation and focused tests under `server/internal/httpserver` | docs, OTP/auth, OpenAPI/generated code, frontend |
| C security | `batch5_security` | OTP/security implementation and focused tests; approval audit read-only | docs, public forms/relations, OpenAPI/generated code, frontend |
| F/G operations | `batch5_ops` | read-only repository/evidence inspection | all writes |
| Integration/UI/docs | root | Batch 5 reports, matrices, Batch 4 VIEW-04 files, final reconciliation | must not overwrite lane edits |

Agents use the clean canonical worktree and isolated test databases. Shared API, migrations, generated types and authorization policy require integration-lead review before any change.
