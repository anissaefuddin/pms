# Analisa Fitur Blueprint (LENS PMS)

Tanggal: 18 Nov 2026
Cakupan: baca kode, API, skema database, permission, dan test. Tidak ada kode yang diubah, tidak ada commit, tidak ada PR.
Dokumen ini adalah bahan untuk batch B0 (audit).

Catatan metode: semua test yang disebut di bawah hanya **dibaca** dari kode, tidak dijalankan. Status "Sudah" berarti ada kode dan test yang mengkonfirmasi, bukan hasil run di sesi ini.

---

## 1. Ringkasan (5 kalimat)

Blueprint sudah ada sebagai fitur sederhana: tabel `blueprints` menyimpan satu pohon JSON per Space, dan ada endpoint untuk daftar, simpan, hapus, dan buat item dari blueprint. Backend sudah menjalankan pembuatan hierarki, dependency, status, dan custom field dalam satu transaksi, dengan permission Space dan test integrasi yang mencakup kegagalan parsial. Kekurangan terbesar ada di sisi produk: tidak ada editor, tidak ada endpoint baca satu blueprint, tidak ada lifecycle (Draft/Active/Archived), tidak ada versi dan statistik pemakaian, dan tidak ada dukungan Custom Item Type (`type_key` hilang saat disimpan). Penjadwalan ulang memakai selisih hari kalender sendiri, tidak memakai service kalender kerja (`internal/schedule`) yang sudah dipakai Gantt dan dependency. Ada juga dua temuan keamanan yang tidak ada di daftar review (lihat bagian 4), yaitu data dari Space sumber bisa dibaca lewat blueprint oleh pengguna yang tidak punya akses ke sumbernya, dan nilai custom field yang restricted ikut tersalin tanpa pengecekan saat simpan, sehingga perbaikan ini sebaiknya dinaikkan prioritasnya.

---

## 2. Peta kode yang ada

| Lapisan | Lokasi | Isi |
|---|---|---|
| Skema | `server/migrations/00032_blueprints.sql:1-12` | Tabel `blueprints(id, space_id, name, tree jsonb, created_by, created_at)`, unik per Space pada `lower(name)`. Tidak ada kolom status, versi, `updated_at`, atau hitungan pemakaian. |
| Route | `server/internal/httpserver/blueprints.go:11-16` | `GET/POST /api/v1/spaces/{spaceId}/blueprints`, `DELETE /api/v1/blueprints/{id}`, `POST /api/v1/blueprints/{id}/items`. Tidak ada GET satu blueprint, tidak ada PATCH, tidak ada archive/restore. |
| Logika | `server/internal/items/blueprints.go` (424 baris) | `SaveBlueprint` (50-196), `ListBlueprints` (213-228), `DeleteBlueprint` (230-251), `UseBlueprint` / `useBlueprint` (269-388), `restoreStatusAndFields` (392-419). |
| Form | `server/internal/forms/forms.go:232-240`, `:504-512`; `server/internal/httpserver/form_submit.go:472-500` | `blueprint_id` per rute form; validasi Space tujuan; blueprint yang sudah dihapus dilewati. |
| Create dialog | `web/src/components/create-item-dialog.tsx:116-117`, `:154-158`, `:230-248` | Pilihan "Start from blueprint"; jalur blueprint hanya mengirim `locations`, `title`, `start_date`. |
| Catalogue | `web/src/routes/_app/blueprints.tsx`, `web/src/lib/workspace-metrics.ts:71-80` | Halaman `/blueprints` (kartu per Space, dialog "Create blueprint", tiga preset "Suggested"). |
| Simpan dari item | `web/src/components/save-blueprint-dialog.tsx`; `web/src/components/item-detail.tsx:206`, `:318` | Menu "Save as blueprint" di item detail. |
| Navigasi | `web/src/components/sidebar.tsx:266`; `web/src/components/topbar.tsx:160`; `web/src/routes/_app/spaces.$spaceId.tsx:147` | Entri sidebar, opsi "From blueprint" di menu Create, empty state Space. |
| Teks | `web/src/i18n/en/blueprints.json` | Deskripsi halaman: "Reusable starting points for projects and folders." |
| Test Go | `server/internal/httpserver/blueprints_integration_test.go` (`TestBlueprintSaveAndUse`); `m3_authorization_integration_test.go:50-52, 126-131`; `endpoint_sweep_integration_test.go:194-196`; `form_blueprint_routing_integration_test.go` (`TestFormBranchingBlueprintItem`); `m8_browser_integration_test.go:47-68` | Simpan, pakai, izin, kegagalan parsial, rute form. |
| Test web | `web/tests/blueprints.spec.ts` (mock Playwright); `web/tests/form-blueprint-live.spec.ts` (live); `web/src/lib/workspace-metrics.test.ts` | Simpan dan pakai dari item (task), pengelompokan katalog, rute form. |

Dokumen perencanaan di repo: `docs/TASKS.md:542-546` (S3.2.2 ditandai selesai), `docs/TASKS.md:939` (menu blueprint: tipe, nama, ikon, belum dicentang), `docs/TASKS.md:1792-1796` (S-UX.C6 Halaman Blueprints, belum dicentang), `docs/DECISIONS.md:116` (D-101, routing form ke blueprint), `docs/PRD.md:140-141` (SPC-13 P1, SPC-14 P2).

---

## 3. Empat konsep: sudah dipisah di kode atau belum?

| Konsep | Di kode sekarang | Catatan |
|---|---|---|
| **Custom Item Type** (bentuk, field, default, layout) | Ada di `item_type_defs` dan `items.type_id` (`migrations/00046_item_types.sql:23`). `create()` memakai `type_key` dan menerapkan `Defaults` tipe. | Blueprint **tidak** menyimpan `type_id` atau `type_key`. Node hanya menyimpan kelas dasar (`type`). |
| **Blueprint** (isi dan struktur template) | `blueprints.tree` jsonb: node (parent index, tipe dasar, judul, deskripsi, status, assignee, offset tanggal, field) dan dependency. | Pemisahan sudah ada: tree tidak punya referensi ke item sumber. |
| **Request Form / Create Dialog** (input dan override) | Form: `overrides` dari jawaban form di `form_submit.go:484-491`. Create dialog: hanya `start_date`, `title`, `locations`. | Override hanya berlaku pada root (`blueprints.go` `if i == 0`). |
| **Live item** (Task/Project/Folder nyata) | `create()` membuat baris baru dengan id baru. Tidak ada link ke blueprint. | Sudah sesuai: edit item hasil tidak mengubah blueprint (diuji di `blueprints_integration_test.go`, bagian "No shared data"). |

Kesimpulan: pemisahan blueprint dan live item sudah benar. Yang belum terpisah dengan baik adalah Custom Item Type, karena tipenya diperlakukan sebagai kelas dasar.

### Urutan default nilai (input user > Request Form > Blueprint > Custom Item Type > system)

Yang ada di kode:
1. **Input user / form (root saja):** `UseBlueprint` menerima `Title` dan `StartDate` dari API; `form_submit.go` mengirim `Overrides` dari `fields` form. Override diterapkan di `blueprints.go` (`if i == 0 { for k, v := range use.Overrides ... }`). `status_id` dari override membuat status blueprint diabaikan.
2. **Blueprint:** nilai per node (`fields` dari `blueprintNode`).
3. **Custom Item Type:** **tidak ada di jalur blueprint.** `create()` hanya menerapkan `Defaults` tipe jika `type_key` diberikan, sedangkan node blueprint tidak membawa `type_key`. Pada jalur form, `type_key` root ikut sebagai override, jadi hanya root yang mendapat tipe custom; anak tetap tipe dasar.
4. **System default:** dari `create()` dan default kolom.

Perilaku nilai kosong (dari kode):
- **null / tidak dikirim:** `start_date` null berarti item tanpa tanggal (`scheduling.go:48-49`). `Title == nil` berarti judul blueprint dipakai.
- **string kosong:** `start_date: ""` gagal parse dan menghasilkan 422 (`scheduling.go:51-53`), tidak diam-diam. Create dialog hanya mengirim `start_date` jika terisi (`create-item-dialog.tsx`, `...(start && {...})`).
- **`title: ""` dari API:** `applyFields` memvalidasi panjang judul 1-500 (`patch.go:229-233`). Perlu dikonfirmasi dengan test, karena tidak ada test blueprint untuk kasus ini.
- **`assignee_ids` kosong dari form:** daftar kosong tidak menimpa assignee blueprint (`form_submit.go`, `len(assignees) == 0`). Ini aturan yang disengaja.
- **0:** `duration_days`, `effort_minutes`, `start_offset`, `due_offset` memakai pointer (`*int`), jadi 0 tetap tersimpan dan dipakai. Ini benar.
- **false:** `is_milestone` false tidak disimpan dan memakai default. Benar secara hasil.
- **Form, string kosong / false:** belum ditelusuri sampai level field di `form_submit.go`. Pertanyaan terbuka.

---

## 4. Tabel BP-01 sampai BP-17

Legenda: **Existing** = sudah ada dan cukup untuk target; **Partial** = sebagian; **Missing** = belum ada.

