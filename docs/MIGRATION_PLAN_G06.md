# MIGRATION PLAN — G-06 / D-13: selaraskan skema M0 ke DATABASE.md

| | |
|---|---|
| **Status** | IMPLEMENTED di working tree (belum di-commit); review #1 (rencana) selesai dan ditanggapi; review #2 (implementasi) dan penerapan ke stack lokal: lihat §8–§9 |
| **Keputusan** | D-13: selaraskan skema ke D-01 dan [DATABASE.md](DATABASE.md) sebelum M1. Singleton `account(id=1)` hanya untuk metadata workspace; **tanpa `account_id`** pada tabel domain. Rewrite baseline migration (belum ada data klien dan belum ada commit), recreate stack lokal dengan backup, review independen |
| **Kontrak yang tidak boleh berubah** | `api/openapi.yaml` (D-11). Rencana ini tidak mengubah satu pun path, status code, atau skema API |
| **Mengubah scope?** | Tidak. Hanya menyelaraskan implementasi M0 dengan dokumen yang sudah ada |

## 1. Kondisi sekarang → target

Migrasi sekarang: `server/migrations/00001_foundation.sql`.

| Objek sekarang | Masalah terhadap DATABASE.md / D-01 | Target |
|---|---|---|
| `accounts` (banyak baris, uuid) | Seharusnya satu baris | `account` (`id smallint primary key check (id = 1)`, `name`, `settings jsonb`) |
| `account_id` di `users`, `spaces`, `workflows`, `invitations` | Melanggar D-01 (single-tenant, tanpa `account_id`) | Dihapus |
| `users.name` tunggal | Kontrak `Me` memakai `first_name`, `last_name`; kode memecah di spasi (hack) | `first_name`, `last_name` |
| `users.role`, `users.active`, `users.password_set` | Seharusnya `user_type`, `status` (`invited`→`active`→`deactivated`) | `user_type`, `status`; `password_set` dihapus (invited = belum punya password) |
| `users.email text` + indeks `lower()` | Seharusnya `citext` unik | `email citext unique` |
| Tidak ada `timezone`, `date_format`, `time_format`, `week_start`, `last_login_at` | `Me` mengisi nilai tetap di kode | Kolom nyata dengan default `UTC`, `yyyy-mm-dd`, `24h`, `monday` |
| `sessions(created_at, last_seen_at, expires_at)` (expires_at = absolut) | Seharusnya idle + absolut | `expires_at` (idle, bergeser) dan `absolute_expires_at` (tetap, 30 hari) |
| `password_reset_tokens` dan `invitations` terpisah | Seharusnya satu `user_tokens` | `user_tokens(token_hash, user_id, purpose, expires_at, used_at, created_by)` |
| Undangan = baris `invitations` tanpa user | Seharusnya `users(status=invited)` + token | Undangan membuat user `invited` |
| `spaces(personal boolean, account_id)` | Seharusnya `kind` enum dan Personal unik per user | `kind space_kind`, `owner_id`, `version`, unik parsial Personal per `owner_id` |
| `workflows(account_id)`, `workflow_statuses(category text, color hex)` | Seharusnya `statuses` dengan `grp` enum dan `color` token | `workflows`, `statuses(grp status_group, color token)`; satu workflow default (unik parsial) |
| Tidak ada trigger `updated_at` | Konvensi §1 | `updated_at` + trigger pada `users`, `spaces` |

**Sengaja tidak diubah (diterima sebagai penyimpangan, dicatat):**

1. Primary key tetap `uuid default gen_random_uuid()` (v4), bukan UUID v7 dari Go. Tabel M0 bervolume rendah; tabel bervolume tinggi (`items`, `activity`) di M1 memakai v7 dari Go sejak awal. Jalur upgrade: `uuidv7()` bawaan Postgres 18 atau generator di Go.
2. `groups` dan `group_members` tetap ditunda ke S1.1.3 (dipakai sharing), bukan bagian baseline M0.
3. Tabel M1 (`items`, `item_locations`, `shares`, dst.) tidak dibuat sekarang; milik S1.1.x / S1.2.1.

## 2. Berkas migrasi baru (menggantikan `00001_foundation.sql`)

goose, forward-only, satu transaksi per berkas, mengikuti urutan DATABASE.md §6:

