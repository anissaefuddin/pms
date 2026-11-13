# Runbook pilot M1 dan M2

## Bootstrap

1. Salin konfigurasi contoh menjadi `deploy/.env`. Atur tag image tetap, domain, `PUBLIC_URL`, kredensial PostgreSQL, `MAX_SEATS`, dan SMTP. Lindungi file konfigurasi dari pembacaan pengguna lain.
2. Siapkan jaringan eksternal Traefik sesuai `TRAEFIK_NETWORK`. Jalankan perintah berikut dari direktori `deploy`.

   ```sh
   docker compose up -d
   docker compose exec app pms bootstrap --account 'Nama workspace' --admin-email admin@example.com
   docker compose exec app wget -qO- http://localhost:8080/api/v1/readyz
   ```

3. Buka tautan aktivasi dari email, buat password admin, lalu periksa General, Personal, dan Settings → Users. Gunakan kotak surat pengujian untuk pemeriksaan SMTP. Pada fixture lokal tanpa SMTP, perintah bootstrap menulis tautan aktivasi ke output CLI.

## Update

Dari `deploy`, jalankan `./update.sh ghcr.io/anissaefuddin/pms:vX.Y.Z` pada jendela pemeliharaan. Script membuat backup sebelum menarik image dan memeriksa readiness setelah start. Validasi login, proyek, Inbox, dan unduh lampiran, lalu catat tag baru di `PMS_IMAGE`. Jika migrasi gagal, pulihkan backup pra-update dengan tag lama dari manifest. Migrasi hanya maju.

## Update ke M2 (migrasi 00015–00023)

M2 menambah sembilan migrasi maju. Semuanya aditif dan mempertahankan data M1, tetapi tidak ada migrasi turun: satu-satunya jalan mundur adalah memulihkan backup pra-update (`./restore.sh` dengan timestamp dan tag M1 dari manifest). Karena itu `./backup.sh` harus selesai dan arsipnya diperiksa sebelum `./update.sh` dijalankan, dan jendela pemeliharaan harus cukup untuk restore.

| Migrasi | Isi | Dampak ke data M1 |
|---|---|---|
| `00015` work schedule | tabel `work_schedules` (satu baris bawaan Senin–Jumat, 8 jam) dan `work_schedule_exceptions` | tidak ada |
| `00016` dependensi | tabel `item_dependencies` | tidak ada |
| `00017`, `00020` custom field | tabel `custom_fields` dan tipe lengkap | nilai disimpan di `items.fields` yang sudah ada |
| `00018` tipe view | `gantt` dan `calendar` pada enum `view_type` | view Table dan Board tidak berubah |
| `00019` effort | kolom effort dan alokasi | tanggal dan durasi lama dipertahankan |
| `00021` progres project | kolom metode dan persen | project lama memakai metode bawaan |
| `00022` | `items.status_changed_at`, diisi dari `updated_at` | "days in status" lama memakai tanggal ubah terakhir, bukan tanggal status berubah |
| `00023` | `items.importance`, bawaan `normal` | semua item menjadi Normal |

Setelah update, periksa berurutan: readiness; login; Settings → Work schedule memuat Senin–Jumat dan 8 jam; Space lama masih menampilkan Table dan Board dengan item yang sama; buka panel Item (Importance Normal, Locations berisi lokasi lama); tambah view Gantt dan Calendar lalu buka keduanya; buat satu custom field Number, isi nilai, dan tampilkan sebagai kolom; unduh satu lampiran. Pada instance dengan data besar, jalankan `./backup.sh` kedua setelah update untuk mendapat titik pulih bersih pada skema M2.

Rollback: hentikan app, `./restore.sh <timestamp pra-update> --force`, lalu mulai dengan `PMS_IMAGE` M1. Data yang dibuat setelah update (dependensi, custom field, view Gantt/Calendar) hilang bersama restore. Jika hanya satu fitur bermasalah, lebih baik tidak rollback: hentikan pemakaian view atau field yang bermasalah dan laporkan. Kompatibilitas image M1 terhadap skema M2 belum diuji, dan view bertipe `gantt` atau `calendar` kemungkinan tidak dikenal image M1, jadi jangan menjalankan image M1 di atas database M2 tanpa restore.

## Backup dan restore

