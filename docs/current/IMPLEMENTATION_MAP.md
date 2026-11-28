# Peta implementasi — PMS v0.13.0

Peta kode dan fitur yang sudah ada, untuk menemukan *di mana* sesuatu dibangun dan *dokumen mana* yang menjelaskannya. Angka baris dihitung pada `72f9902` (Go non-test, termasuk kode hasil generate). Status bukti per fitur: [README.md](README.md).

## 1. Bentuk sistem

```text
Browser (React/Vite SPA, TanStack Router)
   │  REST /api/v1 (OpenAPI: api/openapi.yaml, 270 path) + SSE realtime
   ▼
Go binary `pms` (server/cmd/pms: serve | migrate | bootstrap)
   ├── httpserver   — handler HTTP, satu paket datar (±19 ribu baris, 197 file tes)
   ├── policy       — keputusan akses terpusat (Space, item, share, field, locked)
   ├── domain pkgs  — items, spaces, automation, forms, schedule, analytics, ...
   ├── events       — outbox transaksional domain_events → worker
   └── dbgen        — query sqlc (server/queries → dbgen)
   ▼
PostgreSQL 16 (migrasi goose, 00001–00361)   +   storage blob: local | s3 | pydio
```

Deployment: `deploy/docker-compose.yml` di belakang Traefik, skrip `backup.sh`, `restore.sh`, `restore-test.sh`, `update.sh`, `export.sh`, `decommission.sh`, timer systemd harian untuk backup, monitoring di `deploy/monitoring/`. Proses rilis: [RELEASE_PROCESS.md](../operations/RELEASE_PROCESS.md).

## 2. Backend — paket `server/internal/`

| Paket | Baris | Tanggung jawab |
|---|---:|---|
| `httpserver` | 19.261 | Semua handler REST, middleware auth/sesi, public routes, integrasi test utama |
| `items` | 9.482 | Item (task/project/folder), lokasi, dependensi, custom field, blueprint, calendar, recycle, import Excel, portfolio, My to-do, stream |
| `automation` | 3.114 | Mesin aturan otomasi, template, throttling, run log, scope Workspace |
| `dbgen` | 1.700 | Kode query hasil `sqlc` (jangan diedit tangan) |
| `spaces` | 1.648 | Space, keanggotaan, governance, Space Admin |
| `connectors` | 1.441 | Konektor (GitHub, Google Calendar), sinkronisasi dan konflik (DEC-INT-02) |
| `projectexcel` | 1.275 | Template dan parser Excel project (basic/advanced) |
| `analytics` | 1.015 | Query dashboard/report, `group_by` field dan Space |
| `mail` | 873 | Antrean e-mail di PostgreSQL, template, SMTP |
| `overview` | 869 | Workspace Overview |
| `policy` | 758 | `visibleSpaces`, `CanOnSpace`, `itemsCaps`, role/ceiling, field grant |
| `sso` | 696 | OIDC dan SAML |
| `forms` | 651 | Request form, routing, form publik |
| `webhooks` | 500 | Webhook keluar, validasi URL, kunci |
| `schedule` | 377 | Mesin penjadwalan (dependensi, kalender kerja) |
| `storage` | 346 | Backend blob `local`/`s3`/`pydio` |
| `views` | 289 | Saved view |
| `duereminders`, `reportdigest`, `housekeeping` | 217/192/110 | Job terjadwal |
| `events`, `realtime` | 213/60 | Outbox domain event, SSE |
| `previews` | 196 | Preview file (butuh `pdftoppm` untuk PDF) |
| `twofactor`, `security` | 176/112 | TOTP, hashing password |
| `spacetemplates` | 165 | Template Space dengan `Format`/`Version` |
| `apigen` | 55.277 | Kode OpenAPI hasil generate (jangan diedit tangan) |
| lain | — | `bootstrap`, `config`, `database`, `directory`, `notifications`, `reqid`, `buildinfo`, `testdb` |

Aturan dependensi paket: `server/architecture_test.go` (saat ini hanya aturan paket platform). Arsitektur dan tabel kapabilitas: [ARCHITECTURE.md](../architecture/ARCHITECTURE.md). Skema: [DATABASE.md](../architecture/DATABASE.md).

## 3. Frontend — `web/src/`

| Bagian | Isi |
|---|---|
| `routes/_app/` | Halaman utama: inbox, my-todo, stream, starred, search, created-by-me, shared-with-me, spaces (detail, settings, item types, forms, automation), items/tasks detail, portfolio, workload, timesheets, calendars, dashboards, reports, requests, reviews, blueprints, recycle-bin |
| `routes/_app/settings/` | 30 halaman setting: profile, account, appearance, language, security, email, work schedule/preferences, users, roles, user types, ownership, policy, audit, SSO, provisioning (SCIM), apps, developer (API key), rates, resources, spaces, item types, custom fields, workflows, blueprints, request forms, automation |
| `components/` | Komponen domain (CreateItemDialog, Custom field registry, Form renderer, File proofing, Gantt snapshot, Integration gallery, ...) |
| `components/ui/` | Design system (Button, Dialog, Popover, Toolbar, KpiCard, LoadingState, ...) — aturan: [DESIGN_SYSTEM.md](../product/DESIGN_SYSTEM.md) |
| `i18n/` | ID dan EN |
| `tests/` | 160 spec Playwright |

