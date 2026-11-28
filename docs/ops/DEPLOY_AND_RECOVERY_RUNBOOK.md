# Runbook deploy dan pemulihan PMS (OPS-05)

Runbook ini memakai perintah yang sudah dijalankan dan diverifikasi pada staging (PROMPT #033). Untuk migrasi ownership klien, lihat juga `CLIENT_SCOPE_RECOVERY_RUNBOOK.md`.

## 0. Aturan yang tidak boleh dilanggar

- **Migrasi maju saja.** Setelah schema naik (saat ini 364), binary lama tidak boleh dijalankan lagi di atas database itu. Binary lama tidak menegakkan kepemilikan. Pemulihan berarti: kembalikan backup pra-deploy **bersama** image lama (`deploy/restore.sh`), atau deploy perbaikan maju. Jangan melakukan "rollback" hanya dengan menjalankan image lama.
- Jangan tulis kredensial, token, tautan aktivasi, atau mapping operasional ke repositori, laporan, atau log.
- SMTP, tautan publik, listing Pydio, dan konektor yang belum terverifikasi tetap **OFF**. Periksa nilainya setelah setiap deploy (lihat §3).
- Jangan jalankan pemulihan di atas database staging yang aktif. Pemulihan uji selalu ke database sementara.

## 1. Pra-deploy

1. Verifikasi identitas lingkungan: nama host, `PUBLIC_URL`, dan `docker inspect pms-app-1` menunjukkan image yang diharapkan.
2. Buat commit lokal yang sudah diverifikasi. Catat SHA penuh (`git rev-parse HEAD`).
3. Buat backup pra-deploy dan verifikasi checksum:
   ```bash
   cd /home/deploy-user/pms && ./backup.sh
   cd backups && sha256sum -c <STAMP>-SHA256SUMS
   ```
   Backup lokal saja: `BACKUP_REMOTE` dan `BACKUP_RSYNC_DEST` belum diatur. Lihat §8.
4. Simpan `.env` dengan cap waktu: `cp -p .env .env.bak-pre-<tag>-<stamp>`.
5. Pastikan `pms_pre_candidate` (atau database pra-deploy sebelumnya) masih ada.
6. Jika deploy memuat migrasi: verifikasi dulu di clone terisolasi (lihat §5).

## 2. Build dan deploy

```bash
# dari repositori lokal: arsipkan commit ke host, build dengan tag yang terikat pada SHA
git archive <SHA> | ssh deploy-user@<host> "rm -rf ~/candidate-<SHA> && mkdir -p ~/candidate-<SHA> && tar -x -C ~/candidate-<SHA>"
ssh deploy-user@<host> "cd ~/candidate-<SHA> && docker build -q --build-arg VERSION=0.14.0-candidate.<SHA> --build-arg COMMIT=<SHA> -t pms:candidate-<SHA> ."
```

Lalu, di host, ubah hanya `PMS_IMAGE` di `.env` dan hanya layanan `app`:

```bash
cd /home/deploy-user/pms
sed -i.bak 's|^PMS_IMAGE=.*|PMS_IMAGE=pms:candidate-<SHA>|' .env && rm -f .env.bak
docker compose up -d app
```

Container `postgres` tidak dibuat ulang.

## 3. Verifikasi setelah deploy

```bash
docker inspect pms-app-1 --format '{{.Config.Image}} {{.State.Health.Status}} {{.RestartCount}}'
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8086/api/v1/readyz     # 200
docker exec -i pms-postgres-1 sh -c 'psql -U "$POSTGRES_USER" -d pms -tA' <<< 'select max(version_id) from schema_migrations;'   # 364
deploy/ops-check.sh                                                                    # semua OK, exit 0
```

Revisi yang berjalan dibaca lewat `/api/v1/version` dengan sesi terautentikasi. Pastikan juga:

- `SMTP_HOST` kosong (`docker exec pms-app-1 sh -c 'echo ${#SMTP_HOST}'` harus `0`).
- `PMS_PUBLIC_LINKS` dan `PMS_PYDIO_LISTING` tidak bernilai `true`.
- `PMS_SEED` hanya berisi seed yang disetujui (saat ini `project-master,project-code`).

## 4. Penulisan dibekukan (write freeze)

Diperlukan hanya jika migrasi mengubah data bisnis atau jika pemulihan harus dibandingkan dengan keadaan tepat sebelum cutover:

1. Umumkan jendela pembekuan. Minta pengguna berhenti menyimpan perubahan.
2. `docker compose stop app` untuk menghentikan penulisan. Database tetap berjalan.
3. Buat backup pra-cutover (§1.3).
4. Kerjakan migrasi atau cutover. Setelah selesai: `docker compose up -d app`, lalu verifikasi (§3).

## 5. Migrasi

- Verifikasi migrasi pada clone terisolasi: restore backup terbaru ke database sementara, jalankan binary kandidat terhadapnya (`DB_NAME=<clone>`), lalu periksa jumlah baris dan skema.
- Jangan menjalankan migrasi pada staging aktif sebelum clone lulus.
- Migrasi yang gagal di tengah: jangan ulangi dengan binary lama. Pulihkan dari backup pra-cutover (§7).

## 6. Rekonsiliasi penulisan setelah cutover

Setiap penulisan yang terjadi di antara backup pra-cutover dan pemulihan harus diidentifikasi sebelum pemulihan diputuskan:

- Bandingkan jumlah item, lampiran, dan audit (`audit_events`) antara database aktif dan clone pemulihan, berdasarkan `created_at` setelah waktu backup.
- Lampiran yang diunggah setelah backup ada di direktori file aktif tetapi tidak di backup. Catat dan putuskan apakah perlu diunggah ulang.
- Jangan menimpa database aktif dengan backup tanpa keputusan tertulis atas penulisan tersebut.

## 7. Pemulihan dan rollback

1. Hentikan app: `docker compose stop app`.
2. Pilih backup pra-deploy yang lulus checksum.
3. Uji pemulihan ke database sementara dengan `deploy/restore-test.sh <STAMP>` (tidak menyentuh data aktif). Waktu dan hasil dicatat.
4. Jika pemulihan diputuskan: restore ke database baru, lalu ganti nama database dengan langkah yang sama seperti di `CLIENT_SCOPE_RECOVERY_RUNBOOK.md` (`deploy/restore.sh`). Jangan menghapus database asli sampai pemulihan terverifikasi.
5. Jalankan image pra-deploy bersama database pra-deploy (sesuai aturan §0), atau deploy perbaikan maju.
6. Verifikasi (§3) dan uji akses: pengguna berhak melihat datanya, dan lintas-Customer ditolak.

## 8. Backup off-host (status: EXTERNAL_BLOCKED)

Backup belum disalin ke luar server. Untuk mengaktifkannya, perlu:

- Tujuan yang disetujui (`BACKUP_REMOTE` atau `BACKUP_RSYNC_DEST`), dengan kontrol akses dan enkripsi saat disimpan.
- Penjadwalan (timer atau cron) yang menjalankan `backup.sh` dan memeriksa hasilnya. Saat ini tidak ada.
- Penyimpanan terpisah untuk berkas konfigurasi pemulihan. Kunci enkripsi di `.env` (TOTP, SSO, konektor, OTP) **tidak** ada dalam set backup. Tanpanya, database pulih tetapi rahasia terenkripsi tidak bisa didekripsi.

## 9. Kredensial dan konfigurasi

- Rahasia hanya di `.env` (mode 0600) dan di penyimpanan rahasia yang disetujui. Tidak di Git, log, atau laporan.
- Rotasi password akun sintetis dilakukan lewat `POST /auth/password/change`. Rotasi password admin dilakukan pemilik lewat jalur aman.
- Tautan aktivasi atau reset yang tercetak di log lama sudah tidak berlaku setelah dipakai. Periksa `user_tokens` untuk token yang belum terpakai.
- Log lama yang memuat tautan aktivasi dibatasi aksesnya dan tidak dihapus diam-diam.

## 10. Pemeriksaan dukungan rutin

- `deploy/ops-check.sh` (read-only): readiness, health container dan restart, disk, PostgreSQL, usia dan checksum backup, dan jumlah ERROR 15 menit terakhir. Exit 1 jika ada yang gagal.
- Alert: belum ada penerima yang disetujui. Hubungkan kode keluar ke monitor resmi saat penerima tersedia.
- Restore drill: jalankan `deploy/restore-test.sh` secara berkala. Hasil terakhir: 4 detik untuk set 20261127T070201Z.
