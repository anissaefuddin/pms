# Analisa Space Management dan Space Settings

Dokumen ini berisi hasil Batch 0 (audit sebelum mengubah kode) dan catatan implementasi Batch 1 (halaman Space Settings). Status memakai: DONE, PARTIAL, MISSING, DEFECT, NOT VERIFIED, atau DECISION REQUIRED. DONE hanya diberikan bila ada test di repo yang menutupi kapabilitas itu. Tes integrasi Go butuh database test terisolasi (`PMS_TEST_DATABASE_URL`); pada sesi ini database test tidak berjalan, jadi tes Go berstatus NOT RUN dan status DONE di bawah berarti "ada test", bukan "test dijalankan hari ini".

## 1. Ringkasan

Space sudah punya fondasi yang kuat di backend: tabel `spaces` dengan `kind` (personal atau shared), `version` untuk If-Match, `locked`, dan `create_menu_hidden`; sharing dengan peran Read, Limited, Editor, Full, custom role, dan group; soft delete yang memindahkan item ke Recycle bin; serta template Space. Kekurangannya ada di sisi produk dan UX: Space Settings belum punya halaman sendiri (hanya dialog rename dan dropdown di header), Settings > Spaces hanya daftar sederhana, belum ada visibilitas Public/Private, belum ada ikon, belum ada archive/restore Space, dan belum ada kontrol Space admin di UI.

Batch 1 sudah dikerjakan: halaman `/spaces/:spaceId/settings/:tab` dengan sembilan tab, tab General lengkap (dirty-state, validasi, Save/Cancel, konflik versi, dan penjagaan perubahan belum disimpan), gear sidebar dan tombol Settings di header membuka halaman yang sama, dan rename cepat tetap tersedia.

## 2. Batch 0: Audit

### 2.1 Model data dan migrasi

| Area | Temuan | Bukti |
|---|---|---|
| Tabel `spaces` | `id`, `name`, `description`, `kind` (`personal`/`shared`), `owner_id`, `version`, `created_at`, `updated_at`; indeks unik satu personal per owner | `server/migrations/00003_spaces_workflows.sql` |
| Kolom tambahan | `deleted_at`, `deleted_by` (soft delete), `locked` (00304), `create_menu_hidden` (00320) | `00004_spaces_sharing.sql`, `00304_locked_spaces.sql`, `00320_space_create_menu.sql` |
| Visibilitas Space | Tidak ada. `view_visibility` (`public`/`personal`) hanya untuk View, bukan Space | `00008_views.sql` |
| Ikon Space | Tidak ada kolom | skema |
| Archive Space | Tidak ada `archived_at`. Hanya soft delete | skema |
| Konfigurasi default workflow per Space | Tidak terlihat di skema Space; workflow disimpan per tipe/workspace | `00003_spaces_workflows.sql` (perlu verifikasi lanjutan) |
| Group dan sharing | `groups`, `group_members`, `shares` (00004), `custom_roles` (00301) | skema |

Penilaian migrasi: Batch ini tidak menambah migrasi. Visibilitas, ikon, dan archive memerlukan kolom baru; keputusan produknya belum ada (lihat bagian 5), jadi migrasi ditunda.

### 2.2 Endpoint Space

Endpoint ada di `server/internal/httpserver/spaces.go` dan logika di `server/internal/spaces/spaces.go`:

| Aksi | Endpoint | Catatan |
|---|---|---|
| List | `GET /api/v1/spaces` | Personal dulu, lalu shared urut nama |
| Get | `GET /api/v1/spaces/{id}` | Tidak terlihat = 404 |
| Create | `POST /api/v1/spaces` | Hanya tipe user admin dan regular; pembuat otomatis Full |
| Rename/update | `PATCH /api/v1/spaces/{id}` | If-Match wajib; `locked` hanya admin |
| Delete | `DELETE /api/v1/spaces/{id}` | Soft delete; item tanpa lokasi lain ikut ke Recycle bin; Personal tidak bisa dihapus |
| Shares | `GET/PUT /api/v1/spaces/{id}/shares` | If-Match; batas 500 share |
| Template | `GET /api/v1/space-templates`, `POST .../space-templates/{id}/spaces` | Dibangun dalam satu transaksi |
| Archive/restore | Tidak ada | MISSING |
| Visibilitas Public/Private | Tidak ada | MISSING (DECISION REQUIRED) |

Tidak ada endpoint admin untuk daftar Space lintas workspace. Registry Settings > Spaces memakai daftar yang sama dengan sidebar, yaitu hanya Space yang bisa dilihat user.

### 2.3 UI yang ada sebelum Batch 1

- `web/src/routes/_app/spaces.$spaceId.tsx`: header dengan tombol Item baru, Forms, Automation, Item types, Bookmarks, Share, dan menu "..." (import, export, menu Create, custom field, rename, lock, delete).
- `web/src/components/space-dialog.tsx`: dialog Create dan Rename. Dialog ini tetap dipakai sebagai aksi cepat.
- `web/src/components/sidebar.tsx`: gear di setiap baris Space membuka dialog rename saja.
- `web/src/routes/_app/settings/spaces.tsx`: tabel nama, tipe, akses, dan aksi Rename/Open. Belum ada kolom admin, jumlah anggota, jumlah project, tool, status, atau pencarian lanjutan.
- Tidak ada halaman Space Settings penuh. Tidak ada pemilih ikon. Tidak ada tampilan Public/Private.

### 2.4 Permission Space

Capability Space (`server/internal/policy/policy.go`): View, Comment, ChangeStatus, Attach, Edit, Create, Delete, Share, UseBlueprint, CreateBlueprint, ManageBlueprint. Peran share: Read, Limited, Editor, Full, plus custom role. Tipe user: Admin, Regular, External, Collaborator, Viewer. Ceiling tipe user menentukan peran yang boleh diberikan (`policy.Ceiling`).

Permission diputuskan di backend (`policy.SpaceAccessFor`, `policy.VisibleSpaces`). UI hanya menyembunyikan tombol; server tetap menolak.

Lihat matriks permission Space di bagian 4.

### 2.5 Integrasi modul

| Modul | Status pada audit | Catatan |
|---|---|---|
| Item Types | Ada (`/spaces/{id}/item-types`, `/settings/item-types`) | Dipakai ulang di tab Custom item types |
| Custom Fields | Ada (`CustomFieldsDialog`, `workspace_custom_fields_integration_test.go`) | Dipakai ulang di tab Custom fields |
| Workflows | Ada di Settings > Workflows; default per Space belum terlihat | PARTIAL |
| Automation | Ada per Space dan rule Workspace (test `automation_workspace_integration_test.go`) | Dipakai ulang lewat tautan |
| Request Forms | Ada (`/spaces/{id}/forms`, `form_space_metadata_integration_test.go`) | Dipakai ulang lewat tautan |
| Blueprints | Ada, tetapi ada DEFECT keamanan (lihat `docs/features/blueprint/BLUEPRINT_ANALISA.md` bagian 4) | DEFECT |
| Item lintas Space | `item_locations` (multi-lokasi), `spaces/item_shares.go` | PARTIAL, belum diuji ulang |

### 2.6 Test yang ada

- `server/internal/httpserver/spaces_integration_test.go`: TestAuthorizationMatrix, TestSpaceAccessRules, TestSharesAreUnionedAndCappedAndValidated, TestDirectoryListing, TestSharesReviewFindings. Butuh `PMS_TEST_DATABASE_URL`.
- `space_templates_integration_test.go`: TestSpaceTemplatesBuildWholeSpaceOnceAndOnlyForCreators.
- `form_space_metadata_integration_test.go`, `workspace_custom_fields_integration_test.go`, `automation_workspace_integration_test.go`.
- `web/src/lib/*.test.ts`: `npm test` = 96 lulus, 0 gagal (dijalankan pada sesi ini).
- Playwright E2E ada di `web/` tetapi tidak dijalankan pada sesi ini.

Tidak ada test untuk: visibilitas Public/Private, archive/restore Space, ikon, halaman Space Settings, dan registry Settings > Spaces.

### 2.7 Working tree

Pada awal sesi, working tree sudah berisi banyak perubahan uncommitted dari sesi sebelumnya (Request Forms, audit Blueprint, Import/Export). Perubahan Batch 1 ditambahkan di atas itu. Tidak ada commit, push, merge, atau deploy.

## 3. Matriks ketertelusuran SPC-01 sampai SPC-18

Definisi diambil dari `docs/product/PRD.md` (bagian kapabilitas Space).