| Berkas | Isi |
|---|---|
| `00001_extensions.sql` | `citext`; fungsi `set_updated_at()` |
| `00002_identity.sql` | enum `user_type`, `user_status`, `token_purpose`; `account`; `users`; `sessions`; `user_tokens`; trigger `updated_at` |
| `00003_spaces_workflows.sql` | enum `space_kind`, `status_group`; `spaces`; `workflows`; `statuses` |

Alasan rewrite baseline, bukan migrasi inkremental: belum ada commit untuk kode M0, belum ada data klien, dan satu-satunya data adalah stack uji lokal. Migrasi inkremental akan membawa riwayat `account_id` dan `accounts` yang tidak dibutuhkan siapa pun. `database.Ready` menghitung berkas `*.sql` tertanam terhadap `schema_migrations`, jadi menambah berkas aman.

## 3. Perubahan perilaku yang ikut (semuanya sudah dijanjikan dokumen)

| Area | Sebelum | Sesudah | Dokumen |
|---|---|---|---|
| Bootstrap | Admin langsung aktif tanpa password, token reset 24 jam, Personal space dibuat langsung | Admin `invited` (belum ada password), token `invite` 7 hari, tautan `/invitations/<token>`; hanya Space "General" dan workflow default dibuat; Personal space dibuat saat aktivasi | WORKFLOWS §bootstrap, S0.2.5 |
| Penerimaan undangan | Membuat user baru dari baris `invitations` | Mengubah user `invited` menjadi `active` (nama, password, department, job title), membuat Personal space, membuat session | S0.2.4 |
| Login | `active = true` | `status = 'active'`; mengisi `last_login_at`; akun `invited`/`deactivated` menjawab sama dengan password salah | S0.2.2 |
| Session | Idle dihitung dari `last_seen_at` | `expires_at` bergeser maksimal sekali per 5 menit dan tidak melewati `absolute_expires_at`; kedua batas dicek | SEC-04 |
| Seat | Aktif + undangan pending; dicek saat terima undangan | `status IN ('invited','active')`; **dicek saat undang dan aktifkan kembali (S1.6.1), bukan saat terima** (invitee sudah dihitung); koreksi atas review B-1 | USR-03, WORKFLOWS §3 |
| Forgot password | User aktif | Hanya `status='active'` | S0.2.3 |
| Housekeeping | Menghapus undangan kedaluwarsa | Menghapus **token** kedaluwarsa/terpakai dan session kedaluwarsa; **tidak** menghapus user `invited` (seat tetap dipegang sampai admin membatalkan; USR-01 di M1) | DATABASE §3.1 |
| `Me` | Nilai tetap dan pemecahan nama | Membaca kolom nyata | openapi `Me` |

## 4. Kode yang disentuh

`server/migrations/*`, `server/queries/foundation.sql` (+ `sqlc generate`), `server/internal/bootstrap`, `server/internal/housekeeping`, `server/internal/httpserver/server.go` (login, requireUser, loadMe, forgot, reset, change, invitation get/accept, startSession), `server/cmd/pms/main.go` (tautan bootstrap), tes terkait, `docs/DATABASE.md` (menyelaraskan teks dengan migrasi). Web tidak berubah (alur `/invitations/<token>` sudah ada).

## 5. Rencana penerapan pada stack lokal

1. Backup: `pg_dump -Fc` database `pms` stack uji dan simpan di luar volume (rollback).
2. Hentikan `pms-manual-app`; `DROP DATABASE pms` lalu `CREATE DATABASE pms` (data uji saja: satu admin).
3. Jalankan image baru; migrasi baseline baru terpasang otomatis.
4. `pms bootstrap --account "PMS Manual" --admin-email admin@pms.local`; ambil tautan undangan dari log (SMTP kosong), terima undangan lewat API/UI dengan password uji yang sama seperti sebelumnya agar login user tidak berubah.
5. Verifikasi: `/readyz` 200, login 200, `/me` berisi `personal_space_id`, UI login dan `?next=`.
6. **Rollback:** hentikan app baru, `DROP DATABASE`, `pg_restore` dump dari langkah 1, jalankan image `pms:m0-obs`.

## 6. Rencana uji

