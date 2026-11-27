# B5 operational readiness — v0.14.0 (PROMPT #028)

Status ringkas (HEAD `ee73d93`):
- **B4 local link-first: COMPLETE dengan batas** — konfigurasi tautan mandiri, PY-10 (impor Excel dan blueprint), validasi struktural. Belum: bagian dokumen UI.
- **B5 local: NOT STARTED** (kecuali verifikasi Portfolio dan laporan).
- **OPS readiness: NO-GO.**
- **Produksi: NOT READY.**

## 1. Yang diverifikasi (tes dijalankan, 0 FAIL / 0 SKIP)

| Area | Tes | Catatan |
|---|---|---|
| Tautan standalone | `TestStandaloneLinkConfiguration`, `TestPydioLinkOriginMustBeHTTPS` | Tanpa token; token hanya untuk listing. |
| PY-10 impor Excel | `TestExcelImportCannotPersistAnInvalidPydioReference` | Kontrol negatif: pemeriksaan dimatikan → FAIL. |
| PY-10 blueprint | `TestBlueprintCannotCopyAnInvalidPydioReference` | Nilai tidak valid dilewati saat restore, tidak disalin. |
| Portfolio (carried B3) | `TestPortfolioReturnsOnlyAuthorizedProjects` | Permintaan campuran ditolak utuh; admin tanpa grant tidak mendapat konten; revocasi berlaku. Kontrol negatif FAIL. |
| Laporan (carried B3) | `TestReportResultExcludesAnotherCustomersValues` | Baris dan total Customer lain tidak muncul. |

## 2. Belum dikerjakan (blocker lokal)

- **B4 bagian dokumen di UI** (dan EN/ID) — belum.
- **B5-01 Customer turunan** di tabel dan detail project — belum.
- **B5-02 Create Item** dengan pemilihan Space dan Customer — belum.
- **B5-03 filter Customer** di tampilan monitoring — belum.
- **B5-04 Project Master** (field, inventory, reuse vs baru) — belum. Tidak ada field baru yang ditambahkan dalam pass ini.
- **B5-05 seed project-master** idempoten dan kompatibel dengan ownership — belum diverifikasi.
- **B5-06 monitoring manajemen** (Portfolio sudah diverifikasi secara keamanan; tampilan kolom belum dibuat).
- **B5-07 pilot sintetis dan rekonsiliasi** — belum.
- **B5-08 defek workflow esensial (RT-04 dsb.)** — belum ditinjau.
- **B5-09 notifikasi dan pengingat** (pengiriman SMTP live) — belum diverifikasi; tes yang ada untuk pengiriman digest dan visibilitas tetap berlaku.
- **B5-10 checklist onboarding** — dokumen di bawah.

## 3. S-curve readiness

Data yang diperlukan: tanggal rencana, effort rencana, dan progres/effort aktual. Ada: tanggal start/due, `effort_minutes`, progres di workflow. Belum: sumber progres aktual yang konsisten per project, dan keputusan apakah effort rencana diisi per project. Modul S-curve tidak dibangun.

## 4. Checklist onboarding (tidak dieksekusi)

1. Persetujuan mapping Customer (keputusan bisnis, tidak dibuat di repositori).
2. Setup membership Customer (eksplisit, tidak otomatis).
3. Grant Space dan project eksplisit.
4. Konfigurasi Project Master (setelah B5-04/05).
5. Validasi impor Excel pada data sintetis lalu data nyata yang disetujui.
6. Validasi folder Pydio: format `https://<origin>/<root>/...`, root yang disetujui.
7. Rekonsiliasi pilot dan penerimaan Project Manager (EXTERNAL).
8. Gelombang onboarding berikutnya.

Checklist ini belum dijalankan. Tidak ada data operasional yang ditulis.

## 5. Fitur opsional yang dinonaktifkan

- Dashboard, form, dan snapshot Gantt publik (`PMS_PUBLIC_LINKS`, OFF).
- Listing folder Pydio (`PMS_PYDIO_LISTING`, OFF).
- Storage Pydio ditolak saat start (`PMS_STORAGE=pydio`).
- Konektor tidak diverifikasi: tetap dinonaktifkan sebelum rilis (carried B3).

## 6. Prasyarat eksternal

Pilot operasional, Project Manager acceptance, verifikasi ACL Pydio live, migrasi operasional, backup off-host, rehearsal Docker, RPO/RTO, alert, CI remote, dan E2E terintegrasi.

## 7. Keputusan

- B4 link-first: lokal selesai dengan batas (UI belum).
- B5 lokal: belum selesai. Tidak ada gerbang B5 yang terpenuhi.
- OPS: **NO-GO.**
- Produksi: NOT READY.

## Status PROMPT #033 (2026-11-27)

- Bagian dokumen Pydio di halaman project: **selesai** (status diputuskan server; tanpa pemetaan Customer, status `roots_unavailable`). Verifikasi live: `not_configured`; Read Only dan Limited tidak diubah.
- Project Master: seed `project-code` aktif di staging; field Project Code ditugaskan ke Space QA sintetis. Seed `project-master` tercatat dan memberi flag `pydio_folder` pada Pydio Folder.
- Portfolio dan overview: filter Customer di overview Space; filter Project Manager di daftar item. **Belum** ada tampilan monitoring gabungan per project.
- Owner: 19 project aktif belum punya owner (Unassigned). Tidak ada penugasan otomatis. Daftar tindak lanjut ada di `PROJECT_ONBOARDING_CHECKLIST.md`.
- Operasi: lihat `OPS_READINESS_REPORT.md` dan `docs/ops/DEPLOY_AND_RECOVERY_RUNBOOK.md`. Off-host backup dan alert: EXTERNAL_BLOCKED.
- Pydio live: BLOCKED (akun dan folder uji, pemetaan root Customer).
