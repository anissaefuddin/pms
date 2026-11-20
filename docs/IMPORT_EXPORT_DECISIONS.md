# LENS Import/Export — Keputusan IE-D15 s.d. IE-D18

Lanjutan dari [IMPORT_EXPORT_CLOSEOUT.md](IMPORT_EXPORT_CLOSEOUT.md). Belum di-commit, belum di-push, belum di-merge, belum di-deploy. Tidak ada migrasi baru di sesi ini.

## 1. Status per keputusan

| ID | Keputusan | Status | Bukti |
|---|---|---|---|
| IE-D15 | Filter Table View: Item Type, Custom Field, Status, Assignee, Search, Sort, kolom | **PARTIAL** | Lihat §2 |
| IE-D16 | Down-migration 00342/00343 diuji di database disposable | **DONE** (rehearsal); down script belum diuji di database yang sudah termigrasi produksi | Lihat §3 |
| IE-D17 | Destination picker bertingkat (folder/project bersarang) | **PARTIAL** | Lihat §4 |
| IE-D18 | Resolusi workflow untuk Status, termasuk di Space root | **DONE** untuk aturan yang diminta; pindah workflow di root belum didukung (ditolak) | Lihat §5 |

## 2. IE-D15 — Filter Table View

Yang dikerjakan:
- Export memakai **snapshot** dari engine item yang sama dengan Table View (`items.List`), per container di seluruh subtree. Filter status, assignee, Item Type (`types`), dan Custom Field (`fields`, format `<id>:<nilai>`) diteruskan apa adanya. Pencarian judul memakai aturan yang sama dengan tabel (contains, tanpa membedakan huruf besar-kecil).
- Filter yang tidak valid ditolak dengan 422, tidak diubah menjadi ekspor tanpa filter: Item Type tidak dikenal, field tidak dikenal atau bukan UUID, dan field terbatas yang tidak bisa dibaca pengguna (diperlakukan sebagai tidak dikenal oleh engine).
- Sort (judul, mulai, selesai) dan kolom terlihat diterapkan. External Key dan Parent Key selalu disertakan agar file bisa diimpor ulang.
- UI dialog ekspor dari Table View meneruskan pencarian, preset (aktif / milik saya), sort, kolom, filter jenis, dan filter field.

Bukti:
- `TestExportTableParityWithItemTypeAndFieldFilters`: hasil ekspor berisi tepat dua item tipe `bug` yang sama dengan tabel; filter tipe tidak dikenal, filter field rusak, dan filter field terbatas ditolak 422.
- `TestExcelTableViewExport`: pencarian, status, sort menurun, dan kolom.
- `TestProjectExcelFilteredExportKeepsParents`: induk tetap ikut.

Belum:
- **Penonaktifan dengan penjelasan** untuk field yang tidak punya operator kompatibel: UI belum menonaktifkan opsi; server menolak dengan pesan.
- **Date dan Priority** sebagai filter: engine Table View tidak punya filter ini, jadi tidak diekspor sebagai filter.
- **Table View per container**: tabel non-flat hanya menampilkan anak langsung; ekspor selalu menelusuri seluruh subtree. Perbedaan ini didokumentasikan, belum dijadikan pilihan.

## 3. IE-D16 — Down-migration 00342 dan 00343

Rehearsal di database disposable (PostgreSQL 16 lokal, dibuat dengan `testdb`, dihapus setelah tes). Skrip rehearsal ditempatkan lewat `go test -overlay` di folder scratchpad, tidak masuk repo, sesuai pesan agar tidak membuat harness rollback permanen.

Urutan yang dijalankan (`TestDisposableDownMigrations`):
1. Up sampai versi 341 (skema sebelum 00342).
2. Sisipkan notifikasi biasa (`kind='shared'`).
3. Up sampai 343. Sisipkan notifikasi `import_summary` dan satu baris `excel_import_reports`.
4. Verifikasi: CHECK `notifications_kind_check` memuat `import_summary`; tabel `excel_import_reports` ada.
5. Down sampai 341. Verifikasi: CHECK kembali ke daftar lama; tabel hilang; notifikasi biasa tetap ada.
6. Up lagi sampai 343. Verifikasi: CHECK dan tabel kembali; notifikasi `import_summary` baru bisa dimasukkan.

Hasil: **PASS** (`ok`, 0.36 s).

Data yang hilang saat rollback:
- Semua notifikasi `kind='import_summary'` (dihapus oleh `DELETE` di Down 00342, karena CHECK lama tidak mengizinkannya).
- Semua baris `excel_import_reports` (tabel di-DROP di Down 00343). Laporan impor tidak bisa dipulihkan tanpa restore basis data.

Strategi produksi yang disarankan (sesuai IE-D16): migrasi bersifat aditif, jadi rollback normal adalah **kode lama + biarkan skema**. Down script hanya dipakai bila benar-benar diperlukan, dan sebelumnya perlu backup atau restore basis data.