| ID | Prioritas | Status | Bukti (file:baris, endpoint, tabel, test) | Catatan |
|---|---|---|---|---|
| BP-01 Catalogue | P1 | **Partial** | `web/src/routes/_app/blueprints.tsx` (kartu per Space, dropdown filter Space, tombol "Use blueprint" dan "Create blueprint"); `lib/workspace-metrics.ts:63-67` (kelompok per Space, urut nama). | Sudah: kartu, filter Space, pengelompokan Space, buat, pakai. **Belum:** search, sort pilihan, filter jenis item, pengelompokan owner, pembuat/tanggal/status/pemakaian di kartu, tab All/Workspace/Space/Archived, Table view, aksi detail/duplicate/archive/hapus (API DELETE ada, tidak ada tombol di UI). Deskripsi `en/blueprints.json` "Reusable starting points for projects and folders" **keliru**: backend (`items/blueprints.go:50-61`, tanpa cek tipe) dan OpenAPI (`BlueprintItemType` berisi `task`) menerima Task, dan kunci `blueprints.type.task` sudah ada. Dialog katalog juga hanya menawarkan project/folder (`candidates` filter `item.type === type`). |
| BP-02 Creation flow | P1 | **Partial** | `CreateBlueprintDialog` di `blueprints.tsx`; `create-item-dialog.tsx:230-241` (radio "Start from blueprint"); `workspace-metrics.ts:71-80` (3 preset). | Entry "save existing work" **ada** (dialog katalog dan item detail). Entry "from scratch" **belum**: dialog katalog wajib memilih item sumber yang sudah ada. Entry "suggested template" hanya **mengisi nama dan jenis**, tanpa struktur; tidak ada isi template dan tidak ada API. Judul "Create from Blueprint" **belum**: judul dialog tetap `items.create` ("Create item", `en/items.json:14`) dan label radio "From blueprint". |
| BP-03 Editor blueprint (gap utama) | P1 | **Missing** | `blueprints.go:11-16` (tidak ada GET satu, tidak ada PATCH); tidak ada komponen editor di `web/src`. | Isi blueprint tidak bisa dilihat di Table, Gantt, Board, Calendar, atau Item Detail. Satu-satunya cara "melihat" isi blueprint adalah dengan membuat item dari situ. Ini blocker untuk B1. |
| BP-04 Save existing work | P1 | **Partial** | Entry: hanya `item-detail.tsx:206` (menu "Save as blueprint", untuk semua tipe item). `items/blueprints.go:63-196` (`SaveBlueprint`). Test: `TestBlueprintSaveAndUse` (project), `blueprints.spec.ts` (task, mock). | Sudah: tidak menyalin komentar, lampiran, approval, share, dan time log (tidak ada di query). Belum: entry dari context menu task, folder, dan header project; pilihan atribut (tidak ada toggle, semua disalin). **Kebijakan custom field belum:** `items/blueprints.go:136` membaca `fields` mentah tanpa cek field restricted (lihat temuan X2). Validasi akses: sudah (`requireSpace` Edit di tujuan, View di sumber, `blueprints.go:110-112`). |
| BP-05 Create from blueprint dan reschedule | P1 (tinggi) | **Partial** | `items/blueprints.go:299-303` (anchor `start_date`), `:353-359` (offset kalender dengan `AddDate`), `:322-327` (lokasi), `:329` (judul root). Test: `TestBlueprintSaveAndUse` ("relative dates" 2026-06-08 / 06-09). | Sudah: judul (root), lokasi, anchor tanggal mulai, dependency ikut. Belum: pilihan prefix, pilihan notifikasi assignee (notifikasi mengikuti `create()`), **reschedule by start/finish** (hanya satu anchor, tidak ada pilihan finish), dan **service penjadwalan yang sama dengan Gantt/Work Schedule**. Blueprint memakai `AddDate` sederhana; `internal/schedule.Calendar` (`calendar.go:18-65`, `Finish`, `Offset`, `NextWorkday`) dan `loadCalendar` (`scheduling.go:31-40`) tidak dipanggil. Preview di UI tidak ada. |
| BP-06 Scope Account/Space | P2 | **Partial** | Skema: `blueprints.space_id` (FK, cascade). `policy/engine.go:86-91` (personal Space hanya milik pemilik). Save: `requireSpace` Edit (boleh di personal Space untuk pemilik). UI: `save-blueprint-dialog.tsx` filter `kind === "shared"` dan `can edit`. | Belum ada scope Account. Backend mengizinkan blueprint di personal Space, UI menyembunyikannya; tidak konsisten. Kepemilikan hanya `created_by` (tidak ditampilkan di API list). Visibility mengikuti Space (View). |
| BP-07 Integrasi Custom Item Type | P1 | **Missing** | `items/blueprints.go:21-35` (`blueprintNode` hanya `Type`, tanpa `type_id`/`type_key`); `:134` `SELECT type::text`; `create.go` `resolveCreateType` (`type_key` hanya dari fields). Catatan proyek: `docs/TASKS.md:1000` menyatakan node blueprint "hanya menyimpan kelas, bukan `type_key`". | Custom Item Type hilang saat simpan; item hasil pakai menjadi tipe dasar. Field default dan layout tipe tidak ikut. Tidak ada test. |
| BP-08 Dependency remapping | P1 | **Existing** | `items/blueprints.go:166-180` (dependency dalam tree, pemetaan index), `:381-386` (insert ke id baru). Test: `TestBlueprintSaveAndUse` ("copied dependency"). | Dependency ke item di luar tree dibuang diam-diam. Dependency di-insert langsung, **tanpa** reschedule (`dependencies.go:216-221` memakai `reschedule`; blueprint tidak). |
| BP-09 Copy semantics | P1 | **Partial** | Query save: `items/blueprints.go:134-137`. | Tetap (tanpa pilihan): subitem, status, assignee aktif, deskripsi, tanggal, durasi, milestone, effort, custom field, dependency. Tidak disalin: komentar, lampiran, approval, share, audit, time log (benar). **Belum disalin padahal sering diminta:** default view (`default_view_type`), default workflow (`default_workflow_id`), owner project. Tidak ada kontrol per atribut. |
| BP-10 Work schedule | P1 | **Missing** | `items/blueprints.go:353-359` (kalender hari); `:268` comment `shortcut:` tentang dependent dates; `httpserver/worksched.go` (kalender kerja ada, tidak dipakai blueprint). | Hasil pakai tidak memperhitungkan hari kerja dan hari libur. Ini kontradiksi dengan target BP-10. |
| BP-11 Permissions | P1 | **Partial** | `items/blueprints.go:64` (save: Edit), `:242` (delete: Edit), `:292` (use: View di Space sumber), `create.go` (View dan Create per lokasi, `ErrForbidden`/`ErrNotFound`). Test: `blueprints_integration_test.go` (reader 403, outsider 404), `m3_authorization_integration_test.go:126-131`. | Backend sudah menegakkan (bukan hanya UI). **Belum:** pemisahan hak create dan use; belum ada hak admin workspace/Space khusus blueprint. Role `Editor`, `Full`, dan user `External` memegang Edit (`policy/policy.go:93-104`), sehingga bisa **hapus** blueprint siapa pun di Space (delete memakai Edit, bukan Delete). Jalur form memakai actor `elevated` (`form_submit.go`, `policy.Admin`), jadi permission pembuat form tidak dicek ulang. |
| BP-12 Integrasi Request Form | P1 | **Existing** (dengan catatan) | `forms.go:504-512` (blueprint harus di Space tujuan, else 422); `form_submit.go:472-500` (blueprint hilang dilewati, tanpa 5xx); `form_submit.go:484-491` (override). Test: `TestFormBranchingBlueprintItem`, `m8_browser_integration_test.go:47-68`, `web/tests/form-blueprint-live.spec.ts`. | Sudah sesuai D-101. Catatan: tipe custom anak hilang (BP-07), tanggal blueprint tidak diberi jangkar (sesuai D-101), dan override hanya root. |
| BP-13 Lifecycle (Draft, Active, Archived, restore) | P2 | **Missing** | Skema `00032` (tidak ada kolom status); `blueprints.go:63-73` (hanya hapus permanen). | Tidak ada draft, archive, atau restore. Hapus permanen berlawanan dengan keputusan user (gunakan archive). |
| BP-14 Version history | P2 | **Missing** | Skema; tidak ada tabel versi. Simpan selalu membuat baris baru. | Belum ada. |
| BP-15 Usage tracking | P2 | **Missing** | Tidak ada tabel atau kolom pemakaian. `items/custom_field_lifecycle.go:61` menghitung blueprint yang memuat field (untuk dampak field, bukan pemakaian blueprint). | Belum ada jumlah pemakaian maupun lokasi hasil. Usulan: tabel `blueprint_uses(blueprint_id, item_id, created_by, created_at)`. |
| BP-16 Large hierarchy protection | P1 | **Partial** | `items/blueprints.go:16` (`maxBlueprintNodes = 200`), `:124-125` (batas 200 saat simpan, 422), `:77` (`cardinality(path) < 30`), `:110-112`, transaksi `Serializable` (`:51`, `:270`). Test: "partial tree left behind" (`blueprints_integration_test.go`). | Sudah: batas node, transaksi (satu tree atau tidak sama sekali). **Belum:** retry saat serialization conflict (SQLSTATE 40001 tidak diulang); batas kedalaman 30 **memotong diam-diam** (lihat X3); tidak ada timeout. |
| BP-17 Cross-Space compatibility | P1 | **Partial** | Status: `blueprints.go` `restoreStatusAndFields` (`:392-400`, status harus satu workflow dengan item). Custom field: `updateCustomFieldValues` (item harus di Space itu, `custom_fields.go:349-395`), error `InvalidField` dilewati (`blueprints.go:408-410`). Assignee: hanya yang aktif (`:305-319`). Tipe: `create.go` "this item type belongs to another Space". Test: `blueprints_integration_test.go` (pakai di Space lain). | Sudah: status, field, dan assignee divalidasi di Space tujuan; akses lewat `create()`. **Belum:** pemberitahuan ke pengguna apa yang dibuang (semua dibuang diam-diam); validasi saat simpan, bukan saat pakai; test mismatch workflow/field lintas Space belum ada. |

---

## 5. Tabel BP-T01 sampai BP-T15

**Catatan penting:** teks lengkap BP-T01 sampai BP-T15 tidak ada di konteks sesi ini. Saya hanya punya ringkasan dua belas skenario dari permintaan (urutan dipetakan ke BP-T01 sampai BP-T12 sesuai urutan ringkasan, ini **asumsi**). BP-T13 sampai BP-T15 tidak bisa dipetakan (lihat pertanyaan terbuka).

| ID (asumsi) | Skenario (dari ringkasan) | Status | Test yang ada | Catatan |
|---|---|---|---|---|
| BP-T01 | Membuat project blueprint dari awal | **Belum** | Tidak ada | Tidak ada jalur tanpa item sumber. |
| BP-T02 | Simpan project / task yang sudah ada | **Sudah** | `TestBlueprintSaveAndUse` (project, Go); `blueprints.spec.ts` (task, mock Playwright) | Task hanya diuji dengan mock, bukan live. |
| BP-T03 | Create project dari blueprint dengan ID baru | **Sudah** | `TestBlueprintSaveAndUse` ("copies of Plan = 2", root baru) | Sudah. |
| BP-T04 | Reschedule by start dan finish | **Belum** | Tidak ada | Hanya anchor start; tidak ada finish; tidak ada kalender kerja. |
| BP-T05 | Custom Item Type | **Belum** | Tidak ada | `type_key` hilang (BP-07). |
| BP-T06 | Custom field dengan izin | **Sebagian** | Tidak ada test blueprint (ada test field umum: `custom_field_access_integration_test.go`) | Nilai dipasang lewat `updateCustomFieldValues` dengan actor pemakai; restricted dibuang diam-diam. Tidak ada bukti tertulis untuk blueprint. |
| BP-T07 | Request form | **Sudah** (dengan catatan) | `TestFormBranchingBlueprintItem`; `m8_browser_integration_test.go`; `form-blueprint-live.spec.ts` | Catatan BP-12. |
| BP-T08 | Izin backend | **Sudah** | `m3_authorization_integration_test.go:126-131`; `blueprints_integration_test.go` (403/404) | Belum ada test untuk peran Editor menghapus blueprint (yang seharusnya mungkin dilarang). |
| BP-T09 | Lintas Space | **Sebagian** | `blueprints_integration_test.go` (pakai di `other`); form cross-Space ditolak (`forms.go:504-512`) | Belum ada test workflow/field/assignee yang berbeda antar Space. |
| BP-T10 | Restricted field | **Sebagian** | Tidak ada test blueprint | Restricted dicek saat pakai, **tidak** saat simpan (X2). |
| BP-T11 | Kegagalan parsial tidak menyisakan data | **Sudah** | `blueprints_integration_test.go` ("partial tree left behind", hitungan item sebelum/sesudah) | Sudah dan diuji. |
| BP-T12 | Perubahan blueprint tidak mengubah project yang sudah dibuat | **Sebagian** | `blueprints_integration_test.go` ("source changed", "copies of Plan") | Belum ada endpoint edit blueprint, jadi skenario "ubah blueprint" belum bisa dijalankan. Yang diuji: ubah item hasil dan item sumber. |
| BP-T13 | (tidak diketahui) | — | — | Perlu teks dari user. |
| BP-T14 | (tidak diketahui) | — | — | Perlu teks dari user. |
| BP-T15 | (tidak diketahui) | — | — | Perlu teks dari user. |

---

## 6. Temuan tambahan (tidak ada di daftar review)

**X1 — Pembaca Space tujuan bisa mendapat data Space sumber (P1, keamanan/isolasi).**
Saat simpan, sumber hanya perlu View oleh penyimpan (`items/blueprints.go:110-112`). Blueprint lalu bisa dipakai oleh siapa pun yang punya **Edit** atau **View + Create** di Space blueprint (`useBlueprint`, `:292`), dan deskripsi, status, dan field dari item sumber ikut disalin. Pengguna yang tidak pernah bisa melihat sumbernya dapat membuat item berisi deskripsi sumber. Ini perlu keputusan: apakah blueprint boleh membawa konten yang bukan milik Space-nya.

**X2 — Nilai custom field yang restricted ikut tersalin saat simpan (P1).**
`items/blueprints.go:136` menyalin `items.fields` mentah. Tidak ada `HiddenFieldIDs`, padahal `updateCustomFieldValues` (`custom_fields.go:394`) memakainya saat pasang. Nilai restricted tersimpan dalam `blueprints.tree`. Saat pakai, nilai hanya dipasang jika pemakai punya grant edit untuk field itu, jadi dampaknya terbatas, tetapi nilai tetap ada di database tanpa perlindungan.

**X3 — Pemotongan diam-diam (P2).**
- Kedalaman: CTE berhenti di `cardinality(path) < 30` (`items/blueprints.go:77`). Item di kedalaman lebih dari 30 tidak disalin dan tidak ada error.
- Anak yang induknya tidak terlihat dilewati (`:121`), tanpa pemberitahuan.
- Batas 200 node menghasilkan 422 (`:124-125`), jadi batas ini tidak diam-diam.

**X4 — Anchor adalah tanggal paling awal di tree, bukan tanggal mulai root (P2).**
`base` dihitung dari tanggal paling awal semua node (`items/blueprints.go:143-147`). Input "start_date" di pakai membuat paling awal jatuh di tanggal itu. Jika user mengharapkan "tanggal mulai project", hasilnya bisa berbeda. Ini harus dikonfirmasi (pertanyaan terbuka).

