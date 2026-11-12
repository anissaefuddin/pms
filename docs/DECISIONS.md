# DECISIONS — PMS

| | |
|---|---|
| **Fungsi** | Daftar keputusan yang mengikat semua AI dan developer. Perubahan scope, arsitektur, atau keputusan di sini hanya dengan persetujuan user |
| **Status** | `APPROVED` = disetujui user di chat · `PROPOSED` = usulan AI, menunggu user · `PENDING` = belum ada usulan final |
| **Terakhir diperbarui** | 12 November 2026 |

## 1. APPROVED (diputuskan user)

| ID | Keputusan | Sumber |
|---|---|---|
| D-01 | Self-host, dijual sebagai instance standalone per klien (single-tenant); tanpa `account_id` | PRD §10.1 |
| D-02 | Seat dikelola manual lewat env `MAX_SEATS`; tanpa billing | PRD §10.1 |
| D-03 | Email keluar via SMTP wajib di 1.0; email-to-task dan integrasi lain P2 | PRD §10.1 |
| D-04 | i18n siap sejak awal, MVP hanya English | PRD §10.1 |
| D-05 | Backend Go (`net/http`), frontend TypeScript (React + Vite), Postgres 16, REST + `api/openapi.yaml` sebagai sumber kebenaran | PRD §10.2 |
| D-06 | Penyimpanan file: volume lokal di balik interface `Storage`; Pydio Cells / MinIO menyusul | PRD §10.2, jawaban Q13 |
| D-07 | Tim: satu developer + AI; modular monolith | PRD §10.1 |
| D-08 | Isi PRD v0.3 (116 requirement, milestone M0–M7) setelah review | PRD |
| D-09 | Repo `github.com/anissaefuddin/pms`, commit dan push atas permintaan user | chat |
| D-11 | Kontrak `api/openapi.yaml` adalah SSOT; kode mengikuti kontrak (keputusan G-03, 12 Nov 2026) | chat ("Ikuti kontrak") |
| D-12 | Stack UI: Tailwind v4 + Base UI + token dan komponen dari `apps/ui-kit` (keputusan G-09) | chat ("Migrasi UI") |
| D-13 | Keputusan G-06: selaraskan schema ke D-01/DATABASE sebelum M1. Gunakan singleton `account(id=1)` hanya untuk metadata workspace; tanpa `account_id` pada tabel domain. Rewrite baseline migration karena belum ada data klien/commit, lalu recreate stack lokal dengan backup dan review independen | chat ("putuskan untuk G-06/G-08 berdasarkan rekomendasi anda") |
| D-14 | Keputusan G-08: pertahankan kebijakan WORKFLOWS. Tolak 10 password umum dan rotasi token session caller saat change password; session lain dicabut dan absolute expiry caller tidak diperpanjang | chat ("putuskan untuk G-06/G-08 berdasarkan rekomendasi anda") |
| D-15 | **DISETUJUI user (12 Nov 2026):** `pms bootstrap` dengan email yang sama selagi admin masih `invited` menerbitkan token undangan baru (pemulihan setelah kedaluwarsa 7 hari); selain itu tetap ditolak. Berasal dari review independen #1 (temuan M-3); implementasi dan tes sudah ada | chat ("Setujui D-15"), [MIGRATION_PLAN_G06.md](MIGRATION_PLAN_G06.md) §8 |
| D-10 | Cakupan detail design dan backlog saat ini: **M0 + M1** (47 requirement) | chat ("lanjut kerjakan m1") |

## 2. PROPOSED (dokumen buatan AI, belum disetujui user)

| ID | Keputusan | Lokasi |
|---|---|---|
| D-20 | Arsitektur, model otorisasi (ceiling user type ∩ share role, union lokasi), SSE, `Storage`, jobs | [ARCHITECTURE.md](ARCHITECTURE.md) |
| D-21 | Skema database, soft delete bertingkat, optimistic locking (`If-Match`), migrasi forward-only | [DATABASE.md](DATABASE.md) |
| D-22 | State machine dan kode error stabil | [WORKFLOWS.md](WORKFLOWS.md) |
| D-23 | Kontrak API M0+M1 (52 path, 67 operasi) | [../api/openapi.yaml](../api/openapi.yaml) |
| D-24 | Design system (token `ui-kit`, kepadatan compact, Base UI) dan spesifikasi layar | [DESIGN_SYSTEM.md](DESIGN_SYSTEM.md), [UI_SPEC.md](UI_SPEC.md) |
| D-25 | Backlog 41 story dan urutan kerja | [BACKLOG.md](BACKLOG.md), [TASKS.md](TASKS.md) |

