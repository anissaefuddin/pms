# Analisa Import & Export Excel LENS (Phase 0)

Status: **audit saja, tanpa perubahan kode**. Dokumen ini belum di-commit.
Tanggal: 2026-11-18. Branch: `feat/m12c-ui-clean` (working tree sudah berisi banyak perubahan lain yang bukan milik sesi ini; tidak ada file yang diubah selain dokumen ini).

Catatan singkat: sesi ini tidak mengubah kode, tidak menjalankan migrasi, tidak commit, tidak push, dan tidak deploy. Fixture xlsx dan file uji sementara dibuat di folder scratchpad, bukan di repo.

---

## 0. Ringkasan untuk pembaca

- Fitur yang **sudah ada** cukup kuat di sisi **parser dan keamanan**: batas ukuran, formula ditolak, impor atomik (satu transaksi), idempotensi lewat digest, hierarki dari satu file dengan deteksi siklus, dan cek izin per Space.
- Yang **belum ada** adalah sebagian besar pengalaman pengguna: tidak ada template unduhan, bukan wizard 4 tahap, ekspor hanya untuk project (bukan table view, folder, atau Space), dan kolom sistem penting (status, assignee, priority, effort, dependency) belum ikut diimpor/diekspor.
- Ada **3 temuan defect yang perlu perhatian**:
  1. Impor bulk memicu event `item.created` yang juga dibaca automation dan webhook (bertentangan dengan default MVP "tidak memicu automation").
  2. File ekspor yang sudah jadi masih bisa diunduh sampai kedaluwarsa (1 jam) walau grant field sudah dicabut.
  3. Nilai custom field di ekspor berupa JSON mentah (misal `"Done"` dengan tanda kutip), bukan nilai yang bisa dibaca.
- Workbook yang **bukan format LENS** (contoh dokumen test/seed) **tidak ditolak** dengan pesan format. Parser mengambil sheet pertama dan menerima isinya; error baru muncul sebagai error kolom/baris.
- Error formula (`cell B3: formulas are not supported`) **sudah spesifik dan sudah direproduksi**. Tapi pesannya belum menyebut nama sheet dan belum memberi saran perbaikan.

---

## 1. Peta kode yang diaudit

| Area | Lokasi | Isi singkat |
|---|---|---|
| Parser & writer xlsx | `server/internal/projectexcel/xlsx.go` | `Write` (42-116), `Read` (182-271), formula ditolak di `readCell` (491-493), batas di konstanta (19-25) |
| Exporter (river job) | `server/internal/projectexcel/export.go` | `CheckScope` (32-49), `CanDownload` (60-73), `Work` (75-89), `exportRows` (122-244) |
| Impor (logika bisnis) | `server/internal/items/excel_import.go` | `ImportExcel` (41-135), `planExcel` (137-389) |
| Pembuatan item | `server/internal/items/create.go` | `createWithCustomFields` (143-), cek lokasi `locationCaps` (create.go:188, location.go:17), `recordChange` (create.go:350) |
| Event & automation | `server/internal/items/emit.go:22`, `server/internal/items/patch.go:417-420`, `server/internal/automation/engine.go:78` | item.created dibuat untuk setiap create |
| HTTP | `server/internal/httpserver/project_excel.go` | route (21-26), create export (41-91, batas 5/jam di baris 73), get/download (93-157), impor (159-283) |
| Migrasi | `server/migrations/00036_project_excel.sql`, `00037_project_excel_imports.sql` | tabel export dan receipt impor, kedaluwarsa 1 jam (baris 7 di kedua file) |
| Housekeeping | `server/internal/housekeeping/housekeeping.go:102-105` | hapus export/impor yang kedaluwarsa |
| Worker | `server/internal/mail/mail.go:181`, `:254` | `ExportWorker` didaftarkan ke river |
| Frontend impor | `web/src/components/space-excel-import.tsx` (158 baris) | satu dialog: pilih file → mapping → preview → commit |
| Frontend ekspor | `web/src/components/project-excel-export.tsx` (84 baris) | dialog ekspor dari detail item, polling 1 detik |
| Pintu masuk UI | `web/src/routes/_app/spaces.$spaceId.tsx:132,152` (menu "Import from Excel"), `web/src/components/item-detail.tsx:213,326` (menu Export, hanya jika `item.type === "project"`) | |
| i18n | `web/src/i18n/en/excel.json`, `web/src/i18n/id/excel.json` | teks dialog, 8 label kolom |
| Test unit | `server/internal/projectexcel/xlsx_test.go` | 4 test (literal text, relasi, data invalid, batas) — **lulus** (`go test` OK) |
| Test integrasi Go | `server/internal/httpserver/project_excel_integration_test.go` | 3 test (export, impor, round-trip browser) — **butuh DB**, lihat §8 |
| Test e2e web | `web/tests/project-excel.spec.ts`, `web/tests/excel-roundtrip-live.spec.ts` | belum dijalankan |

