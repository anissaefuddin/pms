# UI/UX audit (Batch 3, lane-B) — build pms:86c33d1, dev-lens, ITA seed data

Method: Playwright with QA admin session cookie (password not typed), 39 routes (home, inbox, my to-do, stream, starred, shared, recycle bin, search, dashboards, reports, requests, forms, reviews, timesheets, workload, portfolio, QA|Engineering Table, ITA item, task LAB-C, space forms/automation, 17 settings pages, not-found) x 1440/390 px x light/dark = 156 checks. Per check: axe wcag2a/2aa/21aa, sideways scroll, first 4 Tab stops (focus ring), 5xx API responses, raw i18n keys. Data: `evidence/ui-audit-b3/audit.json`, screenshots in the same folder (light, all routes; dashboards also dark).

## Result
- axe: 0 violations on 155 of 156 checks (one transient `html-has-lang` on the 404 page in 390 dark, not reproduced in the other three modes).
- Keyboard: Tab reaches a control with a visible ring on every route sampled (no "NORING" hit).
- Raw keys: only `/settings/sso` matched my pattern (a URL with dots; false positive).
- Not covered: manual screen-reader pass, loading/empty/error states beyond what ITA data shows, other roles (viewer, client), Firefox/Safari.

## Findings
| # | Where | Finding | Action |
|---|---|---|---|
| 1 | /settings/audit @390 | sideways scroll 397 px (table) | FIXED 8c3cf9e (scroll region) + test |
| 2 | /settings/apps @390 | tablist overflowed by 61 px | FIXED 8c3cf9e (flex-wrap) + test |
| 3 | /tasks/{id} @390 | approval Edit/Reassign/Delete row overflowed 25 px | FIXED 8c3cf9e (flex-wrap) + test |
| 4 | Approval after final | Approve/Reject stayed active (QA-D-laneD-003) | FIXED 26d4ff3 + tests |
| 5 | Subitems counter | QA-D-laneD-004 | NOT_A_BUG: after Create the page navigates to the new child (counter is the child's); parent shows N after Back; an external create updated the open parent 2->3 in <5 s via realtime |
| 6 | API via Cloudflare | 503 on /activity, /custom-fields, /relations, /events while the page loads (parallel requests) | open, known QA-D-lane3-003 / Cloudflare RCA (lane-D) |
| 7 | /nonexistent @390 dark | axe html-has-lang once | not reproduced; watch |

Deploy needed to verify 1-3 on dev-lens (code fixed locally only).