**X5 — Due date di dialog Create diabaikan saat memakai blueprint (P2).**
`create-item-dialog.tsx` (jalur `blueprint`) hanya mengirim `locations`, `title`, dan `start_date`. `due_date` tetap divalidasi di form, tetapi tidak dikirim, sehingga user bisa mengisi tanggal selesai yang tidak berpengaruh.

**X6 — Hapus permanen dengan hak Edit, tanpa audit (P2).**
`DeleteBlueprint` (`:230-251`) memakai Edit. Tidak ada entri audit untuk simpan, pakai, atau hapus blueprint (tidak ada pemanggilan audit di `items/blueprints.go` maupun `httpserver/blueprints.go`). Keputusan user: gunakan archive.

**X7 — Tidak ada endpoint baca satu blueprint (P2).**
Akibat langsung BP-03. Selain itu, tidak bisa memeriksa isi tree lewat API, sehingga test dan audit harus lewat database.

**X8 — Hitungan hari memakai pembulatan (P3).**
`items/blueprints.go:156` memakai `int(day.Sub(*base).Hours() / 24)`. Aman di zona waktu tanpa DST (kemungkinan besar kasus server ini), tetapi bisa meleset satu hari jika zona waktu sesi memakai DST. Perlu dikonfirmasi zona waktu server.

**X9 — Lokasi banyak hanya dipakai untuk root (P3).**
`items/blueprints.go:322-327`: anak selalu masuk ke `Locations[0]`. Ini tidak dijelaskan di UI.

**X10 — Form memakai actor elevated (P2, berkaitan BP-11).**
`form_submit.go` membuat item lewat blueprint dengan `policy.Admin`. Ini mungkin disengaja karena form publik, tetapi berarti permission Space pembuat form tidak dipakai. Harus dikonfirmasi sebelum B5.

**X11 — Dokumentasi rencana sudah menandai sebagian sebagai belum (info).**
`docs/TASKS.md:939`, `:1792-1796` (S-UX.C6, UX-14) belum dicentang. Ini sejalan dengan temuan BP-01 sampai BP-03. Catatan `docs/TASKS.md:1000` menyebut "Spec Playwright live belum ada", tetapi `web/tests/form-blueprint-live.spec.ts` sekarang ada; dokumentasi itu perlu diperbarui.

**X12 — Screenshot konsep di pesan user (info, bukan kode).**
Screenshot ini tidak ada di aplikasi: tidak ada "Copy configuration", "Draft", tab Table/Gantt/Calendar, atau "Schedule preview" di `web/src` (sudah dicari). Teks di layar sendiri menyebut "Konsep konfigurasi untuk development; tidak menyimpan Blueprint ke aplikasi." Ada dua hal yang perlu diperhatikan dari pratinjau jadwal:
- Pratinjau memakai "kalender offset", bukan hari kerja. Akibatnya "Technical review" mulai 06 Dec 2026, yang hari Minggu (1 Dec 2026 adalah Selasa; dihitung dari kalender).
- Angka pratinjau tidak sama dengan offset di template pada screenshot pertama (Requirements Day 1 seharusnya 02 Dec, pratinjau 03 Dec; Submission Day 10 seharusnya 11 Dec, pratinjau 09 Dec). Jadi pratinjau dan template di mockup tidak diturunkan dari data yang sama. Ini harus diperhatikan agar pratinjau dan hasil simpan tidak berbeda (syarat BP-05).

---

## 7. Pendapat tentang keputusan yang diusulkan

| Keputusan user | Pendapat | Kondisi kode saat ini |
|---|---|---|
| Scope Workspace + Space | **Setuju**, dengan catatan. | Hanya Space. Backend mengizinkan personal Space, UI menyembunyikannya (BP-06). Istilah "Workspace" belum ada di skema blueprint; perlu dipastikan artinya (pertanyaan terbuka). |
| Item: Task, Project, Folder, Custom Item Type | **Setuju.** | Backend sudah menerima Task (tanpa cek tipe). UI katalog hanya project/folder. Custom Item Type belum (BP-07). |
| Editor full workspace, bukan modal | **Setuju.** | Belum ada editor sama sekali. Perlu endpoint GET dan PATCH dulu (BP-03). |
| Katalog Table + Gallery | **Setuju.** | Hanya Gallery (kartu per Space). |
| Save existing item didukung | **Setuju, sudah ada.** | Ada di item detail untuk semua tipe. Perlu entry point di context menu dan header project/folder. |
| Copy attributes configurable | **Setuju.** Usulkan opsi di request body dengan default aman; komentar, time log, approval, dan audit tetap tidak bisa disalin (whitelist di backend). | Belum ada. Sekarang semua atribut tetap. |
| Reschedule relatif plus pilihan mempertahankan tanggal | **Setuju**, dengan catatan: "mempertahankan tanggal" perlu definisi. Saat ini tanpa `start_date` item menjadi tanpa tanggal, bukan mempertahankan tanggal. | Hanya relatif dari tanggal awal tree (X4). Kalender kerja belum dipakai (BP-10). |
| Permission: workspace admin / space admin dengan hak create dan use terpisah | **Setuju tujuannya, tetapi perlu desain capability yang tepat.** Kode hanya punya Edit (save/hapus) dan View+Create (pakai). `AllCapabilities` bersifat append-only (`policy/policy.go:27-28`), jadi capability baru harus ditambahkan di akhir dan role mask harus dicek ulang. | Belum ada pemisahan. Editor bisa hapus blueprint (BP-11). |
| Versioning P2 | **Setuju.** | Belum ada (BP-14). |
| Hapus permanen tidak diperlukan, gunakan archive | **Setuju.** | Kode masih hard delete (`blueprints.go:63-73`); belum ada archive (BP-13). |

---

## 8. Urutan batch yang disarankan (berdasarkan kode)

**B0 Audit (sudah sebagian selesai).** Dokumen ini adalah bagian dari B0. Yang belum: menjalankan test `TestBlueprintSaveAndUse`, `TestM3AuthorizationMatrix`, dan `blueprints.spec.ts` sebagai baseline. Ini tidak dilakukan di sesi ini.

**Usulan perubahan urutan, dengan alasan:**
1. **Tambahkan langkah "Keamanan cepat" ke B1 (dari X1 dan X2):** sebelum entry point bertambah (B2), perbaiki salinan field restricted dan batasi isi yang disalin ke izin sumber. Alasan: B2 menambah jumlah tempat simpan, jadi paparan ikut bertambah.
2. **B1 Registry dan editor:** migrasi skema dulu (kolom `type_id` atau `type_key` per node, `status` untuk Draft/Active/Archived, `copy_options`, `updated_at`), endpoint GET satu blueprint, PATCH, dan editor full workspace. Semua batch berikutnya bergantung pada skema ini.
3. **B2 Create dan save as blueprint:** dari awal (BP-T01), judul "Create from Blueprint" (BP-02), dan entry point context menu dan header (BP-04).
4. **B3 Clone engine dan penjadwalan:** `useBlueprint` sudah menjadi satu-satunya mesin untuk form dan API. Pertahankan itu (sesuai catatan user: jangan duplikasi). Tambahkan `schedule.Calendar`, reschedule finish, dan dependency yang diproses ulang (`reschedule`). Di sini juga perbaiki X3 (pemotongan diam-diam) dan X4 (definisi anchor).
5. **B4 Custom Item Type, Custom Fields, Request Forms:** simpan `type_id` di node, terapkan Defaults tipe pada anak, dan perpanjang test form (BP-07, BP-12).
6. **B5 Scope, permission, lifecycle, QA:** pemisahan hak create dan use, archive, dan keputusan atas X10 (form elevated).
7. **B6 Versioning dan statistik:** tabel versi (BP-14) dan tabel pemakaian (BP-15).

Alasan urutan: skema (B1) harus lebih dulu daripada editor dan entry point; penjadwalan (B3) harus memakai engine yang sama dengan form, jadi tidak boleh dibuat dua kali; Custom Item Type (B4) bergantung pada node yang sudah menyimpan tipe.

---

## 9. Pertanyaan terbuka untuk user

Pertanyaan keputusan (menunggu jawaban; saya tidak mengasumsikan):

1. **Isi lintas Space (X1):** apakah pengguna yang punya akses ke Space blueprint boleh menerima deskripsi dan data dari Space sumber yang tidak bisa mereka lihat? Jika tidak, apakah blueprint hanya boleh dibuat dari item yang bisa dilihat oleh semua pembaca Space tujuan, atau isi sumber dibersihkan saat simpan?
2. **Custom field restricted (X2):** apakah nilai field restricted boleh disalin ke blueprint? Saya usulkan tidak disalin sama sekali.
3. **Arti "Workspace" (BP-06):** apakah ada tabel atau konsep workspace terpisah dari Space dan Account? Jika tidak, apakah scope "Workspace" berarti semua Space di satu instalasi?
4. **Personal Space:** apakah blueprint boleh dibuat di personal Space? Backend mengizinkan, UI tidak.
5. **Anchor tanggal (X4):** untuk "start_date" pada pakai blueprint, apakah tanggal ini harus menjadi tanggal mulai root project, atau tanggal paling awal dalam tree (perilaku sekarang)?
6. **"Mempertahankan tanggal" (BP-05):** jika tidak ada tanggal baru, apakah item hasil harus memakai tanggal dari sumber, atau tanpa tanggal (perilaku sekarang)?
7. **Kalender kerja (BP-10):** kalender kerja mana yang berlaku? Saat ini ada satu kalender per instalasi (`loadCalendar` membaca dari database; konfirmasi bahwa ini global dan bukan per Space).
8. **Permission (BP-11):** siapa yang boleh membuat, memakai, mengarsipkan, dan menghapus blueprint? Apakah Editor boleh menghapus blueprint, atau hanya admin Space?
9. **Form elevated (X10):** apakah item dari form dengan blueprint memang harus dibuat tanpa mengecek permission pembuat form (actor elevated), atau seharusnya memakai permission pengisi form?
10. **Hapus (BP-13, X6):** apakah endpoint DELETE sekarang dipertahankan untuk admin, atau dihapus dan diganti archive saja?
11. **Pratinjau jadwal (X12):** apakah screenshot "Schedule preview" adalah target desain? Angkanya tidak cocok dengan offset template di screenshot pertama. Mana yang benar?
12. **Anchor dan format tanggal:** screenshot memakai format "01/12/26". Konfirmasi format tanggal yang dipakai (dd/mm/yy atau mm/dd/yy).
13. **Batas kedalaman (X3):** apakah batas 30 level sudah sesuai, atau perlu dinaikkan? Dan apakah pemotongan diam-diam harus diganti error?
14. **Versi (BP-14):** apakah setiap versi menyimpan seluruh tree, atau hanya perubahan?
15. **Notifikasi (BP-05):** apakah pembuatan dari blueprint perlu pilihan untuk mematikan notifikasi assignee?
16. **Teks BP-T13 sampai BP-T15:** tolong kirim teksnya. Saya juga perlu konfirmasi bahwa BP-T01 sampai BP-T12 memang sesuai urutan ringkasan yang saya pakai.
17. **Dua tabel keputusan:** apakah B0 dianggap selesai dengan dokumen ini, atau Anda ingin test baseline dijalankan dulu?

---

## 10. Yang tidak saya periksa

- Tidak ada test yang dijalankan (Go maupun Playwright). Status "Sudah" berdasarkan keberadaan test dan kode.
- Tidak membaca detail `create()` untuk setiap validasi (hanya bagian yang relevan).
- Tidak menelusuri `form_submit.go` sampai level setiap field (string kosong dan false pada form).
- Tidak memeriksa OpenAPI untuk detail skema respons Blueprint selain enum tipe.
- Tidak memeriksa data produksi atau isi tabel `blueprints` yang sebenarnya.

---

## 11. Status final: B0 verifikasi keamanan dan B1 hardening

Tanggal: 18 Nov 2026. Basis: bagian 1 sampai 10 di atas. Perubahan kode hanya di `server/internal/items/blueprints.go` dan dua file test baru. Tidak ada migrasi, tidak ada commit, tidak ada PR. Perubahan WIP lain di repo tidak disentuh.

### 11.1 Bukti sebelum dan sesudah

Sebelum perubahan, test reproduksi dijalankan terhadap kode lama (database lokal `pms_test`). Semua tes reproduksi gagal sesuai dugaan.

