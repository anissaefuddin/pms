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
| D-16 | **P-01:** baseline M1 diubah ke **12 minggu** (opsi A); GA minggu 39; PRD §8 sudah diubah (v0.3.2). Kalibrasi ulang setelah epic E1.1; jangan menjanjikan tanggal ke klien sebelum itu | chat ("Setujui semua", 12 Nov 2026) |
| D-17 | **P-02:** admin otomatis `full` di semua Space `shared` (bukan Personal space); flag Space "restricted" dapat ditambahkan nanti tanpa memutus kompatibilitas | chat (12 Nov 2026) |
| D-18 | **P-03:** Recycle bin hanya dibersihkan manual di M1; opsi `BIN_RETENTION_DAYS` (default mati) di M6 | chat (12 Nov 2026) |
| D-19 | **P-04:** bahasa email undangan memakai `PMS_DEFAULT_LOCALE` (default `en`) | chat (12 Nov 2026) |
| D-26 | **P-05:** repo GitHub dijadikan **privat** sebelum push | chat (12 Nov 2026) |
| D-27 | Push `main` setelah D-26 dan jalankan CI GitHub; uji GHCR dengan tag `v0.1.0` terpisah | chat (12 Nov 2026) |
| D-28 | **P-06:** `docs/PRD_PMS.md` diganti nama menjadi `docs/PRD.md` (tautan sudah diperbaiki) | chat (12 Nov 2026) |
| D-29 | **G-10:** satu pemilik per direktori kerja; pemilik tercatat di HANDOVER, sesi lain tidak mengedit direktori itu selama batch | chat (12 Nov 2026) |
| D-30 | Build cache Docker dibersihkan (`docker builder prune`) | chat (12 Nov 2026) |
| D-31 | **G-14:** terima penyimpangan UI_SPEC §4.1; S0.3.2 `DONE`; `Retry-After` pada 429 ditambahkan di backend (tambahan kontrak) dan banner "Your session expired" dikerjakan di S1.5.1 | chat (12 Nov 2026) |
| D-32 | **G-12:** login menghapus sesi yang dibawa request; dikerjakan sebelum M1 (selesai, bertes) | chat (12 Nov 2026) |
| D-33 | **G-13:** URL sekali pakai di `river_job.args` diterima untuk M0; di S1.6.1 worker membuat token saat mengirim sehingga URL tidak lagi masuk argumen job | chat (12 Nov 2026) |
| D-34 | **G-07:** `account_name` ditambahkan ke `Me` (tambahan kontrak) di S1.5.1 | chat (12 Nov 2026) |
| D-35 | **P-07:** warna status vs aksen diputuskan di awal M3; usulan default: palet tetap, kategori status menentukan ikon | chat (12 Nov 2026) |
| D-36 | **M2-1:** prioritas disimpan sebagai kolom `importance` (`high`/`normal`/`low`) pada `items` | chat (13 Nov 2026), [M2_PLAN.md](M2_PLAN.md) |
| D-37 | **M2-2:** penjadwalan otomatis berjalan saat tanggal atau dependensi berubah, dalam satu transaksi; tidak ada tombol "jadwal ulang semua" di M2 | chat (13 Nov 2026), [M2_PLAN.md](M2_PLAN.md) |
| D-38 | **M2-3:** penerus digeser otomatis hanya bila melanggar dependensi, tidak pernah dimajukan | chat (13 Nov 2026), [M2_PLAN.md](M2_PLAN.md) |
| D-39 | **M2-4:** satu work schedule per workspace; per user menunggu M4 | chat (13 Nov 2026), [M2_PLAN.md](M2_PLAN.md) |
| D-40 | **M2-5:** nilai custom field disimpan di `items.fields` berkunci id definisi; kolom tampil bila definisi ada di Space yang sedang dilihat | chat (13 Nov 2026), [M2_PLAN.md](M2_PLAN.md) |
| D-10 | Cakupan detail design dan backlog saat ini: **M0 + M1** (47 requirement) | chat ("lanjut kerjakan m1") |