API yang ada (hanya 4 route):

- `POST /api/v1/spaces/{spaceId}/excel-imports` (impor; preview jika tanpa `commit`)
- `POST /api/v1/items/{projectId}/excel-exports` (mulai ekspor, 202)
- `GET /api/v1/excel-exports/{exportId}` (status)
- `GET /api/v1/excel-exports/{exportId}/file` (unduh)

Tidak ada endpoint template, endpoint ekspor Space, endpoint ekspor table view, atau endpoint ekspor folder.

---

## 2. Matriks gap IMP-01 s.d. IMP-10 (impor)

| ID | Yang diminta | Status | Bukti (file:baris / endpoint / test) | Catatan |
|---|---|---|---|---|
| IMP-01 | Template Excel bisa diunduh | **MISSING** | Tidak ada route template di `project_excel.go:21-26`; tidak ada tombol unduh di `space-excel-import.tsx` | Tidak ada file template di repo untuk diunduh |
| IMP-02 | Wizard berpemandu, bukan modal yang langsung minta file | **MISSING** | `space-excel-import.tsx:109-119` (input file pertama, langsung di dialog); tidak ada langkah | Ada preview dan gerbang commit (`:154-155`), tapi bukan wizard 4 tahap |
| IMP-03 | Format kolom, tipe, dan contoh dijelaskan | **PARTIAL** | `excel.json` `importDescription` hanya 1 kalimat; label 8 kolom di `excel.json` (`column.*`); pesan tanggal "enter a date as YYYY-MM-DD" (`excel_import.go:289`) | Tidak ada penjelasan per kolom, tipe, atau contoh; kolom "Parent identifier" bukan "Parent Key" |
| IMP-04 | Error menyebut sheet, cell, field, dan saran | **PARTIAL** | Error sel dari parser: `xlsx.go:439` → `cell B3: formulas are not supported` (direproduksi, §4). Error baris/kolom dari planner: `excel_import.go` (`add(row, col, msg)`), di-render `space-excel-import.tsx:145-149` | Nama sheet **tidak** ada. Saran perbaikan ("ganti dengan nilai hasil") tidak ada di server. Error parser dikirim sebagai teks biasa (`project_excel.go:250`), bukan daftar per baris |
| IMP-05 | Tujuan (Space/Project/Folder) terlihat di dialog | **PARTIAL** | Dialog hanya berisi judul dan deskripsi generik (`en/excel.json` `importDescription`); tujuan diambil dari halaman Space (`spaces.$spaceId.tsx:152`) | Nama Space tidak ditampilkan. Tujuan Project/Folder tidak ada: item root selalu masuk ke root Space (`excel_import.go:90`) |
| IMP-06 | Column mapping bisa diverifikasi (dynamic mapping) | **DONE** | Dropdown per kolom: `space-excel-import.tsx:120-134`; server validasi tujuan tidak dikenal, duplikat, dan sumber yang tidak ada (`excel_import.go:200-215`); mapping ikut digest (`project_excel.go:233-238`) | Mapping belum bisa disimpan sebagai preset (lihat EXP-10/IE-B7) |
| IMP-07 | Hierarki parent-child terbukti | **DONE** | Parent dari kolom `parent_id` yang merujuk kolom `id` di file (`excel_import.go:273-284`, `:355-388`); urutan buat dari parent ke anak; deteksi siklus (`:362-366`). Test: `project_excel_integration_test.go:129-160` (parent/child), `:170-176` (siklus) | Nama kolom "id/parent_id", bukan "External Key/Parent Key". Multi-parent sampai 50 |
| IMP-08 | Preview baris valid, invalid, dan warning | **PARTIAL** | Preview = jumlah baris, daftar error, sampel 5 baris (`excel_import.go:148-156`, `space-excel-import.tsx:135-149`) | Tidak ada hitungan "valid" dan "invalid" terpisah. Tidak ada kategori **warning**. Preview tidak menjalankan `createWithCustomFields`, jadi error dari aturan create (mis. izin lokasi) baru muncul saat commit |
| IMP-09 | Status job dan laporan hasil | **PARTIAL** | Commit sinkron: 201 `item_ids` (`project_excel.go:282`); receipt idempotensi di `project_excel_imports` (`excel_import.go:68-86`, kedaluwarsa 1 jam) | Tidak ada job impor, tidak ada laporan baris yang dilewati, tidak ada riwayat impor |
| IMP-10 | MS Project (P2) — cukup catat status | **MISSING** (P2) | Tidak ada entri "Import from MS Project" di kode web (`grep "MS Project"` kosong) | Sesuai prioritas P2; hanya dicatat |

