# Dokumentasi PMS / LENS

Mulai dari sini. Untuk melanjutkan pengembangan, baca berurutan:

1. [current/README.md](current/README.md) — **checkpoint v0.13.0**: status terbukti, fakta kode penting, pekerjaan terbuka
2. [current/IMPLEMENTATION_MAP.md](current/IMPLEMENTATION_MAP.md) — peta paket backend, halaman frontend, fitur per milestone
3. [roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md](roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md) — rencana roadmap MVP (R1 → P4)
4. [architecture/DECISIONS.md](architecture/DECISIONS.md) — keputusan yang berlaku

## Struktur folder

| Folder | Isi | Dokumen kunci |
|---|---|---|
| [current/](current/) | Snapshot implementasi saat ini. Diperbarui tiap checkpoint/rilis | README, IMPLEMENTATION_MAP |
| [roadmap/](roadmap/) | Rencana ke depan | LENS_MVP_IMPLEMENTATION_PLAN |
| [roadmap/history/](roadmap/history/) | Rencana dan backlog M0–M13 yang sudah dieksekusi | TASKS, BACKLOG, TRACEABILITY_M7_M12 |
| [product/](product/) | Kebutuhan produk dan spesifikasi UI | PRD, SITEMAP, UI_SPEC, WORKFLOWS, DESIGN_SYSTEM |
| [architecture/](architecture/) | Arsitektur, skema, keputusan | ARCHITECTURE, DATABASE, DECISIONS, MIGRATION_PLAN_G06 |
| [features/](features/) | Analisa, desain, dan closeout per fitur | automation, blueprint, create-task, custom-fields, gantt, import-export, sidebar, space-management, work-area, workspace-overview |
| [parity/](parity/) | Parity Wrike: katalog, baseline, gap layar, ADR fondasi, gate report | PMS_WRIKE_PARITY_DEVELOPMENT_BASELINE, WFP_TO_PRD_MATRIX, adr/, gate-reports/ |
| [quality/](quality/) | Strategi tes, UAT, audit UI | TESTING, UAT, ui-audit/ |
| [qa/](qa/) | Workspace eksekusi QA: seed, skrip, hasil lane, defect register, laporan per rilis | DEFECT_REGISTER.csv, releases/v0.13.0/ |
| [security/](security/) | RBAC, threat model, audit izin | LENS_RBAC_CURRENT_STATE, rbac-audit/ |
| [operations/](operations/) | Rilis, deploy, pilot, handover | RELEASE_PROCESS, PILOT_RUNBOOK, HANDOVER (historis) |
| [guides/](guides/) | Panduan pengguna dan admin (English) | user/, admin/ |
| `screens/` | Screenshot referensi Wrike (lokal, di-ignore git) | — |

## Aturan menambah dokumen

- **Status saat ini** → perbarui `current/`, jangan buat laporan status baru di root.
- **Rencana baru** → `roadmap/`. Rencana yang selesai dipindah ke `roadmap/history/`.
- **Fitur** → `features/<nama-fitur>/` (analisa, desain, closeout dalam satu folder).
- **Keputusan** → tambah baris di `architecture/DECISIONS.md`; ADR panjang di `architecture/` atau `parity/adr/`.
- **Hasil QA per rilis** → `qa/releases/<versi>/` (laporan Markdown saja; xlsx dan evidence besar tetap di `outputs/`).
- Root `docs/` hanya berisi README ini.
- Repo GitHub berstatus publik: jangan menulis host internal, kredensial, atau data klien di dokumen.

## Sumber eksternal (Outline)

- [LENS Unified Platform — C-lite+ Architecture v0.2](https://docs.maloka.app/doc/lens-unified-platform-c-lite-architecture-bZPEK9VvTj)
- [Analisa ide LENS Hybrid Platform](https://docs.maloka.app/doc/analisa-ide-lens-hybrid-platform-kelayakan-kelebihan-kekurangan-dan-risiko-hdeEW5YDcj)
- [LENS Hybrid Platform Architecture v0.1](https://docs.maloka.app/doc/lens-hybrid-platform-architecture-11Oat9fnjy) (digantikan v0.2)