## 2. Disetujui 12 Nov 2026 (sebelumnya PROPOSED, dokumen buatan AI)

| ID | Keputusan | Lokasi |
|---|---|---|
| D-20 | Arsitektur, model otorisasi (ceiling user type ∩ share role, union lokasi), SSE, `Storage`, jobs | [ARCHITECTURE.md](ARCHITECTURE.md) |
| D-21 | Skema database, soft delete bertingkat, optimistic locking (`If-Match`), migrasi forward-only | [DATABASE.md](DATABASE.md) |
| D-22 | State machine dan kode error stabil | [WORKFLOWS.md](WORKFLOWS.md) |
| D-23 | Kontrak API M0+M1 (52 path, 67 operasi) | [../api/openapi.yaml](../api/openapi.yaml) |
| D-24 | Design system (token `ui-kit`, kepadatan compact, Base UI) dan spesifikasi layar | [DESIGN_SYSTEM.md](DESIGN_SYSTEM.md), [UI_SPEC.md](UI_SPEC.md) |
| D-25 | Backlog 41 story dan urutan kerja | [BACKLOG.md](BACKLOG.md), [TASKS.md](TASKS.md) |

**Seluruh D-20..D-25 DISETUJUI user** ("Setujui semua", 12 Nov 2026), dengan catatan: D-21 menyimpang pada UUID v4 untuk tabel M0 dan `groups` ditunda ke S1.1.3; D-23 menerima tambahan yang sifatnya menambah (`Retry-After`, 429 pada reset/accept, `account_name` di `Me`), sedangkan mengubah atau menghapus tetap butuh persetujuan; D-24 dengan penyimpangan D-31; D-25 dengan baseline D-16.

## 3. PENDING (perlu keputusan user)

Tidak ada keputusan yang menunggu user. Semua P-01..P-08 ditutup pada 12 Nov 2026:

| ID | Hasil |
|---|---|
| P-01 | Disetujui → D-16 |
| P-02 | Disetujui → D-17 |
| P-03 | Disetujui → D-18 |
| P-04 | Disetujui → D-19 |
| P-05 | Disetujui → D-26 |
| P-06 | Disetujui → D-28 |
| P-07 | Ditunda ke awal M3 → D-35 |
| P-08 | Selesai: kode M0 sudah di-commit (lima commit lokal); push → D-27 |

## 4. Temuan konflik dokumen vs kode

