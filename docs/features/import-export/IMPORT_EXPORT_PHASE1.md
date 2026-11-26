# LENS Import/Export — Phase 1 (security & validation hardening)

Baseline: [IMPORT_EXPORT_ANALISA.md](IMPORT_EXPORT_ANALISA.md). Belum di-commit, belum di-push.

## 1. Cakupan Phase 1 dan keputusan yang diterapkan

Phase 1 mengerjakan Agent A (keamanan dan kontrol event), Agent B (validasi workbook), dan bagian Agent C (tes dan verifikasi). Keputusan produk yang disebut di pesan user dipetakan seperti ini:

| No. | Keputusan | Status di Phase 1 |
|---|---|---|
| 1 | Header publik External Key / Parent Key / Finish Date, alias lama tetap | **Diterapkan** (impor dan ekspor) |
| 2 | Status dengan konteks workflow, error jika tidak ketemu | Belum (Phase 2) |
| 3 | Assignee dicocokkan lewat email | Belum (Phase 2) |
| 4 | Destination Space root / Project / Folder | Belum (Phase 2) |
| 5 | Bulk import: automation, webhook, notifikasi per-item mati; audit internal tetap | **Diterapkan** (lihat §3) |
| 6 | Ekspor Folder, filter Table View | Belum (Phase 2) |
| 7 | Nilai custom field human-readable | Belum (Phase 2; EXP-06 tetap DEFECT) |
| 8 | Limit 500 item ditampilkan jelas | Belum (Phase 2) |
| 9 | Dependency `T001:FS` | Belum (Phase 2) |
| 10 | Tanggal dan Duration divalidasi, kalender kerja | Belum (Phase 2; NOT VERIFIED) |
| 11 | Template server-generated | Belum (Phase 2) |
| 12 | Skenario sesuai definisi original | **Terblokir** (lihat §5) |
| 13 | Grant dicabut → file lama tidak bisa diunduh | **Diterapkan** (lihat §3) |
| 14 | Satu ringkasan import, detail di import report | Sebagian: per-item dihentikan; ringkasan belum dibuat |

## 2. Gap matrix terbaru (yang berubah dari baseline)

| ID | Baseline | Sekarang | Bukti |
|---|---|---|---|
| IMP-04 | PARTIAL | **PARTIAL+** — error menyebut sheet, row, column, reason, suggested_fix | `projectexcel/xlsx.go` (`Error`, `ReadLENS`), `items/excel_import.go` (`ExcelIssue`, `issueFix`), `httpserver/project_excel.go` (`writeWorkbookError`) |
| IMP-T05 | Belum | **Sudah** — workbook tanpa header LENS ditolak "format is not recognized" | `xlsx.go` `ReadLENS`; tes `TestReadLENSRejectsUnrecognizedFormat`, `TestProjectExcelRefusesUnrecognizedWorkbook` |
| IMP-T06 | Belum | **Sudah** — sheet LENS dipilih; ambigu ditolak | `TestReadLENSPicksTheLENSSheet`, `TestReadLENSAmbiguousSheets` |
| IMP-T07 | Sudah | Sudah, dengan sheet dan saran | `TestReadLENSFormulaNamesSheetAndCell` |
| IMP-03 | PARTIAL | PARTIAL (header publik + alias sudah; penjelasan kolom belum) | `xlsx.go` `Canonical`, `export.go` `ExportHeaders` |
| EXP-06 | DEFECT | DEFECT (tidak diubah di Phase 1; kebijakan revalidasi saja) | `export.go` |
| Revoked grant (P0) | DEFECT | **DONE** — dicek saat unduh | `projectexcel/export.go` `Readable`, `httpserver/project_excel.go` (file branch) |
| Automation/webhook saat bulk (P1) | DEFECT | **DONE** (default off) | `events/events.go`, `automation/engine.go` (`Dispatch`, `Handle`), `items/create.go` |
| Cross-Space (P0) | PARTIAL | **DONE** (kode + tes regresi) | `TestProjectExcelCrossSpacePermissions` |

## 3. Defect fixes dengan bukti file

