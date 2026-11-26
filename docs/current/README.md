# Checkpoint implementasi — PMS v0.13.0

| | |
|---|---|
| **Tanggal checkpoint** | 26 November 2026 |
| **Versi** | `v0.13.0` (tag) = `origin/main` @ `72f9902` (merge PR #28, tree identik dengan kandidat `c3782a7` / `0.12.5-rc3`) |
| **Skema database** | migrasi 00001–00361 (`server/migrations/`, berjalan otomatis saat start) |
| **Penomoran versi** | tetap `0.x.y` sampai pemilik produk menyetujui MVP 1 |
| **Dokumen pendamping** | [IMPLEMENTATION_MAP.md](IMPLEMENTATION_MAP.md) (peta kode dan fitur) · [roadmap MVP](../roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md) |

Dokumen ini adalah titik awal (*checkpoint*) untuk perencanaan berikutnya. Isinya status yang **terbukti** pada v0.13.0, bukan status `DONE` di TASKS. Bila kode sudah bergerak dari `72f9902`, perbarui tabel di bawah lebih dulu.

## 1. Ringkasan

PMS adalah aplikasi project & work management self-hosted, single-tenant (satu instance per klien, D-01), dengan parity fitur terhadap katalog Wrike yang dibekukan. Milestone M0–M12 (plus M12b–M12d untuk UI) selesai di kode. Stack: Go + PostgreSQL 16 + React/Vite, satu binary dan satu image Docker.

| Area | Status v0.13.0 |
|---|---|
| Kode M0–M12, M12b/c/d | Ada di `main` |
| CI kandidat (`test`, `authorization-matrix`, `parity-e2e`) | Lulus (run 45779890214) |
| UAT RC3 (118 baris) | 105 SUCCESS · 0 FAILED · 6 BLOCKED · 7 BACKLOG |
| E2E RC3 (35 skenario) | 34 SUCCESS · 1 BLOCKED |
| Tes lokal | Go 583 tes + 282 subtes (0 gagal, 2 skip); unit web 118; Playwright 697 lulus, 46 skip |
| Defect register | 52 entri: 51 VERIFIED_FIXED, 1 DEFERRED_APPROVED (DEF-043) |
| Staging | **Belum di-upgrade** (masih 0.12.5); hasil QA 15 Nov tidak berlaku untuk v0.13.0 |
| Production | **Belum CONDITIONAL_GO**: menunggu smoke staging, backup, persetujuan |
| Review kode manusia | Di-*waive* pemilik produk (26 Nov); hanya review AI |

Detail: [laporan rilis final](../qa/releases/v0.13.0/FINAL_RELEASE_REPORT.md), [keterbatasan dan penerimaan risiko](../qa/releases/v0.13.0/KNOWN_LIMITATIONS_AND_RISK_ACCEPTANCE.md), [backlog sesudah rilis](../qa/releases/v0.13.0/POST_RELEASE_IMPROVEMENT_BACKLOG.md). Bukti lengkap (xlsx, evidence) ada di `outputs/qa-final/pr28-20261126/` (lokal, tidak di git).

## 2. Yang belum terverifikasi (jangan dianggap siap)

SSO OIDC/SAML ke IdP nyata, GitHub dan Google Calendar, backend storage Pydio, Grafana dan pengiriman alert, e-mail ke provider eksternal (SPF/DKIM), backup off-host, load test, server amd64 production, TLS production.

## 3. Fakta kode yang penting untuk roadmap

Dicek langsung pada `72f9902`.

| Fakta | Lokasi | Dampak ke rencana |
|---|---|---|
| Satu backend storage per instance: `local`, `s3`, atau `pydio` | `server/internal/storage/config.go` | Evidence immutable dan backup harus memakai `local` (atau S3 dengan versioning), bukan `pydio` |
| `deploy/backup.sh` hanya mencakup dump PostgreSQL + `/data/files` | `deploy/backup.sh`, `deploy/systemd/pms-backup.timer` | Lampiran di S3/Pydio tidak ikut backup |
| Endpoint publik tanpa flag konfigurasi: dashboard, form (menerima unggahan anonim), snapshot Gantt | `dashboards_public.go:24`, `forms_public.go:29–30`, `gantt_snapshots.go:27` | Untuk rilis internal, blok prefix `/api/v1/public/` di reverse proxy |
| Keputusan akses terpusat: `visibleSpaces` / `CanOnSpace` (Space) dan `itemsCaps` (item) | `server/internal/policy/engine.go`, `policy/item.go`, `share_paths.go`, `locked.go` | Titik pasang gerbang klien per lokasi (F1) |
| Outbox transaksional `domain_events` | migrasi `00053`, `00341`; paket `events` | Diperluas untuk aggregate selain item, bukan broker baru |
| Versi lampiran immutable + SHA-256; approval terkunci ke versi file (D-82) | tabel `attachment_versions`, `approvals` | Primitive untuk snapshot evidence |
| Tidak ada tabel klien; tidak ada tipe custom field URL | — | Customer di R1 = custom field `single_select`; tautan Pydio = field text |
| Import Excel template "advanced" membawa Status, Workflow, Assignee Email, Depends On, custom field | `server/internal/projectexcel/template.go`, `xlsx.go` | RT-04 sudah terjawab di kode; cukup verifikasi di staging |
| Portfolio Gantt tidak menampilkan custom field | `web/src/routes/_app/portfolio.tsx` (`customFields={[]}`) | Monitoring per customer memakai widget dashboard, bukan Portfolio |
| `architecture_test.go` hanya menegakkan aturan paket platform | `server/architecture_test.go` | Perketat sebelum paket baru (`clients`, `registry`, `assets`) dibuat |

## 4. Keputusan yang berlaku

Register lengkap: [DECISIONS.md](../architecture/DECISIONS.md). Yang paling memengaruhi pengembangan lanjutan:

| ID | Isi singkat |
|---|---|
| D-01 | Single-tenant, satu instance per klien, tanpa `account_id` |
| D-17.1 | Explicit Access Only: Workspace Admin tidak otomatis melihat konten Shared Space |
| D-55 | Rilis 1.0 = M0–M12; Business Object Registry di M13 (setelah 1.0) |
| D-82 | Approval file terkunci per versi lampiran |
| DEC-06 Rev.1, DEC-08..10 | Aturan move/restore/assign hasil QA final (lihat dokumen keterbatasan) |
| K-1 (26 Nov, pemilik produk) | `losaka-lens` dibekukan; HazopApp hanya sumber knowledge; PMS menjadi basis aktif |

Perubahan perilaku v0.13.0 yang perlu dikomunikasikan ke pengguna: editor hanya bisa menugaskan diri sendiri; move ditolak bila membawa assignee/share ke batas akses lain tanpa hak di sumber dan tujuan; undangan hanya memuat akses yang boleh dibagikan pengundang; waktu dihitung biaya setelah Approved.

## 5. Pekerjaan terbuka yang dibawa ke rencana berikutnya

| ID | Butir | Target |
|---|---|---|
| UPG-1 | Upgrade staging ke v0.13.0 dan smoke test | R1 D1 |
| BKP-1 | Backup off-host (`BACKUP_REMOTE`) + uji restore off-host | R1 (blocker) |
| SEC-PUB | Tutup `/api/v1/public/` di proxy selama rilis internal | R1 |
| DEF-045 | Audit read-only Space Admin hasil backfill 00346 di staging/production | R1 D1 |
| RT-32, RT-15, DEF-043 | Edit booking, inheritance field, query submit timesheet | R1.1 |
| RACE-1, OBS-1, UI-1, UI-2, ACL-1, TEST-2 | Sisa race, logging 5xx, kosmetik UI, aturan assign, tes UI di CI | R1.1 |
| LOAD-1, MON-1, MAIL-1 | Load test, monitoring/alert, e-mail eksternal | ops, sebelum/sesudah production |
| SSO-1, INT-1, STO-1 | SSO, integrasi GitHub/Calendar, Pydio | saat dipakai |