| ID | Kapabilitas (PRD) | Prioritas | Status | Bukti dan catatan |
|---|---|---|---|---|
| SPC-01 | CRUD Space dasar: nama, deskripsi, anggota, sharing; Space default dan Personal space per user | P0 | **DONE** | Create/Update/Delete/Shares di `spaces.go`; `TestSpaceAccessRules`, `TestSharesAreUnionedAndCappedAndValidated`; bootstrap di `bootstrap_integration_test.go`. Tes Go belum dijalankan ulang di sesi ini. |
| SPC-02 | Space overview: tampilan Table/Board atas semua item | P0 | PARTIAL | `view-workspace.tsx` dan `ViewWorkspace` ada. Tidak ada tes khusus di sesi ini. |
| SPC-03 | Preset filter (All tasks, All active, My active) dan sort | P1 | NOT VERIFIED | Belum diaudit detail. |
| SPC-04 | Pohon Projects dan folder di sidebar | P0 | PARTIAL | `SpaceNode` di `sidebar.tsx` dengan toggle dan tool. Belum ada tes untuk pohon folder. |
| SPC-05 | Buat Project lewat tombol "+" dengan field lengkap | P0 | NOT VERIFIED | `create-item-dialog.tsx` ada; isi field belum diaudit. |
| SPC-06 | Buat Folder | P0 | NOT VERIFIED | Belum diaudit detail. |
| SPC-07 | Panel detail project dan rename inline | P0 | PARTIAL | Route `items.$itemId.tsx` dan `tasks.$itemId.tsx` ada. |
| SPC-08 | Status project mengikuti workflow | P0 | NOT VERIFIED | Workflow ada; pengujian status per project belum dijalankan. |
| SPC-09 | Set start/due date project | P0 | NOT VERIFIED | Belum diaudit detail. |
| SPC-10 | Multi-lokasi: satu item di beberapa project/folder/space | P1 | PARTIAL | `item_locations` dan `spaces/item_shares.go`; belum ada tes ulang. |
| SPC-11 | Sharing dengan peran Full, Editor, Limited, Read Only ke user, grup, atau email eksternal | P0 | PARTIAL | Peran Read/Limited/Editor/Full, custom role, dan group diuji (`TestSharesAreUnioned...`). Email eksternal belum terlihat di kode share. Tab Members & access baru memakai editor yang sama. |
| SPC-12 | Approval pada item (approvers, Approve/Reject, penghitung) | P1 | NOT VERIFIED | `approvals.go` ada; belum diaudit. |
| SPC-13 | Blueprint: simpan struktur item sebagai template dan buat item dari blueprint | P1 | DEFECT | Dua temuan keamanan di `docs/features/blueprint/BLUEPRINT_ANALISA.md` bagian 4: data Space sumber bisa dibaca lewat blueprint, dan custom field restricted ikut tersalin tanpa cek. Tab Blueprints hanya menautkan halaman yang ada. |
| SPC-14 | Kustomisasi menu blueprint (tipe, nama, ikon) | P2 | PARTIAL | `create_menu_hidden` dan `CreateMenuDialog` ada. |
| SPC-15 | Export project ke Excel dan import | P1 | PARTIAL | `projectexcel`, `excel_import.go`, dan `docs/features/import-export/IMPORT_EXPORT_ANALISA.md` ada. Belum ada tes lulus di sesi ini. |
| SPC-16 | Public snapshot read-only dengan public link | P2 | PARTIAL | `GanttSnapshotDialog` ada; belum diaudit keamanan public link. |
| SPC-17 | Menu view: copy link, rename, duplicate, move, delete | P0 | NOT VERIFIED | Belum diaudit. |
| SPC-18 | Buat Space dari template per kategori | P1 | DONE | `space_templates.go`, `TestSpaceTemplatesBuildWholeSpaceOnceAndOnlyForCreators`. Dialog Create memakai template. |

Ringkasan: 2 DONE, 9 PARTIAL, 1 DEFECT, 6 NOT VERIFIED, 0 MISSING pada level SPC. Public/Private dan archive tidak punya SPC sendiri, tetapi termasuk dalam Batch 4 dan dicatat sebagai MISSING di bagian 5.

## 4. Matriks permission Space

| Aksi | User type | Space role | Penegakan | Catatan |
|---|---|---|---|---|
| Buat Space | Admin, Regular | - | Backend (`CreateTx`) | External, Collaborator, Viewer ditolak 403 |
| Rename/deskripsi | - | Edit | Backend (`Update`) | Personal ditolak 403 di API (`access.Kind == "personal"`) |
| Lock Space | Admin | Share | Backend dan UI (`canLock`) | Admin saja |
| Kelola share | - | Share | Backend (`ReplaceShares`) | Peran yang bisa diberikan dibatasi ceiling tipe user (`policy.Ceiling`); pembuat Space selalu Full. Apakah pemberi share dibatasi peran tertingginya sendiri belum diverifikasi |
| Hapus Space | - | Delete | Backend (`Delete`) | Personal tidak bisa dihapus |
| Kelola custom field | - | Share dan Delete | UI dan backend | Dipakai di tab Custom fields |
| Buat item | - | Create | Backend | |
| Edit form, automation, item types | - | Edit | UI (`canEditForms`) dan backend endpoint masing-masing | |
| Lihat Space | - | View | Backend (`SpaceAccessFor`); tidak terlihat = 404 | |

Batch 2 (Members & access) belum dikerjakan. Aturan konflik grant langsung dan grant lewat group, serta larangan menaikkan akses sendiri, perlu didokumentasikan saat Batch 2 (lihat bagian 5).

## 5. Keputusan yang perlu diambil user

| ID | Keputusan | Pilihan | Rekomendasi | Mengapa ditunda |
|---|---|---|---|---|
| D-01 | Visibilitas Public/Private Space | Kolom baru `visibility` (migrasi) atau tanpa kolom (Public = semua user Workspace bisa melihat) | Usulan: kolom `visibility` (`public`/`private`), default `private` untuk Space baru; Space lama tetap seperti sekarang. Keputusan user | Belum ada kebijakan join. Tanpa keputusan, UI tidak bisa mengubahnya. Tab General hanya menampilkan status. |
| D-02 | Confidential/Locked | Hanya desain | Sesuai prinsip 7, tidak diaktifkan dari UI. Kolom `locked` yang sudah ada tetap dipakai admin | Butuh kajian keanggotaan, sharing keluar Space, pemindahan lintas Space, public link, automation, export, pencarian, notifikasi, field permission, dan perubahan group. Belum dikerjakan. |
| D-03 | Archive Space | Kolom `archived_at` dan efek ke automation, forms, blueprints, dan tool | Archive menyembunyikan dari sidebar dan tetap menyimpan item, ID, dan data; automation dan form nonaktif | Perlu migrasi dan keputusan efek. |
| D-04 | Ikon Space | Kolom `icon` atau tanpa ikon | Kolom `icon` teks pendek | Perlu migrasi. |
| D-05 | Space admin | Peran khusus Space atau pakai owner | Pakai `owner_id` untuk admin pertama, lalu peran Full | Perlu keputusan transfer kepemilikan. |
| D-06 | Konflik grant langsung vs group | Aturan | Usulan: efektif = union peran dari semua sumber (kode `policy.Effective` sudah melakukan union), lalu dibatasi ceiling tipe user | Perlu dikonfirmasi dan didokumentasikan di Batch 2. |

## 6. Peta rute dan navigasi (setelah Batch 1)

| Rute | Halaman | Catatan |
|---|---|---|
| `/spaces/:spaceId` | Space (overview, tool, item) | Tombol Settings di header |
| `/spaces/:spaceId/settings/:tab` | Space Settings | Tab: general, members, tools, item-types, custom-fields, workflows, automation, forms, blueprints. Tab tidak dikenal diarahkan ke general |
| `/settings/spaces` | Registry Workspace | Tombol Settings per Space |
| `/spaces/:spaceId/item-types`, `/automation`, `/forms` | Halaman lama | Tetap berfungsi; ditautkan dari tab |

Gear sidebar dan tombol Settings di header membuka rute yang sama. Rename cepat ada di menu "..." sidebar dan di menu header.

## 7. Batch 1: Implementasi

### 7.1 Perilaku sebelum dan sesudah

| Aspek | Sebelum | Sesudah |
|---|---|---|
| Gear sidebar | Membuka dialog rename | Membuka halaman Space Settings; menu "..." berisi Rename |
| Header Space | Tidak ada Settings | Tombol Settings (jika bisa edit) |
| Halaman Settings | Tidak ada | Halaman penuh dengan sembilan tab |
| Tab General | - | Nama, deskripsi, validasi (panjang rune sama dengan server), Save/Cancel, status dirty, konflik versi (409/428) dengan tombol muat ulang, peringatan perubahan belum disimpan saat pindah tab atau keluar, visibilitas dan status lock read-only |
| Settings > Spaces | Rename dan Open | Tambah Settings |

### 7.2 Komponen yang dipakai ulang

- `SettingsSection`, `SettingsRow`, `SettingsSaveBar` dari `components/settings/settings-ui.tsx`.
- `ShareDialog` lewat `SpaceShareDialog` (hasil ekstraksi dari halaman Space, bukan duplikat).
- `CustomFieldsDialog`, `ConfirmDialog`, `WorkspaceLayout`, `WorkspaceHeader`, `FormError`, `Button`, `Input`.
- `SpaceDialog` tetap untuk rename cepat dan create.

### 7.3 File yang berubah

| File | Perubahan |
|---|---|
| `web/src/routes/_app/spaces_.$spaceId.settings.$tab.tsx` | Baru: rute kanonik |
| `web/src/components/space-settings.tsx` | Baru: shell, tab, General, Members, Custom fields, tab tautan |
| `web/src/components/space-share-dialog.tsx` | Baru: ekstraksi dari halaman Space |
| `web/src/routes/_app/spaces.$spaceId.tsx` | Pakai `SpaceShareDialog`; tombol Settings |
| `web/src/components/sidebar.tsx` | Gear jadi tautan; menu "..." dengan Rename |
| `web/src/routes/_app/settings/spaces.tsx` | Tombol Settings per baris |
| `web/src/routeTree.gen.ts` | Digenerate ulang oleh build |
| `web/src/i18n/en/spaces.json`, `id/spaces.json`, `en/nav.json`, `id/nav.json` | Kunci baru `spaces.spaceSettings.*` dan `nav.space.actionsFor` |

Migrasi database: tidak ada. Perubahan API: tidak ada. Perubahan permission: tidak ada.

### 7.4 Hasil verifikasi