| ID | Uji | Jenis |
|---|---|---|
| M-1 | Migrasi naik dari database kosong; `database.Ready` benar; `Ready` salah bila satu baris `schema_migrations` hilang | Integrasi |
| M-2 | Constraint: `account` hanya `id=1`; email unik tanpa membedakan huruf; satu workflow default; satu Personal space per user; `status`/`user_type` hanya nilai enum | Integrasi (INSERT yang harus gagal) |
| M-3 | Tidak ada kolom `account_id` di tabel mana pun (`information_schema`) | Integrasi |
| M-4 | Bootstrap: admin `invited`, satu token `invite` 7 hari, satu Space "General", workflow default + 5 status, **tanpa** Personal space; bootstrap kedua ditolak; dua bootstrap serentak menghasilkan tepat satu `account` | Integrasi |
| M-5 | `TestAuthContract` diadaptasi: undangan admin → login; idle dan absolut; reset; ganti password (D-14); undangan biasa; `seats_exhausted`; akun `invited`/`deactivated` dijawab sama; `/me` membaca kolom nyata | Integrasi |
| M-6 | Housekeeping: token kedaluwarsa/terpakai dan session kedaluwarsa dihapus; user `invited` tetap | Integrasi |
| M-7 | Regresi penuh `go test ./...`, `npm test`, build Docker, e2e deploy (backup/restore skema baru), uji browser login | Regresi |
| M-8 | Uji mutasi pada aturan kunci: status login, seat, idle/absolut, tanpa rotasi | Mutasi |

## 7. Risiko dan pertanyaan untuk reviewer independen

1. Apakah ada kolom atau constraint penting dari DATABASE.md §1–§3.1 dan §3.2 (spaces) yang terlewat di tabel pada §1 di atas?
2. Apakah menjadikan admin pertama `invited` dengan tautan `/invitations/<token>` konsisten dengan kontrak `getInvitation`/`acceptInvitation` dan WORKFLOWS (state bootstrap, user lifecycle)?
3. Apakah aturan seat pada penerimaan undangan benar secara semantik, dan aman terhadap penerimaan serentak (advisory lock)?
4. Apakah `expires_at` yang bergeser plus `absolute_expires_at` benar, termasuk rotasi token D-14 yang tidak memperpanjang batas absolut?
5. Adakah risiko kehilangan data atau urutan yang salah dalam penerapan stack lokal (§5), terutama rollback?
6. Apakah menunda UUID v7 dan `groups` dapat diterima, atau melanggar sesuatu yang dijanjikan DATABASE.md untuk M0?

## 8. Review independen #1 (rencana) dan tanggapan

Reviewer: agen baru tanpa konteks percakapan, hanya membaca dokumen dan kode, tidak mengubah berkas. Semua temuan dinilai; tidak ada yang ditolak tanpa alasan.

