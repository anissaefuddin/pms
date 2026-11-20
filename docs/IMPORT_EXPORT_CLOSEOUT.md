# LENS Import/Export — Penutupan (closeout)

Lanjutan dari [IMPORT_EXPORT_FINAL.md](IMPORT_EXPORT_FINAL.md). Belum di-commit, belum di-push, belum di-merge, belum di-deploy. Migrasi 00342 dan 00343 dibuat dan diuji di basis data uji terisolasi, belum diterapkan ke basis data mana pun.

## 1. Keputusan atas lima blocker

| No. | Blocker | Keputusan | Status |
|---|---|---|---|
| 1 | Definisi IMP-T/EXP-T | Baseline asli dipakai, ID dipertahankan. Tafsiran audit dipisah sebagai ID tambahan (AUD-*). | **DONE** — lihat §2 |
| 2 | Migrasi jenis notifikasi | Disetujui; migrasi `00342_import_summary_notification.sql`. | **DONE** (diuji lewat suite penuh) |
| 3 | Migrasi import report | Disetujui; migrasi `00343_excel_import_reports.sql`. | **DONE** (diuji lewat suite penuh) |
| 4 | Filter Table View | Filter, search, sort, dan visible columns dari Table View aktif. | **PARTIAL** — lihat §3 |
| 5 | Dependency scheduling | Auto-reschedule terbatas; tanggal eksplisit dilindungi. | **DONE** |

Catatan migrasi: keduanya bersifat aditif. `00342` memperluas CHECK `notifications.kind` dengan `import_summary`; tidak ada baris lama yang berubah. `00343` membuat tabel baru. Down-migration ditulis, tetapi belum dijalankan terhadap basis data yang sudah termigrasi (tidak ada harness down di repo); perlu diuji sebelum penerapan.

## 2. Baseline asli: IMP-T01–T14 dan EXP-T01–T07

ID dipertahankan seperti di review awal. Tes yang disebut sudah dijalankan.

| ID | Skenario asli | Status | Bukti (tes) |
|---|---|---|---|
| IMP-T01 | Download Basic template | **DONE** | `TestExcelTemplatesAreVersionedAndScoped` |
| IMP-T02 | Import 10 Task valid | **DONE** | `TestIMPT02ImportTenValidTasks` |
| IMP-T03 | Import Project, Folder, Task, Subtask | **DONE** | `TestIMPT03ImportProjectFolderTaskSubtask` |
| IMP-T04 | Excel mengandung formula di B3 | **DONE** | `TestReadLENSFormulaNamesSheetAndCell` (unit) |
| IMP-T05 | Worksheet bukan data item | **DONE** | `TestReadLENSRejectsUnrecognizedFormat`, `TestProjectExcelRefusesUnrecognizedWorkbook` |
| IMP-T06 | Parent Key tidak ditemukan | **DONE** | `TestIMPT06ParentKeyNotFound` |
| IMP-T07 | Circular dependencies | **DONE** | cycle di `TestProjectExcelStatusAssigneeDependencyDestination` (kasus `dependency cycle`); siklus hierarki di `TestProjectExcelImport` |
| IMP-T08 | Status tidak ada di Workflow | **DONE** | `TestProjectExcelStatusAssigneeDependencyDestination` (kasus `unknown status`) |
| IMP-T09 | Assignee email tidak terdaftar | **DONE** | kasus `unknown email` dan `email without access` di tes yang sama |
| IMP-T10 | Custom Item Type invalid | **DONE** | `TestIMPT10InvalidCustomItemType` |
| IMP-T11 | Restricted Custom Field | **DONE** (diperbaiki) | `TestRestrictedFieldIsNotImportedOrExported`. Sebelumnya **FAIL**: impor bisa menulis field terlarang; kini ditolak di preview (`items/excel_import.go`, `readOnly`). |
| IMP-T12 | Import file yang sama dua kali | **DONE** | retry di `TestProjectExcelImport` (ID hasil sama) |
| IMP-T13 | Import ke Space tanpa create permission | **DONE** | `TestProjectExcelImport` (reader 403), `TestProjectExcelCrossSpacePermissions` (outsider 404) |
| IMP-T14 | Kegagalan validasi dalam batch tidak menghasilkan data parsial | **DONE** | `TestProjectExcelImport` (rollback, `atomic`), `reject invalid commit` |
| EXP-T01 | Export current Table View | **PARTIAL** | `TestExcelTableViewExport` (search, status, sort, kolom). Filter jenis dan field belum ikut. |
| EXP-T02 | Export complete Project | **DONE** | `TestProjectExcelExport` |
| EXP-T03 | Export dari Folder bersarang | **DONE** | `TestEXPT03ExportNestedFolder`, `TestProjectExcelExportsAFolder` |
| EXP-T04 | Export Custom Fields | **DONE** | `TestProjectExcelExport` (nilai `123.45`), unit `TestPlainValueUnwrapsScalarJSON` |
| EXP-T05 | Restricted field tidak bocor | **DONE** | `TestRestrictedFieldIsNotImportedOrExported` (header dan nilai tidak ada di unduhan reader); revalidasi saat unduh `TestProjectExcelRevokedFieldGrantBlocksDownload` |
| EXP-T06 | Import hasil Export kembali (round-trip) | **DONE** | `TestProjectExcelRoundTripKeepsDependencies`; `TestItemTypeInExportSearchAndReports` (status dikosongkan karena aturan destination) |
| EXP-T07 | Ekspor data besar | **DONE** | `TestEXPT07LargeProjectExport`: 500 item (termasuk project) berhasil; 501 gagal dengan pesan batas |