| Pemeriksaan | Hasil |
|---|---|
| `tsc -b` | PASS |
| `vite build` | PASS |
| `eslint` pada file yang diubah | PASS |
| `npm test` (node unit) | PASS, 96 lulus |
| Browser: halaman General render dan data dari API | PASS (Space "General") |
| Browser: dirty-state, Save menyala saat berubah, Cancel mengembalikan "All changes saved" | PASS |
| Browser: pindah tab dengan perubahan belum disimpan memunculkan "Leave without saving?", "Keep editing" menahan | PASS |
| Browser: tab Members & access render | PASS |
| Browser: Save (mengubah data) | NOT RUN (tidak mengubah data bersama) |
| Browser: sidebar gear dan menu Rename | NOT RUN |
| Browser: layar lebar, mobile, dan keyboard | NOT RUN |
| Playwright `tests/spaces.spec.ts` (mock API, 19 tes: sidebar, create, rename, 409/428, hapus, capability) | PASS, 19/19 |
| Playwright E2E lain (dengan backend) | NOT RUN |
| Go integration (`spaces_integration_test.go`, dll.) | NOT RUN, database test tidak berjalan (port 55432 tidak aktif). Tidak dijalankan di server bersama. |
| Konsol browser | Dua 404 tidak dijelaskan; tidak terkait halaman ini (belum ditelusuri). |

Catatan UI: di lebar pane 800px tab terpotong dan bisa digulir horizontal; judul "General" muncul dua kali (tab dan judul bagian). Keduanya belum diubah.

## 8. Batch 2 sampai 6: Status

| Batch | Status | Alasan |
|---|---|---|
| 2 Members & access | Sebagian selesai (lihat bagian 10) | Daftar anggota efektif, alasan akses, dan aturan penjagaan share sudah ada. Belum ada: pencarian user yang memenuhi syarat dalam tab (masih lewat editor share), dan penetapan admin Space (menunggu D-05). |
| 3 Registry | Belum dikerjakan | Butuh endpoint agregat (jumlah anggota, project, tool, aktivitas). |
| 4 Visibilitas dan lifecycle | DECISION REQUIRED (D-01, D-02, D-03) | Butuh migrasi dan kebijakan. |
| 5 Konfigurasi kerja | Hanya tautan | Integrasi penuh ke tab belum dikerjakan. |
| 6 Tool Space | Belum diaudit | Di luar batch 1. |

## 9. Risiko yang tersisa

1. Tes Go untuk Space belum dijalankan ulang; perlu database test terisolasi.
2. Blueprint masih punya DEFECT keamanan (SPC-13). Jangan mengekspos blueprint lintas Space sebelum diperbaiki.
3. Tab Members & access memakai editor lama; aturan konflik grant belum didokumentasikan.
4. Visibilitas dan archive belum ada; tab General menyatakan itu terang-terangan.
5. Ikon Space belum ada; tidak ditambahkan karena butuh kolom.
6. Dua 404 di konsol belum ditelusuri.

## 10. Batch 2: Members & access (sebagian)

### 10.1 Yang berubah

| Area | Perilaku sebelumnya | Perilaku sekarang |
|---|---|---|
| Daftar anggota | Hanya daftar share langsung; group tidak diuraikan | `GET /api/v1/spaces/{id}/members`: satu baris per user, dengan peran efektif, batas jenis user, dan alasan (share langsung atau lewat group) |
| Hapus atau turunkan share di atas wewenang | Tidak dicek. Pemegang share yang punya `share` tetapi tidak punya seluruh kapabilitas Full bisa menghapus share Full | Ditolak 403: wewenang pemberi harus memuat kapabilitas share yang diubah atau dihapus |
| Penghapusan pemegang Full terakhir | Diizinkan | Ditolak 422 bila Space punya pemegang Full lalu menjadi tidak punya. Space yang sudah tanpa pemegang tetap bisa disimpan tanpa perubahan |
| Tab Members & access | Tautan ke editor share | Tabel anggota efektif dengan alasan akses, aturan cara kerja akses, dan tombol editor share |

Aturan penjagaan hanya berlaku untuk share Space (`target == "space"`), tidak untuk share item.

### 10.2 Aturan konflik grant (D-06, didokumentasikan)

1. Akses efektif user = gabungan (union) kapabilitas dari semua grant yang berlaku: share langsung dan share lewat group, termasuk custom role.
2. Hasil gabungan dibatasi ceiling jenis user (`policy.Ceiling`). Jika dibatasi, UI menandai "Dibatasi jenis user".
3. Admin memiliki semua kapabilitas di setiap Space tanpa share (D-17) dan tidak muncul di daftar anggota.
4. Tidak ada grant yang menimpa grant lain: grant langsung tidak mengalahkan group, dan sebaliknya. Keduanya digabung.
5. Peran efektif diberi nama: peran bawaan yang kapabilitasnya sama persis, atau "custom" jika tidak ada yang sama.

### 10.3 Pemisahan konsep

| Konsep | Memberi otorisasi? | Di mana |
|---|---|---|
| User Type | Ya, sebagai batas atas (ceiling) | Pengaturan user |
| Space Role (read, limited, editor, full, custom) | Ya | Share Space |
| Item Permission | Ya, per item | Share item |
| Field Permission | Ya, per field | Aturan field |
| Job Role | Tidak, hanya informasi | Profil user |

Teks ini tampil di tab Members & access.

### 10.4 Verifikasi

| Pemeriksaan | Hasil |
|---|---|
| `go build ./...`, `go vet` paket spaces dan httpserver | PASS |
| Unit `internal/spaces` (merge grant, ceiling, nama peran efektif) | PASS, 4 tes |
| Integrasi `TestSpaceShareChangeRules` (baru) | PASS pada database uji lokal terisolasi |
| Integrasi `TestAuthorizationMatrix`, `TestSpaceAccessRules`, `TestSharesAreUnioned...`, `TestDirectoryListing`, `TestSharesReviewFindings`, dan sisa paket Space/Share/Access/Lock/Template/Workspace | PASS |
| `TestEveryProtectedEndpointChecksTheSessionAndSurvivesUnknownIds`, `TestAuthContract`, `TestOpenAPIDocumentsApprovalFinalConflict` | PASS |
| Playwright `tests/spaces.spec.ts` (19 tes) | PASS |
| Playwright `tests/space-settings.spec.ts` (4 tes baru, data mock): tabel anggota, tab tak dikenal, Cancel, peringatan keluar | PASS |
| `tsc -b`, `vite build`, `eslint` file yang diubah, `npm test` | PASS |
| Browser di server lokal (8080) | NOT RUN untuk tab Members: proses di port 8080 adalah biner lama tanpa rute `/members` (404). Tidak dimulai ulang karena dipakai sesi lain. |

Database uji: PostgreSQL 16 terisolasi di `/private/tmp/claude-501/pgtest` pada port 55470, dibuat khusus untuk sesi ini. Tidak menyentuh database bersama, dev-lens, atau produksi.

### 10.5 Yang belum dan risikonya

- Penetapan admin Space dan transfer kepemilikan menunggu D-05.
- Pencarian dan tambah anggota dalam tab masih lewat editor share (dialog). Pencarian user yang memenuhi syarat belum dipindah ke tab.
- Peringatan dampak sebelum hapus anggota: belum ditambahkan khusus. Editor share yang ada hanya memakai konfirmasi umum.
- OpenAPI (`api/openapi.yaml`) belum memuat `GET /members`; tidak ada tes yang mewajibkannya. Perlu ditambahkan dan kode dibangkitkan ulang sebelum rilis.
- Dua 404 di konsol browser tetap belum ditelusuri.

## 11. Keputusan yang disetujui dan implementasinya (D-01 sampai D-06, D-17)

### 11.1 Status keputusan

| ID | Keputusan | Status implementasi |
|---|---|---|
| D-01 Public/Private | Kolom `visibility` (default `private`), Public ditemukan dan diikuti dengan Read, Personal tidak bisa diubah, perubahan lewat konfirmasi dan audit | Diimplementasikan (migrasi 00346, `PATCH /spaces/{id}` dengan `visibility`, `GET /spaces/discover`, `POST /spaces/{id}/join`, audit `space.visibility_changed`, `space.joined`) |
| D-02 Confidential | Pertahankan `locked`, jangan tambahkan Confidential sebagai opsi | Tidak diubah. Kolom `locked` dan semantiknya sama seperti sebelumnya |
| D-03 Archive/Restore | `archived_at`, `archived_by`; item dan konfigurasi tetap; create/update dihentikan; automation dan form dinonaktifkan; restore memeriksa ulang | Diimplementasikan untuk akses (archived = View saja), form submission, dan automation (filter Space terarsip di `Handle`). Restore mengembalikan `warnings`. Blueprint, tool, dan riwayat tidak diubah |
| D-04 Icon | `icon_key` dengan whitelist, visual picker, konsisten di sidebar, header, registry | Diimplementasikan: whitelist 16 ikon di `IconKeys`, `SpaceIconPicker`, `SpaceIcon` di header, sidebar, dan registry |
| D-05 Explicit Space Admin | Bukan Full; owner awal; banyak admin; Full tidak dipromosikan otomatis; transfer eksplisit; admin terakhir tidak bisa dicabut | Diimplementasikan lewat tabel `space_admins` (migrasi 00346). Admin mendapat kapabilitas Full (dibatasi ceiling jenis user). Grant hanya ke anggota aktif. Transfer dalam satu transaksi |
| D-06 Union of grants | Pertahankan union; ceiling setelah penggabungan; sumber langsung/group ditampilkan; cegah eskalasi | Diimplementasikan: `buildMembers`, `guardSpaceShareChange` (tidak boleh menurunkan atau menghapus di atas wewenang), tabel anggota dengan sumber |
| D-17 Workspace Admin | Security review: pisahkan administrasi Workspace dari konten Private | Hanya impact assessment (bagian 11.3). Model otorisasi lama tidak diubah |

### 11.2 Migrasi

