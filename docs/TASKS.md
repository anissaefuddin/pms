# TASKS — PMS (M0 + M1)

| | |
|---|---|
| **Versi** | 0.2 (kolom Status ditambahkan; lihat [HANDOVER.md](HANDOVER.md)) |
| **Fungsi** | Urutan kerja berdasarkan dependensi dan daftar tugas teknis per story. Kriteria penerimaan lengkap ada di [BACKLOG.md](BACKLOG.md); dokumen ini tidak mengulanginya |
| **Aturan ukuran** | Satu tugas ≤ 0.5 hari ideal. Tugas yang lebih besar dipecah sebelum dikerjakan |
| **Cara dipakai** | Kerjakan story sesuai urutan di §1. Saat sebuah story dimulai, jadikan tugasnya **GitHub Issues** (satu issue per story, tugas sebagai checklist, label `M0`/`M1` dan nama epic) agar progres terhubung ke commit dan PR. Centang di file ini hanya untuk gambaran awal; sumber kebenaran progres adalah issue |

---

## 1. Urutan eksekusi (dihasilkan dari dependensi)

**Status:** `DRAFT` `PROPOSED` `APPROVED` `IN_PROGRESS` `BLOCKED` `DONE`. `DONE` hanya dengan bukti di [TESTING.md](TESTING.md). `DONE` per 12 Nov 2026: S0.1.1, S0.2.1, S0.2.4, S0.4.1, S0.4.2, S0.5.1. Delapan story M0 lain masih `IN_PROGRESS` dengan kesenjangan yang tercatat di TESTING.md.

Urutan topologis dengan satu developer (tidak paralel), 5 hari kerja per minggu. **Jadwal ini akan bergeser**; kalibrasi ulang setelah M0 selesai.

<!-- SCHEDULE:START -->
| Urutan | Story | Judul | Hari | Hari ke- | Minggu | Status |
|---|---|---|---|---|---|---|
| 1 | S0.1.1 | Kerangka repo dan alur kontrak API | 1.5 | 0–1.5 | 1 | DONE |
| 2 | S0.1.2 | Tooling database dan migrasi identitas | 1.5 | 1.5–3 | 1 | DONE |
| 3 | S0.1.3 | Dockerfile, CI, dan image di GHCR | 1.5 | 3–4.5 | 1 | DONE |
| 4 | S0.2.1 | Platform HTTP | 1 | 4.5–5.5 | 2 | DONE |
| 5 | S0.2.2 | Login, logout, dan session | 1.5 | 5.5–7 | 2 | DONE |
| 6 | S0.2.6 | Email (SMTP + job) | 1 | 7–8 | 2 | DONE |
| 7 | S0.2.3 | Reset dan ganti password | 0.5 | 8–8.5 | 2 | DONE |
| 8 | S0.2.4 | Undangan dan penerimaan undangan | 1 | 8.5–9.5 | 2 | DONE |
| 9 | S0.2.5 | Perintah bootstrap | 0.5 | 9.5–10 | 2 | DONE |
| 10 | S0.3.1 | Setup frontend | 1 | 10–11 | 3 | DONE |
| 11 | S0.3.2 | Layar autentikasi dan shell kosong | 1 | 11–12 | 3 | DONE |
| 12 | S0.4.1 | Compose klien, update, dan backup | 1.5 | 12–13.5 | 3 | DONE |
| 13 | S0.4.2 | Observabilitas | 0.5 | 13.5–14 | 3 | DONE |
| 14 | S0.5.1 | Spike Gantt (SVAR) dan penjadwalan di Go | 1 | 14–15 | 3 | DONE |
| 15 | S1.1.1 | Data Space dan CRUD | 2 | 15–17 | 4 | DONE |
| 16 | S1.1.2 | Paket `policy` (mesin otorisasi) | 3 | 17–20 | 4 | DONE |
| 17 | S1.1.3 | API sharing dan ShareDialog | 3 | 20–23 | 5 | DONE |
| 18 | S1.2.1 | Skema item dan workflow default | 2 | 23–25 | 5 | DONE |
| 19 | S1.2.2 | Item service (create, get, patch) | 4 | 25–29 | 6 | DONE |
| 20 | S1.1.4 | Matriks tes otorisasi | 2 | 29–31 | 7 | IN_PROGRESS |
| 21 | S1.2.3 | Endpoint daftar (children dan root Space) | 2 | 31–33 | 7 | DONE |
| 22 | S1.2.4 | UI pembuatan project, folder, dan task | 2.5 | 33–35.5 | 8 | DONE |
| 23 | S1.2.5 | Panel detail item | 3 | 35.5–38.5 | 8 | DONE |
| 24 | S1.2.6 | Subitem | 1 | 38.5–39.5 | 8 | DONE |
| 25 | S1.3.1 | API view | 1.5 | 39.5–41 | 9 | DONE |
| 26 | S1.3.2 | Table view | 3.5 | 41–44.5 | 9 | DONE |
| 27 | S1.3.3 | Board view | 2.5 | 44.5–47 | 10 | DONE |
| 28 | S1.3.4 | ViewTabs dan pengelolaan view | 1.5 | 47–48.5 | 10 | DONE |
| 29 | S1.4.1 | Komentar dan mention | 2.5 | 48.5–51 | 11 | DONE |
| 30 | S1.4.2 | Lampiran file | 2.5 | 51–53.5 | 11 | DONE |
| 31 | S1.4.3 | Optimistic locking end-to-end | 1 | 53.5–54.5 | 11 | DONE |
| 32 | S1.4.4 | Realtime (SSE) | 2 | 54.5–56.5 | 12 | DONE |
| 33 | S1.5.1 | Sidebar dan navigasi | 1.5 | 56.5–58 | 12 | DONE |
| 34 | S1.5.2 | Notifikasi dan Inbox | 2.5 | 58–60.5 | 13 | DONE |
| 35 | S1.5.3 | My to-do | 1 | 60.5–61.5 | 13 | DONE |
| 36 | S1.5.4 | Pencarian global | 2.5 | 61.5–64 | 13 | DONE |
| 37 | S1.6.1 | Manajemen user dan seat | 3 | 64–67 | 14 | DONE |
| 38 | S1.6.2 | Profil dan pengaturan workspace | 1 | 67–68 | 14 | DONE |
| 39 | S1.7.1 | Soft delete, restore, dan Recycle bin | 2.5 | 68–70.5 | 15 | DONE |
| 40 | S1.8.1 | Tes E2E dan aksesibilitas | 2 | 70.5–72.5 | 15 | DONE |
| 41 | S1.8.2 | Performa dan kesiapan pilot | 1.5 | 72.5–74 | 15 | DONE |
<!-- SCHEDULE:END -->

