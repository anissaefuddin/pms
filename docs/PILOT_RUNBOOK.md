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

## Update ke M3 (migrasi 00024–00033)

M3 menambah migrasi maju sampai `00033`. Nomor `00029` tidak dipakai (celah ini disengaja, bukan migrasi yang hilang). Tidak ada migrasi turun: jalan mundur hanya memulihkan backup pra-update. Jalankan `./backup.sh` dan periksa arsipnya sebelum `./update.sh`.

| Migrasi | Isi | Dampak ke data M2 |
|---|---|---|
| `00024` workflow editor | `status_transitions`, flag status, nama workflow unik | transisi bebas seperti sebelumnya sampai admin mengunci |
| `00025` approval | `approvals`, `approval_approvers` | tidak ada |
| `00026`, `00027` form | `request_forms`, `form_submissions`, kolom `access` | tidak ada; form baru berstatus `private` |
| `00028` 2FA | `user_totp`, kode cadangan, `security_events` | 2FA nonaktif untuk semua user |
| `00030` sidebar | `users.sidebar_hidden` | semua entri sidebar tampil |
| `00031` | `activity.auto`, indeks Stream | aktivitas lama dianggap manual |
| `00032` blueprint | `blueprints` | tidak ada |
| `00033` form publik | `request_forms.public_token`, kolom pemohon pada `form_submissions`, dan **satu akun placeholder** `external-requester@system.invalid` (status `deactivated`, tanpa password, tanpa seat) | tidak ada tautan publik sampai admin membuatnya |

Konfigurasi baru: `PMS_2FA_KEY` (lihat bagian kunci 2FA di bawah) wajib diisi sebelum user bisa mengaktifkan 2FA. Tautan form publik memakai `PUBLIC_URL` untuk alamat dalam email, dan `TRUST_PROXY=true` bila di belakang Traefik agar batas laju per IP memakai IP asli pengunjung. Tanpa `TRUST_PROXY`, semua pengunjung publik terlihat dari IP proxy dan berbagi satu batas 30 pengajuan per jam.

Setelah update, periksa berurutan: readiness; login; Settings → Workflows memuat workflow lama tanpa kunci; buka satu item yang punya status dan ubah statusnya; Settings → Profile menampilkan bagian Sidebar; halaman Stream terbuka; buat form kecil, bagikan secara internal, isi sebagai user biasa, dan lihat My requests. Untuk form publik: buat tautan di dialog Share, buka di jendela tanpa sesi, kirim, lalu pastikan item muncul dengan pembuat "External requester" dan email konfirmasi terkirim (atau tercatat di log bila SMTP kosong). Ganti tautan dan pastikan yang lama menghasilkan "form link is not available".

Hal yang perlu diketahui operator: akun placeholder tidak muncul di daftar user dan tidak memakai seat; jangan diaktifkan atau dihapus. Pengisi publik tidak punya akun, jadi email konfirmasi dan email perubahan status adalah satu-satunya kabar yang mereka terima. Rollback: sama seperti M2, restore backup pra-update dengan image M2. Data M3 (form, approval, tautan publik) hilang bersama restore. Update M3 dari instance M2 belum dilatih pada instance pilot.

## Update ke M5 (migrasi 00034–00043)

M5 menambah migrasi maju sampai `00043` (M4 memakai `00034`–`00037`). Tidak ada migrasi turun: jalan mundur hanya memulihkan backup pra-update, jadi jalankan `./backup.sh` dan periksa arsipnya sebelum `./update.sh`.

| Migrasi | Isi | Dampak ke data M3 |
|---|---|---|
| `00034`–`00037` | time entry, view Timelog, ekspor dan impor Excel | tidak ada |
| `00038` | `dashboards`, `widgets` | tidak ada |
| `00039` | indeks `item_locations(space_id,item_id)` dan `items(due_date)` | membangun indeks; pada data besar butuh beberapa detik |
| `00040`, `00042` | `reports`, `report_subscriptions` (+ `last_sent_at`) | tidak ada |
| `00041` | batasan `widgets.config` | tidak ada |
| `00043` | `item_stars` | tidak ada |