| Temuan | Reproduksi (sebelum) | Sesudah | Test |
|---|---|---|---|
| X1 copy dari Space lain | Simpan item dari Source ke Library: **201**, blueprint berisi deskripsi item Source | **422**, tidak ada blueprint tersimpan | `TestBlueprintSaveRejectsItemsOutsideTargetSpace` |
| X2 restricted field | Nilai Salary 5000 dan id field **masuk ke tree** saat simpan oleh pengguna tanpa grant; blueprint lama yang memuat nilai itu **dipulihkan** untuk pemilik grant | Nilai dan id field **tidak masuk** tree; nilai lama **dilewati** saat pakai | `TestBlueprintNeverCopiesRestrictedFields` |
| X3 silent truncation | Hierarki 32 level disimpan dengan **31 item** tanpa error | **422** "at most 30 levels below its root item" | `TestBlueprintRefusesHierarchyBeyondDepthLimit`, `TestBlueprintAcceptsExactlyThirtyLevels` (batas 30 tetap diterima) |
| Tree tidak valid | Parent index 5 pada tree 2 node: **panic** (index out of range), respons **500**, tidak ada item tertinggal | **422** "the blueprint hierarchy is invalid", tidak ada item dibuat | `TestBlueprintRefusesInvalidTreeWithoutMutation` |
| Audit | Tidak ada event `blueprint.*` sama sekali | Event `blueprint.save`, `blueprint.use`, `blueprint.delete` ditulis dalam transaksi yang sama | `TestBlueprintWritesAuditEvents` |
| finish_date | Dikirim: **400** "invalid request body" (decoder menolak field tak dikenal). Tidak bisa dipakai untuk reschedule dari finish. | Diterima. Item terakhir selesai pada `finish_date`. `start_date` + `finish_date` bersamaan: **422** | `TestBlueprintUseAnchorsOnFinishDate` |
| Item dengan share tersendiri | Tidak ditolak (konten bisa sampai ke orang di luar Space) | **422** "items with their own shares cannot be saved as a blueprint" | `TestBlueprintRefusesItemsWithTheirOwnShares` |

Koreksi atas analisa sebelumnya: finish_date tidak "diabaikan" secara diam-diam. Decoder yang ketat menolaknya dengan 400. Akibatnya sama (tidak ada reschedule dari finish), tetapi pesan yang dilihat klien berbeda.

Hasil regresi (setelah perubahan):
- `TestBlueprint*` (9 tes, termasuk 6 tes baru dan `TestBlueprintSaveAndUse` yang sudah ada), `TestM3AuthorizationMatrix`, `TestFormBranchingBlueprintItem`: **PASS** (diuji ulang setelah aturan share ditambahkan).
- `go test ./internal/...`: dijalankan di latar belakang. Hasil paket non-httpserver: PASS. Hasil akhir paket `httpserver` dilaporkan di bagian 11.5.
- Tes web (Playwright `blueprints.spec.ts`, `form-blueprint-live.spec.ts`) **tidak dijalankan**; tidak ada perubahan web di sesi ini.

### 11.2 Perubahan kode

`server/internal/items/blueprints.go`:
- **Batas kedalaman eksplisit:** konstanta `maxBlueprintDepth = 30`. CTE sekarang membaca sampai kedalaman 31 dan kedalaman lebih dari 30 menghasilkan 422. Tidak ada lagi pemotongan diam-diam.
- **Simpan hanya dari Space tujuan (X1):** setiap item dalam tree harus punya lokasi di Space tempat blueprint disimpan. Jika tidak, 422.
- **Item dengan share tersendiri ditolak:** `shares` dengan `target_type='item'` pada item yang akan disalin menghasilkan 422.
- **Restricted field (X2):** `restrictedFieldIDs` membaca semua field yang punya baris di `custom_field_grants`. Nilainya dibuang saat simpan dan dilewati saat pakai (termasuk tree lama).
- **Validasi tree sebelum tulis (`validateBlueprintTree`):** jumlah node 1 sampai 200, parent harus mendahului anak, dependency harus mengacu ke indeks yang ada dan berbeda, kedalaman maksimal 30. Dipanggil setelah pengecekan izin dan sebelum ada baris yang dibuat.
- **finish_date:** field baru `FinishDate *string` pada `BlueprintUse`. Anchor dihitung dari offset terakhir tree. Tidak ada perubahan pada jalur `start_date`.
- **Audit (`auditBlueprint`):** insert ke `audit_events` dalam transaksi yang sama. `details` hanya berisi `space_id` dan `item_count`, tanpa isi item.

File baru:
- `server/internal/httpserver/blueprint_hardening_integration_test.go` (6 tes)
- `server/internal/httpserver/blueprint_boundary_integration_test.go` (2 tes)

### 11.3 Perubahan schema dan API

- **Schema:** tidak ada migrasi. Tree yang sudah ada tetap bisa dibaca. Restricted value di tree lama tidak dihapus, hanya diabaikan saat pakai.
- **API (aditif):** field request baru `finish_date` pada `POST /api/v1/blueprints/{id}/items`. Field lama tidak berubah. `openapi.yaml` **belum diperbarui** dan `internal/apigen` belum di-generate ulang. Ini perlu dikerjakan sebelum UI memakai `finish_date`.
- **API (perilaku baru, bukan bentuk baru):** respons 422 baru untuk item di luar Space, item dengan share sendiri, tree terlalu dalam, dan tree tidak valid. Sebelumnya 201 atau 500.
- **Tidak ada perubahan** pada `my_capabilities` atau enum permission.

### 11.4 Status setiap butir acceptance dan permintaan

| Butir | Status | Catatan |
|---|---|---|
| 1. Reproduksi X1 dan X2 dengan dua Space | **Selesai** | Bagian 11.1. Reproduksi memakai Source, Library, dan Target dengan akses berbeda. |
| 2. Audit elevated actor X10 | **Sebagian (keputusan diperlukan)** | Lihat 11.6 R1. Tidak diubah. |
| 3. Permission Save, Preview, Use, Copy | **Sebagian** | Save: Edit di Space tujuan (sudah ada, diuji). Use: View di Space blueprint plus Create per lokasi (sudah ada). Copy sama dengan Use. **Preview tidak ada** di API, jadi tidak ada yang bisa diperbaiki. Preview harus memakai filter `restrictedFieldIDs` dan izin View saat dibuat nanti. |
| 4. Validasi depth dan node limit eksplisit | **Selesai** | Depth 30 dan node 200 dengan error eksplisit. |
| 5. Audit Save, Use, Archive/Delete | **Sebagian** | Save, Use, Delete sudah. **Archive belum ada** (tidak ada lifecycle). |
| 6. finish_date | **Selesai** | Bagian 11.1. UI belum mengirim `finish_date` (lihat 11.6 R5). |
| 7. Desain scope workspace dan space, Personal Space | **Desain saja** | Lihat 11.7. |
| 8. Permission `manage_blueprint` terpisah dari `use_blueprint` | **Desain saja** | Lihat 11.7. Tidak diimplementasi karena mengubah daftar capability dan kontrak `my_capabilities`. |
| Pengguna tanpa akses tidak bisa baca atau duplikasi konten terlarang | **Sebagian** | Item dari Space lain tidak bisa disimpan (X1). Item dengan share sendiri tidak bisa disimpan. Sisa risiko: lihat R2 dan R3. |
| Restricted field tidak bocor lewat payload, preview, hasil clone | **Selesai untuk payload dan clone** | Tidak ada preview. |
| Elevated actor tidak menyebabkan privilege escalation | **Belum (NO-GO)** | Lihat R1. |
| Hierarki invalid ditolak tanpa partial mutation | **Selesai** | Validasi sebelum tulis, transaksi tunggal, test `RefusesInvalidTree`. |
| Seluruh perubahan tercakup regression test | **Selesai untuk kode yang diubah** | Lihat 11.1. Tidak ada tes untuk UI. |
| Blueprint lama tetap bisa dibaca (migrasi kompatibel) | **Selesai** | Tidak ada migrasi. Tree lama dibaca dan dipakai. |
| BP-T13 dan BP-T14 punya automated test | **Tidak bisa dipastikan** | Teks BP-T13 dan BP-T14 tidak ada di konteks saya (sudah ditanyakan di bagian 9). Tes yang ada menutup risiko X1 sampai X4, tetapi pemetaan ke BP-T13 dan BP-T14 belum bisa diverifikasi. |
| Setiap temuan dicatat dengan bukti sebelum dan sesudah | **Selesai** | Bagian 11.1. |

### 11.5 Hasil test akhir

- Tes Blueprint dan otorisasi: **PASS** (11 tes terkait, dijalankan dengan `-v`).
- `go test ./internal/...` (database `pms_test`, `-count=1`): **semua paket PASS, EXIT=0**, termasuk `httpserver` (134,7 detik) dan `items`. Ini run setelah aturan share ditambahkan. Catatan: baseline penuh sebelum perubahan tidak dijalankan, jadi regresi hanya dibandingkan terhadap kondisi PASS, bukan delta.

### 11.6 Risiko tersisa

- **R1 (tinggi, NO-GO): elevated actor pada jalur form.** `form_submit.go` menjalankan pembuatan (termasuk pakai blueprint) dengan `policy.Actor{ID: submitter, Type: Admin}`. Admin mendapat capability penuh di setiap Space shared (`policy/engine.go`). Yang dibatasi: lokasi root dan anak tetap Space form (diverifikasi di `useBlueprint`), restricted field tetap dicek per ID pengguna (`policy/field_access.go`), dan item tidak bisa dibuat di Space lain. Yang tidak dibatasi: pengirim form publik mendapat hak Admin saat pembuatan di Space form. Perbaikan yang layak: aktor dengan hak yang sama dengan grant form (bukan Admin). Perubahan ini mengubah perilaku form publik dan butuh keputusan Anda (pertanyaan 9).
- **R2 (sedang): blueprint lama bisa sudah memuat konten dari Space lain.** Blueprint lama tidak menyimpan asal Space, jadi tidak bisa diaudit otomatis. Tidak dihapus karena guardrail. Perlu keputusan: review manual, atau hapus dengan persetujuan.
- **R3 (sedang): restricted value lama masih ada di tabel.** Tidak dipakai saat pakai, tetapi masih tersimpan. Pembersihan perlu persetujuan (menghapus data).
- **R4 (sedang): item dengan share sendiri ditolak.** Ini trade-off: pengguna yang sering membagikan project harus mencabut share dulu sebelum menyimpan blueprint. Alternatif yang lebih presisi (memeriksa setiap principal terhadap Space) belum dibuat.
- **R5 (rendah-sedang): UI belum memakai finish_date.** Dialog Create masih mengabaikan `due_date` pada jalur blueprint (X5). Belum diperbaiki di web.
- **R6 (rendah): retry serialization.** Transaksi `Serializable` tetap tanpa retry untuk SQLSTATE 40001 (BP-16).
- **R7 (rendah): OpenAPI belum sinkron.** `finish_date` belum ada di `openapi.yaml`.
- **R8 (rendah): zona waktu dan DST** pada perhitungan offset (X8). Belum diubah.
- **R9: audit tidak punya IP dan tidak punya `actor_name` saat aktor elevated** (sama dengan event lain; `actor_name` diisi dari email pengguna).

### 11.7 Desain yang diusulkan (belum diimplementasi)

- **Scope:** `workspace` = seluruh instalasi (tidak ada tabel workspace di repo; perlu konfirmasi). `space` = blueprint milik satu Space. Blueprint di Personal Space diizinkan hanya untuk pemilik, sesuai aturan `spaceCaps` yang sudah ada. Usulan: kolom `scope` (`space` | `workspace`) ditambahkan saat B1 dengan migrasi aditif, default `space`.
- **manage_blueprint vs use_blueprint:**
  - `manage_blueprint`: membuat, mengedit, mengarsipkan, dan menghapus. Diberikan ke workspace admin dan space admin.
  - `use_blueprint`: membuat item dari blueprint (sama dengan View plus Create sekarang).
  - Implementasi memerlukan capability baru yang ditambahkan di akhir `AllCapabilities` (append-only, `policy/policy.go:27-28`). Ini mengubah `my_capabilities` dan preset role. Karena itu tidak dikerjakan sekarang.
- **Preview:** harus memakai jalur yang sama dengan Save (filter `restrictedFieldIDs`, izin View) dan tidak boleh menyalin field restricted.

### 11.8 Rekomendasi GO/NO-GO untuk Blueprint Editor

