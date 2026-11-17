# Release cleanup register

Items that must be cleaned up or decided before a production release. Keep one row per item.

| # | Item | Where | State now | Action before release | Owner |
|---|------|-------|-----------|-----------------------|-------|
| 1 | Item type `scope_test` (Space type created for the scope check) | dev-lens (`losaka-app`) | Inactive, not deleted | Decide: delete via the admin UI, or keep as a fixture. Do not delete from SQL. | Product owner |
| 2 | `TestBrowserCollaboration` / `pilot-live` mention-delivery flake | `web/tests/pilot-live.spec.ts`, `server/internal/httpserver/collaboration_integration_test.go` | Passed 10 of 11 local runs; one failure not captured, CI passed on the last 3 heads | Capture the failing output if it recurs. Do not raise the wait again without a cause. | Engineering |
| 3 | Untracked QA evidence folders | `docs/qa/evidence/`, `docs/qa/results/`, `docs/qa/review-1/` | Not committed | Decide what to keep in git; move large screenshots out of the repo if needed. | Engineering |
| 4 | Flaky under full-suite load: `ownership.spec.ts:51`, `reports.spec.ts:169` | `web/tests/` | Each failed once in a full run; each passed when run alone (3 of 3 and 9 of 9) | Find the cause under parallel load (likely shared mocks or timing). | Engineering |