Tidak ada variabel konfigurasi baru. Langganan report dikirim lewat SMTP yang sama dan memakai `PUBLIC_URL` untuk tautan; tanpa `SMTP_HOST` email hanya dicatat di log. Job `report_digest` berjalan tiap 15 menit di proses `serve` (satu proses River).

Setelah update, periksa berurutan: readiness; login; More → Dashboards terbuka; buat dashboard, tambah widget daftar (template Overdue) dan pie, lalu muat ulang; More → Reports, buat report dari template Active tasks by assignee, ekspor CSV; berlangganan pada jam berikutnya di zona waktu Anda dan pastikan email tiba (atau tercatat di log); beri bintang pada satu task lalu buka More → Starred. Rollback: restore backup pra-update dengan image M3; dashboard, report, dan bintang yang dibuat sesudah update ikut hilang. Update M3 ke M5 belum dilatih pada instance pilot, dan pengiriman email langganan belum diuji ke SMTP sungguhan.

## Menghentikan instance (USR-04)

Instance diakhiri oleh operator, tidak pernah dari aplikasi: tidak ada tombol atau endpoint hapus akun. Urutannya tetap:

1. Hentikan pemakaian dan beri tahu pengguna. Jalankan `./export.sh` dari `deploy`. Hasilnya `exports/pms-export-<timestamp>.tar.gz` berisi `database.dump`, `files.tar.gz`, `users.csv`, `spaces.csv`, `items.csv`, `README.txt` dan `SHA256SUMS`. Serahkan ke klien lewat saluran aman dan minta konfirmasi bahwa berkasnya terbuka (`sha256sum -c SHA256SUMS` setelah diekstrak).
2. Jalankan `./backup.sh` untuk backup akhir dan simpan setnya (dump, arsip file, manifest) sampai klien mengonfirmasi.
3. Jalankan `./decommission.sh exports/pms-export-<timestamp>.tar.gz <nilai PMS_HOST>`. Script menolak berjalan bila bundel hilang, rusak, atau tidak memuat `database.dump`, `files.tar.gz` dan `SHA256SUMS`, atau bila nilai `PMS_HOST` tidak diketik ulang dengan benar. Setelah lolos, ia menjalankan `docker compose down --volumes`, yang menghapus kontainer, volume database, dan volume file. Tidak ada undo.
4. Hapus rekaman DNS dan router Traefik, cabut kunci `PMS_2FA_KEY`, dan hapus `deploy/.env` bila klien tidak lagi memerlukannya. Bundel export berisi data klien: perlakukan sebagai rahasia dan hapus salinan Anda sesuai kesepakatan.

CSV hanya salinan yang dapat dibaca; dump database adalah arsip lengkap. Password dan secret 2FA tidak ada di CSV. Dump memuat hash password dan secret 2FA terenkripsi, tetapi tanpa `PMS_2FA_KEY` secret itu tidak terbuka (lihat bagian kunci 2FA).

## Backup dan restore

