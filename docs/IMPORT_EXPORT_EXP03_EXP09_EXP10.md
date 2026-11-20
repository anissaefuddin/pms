# LENS Import/Export — EXP-03, IMP Report Link, EXP-09, EXP-10

Lanjutan dari [IMPORT_EXPORT_DECISIONS.md](IMPORT_EXPORT_DECISIONS.md). Belum di-commit, belum di-push, belum di-merge, belum di-deploy. Migrasi 00344 dan 00345 hanya diuji di basis data disposable; tidak ada yang diterapkan ke production.

Catatan pelaksanaan: keempat workstream dikerjakan berurutan dalam satu sesi, bukan paralel. Pemeriksaan shared files dilakukan di awal (lihat §6).

## 1. Status

| Workstream | Status | Ringkasan |
|---|---|---|
| EXP-03 ekspor Space | **DONE** (backend + UI + E2E) | Migrasi aditif `project_id` nullable + `scope`. Ekspor Space memuat setiap root yang bisa dilihat pemanggil beserta hierarkinya; visibilitas mengikuti engine Table View. |
| IMP Report Link (P1) | **DONE** | Referensi terstruktur `reference_kind` + `reference_id` di notifikasi; inbox memakai referensi, bukan teks. Endpoint laporan tetap memeriksa pemilik dan akses View Space. |
| EXP-09 progres | **PARTIAL** | Progres = jumlah item aktual yang sudah ditulis dari total item yang dihitung dari pohon. Tidak ada persentase fiktif. Belum ada progres yang diuji saat job masih berjalan (lihat §5). |
| EXP-10 preset | **DONE** (MVP, localStorage) | Preset per pengguna, Space, dan view; simpan, muat ulang, perbarui, hapus; E2E mock. Isolasi pengguna hanya diuji lewat unit test (lihat §5). |
| Wizard impor | **DONE** (tidak diubah) | E2E yang sudah lulus tetap lulus. |
| IMP-10 MS Project | **DEFERRED** (P2) | Tidak dikerjakan pada batch ini. |

## 2. Migrasi (aditif, dengan bagian Down)

- `server/migrations/00344_export_scope_progress.sql`
  - `project_excel_exports.project_id` menjadi nullable.
  - `scope text NOT NULL DEFAULT 'project'` dengan CHECK `project|space`.
  - CHECK `project_excel_exports_scope_target`: scope `project` wajib punya `project_id`; scope `space` tidak boleh punya.
  - `total_items` dan `processed_items` (integer ≥ 0, default 0).
  - Baris lama tetap `scope='project'` dengan `project_id` aslinya (kompatibilitas mundur).
- `server/migrations/00345_notification_reference.sql`
  - `notifications.reference_kind` (CHECK: `excel_import_report`) dan `reference_id uuid`, keduanya diisi bersama atau tidak sama sekali (CHECK `notifications_reference_pair`).
  - Notifikasi lama tidak berubah; referensinya NULL.

Rehearsal (`TestRehearsal344And345`, overlay di scratchpad, tidak masuk repo):
1. Up sampai 343, isi 50 ekspor project dan 300 notifikasi.
2. Up ke 345. Verifikasi: 50 ekspor tetap `project`; ekspor `space` dengan `project_id` NULL diterima; `project` tanpa `project_id` ditolak; referensi tanpa id ditolak; referensi lengkap tersimpan.
3. Down ke 343. Verifikasi: kolom referensi hilang; `project_id` kembali NOT NULL; kolom scope/progres hilang; 50 ekspor project tetap.
4. Up lagi ke 345.

Hasil: **PASS**. Data yang hilang saat down: semua ekspor `scope='space'` (dihapus karena tidak bisa direpresentasikan tanpa `project_id`; masa berlakunya 1 jam anyway), dan semua referensi notifikasi (notifikasi itu sendiri tetap ada).

## 3. API dan otorisasi

- `POST /api/v1/spaces/{spaceId}/excel-exports` (baru): ekspor Space. Akses: View Space (selain itu 404). Filter divalidasi sama dengan ekspor project (status, assignee, sort, kolom, search, Item Type, field).
- `POST /api/v1/items/{projectId}/excel-exports` (tidak berubah secara kontrak): project harus ada di Space dan bisa dilihat.
- `GET /api/v1/excel-exports/{id}` dan `/file`: memeriksa akses sesuai scope pada setiap permintaan (`checkExportAccess`); unduhan memeriksa ulang bahwa setiap item masih ada di scope dan bisa dilihat (`CanDownload`, termasuk `policy.ItemsCaps`).
- `GET /api/v1/excel-imports/{reportId}` (baru, tanpa Space di URL): hanya pemilik laporan dan hanya jika masih punya View pada Space laporan. Dicek setiap permintaan.
- `GET /api/v1/notifications` mengembalikan `reference` (`{kind, id}`) bila ada.

## 4. Test: merah dulu, lalu hijau

| Tes | Sebelum | Sesudah |
|---|---|---|
| `TestSpaceScopeExportHoldsVisibleHierarchy` | 404 (rute belum ada) | PASS. Admin dan limited member: root dan anak ada; Parent Key anak = project; outsider 404; share yang dicabut mengakhiri akses. |
| `TestExportScopeAndTargetAreValidated` | 404 | PASS. Filter tidak valid 422; project yang bukan milik Space 404. |
| `TestNotificationCarriesReportReference` | gagal (kolom belum ada) | PASS. Referensi tersimpan, muncul di JSON inbox, pembaca lain tidak bisa membuka laporan. |
| `TestImportReportNeedsCurrentSpaceAccess` | — (tes baru) | PASS. Akses laporan berakhir saat share Space dicabut. |
| `TestExportProgressCountsRealItems` | gagal (total/processed null) | PASS. 3 dari 3 item (project, folder, satu task). |
| `TestSpaceExportDownloadStaysWithLimitedMember` | — (tes baru) | PASS. Lihat catatan §5 tentang jalur penolakan item. |
| Tes ekspor lama (`TestProjectExcel*`, `TestEXPT*`, `TestExcelTableViewExport`, `TestExportTableParity…`, `TestRestrictedField…`, `TestBulk…`) | — | PASS (regresi). |

