# Runbook pilot M1

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

## Backup dan restore

- Jadwalkan `./backup.sh` setiap hari. Periksa pasangan dump database, arsip file, dan manifest dengan timestamp yang sama. Atur `BACKUP_REMOTE` ke tujuan off-server dan lakukan latihan restore pada instance terisolasi. Catat waktu mulai, waktu instance siap, serta hasil unduh lampiran.
- Restore mengganti data instance. Hentikan pekerjaan pengguna dan pilih timestamp serta tag image dari manifest. Jalankan `./restore.sh YYYYMMDDTHHMMSSZ --force` dari `deploy`. Script memeriksa arsip, memulihkan database ke scratch, lalu menyimpan salinan volume file sebelum cutover.
- Setelah restore, periksa readiness, login, dan unduhan lampiran. Jika cutover gagal, script mencoba mengembalikan database dan file asli. Jika pemulihan balik gagal, pertahankan app dalam keadaan berhenti dan simpan nama database serta path arsip rollback dari output sampai kedua penyimpanan diperiksa bersama.
- Target operasional OPS-01 adalah RPO ≤ 24 jam dan RTO ≤ 4 jam. Waktu aktual diukur pada latihan restore per instance.

## Rotasi PMS_SECRET_KEY

Runtime M1 belum membaca `PMS_SECRET_KEY`. TOTP dijadwalkan untuk M3. Saat key mulai dipakai, simpan versi aktif dan versi sebelumnya di secret store, gunakan prosedur migrasi key yang didukung versi rilis untuk data terenkripsi, lalu uji dekripsi serta pemulihan dari backup sebelum menghapus versi lama. Runbook rilis yang memperkenalkan key harus menetapkan perintah rotasi dan rollback yang sesuai dengan format penyimpanannya.

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