Belum:
- Rehearsal belum dijalankan terhadap salinan basis data yang sudah termigrasi sampai versi produksi (hanya basis disposable dengan migrasi sampai 341).

## 4. IE-D17 — Destination picker bertingkat

Yang dikerjakan:
- Backend: destination boleh project atau folder di kedalaman mana pun dalam Space; pemeriksaan izin Create, pemeriksaan Space, dan aturan project hanya di root tetap dijalankan saat preview.
- UI: `web/src/components/excel-destination-tree.tsx`. Pohon dengan pemuatan lazy (anak dimuat saat node dibuka), pencarian pada node yang sudah dimuat, breadcrumb jalur terpilih, dan pilihan Space root.

Bukti:
- `TestImportIntoNestedFolderDestination`: folder bersarang di dalam folder sebagai tujuan; baris root masuk ke folder itu.
- `TestProjectExcelStatusAssigneeDependencyDestination`: tujuan task ditolak; project tidak boleh di bawah tujuan.
- Playwright mock `project-excel.spec.ts`: lulus (2/2).

Belum:
- **Lokasi tanpa izin buat dinonaktifkan atau disembunyikan**: API daftar hanya mengembalikan item yang boleh dilihat. Izin Create tidak diperlihatkan di pohon; server memvalidasinya saat commit (dan menolak dengan pesan).
- **Pencarian** hanya menyaring node yang sudah dimuat, belum mencari ke seluruh hierarki di server.

## 5. IE-D18 — Status dan workflow

Aturan yang diterapkan (`items/excel_import.go`, `resolveWorkflows`):
- Setiap baris punya workflow efektif:
  - Baris dengan induk di file mewarisi workflow induknya.
  - Baris root di bawah destination memakai workflow destination (`COALESCE(default_workflow_id, status, default)`).
  - Baris root tanpa destination memakai default workflow Custom Item Type, lalu default workflow.
- Kolom `Workflow` yang eksplisit harus sama dengan workflow efektif; jika tidak, preview menolak baris itu.
- `Status` harus ada di workflow efektif; jika tidak, preview menolak. Status tidak pernah dibuang atau diganti default.
- Jika workflow tidak bisa ditentukan, preview menolak.

Perubahan kontrak di `items/create.go`: item root sekarang memakai default workflow Custom Item Type bila ada (sebelumnya selalu default global). Ini mengubah perilaku pembuatan item di luar impor juga. Semua tes lama lulus.

Bukti:
- `TestImportStatusResolutionAtSpaceRoot`: status dari default workflow di root diterima dan tersimpan; status yang tidak ada dan workflow yang tidak sesuai ditolak; Custom Item Type dengan default workflow menghasilkan item di workflow itu.
- Tes lama `TestProjectExcelStatusAssigneeDependencyDestination` (status tidak ada, email, siklus, destination task) tetap lulus setelah filter status dibatasi ke destination di tes itu.

Belum:
- **Pindah workflow di Space root** (baris root yang workflow eksplisitnya berbeda dari default): ditolak. Membukanya memerlukan create path menerima workflow dari baris, yang belum dikerjakan.
- **Aturan transisi status dan initial status** di impor: impor memakai status awal workflow saat item dibuat (sama seperti create). Aturan transisi tidak diperiksa di impor karena item baru belum punya status sebelumnya.
- Tes `TestImportStatusResolutionAtSpaceRoot` ditulis setelah implementasi; tidak ada bukti merah untuk tes itu.

## 6. Bukti verifikasi (sesi ini)

| Pemeriksaan | Hasil |
|---|---|
| `go vet ./server/...` | bersih |
| `gofmt -l server` | bersih |
| `go test -count=1 ./server/...` (PostgreSQL 16 terisolasi) | **PASS**, exit 0, 22 paket `ok` |
| Rehearsal down-migration (overlay, database disposable) | **PASS** |
| `TestBrowserExcelRoundTrip` (Playwright, backend hidup) | **PASS** |
| `project-excel.spec.ts` (Playwright mock) | **PASS** 2/2 |
| `npm run test` web | **PASS** 92/92 |
| `npm run typecheck` web | **FAIL** 4 error, semua di file automation (`workspace-automation.tsx`, `settings/automation.tsx`) yang bukan bagian pekerjaan ini |

## 7. Risiko yang tersisa

- Perubahan `create.go` (item root memakai default workflow Custom Item Type) berlaku juga di luar impor; perlu dicek di UI pembuatan item.
- Ekspor dengan filter menelusuri seluruh subtree, sedangkan tabel non-flat hanya menampilkan anak langsung.
- Filter field yang tidak punya operator tidak dinonaktifkan di UI.
- Down-migration menghapus data `import_summary` dan laporan impor; gunakan rollback kode, bukan down script, kecuali ada backup.
- Banyak file bersama masih berisi perubahan tak ter-commit dari pekerjaan lain. Commit harus dipisah per hunk.

## 8. Penutupan item sisa (urut)