---

## 3. Matriks gap EXP-01 s.d. EXP-10 (ekspor)

| ID | Yang diminta | Status | Bukti | Catatan |
|---|---|---|---|---|
| EXP-01 | Export Table View mempertahankan filter, sort, kolom aktif | **MISSING** | `toCsv` hanya dipakai di `web/src/routes/_app/settings/audit.tsx:7`; tidak ada export tabel item | Tidak ada jalur sama sekali |
| EXP-02 | Export Project/Folder termasuk subtree | **PARTIAL** | Subtree lewat CTE rekursif `export.go:51-58`; test `project_excel_integration_test.go:52-100` (folder dan task di dalam project). Tapi `CheckScope` hanya menerima `type='project'` (`export.go:34`) dan menu hanya muncul untuk project (`item-detail.tsx:213`) | Folder tidak bisa diekspor |
| EXP-03 | Export Space sesuai data yang bisa diakses | **MISSING** | Tidak ada route atau UI untuk Space (routes di §1) | |
| EXP-04 | Pilihan cakupan filtered vs all | **MISSING** | Hanya satu cakupan (seluruh tree project) | Tidak ada pilihan |
| EXP-05 | Field sistem: status, assignee, tanggal, priority, effort | **PARTIAL** | Kolom ada: `start_date`, `due_date`, `duration_days`, `is_milestone` (`export.go:19`, `:135-137`). **Tidak ada**: status, assignee, priority (`importance`), effort (`effort_minutes`) | |
| EXP-06 | Nilai custom field sesuai permission dan tipe | **DEFECT** | Permission: field yang dibatasi dibuang sebelum serialisasi (`export.go:127-131`, `:138-142`). Tipe: nilai diambil sebagai JSON mentah (`export.go:138`, `string(n.fields[field])` di `:234`) sehingga teks menjadi `"Done"` dengan tanda kutip, dan orang menjadi UUID | Test hanya memeriksa angka `123.45` (`integration_test.go:84`), jadi teks dan orang tidak diuji. Ini dugaan dari membaca kode (lihat §8) |
| EXP-07 | Dependency bisa direkonstruksi | **MISSING** | Tabel dependency ada (`items/dependencies.go`), tapi export tidak menulis dependency; `Headers` (`export.go:19`) tidak punya kolom dependency | |
| EXP-08 | Metadata: ID, parent ID, workflow, type key | **PARTIAL** | ID, parent_id (multi-parent sebagai daftar JSON, `export.go:226-231`), type, type_key (`export.go:19`) ada. **Workflow tidak ada** | |
| EXP-09 | Progress dan laporan error untuk data besar (P2) | **PARTIAL** | Job asinkron dengan status `pending/ready/failed` dan pesan (`export.go:75-89`); UI polling tiap 1 detik (`project-excel-export.tsx:42`). Batas: 500 item (`export.go:179`), 10.000 relasi lokasi (`export.go:204`), 8 MB teks (`export.go:170`) | Tidak ada progress persentase. Kegagalan selain batas memberi pesan umum "The workbook could not be generated." (`export.go:81`) |
| EXP-10 | Preset kolom dan scope (P2) | **MISSING** (P2) | Tidak ada | Hanya dicatat |

