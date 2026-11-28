# LENS Import/Export — Phase 2 (kontrak dan fitur yang tidak butuh keputusan baru)

Lanjutan dari [IMPORT_EXPORT_PHASE1.md](IMPORT_EXPORT_PHASE1.md). Belum di-commit, belum di-push, tidak ada migrasi.

## 1. Yang dikerjakan

| Keputusan | Hasil | Bukti |
|---|---|---|
| 2 Status dengan konteks Workflow | **Diterapkan**: kolom `Status` (dan `Workflow` opsional, harus sama dengan workflow tujuan). Nama tidak ditemukan atau ambigu → error di preview, tidak diganti otomatis. | `items/excel_import.go` (`resolveStatus`), tes `TestProjectExcelStatusAssigneeDependencyDestination` |
| 3 Assignee lewat email | **Diterapkan**: `Assignee Email` (bisa beberapa, dipisah `;`). Email harus user aktif yang bisa mengakses Space; selain itu error. | `resolveAssignee`, tes yang sama (email tak dikenal dan tanpa akses) |
| 4 Destination | **Sebagian**: backend menerima `destination` (Project/Folder dalam Space, izin Create dicek). Root row masuk ke bawah destination; project tidak boleh di bawah destination. **UI picker belum dibuat.** | `checkDestination`, `httpserver/project_excel.go` (field `destination`, ikut digest) |
| 7 Nilai custom field human-readable | **Sebagian**: nilai skalar (teks, angka, tanggal, pilihan, checkbox) ditulis sebagai teks biasa, bukan JSON mentah. People tetap berupa ID agar round-trip impor tetap cocok. | `projectexcel/export.go` (`plainValue`), tes unit `TestPlainValueUnwrapsScalarJSON` |
| 6 Ekspor Folder | **Diterapkan** di backend dan menu UI untuk folder. **Filter Table View belum** (butuh state tabel dari UI). | `projectexcel/export.go` (`CheckScope`), `item-detail.tsx`, tes `TestProjectExcelExportsAFolder` |
| 8 Limit 500 jelas | **Diterapkan**: error terstruktur `reason=limit` dengan saran "maksimal 500 item". | `xlsx.go` (`rowLimitError`), tes `TestRowLimitIsStructuredAndNamesTheFix` |
| 9 Dependency `T001:FS` | **Sebagian**: kolom `Depends On` dengan `External Key:FS` (atau tanpa tipe = FS). Hanya FS, tanpa lag. Predecessor harus baris di file; self dan siklus ditolak. Disimpan setelah semua item dibuat. | `excel_import.go` (`depends`, `dependencyVisit`), tes `TestProjectExcelStatusAssigneeDependencyDestination` |
| 11 Template server-generated, versi, sesuai field Space | **Diterapkan**: `GET /api/v1/spaces/{id}/excel-templates/{basic|hierarchy|advanced}`, nama file memuat `v1`, kolom advanced = custom field Space yang bisa dibaca aktor. UI: tautan unduh di dialog impor. | `projectexcel/template.go`, `httpserver/project_excel.go` (`excelTemplate`), tes `TestExcelTemplatesAreVersionedAndScoped` |
| 14 Ringkasan import | **Sebagian**: respons 201 memuat `summary` (created, dependencies). Notifikasi ringkasan dan import report tersimpan **belum**. Per-item notifikasi sudah dimatikan untuk bulk. | `httpserver/project_excel.go` |

## 2. Yang belum (dan kenapa)

- **Filter Table View untuk ekspor (keputusan 6 bagian filter, EXP-01/04)**: butuh state filter tabel dari UI yang belum dihubungkan ke API ekspor.
- **Tanggal, Duration, dan kalender kerja (keputusan 10)**: belum divalidasi (sudah diputuskan, belum dikerjakan). Dependency yang disimpan saat impor **tidak** menjadwal ulang tanggal item (tidak memanggil logika reschedule), jadi drift tanggal masih mungkin. Keputusan 10 sudah menyebut kalender kerja yang berlaku; implementasinya belum dikerjakan di sesi ini.
- **Ringkasan dan import report (keputusan 14)**: notifikasi satu ringkasan ke pengimpor dan penyimpanan report belum dibuat.
- **Ekspor dependency dan workflow/status (EXP-07, EXP-08)**: belum ditulis ke file ekspor. Ekspor belum punya kolom status/assignee/priority/effort (EXP-05).
- **UI picker destination** belum ada; backend sudah menerima `destination`.
- **Status pada baris anak** diperiksa terhadap workflow destination. Ini sudah cocok dengan cara item mewarisi workflow di backend, tetapi belum diuji untuk workflow yang diubah di tengah jalan.

## 3. Bukti verifikasi

| Pemeriksaan | Hasil |
|---|---|
| `go test ./server/internal/projectexcel/` | PASS |
| `go test ./server/...` penuh dengan `PMS_TEST_DATABASE_URL` ke PostgreSQL 16 terisolasi (localhost:55500) | **PASS**, exit 0 (httpserver 132 s) |
| `TestBrowserExcelRoundTrip` (Playwright, backend hidup, `PMS_TEST_BROWSER=1`) | **PASS** |
| `web/tests/project-excel.spec.ts` (Playwright mock) | **PASS** (2/2) |
| `npm run test` (web, node --test) | PASS (92/92) |
| `npm run typecheck` | **FAIL** — 4 error di file automation (`workspace-automation.tsx:125`, `settings/automation.tsx:63`) yang bukan bagian perubahan ini; tidak ada error di file Excel |
| Tes merah dulu | Dilakukan untuk template, folder, status/assignee/dependency/destination, dan revocation; semua gagal sebelum perbaikan |

## 4. Catatan operasional

- Basis data uji: PostgreSQL 16 yang saya jalankan di scratchpad (`localhost:55500`, user `pms`). Sudah dihentikan di akhir sesi. Untuk menjalankan ulang: `pg_ctl -D <scratchpad>/pgdata -o "-p 55500 -k /tmp" start`.
- File bersama yang sudah punya perubahan tak ter-commit dari pekerjaan lain: `automation/engine.go`, `events/events.go`, `items/create.go`, `items/excel_import.go` (sebagian). Commit harus dipisah dengan hati-hati.

## 5. GO / NO-GO

- **Keamanan dan validasi (Phase 1)**: hijau.
- **Kontrak Phase 2 yang tidak butuh keputusan**: hijau untuk keputusan 2, 3, 7 (sebagian), 8, 9, 11; sebagian untuk 4, 6, 14.
- **Masih NO-GO** untuk pekerjaan berikutnya sampai: (a) definisi asli skenario IMP-T/EXP-T diberikan (keputusan 12), dan (b) validasi tanggal, Duration, dan kalender kerja (keputusan 10) dikerjakan.