**NO-GO** untuk memulai Blueprint Editor sekarang. Alasan:
1. R1 belum diputuskan. Editor akan menambah banyak cara untuk mengubah tree, dan jalur form publik masih memakai Admin.
2. Desain `manage_blueprint` dan scope belum disetujui, padahal editor harus tahu siapa yang boleh mengedit.
3. Teks BP-T13 dan BP-T14 belum ada, jadi acceptance terkait belum bisa dipastikan.

Kondisi untuk GO:
- Keputusan R1 (elevated actor) diambil dan diimplementasi, dengan test form publik.
- Keputusan desain scope dan `manage_blueprint` disetujui.
- OpenAPI diperbarui dan di-generate.
- Keputusan R2 dan R3 (data lama) diambil.

Editor boleh dikerjakan paralel dengan keputusan R1 jika hanya menyentuh UI read-only (lihat isi tree melalui endpoint baru yang memakai filter yang sama). Tapi endpoint itu sendiri belum ada.

### 11.9 Pertanyaan terbuka tambahan

1. Setujukah Anda dengan aturan baru: item dari Space lain dan item dengan share sendiri tidak bisa disimpan sebagai blueprint?
2. Untuk R1: apakah pengirim form publik boleh membuat item di Space form dengan hak yang sama seperti grant form, bukan Admin?
3. Untuk R2 dan R3: apakah boleh dilakukan review data lama, atau pembersihan dengan persetujuan tertulis?
4. Apakah `workspace` berarti seluruh instalasi?
5. Apakah teks BP-T13 dan BP-T14 bisa dikirim?

---

## 12. Batch A (R1), Batch C, dan keputusan GO/NO-GO

Tanggal: 18 Nov 2026. Basis: bagian 11.

### 12.1 Cara verifikasi (penting)

Repo dipakai bersama beberapa sesi. Saat batch ini dikerjakan, sesi lain sedang mengubah file yang sama di working tree (`items/field_assignments.go`, `httpserver/custom_field_assignments.go`, dan lain-lain) dan membuat commit baru. Akibatnya, run test di working tree utama gagal dengan error pemindaian yang tidak terkait Blueprint.

Untuk verifikasi, saya memakai **worktree terpisah di commit HEAD (`27c7763`)** dan hanya menyalin file yang saya ubah. Hasilnya adalah "HEAD + perubahan saya", bukan working tree utama. Saya tidak mengubah file sesi lain.

Penyebab kegagalan yang sempat muncul di working tree: `policy.Actor` saya tambahi field `Form`, lalu beberapa `pgx.RowToStructByPos[policy.Actor]` (4 tempat: `approvals.go`, `forms.go`, `approval_deadlines.go`, `notifications/record.go`) gagal karena jumlah kolom tidak cocok. Ini kesalahan saya sendiri dan sudah diperbaiki: semua tempat sekarang memakai `policy.CollectActors(rows)`.

### 12.2 Batch A: R1 (elevated actor pada form publik)

**Sebelum:** `form_submit.go` membuat item dengan `policy.Actor{ID: submitter, Type: policy.Admin}`. Admin mendapat semua capability di setiap Space shared (`policy/engine.go` `spaceCaps`).

**Sesudah:**
- `policy.FormActor(actor, formID, spaceID)` membuat aktor dengan identitas dan tipe pengirim sendiri, plus `Form *FormGrant{FormID, SpaceID}`.
- Di Space form, aktor ini hanya punya `formCaps = {View, Comment, Attach, Create, Edit}`. Di Space lain tidak ada akses (`visibleSpaces`, `formItemsCaps`). Tidak ada Share, Delete, atau tipe Admin.
- Jalur submit (member dan publik) memakai `FormActor`. Tidak ada `policy.Admin` tersisa di `form_submit.go` (hasil grep: 0).
- Audit: `form.submit` ditulis dalam transaksi yang sama. `actor_id` adalah pengirim (atau placeholder anonim `…e1` untuk publik), `details` berisi `form_id`, `via` (member atau public), `destination_space_id`, `submission_id`, dan `visitor_email` untuk publik. Jawaban tidak dicatat.

**Mengapa Edit ada di formCaps:** approval (`approvals.go` memakai `approvalCaps … policy.Edit`) dan nilai field yang dipetakan (`SetCustomFieldValuesTx`) membutuhkannya. Batasannya: hak ini hanya berlaku di Space form, dan hanya selama permintaan submit (aktor tidak disimpan).

**Test negatif dan positif** (`form_submission_scope_integration_test.go`, 4 tes):
- `TestMemberSubmissionCannotChooseAnotherDestination`: kunci tambahan (`space_id`, `locations`, `type_key`, `parent_item_id`, `owner_id`) tidak memindahkan item ke Space lain.
- `TestMemberSubmissionGivesRequesterNoRightsInDestination`: setelah submit, pengirim tetap 404 untuk daftar item di Space tujuan dan Space lain, tidak bisa membuat item langsung, dan tidak mendapat share.
- `TestSubmissionAuditNamesTheActor`: audit member dan publik mencatat identitas yang benar.
- `TestPublicSubmissionCannotChooseAnotherDestination`: submit anonim tetap berhasil; permintaan yang dimanipulasi tidak masuk ke Space lain.

**Before/after:** tes negatif ini juga dijalankan terhadap kode HEAD lama (jalur Admin). Tiga tes lulus di kedua versi karena API tidak pernah membuka celah itu secara langsung, jadi tes ini adalah penjaga regresi. Tes audit gagal di versi lama (tidak ada event `form.submit`) dan lulus di versi baru. Perubahan utama adalah penghapusan tipe Admin dari konteks submit, yang bisa diverifikasi dari kode.

**Sisa risiko R1:** `formCaps` memberi Edit ke seluruh item di Space form selama satu permintaan submit, bukan hanya item yang baru dibuat. Ini aman selama ID item hanya berasal dari data server, yang berlaku sekarang. Kendali yang lebih ketat (hanya item yang dibuat dalam transaksi ini) belum dibuat.

### 12.3 Batch C: kontrak dan UI

- **OpenAPI:** `finish_date` ditambahkan di `api/openapi.yaml` pada `POST /blueprints/{blueprintId}/items` sebagai field tambahan (aditif). Deskripsi `start_date` dan `finish_date` menyebut bahwa keduanya tidak boleh dipakai bersamaan.
- **Regenerasi:** `go tool oapi-codegen` (menghasilkan `server/internal/apigen/openapi.gen.go`, menambah `FinishDate`) dan `npm run generate` (menghasilkan `web/src/api/schema.d.ts`). Keduanya berhasil.
- **Create dialog:** jalur blueprint sekarang mengirim `start_date` jika diisi, atau `finish_date` (dari tanggal selesai) jika hanya itu yang diisi. Jika keduanya diisi, dialog menampilkan pesan `blueprints.anchorOne` dan tidak mengirim permintaan. Teks `hint` diperbarui di kedua bahasa. `tsc --noEmit` lulus.
- **Preview dan reschedule:** belum ada UI preview. Satu-satunya perhitungan jadwal blueprint adalah endpoint pakai (`useBlueprint`), sehingga semantik tanggal sudah satu sumber. Saat preview dibuat, harus memakai endpoint atau fungsi yang sama.
- **Pemetaan BP-T13 dan BP-T14:** teks kedua skenario tetap tidak ada di konteks saya, jadi pemetaan belum bisa dipastikan.

### 12.4 Batch B: scope dan permission (tidak diimplementasi)

Tidak dikerjakan dalam batch ini. Alasannya:
- Menambah capability `use/create/manage` mengubah daftar capability (append-only di `policy.go`) dan field `my_capabilities` pada API. Ini perubahan kontrak yang butuh analisis kompatibilitas tersendiri.
- Scope `account`/`space` dengan backfill butuh migrasi pada tabel blueprint yang dipakai bersama. Sebelum itu, perlu keputusan apa arti `workspace` (pertanyaan 4 bagian 11.9).
- Sesi lain sedang mengubah area permission dan custom field, sehingga risiko tabrakan tinggi.

Rencana yang disarankan: (1) jawab pertanyaan scope, (2) tambahkan capability baru di akhir daftar, dengan peta peran Editor dan Full sama seperti sekarang, supaya perilaku tidak berubah, (3) backfill blueprint existing sebagai `space`, (4) baru ubah `my_capabilities`.

### 12.5 Hasil regresi

- `go test ./internal/...` pada HEAD + perubahan saya: **semua paket PASS, EXIT=0** (termasuk `httpserver` 118,8 detik, `items`, `policy`, `notifications`).
- Tes Blueprint dan otorisasi: PASS (`TestBlueprint*`, `TestM3AuthorizationMatrix`, `TestFormBranchingBlueprintItem`, dan tes form publik `TestPublicFormLink`, `TestFormSubmission`).
- Tes web (Playwright) belum dijalankan; `tsc --noEmit` lulus.

### 12.6 Keputusan GO/NO-GO untuk Blueprint Editor

**NO-GO (tetap).** Alasan berdasarkan bukti:
1. **Batch A selesai secara kode dan teruji**, tetapi tes publik hanya memverifikasi perilaku yang sudah ada, bukan perilaku baru yang tidak diminta. Sisa R1 (Edit untuk seluruh item di Space form selama submit) perlu diterima secara eksplisit.
2. **Batch B belum dikerjakan.** Desain `manage_blueprint`, `use_blueprint`, dan scope belum disetujui dan belum diimplementasi. Editor harus tahu siapa yang boleh mengedit.
3. **Blueprint lama** mungkin masih memuat konten dari Space lain atau nilai restricted (R2, R3 di bagian 11.6). Belum ada keputusan.
4. **BP-T13 dan BP-T14** belum bisa dipetakan karena teksnya tidak tersedia.
5. **Working tree bersama** sedang tidak stabil (kegagalan dari sesi lain). Editor sebaiknya dimulai setelah perubahan lain di-commit dan run penuh di working tree utama lulus.

Syarat GO: Batch B disetujui dan dikerjakan (minimal scope dan capability), keputusan sisa R1 diterima, keputusan R2/R3 diambil, teks BP-T13/BP-T14 tersedia, dan run penuh di working tree utama hijau.

### 12.7 Pertanyaan terbuka

1. Apakah Edit untuk seluruh item di Space form selama submit (sisa R1) dapat diterima, atau perlu dibatasi ke item yang dibuat transaksi itu?
2. Apa arti `workspace` (instalasi, atau sekumpulan Space)?
3. Apakah boleh menambah capability `use_blueprint` dan `manage_blueprint` dengan peta peran yang sama seperti sekarang (Editor dan Full)?
4. Apakah blueprint lama dengan konten lintas Space perlu direview atau dihapus (dengan persetujuan)?
5. Mohon teks BP-T13 dan BP-T14.

---

## 13. Batch B: scope dan capability Blueprint

Tanggal: 18 Nov 2026. Keputusan user: R1 (sisa Edit pada Space form selama submit) diterima. Batch B dikerjakan.

### 13.1 Yang diubah

**Capability (kontrak).** Tiga kemampuan baru, ditambahkan di akhir urutan bit (aturan append-only untuk `caps_mask`):
- `use_blueprint` (bit 8): membuat item dari blueprint.
- `create_blueprint` (bit 9): menyimpan blueprint baru.
- `manage_blueprint` (bit 10): menghapus blueprint (tempat untuk edit nanti).

**Peta peran bawaan** (tidak ada hak yang berubah):
- Editor: `use`, `create`, `manage` (sama seperti sebelumnya: simpan dan hapus memakai Edit, pakai memakai View + Create).
- Full, Admin, Regular: semua.
- Limited, Read, Viewer: tidak mendapat hak blueprint baru.
- External (plafon Editor): ikut Editor.

**Migrasi `server/migrations/00333_blueprint_scope.sql`:**
- `blueprints.scope` (`space` | `account`), default `space`. Baris yang berada di Personal Space di-backfill menjadi `account`. Baris lain tetap `space`. Scope mengikuti jenis Space: `account` = Personal Space pemilik (hanya pemilik yang bisa melihat, sama seperti aturan Space pribadi yang sudah ada).
- `custom_roles.caps_mask`: batas CHECK dinaikkan dari 8 ke 11 bit. Backfill: role yang punya `create` mendapat `use_blueprint`; role yang punya `edit` mendapat `create_blueprint` dan `manage_blueprint`. Dengan begitu hak efektif role custom yang sudah ada tidak berubah.
- Down migration tersedia (membuang bit baru dan mengembalikan batas 255).