## 2. Jalur kritis

```mermaid
flowchart LR
    A[S0.1.1 Scaffold] --> B[S0.1.2 DB] --> C[S0.2.1 HTTP] --> D[S0.2.2 Login]
    D --> E[S0.2.4 Undangan] --> F[S0.2.5 Bootstrap] --> G[S1.1.1 Space]
    G --> H[S1.1.2 Policy] --> I[S1.2.2 Item service]
    I --> J[S1.2.3 Daftar] --> K[S1.3.2 Table / S1.3.3 Board]
    I --> L[S1.2.5 Panel detail] --> M[S1.4.1 Komentar] --> N[S1.5.2 Inbox]
    K --> O[S1.8.1 E2E] --> P[S1.8.2 Pilot]
    N --> O
```

Hal yang paling menentukan jadwal: **S1.1.2 (policy)** dan **S1.2.2 (item service)**. Keduanya dipakai hampir semua story berikutnya dan kesalahan desain di sini mahal diperbaiki; sediakan waktu review ekstra dan tulis tesnya lebih dulu.

## 3. Risiko jadwal yang perlu dipantau

| Risiko | Sinyal awal | Tindakan |
|---|---|---|
| Estimasi M1 terlalu rendah | Story M0 melewati estimasi > 20% | Pakai kecepatan nyata untuk menghitung ulang M1 sebelum S1.1.1 dimulai (lihat BACKLOG, bagian selisih) |
| `policy` ternyata lambat | p95 daftar > 300 ms pada data uji (S1.2.3) | Aktifkan rencana tabel closure (ARCHITECTURE §13) |
| SVAR tidak cukup | Hasil spike S0.5.1 negatif | Putuskan SVAR PRO atau library lain **sebelum** M2 direncanakan |
| Bug popup Base UI | Dialog/menu gagal di browser nyata | Uji klik nyata di Playwright sejak S0.3.2 (DESIGN_SYSTEM §4.3) |

---

# 4. Tugas per story

## M0

### E0.1 Fondasi repo dan pipeline

**S0.1.1 Kerangka repo dan alur kontrak API**
- [x] Inisialisasi `server/` (go mod, `cmd/pms`), `web/` (Vite + TS), `Makefile` (`dev`, `gen`, `test`, `lint`)
- [x] Salin `api/openapi.yaml` (sudah ada) dan pasang `oapi-codegen` (konfigurasi strict server) + `openapi-typescript`
- [x] Tes `go list` untuk aturan ketergantungan paket; langkah CI "generate lalu `git diff --exit-code`"
- [x] Halaman hello memanggil `/healthz` lewat klien hasil generate

