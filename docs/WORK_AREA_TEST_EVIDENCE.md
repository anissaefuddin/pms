# Work Area Test Evidence (Phase 3)

Status: first pass, commit `3867409` on `feat/m12c-ui-clean`. Dibuat dari tes yang ada dan hasil run lokal. Tidak ada tes baru yang dijalankan terhadap dataset QA seperti yang diminta epik (Lalang Platform UX QA belum dibuat).

Legenda status: PASS, FAIL, BLOCKED, NOT RUN, NOT APPLICABLE.

Run lokal terakhir: `npx playwright test` = 494 passed, 0 failed, 43 skipped. Unit `npm test` = 85/85. Typecheck, lint, cek token lulus. Go: `go vet ./...` lulus; tes integrasi butuh `PMS_TEST_DATABASE_URL` dan di-skip lokal.

| Test ID | Page | Scenario | Expected result | Actual result | Status | Evidence | Defect reference |
|---|---|---|---|---|---|---|---|
| E2E-01 | Project | Switch Table, Board, Gantt, Calendar, Timelog, Files, Chart | Layout stabil, view aktif jelas | Setiap view punya tes masing-masing; tidak ada satu alur yang membuka semua tujuh berurutan pada data QA | NOT RUN (alur gabungan) | views.spec, calendar.spec, gantt.spec, timelog-states, project-files | - |
| E2E-02 | View | Personal view: buat, ubah nama, sembunyikan, buka dari All views, tampilkan lagi, reload | Preferensi bertahan; user lain tidak bisa | Hide, restore, reload, drag urutan: PASS (mock). Isolasi dua user: hanya Go integration | PARTIAL: UI PASS, isolasi dua user BLOCKED | view-visibility.spec; view_prefs_integration_test.go (BLOCKED) | - |
| E2E-03 | Space, Project, Folder | Buat Project, Folder, Task, subitem | Parent, tipe, Space benar | Entry point Space dan Project: PASS. Folder dan subitem: NOT RUN | PARTIAL | creation-context.spec | - |
| E2E-04 | Table, Board, Chart | Ubah status lalu lihat Board, Gantt, Calendar, Chart | Sinkron lintas view | NOT RUN | NOT RUN | - | - |
| E2E-05 | Gantt, Calendar, Table, panel | Ubah jadwal, cek kalender dan tabel | Tanggal konsisten | Drag Gantt dan kalender: PASS (tes lama). Sinkron lintas view: NOT RUN | PARTIAL | gantt.spec, calendar.spec | - |
| E2E-06 | Calendar | Lintas minggu dan bulan, overflow, pindah | Segmen berlanjut, tidak duplikat | Layout unit: PASS. Overflow e2e: PASS. Lintas bulan e2e: NOT RUN | PARTIAL | calendar-c2.spec, calendar-layout.test | - |
| E2E-07 | Panel | Buka dari Table, Board, Gantt, Calendar; ubah; tutup | Tetap di workspace, filter dan scroll terjaga | Folder dan Project Info: PASS. Dari Table, Board, Gantt, Calendar: NOT RUN. Preservasi filter dan scroll: NOT RUN | PARTIAL | folder-header.spec, detail-panel-d.spec | - |
| E2E-08 | Approvals | Buat, putuskan, finalisasi, guard | Guard tetap berlaku | Lifecycle existing: PASS (approvals.spec, m3-live, m9-live). Tes perubahan presentasi: PASS | PARTIAL | approvals.spec | - |
| E2E-09 | Files | Unggah, lihat Files, unduh, versi baru | Metadata dan review per versi benar | Versi dan lampiran tautan: PASS. Unggah dengan retry: PASS. Review per versi: PASS (mock) dan status Cancelled di Go | PARTIAL | file-versions, files-links, upload-retry, file-review-version | - |
| E2E-10 | Timelog | Filter, total, kondisi kosong, gagal | Total seluruh data yang difilter | Kondisi kosong dan gagal: PASS. Total: PASS (mock). Total benar di database: BLOCKED (Go) | PARTIAL | timelog-states.spec; timelog_totals_integration_test.go (BLOCKED) | - |
| E2E-11 | KPI, Chart, Board | Bandingkan KPI dan label In progress | Label dan angka sesuai | Tally server dan label: PASS (mock). Chart belum memakai tally | PARTIAL | project-tally.spec | Label "In progress" sudah sesuai WA-D02; Active belum ditampilkan di strip |
| E2E-12 | Berbagai | Gagal muat, gagal mutasi, konflik, kosong | Umpan balik jelas, retry aman | Timelog, unggah, Stream, Space: PASS. Konflik dan jaringan putus di tiap view: NOT RUN | PARTIAL | timelog-states, upload-retry, table-bulk (partial success) | - |
| VIS-01 | Semua | Screenshot before/after 1280/1440/1920, light/dark, density | Tampilan sesuai desain | NOT RUN. Tidak ada screenshot yang dirender dan diperiksa dalam batch ini | NOT RUN | - | - |
| PERF-01 | Table, Gantt | Waktu render, panggilan API berlebih | Dibandingkan dengan baseline | NOT RUN (perlu baseline dan data besar) | NOT RUN | gantt-perf.spec ada, belum dibandingkan | - |
| GO-01 | Backend | Integrasi: Timelog totals, file review status, view prefs, view expansion | PASS di database test | Di-compile dan vet, di-skip lokal tanpa database | BLOCKED BY ENVIRONMENT | *_integration_test.go | - |