**Penegakan di backend (`items/blueprints.go`, `httpserver/forms.go`):**
- Simpan blueprint: `create_blueprint` di Space tujuan (sebelumnya Edit).
- Hapus blueprint: `manage_blueprint` (sebelumnya Edit).
- Pakai blueprint: `use_blueprint` di Space blueprint, ditambah Create di setiap lokasi tujuan (sebelumnya View).
- Daftar blueprint: tetap View.
- Rute form ke blueprint: penulis form harus punya `use_blueprint` di Space tujuan (ditolak 422 jika tidak). Submit form memakai grant form yang sekarang juga mencakup `use_blueprint` di Space form (satu-satunya tempat blueprint form diterapkan).
- Respons blueprint memuat `scope` (tambahan, aditif).

**Kontrak API:**
- `Capability` enum ditambah 3 nilai. Batas `capabilities` pada custom role dari 8 menjadi 11 (`api/openapi.yaml`, `apigen`, `web/src/api/schema.d.ts` diregenerasi).
- `Blueprint.scope` ditambahkan (aditif). `finish_date` tetap seperti sebelumnya.
- `my_capabilities` mendapat tiga nilai tambahan. Klien yang hanya membaca nilai yang dikenal tidak terpengaruh.

**Web:** daftar kemampuan (`lib/roles.ts`), label dan bantuan di kedua bahasa, katalog Blueprint memakai `create_blueprint` untuk tombol buat dan `use_blueprint` untuk tombol pakai, dialog simpan memakai `create_blueprint`, serta mock Playwright (`tests/spaces-mock.ts`).

**Dokumen:** `docs/ARCHITECTURE.md` §6.1 dan snapshot kemampuan di tes diperbarui hanya dengan tiga kemampuan baru.

### 13.2 Bukti

Verifikasi dilakukan pada worktree bersih di HEAD (`8d685be`) dengan file saya, karena working tree bersama masih berubah oleh sesi lain.

- `go test ./internal/...`: **semua 20 paket PASS, EXIT=0** (setelah memperbarui snapshot kemampuan di `spaces_integration_test.go` dan `policy_test.go`/`custom_roles_test.go`).
- Tes baru `blueprint_rights_integration_test.go`:
  - `TestBlueprintRightsAreSeparate`: pemegang hanya `use` bisa pakai tapi tidak bisa simpan atau hapus. Pemegang hanya `create` bisa simpan tapi tidak bisa pakai atau hapus. Pemegang hanya `manage` bisa hapus tapi tidak bisa simpan. Pembaca tetap ditolak.
  - `TestEditorPresetKeepsBlueprintRights`: Editor bawaan tetap bisa simpan, pakai, dan hapus.
  - `TestBlueprintScopeFollowsTheSpaceKind`: blueprint di Space bersama bernilai `space`, di Personal Space bernilai `account`.
  - `TestFormRoutingNeedsTheUseRightAndSubmissionAppliesIt`: penulis tanpa `use` ditolak (422). Penulis dengan `use` diizinkan. Pengirim tanpa hak sama sekali mendapat blueprint diterapkan di Space form.
- Backfill role custom diuji dengan mengulang persis UPDATE migrasi pada contoh mask di transaksi yang di-rollback: `create` → `use`; `edit` → `create` dan `manage`; `view` saja tidak berubah. Hasilnya sesuai hak lama.
- Migrasi diterapkan oleh `testdb` pada setiap tes, sehingga nama constraint `custom_roles_caps_mask_check` sudah terverifikasi.
- `tsc --noEmit` untuk web lulus. Tes Playwright belum dijalankan.

### 13.3 Yang belum (dan alasannya)

- **Edit blueprint** belum ada (tidak ada endpoint edit). `manage_blueprint` saat ini hanya dipakai untuk hapus, dan akan dipakai untuk edit saat Editor dibuat.
- **Scope `account` sebagai kepemilikan terpisah** belum dibuat. `account` sekarang berarti "Personal Space pemilik", sehingga tidak ada kolom owner baru. Jika Anda ingin blueprint akun lintas Space, itu perubahan skema yang perlu keputusan sendiri.
- **Pratinjau** belum ada (tidak ada endpoint maupun UI).
- **Nomor migrasi 00333** dipakai sekarang. Jika sesi lain juga membuat migrasi 00333, nomornya harus digeser sebelum merge.
- **Tes Playwright** belum dijalankan (katalog dan dialog memakai nama kemampuan baru).
- **Pembatasan Edit lintas item (sisa R1)** diterima sesuai keputusan Anda.

### 13.4 Risiko tersisa

- **Role custom yang sudah ada** mendapat bit baru dari migrasi. Jika ada role yang dibuat setelah backfill tanpa bit baru, role itu tidak bisa memakai blueprint sampai diedit. Ini perilaku yang diharapkan, tetapi perlu diumumkan ke admin.
- **Mock dan UI web** bergantung pada nama kemampuan baru. Jika `my_capabilities` dari server tidak memuat nama itu (misalnya server lama), tombol simpan dan pakai blueprint tidak tampil. Ini aman (tertutup), bukan terbuka.
- **Data lama**: blueprint yang sudah ada dan memuat konten lintas Space atau nilai restricted (R2, R3 pada bagian 11.6) belum diputuskan.

### 13.5 GO/NO-GO untuk Blueprint Editor

**Status: masih NO-GO**, tetapi sekarang tinggal beberapa syarat:
1. Keputusan R2 dan R3 (blueprint lama dengan konten lintas Space atau nilai restricted) diambil.
2. Endpoint edit dan `manage_blueprint` untuk edit disepakati dan dibuat (Editor baru bisa dikerjakan setelah ini).
3. Tes Playwright katalog dan dialog dijalankan dan hijau.
4. Nomor migrasi 00333 dikonfirmasi tidak bertabrakan dengan sesi lain.
5. BP-T13 dan BP-T14 tersedia teksnya untuk pemetaan.

Jika 1, 3, dan 4 selesai, Blueprint Editor bisa dimulai.

---

## 14. Edit blueprint, pratinjau, dan scope account

Tanggal: 18 Nov 2026. Lanjutan bagian 13.

### 14.1 Keputusan scope `account`

Scope `account` **memakai Personal Space pemilik**, bukan kepemilikan terpisah. Tidak ada kolom owner baru. Blueprint `account` hanya bisa dilihat dan diubah oleh pemilik Personal Space, sesuai aturan Space pribadi yang sudah ada. Implementasi tetap seperti bagian 13 (`blueprints.scope` mengikuti jenis Space).

### 14.2 Edit blueprint (`PATCH /api/v1/blueprints/{id}`)

- **Hak:** `manage_blueprint` di Space blueprint. Reader dan pemegang hanya `use_blueprint` ditolak (403). Pengguna di luar Space mendapat 404.
- **Isi:** `version` (wajib), `name` (opsional, 1 sampai 120 karakter), dan/atau `tree` (opsional, mengganti seluruh struktur). Tanpa `name` maupun `tree`, 422.
- **Konflik:** `version` harus sama dengan versi yang dibaca. Jika tidak, **409**. Setiap perubahan menaikkan `version`.
- **Validasi tree:** sama dengan saat simpan (urutan induk, indeks dependency, batas 200 item dan 30 level), ditambah: tipe item task, project, atau folder; judul 1 sampai 500 karakter; importance `high`, `normal`, atau `low`; selesai tidak sebelum mulai; durasi dan effort tidak negatif; tipe dependency FS, SS, FF, atau SF. Tree yang tidak valid ditolak sebelum ada yang ditulis.
- **Field restricted** dibuang dari tree, seperti saat simpan.
- **Audit:** `blueprint.edit` dengan `changed` (`name`, `tree`). Tidak ada isi tree di audit.
- **Transaksi:** satu transaksi dengan kunci baris (`FOR UPDATE`).

Migrasi `00334_blueprint_version.sql`: `blueprints.version` (default 1, bertambah tiap edit) dan `blueprints.updated_at`. Baris lama mulai dari versi 1. Down migration tersedia.

### 14.3 Pratinjau (`POST /api/v1/blueprints/{id}/preview`)

- **Hak:** `use_blueprint` di Space blueprint. Saya tidak memberi hak tambahan kepada pemegang `manage` saja: mereka ditolak (403). Pembaca ditolak (403). Di luar Space: 404.
- **Tidak menulis apa pun.** Hasilnya daftar item dengan `title`, `type`, `parent` (indeks), `start_date`, dan `due_date`.
- **Semantik sama dengan pembuatan:** kedua jalur memakai `blueprintAnchor` dan `scheduled` yang sama (dipindah dari jalur pembuatan, tidak diduplikasi). Tes memastikan tanggal pratinjau sama dengan tanggal item yang dibuat setelahnya.
- Pratinjau **tidak** menampilkan field restricted (sudah dibuang saat simpan dan edit).

### 14.4 Kontrak API

- Ditambahkan (aditif): `PATCH /blueprints/{id}`, `POST /blueprints/{id}/preview`, skema `BlueprintTree`, `BlueprintPreviewItem`, dan field `version`, `updated_at` pada `Blueprint`.
- `api/openapi.yaml` diperbarui. `apigen` (Go) dan `schema.d.ts` (web) diregenerasi.
- Tidak ada perubahan pada endpoint yang sudah ada selain tambahan field respons.

### 14.5 Bukti

- Tes baru `blueprint_edit_preview_integration_test.go` (5 tes):
  - `TestBlueprintEditNeedsManageAndTheCurrentVersion`: hak, versi wajib (422), konflik (409), audit `blueprint.edit`.
  - `TestBlueprintEditRefusesInvalidTrees`: 7 tree tidak valid ditolak (422) dan tree tersimpan tidak berubah.
  - `TestBlueprintEditDropsRestrictedFields`: nilai restricted tidak tersimpan lewat edit.
  - `TestBlueprintPreviewMatchesUseAndWritesNothing`: hak pratinjau, tidak ada item tertulis, dan tanggal pratinjau = tanggal yang dibuat.
  - `TestBlueprintListReportsVersion`: daftar memuat `version` dan `scope`.
- Semua tes blueprint (16) PASS.
- `go test ./internal/...` pada worktree bersih (HEAD + perubahan saya): **20 paket PASS, EXIT=0**.
- `tsc --noEmit` web: lulus.

### 14.6 Yang belum

- **UI pratinjau dan UI edit** belum dibuat. Yang tersedia sekarang adalah API. Ini sesuai dengan batas "jangan mulai Blueprint Editor" dari instruksi sebelumnya.
- **Edit tree dari UI** akan memerlukan endpoint baca satu blueprint (`GET /blueprints/{id}`) agar editor bisa memuat tree. Endpoint itu belum ada, sehingga editor belum bisa mengambil struktur untuk diubah.
- **Versi lama blueprint** (riwayat) tetap P2 dan belum dibuat.
- **Tes Playwright** belum dijalankan.

### 14.7 Risiko tersisa

- **Edit tree menggantikan seluruh struktur.** Tidak ada penggabungan per node. Dua pengguna yang mengedit berurutan dicegah oleh `version`, tetapi editor UI harus memuat tree terbaru sebelum mengirim.
- **Status dan assignee** di tree tidak diperiksa saat edit (hanya saat pakai, seperti sebelumnya). Status yang tidak ada di workflow tujuan dilewati diam-diam saat pakai.
- **Nomor migrasi 00333 dan 00334** harus dicek tidak bertabrakan dengan sesi lain sebelum merge.

### 14.8 GO/NO-GO untuk Blueprint Editor

**Status: masih NO-GO**, dengan syarat yang lebih sedikit dari sebelumnya:
1. Endpoint baca satu blueprint (`GET /blueprints/{id}`) dengan hak `view` ditambahkan. Ini prasyarat editor.
2. Keputusan R2 dan R3 (data lama lintas Space dan nilai restricted) diambil.
3. Tes Playwright katalog, dialog, dan pratinjau dijalankan dan hijau.
4. Nomor migrasi 00333 dan 00334 dikonfirmasi tidak bertabrakan.
5. Teks BP-T13 dan BP-T14 tersedia.

Edit dan pratinjau di API sudah siap untuk dipakai editor setelah poin 1 selesai.

---

## 15. Penutupan: endpoint baca, verifikasi akhir, dan GO/NO-GO

Tanggal: 18 Nov 2026. Lanjutan bagian 14.

### 15.1 Yang ditambahkan