**S0.1.2 Tooling database dan migrasi identitas**
- [ ] Pool `pgx`, konfigurasi `sqlc` (`queries/`, `gen/`), runner goose tertanam di `pms migrate`
- [ ] Migrasi `0001_extensions` dan `0002_identity` (`account`, `users`, `sessions`, `user_tokens`)
- [ ] `readyz` memeriksa versi migrasi terbaru; tes migrasi maju pada database kosong di CI
- [ ] Helper tes: database sementara per paket tes (template DB + clone)

**S0.1.3 Dockerfile, CI, dan image di GHCR**
- [x] Dockerfile multi-stage (build web → build Go dengan `go:embed` → image minimal, user non-root)
- [x] Workflow Actions: lint, typecheck, `go test` + service Postgres, build; cache modul
- [x] Job rilis pada tag `v*` → push `ghcr.io/anissaefuddin/pms:<versi>` (private)
- [ ] Verifikasi: image berjalan dan melayani SPA + `/api/v1/healthz`

**S0.2.1 Platform HTTP**
- [ ] Router `net/http` + middleware: request ID, log `slog`, recover, batas body, cek `Origin`
- [ ] Penulis error `problem+json` + pemetaan `code` (WORKFLOWS §10)
- [ ] Loader konfigurasi env dengan validasi dan pesan galat jelas
- [ ] Tes middleware (Origin salah → 403, panic → 500 dengan `request_id`)

**S0.2.2 Login, logout, dan session**
- [ ] Hash argon2id + kebijakan password (≥ 8, daftar password umum)
- [ ] Tabel/query session: buat, rotasi, perbarui `last_seen` (≤ 1×/5 menit), cabut
- [ ] Endpoint `login`/`logout`/`me` + middleware autentikasi
- [ ] Rate limit memori per IP dan per email (+ komentar `shortcut:`)
- [ ] Tes: kedaluwarsa idle/absolut, pesan error seragam, rotasi ID

**S0.2.3 Reset dan ganti password**
- [ ] `forgot` (202 seragam) dan `reset` (token sekali pakai, cabut semua session)
- [ ] `change` (cabut session lain, rotasi session ini) + tes

**S0.2.4 Undangan dan penerimaan undangan**
- [ ] Pembuat token undangan + `GET /auth/invitations/{token}`
- [ ] `accept`: aktivasi, hash password, buat Personal space, buat session
- [ ] Tes: token kedaluwarsa, token terpakai (410), password lemah (422)

**S0.2.5 Perintah bootstrap**
- [ ] `pms bootstrap`: account, admin `invited`, workflow dan status default, Space "General"
- [ ] Idempoten + kode keluar ≠ 0 bila admin sudah ada; cetak link bila SMTP kosong
- [ ] Tes dua kali jalan

**S0.2.6 Email (SMTP + job)**
- [ ] Pasang River + migrasi River; klien SMTP; mode `log`
- [ ] Job `send_email` dengan retry; template `en` (undangan, reset)
- [ ] Tes dengan server SMTP tiruan

**E0.3** · **S0.3.1 Setup frontend**
- [ ] TanStack Router/Query, react-i18next (`en`), Tailwind v4, `data-brand`/`data-density`, mode gelap
- [x] Salin komponen `ui-kit`; ganti `LinkButton` ke adapter TanStack; font lokal
- [x] ESLint `no-literal-string`; Playwright + `axe` terpasang

**S0.3.2 Layar autentikasi dan shell kosong**
- [ ] Login, Forgot, Reset, Accept invitation, `/settings/security`
- [ ] Shell: sidebar + topbar + menu profil/logout; `?next=` hanya path internal
- [ ] Tes Playwright klik nyata untuk menu profil

**E0.4** · **S0.4.1 Compose klien, update, dan backup**
- [ ] `deploy/docker-compose.yml` + `.env.example` + label Traefik
- [ ] `update.sh` (backup pre-deploy → pull → up → tunggu `/readyz`)
- [ ] `backup.sh` (dump lalu file, retensi 14 hari, salin keluar) dan `restore.sh`
- [ ] Uji restore penuh pada instance uji; catat waktu terhadap RPO/RTO

**S0.4.2 Observabilitas**
- [ ] Metrik ARCHITECTURE §11 di `/metrics`; metrik `backup_last_success_timestamp` dari script
- [ ] Provisioning alert Grafana (5xx, job gagal, backup basi, disk)