## Temuan
- Tidak ada defect P0 atau P1 baru yang terverifikasi dalam batch ini. Itu bukan bukti tidak ada defect, karena banyak skenario belum dijalankan.
- Batasan yang sudah diketahui dan didokumentasikan: withdrawal sebelum migrasi 00325 tetap "Not requested"; drag-and-drop hanya untuk urutan view; chart belum memakai tally server.

## Status gate
BLOCKED BY ENVIRONMENT. Tes Go integrasi tidak bisa dijalankan tanpa database, dan dataset QA Lalang Platform belum dibuat di dev-lens. Tidak ada gate yang bisa dinyatakan READY FOR STAKEHOLDER REVIEW sebelum VIS-01, PERF-01, dan E2E yang NOT RUN dijalankan.

## Pemeriksaan read-only di dev-lens (browser pane, akun pemilik, tanpa mutasi)

Dilakukan setelah pemilik login sendiri di browser pane. Tidak ada data yang diubah.

| Test ID | Page | Scenario | Expected | Actual | Status | Evidence | Defect reference |
|---|---|---|---|---|---|---|---|
| DEV-01 | Space QA Engineering | Daftar project dan status | Project dan jumlah subitem tampil | Project QA tampil dengan jumlah subitem | PASS | snapshot halaman | - |
| DEV-02 | Project QA seed | Header ringkas, strip KPI | Breadcrumb, judul, Info, Share, Add task; strip satu baris | Sesuai | PASS | screenshot | - |
| DEV-03 | Project QA seed | KPI server tally | Work items dan In progress dari analytics | Work items 105, In progress 11; analytics/query 200 | PASS | network log | - |
| DEV-04 | Project QA seed | Konsistensi Board dan tabel (E2E-11) | Hitungan direct children cocok dengan Board | Board: New 7, Completed 1, In Progress 0 (total 8), tabel 8 baris | PASS | hitung kolom via JS | - |
| DEV-05 | Project QA seed | Label cakupan KPI | Cakupan terlihat di strip | Keterangan cakupan hanya muncul setelah Summary dibuka | FAIL (P2) | teks Summary | Label "Work items 105" dan "In progress 11" tanpa keterangan subtree di strip |
| DEV-06 | Browser | Error konsol | Tidak ada error di luar 401 sebelum login | Hanya 401 pada /me sebelum login | PASS | read_console_messages | - |

Belum dijalankan (perlu persetujuan karena mengubah data atau membuat view): E2E-02 (view pribadi), E2E-03 (buat project dan folder), E2E-04 sampai E2E-07 (perubahan status dan jadwal), E2E-08 (approval), E2E-09 (unggah), E2E-10 (time entry).

## E2E-03 di dev-lens (akun pemilik, data uji diberi prefiks QA-E2E03)

| Test ID | Page | Scenario | Expected | Actual | Status | Evidence | Defect reference |
|---|---|---|---|---|---|---|---|
| E2E-03a | Space QA | Buat Project | Project dibuat di Space | QA-E2E03 Project dibuat | PASS | toast dan URL | - |
| E2E-03b | Project | Buat Folder di dalam Project | Parent = Project | Folder dibuat, halaman folder terbuka, judul tampil sekali | PASS | toast | - |
| E2E-03c | Folder | New item lalu Task | Parent = Folder | Task muncul di tabel folder; tautan parent di panel = folder | PASS | panel | F3 (label footer "Space root") |
| E2E-03d | Task | Subitem Task | Parent = Task | Subtask dibuat; parent di panel = Task | PASS | panel | - |
| E2E-03e | Task | Tipe Project dan Folder di bawah Task | Tidak bisa dipilih | Server dan fieldset menolak; kartu tampak bisa diklik | PASS (secara fungsi), FAIL (tampilan) | DOM fieldset disabled | F4 |
| E2E-03f | Folder | Breadcrumb | Space / nama folder | Tidak ada breadcrumb | FAIL | screenshot | F2 |
| E2E-03g | Project | KPI scope | Keterangan cakupan terlihat | Hanya di Summary | FAIL | teks | F1 |

Temuan F1 sampai F4 sudah diperbaiki di kode dan diuji dengan `dev-lens-findings.spec.ts` (Playwright). Suite penuh: 496 passed, 0 failed, 43 skipped. Verifikasi ulang di dev-lens menunggu deploy, yang belum disetujui.