- Jadwalkan `./backup.sh` setiap hari. Periksa pasangan dump database, arsip file, dan manifest dengan timestamp yang sama. Atur `BACKUP_REMOTE` ke tujuan off-server dan lakukan latihan restore pada instance terisolasi. Catat waktu mulai, waktu instance siap, serta hasil unduh lampiran.
- Restore mengganti data instance. Hentikan pekerjaan pengguna dan pilih timestamp serta tag image dari manifest. Jalankan `./restore.sh YYYYMMDDTHHMMSSZ --force` dari `deploy`. Script memeriksa arsip, memulihkan database ke scratch, lalu menyimpan salinan volume file sebelum cutover.
- Setelah restore, periksa readiness, login, dan unduhan lampiran. Jika cutover gagal, script mencoba mengembalikan database dan file asli. Jika pemulihan balik gagal, pertahankan app dalam keadaan berhenti dan simpan nama database serta path arsip rollback dari output sampai kedua penyimpanan diperiksa bersama.
- Target operasional OPS-01 adalah RPO ≤ 24 jam dan RTO ≤ 4 jam. Waktu aktual diukur pada latihan restore per instance.

## Kunci 2FA (`PMS_2FA_KEY`)

Secret TOTP disimpan terenkripsi (AES-256-GCM) dengan kunci dari `PMS_2FA_KEY` (32 byte acak, base64). Tanpa kunci ini server tetap berjalan, tetapi user tidak bisa mengaktifkan 2FA (`/me/2fa` melapor `configured: false`).

- **Membuat kunci:** `openssl rand -base64 32`. Simpan di secret store, bukan di repositori. Kehilangan kunci berarti semua secret TOTP tidak terbaca: user perlu direset oleh admin (lihat di bawah).
- **Backup:** kunci tidak ikut backup database. Simpan terpisah dari dump dan arsip berkas; dump tanpa kunci tidak membuka secret.
- **Rotasi:** (1) isi `PMS_2FA_KEY_PREVIOUS` dengan kunci lama dan `PMS_2FA_KEY` dengan kunci baru di `deploy/.env`; (2) `docker compose up -d app`; (3) setiap secret dipindah ke kunci baru saat pemiliknya berhasil login dengan 2FA; (4) setelah semua user 2FA pernah login, atau setelah batas waktu yang Anda tetapkan, kosongkan `PMS_2FA_KEY_PREVIOUS`. Secret yang belum dipindah tidak terbaca setelah langkah 4 dan pemiliknya perlu direset.
- **Pemulihan user (kehilangan app dan kode cadangan):** admin memanggil `POST /users/{id}/2fa/reset` (belum ada tombol di UI). Server menghapus secret dan kode cadangan, mengeluarkan user dari semua sesi, dan mencatat `2fa_reset_by_admin` di tabel `security_events` beserta id admin dan waktu. Verifikasi identitas user di luar aplikasi sebelum reset. Setelah itu user login dengan password dan dapat mendaftar ulang 2FA.
- **Riwayat perubahan:** `SELECT * FROM security_events WHERE target_id = '<user id>' ORDER BY id DESC;` (`2fa_enabled`, `2fa_disabled`, `2fa_backup_codes_renewed`, `2fa_reset_by_admin`).

## Menambah seat

Naikkan `MAX_SEATS` di `deploy/.env`, lalu buat ulang container app dengan `docker compose up -d app`. Periksa jumlah active, invited, dan available di Settings → Users. Undangan pending memakai seat. Nonaktifkan akun hanya melalui UI admin agar sesi dicabut dan histori tetap tersimpan.

## Triase pilot

Regresi lokal untuk policy akses, sesi, seat, restore Item, dan rollback database serta lampiran tidak menemukan blocker P0.

| Prioritas | Kondisi | Tindak lanjut |
| --- | --- | --- |
| P0 | Kebocoran akses atau kehilangan data | Tahan pilot. Catat versi, request ID, scope data, dan hasil tes yang relevan. |
| P1 | Required check otorisasi pada `main` belum bisa diwajibkan oleh paket GitHub saat ini | Verifikasi status job CI setiap PR sampai required check bisa diaktifkan. |
| P1 | ShareDialog Item, proyeksi share warisan dan integrasi undang external masih mengikuti S1.1.3 | Gunakan undangan dari Settings dan catat alur yang belum tercakup pada uji pilot. |
| P1 | SMTP instance, restore pada data pilot, dan CI remote belum diverifikasi | Jalankan pemeriksaan instance dan catat hasil sebelum membuka pilot. Restore serta rollback DB dan file sudah lulus pada Docker fixture terisolasi. |