Sitemap berdasarkan kode: [SITEMAP.md](../product/SITEMAP.md). Spesifikasi UI: [UI_SPEC.md](../product/UI_SPEC.md).

## 4. Fitur per milestone

Rencana dan status per story: [TASKS.md](../roadmap/history/TASKS.md). Panduan pengguna/admin: [guides/](../guides/).

| Milestone | Fitur utama | Dokumen rujukan |
|---|---|---|
| M0 | Repo, pipeline CI, auth (password, undangan, reset), shell UI, job queue, observabilitas, deploy | [ARCHITECTURE](../architecture/ARCHITECTURE.md), [RELEASE_PROCESS](../operations/RELEASE_PROCESS.md) |
| M1 | Space dan otorisasi, item inti, view Table/Board/List, komentar & mention, navigasi, pencarian, admin user, recycle bin, pilot | [guides/user/items](../guides/user/items.md), [spaces-and-folders](../guides/user/spaces-and-folders.md), [PILOT_RUNBOOK](../operations/PILOT_RUNBOOK.md) |
| M2 | Dependensi dan penjadwalan, effort/progress, custom field, Gantt, Calendar | [dependencies-and-scheduling](../guides/user/dependencies-and-scheduling.md), [GANTT_SPIKE](../features/gantt/GANTT_SPIKE.md), [custom-fields](../features/custom-fields/) |
| M3 | Workflow, approval, blueprint, request form, 2FA | [WORKFLOWS](../product/WORKFLOWS.md), [features/blueprint](../features/blueprint/) |
| M4 | Time tracking, budget, export/import Excel | [features/import-export](../features/import-export/), [time-tracking](../guides/user/time-tracking.md) |
| M5 | Dashboard dan report | [reports-and-dashboards](../guides/user/reports-and-dashboards.md) |
| M6 | Onboarding, template, pengaturan, hardening rilis | [guides/admin](../guides/admin/) |
| M7 | Stabilisasi, katalog WFP, desain fondasi platform (ADR-REG, ADR-CIT, ADR-AUT, ADR-IDP) | [parity/adr](../parity/adr/), [G0](../parity/gate-reports/G0_STABILIZATION.md) |
| M8 | Custom item type, field & workflow per tipe, form & tema | [G1A](../parity/gate-reports/G1A_CONFIG.md) |
| M9 | Mesin otomasi, approval lanjutan | [features/automation](../features/automation/), [G1B](../parity/gate-reports/G1B_AUTOMATION_APPROVALS.md) |
| M10 | Workload & kapasitas, role/skill/booking, jadwal lanjutan, biaya | [G2A](../parity/gate-reports/G2A_RESOURCE_SCHEDULE_COST.md) |
| M11 | File (versi, proofing, approval per versi), analitik, API publik, webhook, konektor | [G2B](../parity/gate-reports/G2B_FILES_ANALYTICS_INTEGRATION.md), [connectors-webhooks](../guides/admin/connectors-webhooks.md) |
| M12 | Governance enterprise: SSO, SCIM, audit log, ownership, access policy, OTP login, custom role | [security/](../security/), [guides/admin](../guides/admin/) |
| M12b | Kesetaraan tampilan FE dengan layout Wrike | [parity/GAP_TAMPILAN_FE_VS_SCREEN](../parity/GAP_TAMPILAN_FE_VS_SCREEN.md), [quality/ui-audit](../quality/ui-audit/) |
| M12c | Redesign layout LENS Workspace, sidebar, Workspace Overview | [features/sidebar](../features/sidebar/), [features/workspace-overview](../features/workspace-overview/) |
| M12d | Work Area: kontrak komponen, standar interaksi | [features/work-area](../features/work-area/) |
| QA final (0.12.x → 0.13.0) | Perbaikan RBAC (assign per lokasi, transfer, share-restore, Space Admin), branding workspace, notifikasi | [security/rbac-audit](../security/rbac-audit/), [qa/releases/v0.13.0](../qa/releases/v0.13.0/) |

Di luar rilis 1.0 (M13, D-55): AI assistant, whiteboard, **Business Object Registry**, aplikasi native/offline, editor dokumen kolaboratif, manajemen kunci enterprise, bahasa selain ID/EN.

## 5. Kemampuan yang bisa dipakai ulang untuk platform LENS

Ringkasan untuk perencanaan C-lite+ ([roadmap](../roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md)).

| Kebutuhan platform | Sudah ada | Belum ada |
|---|---|---|
| Customer/klien | Custom field `single_select` tingkat Workspace + field grant | Tabel `clients`, `spaces.client_id`, membership |
| Asset register (pilot) | Custom item type, custom field per tipe, field `relation` (item→item) | Master record terpisah dari item, tipe quantity (nilai + satuan), keunikan tag |
| Revisi dokumen | `attachment_versions` (immutable, SHA-256), approval per versi | Pin versi dari record, snapshot dari Pydio |
| Event | Outbox `domain_events` (aggregate = item) | Kolom aggregate generik |
| Identitas | OIDC, SAML, SCIM, 2FA, OTP | Uji IdP nyata |
| Template | `spacetemplates` dengan `Format`/`Version` | Definisi solution statis |
| Laporan lintas klien | Portfolio, Workload, Workspace Overview, dashboard `group_by` field | Custom field di Portfolio |