**E0.5** · **S0.5.1 Spike Gantt dan penjadwalan**
- [x] Halaman SVAR dengan ±200 task dan dependensi dummy
- [x] Fungsi Go geser penerus FS dengan hari kerja Senin–Jumat + 3 tes
- [x] Tulis catatan keputusan di `docs/`

## M1

### E1.1 Space dan otorisasi

**S1.1.1 Data Space dan CRUD**
- [ ] Migrasi `spaces`, `shares`, `groups`, `group_members` + trigger validasi target share
- [ ] Query sqlc dan service Space; endpoint `/spaces*`
- [ ] Personal space saat aktivasi; guard hapus/bagikan

**S1.1.2 Paket `policy`**
- [x] Definisi tunggal matriks (share role, user type, kemampuan) sebagai data
- [x] Query CTE rekursif akses (leluhur lewat semua lokasi + share user/grup)
- [x] `Can()` + filter SQL untuk daftar; aturan 404 vs 403
- [x] Tes unit tabel-driven untuk seluruh kombinasi matriks

**S1.1.3 API sharing dan ShareDialog**
- [x] `GET/PUT` shares Space dan item (+ share diwarisi dengan asal)
- [x] ShareDialog (cari, role, peringatan hapus akses sendiri)
- [x] Alur "undang sebagai external" dari dialog (admin)

**S1.1.4 Matriks tes otorisasi**
- [x] Matriks 5 tipe × 4 role × 4 jalur untuk GET, PATCH, create subtask, dan DELETE Item pada endpoint nyata
- [x] Tes batas visibilitas Item dan Item di Personal Space
- [x] Tes batas visibilitas file, komentar, dan lampiran
- [ ] Jadikan langkah CI yang memblokir merge. GitHub menolak akses pengaturan required check pada paket repo privat saat ini (HTTP 403).

### E1.2 Item inti

**S1.2.1 Skema item dan workflow default**
- [ ] Migrasi `workflows`, `statuses`, `items`, `item_locations`, assignees, followers
- [ ] Trigger anti-siklus + tes; indeks DATABASE §4
- [ ] Seed workflow default lewat bootstrap

**S1.2.2 Item service**
- [x] `POST/GET/PATCH/DELETE /items` + optimistic locking (`If-Match`, 409/428)
- [x] `activity` dalam transaksi yang sama; follower otomatis
- [x] Aturan status (`completed_at`), validasi tanggal, assignee aktif
- [x] `reorder` dengan `rank` fractional
- [x] Tes konflik versi dan rollback ketika penulisan activity gagal

**S1.2.3 Endpoint daftar**
- [x] `/spaces/{id}/items` dan `/items/{id}/children` (filter, sort, kursor)
- [x] Data uji 500 item; p95 80 ms pada HTTP lokal dengan 200 Item per halaman

**S1.2.4 UI pembuatan project, folder, dan task**
- [x] Picker Space/container, kalender hari kerja, owner, default view, dan sharing di modal Create Item
- [x] Create task dari tombol **+** global. Entry point Table dan Board mengikuti S1.3.2–S1.3.3
- [x] Validasi nama dan pemetaan error server terstruktur ke field

**S1.2.5 Panel detail item**
- [x] Panel samping dengan `?task=` dan halaman `/tasks/$itemId`
- [x] StatusPicker, AssigneePicker, dan kalender hari kerja
- [x] Judul inline, deskripsi TipTap tersimpan saat kehilangan fokus, dan menu ⋯
- [x] Kontrol berdasarkan `my_capabilities`, PATCH dengan ETag, dan tes browser

**S1.2.6 Subitem**
- [x] Daftar, "Add subitem", dan navigasi kembali ke induk
- [x] Jumlah subitem pada baris yang tersedia. Form membuat Item baru dengan parent tetap, dan siklus lokasi diuji pada PostgreSQL
- [x] Tampilkan jumlah subitem pada kartu Board. Playwright memverifikasi angka pada kartu

### E1.3 View

**S1.3.1 API view**
- [ ] Migrasi `views`; CRUD, duplicate, default unik per scope
- [ ] Pembuatan view bawaan saat container dibuat

**S1.3.2 Table view**
- [ ] DataTable (TanStack Table + Virtual) dengan hierarki expand/collapse
- [ ] Edit inline sel; baris "+ Item"
- [ ] Reorder drag + "Move to…"; navigasi keyboard
- [ ] Filter/sort/group di URL; ukur 500 item