**a. Revoked field grant pada unduhan ekspor (Agent A)**
- Masalah: `CanDownload` hanya mengecek View Space dan isi tree; file yang sudah berisi field lalu dicabut tetap bisa diunduh.
- Perbaikan: `projectexcel.Readable` membaca header `field:<id>` dari workbook tersimpan, menghitung field yang tidak bisa dibaca aktor, dan menolak jika ada field terlarang. Formula yang ekspresinya menyebut field terlarang juga ditolak, sama dengan aturan ekspor.
- Di mana: `server/internal/projectexcel/export.go` (fungsi `Readable`), `server/internal/httpserver/project_excel.go` (cabang `/file`, 404 dengan pesan "create a new export").
- Tidak ada migrasi: kolom baru tidak diperlukan.

**b. Kebijakan event bulk import (Agent A)**
- Masalah: setiap create memancarkan `item.created` yang juga dibaca automation dan webhook.
- Perbaikan: event bulk diberi `payload.origin = "bulk_import"` (`events.WithBulkImport`, `events.Emit`). Automation (`Handle` dan `Dispatch`) dan hook webhook melewatinya. Activity (audit internal) dan `domain_events` tetap ditulis. Notifikasi per-item tidak dibuat untuk create bulk.
- Di mana: `server/internal/events/events.go`, `server/internal/automation/engine.go`, `server/internal/items/create.go` (blok notifikasi), `server/internal/items/excel_import.go` (`ctx = events.WithBulkImport(ctx)` saat commit).

**c. Pemilihan worksheet dan deteksi format (Agent B)**
- Masalah: parser selalu mengambil sheet pertama dan menerima workbook apa pun.
- Perbaikan: `ReadSheets` membaca semua sheet (batas 64). `ReadLENS` memilih sheet yang punya header LENS; tidak ada → `format`; lebih dari satu → `sheet` (kecuali ada yang bernama Project). Formula hanya dicek pada sheet terpilih; sheet lain tidak menghalangi.
- Di mana: `server/internal/projectexcel/xlsx.go`.

**d. Error terstruktur dan alias header (Agent B)**
- `ExcelIssue` sekarang memuat `sheet`, `reason` (`column`/`row`/`value`), dan `suggested_fix`. Field lama (`row`, `column`, `message`) tetap.
- `Canonical` memetakan `External Key`→`id`, `Parent Key`→`parent_id`, `Finish Date`→`due_date`; nama lama tetap.
- Ekspor memakai nama publik (`ExportHeaders`); impor lama tetap bisa.
- Di mana: `items/excel_import.go`, `projectexcel/xlsx.go`, `projectexcel/export.go`.

**e. Transaksi dan idempotensi (tidak diubah)**
- Logika transaksi, digest, dan receipt tidak disentuh. Tes lama `rolls back earlier rows` dan `retry same import` tetap lulus.

## 4. Tes: PASS / FAIL / SKIP

Basis data: PostgreSQL 16 terisolasi di `localhost:55500`, data dir di scratchpad, dibuat khusus untuk sesi ini (bukan `5432`, bukan `pms_qa`/`maloka_dev`).

Metode merah→hijau: setiap tes baru dijalankan dulu tanpa perbaikan.