`server/migrations/00346_space_governance.sql` (aditif, tanpa perubahan baris `shares`):
- `spaces`: `visibility` (default `private`, dibatasi `public|private`), `archived_at`, `archived_by`, `icon_key` (regex katalog), constraint `spaces_personal_is_private`.
- Tabel baru `space_admins` (space_id, user_id, granted_by, granted_at).
- Backfill: admin awal = pembuat Space shared yang punya share Full untuk dirinya sendiri (`created_by = principal_id`). Space tanpa baris seperti itu tidak mendapat admin dan tetap dikelola admin Workspace.
- Nomor 00346 dipilih karena 00345 adalah migrasi terakhir di folder. Migrasi lain yang belum di-commit dari sesi lain (00333–00345) berisiko bentrok nomor. Perlu dicek sebelum merge.

### 11.3 Impact assessment D-17 (Workspace Admin)

Kondisi sekarang:
- `spaceCaps` memberi tipe user Admin kapabilitas penuh di setiap Space shared, termasuk Private (D-17 yang sudah ada). Admin juga melihat semua Space shared di `VisibleSpaces`, yang berarti nama dan deskripsi Space Private terlihat di registry.
- Space Personal tetap tidak terlihat oleh siapa pun selain pemilik.
- Perubahan baru di batch ini mengikuti pola yang sama: `CanAdministerSpace` memberi Workspace admin hak mengelola admin, visibilitas, dan arsip Space Private tanpa harus menjadi anggota. Ini sengaja tidak mengubah model lama, tetapi harus dinilai ulang.

Dampak yang harus ditinjau:
1. Admin Workspace bisa membaca item Space Private tanpa share. Bertentangan dengan tujuan Private.
2. Registry Settings > Spaces menampilkan nama Space Private ke admin. Batch 3 mensyaratkan metadata Private tidak tampil ke yang tidak berwenang.
3. Admin Workspace bisa mengubah visibilitas dan mengarsipkan Space Private, lalu menjadikan dirinya admin Space. Ini mengubah akses tanpa jejak di luar audit.
4. Automation dan form berjalan atas nama user dan space; admin tetap melihat hasilnya.

Opsi untuk security review (tidak dipilih di sini):
- A. Admin Workspace hanya bisa mengelola metadata (nama, visibilitas, arsip, admin) Space Private, tanpa membaca item. Perlu pemisahan di `VisibleSpaces` dan `Can` untuk item.
- B. Break-glass: akses konten Private hanya dengan alasan tertulis dan audit terpisah.
- C. Pertahankan model sekarang dan dokumentasikan sebagai keputusan eksplisit.

Sampai keputusan, perilaku lama dipertahankan. Tes integrasi yang ada, terutama `TestAuthorizationMatrix` (baris admin: penuh di setiap Space), mengunci perilaku ini.

### 11.4 Verifikasi

| Pemeriksaan | Hasil |
|---|---|
| Go integrasi `TestSpaceGovernance` (baru): visibilitas, join, Read saja, icon, admin explicit, transfer, arsip, restore, item tetap | PASS (database uji terisolasi, port 55470) |
| Go integrasi `TestSpaceShareChangeRules`, `TestAuthorizationMatrix`, `TestSpaceAccessRules`, `TestSharesAreUnioned...`, `TestDirectoryListing`, `TestSharesReviewFindings`, dan paket Space/Share/Access/Lock/Template/Workspace | PASS |
| Go `./...` penuh pada database uji | PASS, 23 paket dengan tes lulus, exit 0 (dijalankan dua kali, run kedua setelah perubahan automation) |
| Unit `internal/policy`, `internal/spaces` | PASS |
| OpenAPI: path `/members`, `/archive`, `/restore`, `/admins`, `/transfer-ownership`, `/spaces/discover`, `/join`, `/space-icons`, dan field Space | Ditambahkan ke `api/openapi.yaml`; `schema.d.ts` dibangkitkan ulang (`npm run generate`). Kode Go hasil `apigen` belum dibangkitkan ulang dan tidak diuji |
| Playwright `spaces.spaceSettings` dan `space-settings.spec.ts` (7 tes, data mock) | PASS, 7/7 (termasuk arsip dengan `If-Match`, icon, discovery + join) |
| Playwright `spaces.spec.ts` (19 tes) | PASS |
| `tsc -b`, `vite build`, eslint file yang diubah | PASS |
| Mobile 375px: General dan Members, overflow horizontal | PASS, 0 px overflow; screenshot dicek |
| Keyboard | Tidak diuji secara terpisah. Kontrol memakai `button` dan `select` asli; fokus terlihat dari komponen yang sudah ada |
| Browser desktop pada server lokal (8080) | Terbatas: proses di 8080 adalah biner lama tanpa rute baru. Tab Members dan admin tidak bisa diuji di server itu |
| Konsol 404 | Dijelaskan: `/admins` dan `/members` belum ada di proses 8080 yang lama. 404 pertama di awal sesi tidak bisa direproduksi dari log saat ini dan belum ditelusuri lebih jauh |
| SPC-13 Blueprint lintas Space | Tidak diverifikasi ulang di sesi ini. Perubahan blueprint ada di migrasi 00333–00335 dari sesi lain yang belum di-commit. Perlu tes khusus sebelum rilis |

### 11.5 Yang belum selesai

- Pencarian dan tambah anggota di dalam tab (masih lewat editor share).
- Restore belum menjalankan ulang automation dan form secara otomatis; hanya memberi peringatan.
- Tes integrasi untuk form submission dan automation di Space terarsip belum ditulis. Perubahan kodenya ada (`engine.go` form caps, `Handle`), tetapi belum diuji langsung.
- Tab Tools dan Workflows masih berupa tautan.
- Keputusan D-17 menunggu security review.
- Konflik nomor migrasi dengan sesi lain belum diperiksa.

## 12. Lanjutan: D-17, Batch 2 sisa, Batch 3, Batch 5, Batch 6

### 12.1 D-17 opsi A: dicoba, dibatalkan, perlu keputusan

> **Keputusan 20 Nov 2026 (D-17.1, APPROVED): Explicit Access Only.** Opsi A diterima sebagai keputusan produk. A1 dan A2 tidak dipilih. Bagian di bawah tetap sebagai riwayat.

Opsi A (Workspace Admin tanpa akses implisit ke konten Space shared) diterapkan sebagai percobaan pada policy: `spaceCaps`, `CanAdministerSpace`, query kapabilitas item, dan pengecualian locked. Hasilnya:
- 194 tes gagal di paket policy, spaces, httpserver, automation, forms, connectors, dan bootstrap. Sebagian besar fixture memperlakukan admin Workspace sebagai anggota implisit semua Space.
- Dampak produk: analitik, approval, dashboard, form, impor/ekspor, dan Excel untuk admin ikut berubah. Ini bukan hanya perubahan tes.

Perubahan dibatalkan. Kode policy kembali ke perilaku sebelumnya dan tes Space lulus lagi. Opsi A tidak bisa diterapkan tanpa keputusan produk untuk laporan Workspace. Dua varian yang perlu dipilih:
- A1. Admin Workspace tetap bisa membaca data untuk laporan dan approval, tetapi tidak bisa membuka item Space Private atau mengubah izinnya tanpa Space Admin. Perlu definisi laporan mana yang dianggap "operasional".
- A2. Admin Workspace tanpa akses konten. Semua laporan dan approval Workspace harus dibangun ulang dengan scope akses user yang melihat. Dampaknya besar.

### 12.2 Batch 2 sisa

- Tab Members: cari user (dan undang user eksternal) lewat komponen `Search` yang sudah ada, tambah dengan peran, ubah peran langsung, hapus dengan dialog yang menyebut akses lewat group yang tetap ada. Simpan memakai `PUT /shares` dan `If-Match`.
- Admin Space (grant, cabut, transfer) sudah ada sebelumnya.

### 12.3 Batch 3 (registry)

- Endpoint baru: `GET /api/v1/spaces/registry`. Hanya Space yang sudah bisa dilihat pemanggil. Berisi admin (nama), jumlah share, jumlah project, jumlah tool (view tingkat Space), aktivitas terakhir, status, visibilitas, dan `version` untuk rename.
- Halaman: pencarian nama, filter status dan visibilitas, urut (nama, dibuat, aktivitas, project), kelompokkan (visibilitas atau status), kolom opsional (deskripsi, tool, dibuat, aktivitas), pagination 25 per halaman, empty state yang membedakan "tidak ada Space" dan "tidak cocok filter", aksi: rename cepat, Settings, Salin tautan, Buka.
- Metadata Space privat tidak tampil bagi yang tidak berwenang: query memakai `VisibleSpaces`. Tes Go `TestSpaceRegistryShowsOnlyWhatTheCallerCanSee` membuktikannya.
- Belum: aksi Delete di registry (menunggu preview dampak Batch 4). Arsip dan pulihkan ada di tab General.

### 12.4 Batch 5 (konfigurasi kerja)

| Tab | Status | Keterangan |
|---|---|---|
| Custom item types | Embed `ItemTypesPanel` (komponen yang sudah ada) | Daftar Workspace dan Space, builder yang sama |
| Custom fields | Dialog `CustomFieldsDialog` (sudah ada) | Daftar Workspace belum ditampilkan di tab |
| Workflows | Hanya tautan ke pengaturan Workspace | Default workflow Task/Project per Space belum ada di backend. Perlu keputusan dan endpoint |
| Automation | Embed `SpaceAutomationPanel` (aturan Space dan tampilan Workspace read-only sesuai panel) | Kontrak eksekusi tidak diubah |
| Request forms | Embed `FormsList` (dipecah dari halaman, tanpa header saat tertanam) | Edit admin dipisah dari submit user sesuai halaman form yang sudah ada |
| Blueprints | Tautan ke halaman Blueprint Workspace | Daftar khusus per Space belum dibuat |

### 12.5 Batch 6 (tools Space)

