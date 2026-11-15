# Remaining gap register

| ID | Area | Gap/blocker | Class | Next action |
|---|---|---|---|---|
| B4-AUTH-01 | AUTH-08 | Feature-on lifecycle needs isolated mail sink | environment/evidence | run request/valid/invalid/expired/reuse/excessive/inactive/2FA/concurrent cases |
| B4-AUTH-02 | AUTH-08 | Six-digit code hashing conflicts with offline-guess-resistance wording | security decision | Security/Product Owner to accept D-97 or require stronger verifier |
| B4-AUTH-03 | AUTH-08 | No visible resend countdown; invalid and expired share a safe generic message | AC conflict | confirm distinct UI-state requirement without enabling enumeration |
| B4-VIEW-01 | VIEW-04 | New toggles need shared/personal, restricted-field and deployed probes | evidence | deploy candidate and run negative authorization/cross-user retest |
| B4-CI-01 | regression | CI run 43682109614 failed PostgreSQL browser workflows | CI | inspect/fix and obtain green candidate |
| B4-RT-04 | SPC-15 | Excel status/assignee round-trip pending | decision | Product Owner decision |
| B4-RT-14 | WF-03B/APR-01 | inaccessible approver behavior pending | decision | Product Owner decision |
| B4-RT-15 | CUST-02 | custom-field defaults/inheritance incomplete | decision/implementation | approve scope before implementation |
| B4-RT-32 | RSC-05 | booking edit/overlap warning pending | decision | Product Owner decision |
| B4-GATES | release | UAT, provider, IdP, off-host restore and GA gates remain blocked | external/ops | satisfy gate owners; do not mark PASS |

