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

### Tes ulang di dev-lens setelah deploy b82e6ec (health 200, backup 20261116T020947Z)

| Test ID | Scenario | Actual | Status |
|---|---|---|---|
| F1 | Keterangan cakupan KPI tampil di halaman project tanpa membuka Summary | Teks "Tasks in this project and its folders and subprojects; milestones excluded" terlihat | PASS (sebelumnya FAIL, DEV-05) |
| F2 | Breadcrumb di halaman folder | "QA | Engineering / QA-E2E03 Folder", h1 tampil sekali | PASS |
| F3 | Footer dialog Add item di folder | "Task · QA | Engineering · QA-E2E03 Folder"; teks "Space root" tidak ada | PASS |
| F4 | Tipe di bawah task | Project dan Folder radio disabled, Task terpilih, kelas gaya terkunci aktif | PASS |

## E2E-04 di dev-lens (cross-view status, item uji QA-E2E03 Task)

| Test ID | Page | Scenario | Expected | Actual | Status | Evidence | Defect reference |
|---|---|---|---|---|---|---|---|
| E2E-04a | Folder, Table | Ubah status New → In Progress lewat dropdown | Tersimpan | PATCH /items/{id} 200 | PASS | network log | - |
| E2E-04b | Folder, Board | Task pindah ke kolom In Progress | Kolom In Progress 1, New 0 | In Progress 1 (kartu QA-E2E03 Task), New 0 | PASS | screenshot | - |
| E2E-04c | Project, KPI | Work items dan In progress sesuai | Work items 2 (task + subtask), In progress 1 | Work items 2, In progress 1 | PASS | teks strip | - |
| E2E-04d | Gantt, Calendar | Status presentation | Bar dan event dengan warna status | NOT RUN: folder dan project hanya punya view Table dan Board; menambah view adalah mutasi yang belum disetujui | NOT RUN | - | - |
| E2E-04e | Chart | Agregat chart | Sama dengan KPI | NOT RUN: tidak ada view Chart di folder atau project QA ini | NOT RUN | - | - |

Status item uji tetap In Progress setelah tes. Tidak ada data operasional yang diubah.

### E2E-04d dan E2E-04e setelah deploy df3c6e6 (health 200, backup 20261116T023345Z)

| Test ID | Page | Scenario | Expected | Actual | Status | Evidence | Defect reference |
|---|---|---|---|---|---|---|---|
| E2E-04d-cal | Folder, Calendar | Warna status pada item tanpa tanggal (daftar Unscheduled) | Chip memakai warna status In Progress | Chip "QA-E2E03 Task" bg-blue-100 text-blue-900 | PASS | DOM | - |
| E2E-04d-gantt | Folder, Gantt | Warna status pada bar | Bar memakai warna status | NOT RUN: kedua item belum punya tanggal, bar lebar 0. Diuji setelah E2E-05 memberi jadwal | NOT RUN | - | - |
| E2E-04e | Project, Chart | Agregat chart sama dengan KPI | Total 2, In Progress 1, New 1 | Total 2; In Progress 1 (50%); New 1 (50%) | PASS | teks chart | Chart sebelumnya FAIL (menghitung folder), diperbaiki di df3c6e6 |

## E2E-05 di dev-lens (jadwal item uji QA-E2E03 Task)

| Test ID | Page | Scenario | Expected | Actual | Status | Evidence | Defect reference |
|---|---|---|---|---|---|---|---|
| E2E-05a | Task, panel | Beri jadwal 18–20 Nov 2026 lewat Dates | Tersimpan | PATCH 200 untuk start dan finish; reload menampilkan 2026-11-18 – 2026-11-20 | PASS | network log, reload | - |
| E2E-05b | Folder, Table | Start dan Finish pada baris | 2026-11-18 dan 2026-11-20 | Sesuai | PASS | teks baris | - |
| E2E-05c | Folder, Gantt | Bar multi-hari dan warna status | Lebar 3 hari, warna In Progress | Bar summary lebar 108 px; fill gradasi biru (In Progress) | PASS | DOM | - |
| E2E-05d | Folder, Calendar | Rentang event | Satu bar 18–20 Nov | Satu bar "QA-E2E03 Task, 2026-11-18 to 2026-11-20", lebar 347 px, tanpa duplikat | PASS | DOM | - |
| E2E-05e | Task, panel | Jadwal tidak valid: finish 10 Nov sebelum start 18 Nov | Ditolak, data tidak berubah | UI menampilkan "Finish date must be on or after the start date"; tidak ada PATCH; setelah reload tetap 18–20 Nov | PASS (UI dan data) | alert, log, reload | Penolakan di sisi server belum diuji langsung |
| E2E-05f | Gantt | Drag untuk menjadwal ulang | Tanggal berubah sesuai aturan | NOT RUN: tidak dijalankan untuk menghindari perubahan tambahan pada item uji | NOT RUN | - | - |

## E2E-06 di dev-lens (kalender lintas minggu dan bulan, item uji QA-E2E03 Task)

