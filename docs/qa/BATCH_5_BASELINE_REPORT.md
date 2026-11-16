# Batch 5 baseline report

Date: 15 November 2026 (Asia/Jakarta).

## Canonical integration baseline

The canonical code baseline is PR #15, branch `qa/integration-b3`, commit `32fb1121b37910c9a97a753d59f05358214b67e1` in clean worktree `/Volumes/MyHDD/homeserver/apps/pms-b3-int`.

Reasons:

- it contains `origin/main` (`cd85751`) and the integrated Batch 1–3 remediation chain;
- PR #15 is OPEN and MERGEABLE against `main`;
- authorization-matrix, parity-e2e and full `test` checks are green on the exact PR head (run 43684066888; image job skipped because this is not a release tag);
- the active `/Volumes/MyHDD/homeserver/apps/pms` checkout is `main` at `247ca12`, 125 commits behind `origin/main`, with extensive unrelated uncommitted changes, so it is not safe as an integration base.

Batch 4 VIEW-04 changes exist only in the dirty legacy checkout and are not yet part of the canonical candidate. They must be ported after independent Batch 5 lanes finish.

## Deployment and evidence baseline

- The most recent documented deployed image is `pms:9e78660` on dev-lens for the Batch 3 retest; other documents still mention `pms:a0cbad3`/`cd85751` and are historical.
- No Batch 4 or Batch 5 deploy is authorized or claimed.
- Candidate SHA and deployed SHA therefore differ. Local/CI evidence must not be reported as DEV_VERIFIED.

## 208-scenario baseline

CSV recount on 15 November 2026: PASS 70, PARTIAL 107, FAIL 2, NOT_IMPLEMENTED 8, BLOCKED 20, NOT RUN 1; total 208. These statuses remain historical evidence until individual assertions are rerun.

## Governance findings

- Pending product decisions: RT-04, RT-14, RT-15 and RT-32. Prompt recommendations are not approvals.
- RT-08 is closed by D-99. RT-07 is fixed locally but not integrated/deployed.
- Required external/operational blockers remain: Cloudflare edge 503, automated backup/RPO evidence, sanitized isolated QA, provider credentials, IdP and off-host restore.
- No commit, merge, push, deploy or external configuration mutation is authorized in Batch 5 execution.

