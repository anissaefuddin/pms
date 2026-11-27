# Laporan kesiapan operasional PMS (OPS-01..05)

Tanggal: 2026-11-27. Lingkungan: staging `dev-lens`. Hanya pengukuran dan tindakan yang dijalankan dalam sesi ini yang dicatat. Tidak ada infrastruktur produksi yang dibuat atau diubah.

## OPS-01 Lingkungan deploy

- Host: Linux 6.8, 4 vCPU, RAM 7,9 GB (sekitar 4 GB tersedia), disk 116 GB dengan 61% terpakai.
- Container PMS: `pms-app-1` (healthy, 0 restart), `pms-postgres-1` (postgres 16.10, healthy). Compose hanya berisi dua layanan ini.
- Host bersama: Pydio Cells staging (`cells-1`, `cells-dev-1`), CI, Jenkins, dan buildkit. Beban bersama perlu diperhatikan.
- Proxy: Traefik di luar stack PMS (labels di compose). `TRUST_PROXY=true`, port aplikasi tidak dipublikasikan.
- Volume: database di volume postgres; lampiran di `/data/files` (bind mount host).

## OPS-02 Backup off-host (status: EXTERNAL_BLOCKED)

- Backup lokal berfungsi: set `20261127T073630Z` dibuat dalam 1 detik (dump 1,17 MB, arsip file 1,32 MB), checksum valid.
- Isi set: dump database, arsip file lampiran, manifest (image tag), dan SHA-256. **Tidak berisi `.env`.** Kunci enkripsi (TOTP, SSO, konektor, OTP) hanya ada di host, sehingga pemulihan tanpanya tidak bisa mendekripsi rahasia.
- `BACKUP_REMOTE` dan `BACKUP_RSYNC_DEST` belum diatur. Tidak ada tujuan off-host yang disetujui. Tidak ada timer atau cron untuk backup. Backup hanya berjalan manual.
- Ini **bukan** backup off-host. Tidak ada data staging yang diunggah ke tujuan yang belum disetujui.

## OPS-03 Rehearsal pemulihan

Dua jalur dijalankan pada set `20261127T070201Z`:

| Tahap | Hasil | Waktu |
|---|---|---|
| `restore-test.sh` (database ke database sementara, cek skema dan jumlah baris) | Lulus; 43 user, 62 space, skema sama dengan live | 4 detik |
| Drill penuh: restore database sementara (`pms_drill`) | Lulus | 3 detik |
| Verifikasi lampiran (checksum SHA-256 dan ukuran) | 23 dari 23 cocok | 1 detik |
| Start kandidat terhadap database sementara | Ready (200), skema 364 | 2 detik |
| Login dan akses: pemilik project mendapat project sendiri (200), lintas-Customer ditolak (404) | Lulus | - |
| Hapus database sementara dan direktori sementara | Selesai | - |

- Total drill: 7 detik untuk data staging saat ini (sekitar 1,2 MB dump, 23 lampiran).
- **Titik pemulihan yang diamati:** waktu backup (`20261127T070201Z`), karena tidak ada penulisan setelah backup itu sebelum rehearsal.
- Ini bukan target RPO/RTO produksi. Target produksi belum disetujui, dan data staging jauh lebih kecil dari data produksi. Pengukuran tidak bisa diekstrapolasi.
- Pemulihan selalu ke database sementara. Database `pms` dan `pms_pre_candidate` tidak disentuh.

## OPS-04 Monitoring dan alert

- Diverifikasi (read-only, `deploy/ops-check.sh`): readiness 200, container healthy dan 0 restart, disk 61%, PostgreSQL menerima koneksi, backup terbaru valid dan berumur kurang dari 26 jam, 0 baris ERROR dalam 15 menit terakhir. Exit 0.
- Ada konfigurasi `deploy/monitoring` (Prometheus dan Grafana), tetapi **container itu tidak berjalan** di staging. Tidak dipasang karena tidak ada target yang disetujui dan tidak perlu platform kompleks untuk MVP.
- Alert: `BACKUP_ALERT_URL` belum diatur. Tidak ada penerima yang disetujui. Tidak ada alert yang dikirim, dan tidak ada kondisi buatan yang dipicu. **EXTERNAL_BLOCKED** sampai penerima tersedia.

## OPS-05 Runbook

- `docs/ops/DEPLOY_AND_RECOVERY_RUNBOOK.md`: pra-deploy, build, migrasi, verifikasi, penulisan dibekukan, rekonsiliasi, pemulihan dan rollback, kredensial, dan pemeriksaan rutin.
- Aturan utama: migrasi maju saja. Pemulihan berarti backup pra-deploy bersama image lama.

## Blocker yang tersisa

| Blocker | Pemilik | Status |
|---|---|---|
| Tujuan backup off-host yang disetujui, enkripsi, dan penjadwalan | Operasi | EXTERNAL_BLOCKED |
| Escrow konfigurasi pemulihan (`.env`) di luar server | Operasi / keamanan | Belum |
| Penerima alert | Operasi | EXTERNAL_BLOCKED |
| Target RPO/RTO produksi | Pemilik | Belum disetujui |
| Rotasi password admin | Pemilik | Belum |
| Akun dan folder Pydio uji yang disetujui; root Customer | Pemilik / Pydio | EXTERNAL_BLOCKED |
| CI jarak jauh, STG-14, dan regresi rilis final | Tim | Belum |