- **`GET /api/v1/blueprints/{id}`**: membaca satu blueprint beserta `tree`. Butuh `view` pada Space (sama seperti daftar). Outsider mendapat 404. Field restricted dibuang saat dibaca, sehingga tree lama yang masih memuatnya tidak membocorkan nilai. Dokumen OpenAPI diperbarui dan kode Go serta TypeScript diregenerasi.
- Tes `blueprint_read_integration_test.go`: pembaca mendapat tree, outsider 404, dan nilai restricted pada tree lama tidak dikembalikan.

### 15.2 Verifikasi

- **Go, blueprint:** semua tes blueprint PASS (termasuk tes baru edit, pratinjau, daftar, dan baca).
- **Go, penuh:** `go test ./internal/...` pada worktree bersih di HEAD `39964dc` dengan perubahan saya: **20 paket PASS, EXIT=0**.
- **Playwright, mock:** `tests/blueprints.spec.ts` PASS. `create-menu`, `spaces`, `forms` (termasuk rute form ke blueprint), dan `form-blueprint-live` (mock): 39 PASS, 1 skipped.
- **Playwright, live:** `TestBrowserFormBranchingBlueprint` (form bercabang → blueprint → item, dengan server dan browser sungguhan) PASS.
- **TypeScript:** `tsc --noEmit` lulus.
- Worktree verifikasi dibuat dan dihapus. Working tree utama tidak diubah oleh verifikasi.

### 15.3 Nomor migrasi

Saat ini `00333_blueprint_scope.sql` dan `00334_blueprint_version.sql` tidak dipakai oleh cabang atau worktree lain (dicek pada semua cabang lokal dan remote). Ini hanya berlaku pada saat pengecekan; jika sesi lain menambah migrasi dengan nomor yang sama sebelum merge, nomornya harus digeser.

### 15.4 Yang masih terbuka (tidak bisa diselesaikan tanpa Anda)

- **R2:** blueprint lama yang mungkin memuat konten dari Space lain. Asal Space tidak tersimpan, jadi tidak bisa diaudit otomatis. Perlu keputusan: review manual, atau hapus dengan persetujuan tertulis.
- **R3:** nilai field restricted lama yang masih tersimpan di tabel (tidak dipakai atau dibaca lagi, tetapi tetap ada). Pembersihan menghapus data, jadi perlu persetujuan.
- **BP-T13 dan BP-T14:** teksnya tidak ada di konteks saya. Tanpa teks itu, pemetaan ke acceptance belum bisa dipastikan.
- **UI editor** belum dibuat. Ini sesuai instruksi awal bahwa Blueprint Editor tidak dimulai dalam batch ini. API untuk baca, ubah, dan pratinjau sudah tersedia.

### 15.5 GO/NO-GO untuk Blueprint Editor

**Status: NO-GO sampai dua hal diputuskan.**

Semua prasyarat teknis sudah terpenuhi: endpoint baca, ubah, dan pratinjau; kemampuan terpisah (`use`, `create`, `manage`); versi dan konflik; scope; verifikasi Go dan Playwright; dan nomor migrasi tidak bertabrakan.

Yang masih menghalangi:
1. Keputusan R2 dan R3 (apakah review atau pembersihan data lama disetujui).
2. Teks BP-T13 dan BP-T14, supaya acceptance dapat dipetakan dan diverifikasi.

Setelah dua hal itu, Blueprint Editor dapat dimulai, dengan UI yang memakai `GET`, `PATCH` (dengan `version`), dan `preview`.

---

## 16. UI Blueprint Editor, pratinjau, dan edit

Tanggal: 18 Nov 2026. Lanjutan bagian 15.

### 16.1 Yang dibuat

- **Halaman editor** `/blueprints/{id}` (file `web/src/routes/_app/blueprints_.$blueprintId.tsx`). Memuat blueprint lewat `GET`, lalu menampilkan:
  - Nama blueprint.
  - Daftar item dalam urutan blueprint: judul, tipe, prioritas, hari mulai, dan hari selesai.
  - Tambah subitem, dan hapus item yang tidak punya anak (akar tidak bisa dihapus). Hapus juga merapikan indeks induk dan dependency.
- **Simpan** memakai `PATCH` dengan `version` yang dibaca editor. Jika versinya sudah berubah (409), editor menampilkan pesan dan tombol muat ulang. Draft lama tidak ditimpa diam-diam.
- **Pratinjau** memilih tanggal mulai atau tanggal selesai, lalu memanggil `POST /preview`. Pratinjau memakai blueprint yang tersimpan, jadi tombolnya nonaktif saat ada perubahan yang belum disimpan.
- **Katalog** `/blueprints` menampilkan tautan "Edit/Ubah" untuk pengguna yang punya `manage_blueprint` di Space tersebut. Tombol "Pakai" tampil hanya jika punya `use_blueprint`.
- **Hak** di UI: editor hanya bisa diubah dengan `manage_blueprint`. Tanpa hak itu, halaman tampil hanya-baca. Pratinjau butuh `use_blueprint`. Backend tetap menjadi penentu akhir.
- **Field dan deskripsi** item yang tidak ditampilkan di editor (misalnya field kustom, status, dan assignee) tetap dikirim apa adanya saat simpan, sehingga tidak hilang. Field restricted dibuang oleh backend seperti sebelumnya.

### 16.2 Verifikasi

- **Playwright (mock)**, `tests/blueprint-editor.spec.ts`: 3 tes, semua PASS.
  - Ubah nama dan judul item, simpan, dan `PATCH` mengirim `version`, `name`, dan `tree` yang benar.
  - Versi basi (409) menampilkan pesan dan tombol muat ulang.
  - Tambah dan hapus subitem, tombol pratinjau nonaktif saat ada perubahan, dan pratinjau tidak memanggil `PATCH`.
- **Playwright (mock)**, `tests/blueprints.spec.ts`: PASS. Regresi `create-menu`, `spaces`, `forms`, dan `form-blueprint-live` (mock): 43 PASS, 1 skipped.
- **Lint**: `lint:tokens` OK. ESLint untuk file yang diubah bersih.
- **TypeScript**: `tsc --noEmit` lulus. `routeTree.gen.ts` diregenerasi lewat `vite build`.

Catatan: halaman editor belum diuji terhadap backend yang berjalan (live). Tes ini memakai mock API, sesuai kebiasaan spec blueprint lainnya. Tidak ada perubahan Go di bagian ini.

### 16.3 Yang belum

- **Dependency** tidak bisa diatur dari editor. Dependency yang sudah ada dipertahankan saat simpan, tetapi belum bisa dibuat atau diubah dari UI.
- **Assignee, status, dan field kustom** belum bisa diubah dari editor.
- **Penyebaran** ke `losaka-app` belum dilakukan. Image `pms-app-1` masih versi lama.
- **E2E terhadap backend live** untuk editor belum dibuat.

### 16.4 GO/NO-GO

Prasyarat teknis editor sudah lengkap: baca, ubah dengan versi, pratinjau, dan UI dasar. Yang masih menunggu keputusan Anda: teks BP-T13 dan BP-T14 untuk pemetaan acceptance, serta penyebaran perubahan ke server.

---

## 17. Deploy ke dev-lens (losaka-app)

Tanggal: 18 Nov 2026.

- **Commit:** `9b3abee` di branch lokal `feat/blueprint-b5-scope-edit`, dibuat dari HEAD `39964dc` dengan hanya file Blueprint. Tidak di-push.
- **Build:** image `pms:9b3abee` dibangun di `losaka-app` dari `~/build-9b3abee` (source hasil `git archive`).
- **Backup pra-deploy:** stamp `20261118T032227Z` (`backup.sh`).
- **Switch:** `PMS_IMAGE=pms:9b3abee` di `/home/lsuser/pms/.env` (salinan lama: `.env.bak-9b3abee-pre`), lalu `docker compose up -d app`.
- **Verifikasi:**
  - Migrasi `00333` dan `00334` tercatat di `schema_migrations` (versi terakhir 334).
  - Kolom `blueprints.scope`, `version`, dan `updated_at` ada.
  - Batas `custom_roles.caps_mask` sudah 2047.
  - `blueprints` 0 baris. `items` 1667, tidak berubah.
  - `healthz` dan `readyz` OK. Container `pms-app-1` healthy. Tidak ada error atau panic di log.
- **Rollback jika diperlukan:** `./restore.sh 20261118T032227Z --force` dari `/home/lsuser/pms`, yang mengembalikan image lama dan data cadangan.

Catatan: kedua perbaikan tipe di web (`version` wajib pada skema `Blueprint`) ditemukan saat build Docker, bukan saat pengecekan lokal. Pengecekan `tsc -b` sekarang sudah termasuk di alur verifikasi.

Belum dilakukan: tes e2e terhadap server live (login dan pembuatan data), dan pembuatan data e2e yang diminta di akhir sesi.

---

## 18. E2E di dev-lens: membuat data baru

Tanggal: 18 Nov 2026. Dijalankan lewat API publik `https://dev-lens.losakaenergi.app/api/v1` dengan akun admin uji dari `docs/qa/usertest.md` (login sukses, sesi ditutup setelahnya; file cookie sementara dihapus).

Data baru yang dibuat (diberi label "E2E" dan tanggal):
- Space uji: `E2E Blueprint 20261118052039` (id `5d02fe5e-9dd5-4544-b901-2a7a03c6b7f7`).
- Project sumber `E2E Launch 20261118052039`, dengan task `Plan` dan `Ship` serta dependency FS.
- Blueprint `E2E kit 20261118052039 (edited)` (id `b2deb260-0a7a-4fdc-90a3-920012b6b7c5`).
- Item hasil pakai `E2E from blueprint 20261118052039` (id `01a1ed19-9feb-71ba-b786-66f9ced012d4`), dengan anak `Plan (edited in e2e)` 22–24 Juni dan `Ship` 29–30 Juni 2026.

Hasil langkah (semua sesuai harapan):
- Simpan dari project: 201, `scope=space`, `version=1`, `item_count=3`.
- Baca blueprint dengan tree: 200, 3 node dan 1 dependency.
- Ubah nama dengan versi terbaru: 200. Versi basi: 409.
- Ubah judul node lewat tree: 200.
- Pratinjau dengan tanggal selesai 30 Juni 2026: 200. Tanggal pratinjau sama dengan tanggal item yang benar-benar dibuat.
- Pakai blueprint dengan tanggal selesai: 201.

Catatan:
- Data uji ini belum dihapus. Hapus Space uji dan item-itemnya jika tidak diperlukan.
- `docs/qa/usertest.md` berisi password dalam teks biasa di repositori. Sebaiknya dipindahkan ke tempat yang aman dan dirotasi.
- Belum dilakukan: tes UI editor di browser terhadap server live.

---

## 19. Tes editor live di dev-lens (browser)

Tanggal: 18 Nov 2026. Tes Playwright dijalankan dari luar repo (`scratchpad/live`) terhadap `https://dev-lens.losakaenergi.app`, login dengan akun uji dari `docs/qa/usertest.md`, dan hanya menyentuh blueprint `E2E kit 20261118052039` milik e2e sebelumnya.

Hasil:
- Login, membuka editor dari katalog: berhasil.
- Simpan dari A (ubah nama): berhasil, versi naik.
- **Temuan (bug):** halaman B yang belum menyimpan apa pun ikut berubah ke data A (nama "(live A)", v4) tanpa peringatan. Penyebabnya: editor dibuat ulang setiap kali data detail dimuat ulang (termasuk refetch saat jendela fokus), sehingga draft B yang belum disimpan bisa hilang diam-diam. Ini juga membuat tombol simpan dan baseline tidak sesuai.
- Akibatnya langkah konflik (B menyimpan dan harus mendapat 409) tidak tercapai pada build yang sedang berjalan. Tes berhenti di sana.

Perbaikan (ada di web, belum di-deploy):
- Editor dibuat ulang hanya saat pengguna menekan Muat ulang setelah konflik.
- Setelah simpan, editor memakai hasil simpan sebagai baseline.
- Query detail tidak refetch saat jendela fokus.
- Verifikasi lokal: `tsc -b` lulus, tes Playwright mock editor 4/4 lulus.

Belum terverifikasi terhadap server live: perbaikan di atas. Perlu deploy ulang untuk memverifikasinya.

Catatan data: nama blueprint uji sekarang `E2E kit 20261118052039 (live A)`, dan node `Plan` berjudul "Plan (edited in e2e)". Tidak ada data lain yang diubah.