Unit web: `src/lib/export-presets.test.ts` (3 tes: namespace per pengguna/Space/view, perbarui dan hapus, storage rusak diabaikan).

E2E mock (Playwright):
- `tests/export-presets.spec.ts`: simpan, muat ulang, perbarui, hapus; isolasi antar-Space. 2/2.
- `tests/space-export.spec.ts`: ekspor Space memanggil rute Space; progres tampil dengan jumlah item nyata. 2/2.
- `tests/import-report-link.spec.ts`: ringkasan membuka laporan lewat referensi; notifikasi tanpa referensi tidak meminta laporan. 2/2.
- `tests/spaces.spec.ts`: menu Space sekarang memuat "Export Space to Excel" (ekspektasi diperbarui). 19/19.

E2E browser dengan backend hidup: `TestBrowserExcelRoundTrip` PASS (dengan wizard).

## 5. Risiko dan batasan

1. **EXP-09 saat job berjalan: sudah diuji** (`TestExportProgressWhileRunning`). Job ditahan di 50 dari 123 item lewat `projectexcel.ProgressObserver` (seam uji, nil di produksi); poll membaca `processed_items=50`, `total_items=123`, status pending; setelah dilepas, hasil akhir 123 dari 123. Saat menulis tes ini ditemukan off-by-one (progres dilaporkan pada 49, 99, ...), sudah diperbaiki menjadi 50, 100, ....
2. **EXP-10 isolasi pengguna hanya lewat unit test.** E2E mock hanya punya satu pengguna. Preset tetap di localStorage browser: tidak dibagikan antar perangkat, dan hilang bila data situs dihapus.
3. **Jalur penolakan ItemsCaps saat unduhan tidak tercapai oleh anggota Space.** Custom role selalu menyertakan View (bit 0 wajib), dan pencabutan share Space ditolak lebih awal. Yang tercapai dan diuji: item yang keluar dari Space atau dihapus setelah ekspor membuat unduhan 404 (`TestSpaceExportDownloadRefusesMovedAndDeletedItems`), dan pencabutan share Space (`TestSpaceExportDownloadStaysWithLimitedMember`).
4. **Ekspor Space bisa besar.** Daftar item yang terlihat dikirim sebagai snapshot di argumen job (maksimum 10.000 relasi dari batas pohon). Belum ada batas tersendiri untuk ukuran snapshot.
5. **Migrasi belum diuji di salinan produksi.** Rehearsal di basis disposable dengan data sintetis.
6. **Down-migration** menghapus ekspor Space dan referensi laporan. Gunakan rollback kode dan biarkan skema, kecuali ada backup.
7. **Kontrak API di-regenerasi** (`openapi.gen.go`, `schema.d.ts`) dan diff-nya lebih besar dari perubahan ini, karena file-file itu sudah berisi perubahan lain yang belum ter-commit.

## 6. Shared files dan kontrak (pemeriksaan awal)

- Ada 170 entri di `git status` pada awal sesi ini. File yang disentuh: `server/internal/httpserver/project_excel*.go`, `server/internal/projectexcel/*`, `server/internal/items/excel_import.go`, `server/internal/notifications/record.go`, `server/internal/httpserver/notifications.go`, `api/openapi.yaml`, `server/internal/apigen/openapi.gen.go`, `web/src/api/schema.d.ts`, komponen web ekspor/impor, `routes/_app/inbox.tsx`, `routes/_app/spaces.$spaceId.tsx`, `lib/export-presets.ts` (baru), tes Playwright terkait.
- `server/migrations/00340_*`, `00341_*` tidak tersentuh sesi ini (milik pekerjaan lain, untracked).
- `notifications/record.go`: ditambah `NotifyRef`; `Notify` menjadi pembungkus tanpa perubahan perilaku.

## 7. Verifikasi

| Pemeriksaan | Hasil |
|---|---|
| `go vet ./server/...`, `gofmt -l server` | bersih |
| `go test -count=1 ./server/...` (PostgreSQL 16 disposable) | PASS, exit 0, 22 paket `ok` |
| Rehearsal 00344/00345 (overlay) | PASS |
| `TestBrowserExcelRoundTrip` (Playwright + backend) | PASS |
| `npm run test` (web) | PASS 95/95 |
| `npm run typecheck` (web) | 0 error |
| Playwright mock penuh | 568 passed, 43 skipped (skip bawaan) |

## 8. GO / NO-GO

- **GO** untuk EXP-03, IMP Report Link, dan EXP-10 (MVP) sebagai perubahan kode, dengan tes dan E2E yang sudah lulus.
- **NO-GO** untuk merilis EXP-09 sebagai "selesai" sampai progres saat berjalan diuji. Yang sekarang ada adalah progres akhir dan penulisan berkala; itu belum terbukti secara langsung.
- **Belum ada** persetujuan untuk menerapkan migrasi 00344/00345 ke database mana pun. Perlu persetujuan terpisah dan rencana rollback (kode lama + skema dibiarkan).
- **Belum di-commit.** Perlu pemisahan per hunk karena file bersama.
