# Backlog remediasi (diperbarui 15 Nov 2026)

Cabang lokal: `fix/qa-batch1-remediation` (5289ed4, b8b3239), `fix/qa-b2-a/b/c/d`, terintegrasi di `qa/integration-b2` (worktree `/Volumes/MyHDD/homeserver/apps/pms-b2-int`, hijau: Go penuh, `tsc`, Playwright mock 443 lulus). **Belum dipush, belum dimerge, belum dideploy.**

Status: FIXED = kode dan tes lulus lokal; VERIFIED = terbukti di build terdeploy; PENDING CI; DECISION = butuh keputusan Anda; NOT_A_BUG; BLOCKED.

| ID | Batch | Sev | Req | Defect | Kelas | Judul | Status |
|---|---|---|---|---|---|---|---|
| RT-01 | F | Low | REQ-029 | QA-D-lane1-001 | fixture | Fixture: u-qa tidak anggota grup QA-qa; grup QA-finance dan QA-management kosong | fixture |
| RT-02 | F | Medium | REQ-031 | QA-D-lane1-002 | fixture/policy | u-pm (Full pada ITA/ITA-LAB lewat share item) tidak melihat Space QA: /spaces ha | fixture (usulan) |
| RT-03 | F | Low | REQ-014 | QA-D-lane1-003 | fixture | Fixture drift: LAB-A dan LAB-E tanpa assignee dan status New (seed: u-psi, Compl | fixture |
| RT-04 | B2 | Low | REQ-033 | QA-D-lane1-004 | missing feature | Round-trip Excel tidak membawa status dan assignee | DECISION (butuh keputusan Anda) [lane-A] |
| RT-05 | B2 | Medium | UIP-4.1 | QA-D-lane1-005 | product (a11y) | axe serious label-content-name-mismatch pada halaman item (/items/{id}) dan scro | VERIFIED di dev-lens (pms:86c33d1) |
| RT-06 | B2 | Low | NFR-BROWSER | QA-D-lane1-006 | product (responsif) | /reports di viewport 390 melebar horizontal (scrollWidth 750) | VERIFIED di dev-lens (pms:86c33d1) |
| RT-07 | B4 | Low | REQ-040 | QA-D-lane1-007 | missing feature | Card fields tidak mengontrol assignee/tanggal/priority | FIXED_LOCAL; typecheck dan Playwright 6/6 PASS; perlu deploy/retest |
| RT-08 | B4 | Low | REQ-032 | QA-D-lane1-008 | stale evidence | Konfigurasi menu Blueprint | CLOSED / NOT_A_BUG di D-99; perlu deployed retest |
| RT-09 | F | Low | REQ-049 | QA-D-lane2-001 | fixture/policy | u-pm cannot create at Space root or move items (404) | fixture (usulan) |
| RT-10 | B2 | Medium | REQ-050 | QA-D-lane2-002 | product | Task items expose no child_count (subitem count missing on card/panel data) | NOT_A_BUG [lane-A] |
| RT-11 | B2 | Medium | REQ-059 | QA-D-lane2-003 | product | Project progress identical for all three methods; risk always red | NOT_A_BUG [lane-A] |
| RT-12 | B1 | High | REQ-061 | QA-D-lane2-004 | product | u-pm / u-finance cannot read or set Space custom fields (budget, relation) - 422 | TERVERIFIKASI di dev-lens (5289ed4) |
| RT-13 | B1 | High | REQ-068 | QA-D-lane2-005 | product | PATCH /statuses/{id} with required_fields [] returns 500 and cannot clear requir | TERVERIFIKASI di dev-lens (5289ed4) |
| RT-14 | B2 | Low | REQ-069 | QA-D-lane2-006 | product | Approver without item access silently dropped from auto-approval | DECISION (butuh keputusan Anda) [lane-A] |
| RT-15 | B2 | Medium | REQ-117 | QA-D-lane2-007 | missing feature | Item-type defaults cannot hold custom field values; no inheritance from parent | DECISION (butuh keputusan Anda) [lane-C] |
| RT-16 | F | Medium | REQ-070 | QA-D-lane2-008 | fixture/policy | u-pm cannot create forms in QA | Requests or Engineering (404) | fixture (usulan) |
| RT-17 | B2 | Low | REQ-074 | QA-D-lane2-009 | product | Manage forms list ignores q and access filters server-side | NOT_A_BUG [lane-A] |
| RT-18 | B2 | Medium | REQ-125 | QA-D-lane2-010 | product (perlu reproduksi) | Published automation rules stay disabled; due-date clock rule not observed firin | NOT_A_BUG [lane-A] |
| RT-19 | B1 | High | REQ-131 | QA-D-lane2-011 | product | Finalized approval decision can be flipped (overwrite) and desyncs item status | TERVERIFIKASI di dev-lens (5289ed4) |
| RT-20 | F | Low | REQ-138 | QA-D-lane2-012 | fixture | Fixture actor u-pm has no access to ITA-FOLLOWUP, so cross-project scenario cann | fixture |
| RT-21 | B1 | High | NFR-PERF | QA-D-lane3-001 | product | Folder cannot hold more than ~512 children: create returns misleading 409 | TERVERIFIKASI di dev-lens (5289ed4) |
| RT-22 | B5 | Medium | REQ-113 | QA-D-lane3-002 | product | Concurrent public-form submissions returned 409 | FIXED_LOCAL: 30×201 and exactly-once side effects; stress 5×; pending CI/deploy |
| RT-23 | E | Medium | STAB-02 | QA-D-lane3-003 | environment (Cloudflare edge) | Cloudflare edge returns 503 (HTML) at modest concurrency instead of app handling | blocker eksternal |
| RT-24 | B2 | Medium | NFR-A11Y | QA-D-lane3-004 | product (a11y) | Dark mode: unread badge fails WCAG AA contrast (1.89:1) | VERIFIED di dev-lens (pms:86c33d1) |
| RT-25 | B2 | Low | NFR-A11Y | QA-D-lane3-005 | product (a11y) | Workload scrollable region not keyboard focusable | VERIFIED di dev-lens (pms:86c33d1) |
| RT-26 | B2 | Medium | NFR-A11Y | QA-D-lane3-006 | product (responsif) | Mobile 390px: horizontal page scroll on dashboards, inbox and reports | VERIFIED di dev-lens (pms:86c33d1) |
| RT-27 | B1 | High | STAB-03 | QA-D-lane3-007 | test flake + konfigurasi remote | CI red on candidate SHA cd85751 and required check can be bypassed by admins | tes diperbaiki; menunggu CI hijau; enforce_admins = keputusan Anda |
| RT-28 | B2 | Low | REQ-080 | QA-D-lane3-008 | product (UI) | Chart y-axis title and bar x-axis label clipped | VERIFIED di dev-lens (pms:86c33d1) |
| RT-29 | B2 | Low | REQ-087 | QA-D-lane3-009 | product | Unassigned bucket has blank label in assignee reports/CSV | VERIFIED di dev-lens (pms:86c33d1) |
| RT-30 | B1 | High | REQ-141 | QA-D-lane4-001 | product + environment (image) | PDF preview and compare produce no pages: server lacks pdftoppm | TERVERIFIKASI di dev-lens (5289ed4) |
| RT-31 | B1 | Medium | REQ-140 | QA-D-lane4-002 | product | GET /attachments/{id}/versions/1/preview/1 returns 500 for a PDF without rendere | TERVERIFIKASI di dev-lens (5289ed4) |
| RT-32 | B2 | Medium | REQ-136 | QA-D-lane4-003 | product/missing | Resource booking cannot be edited and overlap is not warned | DECISION (butuh keputusan Anda) [lane-C] |
| RT-33 | B2 | Low | REQ-142 | QA-D-lane4-004 | test coverage gap | Expiry of external file-review access cannot be tested: no link/expiry control f | NOT_A_BUG [lane-C] |
| RT-34 | B2 | Medium | REQ-146 | QA-D-lane4-005 | documentation gap | Deployed build routes differ from api/openapi.yaml (spec drift) | FIXED (lulus lokal + terintegrasi; belum deploy; PENDING CI) [lane-C] |
| RT-35 | B2 | Low | REQ-147 | QA-D-lane4-006 | product | Connector sync with rejected provider token returns bare 502 text | FIXED (lulus lokal + terintegrasi; belum deploy; PENDING CI) [lane-A] |
| RT-36 | B2 | Medium | PAR-CAT-01 | QA-D-lane4-007 | documentation gap | Probe CSV and WFP_TO_PRD_MATRIX disagree on PRD IDs and scope for 40 of 106 rows | FIXED (lulus lokal + terintegrasi; belum deploy; PENDING CI) [lane-C] |
| RT-37 | B2 | Medium | GAP-15 | QA-D-lane4-008 | product | Table quick filter does not search nested items | FIXED lokal (3276e04, `fix/qa-b3-a`: pencarian hierarkis sisi server); belum deploy; PENDING CI |

Baru dari Batch 2: QA-D-laneD-001 (FIXED b8b3239), -002 (preview lama skipped, butuh regenerasi), -003 dan -004 (UI Low, usulan Batch 3).

Batch 3/4: laneD-003 FIXED, laneD-004 NOT_A_BUG, lane3-010 FIXED, laneD-002 VERIFIED, lane5-001 FIXED (lock reorder/move), laneD-005 dan -006 OPEN (operasional). AUTH-08 BLOCKED pada retest mail-sink/konflik keamanan; VIEW-04 FIXED_LOCAL; SPC-14 CLOSED di D-99. Cabang integrasi: `qa/integration-b3` @ `828c2ef`; perubahan VIEW-04 belum push/deploy.

Batch 5: baseline PR #15 `32fb112`. RT-22 public-form concurrency FIXED_LOCAL. AUTH-08 uses keyed HMAC and concurrent single-use passes locally but remains BLOCKED for mail-sink/deployed acceptance. VIEW-04 was ported to the canonical Board and passes full local regression. RT-04/14/15/32 remain DECISION_REQUIRED; operational blockers are in `REMAINING_BLOCKER_REGISTER.md`.