Dokumen D-20..D-25 sudah ada di `main` (commit `36d0c6e`) tetapi **belum disetujui user secara eksplisit**. Implementasi mengikutinya sebagai kontrak kerja sementara.

## 3. PENDING (perlu keputusan user)

| ID | Pertanyaan | Opsi / rekomendasi | Yang terblokir |
|---|---|---|---|
| P-01 | Estimasi M1: backlog ±59 hari vs 40 hari di PRD | A re-baseline (rekomendasi, GA ≈ minggu 39) · B pangkas cakupan (hemat ±3 hari) · C pecah M1a/M1b. Lihat BACKLOG §Gap | Perubahan PRD §8 |
| P-02 | Admin otomatis `full` di semua Space `shared`? | Ya (usulan) atau admin harus menambah diri sendiri | S1.1.2 (policy) |
| P-03 | Recycle bin dibersihkan otomatis setelah N hari? | Rancangan: hanya manual | S1.7.1 |
| P-04 | Bahasa email undangan untuk user baru | Pakai `PMS_DEFAULT_LOCALE` (usulan) | Template email undangan |
| P-05 | Visibilitas repo GitHub: saat ini **PUBLIC** | Ubah ke private bila PRD/rancangan tidak boleh terbuka | Tidak ada, tetapi bersifat segera |
| P-06 | Nama berkas PRD: standar tim `PRD.md`, aktual `PRD_PMS.md` | Rename memutus tautan di README dan dokumen lain; belum dilakukan | Kepatuhan struktur docs |
| P-07 | Status warna accent vs status (DESIGN_SYSTEM) | Diputuskan sebelum M3 | M3 |
| P-08 | Commit/push untuk kode M0 yang belum di-track (`server/`, `web/`, `deploy/`, `.github/`, dll.) | Perlu review user dulu | Handover bersih |

## 4. Temuan konflik dokumen vs kode

