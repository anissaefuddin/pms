# Eksekusi Batch 2 (22 task)

Kandidat: `fix/qa-batch1-remediation` @ `b8b3239` (Batch 1: `5289ed4`); Batch 2 di cabang `fix/qa-b2-a..d`, terintegrasi di `qa/integration-b2`. Semua lokal: tidak ada push, PR, merge atau deploy baru setelah deploy `pms:5289ed4` yang Anda setujui. 15 Nov 2026.

Dikerjakan paralel oleh empat jalur (worktree dan Postgres sekali pakai masing-masing). Detail: `results/b2_lane{A,B,C,D}.md`.

| Hasil | Task |
|---|---|
| **FIXED** (tes merah→hijau lokal, PENDING CI, belum deploy) | RT-29, RT-35 (A); RT-05, RT-06, RT-24, RT-25, RT-26, RT-28, RT-37 (B); RT-34 (kontrak 409 + `make gen`), RT-36 (usulan koreksi parity, matriks resmi tidak diubah) (C) |
| **NOT_A_BUG** (setelah validasi akar masalah) | RT-10 (child_count ada di build terdeploy; tes regresi ditambah), RT-11 (rumus beda per metode; data QA seragam), RT-17 (kontrak/PRD tak mendefinisikan filter q/access), RT-18 (aturan terbit sengaja tidak aktif otomatis; AR-4 terbukti jalan), RT-33 (lingkup reviewer eksternal sudah diuji; temuan karena share Space di seed) |
| **DECISION** (bukan bug atau fitur baru; tidak dikerjakan) | RT-14 (approver tanpa akses dilewati diam-diam: sengaja, perlu keputusan produk), RT-04 (Excel round-trip status/assignee di luar SPC-15), RT-32 (edit booking/peringatan tumpang tindih bukan requirement), RT-15 (CUST-02 belum dibangun, 5–7 hari), RT-07 (VIEW-04, kartu Board), RT-08 (SPC-14, blueprint menu +) |

RT-06: `/profile` dan `/account` memang bukan rute (`/settings/profile`, `/settings/account`): bukan bug.
Opsi dan estimasi tiap DECISION ada di `results/b2_laneC.md` (RT-32/33/15/07/08) dan `b2_laneA.md` (RT-14, RT-04).

**UI lane 2** (lane-D, Playwright + cookie sesi akun QA di dev-lens): 45 baris: 8 PASS, 35 PARTIAL, 2 BLOCKED. Sisa PARTIAL berarti sub-asersi tidak bisa/tidak dicoba lewat UI (Table inline, Board, Gantt, siklus dependensi dll.), bukan gagal. Temuan UI baru: QA-D-laneD-003 (tombol Approve/Reject aktif setelah final) dan -004 (penghitung Subitems basi): Low, diusulkan Batch 3.

**Batas:** tidak ada task Batch 2 yang VERIFIED di build terdeploy; semua menunggu deploy. Batch 3 tidak dimulai.

## Deploy dan retest (disetujui pengguna, 15 Nov 2026)
Build `pms:86c33d1` (= `qa/integration-b2`: Batch 1 + `b8b3239` + Batch 2 a–d) dideploy ke dev-lens; backup `20261115T013823Z`; healthy, `/healthz` 200, 0 ERROR.
- **VERIFIED** di dev-lens: RT-05, 06, 24, 25, 26, 28 (axe 0 pelanggaran di 1440/390 terang/gelap, tanpa scroll horizontal; `evidence/retest-b2/`), RT-29 (bucket "Unassigned"), `required_fields: []` 200, approval final 409 `approval_final` (suara approver lain tetap 200).
- **QA-D-laneD-001**: 76 create paralel lewat edge semuanya 201, **0 kali 409** (44 kali 503 dari Cloudflare). Uji tulis langsung ke origin dari Mac tidak bisa (cookie sesi `Secure` ditolak lewat http), jadi bukti skala di origin tetap tes lokal 2.000 anak.
- **RT-37: FAIL pada retest**, tidak ditutup: filter bertingkat level-1 jalan, tetapi item level-2 di cabang yang belum dimuat tetap tidak ketemu karena filter hanya mencari cabang yang sudah dimuat. Perbaikan benar butuh pencarian sisi server (API): keputusan/Batch 3.
- Tidak diretest di build terdeploy: RT-34 (kontrak), RT-35 (butuh connector dengan token ditolak tanpa menyentuh connector asli), RT-36 (dokumen).
