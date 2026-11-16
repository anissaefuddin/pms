# Batch 5 dependency matrix

| Work item | Depends on | Blocks | Shared risk |
|---|---|---|---|
| B1 public forms | `forms_public.go`, item ranking/transaction helpers, rate limits | security regression, matrix closure | item-create advisory lock and notification idempotency |
| B2 relations | item access policy and custom-field visibility | CUST-05 acceptance | shared authorization policy; read-only until leak reproduced |
| C1 OTP HMAC | D-97 OTP implementation and server-side secret source | AUTH-08 acceptance | configuration/startup and migration compatibility |
| C2 approvals | RT-14 decision for any policy change | approval acceptance | do not change runtime semantics before approval |
| E VIEW-04 | Batch 4 local patch | UI acceptance | dirty legacy files must be ported, not copied wholesale |
| F1 Cloudflare | tunnel/provider access and comparable origin/edge workload | operational readiness | no WAF/rate-limit weakening |
| F2/F3 DR/QA | isolated host/storage/mail/secrets and approval for configuration | destructive/security tests | never reset existing DB or reuse production providers |
| F4 CI | exact candidate commit | feature-freeze recommendation | required-check plan limitation |
| G providers | disposable credentials/resources | live acceptance | mocks are not live evidence |