| No. | Item | Status | Bukti |
|---|---|---|---|
| 1 | Typecheck web | **DONE**: 4 error diperbaiki (`rule.target_space_ids ?? []`, `workspace-automation.tsx`, `settings/automation.tsx`). `tsc -b`: 0 error. | `npm run typecheck` |
| 2 | IE-D18: aturan transisi status saat impor | **DONE** untuk status awal yang terkunci: status yang tidak bisa dicapai dari status awal ditolak di preview. | `TestImportStatusRespectsLockedTransitions` (merah dulu, lalu hijau) |
| 3 | IE-D15: field yang tidak kompatibel dinonaktifkan dengan penjelasan | **DONE** untuk tipe `relation` (satu-satunya tipe tanpa operator). Opsi tampil disabled dengan keterangan. Tidak ada E2E khusus untuk opsi itu; dicek lewat typecheck dan `views.spec.ts` (6/6). | `lib/view-columns.ts` (`fieldFilterable`), `components/view-columns.tsx` |
| 4 | IE-D17: lokasi tanpa izin create tampil disabled | **DONE**: endpoint `GET /api/v1/spaces/{id}/excel-destinations?parent=` memberi `can_create`; picker menonaktifkan node yang tidak bisa dibuat. | `TestExcelDestinationsReportCreateRights` (merah dulu) |
| 5 | IE-D16: rehearsal di database termigrasi dengan volume | **DONE** untuk database disposable yang termigrasi sampai versi kepala (343): 5000 notifikasi biasa, 200 `import_summary`, 200 laporan. Setelah rollback ke 341: 200 notifikasi `import_summary` dan 200 laporan hilang, 5000 notifikasi biasa tetap, lalu up lagi berhasil. Belum dijalankan pada salinan database produksi (tidak tersedia di sesi ini). | rehearsal overlay (scratchpad, tidak masuk repo) |
| 6 | Commit | **Tidak dilakukan**. Belum diminta; banyak file bersama memuat perubahan pekerjaan lain sehingga commit perlu dipisah per hunk. | — |

Suite penuh `go test ./server/...` (PostgreSQL 16 terisolasi): PASS, exit 0, 22 paket `ok`. E2E `TestBrowserExcelRoundTrip`: PASS. Web: `npm run test` 92/92, typecheck 0 error, Playwright `project-excel` dan `views` 8/8.

## 9. Item lanjutan (3–7)

| No. | Item | Status | Bukti / catatan |
|---|---|---|---|
| 3 | Ekspor level Space (EXP-03) | **BLOCKED** (butuh migrasi). `project_excel_exports.project_id` NOT NULL, jadi ekspor Space tidak bisa disimpan tanpa mengubah skema. Usulan: migrasi yang membuat `project_id` nullable dan menambah `scope`; perlu persetujuan Anda. | `server/migrations/00036_project_excel.sql` |
| 4 | Wizard 4 tahap (AUD-IMP-01) | **DONE**. Unggah → Petakan kolom → Validasi → Impor, dengan indikator langkah dan tombol Kembali/Lanjut. Tes E2E diperbarui dan lulus. | `components/space-excel-import.tsx`, `tests/project-excel.spec.ts`, `tests/excel-roundtrip-live.spec.ts` |
| 5 | Tautan dari notifikasi ringkasan ke laporan | **DONE**. Notifikasi menyebut ID laporan; inbox menampilkan ringkasan laporan. Endpoint baru `GET /api/v1/excel-imports/{reportId}` (tanpa Space; hanya pengimpor dengan akses View). ID laporan diambil dari teks notifikasi (regex), bukan kolom khusus. | `TestImportReportReadableFromNotification`, `routes/_app/inbox.tsx` |
| 6a | EXP-09: laporan error untuk data besar | **DONE** sebagian. Error sekarang menyebut jumlah item dan batas (contoh: 501 items, at most 500). | `TestEXPT07LargeProjectExport` |
| 6b | EXP-09: progres ekspor | **BLOCKED** (butuh kolom progres = migrasi). | — |
| 6c | EXP-10: preset kolom dan scope | **DONE** dengan batasan: pilihan kolom dan scope disimpan di localStorage browser (per viewer, tidak dibagikan). Belum ada E2E untuk preset. | `components/project-excel-export.tsx` |
| 7 | IMP-10 (MS Project) | **Tidak dikerjakan** (P2, hanya dicatat). | — |

Catatan regenerasi kontrak: menambahkan `import_summary` ke `api/openapi.yaml` lalu menjalankan generator mengubah `server/internal/apigen/openapi.gen.go` dan `web/src/api/schema.d.ts` lebih luas dari perubahan itu, karena file-file tersebut sudah berisi perubahan kontrak lain yang belum ter-commit. `go build` dan typecheck lulus; diff generated perlu ditinjau sebelum commit.

Hasil: `go test ./server/...` PASS (22 paket, exit 0); E2E browser PASS; `npm run test` 92/92; typecheck 0 error; Playwright mock penuh 562 passed, 43 skipped (skip bawaan).