---

## 4. Reproduksi error formula dengan fixture terkendali

Fixture dibuat dengan Python di scratchpad (bukan repo):
`/private/tmp/claude-501/-Volumes-MyHDD-homeserver-apps-pms/0f96222c-8601-4c57-b2d5-18b4afd14e25/scratchpad/`

- `A_tasks_formula_B3.xlsx` — sheet "Sheet Tasks", header Title/Parent Key/Assigned To/End Date, sel B3 berisi formula.
- `B_test_doc_two_sheets.xlsx` — sheet pertama "Cover" (teks biasa), sheet kedua "Test Cases" (TC ID, Scenario, Expected, Result) dengan formula di D2.
- `C_seed_no_formula.xlsx` — sheet "Seed", header Scenario/Expected, tanpa formula.

Cara jalankan: file uji sementara `zz_repro_test.go` disisipkan lewat `go test -overlay` (file tidak ditulis ke repo). Hasil:

| Fixture | Hasil `projectexcel.Read` | Penilaian |
|---|---|---|
| A (formula di B3) | `err = cell B3: formulas are not supported` | **Positif**: validasi formula spesifik dan sel benar. Tapi nama sheet hilang, dan belum ada saran |
| B (dokumen test, sheet pertama "Cover") | `err = nil`, rows = `[["Test plan"], ["Sprint 1 regression"]]` | **DEFECT** (IMP-T05/T06): sheet pertama diambil diam-diam, formula di sheet "Test Cases" tidak terlihat, dan tidak ada pesan "format tidak dikenali" |
| C (seed tanpa formula, header bukan LENS) | `err = nil`, rows = 2 baris | **DEFECT** (IMP-T05): lolos parser. Pesan yang muncul nanti hanya "unknown column…" dan "map a column to title" (`excel_import.go:200-202`, `:216-217`), bukan "format tidak dikenali" |

Kesimpulan untuk pertanyaan user:

- **Validasi formula sudah benar dan spesifik** (positif).
- **Pemilihan worksheet belum benar**: selalu sheet pertama (`xlsx.go:214-233`).
- **Validasi skema workbook belum ada**: tidak ada cek nama sheet, tidak ada cek header minimum, tidak ada cek bahwa file memang template LENS. Workbook dokumen test/seed belum ditolak dengan pesan format.

Catatan: sheet pertama yang dipilih oleh parser bisa berisi formula yang tidak terlihat dari UI, sementara error yang tampil berasal dari sheet lain (atau sebaliknya). Itu sebabnya nama sheet harus masuk ke pesan error.

---

## 5. Cek kontrak yang diusulkan user

Kolom yang diusulkan dibandingkan dengan kode saat ini. Nama header di kode: `id, parent_id, type, title, start_date, due_date, duration_days, is_milestone, type_key` (`export.go:19`) plus `field:<uuid>` untuk custom field.