- Tab Tools menampilkan tool Space (`GET /spaces/{id}/views`), tipe, penanda landing, tombol buka (ke halaman Space dengan `?view=`), dan "Jadikan halaman awal" (`PATCH is_default`) untuk yang berwenang.
- Tambah, ubah nama, duplikat, hapus, dan urut tetap di toolbar tool halaman Space (`ViewTabs`). Tidak diduplikasi di Settings, sesuai aturan "jangan buat duplikat instance".
- Belum diaudit: tab Calendar yang duplikat di halaman Space dan pengelompokan toolbar. Keduanya perlu ditelusuri di `view-tabs.tsx` dan halaman `calendars.tsx` sebelum diubah.

### 12.6 Verifikasi

| Pemeriksaan | Hasil |
|---|---|
| Go tes Space/share/policy/governance (scratch DB) | PASS setelah pembatalan D-17 |
| Go tes registry baru | PASS |
| Go `./...` penuh (scratch DB) | PASS, exit 0, 23 paket dengan tes lulus (dijalankan setelah pembatalan D-17 dan registry) |
| Playwright `spaces` (19) + `space-settings` (9) | PASS, 28/28 (termasuk registry dan akses langsung). Tab Tools belum punya tes E2E |
| `npm test` (node) | PASS, 96/96 |
| `vite build`, eslint file yang diubah | PASS |
| Browser desktop/mobile | Mobile sudah dicek sebelumnya. Registry dan tab baru belum dicek visual |

## 13. Keputusan terbaru: migrasi 00346 dan status rilis

### 13.1 Audit migrasi
- Nomor duplikat di folder kerja: tidak ada. Migrasi 00333–00346 belum di-commit; 00332 adalah migrasi terakhir yang ter-commit.
- Runner memakai goose (`schema_migrations`, nomor versi). Tidak ada checksum. Urutan penting: versi baru yang lebih kecil dari yang sudah applied akan ditolak goose sebagai out-of-order.
- `00346_space_governance.sql` unik dan aditif (kolom baru, tabel baru, backfill tanpa mengubah `shares`). Sudah diuji pada database uji terisolasi dari nol.
- Keputusan: **KEEP**. Risiko yang tersisa: sesi lain bisa membuat migrasi 00346 sebelum merge. Cek ulang saat merge; jika bentrok, renumber hanya sebelum applied.
- Migrasi yang sudah applied di database bersama tidak diubah. Tidak ada rekonsiliasi migrasi yang diperlukan dalam sesi ini.

### 13.2 A1 (D-17)
- Percobaan opsi A pada salinan: 196 kegagalan. Dikategorikan di `docs/features/space-management/SPACE_A1_IMPACT_ASSESSMENT.md`: 173 fixture, 16 kontrak, 7 perlu telaah, 0 defek nyata.
- Implementasi A1 penuh (laporan agregat operasional, audit akses istimewa, approval dengan assignment, penutupan bypass di dashboard, search, API, export, automation, dan report) belum dikerjakan. Ini pekerjaan lintas modul.

### 13.3 Status yang belum selesai
- Workflows default Task/Project per Space: belum.
- Tab Blueprints dan Custom Fields dengan komponen existing: belum (masih tautan dan dialog).
- Delete impact preview dan dependency validation: belum.
- Audit duplicate Calendar dan toolbar grouping: belum.
- Browser visual review dan regression untuk tab yang belum teruji: belum.

### 13.4 Status rilis
**NO-GO.** Alasan: A1 belum diterapkan; 16 tes kontrak menunggu keputusan; 7 tes belum ditelusuri; lima item di 13.3 belum dikerjakan. Tes Go penuh pada repo utama lulus (lihat 12.6), tetapi itu belum mencakup A1.

## 14. Progres implementasi A1 (D-17) per kelompok

Status: **IN PROGRESS, belum DONE. Rilis tetap NO-GO.**

### 14.1 Yang sudah diterapkan di repo
- Policy: admin Workspace tidak punya kapabilitas implisit di Space shared (`spaceCaps`). `CanAdministerSpace` hanya lewat Space Admin eksplisit.
- Kapabilitas item: `itemsCaps` memberi `full` hanya lewat share, group, atau baris `space_admins`. Admin Workspace tidak lagi mendapat `full` di setiap Space shared.
- Locked: pengecualian `user_type='admin'` dicabut di `LockedAllows`. Space Admin eksplisit dihitung sebagai punya akses.
- Pindah item keluar dari Space terkunci: dibatasi kepada Space Admin eksplisit untuk setiap Space terkunci tempat item berada (`items/location.go`, helper `policy.IsSpaceAdmin`).

### 14.2 Kelompok KONTRAK (16 tes)
Selesai (7), asersi diubah sesuai A1, bukan untuk membuat hijau:
- `TestSpaceCapsRules`: admin tanpa share tidak punya kapabilitas; admin dengan share Full mendapat Full dibatasi ceiling admin.
- `TestAuthorizationMatrix`, `TestItemAuthorizationMatrix`: kapabilitas admin mengikuti share-nya sendiri, bukan Full implisit.
- `TestFormSpaceMetadataFollowsSpaceAccess`: admin tanpa akses Space tidak melihat nama Space di katalog. Public link dibuat oleh owner yang berwenang.
- `TestItemLocationRejectsCycle`: fixture memberi admin grant eksplisit (prasyarat edit).
- `TestLockedSpaceIsAdminOnlyAndAudited`, `TestLockedSpaceKeepsItemsAmongThePeopleWhoHaveTheSpace`: fixture memberi baris Space Admin eksplisit.

Belum (9), menunggu desain operational reporting (14.4): `TestAnalyticsEngineMatchesRawDataAndPolicy`, `TestAnalyticsMatchesRawDataOnVariedFixtures`, `TestReportActualCostIsAdminOnly`, `TestNotificationWorkerHonoursEmailPreferences`, `TestGoogleOnlyShowsWhatTheCreatorCanSee`, `TestReportTemplatesProduceTheRightNumbers`, `TestUtilizationPlannedVersusActualMatchesRawData`, `TestReportsLifecycleAndAccess`, `TestSearchPrefixAndScopesRespectVisibility`.

### 14.3 Kelompok FIXTURE (173 kegagalan) dan PERLU TELAAH (7)
- Belum diperbaiki secara massal. Pola: langkah setup memakai admin Workspace tanpa akses ke Space. Perbaikan yang sah: grant share eksplisit pada fixture, per tes, hanya bila akses itu prasyarat skenario.
- Jumlah kegagalan di repo: 189 (dari 196 pada salinan percobaan). Paket yang gagal: `httpserver`, `connectors`, `mail`.
- 7 tes telaah belum ditelusuri satu per satu.

### 14.4 Yang belum ada di kode
- Endpoint operational reporting khusus dengan audit akses istimewa. Analitik, report, dan search masih memakai jalur item policy lama.
- Approval Workspace Admin hanya dengan assignment/grant eksplisit: belum diverifikasi di kode approval.
- Regression test keamanan (unauthorized content, metadata leakage, reporting inference, approval bypass, cross-Space): belum ditulis.

### 14.5 Pembaruan: A1.1 dan A1.2 (sebagian)
- A1.1 selesai: `server/internal/policy/contract.go` mendefinisikan `CanAdministerWorkspace`, `CanAdministerSpace` (Space Admin eksplisit), `CanViewItem` (hanya dari caps item), `CanViewOperationalReport` (admin tipe, hanya agregat, perlu audit di endpoint), dan `CanApprove` (hanya dengan assignment atau grant). Tes `TestA1ContractSeparatesAdministrationFromContent` PASS.
- A1.2 sebagian: fixture Excel (`project_excel_integration_test.go`) mendapat satu share Full eksplisit untuk admin pada Space yang memang milik skenario. Paket Excel PASS. Ini menutup kelompok terbesar (37 kegagalan). Grant global tidak ditambahkan.
- Kegagalan tersisa di repo: 152 (httpserver, connectors, mail), tersebar di sekitar 50 file. Telah dikelompokkan per file untuk pekerjaan berikutnya.
- A1.3 (laporan operasional agregat dengan endpoint khusus dan audit), A1.4 (7 tes telaah), dan A1.5 (regression keamanan dan E2E lintas Space) belum dimulai.
- Status: IN PROGRESS. A1 belum DONE. Rilis NO-GO.

### 14.6 Pembaruan: perbaikan fixture per file (A1.2)
- Jumlah kegagalan Go penuh: 152 → 131.
- Perbaikan yang sah (grant eksplisit per skenario, bukan global):
  - `connectors/pilot_integration_test.go`: installer connector adalah anggota Full Space Repo. Paket connectors PASS.
  - `httpserver/project_excel_integration_test.go`: pemilik delivery adalah anggota Full. PASS.
  - `httpserver/dependencies_integration_test.go`, `scheduling_integration_test.go`, `item_effort_integration_test.go`, `portfolio_integration_test.go`, `workload_integration_test.go`: admin sebagai perencana di Space yang ia bangun. Hanya Space yang dipakai skenario.
- Tidak ditambahkan: grant global untuk admin, dan tidak ada asersi yang dilemahkan.
- Sisa dan klasifikasi awal:
  - 9 kontrak reporting/notifikasi (A1.3): analitik, cost, notifikasi email, Google visibilitas, report templates, utilization, report access, search. Membutuhkan endpoint laporan operasional dan aturan notifikasi.
  - Sebagian besar sisa (≈120) adalah fixture setup serupa dan belum ditelusuri per file (automation, webhooks, forms, custom fields, approvals, views, dashboards, time entries, dan lainnya).
  - Tes telaah (A1.4): belum diklasifikasikan satu per satu.
- Status A1: IN PROGRESS. Rilis NO-GO. Belum ada commit, push, atau deploy.

