# LENS Import/Export — Laporan finalisasi (Phase 3)

Lanjutan dari [IMPORT_EXPORT_PHASE2.md](IMPORT_EXPORT_PHASE2.md). Belum di-commit, belum di-push, tidak ada migrasi, tidak ada deploy.

## 1. Status per pekerjaan

Status: **DONE** (selesai, ada tes), **PARTIAL** (sebagian, bagian sisanya disebut), **BLOCKED** (menunggu input atau persetujuan), **FAIL** (tes gagal).

### Lane A — Import

| Pekerjaan | Status | Bukti |
|---|---|---|
| Scheduling dependency dengan kalender kerja | **PARTIAL** — FS dicek terhadap tanggal di kalender (successor harus mulai setelah predecessor selesai); tanggal item **tidak** dijadwal ulang saat impor | `items/excel_import.go` (`dependencyVisit`, pengecekan `startDay`/`finishDay`), tes `TestProjectExcelCalendarDurationAndDependencyDates` |
| Start/Finish eksplisit dan validasi Duration | **DONE** — Duration harus sama dengan jumlah hari kerja Start..Finish; Finish kosong dihitung dari Start + Duration di kalender | `excel_import.go` (blok `cal.Duration` / `cal.Finish`), tes yang sama (`duration_days` error, `G2` Finish = 18 Nov) |
| Destination Picker (Space root, Project, Folder) | **DONE** untuk root, Project, dan Folder **level atas** Space; Folder bersarang belum muncul di picker | backend `checkDestination`; UI `web/src/components/space-excel-import.tsx` (select dari `GET /spaces/{id}/items?type=project,folder`); tes `TestProjectExcelStatusAssigneeDependencyDestination` (tujuan task ditolak) |
| Import Report + satu notifikasi ringkasan | **PARTIAL** — ringkasan di respons 201 (`summary.created`, `summary.dependencies`). **BLOCKED** untuk notifikasi: tabel `notifications` memiliki CHECK `kind IN (...)` yang tidak memuat jenis ringkasan; menambah jenis butuh migrasi (perlu persetujuan). Laporan tersimpan juga butuh kolom baru (migrasi). | `server/migrations/00011_notifications.sql:6` |

### Lane B — Export

| Pekerjaan | Status | Bukti |
|---|---|---|
| Ekspor berdasarkan filter Table View aktif, parent dipertahankan | **PARTIAL** — backend menerima filter `status_groups` dan `assignee`; folder/project yang memuat hasil tetap ikut (parent dipertahankan). UI hanya menyediakan pilihan cakupan (semua / aktif / selesai), **belum** membaca state filter tabel yang sedang aktif. Filter pencarian dan custom field belum didukung. | `projectexcel/export.go` (`ExportFilter`, `filterMatches`, `keepWithParents`), `httpserver/project_excel.go` (validasi filter), tes `TestProjectExcelFilteredExportKeepsParents` |
| Dependency, workflow, status pada workbook | **DONE** — kolom `Status`, `Workflow`, `Depends On` (FS saja, `External Key:FS` dengan ID item). Dependency dengan tipe lain tidak diekspor. | `export.go` (`ExportHeaders`, query dependency), tes `TestProjectExcelRoundTripKeepsDependencies` (EXP-T08) |
| Format ekspor bisa diproses kembali oleh importer | **DONE** dengan batasan: round trip ke Space root berhasil untuk dependency dan tipe. Status **hanya** bisa diimpor ke destination yang workflow-nya memuat status itu; impor ke Space root dengan status ditolak (sesuai aturan). Tes lama `TestItemTypeInExportSearchAndReports` disesuaikan: status dikosongkan sebelum impor ulang. | `TestProjectExcelRoundTripKeepsDependencies`, `TestItemTypeInExportSearchAndReports` (diubah) |

### Lane C — QA dan kontrak

| Pekerjaan | Status | Bukti |
|---|---|---|
| Pulihkan definisi IMP-T01–T14 dan EXP-T01–T07 dari sumber review asli | **BLOCKED** — dicari di Outline (`outline_search` untuk "IMP-T01" dan "EXP-T01 import export Excel": hasil kosong), di repo (`grep IMP-T0|EXP-T0`: hanya dokumen analisa yang saya tulis), dan di folder catatan sesi (tidak ada). Definisi asli perlu diberikan. | — |
| EXP-T08 sebagai skenario tambahan eksplisit | **DONE** — didefinisikan sebagai round trip ekspor→impor yang menjaga dependency. Ini **bukan** bagian baseline lama. | `TestProjectExcelRoundTripKeepsDependencies` |
| Acceptance test untuk pekerjaan terbuka | **DONE** untuk item di Lane A/B di atas (lihat tabel tes §2). | §2 |
| Verifikasi ulang: automation/webhook pada impor | **PARTIAL** — automation dicek lewat tes (`TestBulkImportRunsNoAutomationByDefault`). Webhook memakai guard di jalur yang sama (`automation/engine.go` `Dispatch`) tetapi **belum ada tes webhook khusus**. | `automation/engine.go`, `items/create.go`, `events/events.go` |
| Verifikasi ulang: revalidasi hak saat unduh | **DONE** — tes: grant dicabut → unduhan lama 404. | `TestProjectExcelRevokedFieldGrantBlocksDownload` |