---

## 20. Draf teks BP-T13 dan BP-T14 (dibuat oleh asisten, perlu konfirmasi)

Teks asli BP-T13 dan BP-T14 tidak tersedia di repo maupun di konteks. Karena itu dua acceptance di bawah adalah **draf** yang saya susun dari risiko yang sudah dibahas di bagian 11 sampai 15. Anda bisa menggantinya dengan teks resmi kapan saja.

**BP-T13 (draf): perubahan blueprint memakai versi, dan perubahan basi ditolak tanpa mengubah data.**
- Given: blueprint berversi N dibaca dua editor.
- When: editor pertama menyimpan (menjadi N+1), lalu editor kedua menyimpan dengan versi N.
- Then: penyimpanan kedua dijawab 409, tidak ada perubahan yang tersimpan, dan tree yang tidak valid juga ditolak sebelum ada yang ditulis.
- Bukti: `TestBlueprintEditNeedsManageAndTheCurrentVersion`, `TestBlueprintEditRefusesInvalidTrees` (Go); tes live di browser (bagian 19) menguji langkah konflik, tetapi belum lulus pada build yang berjalan karena bug draf hilang. Perbaikannya ada di commit `d949653`.

**BP-T14 (draf): pratinjau dan pemakaian blueprint menghasilkan tanggal yang sama, dan pratinjau tidak menulis apa pun.**
- Given: blueprint dengan item dan dependency.
- When: pratinjau dengan tanggal selesai, lalu pemakaian dengan tanggal yang sama.
- Then: setiap item di pratinjau punya tanggal mulai dan selesai yang sama dengan item yang dibuat, dan jumlah item tidak berubah setelah pratinjau.
- Bukti: `TestBlueprintPreviewMatchesUseAndWritesNothing` (Go, PASS); `E2E` di dev-lens (bagian 18) mencatat hasil yang sama untuk Plan dan Ship.

**Status: draf disetujui oleh user (18 Nov 2026).** Teks resmi dari repo belum tersedia; jika ada teks asli, gantikan draf ini.

---

## 21. Deploy perbaikan editor dan tes live ulang

Tanggal: 18 Nov 2026.

- **Commit:** `d949653` (perbaikan draf editor), lalu `57f4a9f` (dokumen). Keduanya di branch lokal `feat/blueprint-b5-scope-edit`, tidak di-push.
- **Build:** `pms:57f4a9f` di `losaka-app`, tanpa error TypeScript.
- **Backup pra-deploy:** `20261118T061354Z`. Salinan `.env` lama: `.env.bak-57f4a9f-pre`.
- **Verifikasi server:** container `pms-app-1` healthy. Migrasi tercatat sampai 334. `items` 1673 (1667 + 6 item uji e2e). `blueprints` 1 (blueprint uji). `healthz` publik 200.
- **Tes live editor (browser):** PASS.
  - A menyimpan nama baru (versi naik).
  - B, yang memegang versi lama, mendapat pesan konflik saat menyimpan.
  - Pratinjau dengan tanggal selesai menampilkan 3 item.
- Kegagalan sebelumnya (run kedua) disebabkan nama tes yang sama dengan nama yang sudah tersimpan, sehingga tombol simpan nonaktif. Bukan bug aplikasi. Nama tes sekarang unik per run.

Catatan data: blueprint uji sekarang bernama `E2E kit 20261118052039 (A <timestamp>)`. Item uji e2e dari bagian 18 masih ada dan belum dihapus.

Draf BP-T13 dan BP-T14 ada di bagian 20 dan perlu konfirmasi Anda.

---

## 22. Editor: status, assignee, field kustom, dan dependency

Tanggal: 18 Nov 2026.

**Yang ditambahkan di UI** (`web/src/routes/_app/blueprints_.$blueprintId.tsx`):
- Tiap item punya panel "Detail item ini" dengan:
  - **Status:** daftar status dari workflow default.
  - **Assignee:** pemilih user (`ItemUserPicker`, banyak pilihan).
  - **Field kustom:** tipe teks, angka, tanggal, centang, dan pilihan tunggal bisa diubah. Tipe lain (orang, pilihan ganda, mata uang, persen, durasi, relasi, rumus) ditampilkan hanya-baca dan nilainya dipertahankan.
- **Dependency:** daftar dependensi dengan sebelumnya, sesudahnya, tipe (FS, SS, FF, SF), dan jeda. Dependensi yang mengarah ke diri sendiri langsung ditandai dan tombol simpan nonaktif.

**Validasi backend** (`server/internal/items/blueprints.go`, saat edit):
- Dependensi yang sama dua kali ditolak (422).
- Lingkaran dependensi ditolak (422).
- Status yang tidak ada di database ditolak (422).
- Semua ditolak sebelum ada yang ditulis.

**Bukti:**
- Go: `TestBlueprintEditKeepsEditorFieldsAndRefusesBadGraphs` (PASS). Memeriksa lingkaran, duplikat, status tidak dikenal, dan bahwa status, assignee, field, serta dependensi yang valid tersimpan utuh.
- Playwright mock: 4 tes editor PASS, termasuk tes baru untuk status, field, dependensi, dan penolakan dependensi pada diri sendiri.
- `tsc -b` dan ESLint untuk file editor: lulus.

**Batasan yang masih ada:**
- Status ditampilkan dari workflow default. Saat dipakai, status hanya diterapkan jika workflow item tujuan memiliki status tersebut. Jika tidak, status dilewati, seperti sebelumnya.
- Field kustom yang tidak ada di Space tujuan atau yang dibatasi (restricted) dilewati saat dipakai.
- Tes live di browser untuk fitur ini belum dijalankan. Perlu deploy ulang untuk itu.

---

## 23. Settings → Blueprints: registry terpisah dari katalog Workspace

Tanggal: 18 Nov 2026. Tanpa commit, push, atau PR.

### 23.1 Audit routing (sebelum)
- Item "Blueprints" di grup **Work configuration** Settings (`web/src/components/settings-sidebar.tsx`) mengarah ke **`/blueprints`**, yaitu katalog Workspace. Jadi menu Settings membuka halaman yang salah.
- Route `/settings/blueprints` belum ada. Halaman management Blueprint pada Settings belum diimplementasikan.
- Komponen dan API yang bisa dipakai ulang: `GET /spaces/{id}/blueprints` (sudah memuat `scope`, `version`, `updated_at`), `useSpaces`, `can()`, dialog Create blueprint, `DropdownMenu`, `EmptyState`, dan `formatDate`.

### 23.2 Routing (sesudah)
- Settings → Blueprints mengarah ke **`/settings/blueprints`**, di dalam layout Settings (breadcrumb "Settings / Work configuration", lebar penuh).
- Workspace → Blueprints tetap **`/blueprints`** (katalog, tidak diubah).
- Deep link dan reload berfungsi. Back navigation berfungsi.

### 23.3 Perubahan file
- `web/src/routes/_app/settings/blueprints.tsx` (baru): halaman registry.
- `web/src/lib/blueprint-registry.ts` (baru): filter, urutan, dan pengelompokan (fungsi murni).
- `web/src/lib/blueprint-registry.test.ts` (baru): 3 tes unit.
- `web/src/components/blueprint-create-dialog.tsx` (baru): dialog Create blueprint, dipindah dari `routes/_app/blueprints.tsx` tanpa perubahan perilaku.
- `web/src/routes/_app/blueprints.tsx`: memakai komponen dialog di atas. Perilaku katalog tidak berubah.
- `web/src/components/settings-sidebar.tsx`: item Blueprints menunjuk ke `/settings/blueprints`.
- `web/src/routes/_app/settings.tsx`: `/settings/blueprints` masuk daftar halaman lebar.
- `web/src/i18n/{en,id}/blueprints.json`: kunci `registry.*`.
- `web/src/routeTree.gen.ts`: diregenerasi.
- `web/tests/settings-blueprints.spec.ts` (baru): 6 tes Playwright mock.

### 23.4 Isi registry
- **Tampil:** Nama, Tipe dasar (Task, Project, Folder), Cakupan (Space atau Account), Milik (nama Space; Personal Space untuk Account), Jumlah item, Terakhir diubah.
- **Cari, filter (tipe dan cakupan), urut (terbaru, terlama, nama), kelompokkan (cakupan, Space, tipe).**
- **Aksi per baris:** Ubah (aktif hanya dengan hak kelola; membuka editor yang sudah ada), Duplikasi dan Arsipkan (nonaktif dengan penjelasan).
- **Create blueprint:** membuka dialog yang sama dengan katalog (simpan dari item yang sudah ada). Hanya tampil jika ada Space yang memberi hak `create_blueprint`.
- **Empty state:** "Belum ada blueprint" dengan petunjuk.

### 23.5 Yang belum tersedia (ditampilkan sebagai catatan, tidak dimock)
- **Status (Draft/Published/Archived), Author, dan Usage count:** API belum menyediakan. Kolom ini tidak ditampilkan, dan catatan di halaman menjelaskannya.
- **Duplikasi dan arsip:** belum ada endpoint. Menu nonaktif dengan penjelasan. Tidak ada hapus permanen dari halaman ini.
- **Create dari nol:** belum ada. Dialog hanya menyimpan dari item yang sudah ada.
- **Account Admin:** model hak belum punya peran Account Admin. Blueprint akun hanya dikelola oleh pemilik Personal Space-nya.

### 23.6 Permission
- Daftar per Space diambil dari API yang sudah membatasi `view`. Tombol Ubah dan Create dinilai dari hak `manage_blueprint` dan `create_blueprint` di Space tersebut, dan tetap dipaksa di backend (tes Go `TestBlueprintRightsAreSeparate`, PASS).
- Pengguna yang hanya bisa melihat: tidak ada tombol Create, dan Ubah nonaktif (tes Playwright).
- Editor belum diperluas. Fungsi editor yang sudah ada (sudah di-deploy sebagai `pms:7e82090`) hanya dibuka lewat tombol Ubah yang memerlukan hak kelola. Tidak ada kapabilitas editor baru yang diaktifkan.

### 23.7 Hasil tes
- `npm test` (unit): 92 PASS, 0 gagal (termasuk 3 tes registry baru).
- Playwright:
  - `settings-blueprints.spec.ts`: 6 PASS (routing dan reload, back, filter/urut/kelompok, aksi dan editor, permission, empty state, katalog tetap).
  - Set regresi (blueprints, blueprint-editor, create-menu, spaces, forms, settings, sidebar, dr02): 75 PASS.
  - Navigasi dan form-blueprint-live (mock): 19 PASS, 1 dilewati.
- `tsc -b` dan ESLint untuk file baru: lulus.
- Screenshot halaman: `scratchpad/settings_blueprints_registry.png`.

### 23.8 Keamanan dan data
- Tidak ada hard delete. Tidak ada perubahan skema atau kontrak API.
- Tidak ada data mock yang ditampilkan sebagai data server.
- Fungsi editor baru tidak diaktifkan. Keputusan R1 (elevated actor) tetap seperti di bagian 12 dan 13.

---

## 24. Penutupan registry Settings → Blueprints

Tanggal: 18 Nov 2026.

- **Commit:** `d82d464` di `feat/blueprint-b5-scope-edit`, sudah di-push. PR #19 ikut terbarui.
- **Deploy:** `pms:d82d464` berjalan di dev-lens. Backup pra-deploy `20261118T082405Z`. Container healthy, migrasi 334, `items` 1673.
- **Verifikasi live (browser, akun uji):** PASS. Sidebar mengarah ke `/settings/blueprints`. Daftar memuat data server. Reload tetap di halaman. Pencarian memfilter baris. Duplikasi dan Arsip nonaktif. Screenshot live tersimpan di scratchpad.
- **Playwright penuh (mock):** 555 PASS, 43 dilewati, 0 gagal.
- **Unit:** `npm test` 92 PASS.

**Temuan saat verifikasi:** ada blueprint baru `Software delivery` di dev-lens, dibuat 08:06 UTC oleh `anissaefuddin@gmail.com` (`blueprint.save` di audit). Tidak ada yang dihapus. Perlu konfirmasi dari pemilik akun apakah ini dibuat secara sengaja.

**Account Admin (belum diimplementasi):** model hak belum punya peran Account Admin. Mengimplementasikannya mengubah model hak dan perlu keputusan: apakah peran ini global (instance) atau per Personal Space, dan apa saja yang boleh dilakukannya di Blueprint akun pengguna lain.