| Kolom usulan | Di kode sebagai | Status | Catatan |
|---|---|---|---|
| External Key | `id` (`excel_import.go:236-238`) | PARTIAL | Fungsi sama, nama beda. Jika kosong dipakai `@row:N` |
| Parent Key | `parent_id` | PARTIAL | Fungsi sama; nilai harus `id` di file yang sama |
| Item Type | `type` + `type_key` (`:244-264`) | DONE | Bisa key atau nama tipe aktif |
| Title | `title` (`:265-268`) | DONE | 1–500 karakter |
| Description | — | **MISSING** | Ada `description` di `patch.go:345`, belum di impor/ekspor |
| Status | — (`status_id` ada di `patch.go:283`) | **MISSING** | Perlu aturan: nama status vs ID, dan status harus ada di workflow item |
| Workflow | — (`default_workflow_id`, `create.go:171`) | **MISSING** | Ekspor juga tidak menulis workflow (EXP-08) |
| Priority | — (`importance`, `patch.go:268`) | **MISSING** | |
| Assignee Email | — (hanya `assignee_ids` / custom field `people` dengan UUID, `custom_field_types.go:263-293`) | **MISSING** | Tidak ada pencocokan email ke user |
| Start Date | `start_date` | DONE | Format `YYYY-MM-DD` |
| Finish Date | `due_date` | PARTIAL | Nama beda; fungsi sama |
| Duration | `duration_days` | DONE | Angka ≥ 0 |
| Effort | — (`effort_minutes`, `item_types.go:153`) | **MISSING** | |
| Depends On | — (`items/dependencies.go`) | **MISSING** | Perlu format dan aturan siklus; siklus sudah dicek di level dependency (`dependencies.go:128-141`) |
| Custom Fields | `field:<uuid>` (`export.go:213-214`, `excel_import.go:319-348`) | PARTIAL | Impor memvalidasi tipe dan membuang field yang diarsipkan (`excel_import.go:157`); ekspor pakai UUID di header, dan nilai JSON mentah |

Tiga template yang diusulkan (**Basic / Project & Hierarchy / Advanced**): **MISSING**. Tidak ada file template dan tidak ada endpoint. Kolom yang perlu masuk ke masing-masing template tergantung keputusan di §10.

Import **create-only** untuk MVP: **DONE**. `ImportExcel` hanya membuat item (`excel_import.go:87-129`); tidak ada jalur update.

---

## 6. Risiko P0 dan P1