- Jadwalkan `./backup.sh` setiap hari. Periksa pasangan dump database, arsip file, dan manifest dengan timestamp yang sama. Atur `BACKUP_REMOTE` ke tujuan off-server dan lakukan latihan restore pada instance terisolasi. Catat waktu mulai, waktu instance siap, serta hasil unduh lampiran.
- Restore mengganti data instance. Hentikan pekerjaan pengguna dan pilih timestamp serta tag image dari manifest. Jalankan `./restore.sh YYYYMMDDTHHMMSSZ --force` dari `deploy`. Script memeriksa arsip, memulihkan database ke scratch, lalu menyimpan salinan volume file sebelum cutover.
- Setelah restore, periksa readiness, login, dan unduhan lampiran. Jika cutover gagal, script mencoba mengembalikan database dan file asli. Jika pemulihan balik gagal, pertahankan app dalam keadaan berhenti dan simpan nama database serta path arsip rollback dari output sampai kedua penyimpanan diperiksa bersama.
- Target operasional OPS-01 adalah RPO ≤ 24 jam dan RTO ≤ 4 jam. Waktu aktual diukur pada latihan restore per instance. Latihan 13 Nov 2026 pada data seukuran pilot (25 user, 10.000 item, 500 lampiran, 51,6 MB file): backup 2,8 detik, restore penuh pada stack bersih 44 detik. Ulangi latihan ini pada instance klien dengan data aslinya sebelum GA dan tiap bulan.

## Kunci 2FA (`PMS_2FA_KEY`)

Secret TOTP disimpan terenkripsi (AES-256-GCM) dengan kunci dari `PMS_2FA_KEY` (32 byte acak, base64). Tanpa kunci ini server tetap berjalan, tetapi user tidak bisa mengaktifkan 2FA (`/me/2fa` melapor `configured: false`).

- **Membuat kunci:** `openssl rand -base64 32`. Simpan di secret store, bukan di repositori. Kehilangan kunci berarti semua secret TOTP tidak terbaca: user perlu direset oleh admin (lihat di bawah).
- **Backup:** kunci tidak ikut backup database. Simpan terpisah dari dump dan arsip berkas; dump tanpa kunci tidak membuka secret.
- **Pemulihan (restore):** sebelum menyatakan restore berhasil, pastikan `PMS_2FA_KEY` di `deploy/.env` instance tujuan sama dengan kunci instance asal. Restore tanpa kunci yang sama membuat setiap user 2FA tidak bisa login dengan kode, dan semuanya perlu direset admin. Catat lokasi salinan kunci (secret store, bukan folder backup) di serah terima klien. Saat server mulai tanpa `PMS_2FA_KEY`, log memuat peringatan `PMS_2FA_KEY is not set`; cari baris itu setelah setiap update atau restore.
- **Rotasi:** (1) isi `PMS_2FA_KEY_PREVIOUS` dengan kunci lama dan `PMS_2FA_KEY` dengan kunci baru di `deploy/.env`; (2) `docker compose up -d app`; (3) setiap secret dipindah ke kunci baru saat pemiliknya berhasil login dengan 2FA; (4) setelah semua user 2FA pernah login, atau setelah batas waktu yang Anda tetapkan, kosongkan `PMS_2FA_KEY_PREVIOUS`. Secret yang belum dipindah tidak terbaca setelah langkah 4 dan pemiliknya perlu direset.
- **Pemulihan user (kehilangan app dan kode cadangan):** admin memanggil `POST /users/{id}/2fa/reset` (belum ada tombol di UI). Server menghapus secret dan kode cadangan, mengeluarkan user dari semua sesi, dan mencatat `2fa_reset_by_admin` di tabel `security_events` beserta id admin dan waktu. Verifikasi identitas user di luar aplikasi sebelum reset. Setelah itu user login dengan password dan dapat mendaftar ulang 2FA.
- **Riwayat perubahan:** `SELECT at, actor_name, action FROM audit_events WHERE target_id = '<user id>' AND action LIKE 'auth.2fa%' ORDER BY id DESC;` atau halaman Settings → Security audit (`auth.2fa_enabled`, `auth.2fa_disabled`, `auth.2fa_backup_codes_renewed`, `auth.2fa_reset_by_admin`; baris sebelum migrasi `00300` ada di `security_events`).

## Single sign-on OIDC (`PMS_SSO_KEY`)