### 14.7 Pembaruan: fixture batch kedua (A1.2)
- Hitungan FAIL Go penuh: 131 → 79 (12 gerbang lingkungan `PMS_*`, 3 ambang waktu, sisa ≈64 nyata).
- Grant eksplisit ditambahkan per skenario (bukan global) pada: audit, custom roles, custom fields, dan satu grant otomatis per baris setup yang gagal (6).
- Dua tes share diperbaiki karena aturan last-holder: helper tes kini selalu menyertakan admin sebagai pemegang Full. Asersi perubahan share tidak diubah.
- Tes gerbang lingkungan (`PMS_TEST_BROWSER`, `PMS_PERF`, `PMS_TEST_SCALE`, `PMS_TEST_SSE_HEARTBEAT`, `PMS_TEST_PILOT_PERFORMANCE`, `PMS_TEST_SCHEDULING_PERFORMANCE`) gagal bila variabel tidak diset. Ini status BLOCKED (lingkungan), bukan defek. Tidak di-skip diam-diam.
- Sisa nyata (≈64) belum ditelusuri per file: automation (lifecycle, a4, b, tryit, guards, registry, settings), attachment (upload 404), webhooks, forms public link, item type defaults, relation fields, recycle, dashboards, reports, sse, time entries. Banyak memakai helper atau nama variabel Space yang berbeda.
- Status A1: IN PROGRESS. Rilis NO-GO.

## 15. Ringkasan putaran terbaru (A1.1–A1.3)

### 15.1 Kontrak otorisasi (A1.1 dan Fase 1)
- `server/internal/policy/contract.go`: `CanAdministerWorkspace`, `CanViewItem`, `CanViewOperationalReport`, `CanApprove`. `CanAdministerSpace` (Space Admin eksplisit) ada di `engine.go`.
- `policy.IsSpaceAdmin` ditambahkan untuk pemeriksaan Space Admin eksplisit di tempat lain.
- Admin Workspace tidak punya akses konten implisit ke Space shared. Admin tetap bisa administrasi Workspace (pengguna, pengaturan, workflow, tarif) tanpa akses konten.

### 15.2 Laporan operasional (A1.3, Fase 2)
- Mesin analitik yang sama, dengan mode operasional: `analytics.RunOperational`, dipanggil oleh `POST /api/v1/analytics/operational`.
- Izin: `CanViewOperationalReport` (tipe admin). Tanpa izin → 403.
- Hasil hanya agregat (grup dan nilai). Tidak ada baris item, tidak ada break-down (`break_by` ditolak 403), tidak ada biaya (`actual_cost_minor` ditolak 403), dan tidak ada metrik turunan (`completion_percent`, `avg_field` ditolak 403).
- Ambang privasi: grup dengan kurang dari `MinGroupItems` (5) item disembunyikan, dan jumlahnya ditampilkan sebagai `suppressed`. Total hanya dihitung dari grup yang ditampilkan.
- Audit: `report.operational_read` untuk panggilan berhasil dan `report.operational_denied` untuk panggilan ditolak. Yang dicatat hanya bentuk query dan jumlah hasil, bukan isi baris.
- Tes: `TestOperationalReportIsAggregateOnlyAndAudited` (positif dan negatif, termasuk audit) PASS.

### 15.3 Kontrak tes yang diselaraskan (9 kontrak, A1.3)
| Tes | Status | Catatan |
|---|---|---|
| TestAnalyticsEngineMatchesRawDataAndPolicy | PASS | Admin dengan grant eksplisit mendapat angka penuh. Admin tanpa grant: laporan reguler kosong, operasional tersembunyi karena ambang. |
| TestAnalyticsMatchesRawDataOnVariedFixtures | PASS | Admin mendapat grant eksplisit pada satu Space; `seesSpace` mengikuti grant itu. |
| TestReportActualCostIsAdminOnly | PASS | Fixture: admin punya grant pada Space biaya. |
| TestReportTemplatesProduceTheRightNumbers | PASS | Fixture. |
| TestGoogleOnlyShowsWhatTheCreatorCanSee | PASS | Installer connector adalah anggota Full (fixture, paket connectors). |
| TestUtilizationPlannedVersusActualMatchesRawData | PASS | Fixture: perencana punya grant. |
| TestReportsLifecycleAndAccess | PASS | Admin tanpa grant tidak melihat laporan pribadi/privat orang lain; asersi tetap. |
| TestNotificationWorkerHonoursEmailPreferences | OPEN | Penerima diberi grant eksplisit, tetapi worker masih tidak mengirim email (0 dari 5). Penyebab belum ditelusuri: jalur `deliver` di `internal/mail/notification.go` belum diperiksa. |
| TestSearchPrefixAndScopesRespectVisibility | OPEN | Hasil pencarian `space_id` berbeda dari yang diharapkan (ID berbeda, bukan hanya jumlah). Penyebab belum ditelusuri. Indeks `items.search` hanya judul dan deskripsi, jadi bukan karena anak item. |

### 15.4 Status kelompok kegagalan
- Hitungan FAIL Go penuh: 77 (11 gerbang lingkungan `PMS_*`, sisanya nyata), PASS 382.
- Daftar lengkap: `docs/features/space-management/SPACE_A1_FAILURE_LEDGER.md` (dibuat pada putaran sebelumnya; belum diperbarui ulang per tes).
- A1 tetap IN PROGRESS. Rilis NO-GO.


## 16. Status akhir putaran ini (kontrak dan fixture)

- Hitungan Go penuh: FAIL 39 (10 gerbang lingkungan `PMS_*`, 29 nyata), PASS 421. Semua kegagalan berada di paket `httpserver`. Paket `mail`, `connectors`, dan `automation` lulus.
- Sembilan tes kontrak: **9 dari 9 PASS**. Notifikasi: penerima diberi grant eksplisit (akar: setup tes menaruh grant di luar blok error); ditambah tes regresi `TestNotificationNeverLeaksAnItemTheRecipientCannotView`. Pencarian: penerima `users` harus punya View pada Space yang dicari, pemilik diberi grant eksplisit pada Space yang ia buat.
- Tujuh tes investigasi: PASS satu per satu (lihat `docs/features/space-management/SPACE_A1_FAILURE_LEDGER.md`).
- Kontrak diubah secara eksplisit (asersi A1), bukan dihapus: `TestSpaceAccessRules` (admin tanpa share: 404, dengan share: 200), serta tes analitik yang kini memakai grant eksplisit pada admin tertentu dan admin tanpa grant sebagai kasus negatif.
- A1 tetap IN PROGRESS. Rilis NO-GO.

### 16.1 Kegagalan nyata yang tersisa (29)
- TestCompareTwoVersionsOfOneFile: `    attachment_pdf_skipped_integration_test.go:40: upload: 404 map[code:not_found detail:not found request_id:n7-b0pHNFQqVInyT status:404 title:Not Fo`
- TestPDFPreviewWithoutRendererIsSkippedAndPageIs404: `    attachment_proofing_integration_test.go:54: upload logo.png: 404 map[code:not_found detail:not found request_id:pN6uWTrsii379ogx status:404 title`
- TestProofingAnnotationsStayOnTheirVersion: `    attachment_storage_integration_test.go:82: upload: 404 map[code:not_found detail:not found request_id:3bYxqY_2p5ffIC-a status:404 title:Not Found `
- TestAttachmentsOnExternalStorageAndLinks: `    attachment_versions_integration_test.go:53: v1: 404 map[code:not_found detail:not found request_id:4SPPqPt-Ss03qyzl status:404 title:Not Found typ`
- TestAttachmentVersionsAreImmutableAndPolicyChecked: `    automation_integration_test.go:329: INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('space',$1,$2::principal_t`
- TestTriggersConditionsAndActions: `    blueprints_integration_test.go:16: create root: status 404, want 201 (map[code:not_found detail:not found request_id:esE0B4oLNqBVYP2G status:404 t`
- TestCustomFieldAccessRegression: `    custom_field_item_share_integration_test.go:45: Secret: status 404, want 201 (map[code:not_found detail:not found request_id:yyCfCVERj_K2Ecnl stat`
- TestRelationFieldRejectsHiddenTarget: `    custom_field_types_integration_test.go:123: create People field: status 404, want 201 (map[code:not_found detail:not found request_id:0IblTE3rGfXw`
- TestWidgetDataPresetsAndCustomQueries: `    dashboards_data_integration_test.go:182: admin stars the secret task: status 404, want 204 (map[code:not_found detail:not found request_id:hO3WmhY`
- TestStarsAndStarredWidget: `    endpoint_sweep_integration_test.go:268: 139 operations exercised with real ids; 54 have no real id in the fixture:`
- TestFormBranchingBlueprintItem: `    form_routing_integration_test.go:28: severity: status 404, want 201 (map[code:not_found detail:not found request_id:LXu3S8QaJNqu2CO3 status:404 ti`
- TestFormBranchesDecideQuestionsItemTypeAndFields: `    form_submission_scope_integration_test.go:76: create public link: status 404, want 200 (map[code:not_found detail:not found request_id:0Cu8ft03bhJ`
- TestSubmissionAuditNamesTheActor: `    form_submission_scope_integration_test.go:90: create public link: status 404, want 200 (map[code:not_found detail:not found request_id:zIJYQ0JNU51`
- TestPublicSubmissionCannotChooseAnotherDestination: `    forms_integration_test.go:84: create: 422 map[code:validation_failed detail:approvers must be able to see the destination errors:[map[field:respo`
- TestFormsDefinitionAndSettings: `    forms_integration_test.go:160: admin manages every form, ordered by Space then name: map[items:[map[access:private description:Ask IT id:01a1f346-`
- TestFormAccessAndManageList: `    forms_public_integration_test.go:74: create link: status 404, want 200 (map[code:not_found detail:not found request_id:hOjMR3Ui-f8eha4I status:404`
- TestPublicFormLink: `    forms_public_integration_test.go:222: build form: status 404, want 201 (map[code:not_found detail:not found request_id:TuIDBvgFyXbRoRKZ status:404`
- TestConcurrentPublicFormSubmissionsAllSucceedOnce: `    forms_public_integration_test.go:267: build form: status 404, want 201 (map[code:not_found detail:not found request_id:0e5n3AMMhldr0EGZ status:404`
- TestPublicConfirmationUsesTheFormsWording: `    formula_fields_integration_test.go:22: field Price: status 404, want 201 (map[code:not_found detail:not found request_id:sSITLseR6fJ1ynKP status:4`
- TestFormulaFieldsAreValidatedRecalculatedAndProtected: `    formula_fields_integration_test.go:128: formula over 500 items: 35.216792ms`
- TestListFilterAndSortByCustomField: `    item_field_list_integration_test.go:104: 500 items, 10 number fields, sort+filter by field: p50=45.976041ms p95=124.017292ms max=126.187208ms (one`
- TestItemTypeScope: `    items_integration_test.go:87: create: 404 map[code:not_found detail:not found request_id:GO9fycIq1DwrtF_1 status:404 title:Not Found type:about:bl`
- TestProjectProgressFollowsAuthorizedLiveLocations: `    qa_b3a_search_test.go:114: 500 tasks, scoped to project, limit 50 with ancestor paths: p50=11.418125ms p95=12.872166ms max=12.962208ms`
- TestRecycleRestoresCascadeAndMissingParent: `    recycle_purge_integration_test.go:42: delete e3943e54-d6a7-4b44-b48b-8b3919d63190: {status:404 body:map[code:not_found detail:not found request_id`
- TestEmptyRecycleBinPurgesOnlyWhatActorMayDelete: `    relation_fields_integration_test.go:51: admin relates to the secret item: status 422, want 200 (map[code:validation_failed detail:Blocks: choose i`
- TestRelationFieldsValidateTargetsAndDoNotLeakNames: `    report_digest_integration_test.go:99: the admin sees every Space:`
- TestReportSubscriptionsAreDeliveredOncePerSlotWithTheSubscribersRights: `    reports_integration_test.go:204: admin result = map[layout:column name:Counts result:map[rows:[map[break:<nil> group:map[key:`
- TestSSELatencyUnderClientLoad: `    sse_multiuser_integration_test.go:144: patch: {status:404 body:map[code:not_found detail:not found request_id:V9VlWkDjFKBYBSxJ status:404 title:No`
- TestSSETwoUsersLatencyOrderReconnectAndResync: `    time_entries_integration_test.go:70: create project view: status 404, want 201 (map[code:not_found detail:not found request_id:tnKchrID2jy8-5C2 st`