| Tes | Merah sebelum | Hijau sesudah | Catatan |
|---|---|---|---|
| `TestReadLENSPicksTheLENSSheet` | compile error | PASS | unit |
| `TestReadLENSRejectsUnrecognizedFormat` | compile error | PASS | unit |
| `TestReadLENSFormulaNamesSheetAndCell` | compile error | PASS | unit |
| `TestReadLENSAmbiguousSheets` | compile error | PASS | unit |
| `TestCanonicalHeaderAliases` | compile error | PASS | unit |
| `TestProjectExcelExport` (header publik) | FAIL | PASS | diperbarui: `External Key`/`Finish Date` |
| `TestProjectExcelPublicHeadersAndStructuredErrors` | FAIL | PASS | alias + sheet + suggested_fix |
| `TestProjectExcelRefusesUnrecognizedWorkbook` | FAIL | PASS | 422 format |
| `TestProjectExcelCrossSpacePermissions` | (regresi) | PASS | tidak ada perubahan perilaku; sudah benar |
| `TestProjectExcelImportMarksBulkOriginEvents` | FAIL | PASS | origin di `domain_events`; `activity` tetap 1 |
| `TestProjectExcelRevokedFieldGrantBlocksDownload` | FAIL | PASS | 200 → 404 setelah grant dicabut |
| `TestBulkImportRunsNoAutomationByDefault` | FAIL (saat guard dimatikan sementara) | PASS | kontrol: rule tetap jalan untuk item manual |
| `TestWorkbookLiteralText`, `TestWorkbookRejectsInvalidData`, `TestWorkbookReadsRelationshipsAndSparseCells`, `TestWorkbookWriteBounds` | — | PASS | regresi |
| `TestProjectExcelImport`, `TestItemTypeInExportSearchAndReports` | — | PASS | regresi |
| `TestBrowserExcelRoundTrip` | — | **SKIP** | butuh Chrome dan `PMS_TEST_BROWSER=1`; tidak dijalankan |
| Web `project-excel.spec.ts`, `excel-roundtrip-live.spec.ts` | — | **NOT RUN** | belum dijalankan |

Suite penuh `go test -count=1 ./server/...` dengan `PMS_TEST_DATABASE_URL` ke basis data terisolasi: **semua paket `ok`, exit 0** (httpserver 133 s). Catatan: paket lain yang bertanda `[no test files]` tidak diuji.

## 5. Terblokir: sinkronisasi skenario IMP-T / EXP-T

Keputusan 12 meminta definisi skenario asli dari review fitur. Definisi itu tidak ada di repo maupun di percakapan ini, jadi saya **tidak** mengganti tafsiran audit dengan definisi baru. Daftar tafsiran di `IMPORT_EXPORT_ANALISA.md` §7 tetap berlaku sampai definisi asli diberikan.

## 6. Risiko teknis yang tersisa

1. **EXP-06**: nilai custom field masih JSON mentah (`"Done"`), belum human-readable (keputusan 7).
2. **Preview belum menjalankan aturan create** (IMP-08): error dari izin lokasi atau constraint baru muncul saat commit.
3. **Ringkasan import belum ada** (keputusan 14): import report dan notifikasi ringkasan belum dibuat. Saat ini bulk import tidak mengirim notifikasi sama sekali.
4. **Webhook replay**: `Dispatch` melewati event bulk; jalur replay webhook (jika ada) tidak diperiksa.
5. **Event bulk masih ada di `domain_events`**: konsumen lain yang membaca tabel itu langsung tidak diperiksa.
6. **Revalidasi formula**: aturan formula di `Readable` diuji lewat kode, belum punya tes khusus untuk formula yang menyebut field terlarang.
7. **UI impor** belum tahu alias: auto-mapping di `space-excel-import.tsx` hanya mencocokkan `id`/`parent_id`/`due_date`, jadi `External Key` tampil sebagai "unmapped" meski server menerimanya.
8. **File bersama dengan perubahan lain**: `automation/engine.go`, `events/events.go`, dan `items/create.go` sudah punya perubahan tak ter-commit dari pekerjaan lain sebelum sesi ini. Hunk saya ada di sana; commit harus dipisah dengan hati-hati.
9. **Migrasi**: tidak ada. Tidak ada yang perlu persetujuan migrasi di Phase 1.
10. **Browser e2e** dan web spec belum dijalankan.

## 7. GO / NO-GO untuk Phase 2

**NO-GO sampai dua hal ini selesai:**
1. Definisi asli IMP-T01–T14 dan EXP-T01–T07 disediakan dan dipetakan ke tes (keputusan 12).
2. Browser e2e (`TestBrowserExcelRoundTrip`) dan web spec Excel dijalankan dan hijau, atau diputuskan tidak perlu.

Keamanan Phase 1 (revalidasi grant, event bulk, cross-Space, validasi workbook) sudah hijau di tes integrasi. Tidak ada commit, push, merge, atau deploy.