**S1.3.3 Board view**
- [ ] Kolom dari workflow; BoardCard (dnd-kit) + sensor keyboard
- [ ] Drag ubah status optimistis + rollback + toast
- [ ] "+ Add" per kolom

**S1.3.4 ViewTabs dan pengelolaan view**
- [ ] Menu per view (copy link, rename, duplicate, default, urut, hapus)
- [ ] Tes membuka URL tersalin memulihkan tampilan

### E1.4 Kolaborasi

**S1.4.1 Komentar dan mention**
- [x] Skema `comments`, `comment_mentions`; validator JSON ProseMirror (allowlist node/mark)
- [x] Endpoint komentar + ekstraksi mention
- [x] CommentThread + editor mention; tes XSS

**S1.4.2 Lampiran file**
- [x] Interface `Storage` + `LocalStorage` (+ komentar `shortcut:`)
- [x] Upload: batas ukuran, deteksi tipe, tolak executable, SHA-256, nama acak
- [x] Download berizin + header aman; FileDropzone + batas ukuran dan pesan penolakan
- [x] Tes file palsu (`.png` berisi ELF)

**S1.4.3 Optimistic locking end-to-end**
- [x] ConflictBanner + draf lokal + Reapply dan Discard; rollback dan toast untuk status
- [x] Tes E2E dua sesi

**S1.4.4 Realtime (SSE)**
- [x] Hub memori, heartbeat, `Last-Event-ID`, `resync`
- [x] Publikasi event setelah commit; izin penerima diperiksa saat pengiriman dan replay
- [x] `lib/realtime.ts` + invalidasi query + indikator koneksi
- [x] Tes E2E ≤ 3 detik

### E1.5 Navigasi dan pencarian

**S1.5.1 Sidebar dan navigasi**
- [x] Pohon Space/project/folder, Personal, Shared with me (`/me/shared`), serta rute Recycle bin melalui More. Daftar dan restore Recycle bin mengikuti S1.7.1.
- [x] Ciut/lebar per user, navigasi keyboard, dan Space terakhir dengan fallback Personal
- [x] `account_name` di `Me`, banner sesi kedaluwarsa, dan `Retry-After` pada 429 autentikasi

**S1.5.2 Notifikasi dan Inbox**
- [x] Migrasi `notifications`; pembuat notifikasi mengikuti WORKFLOWS §6
- [x] Inbox dengan filter Unread only, Mark all as read, Archive all, dan badge realtime
- [x] Job email transaksional untuk event selain komentar biasa

**S1.5.3 My to-do**
- [x] `GET /me/todo` dan UI dengan filter, sort, pagination, dan perubahan status ber-ETag

**S1.5.4 Pencarian global**
- [x] Kolom `tsvector` + indeks GIN; query berfilter akses
- [x] CommandPalette + `/search`; scope Everywhere, Space, dan Project
- [x] p95 HTTP lokal 46,18 ms pada 10.000 Item dan 500 komentar

### E1.6 Admin user

**S1.6.1 Manajemen user dan seat**
- [x] Invite/resend/deactivate/reactivate/ubah type + guard `last_admin` + hitung seat
- [x] UI Users + Groups + ringkasan seat; matriks `/settings/users/types`
- [x] Tes `seats_exhausted`, `last_admin`, pencabutan sesi, dan pembuatan token oleh worker undangan

**S1.6.2 Profil dan pengaturan workspace**
- [x] `GET/PATCH /me`; halaman profil, density, tema, dan pintasan komentar
- [x] Format tanggal, angka, zona waktu, dan awal minggu diterapkan di UI

### E1.7 Recycle bin

**S1.7.1 Soft delete, restore, dan Recycle bin**
- [x] Hapus kaskade dengan `deleted_root_id`; restore + fallback ke root Space
- [x] Halaman Recycle bin + toast Undo 8 detik
- [x] Item terhapus dikecualikan dari pencarian/to-do/notifikasi; tes multi-lokasi

### E1.8 Hardening dan kesiapan pilot

**S1.8.1 Tes E2E dan aksesibilitas**
- [x] 5 alur Playwright dengan PostgreSQL (login, buat proyek, buat task, drag status, komentar+mention → Inbox)
- [x] `axe` pada layar utama terang/gelap; workflow CI setiap PR

**S1.8.2 Performa dan kesiapan pilot**
- [x] Data uji 500/10 ribu item; p95 dicatat di TESTING
- [x] [Runbook pilot](PILOT_RUNBOOK.md) mencakup bootstrap, update, backup, restore, rotasi kunci, dan seat
- [x] Triase pilot dan latihan rollback DB + file; regresi otorisasi dan pemulihan lulus pada fixture