### 16.2 Gerbang lingkungan (10, BLOCKED_ENV)
- TestBlueprintSaveAndUse
- TestCustomFieldPeopleDirectoryAndLocation
- TestEventReplayRespectsCurrentAccess
- TestReplaceItemLocations
- TestItemLifecycle
- TestNotificationReplayChecksCurrentAccess
- TestProjectFiles
- TestReportResultAndExport
- TestProjectTimelog
- TestMyTodoOnlyReturnsVisibleAssignedTasks

## 17. Hasil akhir suite Go (putaran ini)
- FAIL 7, PASS 453. Hanya paket `httpserver`; paket lain lulus.
- BLOCKED_ENV (6): `PMS_TEST_BROWSER` (3), `PMS_TEST_SSE_HEARTBEAT`, `PMS_TEST_SCALE`, `PMS_TEST_PILOT_PERFORMANCE`. Tidak dihitung PASS.
- FLAKY_TIMING (1): `TestListFilterAndSortByCustomField`, p95 ≈ 120–130 ms terhadap ambang 300 ms pada mesin lokal. Belum ditentukan apakah ambang tidak andal atau ada regresi latensi nyata; perlu pengukuran ulang pada mesin tetap.
- Yang belum selesai: A1.6 (otorisasi approval), A1.7 (regresi keamanan lintas modul), workstream B–E.
- Rilis tetap NO-GO. Belum ada commit, push, atau deploy.

## 18. A1.6 Otorisasi approval (selesai)
- Aturan: keputusan hanya lewat baris `approval_approvers` milik aktor (assignment eksplisit). Admin Workspace atau pembaca item yang bukan approver tidak bisa memutuskan. Keputusan final tidak bisa diubah (409).
- Tes: `TestApprovalDecisionNeedsAnAssignment` (`approval_authorization_integration_test.go`) PASS: admin tanpa akses ditolak; admin dengan grant tapi bukan approver 403; pembaca bukan approver 403; approver yang ditugaskan 200; perubahan setelah final 409; tepat satu keputusan tercatat.
- Automation: tidak ada aksi automation yang memutuskan approval (tidak ditemukan di kode). Perubahan status tetap melewati guard workflow (tes status guard yang ada PASS pada suite penuh).
- Belum: workstream B (default workflow Task/Project), C (tab Custom Fields/Blueprints), D (delete impact preview, audit Calendar dan toolbar), E (E2E Playwright, build/lint final, OpenAPI check untuk endpoint baru, screenshot responsif).

## 19. Workstream B–E

### 19.1 B. Default workflow Space (Task dan Project)
- Migrasi `00347_space_default_workflows.sql` (aditif): tabel `space_default_workflows(space_id, base_type ∈ {task, project}, workflow_id)`.
- Urutan untuk root item: (1) `workflow_id` di request, (2) default Item Type, (3) default Space untuk tipe dasar, (4) default Workspace. Anak item mengikuti workflow induknya; menyebut `workflow_id` untuk anak ditolak 422.
- API: `GET/PUT /api/v1/spaces/{id}/default-workflows`. PUT butuh `edit` pada Space shared; workflow harus ada dan punya minimal satu status.
- Item yang sudah ada tidak berubah (diuji).
- UI: tab Workflows punya dua pemilih (Task dan Project) dengan daftar workflow yang sama.
- Tes: `TestSpaceDefaultWorkflowsPrecedenceAndPermissions` PASS; E2E `workflows tab saves the Space default for tasks` PASS.
- Belum: pemilih `workflow_id` belum ada di OpenAPI schema ItemCreate (hanya dokumentasi di sini).

### 19.2 C. Custom Fields dan Blueprints
- Custom Fields: tab memakai `CustomFieldRegistry` (komponen yang sama dengan pengaturan Workspace) dengan `spaceId` terkunci. Tidak ada duplikasi definisi; dialog lama tetap dipakai di header halaman Space.
- Blueprints: daftar blueprint Space dari endpoint yang ada (`GET /spaces/{id}/blueprints`), yang hanya mengembalikan ringkasan (nama, jumlah item, tipe item, versi). Tidak ada nilai field. Create dan edit tetap di halaman Blueprint.
- Masih terbuka: kebocoran field restricted pada blueprint (temuan SPC-13 di `docs/features/blueprint/BLUEPRINT_ANALISA.md`) belum diverifikasi ulang di sesi ini. Pemakaian (usage) per blueprint belum ditampilkan.
- Item type, automation, request forms, dan tools tetap memakai komponen existing (sudah di tab).

### 19.3 D. Delete impact dan Calendar/toolbar
- `GET /api/v1/spaces/{id}/delete-impact` (hanya baca): proyek dan folder, item hanya di Space ini, item yang juga ada di Space lain, jumlah anggota dan grup, rule automation, form permintaan, blueprint, dan tool. Hanya untuk yang berhak hapus; yang lain 403.
- Dialog hapus Space menampilkan rincian ini sebelum konfirmasi. Tes: `TestSpaceDeleteImpactCountsWhatTheDeleteTouches` PASS.
- Calendar duplikat: audit kode belum menemukan sumber duplikat di Space. Tabel tampilan Space hanya di-seed Table dan Board. Calendar hanya di-seed untuk project atau folder yang `default_view_type`-nya calendar, dan dapat ditambah manual. Kasus duplikat yang dimaksud belum bisa dikonfirmasi tanpa tangkapan layar halaman; status NOT VERIFIED.
- Pengelompokan toolbar: belum diubah.

### 19.4 E. Verifikasi
- Go: lihat hasil akhir di bawah (run penuh dengan database uji terisolasi).
- Vite build PASS. `npm test` (node) 96/96 PASS. ESLint: 5 error di `excel-destination-tree.tsx` dan `space-excel-import.tsx`. Kedua file dimodifikasi sesi lain dan tidak disentuh di sini.
- OpenAPI: ditambahkan path default-workflows dan delete-impact serta schema SpaceDefaultWorkflows. `schema.d.ts` dibangkitkan ulang.
- Migrasi: 00346 dan 00347 unik di folder kerja. Cek bentrok ulang sebelum merge.

### 19.5 Penutupan item terbuka (putaran ini)
- B, OpenAPI: `workflow_id` ditambahkan ke ItemCreate (api/openapi.yaml), `schema.d.ts` dibangkitkan ulang.
- C, SPC-13 (kebocoran field restricted dan sumber lintas Space): VERIFIED dengan tes. 22 tes blueprint PASS termasuk `TestBlueprintNeverCopiesRestrictedFields`, `TestBlueprintReadReturnsTreeAndHidesRestrictedFields`, `TestBlueprintSaveRejectsItemsOutsideTargetSpace`, `TestBlueprintRefusesItemsWithTheirOwnShares`. Satu tes browser dilewati (PMS_TEST_BROWSER).
- C, jumlah pemakaian: migrasi `00348_blueprint_uses.sql` (tabel `blueprint_uses`, satu baris per pemakaian sukses, dalam transaksi yang sama dengan pembuatan item). Ringkasan blueprint berisi `uses`. Tes `TestBlueprintUsesAreCounted` PASS (pemakaian ditolak tidak dihitung).
- D, validasi dependensi: Space terkunci tidak bisa dihapus (422, "unlock this Space before deleting it"). Tes `TestLockedSpaceCannotBeDeletedWhileLocked` PASS.
- D, duplikat Calendar: masih NOT VERIFIED (butuh tangkapan layar halaman Space yang berjalan; server lokal di 8080 adalah biner lama).
- D, pengelompokan toolbar: belum diubah.
- Migrasi baru: 00348. Nomor 00346, 00347, 00348 unik di folder kerja; cek bentrok ulang sebelum merge.

