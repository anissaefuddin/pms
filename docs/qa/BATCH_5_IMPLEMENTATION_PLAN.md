# Batch 5 implementation plan

Baseline: PR #15 / `qa/integration-b3` @ `32fb112`.

| Order | Track | Work | Gate |
|---|---|---|---|
| 1 | A | freeze baseline, counts, dependencies and file ownership | complete before implementation |
| 2 | B | reproduce/fix public-form concurrency; verify relation authorization and remaining backend locks/errors | focused Go tests on isolated test DB |
| 2 | C | replace OTP verifier with keyed construction; add concurrent single-use test; audit approval without changing RT-14 policy | security-focused Go tests |
| 2 | E | preserve and later port Batch 4 VIEW-04 patch; run accessibility/UI evidence available locally | no shared edits during B/C |
| 2 | F/G | read-only operational/live-integration audit; run only safe local checks | external mutations require approval |
| 3 | Integration | reconcile lane diffs, port VIEW-04, regenerate only if contracts changed | clean candidate ancestry and migration order |
| 4 | Verification | full Go, typecheck, lint, build, Playwright, security/performance targets | exact candidate SHA |
| 5 | Acceptance | deploy/retest only with approval | exact deployed SHA and environment evidence |

RT-04, RT-14, RT-15 and RT-32 remain `DECISION_REQUIRED`; no implementation begins without a recorded Product Owner decision.

