# Batch 4 implementation plan

Date: 15 November 2026. Scope: Phase 1 M0–M12 only.

## Preflight

- Preserve the active dirty checkout. Local `main` is `247ca12`, 125 commits behind `origin/main` (`cd85751`); Batch 1–3 and M12 changes are split across existing worktrees and uncommitted files.
- Use `qa/integration-b3` (`828c2ef`) for read-only verification because it contains `origin/main`; do not cherry-pick the M12 feature chain into stale `main`.
- Treat deployed, local, CI and UAT evidence separately. No deploy, push, merge or production mutation is authorized.

## Confirmed work

| Track | Requirement | Action | Exit evidence |
|---|---|---|---|
| A | AUTH-08 | Verify D-97; record security/UX contract conflicts; do not build a second auth path | focused Go tests; isolated mail-sink lifecycle retest required |
| B | SPC-14 | Verify D-99 create-menu; reconcile stale decisions | focused Go tests; deployed retest required |
| C | VIEW-04 | Add missing assignee/date/priority toggles; preserve existing CIT changes | typecheck and focused Playwright |
| D | remaining partials | Carry forward only evidence-backed statuses | remaining-gap register |
| E | decisions | Close RT-08 under D-99; update RT-07; leave RT-04/14/15/32 pending | product decision register |

## Integration gates

1. Focused tests for changed code.
2. Full Go, typecheck, lint, build and Playwright on an integrated clean candidate.
3. Isolated OTP/mail-sink and destructive lifecycle tests.
4. CI green, deployed retest, then Product Owner feature-freeze review.

