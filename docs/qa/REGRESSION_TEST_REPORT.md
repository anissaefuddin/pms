# Laporan tes regresi (Batch 3/4)

Kandidat lokal: `qa/integration-b3` @ `9e78660` (worktree `/Volumes/MyHDD/homeserver/apps/pms-b3-int`), turunan `qa/integration-b2` @ `86c33d1` (terdeploy dev-lens). Tidak ada push, PR, merge atau deploy baru. 15 Nov 2026.\n\n
## Lokal, `qa/integration-b3`
- Go: `go test -count=1 ./server/...` setelah `9e78660`: 22 paket ok, 0 gagal. `go vet`/`gofmt` bersih. Tes skala (`PMS_TEST_SCALE=1`): 500/1.000/2.000 anak, 0 galat. Tes lock: 5 dari 5 putaran hijau (sebelum perbaikan: gagal 4 dari 4).
- Web: `tsc -b` bersih; Playwright mock 452 lulus, 42 dilewati (tes live butuh `PMS_TEST_BROWSER=1`), 0 gagal (dijalankan sebelum `9e78660`, yang hanya mengubah server).
## CI GitHub
PENDING (belum push).
## dev-lens (`pms:86c33d1`)
Retest Batch 1–2: lihat `BATCH_2_EXECUTION_REPORT.md`. Batch 3 **tidak** ada di build ini.
## Instance QA lokal terisolasi (lane-D)
ITA-REQ-007, 009 dan NFR-ISOLATION PASS; ITA-REQ-114/115 dan STAB-10 PARTIAL. Belum teruji: migrasi data besar, rollback ke skema tak kompatibel, restore di host fisik lain, RTO pada data produksi.
## Provider live
Tidak dijalankan (tanpa kredensial).
## Regresi 208 skenario
**Belum dijalankan ulang secara penuh**: butuh Batch 3 terdeploy ke dev-lens (belum disetujui) dan lane live. Matriks saat ini: PASS 54, PARTIAL 119, FAIL 4, NOT_IMPLEMENTED 11, BLOCKED 19, NOT RUN 1 (dari 208). PARTIAL, SKIPPED dan BLOCKED tidak dianggap PASS.

## Retest penuh di dev-lens `pms:9e78660` (15 Nov 2026)
Empat jalur menjalankan ulang 208 baris (lane2 API: 13 baris langsung + 36 baris oleh lane3; UI lane 2 tidak dijalankan oleh lane2 sendiri, sebagian UI oleh lane3; lane1 22 baris oleh lane4). Hasil mentah: `results/r3_lane*.csv`, `r3_lane*.md`.
Matriks setelah retest: PASS 70, PARTIAL 107, FAIL 2, NOT_IMPLEMENTED 8, BLOCKED 20, NOT RUN 1 (dari 208).
- **Batch 3 UI terverifikasi live (lane3):** approval final tanpa tombol, pending dengan tombol; Kurau Plant bisa diperluas dan filter "HAZOP" menemukan item level-2 dengan breadcrumb (user terbatas tak melihat judul induk); 0 px overflow di 390 px dan 0 pelanggaran axe pada /settings/audit, /settings/apps, panel approval, /reports, /inbox, /workload, /timesheets, /dashboards. "Load more" tidak diuji (<50 hasil).
- **Naik:** NFR-A11Y (axe 0 pelanggaran, 14 rute), REQ-140 (PDF render), GAP-15 (pencarian bertingkat) dan sejumlah baris lane2 PARTIAL→PASS (052, 055, 060, 061, 062, 065, 067, 076, 091, 121, 122, 129, 131).
- **Turun/tetap:** REQ-113 PASS→PARTIAL (form publik 30 paralel: 15×409, 8×429, 7×201; defect QA-D-lane3-002 masih terbuka di jalur form publik, di luar perbaikan create item). BLOCKED tetap untuk mail sink, IdP, provider live, backup/restore host terpisah, GA/gate.
- Observasi: API mengizinkan suara terlambat dari approver yang belum memilih setelah hasil final (UI menyembunyikan tombolnya); sesuai aturan yang dipilih (approver yang sudah memilih terkunci), tapi perlu keputusan produk.
- **CI GitHub PR #15:** `test` merah pada putaran pertama karena `m5-live.spec.ts` masih mengharapkan "(none)" (label RT-29 menjadi "Unassigned"); diperbaiki di `828c2ef`, lulus lokal; CI putaran baru berjalan. `authorization-matrix` dan `parity-e2e` hijau.

## Batch 4 focused verification (15 Nov 2026)

- Backend OTP/create-menu/card-field tests: PASS on `qa/integration-b3`.
- TypeScript typecheck: PASS.
- Board/view-field Playwright: 6/6 PASS, including configurable assignee, individual dates and priority plus drag behavior.
- Full 208-scenario regression was not rerun. `test_matrix.csv` remains evidence-based; PARTIAL/BLOCKED rows were not promoted.

## Batch 5 combined local candidate (15 Nov 2026)

- Full Go suite PASS after public-form locking and OTP HMAC changes.
- Web lint/typecheck/unit/build PASS.
- Playwright full mock/local suite: 452 PASS, 42 SKIPPED, 0 FAIL; focused Board suite 6/6 PASS.
- Public-form 30-way concurrency PASS with exact side-effect counts; stress 5× PASS.
- OTP concurrent single-use PASS; stress 10× PASS.
- Historical 208-scenario statuses are not globally promoted because live, deployed and UAT sub-assertions remain.
