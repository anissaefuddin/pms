# Laporan gap dan defect

Kandidat `cd85751` (terpasang `pms:a0cbad3`), dev-lens, 15 Nov 2026. Tidak ada kode diubah; perbaikan hanya diusulkan.

## Fitur belum ada (NOT_IMPLEMENTED, 11)
| Baris | Fitur | Bukti ringkas |
|---|---|---|
| REQ-008 | Login dengan one-time password | Tidak ada rute login OTP-email: OpenAPI hanya /auth/login/2fa (TOTP); probe POST /auth/login/otp dan /auth/otp = 404. docs/TASKS.m |
| REQ-032 | Kustomisasi menu blueprint: tipe item, nama, ikon | Dialog "+" (Create item) hanya memuat Name/Space/Parent/Type (Project, Folder, Task)/Workflow/Owner/tanggal/Default view; tidak ad |
| REQ-040 | Board: atur field yang tampil di kartu (cover image, days in status, l | UI Board: "Board options" hanya memuat Copy link, Rename view, Duplicate as public/personal, Set as default, Move left, Delete vie |
| GAP-06 | Blueprint versioning | POST /spaces/{id}/blueprints (name + source_item_id) creates a snapshot (201). No GET /blueprints/{id}, no PATCH/edit and no versi |
| GAP-08 | Timer | No timer/start-stop routes (/timer, /me/timer, /time-entries/timer, /time-entries/active all 404); only manual POST /time-entries  |
| GAP-09 | Recurring task and bulk updates | No recurring/bulk endpoints (/items/bulk, /bulk, /items/recurrence 404); item model has no recurrence field; Table rows have no mu |
| GAP-11 | Asset scheduling | /resource-directory contains only users, roles and skills; no equipment/asset resource type in the directory, calendar or workload |
| GAP-12 | Office/video/website proofing | Preview only returns pages for image content (PNG ready); PDF preview skipped (pdftoppm missing). No office/video/website proofing |
| GAP-13 | Branding | GET /account has no logo/theme fields; Settings > Account and Profile pages show no branding/logo option (only 'Workspace appearan |
| GAP-14 | Email-to-task | No inbound email routes (/inbound-email, /email-to-task, space email-intake/inbound all 404). |
| GAP-18 | Portfolio reporting | POST /analytics/query tasks/effort_minutes group_by project = 422 'unknown group_by for tasks' (assignee and status work); no port |

## Defect (37)
| ID | Baris | Sev | Judul |
|---|---|---|---|
| QA-D-lane2-004 | REQ-061 | High | u-pm / u-finance cannot read or set Space custom fields (budget, relation) - 422 'unknown or unavailable custom field' |
| QA-D-lane2-005 | REQ-068 | High | PATCH /statuses/{id} with required_fields [] returns 500 and cannot clear required fields |
| QA-D-lane2-011 | REQ-131 | High | Finalized approval decision can be flipped (overwrite) and desyncs item status |
| QA-D-lane3-001 | NFR-PERF | high | Folder cannot hold more than ~512 children: create returns misleading 409 |
| QA-D-lane3-007 | STAB-03 | high | CI red on candidate SHA cd85751 and required check can be bypassed by admins |
| QA-D-lane4-001 | REQ-141 | High | PDF preview and compare produce no pages: server lacks pdftoppm |
| QA-D-lane1-002 | REQ-031 | Medium | u-pm (Full pada ITA/ITA-LAB lewat share item) tidak melihat Space QA: /spaces hanya Personal, tidak bisa simpan Blueprint (404) maupun menam |
| QA-D-lane1-005 | UIP-4.1 | Medium | axe serious label-content-name-mismatch pada halaman item (/items/{id}) dan scrollable-region-focusable pada /workload dan /timesheets |
| QA-D-lane2-002 | REQ-050 | Medium | Task items expose no child_count (subitem count missing on card/panel data) |
| QA-D-lane2-003 | REQ-059 | Medium | Project progress identical for all three methods; risk always red |
| QA-D-lane2-007 | REQ-117 | Medium | Item-type defaults cannot hold custom field values; no inheritance from parent |
| QA-D-lane2-008 | REQ-070 | Medium | u-pm cannot create forms in QA | Requests or Engineering (404) |
| QA-D-lane2-010 | REQ-125 | Medium | Published automation rules stay disabled; due-date clock rule not observed firing |
| QA-D-lane3-002 | REQ-113 | medium | Concurrent creates under one parent / public-form submits fail with 409 version_conflict |
| QA-D-lane3-003 | STAB-02 | medium | Cloudflare edge returns 503 (HTML) at modest concurrency instead of app handling / 429 |
| QA-D-lane3-004 | NFR-A11Y | medium | Dark mode: unread badge fails WCAG AA contrast (1.89:1) |
| QA-D-lane3-006 | NFR-A11Y | medium | Mobile 390px: horizontal page scroll on dashboards, inbox and reports |
| QA-D-lane4-002 | REQ-140 | Medium | GET /attachments/{id}/versions/1/preview/1 returns 500 for a PDF without rendered pages |
| QA-D-lane4-003 | REQ-136 | Medium | Resource booking cannot be edited and overlap is not warned |
| QA-D-lane4-005 | REQ-146 | Medium | Deployed build routes differ from api/openapi.yaml (spec drift) |
| QA-D-lane4-007 | PAR-CAT-01 | Medium | Probe CSV and WFP_TO_PRD_MATRIX disagree on PRD IDs and scope for 40 of 106 rows |
| QA-D-lane4-008 | GAP-15 | Medium | Table quick filter does not search nested items |
| QA-D-lane1-001 | REQ-029 | Low | Fixture: u-qa tidak anggota grup QA-qa; grup QA-finance dan QA-management kosong |
| QA-D-lane1-003 | REQ-014 | Low | Fixture drift: LAB-A dan LAB-E tanpa assignee dan status New (seed: u-psi, Completed/Blocked) |
| QA-D-lane1-004 | REQ-033 | Low | Round-trip Excel tidak membawa status dan assignee |
| QA-D-lane1-006 | NFR-BROWSER | Low | /reports di viewport 390 melebar horizontal (scrollWidth 750) |
| QA-D-lane1-007 | REQ-040 | Low | Konfigurasi kartu Board (cover, ID, type, tanggal, lokasi, custom field) tidak ada di UI |
| QA-D-lane1-008 | REQ-032 | Low | Tidak ada konfigurasi nama/ikon/jenis Blueprint pada menu + |
| QA-D-lane2-001 | REQ-049 | Low | u-pm cannot create at Space root or move items (404) |
| QA-D-lane2-006 | REQ-069 | Low | Approver without item access silently dropped from auto-approval |
| QA-D-lane2-009 | REQ-074 | Low | Manage forms list ignores q and access filters server-side |
| QA-D-lane2-012 | REQ-138 | Low | Fixture actor u-pm has no access to ITA-FOLLOWUP, so cross-project scenario cannot run as pm |
| QA-D-lane3-005 | NFR-A11Y | low | Workload scrollable region not keyboard focusable |
| QA-D-lane3-008 | REQ-080 | low | Chart y-axis title and bar x-axis label clipped |
| QA-D-lane3-009 | REQ-087 | low | Unassigned bucket has blank label in assignee reports/CSV |
| QA-D-lane4-004 | REQ-142 | Low | Expiry of external file-review access cannot be tested: no link/expiry control found |
| QA-D-lane4-006 | REQ-147 | Low | Connector sync with rejected provider token returns bare 502 text |

## Parity Wrike (probe 106 WFP, `WFP_PARITY_MATRIX_QA.csv`)
Agregasi status dari baris uji terkait: {'PASS': 28, 'PARTIAL': 59, 'FAIL': 4, 'NOT RUN': 12, 'BLOCKED': 3}. 12 WFP tanpa baris uji yang dieksekusi tetap NOT RUN.
Rekonsiliasi probe vs `docs/parity/WFP_TO_PRD_MATRIX.md` (`results/lane4_parity.csv`): {'PROBE_SUBSET': 7, 'EXACT': 66, 'PROBE_SUPERSET': 27, 'DIFFERENT': 6}. Defect QA-D-lane4-007: 40 dari 106 baris berbeda ID/cakupan PRD. Probe hanya kandidat; matriks resmi repo tetap acuan. Parity tidak boleh diklaim dari 208 baris ini.

## Usulan backlog perbaikan (bukan dikerjakan di tahap ini)
1. Perbaiki 500 pada `PATCH /statuses/{id}` dengan `required_fields: []` (lane2-005) dan overwrite keputusan approval final (lane2-011).
2. Pasang `pdftoppm` (poppler) di image agar preview/bandingkan PDF jalan (lane4-001/002).
3. Akses fixture: u-pm tak bisa membuat form, melihat Space QA, atau memakai custom field Space (lane1-002, lane2-004/008).
4. Batas ±512 anak per folder dan konflik 409 pada pembuatan paralel (lane3-001/002).
5. CI utama merah dan required check bisa dilewati admin (lane3-007), edge 503 pada konkurensi sedang (lane3-003).
6. Aksesibilitas: kontras mode gelap, area gulir tak terfokus, scroll horizontal di 390 px (lane1-005/006, lane3-004/005/006).
