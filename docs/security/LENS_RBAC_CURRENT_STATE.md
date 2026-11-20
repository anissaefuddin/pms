# LENS RBAC — Kondisi Saat Ini (baseline audit)

- Baseline: branch `feat/m12c-ui-clean`, commit `f429094` (remote sudah sinkron).
- Working tree saat audit: 62 entri perubahan belum di-commit dari sesi lain. Tidak ada yang diubah oleh audit ini kecuali satu file tes baru (lihat `LENS_RBAC_REMEDIATION_PLAN.md`).
- Metode: pembacaan kode (`server/internal/policy`, `spaces`, `items`, `httpserver`), dokumen `docs/ARCHITECTURE.md` §6, dan tiga tes: unit `policy` (lulus), integrasi `TestReplaceItemLocations` (lulus), probe `TestMoveTaskUnderOwnDescendantIsRefused` (lulus) terhadap PostgreSQL 16.15 terisolasi di container lokal.
- Tidak dijalankan: matriks otorisasi penuh (CI job `authorization-matrix`), E2E, dan skenario uji di dev-lens.

## 1. Model yang diimplementasikan (terverifikasi di kode)

| Elemen | Implementasi | Bukti |
|---|---|---|
| Share role → kemampuan | `read`: view; `limited`: + comment, change_status, attach; `editor`: + edit, create, use/create/manage blueprint; `full`: + delete, share | `policy/policy.go:98-103` |
| Plafon user type | `admin`, `regular`: semua; `external`: sampai editor; `collaborator`: sampai limited; `viewer`: view, comment | `policy/policy.go:106-112` |
| Hak efektif | union share yang berlaku ∩ plafon user type | `policy/policy.go:150-156`, `policy/engine.go:122` |
| Pewarisan item | rekursif naik lewat `item_locations` (semua lokasi), dalam Space yang sama | `policy/item.go:50-75` |
| Custom role | mask kemampuan dari `custom_roles.caps_mask`, tetap dibatasi plafon | `policy/policy.go:116`, `spaces/spaces.go:471-482` |
| Akses tanpa hak | 404; 403 hanya jika item terlihat tetapi aksi ditolak | `spaces/item_shares.go:27` (404), `:42` (403) |

## 2. Kemampuan di level workspace

| Kemampuan | Aturan di kode | Bukti |
|---|---|---|
| Kelola user (`manage_users`) | hanya `admin` | `httpserver/server.go:272` |
| Buat Space (`create_space`) | `admin` dan `regular` | `httpserver/server.go:273`, `spaces/spaces.go:159-160` |
| Endpoint admin | 19 file handler memeriksa tipe `admin` | pencarian `policy.Admin` / `UserType` di `httpserver` |

## 3. Kemampuan per level, dari kode

| Level | Yang menentukan hak | Catatan |
|---|---|---|
| Workspace | tipe user | lihat bagian 2 |
| Space shared | share Space + Space Admin eksplisit | Admin workspace tanpa share tidak punya hak implisit (lihat `LENS_SPACE_ADMIN_AUDIT.md`) |
| Space personal | hanya pemilik; admin pun tidak | `spaces/spaces.go` (spaceCaps), `engine.go:117` |
| Folder, project, task, subtask | share pada item atau leluhur dalam Space yang sama | `policy/item.go` |
| Subtask | diperlakukan sebagai item biasa, aturan parent type berlaku | `items/location.go:187-205` |

## 4. Ringkasan status

- Matriks role × user type cocok dengan spesifikasi: **CONFIRMED** (kode dan unit test).
- Pewarisan dalam Space: **CONFIRMED** (kode). Tes INH-01..INH-13 belum dijalankan satu per satu.
- Gate keseluruhan: **CONDITIONAL**. Lihat `LENS_RBAC_THREAT_MODEL.md` untuk alasan.