Client secret penyedia SSO disimpan terenkripsi (AES-256-GCM) dengan `PMS_SSO_KEY` (32 byte acak, base64; `openssl rand -base64 32`). Tanpa kunci, halaman Settings → Single sign-on tidak dapat menyimpan secret. Simpan kunci di luar backup dan repositori; rotasi: isi `PMS_SSO_KEY_PREVIOUS` dengan kunci lama dan `PMS_SSO_KEY` dengan yang baru, lalu simpan ulang secret di halaman itu. Langkah pasang: (1) buat aplikasi OIDC di penyedia dan daftarkan alamat "Redirect URI" persis seperti yang ditampilkan halaman (`PUBLIC_URL` + `/api/v1/auth/sso/oidc/callback`); (2) isi Issuer, Client ID, dan Client secret, simpan dulu tanpa mengaktifkan; (3) aktifkan: server memeriksa discovery sebelum menerima; (4) masuk dari jendela lain dengan tombol baru. Login password tetap berjalan sehingga admin tidak terkunci bila penyedia bermasalah. Pengguna dibuat lewat undangan; login SSO pertama tertaut ke pengguna aktif dengan email terverifikasi yang sama. Setiap penolakan tercatat di Settings → Security audit (`sso.login_failed`, alasan di `details`).

## Single sign-on SAML

Settings → Single sign-on → SAML 2.0. Berikan ke administrator penyedia identitas: Entity ID, alamat ACS (binding HTTP-POST), dan alamat metadata yang ditampilkan halaman (`PUBLIC_URL` + `/api/v1/auth/sso/saml/metadata`). Tempel XML metadata penyedia (EntityDescriptor lengkap) di kolom metadata; server memeriksa entity ID, layanan HTTP-Redirect https publik, dan sertifikat penandatangan. Persyaratan di penyedia: tanda tangan pada assertion atau response, NameID `persistent` atau `emailAddress` (bukan `transient`), `AudienceRestriction` berisi Entity ID, atribut email (`email`, `mail`, atau nama yang diisi di halaman), dan jam server yang sinkron (toleransi 3 menit). Tidak didukung: assertion terenkripsi, permintaan bertanda tangan, dan sign-in yang dimulai penyedia (IdP-initiated); mulai dari tombol di halaman login. **SAML hanya bekerja lewat HTTPS** (`COOKIE_SECURE=true`) karena jawabannya adalah POST lintas situs yang membutuhkan cookie `SameSite=None; Secure`. Rotasi sertifikat penyedia: tempel metadata baru. Setiap penolakan tercatat di Settings → Security audit (`sso.login_failed`); alasan rinci ada di log server, bukan di layar.

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

## Provisioning SCIM

Settings → Provisioning (admin). Buat token (nama = penyedia, tipe pengguna baru bukan admin), salin nilainya saat itu juga (tidak dapat dibaca lagi), lalu isi di penyedia identitas: **SCIM base URL** `PUBLIC_URL` + `/api/v1/scim/v2`, autentikasi *Bearer token*, atribut `userName` = email, `name.givenName`, `name.familyName`, `externalId`, `active`, dan grup (`displayName`, `members`). Okta, Entra ID, dan OneLogin menemukan akun lewat `GET /Users?filter=userName eq "..."`; akun yang sudah ada (undangan atau kata sandi) dicocokkan lewat email lalu ditandai dengan `externalId`. Orang yang dibuat lewat SCIM tidak punya kata sandi dan masuk lewat SSO (aktifkan OIDC atau SAML dengan penautan email otomatis agar masuk pertama mereka tertaut). Mencopot orang di penyedia (nonaktif atau hapus) mencabut semua sesinya dan membebaskan seat saat itu juga; mengaktifkannya lagi memakai seat bila masih ada. **Administrator tidak dapat dikelola lewat SCIM**; ubah tipenya di Settings → Users. Token bocor: cabut di halaman yang sama (berlaku seketika). Semua perubahan lewat SCIM tercatat di Settings → Security audit dengan aktor `scim:<nama token>`. Batas: 600 permintaan per menit per token.