| Prioritas | Risiko | Temuan | Bukti | Status |
|---|---|---|---|---|
| P0 | Cross-Space import (otorisasi per target) | Akses dicek di Space tujuan (`requireSpace` Create, `excel_import.go:61`; HTTP `project_excel.go:165-177`), dan per lokasi (`create.go:188-`, `location.go:17`), serta tipe item harus milik Space itu (`create.go` cek `itemType.SpaceID`). Tidak ada test khusus cross-Space untuk impor | kode saja | **PARTIAL** (kode ada, test belum) |
| P0 | Restricted field export (grant dicek sebelum serialisasi) | Dicek sebelum tulis: `export.go:127-131`, `:138-142`. **Tetapi** unduhan tidak mengecek ulang grant field (`CanDownload`, `export.go:60-73`, hanya cek View Space dan tree). Test untuk field tertutup tidak ditemukan | `export.go:60-73` | **DEFECT** (window sampai 1 jam) |
| P0 | Duplicate import (idempotency key dan job tracking) | Digest = hash(mapping + file), receipt per user+Space (`project_excel.go:233-247`, `excel_import.go:68-86`), file berbeda dengan digest sama ditolak 409 (`project_excel.go:244-246`). Test: `integration_test.go:145-151` | test ada | **DONE** (job tracking: PARTIAL, lihat IMP-09) |
| P0 | Partial failure (transaksi atau rollback) | Satu transaksi Serializable (`excel_import.go:42-46`); error mana pun return tanpa commit (`:65`, `:109-118`); test "database failure rolls back earlier rows" (`integration_test.go:180-186`) | test ada | **DONE** (test belum dijalankan, lihat §8) |
| P0 | Malicious Excel (ukuran, formula, resource limit) | Batas 5 MiB, 501 baris, 100 kolom, 16 MiB teks, 32767 unit per sel (`xlsx.go:19-25`); ukuran dekompresi dibatasi dari ukuran yang dideklarasi (`xlsx.go:312-330`); formula ditolak (`:491-493`); file harus `.xlsx` dari nama file (`project_excel.go:203`) | `xlsx_test.go:64-92` lulus | **DONE** (catatan: cek ekstensi dari nama file, tidak cek isi/MIME) |
| P1 | Broken hierarchy | Parent harus ada di file dan beda baris (`excel_import.go:370-374`) | test siklus | **DONE** |
| P1 | Invalid dependency dan siklus | Dependency tidak diimpor (**MISSING**). Siklus hierarki dicek (`:362-366`) | test siklus | **PARTIAL** |
| P1 | Status tidak ada di workflow | Status tidak diimpor (**MISSING**), jadi risiko belum ada | — | N/A sekarang; wajib dicek saat status ditambah |
| P1 | Drift tanggal dan kalender kerja | Tanggal disimpan apa adanya (`excel_import.go:285-297`). Jalur `create` tidak memanggil `schedule` (`create.go` tanpa pemanggilan `schedule`); `prepareSchedulePatch` hanya dipakai di patch (`patch.go:118`, `:160`) | kode | **NOT VERIFIED** (dugaan DEFECT: start + duration bisa tidak cocok dengan due dan kalender) |
| P1 | Mismatch custom field | Tipe dinormalisasi, field arsip dibuang, budget hanya project, people divalidasi (`excel_import.go:319-348`) | kode | **DONE** (format ekspor: lihat EXP-06) |
| P1 | Automation side effect saat bulk import (default MVP: tidak memicu automation) | Setiap create membuat event `item.created` di transaksi yang sama (`create.go:350` → `patch.go:417-420` → `emit.go:22`). Automation membaca event itu (`automation/engine.go:67-78`, `eval.go:137`), dan webhook juga berlangganan `item.created` (`webhooks/webhooks.go:59`). Impor tidak punya opsi untuk menekan event | kode | **DEFECT** (bertentangan dengan default MVP) |

Catatan tambahan: impor juga membuat notifikasi per baris (`create.go:353` `notifyChanges`). Belum dicek apakah ini ikut menjadi spam; dicatat sebagai pertanyaan.

---

## 7. Status skenario IMP-T01 s.d. IMP-T14 dan EXP-T01 s.d. EXP-T07

**Catatan penting:** daftar nama skenario ada di brief, tapi isi detail tiap skenario tidak ada di brief dan tidak ditemukan di repo (`grep IMP-T` kosong). Tabel di bawah memakai **tafsiran dari judul** dan harus dikonfirmasi user (§10).