## 2. Test matrix

Basis data: PostgreSQL 16 terisolasi di `localhost:55500` (dibuat untuk sesi ini; sudah dihentikan).

| Lapisan | Perintah | Hasil |
|---|---|---|
| Unit Go (`projectexcel`) | `go test ./server/internal/projectexcel/` | **PASS** |
| Integrasi Go (paket lain + DB) | `PMS_TEST_DATABASE_URL=… go test ./server/...` | **PASS** — exit 0, 22 paket `ok` (httpserver ±135 s) |
| Integrasi Excel (tes baru Phase 3) | `TestExcelTemplatesAreVersionedAndScoped`, `TestProjectExcelExportsAFolder`, `TestProjectExcelStatusAssigneeDependencyDestination`, `TestProjectExcelCalendarDurationAndDependencyDates`, `TestProjectExcelRoundTripKeepsDependencies`, `TestProjectExcelFilteredExportKeepsParents` | **PASS** |
| Integrasi keamanan (Phase 1) | `TestProjectExcelRevokedFieldGrantBlocksDownload`, `TestBulkImportRunsNoAutomationByDefault`, `TestProjectExcelImportMarksBulkOriginEvents`, `TestProjectExcelCrossSpacePermissions` | **PASS** |
| E2E Go + Playwright (backend hidup) | `PMS_TEST_BROWSER=1 go test -run TestBrowserExcelRoundTrip` | **PASS** (1/1) |
| Playwright mock | `npx playwright test project-excel.spec.ts` | **PASS** (2/2) |
| Unit web | `npm run test` | **PASS** (92/92) |
| Typecheck web | `npm run typecheck` | **FAIL** — 4 error di file automation (`workspace-automation.tsx:125`, `settings/automation.tsx:63`). Bukan dari perubahan ini; tidak ada error di file Excel. |
| Tes yang di-skip | — | Tidak ada tes Excel yang di-skip di jalur yang dijalankan. |

Tes merah dulu: tes baru untuk template, folder, status/assignee/dependency/destination, kalender, filter, dan round trip ditulis dan dilihat gagal sebelum implementasi.

## 3. Blocker yang tersisa

1. **Definisi asli IMP-T01–T14 dan EXP-T01–T07** — tidak ditemukan di Outline, repo, maupun catatan sesi. Perlu diberikan (atau lokasinya ditunjukkan).
2. **Notifikasi ringkasan impor** — perlu migrasi untuk menambah jenis notifikasi (`notifications.kind`), dan persetujuan migrasi.
3. **Import report tersimpan** — perlu migrasi tabel/kolom baru dan persetujuan.
4. **Filter Table View penuh** — perlu keputusan UI: filter mana yang ikut (pencarian, custom field) dan bagaimana state tabel diteruskan ke dialog ekspor.
5. **Penjadwalan ulang saat impor** — dependency yang masuk tidak menggeser tanggal successor. Perlu keputusan: geser otomatis, atau tolak (sekarang: tolak jika successor mulai tidak setelah predecessor selesai).

## 4. Risiko tersisa

- Ekspor status/workflow menjadikan round trip ke Space root tidak bisa membawa status (ditolak dengan pesan jelas).
- Dependency non-FS tidak diekspor (SS/FF/SF) dan tidak bisa diimpor (MVP FS saja).
- Folder bersarang belum muncul di picker destination (UI hanya level atas Space).
- Webhook belum punya tes khusus untuk bulk import.
- Typecheck web gagal di 4 file automation yang bukan bagian pekerjaan ini.
- Banyak file bersama (`automation/engine.go`, `events/events.go`, `items/create.go`, `items/excel_import.go`, `httpserver/project_excel*.go`) masih berisi perubahan tak ter-commit dari pekerjaan sendiri dan pekerjaan lain. Commit harus dipisah.

## 5. Keputusan GO/NO-GO

**Belum GO untuk rilis fitur penuh.** Lane A dan B sudah berfungsi di jalur utama dan lulus tes integrasi, E2E, dan unit. Tetapi notifikasi ringkasan, report, dan filter Table View penuh masih terblokir, dan definisi skenario asli belum tersedia.