| ID | Temuan | Tindakan |
|---|---|---|
| G-01 | `docs/` belum punya `TESTING.md`/`HANDOVER.md`/`DECISIONS.md` sesuai standar | Dibuat sesi ini |
| G-02 | Kode M0 (`server/`, `web/`, `deploy/`) ditulis sesi lain dan **belum ter-commit**; tidak ada catatan handover sebelumnya | Diverifikasi sebagian, lihat TESTING.md |
| G-03 | ~~Kode M0 menyimpang dari kontrak OpenAPI~~ **SELESAI** (D-11). Path, status code, payload snake_case, `problem+json`, `/me`, `/healthz`, `/readyz`, dan penerimaan undangan kini mengikuti kontrak; skrip deploy memakai `/api/v1/readyz`. Dibuktikan oleh `TestAuthContract` | Selesai. Stack manual `pms-manual-app` masih memakai image lama; perlu rebuild agar memakai path baru |
| G-04 | `.gitignore` dimodifikasi di working tree, belum di-commit | Tunggu keputusan P-08 |
| G-05 | ~~Panjang minimum password PRD 6 vs kontrak 8~~ **SELESAI**: PRD AUTH-03 diubah ke 8 dan SEC-04 (ganti password: session lain dicabut) diselaraskan dengan kontrak, sesuai D-11 (PRD v0.3.1, 12 Nov 2026) | Selesai; tolong tinjau teks PRD |
| G-06 | ~~Skema DB M0 menyimpang dari DATABASE.md/D-01~~ **SELESAI** (D-13, 12 Nov 2026): baseline ditulis ulang menjadi `00001_extensions`, `00002_identity`, `00003_spaces_workflows` — singleton `account(id=1)`, tanpa `account_id`, `users.status/user_type`, `first_name/last_name`, session idle+absolut, `user_tokens`, `statuses` bertoken warna. Direncanakan di [MIGRATION_PLAN_G06.md](MIGRATION_PLAN_G06.md), direview independen dua kali, diterapkan ke stack lokal dengan backup dan rollback teruji | Penyimpangan yang diterima: UUID v4 (bukan v7) di tabel M0, `groups` ditunda ke S1.1.3, kolom profil `company/country/location/phone` ditunda ke S1.6.2 |
| G-07 | ~~`Me` tidak memuat nama workspace~~ **DITERIMA** (D-34): `account_name` ditambahkan di S1.5.1 | Dikerjakan di S1.5.1 |
| G-08 | ~~Common password dan rotasi token session~~ **SELESAI** (D-14): 10 password umum ditolak setelah trim dan lowercase (422); ganti password mencabut sesi lain, mengganti token sesi pemanggil tanpa memperpanjang batas absolut, dan mencabut link reset; panjang password dihitung per karakter | Daftar hanya 10 entri (`shortcut:` di kode); daftar lebih besar bila klien meminta |
| G-09 | ~~Kode web memakai Tailwind 3.4 dan Radix, dokumen menetapkan Tailwind v4 dan Base UI~~ **SELESAI** (D-12, 12 Nov 2026): web kini Tailwind 4.2.2 (plugin Vite, token `@theme inline` dari `ui-kit`), Base UI 1.3.0, font Inter dan JetBrains Mono lokal (`@fontsource-variable`), komponen `button`, `input`, `dialog`, `dropdown-menu`, `popover`, `toaster` disalin dari `apps/ui-kit` | Selesai; `tailwind.config.ts` dan PostCSS dihapus, dependensi `@radix-ui/*`, `autoprefixer`, `postcss` dihapus |
| G-10 | ~~Dua sesi menulis ke file yang sama~~ **DITERIMA** (D-29): satu pemilik per direktori, dicatat di HANDOVER | Berlaku mulai sekarang |
| G-11 | ~~INSIDEN disk `/Volumes/MyHDD` penuh, Docker I/O error~~ **TERATASI** (12 Nov 2026): user mengosongkan disk (kini ±11 GB bebas) dan me-restart Docker Desktop; Postgres uji pulih otomatis lewat replay WAL tanpa kehilangan data; semua image dan volume lain terlihat utuh; satu image Grafana yang terpotong saat disk penuh (binary 20 MiB) dihapus dan ditarik ulang | **Risiko tersisa:** `Docker.raw` (48 GB) tetap di disk yang hampir penuh (96%); build cache Docker 20 GB (13,5 GB dapat dibersihkan lewat `docker builder prune`, belum dijalankan sesi ini) |
| G-12 | ~~Login tidak menghapus sesi yang dibawa~~ **SELESAI** (D-32): login menghapus sesi yang dibawa request; tes `login while holding a session` (mutasi → FAIL) | Selesai |
| G-13 | URL sekali pakai di `river_job.args` dan backup. **DITERIMA untuk M0, perbaikan dijadwalkan** (D-33): di S1.6.1 worker membuat token saat mengirim; retensi job selesai 1 jam tetap berlaku | Dikerjakan di S1.6.1 |
| G-14 | ~~Penyimpangan UI_SPEC §4.1~~ **DITERIMA** (D-31): 429 tanpa waktu (menunggu `Retry-After`) dan banner sesi kedaluwarsa dijadwalkan di S1.5.1; S0.3.2 `DONE` | `Retry-After` dan banner dikerjakan di M1 |
