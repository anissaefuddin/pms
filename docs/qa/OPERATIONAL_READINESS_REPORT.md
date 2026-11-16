# Operational readiness report — Batch 5

Status: **NOT READY** (15 November 2026).

| Area | Evidence | Status |
|---|---|---|
| Baseline CI | PR #15 @ `32fb112`; authorization-matrix, parity-e2e and full test green in run 43684066888 | CI_PASS for baseline only; Batch 5 diff not run in CI |
| Branch protection | only authorization-matrix required; `strict=false`; admins can bypass | BLOCKED pending approved compensating control/plan change |
| Deployment | documented dev-lens image is older than candidate | BLOCKED for DEV_VERIFIED |
| Cloudflare | origin healthy at much higher load; edge returns Cloudflare HTML 503 at approximately 17–64 rps depending source | FAIL/BLOCKED; tunnel/dashboard access required |
| Backup RPO | local mechanics pass; dev-lens has no proven active scheduler and observed gap was 27.97 h | FAIL against 24 h RPO |
| Restore RTO | isolated local restore-to-ready 13.92 s with counts/checksums matching | PARTIAL; no off-host/production-scale drill |
| Isolated QA | scripts and local instances exist; cloned sessions/password hashes were retained | BLOCKED pending sanitization |
| Image security | older image scan had 0 HIGH/CRITICAL | NOT RUN on exact candidate |

No WAF, tunnel, rate-limit, scheduler, provider or deployment configuration was changed.