| ID | Tafsiran skenario | Status | Bukti |
|---|---|---|---|
| IMP-T01 | Unduh template | **Belum** | Tidak ada template (IMP-01) |
| IMP-T02 | Wizard 4 tahap | **Belum** | IMP-02 |
| IMP-T03 | Format kolom dan contoh tampil | **Belum** | IMP-03 |
| IMP-T04 | Error menyebut sheet, cell, field | **Sebagian** | Cell dan field ada; sheet belum (§4) |
| IMP-T05 | Workbook bukan format LENS ditolak dengan pesan format | **Belum** (DEFECT) | Repro B dan C lolos parser (§4) |
| IMP-T06 | Pemilihan worksheet | **Belum** (DEFECT) | Repro B mengambil sheet pertama (§4) |
| IMP-T07 | Formula ditolak dengan pesan spesifik | **Sudah** | `xlsx_test.go:64-92` (kasus "formula"), repro A |
| IMP-T08 | Mapping kolom tampil dan bisa diubah | **Sebagian** | Ada e2e `project-excel.spec.ts:26` (belum dijalankan); test Go tidak ada |
| IMP-T09 | Hierarki parent-child | **Sudah** (kode dan test, `integration_test.go:129-160`) | belum dijalankan |
| IMP-T10 | Siklus ditolak | **Sudah** (`integration_test.go:170-176`) | belum dijalankan |
| IMP-T11 | ID duplikat di file | **Sebagian**: kode ada (`excel_import.go:240-242`), test belum ditemukan | |
| IMP-T12 | Idempotensi (file sama, digest sama) | **Sudah** (`integration_test.go:145-151`) | belum dijalankan |
| IMP-T13 | Rollback saat gagal di tengah | **Sudah** (`integration_test.go:180-186`) | belum dijalankan |
| IMP-T14 | Cross-Space dan izin (403 vs 404) | **Sebagian**: kode ada (`project_excel.go:165-177`), test khusus belum | |
| EXP-T01 | Table view mempertahankan filter, sort, kolom | **Belum** | EXP-01 |
| EXP-T02 | Subtree project | **Sudah** (`integration_test.go:52-100`) | belum dijalankan |
| EXP-T03 | Folder sebagai sumber ekspor | **Belum** (DEFECT: `export.go:34`) | EXP-02 |
| EXP-T04 | Ekspor Space | **Belum** | EXP-03 |
| EXP-T05 | Field tertutup tidak ikut | **Belum** (tidak ada test; kode ada) | `export.go:127-131` |
| EXP-T06 | File lama tidak membocorkan item yang sudah pindah | **Sudah** (`integration_test.go:59`, `:88`) | belum dijalankan |
| EXP-T07 | Teks tidak dijalankan sebagai formula saat diekspor | **Sudah** (`xlsx_test.go:14-45`, `TestWorkbookLiteralText`) | lulus |

---

## 8. Yang tidak dicek dan status verifikasi

- **Test integrasi Go (DB)**: `go test ./server/internal/httpserver/ -run 'TestProjectExcel|TestBrowserExcel'` menghasilkan `ok`, tetapi test di-**skip** karena `PMS_TEST_DATABASE_URL` tidak diisi (`testdb` memanggil `t.Skip`). Ada Postgres yang menerima koneksi di `/tmp:5432`, tetapi tidak dipakai karena tidak ada konfigurasi yang diminta user. **NOT VERIFIED.**
- **Test e2e web** (`web/tests/project-excel.spec.ts`, `excel-roundtrip-live.spec.ts`): tidak dijalankan. **NOT VERIFIED.**
- **Nilai custom field teks/orang di ekspor (EXP-06)**: dugaan dari membaca kode (JSON mentah). Belum diuji dengan data nyata. **NOT VERIFIED.**
- **Automation dan webhook saat impor**: alur kode sudah dibaca, tapi belum dijalankan end-to-end. **NOT VERIFIED.**
- **Restricted field saat unduh**: belum diuji dengan grant yang dicabut setelah ekspor. **NOT VERIFIED.**
- **Drift tanggal dan kalender**: belum dijalankan. **NOT VERIFIED.**
- **Performa**: 500 baris, file 5 MiB, dan ekspor besar tidak diukur.
- **UI dengan browser**: dialog impor/ekspor tidak dibuka di browser pada sesi ini.
- **Kolom lain di workbook**: pengecekan hanya untuk header dan sel, tidak untuk format angka, gaya, atau tabel Excel (ListObject).

---

## 9. Usulan urutan batch IE-B0 s.d. IE-B7

Urutan disusun dari kode yang ada: keamanan dan defect dulu, lalu pengalaman pengguna, lalu cakupan data.