### 19.6 Pengelompokan toolbar Space (D)
- Header Space sekarang: New item (utama) | Share, Bookmarks | Settings | menu admin (⋯). Tombol Forms, Automation, dan Item types di header dihapus karena sudah menjadi tab di Settings (tidak ada duplikat).
- Form permintaan di tab Settings sekarang punya tombol "New form" sendiri (sebelumnya hanya ada di halaman terpisah).
- Tes E2E `forms.spec.ts` (list shows forms…) diubah: navigasi lewat Settings lalu tab Request forms, bukan tautan header yang sudah dihapus.
- Verifikasi: Playwright forms, spaces, space-settings, automation-settings, custom-fields, m8, requests, my-requests, batch5-visual: lulus (forms 19/19 setelah perubahan). Tangkapan layar header setelah perubahan: `/private/tmp/claude-501/space-header-after.png` (mock, desktop 1280px). Tangkapan sebelum perubahan tidak diambil.
- Belum: penyamaan dengan toolbar halaman Project dan Folder belum diperiksa.

### 19.7 Toolbar Project dan Folder (D)
- Project: tetap [avatar] [Info] [Share] [Add task], dengan menu ⋯ untuk layar kecil.
- Folder sebelumnya hanya punya Info. Sekarang Folder punya Info, Share (jika berhak share), dan New item (jika berhak create). Dialog share dan create kini juga dibuka untuk folder; create tetap task-only hanya untuk project.
- Header Space diurutkan seperti Project dan Folder: Share, Bookmarks, Settings, New item (utama, paling kanan), lalu menu admin ⋯.
- Verifikasi: Playwright project/folder/space/forms/settings lulus (27 + 48). Tangkapan layar Folder dan Project belum diambil (butuh data mock item yang lengkap).

### 19.8 Tangkapan layar toolbar Folder dan Project (E, mock)
- Lokasi: `/private/tmp/claude-501/shots/` (folder-desktop, folder-phone, project-desktop, project-phone).
- Desktop 1280px: Folder = Info | Share | New item. Project = Info | Share | Add task. Keduanya berurutan sama, aksi utama di kanan.
- Phone 390px: tidak ada overflow horizontal (0 px) di keempat tangkapan. Folder menampilkan tiga aksi langsung. Project menyembunyikan Info dan Share di menu ⋯ (perilaku yang sudah ada).
- Temuan: label KPI progres Project terpotong ("Work items …", "In progress …") di desktop dan phone. Belum diperbaiki.
- Belum: audit keyboard dan fokus, dark mode, dan tablet.

### 19.9 Audit keyboard dan fokus (E)
- Tes baru `web/tests/space-keyboard-focus.spec.ts` (4 tes, PASS):
  - tab settings Space dapat dijangkau keyboard dan memiliki focus ring;
  - dialog "Leave without saving?" menjebak fokus (Tab tetap di dalam dialog) dan Escape menutupnya;
  - axe tidak menemukan pelanggaran serius/kritis di Space Settings (general, workflows) dan registry;
  - urutan aksi header Space: Share, Bookmarks, Settings, New item.
- Temuan dokumen: urutan header di bagian 19.6 sempat ditulis Bookmarks dulu. Kode dan tes memakai Share dulu; dokumen sudah diperbaiki.
- Belum: audit keyboard seluruh halaman, dark mode, tablet, dan verifikasi pembaca layar.

### 19.10 Label KPI Project (perbaikan)
- Penyebab "truncated": bukan pemotongan CSS. Strip menampilkan "…" selama query hitungan belum selesai; di mock query itu tidak dijawab, jadi placeholder terlihat seperti terpotong. Dengan data, keempat label muat dalam satu baris.
- Perbaikan nyata: jika query hitungan gagal, placeholder sebelumnya tetap "…" selamanya tanpa tanda kesalahan. Sekarang tampil "—" saat gagal (`project-header.tsx`).
- Tes baru `web/tests/project-kpi.spec.ts` (2 tes, PASS): angka tampil saat berhasil; "—" saat gagal, dan tidak pernah "…".

### 19.11 Dark mode dan tablet
- Cakupan: Space (ringkasan), Settings > General, dan Settings > Spaces (registry), masing-masing pada tema terang dan gelap, lebar tablet 768x1024 dan desktop 1280x800. Ditambah header Project dan Folder pada tablet gelap.
- Hasil awal: Settings > General pada tablet melebar 8px (`document.documentElement.scrollWidth` 776 > 768) di kedua tema. Penyebabnya bar simpan sticky (`SettingsSaveBar`, `settings/settings-ui.tsx`): margin negatif `sm:-mx-8` lebih lebar dari padding workspace `sm:p-6`, sehingga tepi kanan bar keluar 8px. Tab strip tidak menyebabkan ini karena sudah scroll di dalam `nav overflow-x-auto`.
- Perbaikan: `sm:-mx-8 sm:px-8` menjadi `sm:-mx-6 sm:px-6`. Hanya Space settings yang memakai bar ini, jadi tidak ada halaman lain yang berubah.
- Catatan: pemeriksaan awal menyebut overflow 8px di halaman Space. Pada pengukuran ulang halaman Space bersih (0px), jadi tidak terbukti di sana.
- Hasil setelah perbaikan: 13/13 PASS (overflow 0 di semua kombinasi; axe tanpa pelanggaran serius/kritis; latar `#f9fafb` terang, `#111827` gelap). `tests/space-settings.spec.ts` 10/10 PASS.
- Tangkapan layar: `/private/tmp/claude-501/shots/` (space-*, settings-general-*, registry-*, project-dark-tablet.png).
- Spec sementara `web/tests/zz-theme-tablet.spec.ts` sudah dihapus. Belum: pemeriksaan visual manual di lebar phone dan tema gelap pada Calendar.
- Tidak ada commit, push, atau deploy. Status rilis tetap NO-GO.

### 19.12 Verifikasi lengkap pada commit yang di-push (037c60e)
- Pengukuran pada worktree bersih di commit `037c60e` (tip `feat/m12c-ui-clean` setelah push), dan pada working tree.
- Go test (`go test -count=1 ./server/...`, DB terisolasi port 55470):
  - Commit `037c60e`: GAGAL. 137 tes gagal di 4 paket: `connectors`, `database`, `httpserver`, `mail`. Penyebab utama: perubahan kebijakan A1 ikut ter-commit, tetapi pembaruan tes dan fixture yang menyertainya belum (mis. `TestApprovals`, `TestCommentLifecycle` mendapat 404 saat membuat item).
  - Working tree: 22 paket OK, 4 tes gagal di `httpserver` (`TestEventReplayRespectsCurrentAccess`, `TestListFilterAndSortByCustomField`, `TestNotificationReplayChecksCurrentAccess`, `TestMyTodoOnlyReturnsVisibleAssignedTasks`), belum ditelusuri.
- Web typecheck (`tsc -b`):
  - Commit `037c60e`: GAGAL, 9 error TS (`space-automation.tsx` 5, `blueprints.tsx` 1, `spaces.$spaceId.tsx` 1 (`SpaceExcelExport` tidak diekspor), `tasks.$itemId.tsx` 2).
  - Working tree: lulus (0 error).
- Lint (`eslint src`): lulus di kedua kondisi (exit 0).
- Playwright (`space-settings`, `space-keyboard-focus`, `project-kpi`, `spaces`):
  - Commit `037c60e`: 0 dari 35 lulus.
  - Working tree: 35 dari 35 lulus.
- Status rilis: NO-GO. Commit yang di-push belum konsisten dan tidak lulus tes maupun typecheck. Perlu commit susulan yang melengkapi kode dan tes, atau keputusan untuk menarik commit itu dari remote.

### 19.13 Verifikasi pada tip remote terbaru (c28da4d)
- Worktree bersih di `origin/feat/m12c-ui-clean` = `c28da4d`. Tidak ada commit atau push.
- Build dan `go vet ./...`: LULUS.
- Go test penuh: 3 tes gagal dari 23 paket.
  - `database/TestItemLocationRejectsCycle`: admin tidak mendapat akses lewat Space bersama. Versi working tree dari `items_integration_test.go` (pembaruan A1 yang belum di-commit) membuatnya LULUS.
  - `mail/TestNotificationWorkerHonoursEmailPreferences`: 0 email, harusnya 5. Versi working tree dari `notification_integration_test.go` membuatnya LULUS.
  - `httpserver/TestEveryProtectedEndpointChecksTheSessionAndSurvivesUnknownIds`: 404 untuk `/automation-rules/{id}/duplicate`, `/spaces/{id}/excel-exports`, `/spaces/{id}/workspace-automation`. Rute ini berasal dari commit automation (`6eee477`) dan excel yang belum ada di remote.
- Typecheck (`tsc -b`): GAGAL, 9 error. `space-automation.tsx` mengimpor `automation-rule-list` dan `workspace-automation` (tidak ter-track), dan `EditorStep` dari `automation-editor`.
- Lint: LULUS.
- Playwright (`space-settings`, `space-keyboard-focus`, `project-kpi`, `spaces`): 0 dari 35 lulus. Halaman Space gagal dimuat karena `SpaceExcelExport` tidak diekspor oleh `project-excel-export.tsx`, dan karena modul automation di atas.
- Status rilis: NO-GO. Commit Space di remote belum lengkap: butuh pembaruan tes A1 dan modul web dari sesi automation dan excel.
