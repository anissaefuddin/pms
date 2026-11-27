# Penerimaan rilis kandidat v0.14.0 (PROMPT #034)

Tanggal: 2026-11-27. Kandidat: `b3bfa8cec81ce7f0600eefc9b4b77f6bdb490118` (dibangun dari commit ini; image `pms:candidate-b3bfa8ce…`; revisi terverifikasi di `/api/v1/version`). Schema 364. Lingkungan: staging `dev-lens`. Tidak ada production. Push: **SKIPPED**. Tidak ada tag rilis, PR, atau perubahan visibilitas repositori.

Status: **PASS** (dijalankan dan lulus), **FAIL**, **BLOCKED** (prasyarat eksternal belum ada), **DEFERRED** (belum dijalankan dalam sesi ini).

| Kriteria | Status | Perintah atau tes | Lingkungan | Hasil dan bukti | Risiko sisa | Tindakan pemilik |
|---|---|---|---|---|---|---|
| Portfolio monitor terkonsolidasi | PASS (fungsi inti) | `TestPortfolioMonitor*` (4 tes), uji live API, browser sebagai `qa-reader-a` | Test DB + staging | Baris hanya proyek yang boleh dilihat; pengecualian Customer; revokasi langsung; unresolved ditutup. Live: pm-a 5 baris, pm-b 2, outsider 0 | Monitor per-Space tidak memuat semua kolom gabungan dengan filter tanggal | Tidak ada |
| Pembekuan fitur MVP | PASS (keputusan) | Lihat MVP_SCOPE.md | Dokumen | Fitur S-curve, listing Pydio, template publish, portal Customer, konektor baru, AIMS/HAZOP/RMS ditunda | Tidak ada | Tidak ada |
| Regresi backend terintegrasi (paket terdampak) | PASS untuk paket terdampak | `go test` untuk items, httpserver, spaces, policy, config, seed, cmd/pms; clientscope, pydio, webhooks, automation | Test DB lokal | Semua ok (exit 0) setelah koreksi | Suite penuh belum dijalankan | CI jarak jauh |
| Matriks otorisasi dan tes negatif | PASS | STG-01..16 (dijalankan sebelumnya dan diulang pada build lebih awal): 15 PASS, 0 FAIL | Staging | Termasuk lintas-Customer, pencarian, Portfolio, ekspor, lampiran, listing Pydio OFF, unresolved | Matriks penuh belum diulang pada `b3bfa8ce` | Tidak ada |
| STG-14 (token lama) | PASS (terisolasi) / BLOCKED (staging) | `TestGanttSnapshotsHidePrivateDataAndExpire` dan tes link publik OFF | Test DB | Tautan dicabut → 404; kedaluwarsa → 404; token tidak valid → 404 | Di staging tidak ada token lama; tautan publik sengaja tetap OFF | Tidak ada |
| Migrasi: pemisahan rencana, penolakan keseluruhan | PASS | paket `clientscope` | Test DB | Ok, termasuk rehearsal pemulihan | Quarantine per-resource belum lengkap (dicatat sebagai batasan) | Keputusan pemilik untuk quarantine |
| Migrasi: mapping konflik tidak bisa membuka akses | PASS (uji) | Tes `clientscope` | Test DB | Ok | Hanya diuji pada data sintetis | Review mapping nyata |
| Unresolved tetap tertutup | PASS | `TestPortfolioMonitorExcludesUnresolvedOwnership`, STG-16 | Test DB + staging | 404 untuk admin dan pm-a; tidak di pencarian | 18 Space tetap belum diklasifikasi | Keputusan bisnis |
| Konektor tidak aktif di level eksekusi | **FAIL** | Inspeksi `connector_installs` (read-only) | Staging | 1 dari 3 install aktif: GitHub "Bebas", sinkron berhasil 08:01 hari ini. Dokumen MVP menyebutnya default nonaktif dan belum diuji sandbox. Penonaktifan diminta tetapi ditolak oleh classifier (lihat catatan) | Konektor tidak terverifikasi berjalan terhadap data nyata | **Pemilik: putuskan dan nonaktifkan** |
| Tautan publik dan listing Pydio OFF | PASS | Respons 404 publik; respons 503 `pydio_listing_disabled` | Staging | Terverifikasi | Tidak ada | Tidak ada |
| SMTP OFF | PASS | `SMTP_HOST` kosong; 43 email tercatat, tidak terkirim | Staging | Terverifikasi | Log lama memuat tautan | Tidak ada |
| Dokumen Pydio (UI) | PASS (status) | `TestProjectDocumentStates…`; browser project detail | Test DB + staging | Status server; tautan hanya setelah validasi terhadap root Customer | Tidak ada root yang disetujui; tidak ada tautan aktif di staging | Pemetaan root Customer |
| Pydio ACL dan DMS live (AC-14) | **BLOCKED** | DMS-01 dan DMS-09 PASS; sisanya tidak bisa dijalankan | Staging | Tidak ada akun dan folder uji yang disetujui | Tidak ada verifikasi ACL live | Akun dan folder uji disetujui |
| Rotasi password admin | **BLOCKED** | Tidak ada jalur aman yang tersedia dalam sesi ini | - | Password lama pernah dikirim di chat | Kredensial terekspos | Pemilik rotasi lewat jalur aman |
| Rotasi password sintetis | PASS | `POST /auth/password/change` (9 × 204) dan login ulang (9 dari 9) | Staging | Sesi lama dicabut | Tidak ada | Tidak ada |
| Backup terjadwal harian (lokal) | PASS (lokal) | Cron 02:17, `backup.sh`, `ops-check.sh` | Staging | Set terbaru checksum valid; usia dicek | Tidak ada retensi; hanya di server | Kebijakan retensi |
| Backup off-host | **BLOCKED (EXTERNAL_BLOCKED)** | `BACKUP_REMOTE` belum diatur | Staging | Tidak ada tujuan yang disetujui | Tidak ada salinan di luar server | Tujuan, akses, dan enkripsi |
| Escrow konfigurasi pemulihan | **BLOCKED** | - | - | `.env` tidak ada dalam backup | Pemulihan tidak bisa mendekripsi rahasia tanpa kunci | Escrow terenkripsi |
| Rehearsal pemulihan dan RPO/RTO | PASS (rehearsal) / BLOCKED (target) | `restore-test.sh` 4 detik; drill penuh 7 detik; 23/23 lampiran | Staging (salinan terisolasi) | Pemulihan ke database sementara; akses dan lintas-Customer diverifikasi | Angka tidak bisa diekstrapolasi; target 24 jam / 2 jam belum disetujui | Persetujuan target RPO 24 jam dan RTO 2 jam |
| Alert | **BLOCKED** | `ops-check.sh` (read-only) | Staging | Pemeriksaan lulus; tidak ada penerima alert | Tidak ada alert keluar | Penerima yang disetujui |
| Runbook deploy dan pemulihan | PASS (dokumen) | `docs/ops/DEPLOY_AND_RECOVERY_RUNBOOK.md` | Dokumen | Dibuat dari prosedur yang sudah dijalankan | Belum diuji ulang oleh orang lain | Tinjau operasi |
| Tes frontend | PASS | typecheck, lint, token check, build, 127 tes unit | Lokal | Lulus | Tes E2E Playwright penuh belum dijalankan | Dijalankan di CI jarak jauh |
| Browser UAT | PASS (terbatas) | Portfolio, dokumen, Read Only, Limited, dan alur project | Staging | Lihat STAGING_REHEARSAL.md §14 | Tidak semua alur UI dijalankan ulang | Tidak ada |
| Pengecekan CI jarak jauh (repositori privat) | **BLOCKED** | Tidak ada CI jarak jauh yang bisa dihubungi tanpa push | - | - | Belum ada bukti CI | CI jarak jauh |
| Persetujuan produksi | **BLOCKED** | - | - | Belum ada | - | Persetujuan produk eksplisit |

## Keputusan GO

- Deployment teknis staging: **GO**.
- Fungsi MVP untuk onboarding terkendali pada staging: **CONDITIONAL GO** (Portfolio, dokumen, dan alur inti tersedia; konektor tidak aktif harus diputuskan).
- Keamanan: **CONDITIONAL GO** untuk staging. Temuan terbuka: konektor aktif tanpa verifikasi (FAIL).
- Produksi: **NOT READY**. Prasyarat yang wajib sebelum GO: backup off-host dan enkripsi, alert, rotasi password admin, verifikasi ACL Pydio live atau pengecualian lingkup dari pemilik, CI jarak jauh, persetujuan RPO/RTO, pemetaan operasional yang disetujui, dan persetujuan produk.

## Catatan percobaan yang ditolak

Penonaktifan konektor `github` "Bebas" pada staging dicoba melalui endpoint admin yang ada, tetapi permintaan ditolak oleh pemeriksa otomatis (kategori tindakan yang tidak diinginkan). Tidak ada upaya lain yang dilakukan. Konektor tetap aktif sampai pemilik memutuskan.
