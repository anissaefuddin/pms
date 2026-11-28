# Proses rilis: commit, push, deploy, dan versi

Dokumen ini adalah aturan kerja untuk setiap perubahan yang di-commit, di-push, dan di-deploy ke staging-host.
Berlaku sampai pemilik proyek menyetujui perubahannya.

## 1. Aturan versi

- Sebelum pemilik menyetujui MVP 1, semua versi berbentuk `0.MINOR.PATCH`.
- `1.0.0` atau lebih tinggi **tidak boleh** dibuat, di-tag, atau di-deploy tanpa persetujuan eksplisit dari pemilik.
- **MINOR** naik untuk deploy yang menambah fitur atau perubahan perilaku: `0.2.0` → `0.3.0`.
- **PATCH** naik untuk deploy yang hanya berisi perbaikan: `0.2.0` → `0.2.1`.
- `-rc.N` (contoh `0.3.0-rc.1`) hanya untuk staging, jika diperlukan.
- Versi ditentukan dari satu sumber: tag git `vX.Y.Z`. Nilai yang sama harus dipakai di:
  - `web/package.json` dan `web/package-lock.json` (field `version`), yang tampil di Settings > Installation;
  - argumen build Docker `VERSION` (tanpa huruf `v`), yang dipakai server;
  - tag image `pms:X.Y.Z`.
- Hash commit tetap dicatat sebagai label image, untuk penelusuran.

## 2. Urutan kerja

Setiap rilis mengikuti urutan ini. Jangan melompati langkah.

1. **Tentukan versi.** Lihat tag terakhir (`git tag --list 'v*'`), lalu pilih MINOR atau PATCH sesuai aturan di atas.
2. **Naikkan versi di kode.** Ubah `web/package.json` dan `web/package-lock.json` ke `X.Y.Z`. Jalankan `npx tsc -b` dan test.
3. **Commit.** Pesan commit menyebut versi, contoh `chore(release): 0.2.0`, di commit terpisah dari perubahan fitur bila memungkinkan.
4. **Verifikasi.** Jalankan test Go (`go test ./server/...` dengan `PMS_TEST_DATABASE_URL`), `tsc -b`, dan Playwright seluruh suite. Jangan deploy jika ada yang gagal.
5. **Tag.** `git tag -a vX.Y.Z -m "vX.Y.Z"` di commit rilis.
6. **Push.** Push branch dan tag: `git push origin <branch> vX.Y.Z`. Push hanya setelah pemilik menyetujui (lihat bagian 3).
7. **Build di server** dengan versi eksplisit:
   ```bash
   git archive vX.Y.Z | ssh deploy-user@198.51.100.10 'rm -rf ~/pms-src-X.Y.Z && mkdir -p ~/pms-src-X.Y.Z && tar -x -C ~/pms-src-X.Y.Z && cd ~/pms-src-X.Y.Z && docker build --build-arg VERSION=X.Y.Z -t pms:X.Y.Z .'
   ```
8. **Backup pra-update.** Di server: `cd ~/pms && ./backup.sh`. Catat nama backup dan periksa dump dapat dibaca.
9. **Ganti image.** Di server, simpan `.env` lama, lalu set `PMS_IMAGE=pms:X.Y.Z` dan jalankan `docker compose up -d app`.
10. **Cek kesehatan.** `curl localhost:8086/readyz` dan `/healthz` harus 200. Cek log tidak ada `level=ERROR` sejak start.
11. **Catat.** Tulis di laporan: versi lama dan baru, nama backup, status migrasi, dan cara rollback.

## 3. Persetujuan

- Commit dan push hanya dilakukan jika pemilik meminta secara eksplisit di pesan terbaru.
- Deploy hanya dilakukan jika pemilik meminta secara eksplisit. Persetujuan untuk satu deploy tidak berlaku untuk deploy berikutnya.
- Migrasi yang mengubah atau menghapus data (`DROP`, `DELETE`, `TRUNCATE`) di bagian Up wajib dilaporkan ke pemilik sebelum deploy.

## 4. Rollback

- Ganti `PMS_IMAGE` ke tag versi sebelumnya, lalu `docker compose up -d app`.
- Jika migrasi sudah berjalan, rollback image saja tidak cukup. Pulihkan backup pra-update dengan `./restore.sh <stamp> --force` sesuai `docs/operations/PILOT_RUNBOOK.md`.
- Migrasi tidak memiliki jalur turun di produksi. Bagian `-- +goose Down` tidak dijalankan.

## 5. Riwayat versi

| Versi | Isi | Status |
|---|---|---|
| v0.1.0 | Baseline sebelum pekerjaan Space, Blueprint, Import/Export, dan Automation | tag ada |
| v0.2.0 | Space settings, Blueprint, Import/Export, Automation, perbaikan sidebar dan profil | disetujui untuk deploy berikutnya |
