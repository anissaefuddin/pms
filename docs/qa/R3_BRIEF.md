# Brief retest penuh (build `pms:9e78660` di dev-lens)

Target: https://dev-lens.losakaenergi.app, build `pms:9e78660` (= PR #15, cabang `qa/integration-b3`; Batch 1–3). Kandidat kode: worktree `/Volumes/MyHDD/homeserver/apps/pms-b3-int`. Baseline pengujian: `docs/qa/ PMS_ITA_PHASE1_SEED/` dan `docs/qa/test_matrix.csv` (status sebelumnya).

Aturan: sama dengan `docs/qa/LANE_BRIEF.md` (akun QA `@pms-ita.test`, kata sandi di file `/private/tmp/claude-501/qa/seed_password`, tidak diketik di browser: login via API lalu cookie sesi ke Playwright; Origin wajib; UA bukan default Python; `SSL_CERT_FILE=/etc/ssl/cert.pem`), **tidak mengubah kode**, tidak push/merge/deploy, tidak reset/hapus data di luar namespace QA, tidak membuat session. Temuan produk = defect baru (`QA-D-r3<lane>-NNN`, file `docs/qa/results/r3_<lane>_defects.csv`).

Pengguna menyetujui: "lakukan pengujian di dev-lens: perbaikan UI Batch 3 (tombol approval, ekspansi level 2, overflow 390 px) dan retest penuh."

Keluaran per jalur: `docs/qa/results/r3_<lane>.csv` dengan kolom `test_id,status,previous,actual,evidence,defect_id` untuk SETIAP baris milik jalur (status baru berdasarkan hasil aktual; jangan menaikkan PARTIAL/BLOCKED ke PASS tanpa uji lengkap; sebutkan jika berubah dari `previous`), bukti di `docs/qa/evidence/<test_id>/` (UI: 1440 dan 390 px bila relevan). Laporan ringkas `docs/qa/results/r3_<lane>.md` (jumlah per status, perubahan, defect, apa yang tak diuji).

## Pembagian (sama seperti run pertama)
- **qa-lane1**: AUTH, NAV, SPC, VIEW, 16 baris S* (UI M12b) + audit visual/responsif.
- **qa-lane2**: TASK, WF, FORM, CUST, AUT, APR, BIN, SCH. Bagian UI diuji lewat Playwright + cookie (akun QA ini dibuat khusus untuk pengujian di dev-lens atas permintaan pengguna).
- **qa-lane3**: DASH, RPT, SET, USR, SEC, OPS, API, ENT, NFR*, STAB*, G*, GA* **plus verifikasi perbaikan UI Batch 3**: (a) tombol Approve/Reject/komentar tersembunyi setelah approval final untuk semua approver, tetap tampil saat pending; (b) ekspansi baris level 2 di Table (Kurau Plant dan sejenisnya) + quick filter hierarkis (filter "HAZOP" harus menemukan item level-2 dengan breadcrumb, "Load more", dan tidak membocorkan induk yang tak boleh dilihat user terbatas); (c) tidak ada overflow horizontal 390 px pada `/settings/audit`, `/settings/apps`, panel approval; ditambah axe di rute terdampak, terang dan gelap.
- **qa-lane4**: RSC, FIL, INT, REG, PAR*, GAP-* (termasuk GAP-15 pencarian tabel) dan parity (`WFP_PARITY_MATRIX_QA.csv`).