### ID tambahan (AUD-*): tafsiran audit yang belum terwakili baseline

| ID | Skenario | Status | Bukti |
|---|---|---|---|
| AUD-IMP-01 | Wizard 4 tahap | **PARTIAL** — dialog impor tetap satu layar dengan preview, mapping, dan destination. Langkah terpisah belum ada. | UI `space-excel-import.tsx` |
| AUD-IMP-02 | Pemilihan worksheet | **DONE** | lihat IMP-T05 |
| AUD-IMP-03 | Destination Space/Project/Folder | **DONE** (picker level atas Space) | `TestProjectExcelStatusAssigneeDependencyDestination` |
| AUD-IMP-04 | Dependency mengikuti kalender kerja | **DONE** | `TestProjectExcelCalendarDurationAndDependencyDates`, `TestProjectExcelReschedulesOnlyWithoutExplicitStart` |
| AUD-IMP-05 | Import report dan ringkasan | **DONE** | `TestProjectExcelImportReportAndSummaryNotification` |
| AUD-SEC-01 | Automation tidak berjalan untuk bulk | **DONE** | `TestBulkImportRunsNoAutomationByDefault` |
| AUD-SEC-02 | Webhook tidak terkirim untuk bulk | **DONE** | `TestBulkImportSendsNoWebhook` |
| AUD-SEC-03 | Cross-Space | **DONE** | `TestProjectExcelCrossSpacePermissions` |
| AUD-EXP-01 | Teks dari ekspor tidak dijalankan sebagai formula | **DONE** | `TestWorkbookLiteralText` |

## 3. Blocker yang masih terbuka

1. **EXP-T01 / Table View penuh (keputusan 4)**: UI sekarang meneruskan filter status (aktif, milik saya), pencarian judul, urutan (title, start, finish), dan kolom yang terlihat. **Belum** diteruskan: filter jenis item (`typeFilter`) dan filter custom field (`fieldFilter`). Keduanya tidak dikirim, jadi ekspor bisa lebih lebar daripada tabel di layar. Perlu keputusan: ikutkan keduanya (kerja backend tambahan) atau nyatakan tidak didukung di UI.
2. **Destination picker** hanya menampilkan Project/Folder level atas Space. Folder bersarang belum muncul (API listing tidak merekursi).
3. **Notifikasi ringkasan** dikirim ke pengimpor saja. Belum ada tautan dari notifikasi ke report di UI; report dibuka lewat `GET /api/v1/spaces/{spaceId}/excel-imports/{reportId}`.
4. **Down-migration** 00342 dan 00343 belum diuji di basis data yang sudah termigrasi.
5. **Status pada impor ke Space root** tetap ditolak karena aturan workflow destination. Ekspor yang memuat status tidak bisa diimpor ke root tanpa destination.

## 4. Bukti verifikasi

| Pemeriksaan | Hasil |
|---|---|
| `go vet ./server/...` | bersih |
| `gofmt -l server` | bersih |
| `go test -count=1 ./server/...` dengan PostgreSQL 16 terisolasi (localhost:55500) | **PASS**, exit 0, 22 paket `ok` |
| `TestBrowserExcelRoundTrip` (Playwright, backend hidup) | **PASS** |
| `project-excel.spec.ts` (Playwright mock) | **PASS** 2/2 |
| `npm run test` web | **PASS** 92/92 |
| `npm run typecheck` web | **FAIL**: 4 error di file automation (`workspace-automation.tsx:123,125`, `settings/automation.tsx:63`). Bukan dari perubahan ini. Tidak ada error di file Excel atau `view-workspace.tsx`. |

Tes yang ditulis merah dulu: IMP-T11 (gagal lalu diperbaiki), tes kalender, report, notifikasi, auto-reschedule, filter ekspor, dan pembatasan 500 item. Tes Table View (`TestExcelTableViewExport`) ditulis setelah implementasi, jadi tidak ada bukti merah untuk itu.

Catatan tambahan: `TestItemTypeInExportSearchAndReports` diubah. Impor ulang ke Space root dengan status ditolak; tes sekarang mengosongkan kolom status sebelum impor ulang.

## 5. GO / NO-GO

**GO** untuk pekerjaan yang sudah ditutup (IMP-T01–T14, EXP-T01–T07 baseline kecuali EXP-T01 penuh; blocker 2, 3, 5; AUD-*). Semua lulus di suite penuh.

**NO-GO** untuk rilis penuh sampai blocker 4 (filter Table View penuh) diputuskan dan down-migration diuji.

## 6. File bersama yang masih berisi perubahan tak ter-commit

`server/internal/automation/engine.go`, `server/internal/events/events.go`, `server/internal/items/create.go`, `server/internal/items/excel_import.go`, `server/internal/projectexcel/*`, `server/internal/httpserver/project_excel*.go`, `web/src/components/{project-excel-export,space-excel-import,view-workspace,item-detail}.tsx`, `web/src/i18n/{en,id}/excel.json`. Beberapa file sudah punya perubahan dari pekerjaan lain. Commit harus dipisah per hunk.
