# Remaining blocker register — Batch 5

| ID | Priority | Blocker | Owner/prerequisite |
|---|---|---|---|
| B5-CI-01 | P0 | PR #15 base is green, but the uncommitted combined Batch 5 diff has no exact-candidate CI run | integration/CI |
| B5-CI-02 | P1 | branch protection is non-strict, admin-bypassable and requires only authorization-matrix | repository owner decision/plan |
| B5-OPS-01 | P0 | Cloudflare edge 503 root cause unavailable without tunnel host/dashboard evidence | infrastructure access |
| B5-OPS-02 | P0 | dev-lens backup interval exceeded 24 h; scheduler not proven active | approved scheduler configuration |
| B5-OPS-03 | P1 | off-host production-scale restore and migration recovery not run | isolated host/storage |
| B5-SEC-01 | P0 | QA clone sanitization not implemented/proven | isolated QA owner |
| B5-LIVE-01 | P1 | disposable provider, SMTP, IdP and storage credentials unavailable | provider owners |
| B5-DEC-04 | P1 | RT-04 has no Product Owner decision | Product Owner |
| B5-DEC-14 | P1 | RT-14 has no Product Owner decision | Product Owner |
| B5-DEC-15 | P1 | RT-15 has no Product Owner decision | Product Owner |
| B5-DEC-32 | P1 | RT-32 has no Product Owner decision | Product Owner |
| B5-UAT-01 | P1 | exact deployed candidate UAT not run | deployment approval and pilot users |