| ID | Temuan | Tanggapan |
|---|---|---|
| B-1 | Cek seat saat terima undangan salah: invitee sudah dihitung; WORKFLOWS §3 menempatkan guard di undang/aktifkan ulang | **Diterima.** Cek seat dan advisory lock dihapus dari `acceptInvitation`; tes diganti: menerima undangan tidak mengubah jumlah seat. Penegakan `MAX_SEATS` pindah ke S1.6.1 (invite) |
| B-2 | Menulis ulang `00001` dengan nomor versi goose yang sama berbahaya untuk database lama | **Diterima.** `database.Ready` kini membandingkan **setiap nomor versi** berkas tertanam; DATABASE.md §6 mewajibkan database lama dibuat ulang (jangan di-restore ke image baru). Image lama `pms:m0-obs` tidak ditimpa |
| M-1 | `first_name`/`last_name` untuk user `invited`; CHECK status/password; enum format | **Diterima.** `NOT NULL DEFAULT ''`; CHECK `active` punya password dan nama, `invited` tanpa password; CHECK `date_format`/`time_format`/`week_start`; bootstrap tidak lagi memakai nama "Administrator" |
| M-2 | Filter `purpose` dan status user pada setiap konsumsi token | **Diterima.** Semua query memfilter `purpose` dan status; konsumsi token dan perubahan user dalam satu transaksi (ditolak → token tidak terbakar). Tes silang baru: token undangan di reset, token reset di terima, token undangan milik user aktif di reset dan di terima (uji mutasi) |
| M-3 | Admin terkunci setelah undangan 7 hari kedaluwarsa | **Diterima, keputusan kecil baru (D-15 diusulkan).** `pms bootstrap` dengan email yang sama selagi admin masih `invited` menerbitkan token baru dan mencabut yang lama; selain itu tetap ditolak. `FOR UPDATE` menjaga satu token tersisa saat dijalankan serentak (uji mutasi) |
| M-4 | Bootstrap serentak hanya bergantung pada kegagalan PK mentah | **Diterima.** `INSERT … ON CONFLICT (id) DO NOTHING` dan pemetaan ke "already bootstrapped"; tes serentak (6 proses) |
| M-5 | Semantik session: cookie, CHECK, geser, SQL | **Diterima.** Cookie memakai `absolute_expires_at`; `CHECK (expires_at <= absolute_expires_at)`; geser dengan `LEAST`, paling banyak sekali per 5 menit; `last_seen_at` dihapus, `created_at` tetap; housekeeping memakai kedua batas; tes batas idle, absolut, geser, dan batas atas |
| M-6 | Indeks yang hilang; pembatalan undangan | **Sebagian.** Ditambah `sessions(user_id)`, `sessions(absolute_expires_at)`, `users(status)`, `user_tokens(user_id,purpose)`, `user_tokens(expires_at)`. GIN pencarian user ditunda ke S1.5.4 (DATABASE.md §4 diperbarui ke `first_name`/`last_name`). Pembatalan undangan = hapus baris user `invited` (`ON DELETE CASCADE`), pengecualian dicatat di DATABASE.md §3.1 |
| m-1 | Kolom spaces | **Sebagian.** `description` ditambah; CHECK satu arah (`personal` wajib `owner_id`); `deleted_at`/`created_by` ditunda ke S1.1.1 (CRUD Space) agar tidak menebak kebutuhan. General: `owner_id` NULL |
| m-2 | Nama status dan warna token | **Diterima.** Seed New/In Progress/Completed/On Hold/Cancelled dengan token `slate/blue/green/amber/red`; `UNIQUE(workflow_id, position)` dan `UNIQUE(workflow_id, name)`; CHECK daftar token tidak dibuat (palet masih bisa bertambah di M3) |
| m-3 | Template email, query sqlc, fixture tes | **Diterima.** Teks email undangan (7 hari) dan URL `/invitations/<token>`; `foundation.sql` ditulis ulang dan dipakai tes terhadap Postgres sungguhan; fixture `testdb` tidak berubah |
| m-4 | Field `Me` (`company`, `country`, `location`, `phone`); trigger `updated_at` | **Ditunda** ke S1.6.2; trigger hanya pada `users` dan `spaces` (tabel lain menambah sendiri) |
| m-5 | Wording `uuidv7()` | **Diterima.** Dikoreksi di DATABASE.md §1: Postgres 16 tidak punya `uuidv7()` |
| m-6 | Tes token lintas tujuan | **Diterima** (lihat M-2). Rotasi token saat login: setiap login membuat token acak baru (tes M-5) |
| m-7 | Higiene backup | **Diterima.** Backup di `deploy/backups/` (diabaikan git); `DROP DATABASE … WITH (FORCE)` |

Yang tidak ditemukan reviewer: perubahan path atau status code yang terlihat di `openapi.yaml`; respons login/forgot untuk `invited`/`deactivated` tetap identik; penggunaan token tunggal bersifat atomik.

## 9. Review independen #2 (implementasi) dan penerapan

Reviewer: agen baru (model terkuat) tanpa konteks percakapan, membaca berkas langsung, tidak menjalankan dan tidak mengubah apa pun. Tidak ada BLOCKER; 4 MAJOR dan sejumlah MINOR. Tanggapan:

| Temuan | Tanggapan |
|---|---|
| MAJOR: geser idle menulis ke `sessions` di setiap request pada 14 hari terakhir sesi (perbandingan memakai `now+14d`, bukan batas yang sudah dipotong absolut) | **Diperbaiki**: dibandingkan dengan `min(now+idle, absolut) − 5 menit`; tes memeriksa `xmin` baris tidak berubah pada request kedua (uji mutasi → FAIL) |
| MAJOR: reset tidak mencabut link reset lain milik user; ganti password tidak mencabut satu pun | **Diperbaiki**: kedua alur menghapus semua token `reset` user dalam transaksi yang sama; tes untuk keduanya (mutasi → FAIL) |
| MAJOR: URL sekali pakai tersimpan di `river_job.args` (retensi 24 jam, ikut backup) dan di log saat SMTP kosong | **Sebagian**: retensi job selesai diturunkan ke 1 jam. Job *discarded* tetap memakai retensi bawaan River (untuk debugging) dan backup tetap memuat args; log mode SMTP kosong adalah kriteria S0.2.6. Risiko diterima dan ditandai `shortcut:` di kode (G-13) |
| MINOR: error DB sesaat menghapus sesi dan memaksa logout | **Diperbaiki**: hanya `ErrNoRows` dan kedaluwarsa yang menghasilkan 401; error lain 500 |
| MINOR: `change password` tanpa pembatas | **Diperbaiki**: memakai pembatas kegagalan per user (5 salah → 429); tes |
| MINOR: lookup login memakai email mentah, kunci kunci akun memakai email yang dipangkas | **Diperbaiki**: keduanya memakai email ternormalisasi |
| MINOR: indeks `sessions(absolute_expires_at)` salah kolom | **Diperbaiki**: indeks `expires_at`; housekeeping cukup `expires_at < now()`; DATABASE.md diperbarui |
| MINOR: `UNIQUE (workflow_id, position)` tidak deferrable (reorder M3 gagal) | **Diperbaiki**: `DEFERRABLE INITIALLY DEFERRED` |
| MINOR: kolom `company`, `country`, `location`, `phone` | **Dicatat** di DATABASE.md §7: ditambahkan di S1.6.2 |
| MINOR: panjang password dihitung byte, bukan karakter | **Diperbaiki**: `utf8.RuneCountInString`; tes (mutasi → FAIL) |
| MINOR: `maxLength: 80` tidak ditegakkan | **Diperbaiki** di `acceptInvitation` (422); tes |
| MINOR: `Ready` menghitung baris, bukan versi berbeda | **Diperbaiki**: `count(DISTINCT version_id)`; tes duplikat baris (mutasi → FAIL) |
| MINOR: database baseline lama membuat kontainer crash-loop tanpa pesan jelas | **Diperbaiki**: `Open` menolak database yang masih punya tabel `accounts` dengan pesan "recreate"; tes |
| MINOR: tes lemah (`mustFail` tidak memeriksa constraint; reset user nonaktif; link reset kedua) | **Diperbaiki**: `mustFail` memeriksa nama constraint; tes reset untuk user nonaktif (mutasi → FAIL) dan link reset kedua |
| MINOR: login tidak menghapus sesi yang sedang dibawa (SEC-04 "rotasi saat login") | **Ditunda (G-12)**: token baru selalu dibuat server, jadi fixation tidak mungkin; sesi lama tetap berlaku sampai kedaluwarsa. Memperbaikinya mengubah tes sesi bersama; dikerjakan terpisah bila diminta |
| MINOR: timing forgot-password; XFF dipercaya mentah; 429 tanpa `Retry-After` dan belum tercantum di openapi untuk reset/accept; `errors[]` pada 422; status name case-sensitive; sesi gagal dibuat setelah accept commit | **Tidak dikerjakan sekarang**; dicatat. Kontrak (`openapi.yaml`) tidak diubah |

## 10. Penerapan ke stack lokal (12 Nov 2026)

| Langkah | Hasil |
|---|---|
| Backup | `deploy/backups/g06-pre-20261112T041946Z.dump` (30 KB, terbaca; **dry-run rollback**: dump dipulihkan ke database sementara tanpa kesalahan lalu dihapus) |
| Recreate | Container lama dihentikan dan disimpan (`pms-manual-app-prevg06`, image `pms:m0-obs`); `DROP DATABASE pms WITH (FORCE)` lalu `CREATE DATABASE`; image baru `pms:m0-g06` memasang migrasi versi 1–3 sendiri; `/readyz` 200 |
| Bootstrap | `pms bootstrap` mencetak tautan aktivasi ke log (SMTP kosong); akun "PMS Manual", admin `admin@pms.local` berstatus `invited`, Space General, 5 status |
| Aktivasi lewat UI | Halaman `/invitations/<token>` menampilkan email undangan, form diisi di browser, mendarat di `/` dengan session; `/me`: `status=active`, `personal_space_id` terisi, kolom nyata; login API 200 dengan kredensial uji yang sama seperti sebelumnya |
| Regresi | `go test ./...` (semua paket), `npm test`, typecheck, e2e deploy (backup/update/restore pada skema baru) lulus |
| Rollback bila perlu | Hentikan `pms-manual-app`, `DROP DATABASE pms WITH (FORCE)`, `pg_restore` dump di atas, jalankan image `pms:m0-obs` (jangan jalankan image baru di atas dump lama) |