| Batch | Isi | Dasar kode / ID | Catatan |
|---|---|---|---|
| **IE-B0** | Baseline: jalankan test DB-backed (`PMS_TEST_DATABASE_URL`), tambah test cross-Space, field tertutup, ID duplikat, dan fixture sheet/format | §8, IMP-T11, IMP-T14, EXP-T05 | Hanya test, tanpa perubahan perilaku |
| **IE-B1** | Pemeriksaan format workbook: tolak workbook tanpa header LENS dengan pesan format; pilih sheet bernama (mis. "Tasks"); sebut nama sheet di error | IMP-04, IMP-T05, IMP-T06, §4 | Perubahan di `projectexcel/xlsx.go` dan `excel_import.go` |
| **IE-B2** | Defect P0/P1: (a) impor tidak memicu `item.created` untuk automation dan webhook (opsi default off); (b) unduhan ekspor mengecek ulang grant field | §6, EXP-06 | Perlu keputusan §10 soal automation |
| **IE-B3** | Preview: hitung valid/invalid/warning terpisah; jalankan validasi aturan create sebelum commit; laporan baris yang gagal | IMP-08, IMP-09 | Tanpa job baru |
| **IE-B4** | Kontrak kolom dan template: alias External Key/Parent Key/Finish Date; tambah status, priority, assignee email, effort, description; buat 3 template | IMP-01, IMP-03, §5 | Keputusan §10 wajib sebelum mulai |
| **IE-B5** | Ekspor: folder, table view, Space, field sistem, nilai custom field yang bisa dibaca, metadata workflow | EXP-01…EXP-05, EXP-08 | `CheckScope` perlu diubah untuk folder |
| **IE-B6** | Dependency (impor dan ekspor) dan kalender kerja untuk tanggal impor | EXP-07, §6 (P1) | Perlu format "Depends On" |
| **IE-B7** | Wizard 4 tahap di UI, progress ekspor, preset mapping/kolom, MS Project (dicatat, tidak dikerjakan) | IMP-02, EXP-09, EXP-10, IMP-10 | Batch UI terakhir |

---

## 10. Pertanyaan keputusan untuk user (tidak diasumsikan)

1. **Nama kolom resmi**: pakai `External Key`/`Parent Key` sebagai nama header di template, atau tetap `id`/`parent_id` dengan alias?
2. **Status**: diisi dengan nama status atau ID? Jika status tidak ada di workflow item, error atau pakai status default?
3. **Assignee Email**: email harus user aktif yang sudah ada di Space? Jika tidak cocok, error atau warning?
4. **Tujuan impor**: cukup Space root untuk MVP, atau perlu pilih Project/Folder sebagai induk?
5. **Automation saat impor**: setuju default off untuk semua impor (termasuk webhook), atau perlu opsi per impor?
6. **Ekspor folder**: boleh? Dan "Projects/folders filtered" berarti filter tabel yang aktif, atau filter lain?
7. **Nilai custom field di ekspor**: tampilkan nilai yang mudah dibaca (label, nama orang), atau format mesin yang bisa diimpor ulang?
8. **Batas 500 item per file** cukup untuk MVP?
9. **Depends On**: format apa (external key, tipe FS, lag)? Apakah lag perlu?
10. **Tanggal impor**: dihitung ulang dengan kalender kerja Space, atau disimpan apa adanya?
11. **Tiga template**: file statis di repo atau dibangkitkan server?
12. **Skenario IMP-T dan EXP-T**: detail skenario belum ada di brief. Apakah tafsiran di §7 bisa dipakai?
13. **Grant dicabut setelah ekspor**: file harus langsung tidak bisa diunduh (cek ulang saat unduh)?
14. **Notifikasi per baris** saat impor besar: tetap dikirim, atau dikumpulkan jadi satu?

---

## 11. Batas audit

- Tidak ada kode yang diubah. Satu-satunya file baru adalah dokumen ini (belum di-commit).
- Fixture dan file uji sementara ada di scratchpad, bukan di repo.
- Perintah yang dijalankan: pencarian `grep`, baca file, `go test ./server/internal/projectexcel/` (lulus), `go test ./server/internal/httpserver/ -run ...` (skip karena DB tidak diatur), dan `go test -overlay ... -run TestReproFixtures` dengan file uji dari scratchpad.