| Test ID | Page | Scenario | Expected | Actual | Status | Evidence | Defect reference |
|---|---|---|---|---|---|---|---|
| E2E-06a | Folder, Calendar | Item lintas minggu dan bulan (27 Nov – 2 Des 2026) | Dua segmen berlanjut, satu per baris minggu | Dua bar, lebar sama, di dua baris minggu | PASS | DOM | - |
| E2E-06b | Folder, Calendar | Tanggal segmen | Sesuai rentang item | Label "2026-11-27 to 2026-12-02" pada kedua segmen | PASS | aria-label | - |
| E2E-06c | Folder, Calendar | Identitas item, tanpa duplikat | Satu item di Table | Satu baris di Table; dua segmen di Calendar berasal dari satu item | PASS | Table dan DOM | - |
| E2E-06d | Folder, Calendar | Penanda lanjutan | Label aksesibel dalam kata-kata | Label menampilkan kunci mentah "views.calendar.continuesAfter" | FAIL (P2) | aria-label | Kunci i18n belum ada; diperbaiki di kode (continuesBefore/continuesAfter ditambahkan) dan belum di-deploy |
| E2E-06e | Folder, Calendar | Pindah dengan keyboard (Alt+Right) | Rentang geser 1 hari, kedua segmen ikut | 27 Nov–2 Des menjadi 28 Nov–3 Des; Table ikut berubah | PASS | aria-label, Table | - |
| E2E-06f | Calendar | Pindah dengan drag | Rentang berubah, tanpa item baru | NOT RUN | NOT RUN | - | - |
| E2E-06g | Calendar | Pergantian bulan (navigasi ke Desember) | Segmen tampil di bulan berikutnya | NOT RUN | NOT RUN | - | - |
| E2E-06h | Calendar | Overflow (+N) di dev-lens | Daftar terbuka dengan item tersembunyi | NOT RUN di dev-lens; tercakup tes mock (calendar-c2.spec) | NOT RUN | - | - |

## Phase 3 local QA pass (16 Nov 2026)

Environment: local `pms_qa` database, Go API on localhost:8080, Vite on localhost:5173. Seed: `docs/qa/seed/ux_qa_seed.py` (ledger `ux_qa_ledger.json`). dev-lens was not used.

Test ID | Page | Scenario | Expected result | Actual result | Status | Evidence | Defect reference
--- | --- | --- | --- | --- | --- | --- | ---
P3-E01-a | Project | Gantt tree shows statuses and bars | Tree and bars for seeded items | Tree, statuses and bars shown | PASS | browser screenshot 16 Nov | none
P3-E01-b | Project | Calendar shows tasks inside folders | Bars for scheduled tasks in folders | Only folders listed before fix; bars shown after fix | PASS (after fix) | QA-P3-01; browser screenshot | DEF-P3-01
P3-E01-c | Project | Timelog empty state | Empty state message | "No time entries yet" | PASS | browser screenshot | none
P3-E01-d | Project | Files empty state | Empty state message | "No files in this project" | PASS | browser screenshot | none
P3-E01-e | Project | Board view | Columns by status | Not run in this pass | NOT RUN | none | none
P3-E01-f | Project | Chart view | Chart of the subtree | Not run in this pass | NOT RUN | none | none
P3-E02 | Project | Saved view lifecycle incl. second user | Personal view private, public view shared | Not run; needs second user | BLOCKED | none | none
P3-E03 | Project | Contextual creation | Hierarchy and type correct | Passed on dev-lens earlier; not re-run locally | NOT RUN | WORK_AREA_TEST_EVIDENCE.md (dev-lens sections) | none
P3-E04 | Project | Status sync across views | Same status in Table, Board, Gantt, Calendar, Chart | Not run | NOT RUN | none | none
P3-E05 | Project | Schedule sync | Dates match in Gantt, Calendar, Table, detail | Not run | NOT RUN | none | none
P3-E06-a | Calendar | Cross-week segments render | Continuous segments across week rows | "Late review" and "Equipment Datasheet" segments render across rows | PASS | browser screenshot | none
P3-E06-b | Calendar | Keyboard move and month transition | Move and synced views | Not run | NOT RUN | none | none
P3-E07 | Detail | Panel from each view | Panel keeps view, filters, selection | Not run | NOT RUN | none | none
P3-E08 | Approvals | Approval lifecycle | Decisions and guards | Needs approvers other than admin | BLOCKED | none | none
P3-E09 | Files | Upload and version | Metadata and history | Not run (files empty; no upload) | NOT RUN | none | none
P3-E10 | Timelog | Entries, filters, totals, locked | Totals and rules | Empty state only; no entries seeded | PARTIAL | browser screenshot | none
P3-E11-a | Project | Work items equals analytics total | Same number | 28 = 28 | PASS | API analytics query and header | none
P3-E11-b | Project | In progress equals In Progress count | Same number | 10 = 10 | PASS | API and header | none
P3-E11-c | Project | Upcoming milestones counts nested milestones | 2 for seed | 0 before fix, 2 after | PASS (after fix) | QA-P3-02 | DEF-P3-02
P3-E12 | Mock suite | Failed load, retry, upload retry, star failure | Clear feedback and retry | Covered by mock tests (in 499 passed) | PASS (mocked) | `npx playwright test` | none
P3-VIS-01 | All | Resolutions 1280, 1440, 1920 | No overflow, readable | Not run | NOT RUN | none | none
P3-VIS-02 | All | Light theme, compact and comfortable density | Readable | Only dark observed | NOT RUN | none | none