| ID | Temuan | Tindakan |
|---|---|---|
| G-01 | `docs/` belum punya `TESTING.md`/`HANDOVER.md`/`DECISIONS.md` sesuai standar | Dibuat sesi ini |
| G-02 | Kode M0 (`server/`, `web/`, `deploy/`) ditulis sesi lain dan **belum ter-commit**; tidak ada catatan handover sebelumnya | Diverifikasi sebagian, lihat TESTING.md |
| G-03 | ~~Kode M0 menyimpang dari kontrak OpenAPI~~ **SELESAI** (D-11). Path, status code, payload snake_case, `problem+json`, `/me`, `/healthz`, `/readyz`, dan penerimaan undangan kini mengikuti kontrak; skrip deploy memakai `/api/v1/readyz`. Dibuktikan oleh `TestAuthContract` | Selesai. Stack manual `pms-manual-app` masih memakai image lama; perlu rebuild agar memakai path baru |
| G-04 | `.gitignore` dimodifikasi di working tree, belum di-commit | Tunggu keputusan P-08 |
| G-05 | ~~Panjang minimum password PRD 6 vs kontrak 8~~ **SELESAI**: PRD AUTH-03 diubah ke 8 dan SEC-04 (ganti password: session lain dicabut) diselaraskan dengan kontrak, sesuai D-11 (PRD v0.3.1, 12 Nov 2026) | Selesai; tolong tinjau teks PRD |
| G-06 | ~~Skema DB M0 menyimpang dari DATABASE.md/D-01~~ **SELESAI** (D-13, 12 Nov 2026): baseline ditulis ulang menjadi `00001_extensions`, `00002_identity`, `00003_spaces_workflows` — singleton `account(id=1)`, tanpa `account_id`, `users.status/user_type`, `first_name/last_name`, session idle+absolut, `user_tokens`, `statuses` bertoken warna. Direncanakan di [MIGRATION_PLAN_G06.md](MIGRATION_PLAN_G06.md), direview independen dua kali, diterapkan ke stack lokal dengan backup dan rollback teruji | Penyimpangan yang diterima: UUID v4 (bukan v7) di tabel M0, `groups` ditunda ke S1.1.3, kolom profil `company/country/location/phone` ditunda ke S1.6.2 |
| G-07 | `Me` pada kontrak tidak memuat nama akun/organisasi, sehingga sidebar M0 kini menampilkan nama user dan bagian Spaces dihapus sampai S1.5.1 | Bila nama akun perlu tampil, kontrak harus ditambah (butuh persetujuan) |
| G-08 | ~~Common password dan rotasi token session~~ **SELESAI** (D-14): 10 password umum ditolak setelah trim dan lowercase (422); ganti password mencabut sesi lain, mengganti token sesi pemanggil tanpa memperpanjang batas absolut, dan mencabut link reset; panjang password dihitung per karakter | Daftar hanya 10 entri (`shortcut:` di kode); daftar lebih besar bila klien meminta |
| G-09 | ~~Kode web memakai Tailwind 3.4 dan Radix, dokumen menetapkan Tailwind v4 dan Base UI~~ **SELESAI** (D-12, 12 Nov 2026): web kini Tailwind 4.2.2 (plugin Vite, token `@theme inline` dari `ui-kit`), Base UI 1.3.0, font Inter dan JetBrains Mono lokal (`@fontsource-variable`), komponen `button`, `input`, `dialog`, `dropdown-menu`, `popover`, `toaster` disalin dari `apps/ui-kit` | Selesai; `tailwind.config.ts` dan PostCSS dihapus, dependensi `@radix-ui/*`, `autoprefixer`, `postcss` dihapus |
| G-10 | Sesi lain mengedit file yang sama (`server_integration_test.go`, `mail_test.go`, `schedule*.go`, `database.go`) bersamaan dengan sesi ini; satu blok tes saya terduplikasi dan password uji sempat bentrok sampai disatukan | Disarankan satu penulis per file; tidak ada kehilangan data yang diketahui |
| G-11 | ~~INSIDEN disk `/Volumes/MyHDD` penuh, Docker I/O error~~ **TERATASI** (12 Nov 2026): user mengosongkan disk (kini ±11 GB bebas) dan me-restart Docker Desktop; Postgres uji pulih otomatis lewat replay WAL tanpa kehilangan data; semua image dan volume lain terlihat utuh; satu image Grafana yang terpotong saat disk penuh (binary 20 MiB) dihapus dan ditarik ulang | **Risiko tersisa:** `Docker.raw` (48 GB) tetap di disk yang hampir penuh (96%); build cache Docker 20 GB (13,5 GB dapat dibersihkan lewat `docker builder prune`, belum dijalankan sesi ini) |
| G-12 | Login tidak menghapus sesi yang sedang dibawa request (SEC-04 "ID dirotasi saat login"); token baru selalu dibuat server sehingga session fixation tidak mungkin, tetapi sesi lama hidup sampai kedaluwarsa | Ditunda; perbaikan memengaruhi tes sesi bersama. Kerjakan bila diminta |
| G-13 | URL sekali pakai (undangan/reset) tersimpan di `river_job.args`; retensi job selesai 1 jam, job *discarded* dan backup tetap memuatnya; mode log SMTP kosong mencetak URL | Risiko diterima untuk M0; enkripsi args atau token pendek bila klien meminta |
| G-14 | UI menyimpang dari UI_SPEC §4.1 pada dua hal kecil: (a) pesan 429 login tanpa "dalam N menit" (server hanya mengirim 429, belum `Retry-After`); (b) banner "Your session expired" belum ada (shell tidak membedakan sesi kedaluwarsa dari kunjungan pertama). Rute undangan memakai `/invitations/<token>` mengikuti tautan email, dan UI_SPEC diperbarui | Butuh keputusan user: terima penyimpangan (rekomendasi untuk (a), karena menambah `Retry-After` menyentuh kontrak) dan kerjakan (b) di M1 dengan parameter `?reason=expired` dari guard, atau blokir S0.3.2 sampai keduanya selesai |
