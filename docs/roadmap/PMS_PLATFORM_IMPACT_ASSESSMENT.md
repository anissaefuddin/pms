# PMS Platform — Impact Assessment (B0-03: inventaris permukaan akses)

| | |
|---|---|
| **Dokumen** | Impact assessment. Bagian ini hanya inventaris permukaan akses (B0-03). Bagian data inventory (B0-04) dan pemetaan klien (B0-05) **belum diisi** |
| **Tanggal** | 27 November 2026 |
| **Mode** | Read-only. Tidak ada kode, tes, migrasi, data, deploy, commit, atau push yang diubah |
| **Rencana acuan** | [Rencana MVP v0.2](LENS_MVP_IMPLEMENTATION_PLAN.md) §5 B0-03 dan B3-01..22 |
| **Status B0-03** | **PARTIAL.** Permukaan utama sudah dipetakan dengan bukti. Beberapa jalur ditandai NEEDS_TRACE (§10.1) |
| **Pertanyaan tidak terjawab** | Apakah inventaris ini lengkap: tidak. Jumlah rg bukan bukti cakupan penuh |

B0-03 selesai tidak berarti isolasi klien sudah dibangun, diuji, atau disetujui.

## 1. Ruang lingkup dan HEAD

| Item | Nilai | Catatan |
|---|---|---|
| Branch | `docs/restructure-mvp-roadmap` | Sama dengan Prompt #001 |
| HEAD | `a8477f4` | Sama dengan Prompt #001 (rencana menyebut `c4f7afc`, yang kini satu commit di belakang) |
| Release dasar | `v0.13.0` = `72f9902` | Ancestor HEAD |
| Migrasi terakhir di kode | `00362_seed_runs.sql` | Ditambahkan `fdff421` |
| Working tree (awal) | Tidak ada berkas tracked berubah. Berkas tak-ter-track sama seperti sebelumnya + `docs/qa/releases/v0.14.0/` (CANDIDATE.md, dari Prompt #001) | Dicatat sebelum audit |
| Working tree (akhir) | Lihat §10.3 | — |

### 1.1 Verifikasi path di rencana

| Path di rencana | Ada? | Lokasi nyata |
|---|---|---|
| `policy` (engine, item, share, locked) | Ya | `server/internal/policy/` (`engine.go`, `item.go`, `share_paths.go`, `locked.go`, `field_access.go`, `contract.go`, `policy.go`) |
| Item service | Ya | `server/internal/items/` (~60 berkas non-tes) |
| Space service | Ya | `server/internal/spaces/` (`spaces.go`, `item_shares.go`, `governance.go`, `members.go`, `registry.go`, `delete_impact.go`) |
| HTTP handler | Ya | `server/internal/httpserver/` (~90 berkas non-tes, `server.go` = router) |
| Worker | Ya | `automation/`, `duereminders/`, `reportdigest/`, `connectors/worker.go`, `mail/`, `webhooks/`, `housekeeping/`, `previews/`, `httpserver/approval_deadlines.go`, `realtime/` |
| Migrasi | Ya | `server/migrations/` (berkas `.sql` hingga `00362`, `embed.go`) |
| `spaceRows` | Ya, sebagai konstanta SQL | `policy/engine.go:57` |
| `itemsCaps` | Ya | `policy/item.go:51` |
| `visibleSpaces` | Ya | `policy/engine.go:80` (`VisibleSpaces` di `:71`) |
| `SpaceAccessFor` | Ya | `policy/engine.go:153` |
| `CanOnSpace` | Ya | `policy/engine.go:168` |
| `share_paths` | Ya, sebagai berkas | `policy/share_paths.go` (fungsi `SharedOnEveryPath` :13) |
| `policyCache` | Ya, tetapi **bukan cache otorisasi** | `httpserver/access_policy.go:33` (IP allowlist dan OTP, TTL, `forgetPolicy`) |

**Temuan 1.1:** rencana mengasumsikan ada cache keputusan akses (B3-07: "invalidasi cache otorisasi"). Di kode, `policyCache` hanya menyimpan `access_policy` (IP allowlist dan OTP login). Tidak ada cache keputusan Space atau item, jadi tidak ada yang perlu diinvalidasi saat membership atau owner berubah. Ini memengaruhi B3-07: rumusan task perlu diperbarui sebelum B3.

## 2. Entry point otorisasi

Metode: pencarian nama fungsi dengan `rg`, lalu membaca badan fungsi. Kolom "Kandidat enforcement klien" = titik yang akan dipasangi gerbang B3 (bukan rancangan).

| ID | File:fungsi | Tanggung jawab | Sumber data/grant | Kandidat enforcement klien | Task B3 |
|---|---|---|---|---|---|
| AUTH-001 | `policy/engine.go:57` `spaceRows` (konstanta SQL) | Daftar Space hidup dengan peran share yang sampai ke aktor (langsung atau via grup) | `spaces`, `shares` (target space), `group_members`, `custom_roles`, `space_admins` | **Ya.** Satu-satunya SQL yang menghubungkan share ke aktor untuk keputusan Space. Gerbang membership klien harus di sini atau sebelum union | B3-04 |
| AUTH-002 | `policy/engine.go:80` `visibleSpaces` / `:71` `VisibleSpaces` | Membangun daftar `SpaceAccess` dari `spaceRows`, lalu `spaceCaps` | AUTH-001 | **Ya** | B3-04 |
| AUTH-003 | `policy/engine.go:125` `spaceCaps` | Aturan Space per ARCHITECTURE §6.2: admin, owner, archived, kind | Tipe Space, peran, `user_type` | Tidak (aturan Space, bukan klien) | — |
| AUTH-004 | `policy/engine.go:153` `SpaceAccessFor` | Satu Space untuk satu aktor; dipakai banyak pemanggil di luar policy (lihat §3) | AUTH-002 | **Ya** (titik panggilan paling banyak) | B3-04 |
| AUTH-005 | `policy/engine.go:168` `CanOnSpace` | Keputusan Found/Forbidden/Allowed untuk satu kebutuhan Capability | AUTH-004 | **Ya** | B3-04 |
| AUTH-006 | `policy/item.go:51` `itemsCaps` (dibungkus `ItemCaps`, `ItemCapsInSpace`, `ItemsCaps`, `RecycleItemsCaps`) | Union cap semua lokasi item: rekursi ke atas (`ancestry`), target `space` + `item` dari `shares`, `space_admins` di Space shared, personal Space = Full | `item_locations`, `items`, `spaces`, `shares`, `space_admins`, `group_members` | **Ya, titik paling sentral.** Gerbang per lokasi harus di dalam CTE `ancestry` **sebelum** `caps[id] |=` | B3-05 |
| AUTH-007 | `policy/item.go:103` `formItemsCaps` | Aktor formulir: `formCaps` hanya di Space formulir | `FormGrant` | Ya, perlu aturan owner formulir | B3-12 |
| AUTH-008 | `policy/item.go:126` `VisibleItemSpace` | Satu Space tampak untuk item | AUTH-006 | Ya | B3-05 |
| AUTH-009 | `policy/share_paths.go:13` `SharedOnEveryPath` | Untuk assignment: aktor harus punya Share di **setiap** jalur dari item ke Space (aturan DEC-06 Rev.1) | `item_locations` (satu Space), `shares`, `space_admins` | Ya. Tidak boleh tersaring oleh gerbang klien sehingga menghapus jalur restriktif | B3-06 |
| AUTH-010 | `policy/locked.go:14` `LockedAllows` | Space terkunci: hanya share tertentu yang diizinkan | `spaces.locked`, `shares` | Tidak langsung (kontrol kunci) | B3-11 |
| AUTH-011 | `policy/locked.go:40` `LockedSpacesOf` / `:59` `IsSpaceAdmin` | Kunci dan admin Space | `spaces`, `space_admins` | Tidak | — |
| AUTH-012 | `policy/field_access.go:17` `HiddenCustomFieldIDs` | Field yang disembunyikan per aktor (CF-D03) | `custom_field_grants` | Tidak (di luar klien) | — |
| AUTH-013 | `items/create.go:571` `CanReassign` → `:577` `CanReassignMany` | Assignment pada PATCH: reassign orang lain butuh Full di semua lokasi relevan. Memanggil `policy.SharedOnEveryPath` (`create.go:602`) | AUTH-009 + SQL sendiri untuk daftar Space shared | Ya. Pastikan gerbang B3 tidak mengubah daftar lokasi di sini | B3-06 |
| AUTH-014 | `items/create.go:522` `AuthorizeAssignments` → `:559` `CanAssignOthers` | Assignment pada create (`POST /items`). Memanggil `SharedOnEveryPath` (`create.go:559`) | AUTH-009 | Ya | B3-06 |
| AUTH-015 | `items/create.go:444` `addInitialShares` → `policy.LockedAllows` | Share awal saat create | AUTH-010 | Ya | B3-11 |
| AUTH-016 | `items/create.go:153` `createWithCustomFields` → `policy.RoleCaps`, `policy.Share` | Validasi peran share saat create | `policy/policy.go` | Tidak | B3-03 |
| AUTH-017 | `items/location.go:84` `replaceLocations` → `policy.Edit`, `IsSpaceAdmin`, `ItemCaps`, `LockedSpacesOf` | Ganti lokasi (move, tambah, hapus) | AUTH-006, AUTH-011 | **Ya** | B3-08 |
| AUTH-018 | `items/patch.go:86` `patch` → `policy.ItemCaps` (`:87`) | PATCH item: butuh View, lalu Edit atau ChangeStatus | AUTH-006 | Ya (lewat AUTH-006) | B3-05 |
| AUTH-019 | `httpserver/access_policy.go:33` `policyCache`, `:40` `loadPolicy` | IP allowlist dan kebijakan OTP | `access_policy` | **Tidak.** Bukan cache otorisasi (lihat §1.1) | — |

**Tidak ditemukan:** tidak ada pemanggilan `policy.` di badan fungsi tulis `applyFields` (`items/patch.go:222`), `setAssignees` (`patch.go:404`), `syncRelations` (`custom_field_types.go:341`), `reschedule` (`scheduling.go:163`), `updateAllocations` (`effort.go:18`), `SetCustomFieldValuesTx` (`custom_fields.go:537`), `restoreStatusAndFields` (`blueprints.go:596`). Keputusan ada di pemanggil (§4). Ini bukan cacat yang terbukti; ini jalur yang membutuhkan trace (NEEDS_TRACE).

## 3. Akses langsung ke basis data

### 3.1 Metode

- Pola pencarian: `(FROM|JOIN|INTO|UPDATE|DELETE FROM|REFERENCES) +<tabel>\b` pada `server/internal/**/*.go`.
- Dikecualikan: `_test.go`, `internal/apigen/`, `internal/dbgen/` (kode generate).
- `server/queries/` hanya berisi `foundation.sql` (4 pernyataan). Tidak satu pun menyebut tabel ini.
- Pola kata tunggal (`\bitems\b`) **tidak dipakai** untuk hitungan final karena `items` juga nama paket dan variabel Go. Hitungan kata tunggal lebih tinggi karena itu.
- Hitungan di sini adalah **jumlah berkas**, bukan jumlah pernyataan SQL. Berkas yang menyebut tabel bisa hanya membaca melalui fungsi `policy`.

### 3.2 Hitungan

| Tabel | Berkas non-tes | Di `policy/` | Di luar `policy/` | Berkas dengan pola tulis |
|---|---:|---:|---:|---:|
| `spaces` | 32 | 3 | 29 | 5 |
| `items` | 63 | 1 | 62 | 16 |
| `item_locations` | 49 | 3 | 46 | 7 |
| `shares` | 20 | 4 | 16 | 7 |
| `item_assignees` | 17 | 0 | 17 | 2 |
| `item_relations` | 3 | 0 | 3 | 1 |
| `item_dependencies` | 7 | 0 | 7 | 3 |

Catatan pembacaan: hitungan ini **tidak** membuktikan bahwa berkas di luar `policy/` melewati otorisasi atau tidak. Banyak handler memanggil paket `items` atau `spaces` yang lalu memanggil `policy`. Label per berkas ada di §3.3.

### 3.3 Label per kelompok

| ID | Lokasi | Label | Catatan |
|---|---|---|---|
| DA-001 | `items/*.go` (fungsi baca: `read.go`, `list.go`, `search.go`, `portfolio.go`, `todo.go`, `stream.go`) | `CENTRAL_POLICY` / `INDIRECT_POLICY` | Diverifikasi: `Search`, `Portfolio`, `MyTodo`, `Stream` memanggil `policy.ItemsCaps`. `Get` → `readDetail` → `readDetails` → `policy.ItemsCaps` (`read.go:22/35/38`). `List` → `visibleContainer` → `policy.SpaceAccessFor` dan `policy.ItemCapsInSpace` (`list.go:155–180`) |
| DA-002 | `items/*.go` (fungsi tulis, §4) | `CENTRAL_POLICY` untuk sebagian, `INDIRECT_POLICY` atau `NEEDS_TRACE` untuk sisanya | Lihat §4 |
| DA-003 | `spaces/*.go` | `CENTRAL_POLICY` | `spaces.go` memakai `CanOnSpace` (`:236`, `:308`, `:398`) dan `SpaceAccessFor` (`:112`, `:199`). `item_shares.go:25` memakai `ItemCaps` dan `policy.Share` |
| DA-004 | `httpserver/*.go` yang memakai SQL langsung ke tabel | `NEEDS_TRACE` per berkas | Sebagian besar handler memanggil paket domain. Berkas yang tidak lewat paket domain dan menulis SQL langsung: `admin.go` (`shares` :237, :428), `auto_approval.go` (`items` :66), `workflows.go` (`items` :418, :487), `scim.go` (`shares` :1005), `create_menu.go` (`spaces` :107), `server.go` (`spaces` :678 `acceptInvitation`), `form_submit.go`, `forms.go` (`BeginTx` :259) |
| DA-005 | `bootstrap/bootstrap.go` (`spaces` :72, `shares` :73) | `SYSTEM_ACTOR` | Pembuatan instance; bukan jalur pengguna |
| DA-006 | `housekeeping/housekeeping.go` (`shares` :94, `DELETE FROM shares … expired`) | `SYSTEM_ACTOR` | Penghapusan share kedaluwarsa dengan SQL langsung. Risiko rendah; perlu aturan retensi, bukan gerbang akses |
| DA-007 | `connectors/pilot.go` (`items` writeItem :51) | `SYSTEM_ACTOR` (dengan aktor pembuat instalasi) | `ItemsCaps` pada `pilot.go:211` |
| DA-008 | `automation/*.go` | `SYSTEM_ACTOR` (aktor = pemilik rule) | Aktor dibentuk di `automation/engine.go:281`; izin diperiksa ulang dengan `CanOnSpace` di `automation/rules.go:171` |
| DA-009 | `httpserver/forms_public.go` | `PUBLIC_TOKEN` | Aktor `policy.External` dari token (`forms_public.go:173`) |
| DA-010 | `httpserver/dashboards_public.go` | `PUBLIC_TOKEN` | Dashboard dihitung dengan hak **pembuatnya** (`creator policy.Actor`, dibaca dari DB, `:182–200`) |
| DA-011 | `httpserver/gantt_snapshots.go` `getPublicGanttSnapshot` (`:211`) | `PUBLIC_TOKEN` | Membaca payload snapshot yang dibekukan saat dibuat; tidak ada `policy.` di badan fungsi (dibenarkan, lihat §5) |
| DA-012 | `analytics/analytics.go` (`items` lewat `item_locations`) | `CENTRAL_POLICY` (mode regular) / `NEEDS_TRACE` (mode operasional) | Mode regular memanggil `policy.ItemsCaps` (`:397`). Mode operasional (`operational=true`, `:371`) **melewati** gerbang per item dan hanya menyaring grup dengan `MinGroupItems` (`analytics.go:364–371`) |
| DA-013 | `projectexcel/export.go` (`ItemsCaps` :140, `ItemCapsInSpace` :73) | `CENTRAL_POLICY` | — |
| DA-014 | `overview/*.go` (`overview.go:103` `scope`) | `INDIRECT_POLICY` | Memanggil `policy.VisibleSpaces` |
| DA-015 | `mail/notification.go` (`ItemCaps` :58, `SpaceAccessFor` :60) | `CENTRAL_POLICY` | Pemeriksaan per penerima |
| DA-016 | `reportdigest/reportdigest.go` (`SpaceAccessFor` :128; aktor :100) | `CENTRAL_POLICY` (aktor = pemilik langganan) | Lihat JOB-003 |
| DA-017 | `duereminders/duereminders.go` (`ItemCaps` :112; aktor :106) | `CENTRAL_POLICY` (aktor = penerima pengingat) | — |
| DA-018 | `webhooks/webhooks.go` (`ItemCaps` :373, aktor pemilik webhook) | `CENTRAL_POLICY` | Pemeriksaan per penerima event |
| DA-019 | `seed/seed.go:90` | `SYSTEM_ACTOR` (`Admin`) | Hanya berjalan saat `PMS_SEED` (MVP-03), sekali per database |

Hitungan "berkas di luar `policy/`" pada §3.2 mencampur kelompok di atas. Yang **bukan** `CENTRAL_POLICY` dan belum di-trace: `httpserver` SQL langsung (DA-004), `analytics` mode operasional (DA-012), `housekeeping` (DA-006), dan fungsi tulis di §4 yang bertanda NEEDS_TRACE.

## 4. Permukaan tulis

Diverifikasi per fungsi: `file:line`, jalur HTTP, guard di badan atau pemanggil, dan transaksi. Operasi tulis sudah dicari dengan pola `(INTO|UPDATE|DELETE FROM) <tabel>` per fungsi.

| Surface ID | Operasi | Jalur HTTP | File:fungsi | Guard | Transaksi | Label | Task | Tes |
|---|---|---|---|---|---|---|---|---|
| WRITE-001 | Buat item | `POST /api/v1/items` (`server.go:224`) → `createItem` (`items.go:106`) | `items/create.go:32` `Create` → `createWithCustomFields` (`:153`), `:361` (items), `:401` (item_locations), `:461` (shares) | `locationCaps` (`create.go:236`, `:550`, `:626`), `CanAssignOthers`/`AuthorizeAssignments` (`:522`), `LockedAllows` (`:455`) | `Serializable` | `CENTRAL_POLICY` | B3-03 | `items/assign_error_test.go`, `httpserver/access_policy_integration_test.go` |
| WRITE-002 | Ubah item (field, status, assignee) | `PATCH /api/v1/items/{itemId}` (`:229`) → `patchItem` (`items.go:165`) → `items.Patch` | `items/patch.go:50` `Patch` → `patch` (`:86`): `ItemCaps` (`:87`), `ChangeStatus`/`Edit` (`:98–100`); `applyFields` (`:222`, `items`) tanpa guard di badan; `setAssignees` (`:404`) dipanggil setelah `requireReassign` (`:374`) → `CanReassign` (`create.go:648`) | `ItemCaps`; reassign: `SharedOnEveryPath` (AUTH-013) | `Serializable` | `CENTRAL_POLICY` (pemanggil); `applyFields`/`setAssignees` `INDIRECT_POLICY` | B3-05, B3-06 | `httpserver/assignment_authority_integration_test.go`, `items/*` |
| WRITE-003 | Ganti lokasi, move | `PUT /api/v1/items/{itemId}/locations` (`:235`) → `replaceItemLocations` (`items.go:150`) → `items.ReplaceLocations` (`location.go:68`) → `replaceLocations` (`:84`) | `policy.Edit`, `ItemCaps`, `IsSpaceAdmin`, `LockedSpacesOf` (`:84`); tulis `item_locations` (`:183`) dan `items` (`:258`) | `CENTRAL_POLICY` | `Serializable` | `CENTRAL_POLICY` | B3-08 | `httpserver/move_*_integration_test.go` (`move_race_integration_test.go`, `move_parent_child_regression_integration_test.go`, `move_audit_integration_test.go`) |
| WRITE-004 | Urutkan ulang (reorder) | `POST /api/v1/items/{itemId}/reorder` (`:234`) → `reorderItem` (`items.go:136`) → `items.Reorder` (`reorder.go:20`) → `reorder` (`:41`) | `policy.Edit`, `ItemCaps`, `ItemCapsInSpace`, `View` (`:41`); tulis `item_locations` (`:133`), `items` (`:138`) | `CENTRAL_POLICY` | `Serializable` + `FOR UPDATE` | `CENTRAL_POLICY` | B3-08 | `httpserver/move_*` |
| WRITE-005 | Hapus item (ke recycle bin) | `DELETE /api/v1/items/{itemId}` (`:230`) → `deleteItem` (`items.go:126`) → `items.Delete` (`delete.go:12`) | `policy.Delete`, `ItemCaps`, `View` (`delete.go:12`); tulis `item_locations` (`:52`), `items` (`:63`) | `CENTRAL_POLICY` | `Serializable` | `CENTRAL_POLICY` | B3-08 | — |
| WRITE-006 | Pulihkan dari recycle bin | `POST /api/v1/recycle-bin/{itemId}/restore` (`server.go:233`) → `restoreItem` (`recycle.go:34`) → `items.Restore` (`items/recycle.go:215`) | `RecycleItemsCaps` (`:234` / `:96`), `View`; tulis `item_locations` (`:250`), `items` (`:266`) | `CENTRAL_POLICY` | `Serializable` + `FOR UPDATE` | `CENTRAL_POLICY` | B3-08 | `httpserver/recycle_integration_test.go`, `recycle_purge_integration_test.go` |
| WRITE-007 | Hapus permanen (purge) | `items.Purge` dipanggil dari `httpserver/recycle.go:46` | `items/purge.go:13`: `policy.Delete`, `RecycleItemsCaps`, `View`; tulis `item_locations` (`:44`), `items` (`:46`) | `CENTRAL_POLICY` | `Serializable` | `CENTRAL_POLICY` | B3-08 | — |
| WRITE-008 | Share item | `PUT /api/v1/items/{itemId}/shares` (`server.go:223`) → `itemShares` (`items.go:16`) → `spaces.ItemShares` (`spaces/item_shares.go:16`) | `ItemCaps`, `policy.Share`, `View` (`item_shares.go:16–25`); tulis `shares` (`:63`) via `items` | `CENTRAL_POLICY` | `Serializable` | `CENTRAL_POLICY` | B3-11 | `httpserver/item_shares_integration_test.go`, `item_share_revoke_audit_integration_test.go`, `custom_field_item_share_integration_test.go` |
| WRITE-009 | Share Space (ganti) | `PUT /api/v1/spaces/{spaceId}/shares` (`spaces.go:41`) → `replaceSpaceShares` (`spaces.go:256`) → `spaces.ReplaceShares` (`spaces/spaces.go:416`) | `CanOnSpace(Share)` (`:416`), `policy.Admin`; `FOR UPDATE`; tulis `shares` via `writeTargetShares` (`:557`, `:473`) | `CENTRAL_POLICY` | `FOR UPDATE` (transaksi penuh) | `CENTRAL_POLICY` | B3-11 | `space_members_integration_test.go`, `space_governance_integration_test.go` |
| WRITE-010 | Undang user / grup (admin) | `httpserver/admin.go:237` `adminInvite`, `:428` `adminGroup` | `admin.go:251` `LockedAllows`; tulis `shares` langsung | `NEEDS_TRACE` (guard admin di handler, bukan paket policy) | tidak teridentifikasi cepat | `NEEDS_TRACE` | B3-11 | `invitation_integration_test.go` (`mail/`) |
| WRITE-011 | Gabung Space | `POST /api/v1/spaces/{spaceId}/join` (`spaces_governance.go:19`) → `spaces/governance.go:339` `Join` | `policy.Admin` (`:339`); tulis `shares` (`:376`), `spaces` (`:379`) | `CENTRAL_POLICY` (aturan "Admin" tanpa pengecekan klien) | — | `CENTRAL_POLICY` | B3-04 | `space_members_integration_test.go` |
| WRITE-012 | Buat/ubah/hapus/arsip Space | `POST /api/v1/spaces` (`spaces.go:21`), `PATCH`/`DELETE /spaces/{id}`, `POST …/archive` | `CreateTx` (`spaces.go:169`: `policy.Ceiling`, `policy.SpaceAccessFor`), `Update` (`:218`: `CanOnSpace(Edit)`), `Delete` (`:299`: `CanOnSpace(Delete)`), `SetArchived` (`governance.go:75`) tanpa guard di badan | `CENTRAL_POLICY` (kecuali `SetArchived` = `NEEDS_TRACE`) | B3-03 (klien di Space baru) | `CENTRAL_POLICY` | B3-03, B3-04 | `spaces_integration_test.go`, `space_delete_impact_integration_test.go`, `space_governance_integration_test.go` |
| WRITE-013 | Dependensi (buat/ubah/hapus) | `POST /api/v1/items/{itemId}/dependencies` (`dependencies.go:13`) → `createDependency` → `items.CreateDependency` (`dependencies.go:93`); `PATCH`/`DELETE /dependencies/{id}` | `CreateDependency`: `policy.Edit`, `ItemCaps` (`:106`). `UpdateDependency` (`:157`) dan `DeleteDependency` (`:232`) tanpa `policy.` di badan | `CENTRAL_POLICY` (create); `NEEDS_TRACE` (update/delete) | Lintas owner harus ditolak di ketiganya (B3-10) | `CENTRAL_POLICY` / `NEEDS_TRACE` | B3-10 | `httpserver/dependencies_integration_test.go` |
| WRITE-014 | Relasi field (relation) | Lewat `SetCustomFieldValuesTx` (`custom_fields.go:537`) → `syncRelations` (`custom_field_types.go:341`) menulis `item_relations` | `SetCustomFieldValuesTx` dan `syncRelations` tanpa `policy.` di badan; diterima dari `patch.go` dan `create.go` | `INDIRECT_POLICY` (lewat `patch`/`create`); `NEEDS_TRACE` untuk jalur lain | Relasi lintas owner ditolak (B3-10) | `INDIRECT_POLICY` | B3-10 | — |
| WRITE-015 | Jadwal ulang (reschedule) | Dari `patch` (`patch.go:185`) dan `DeleteDependency` (`dependencies.go:221`) → `reschedule` (`scheduling.go:163`) menulis `items` | Tanpa `policy.` di badan. Pemanggil sudah memeriksa item utama | `INDIRECT_POLICY` | Jadwal menyentuh item di owner lain: perlu ditinjau (B3-10) | `INDIRECT_POLICY` | B3-10 | tes jadwal belum diidentifikasi |
| WRITE-016 | Alokasi effort | `items/effort.go:18` `updateAllocations` menulis `item_assignees` tanpa guard di badan | `INDIRECT_POLICY` (dari `patch`) | Assignee lintas owner (B3-06) | `INDIRECT_POLICY` | B3-06 | — |
| WRITE-017 | Pakai blueprint (instansiasi) | `POST /api/v1/blueprints/{blueprintId}/items` (`blueprints.go:15`) → `useBlueprint` (`blueprints.go:79`) → `items.UseBlueprint` (`items/blueprints.go:427`) → `useBlueprint` (`:440`) | `requireSpace` (`blueprints.go:295`, dipanggil dari `:328`); `UseBlueprint` memakai `BeginTx`/`Serializable`. Tulis `items` (`:621` via `restoreStatusAndFields`), `item_dependencies` (`:553`) | `CENTRAL_POLICY` (`requireSpace` → `CanOnSpace`); `restoreStatusAndFields` `NEEDS_TRACE` | Blueprint ke Space klien lain ditolak (B3-09) | `CENTRAL_POLICY` | B3-09 | `blueprint_edit_preview_integration_test.go`, `blueprint_hardening_integration_test.go`, `assignment_authority_integration_test.go` |
| WRITE-018 | Ubah blueprint | `PATCH /api/v1/blueprints/{id}` (`blueprints.go:18`) → `items.EditBlueprint` (`items/blueprints.go:688`) | `requireSpace` (`:328`), `BeginTx` | `CENTRAL_POLICY` | B3-09 | `blueprint_*` tes | B3-09 |
| WRITE-019 | Simpan blueprint dari item | `POST /api/v1/spaces/{spaceId}/blueprints` (`blueprints.go:13`) → `items.SaveBlueprint` (`:58`) → `saveBlueprint` (`:71`) | `requireSpace(policy.CreateBlueprint)` (`:72`) | `CENTRAL_POLICY` | B3-09 | `blueprint_*` tes | B3-09 |
| WRITE-020 | Simpan/instansiasi template Space | `POST /api/v1/space-templates` (`spaces.go:23`) → `httpserver/space_template_save.go`; `POST /api/v1/space-templates/{id}/spaces` (`spaces.go:36`) | Tidak diverifikasi di tahap ini | `NEEDS_TRACE` | B3-09 | `space_templates_integration_test.go`, `space_default_*` | B3-09 |
| WRITE-021 | Kirim form internal | `POST /api/v1/forms/{formId}/submissions` (`forms.go:40`) → `submitForm` (`form_submit.go:348`) | Aktor `policy.FormActor` (`engine.go:35`) → `formItemsCaps` (`policy/item.go:103`); error `items.ErrAssignForbidden` | `CENTRAL_POLICY` | B3-12 | `form_submission_scope_integration_test.go`, `forms_*` |
| WRITE-022 | Kirim form publik | `POST /api/v1/public/forms/{token}/submissions` (`forms_public.go:30`) → `submitPublicForm` (`:87`) | Aktor `policy.External` dengan `externalRequesterID` (`:173`) | `PUBLIC_TOKEN` | B3-12, B3-17 | `form_*` tes |
| WRITE-023 | Import Excel | `POST /api/v1/spaces/{spaceId}/excel-imports` (`project_excel.go:24`) → `importProjectExcel` (`project_excel.go:291`) → `items.ImportExcel` (`excel_import.go:65`) | `policy.Create` dan `requireSpace` (`excel_import.go`); `SpaceAccessFor` di handler (`project_excel.go:291`). Tulis `item_dependencies` (`:162`) | `CENTRAL_POLICY` | Baris dependensi lintas owner (B3-14) | `CENTRAL_POLICY` | B3-14 | `projectexcel/` (unit); tes integrasi import Excel belum diidentifikasi |
| WRITE-024 | Aturan otomasi | `POST /api/v1/spaces/{spaceId}/automation-rules` (`automation.go:13`) | `NEEDS_TRACE` (cek di `automation/` dan `httpserver/automation*`) | — | B3-13 | `automation_workspace_integration_test.go` |
| WRITE-025 | Webhook (kelola) | `POST /api/v1/webhooks` (`webhooks.go:15`) | `NEEDS_TRACE` | — | B3-16 | `webhooks_integration_test.go` |
| WRITE-026 | Bookmark, follow, star | `…/bookmarks`, `PUT /items/{id}/star` (`stars.go:16`), `PUT /items/{id}/followers` (`spaces.go:35`) | `NEEDS_TRACE` | — | B3-03 (mungkin tidak relevan) | — |

## 5. Permukaan baca, agregasi, dan pengiriman

| Surface ID | Operasi | Jalur | Titik masuk → kebijakan | Penyaringan sebelum agregasi | Kebocoran metadata | Task | Tes |
|---|---|---|---|---|---|---|---|
| READ-001 | Daftar item (per container) | `GET /api/v1/items` | `items/list.go:49` `List` → `visibleContainer` (`:155`) → `SpaceAccessFor` (`:157`), `ItemCapsInSpace` (`:176`) | Ya, per container | Nama container | B3-15 | `search_integration_test.go`, `workload_integration_test.go` |
| READ-002 | Detail item | `GET /api/v1/items/{id}` (`items.go:51`) → `items.Get` (`read.go:22`) → `readDetails` → `policy.ItemsCaps` | Ya, per item | Lokasi (nama Space) jika tidak dibatasi | B3-05 | — |
| READ-003 | Pencarian | `GET /api/v1/search` (`httpserver/search.go:10`) → `items.Search` (`search.go:32`) → `policy.ItemsCaps`, `policy.View` | Ya, per item **sebelum** hitungan | Ya; hitungan dan saran (`search.go:273` kandidat aktor) | B3-15 | `httpserver/search_integration_test.go`, `qa_b3a_search_test.go` |
| READ-004 | Portfolio | `GET /api/v1/portfolio` → `items/portfolio.go:32` → `policy.ItemsCaps` | Ya | Hitungan per status (jika tidak disaring) | B3-15 | `httpserver/portfolio_integration_test.go` |
| READ-005 | Ringkasan Workspace (Overview) | `GET /api/v1/overview`, `…/tasks`, `…/timeline`, `…/agenda` (`spaces.go:44–47`) → `overview.Build` (`overview.go:71`) → `scope` (`:103`) → `policy.VisibleSpaces` | Ya, per Space | Hitungan KPI dan aktivitas (`lastActivity`, `:212`) | B3-15 | `overview_integration_test.go` |
| READ-006 | Beban kerja | `GET /api/v1/workload` (`resources.go:22`) → `loadWorkload` (`workload.go:77`) → `policy.ItemsCaps` (`:140`) | Ya | Nama user dan jumlah tugas | B3-15 | `workload_integration_test.go`, `workload_lane_create_integration_test.go` |
| READ-007 | Analitik (regular) | `POST /api/v1/analytics/query` (`dashboards.go:71`) → `analytics.Run` (`analytics.go:293`) → `runGrouped` (`:371`) → `policy.ItemsCaps` (`:397`) | Ya, per item (`:393–402`) | Ya | B3-15 | `analytics_*_integration_test.go` |
| READ-008 | **Analitik operasional** | `POST /api/v1/analytics/operational` (`dashboards.go:72`) → `RunOperational` → `runGrouped(operational=true)` | **Tidak** per item. Hanya grup dengan `MinGroupItems` | **Ya, tanpa per item**: hitungan grup bisa membocorkan jumlah lintas owner | B3-15 (**prioritas tinggi**) | `operational_report_integration_test.go` |
| READ-009 | Dashboard (lihat) | `GET /api/v1/dashboards/{id}` (`dashboards.go`) → `getDashboard` (`dashboards.go:213`) → `policy.View` | Ya, berdasarkan Space dashboard | Widget memanggil analitik (READ-007) | B3-15 | `httpserver/dashboards_integration_test.go`, `dashboards_data_integration_test.go` |
| READ-010 | Dashboard publik | `POST /api/v1/public/dashboards/{token}` (`dashboards_public.go:24`) → `viewPublicDashboard` (`:182`) | Hak **pembuat** dashboard (`creator`, `:182–200`) | Ya, dengan hak pembuat | B3-17 | `dashboard_*` / publik tes |
| READ-011 | Laporan (report) | `reports.go` (`reports.$reportId` di web) | `NEEDS_TRACE` | — | B3-15 | `reports_integration_test.go` |
| READ-012 | Notifikasi, inbox, my to-do | `GET /api/v1/inbox`, `/my-todo` (`todo.go:14` `MyTodo` → `policy.ItemsCaps`) | Ya | — | B3-16 | `httpserver/todo_integration_test.go` (My to-do); tes inbox tidak ditemukan |
| READ-013 | Stream aktivitas | `items/stream.go:41` `Stream` → `policy.ItemsCaps` | Ya | — | B3-16 | Tidak ditemukan tes khusus stream |
| READ-014 | SSE realtime | `GET /api/v1/events` (`server.go:170`) → `streamEvents` (`httpserver/events.go:64`) → `eventContainers` (`:187`) → `policy.ItemCaps` (`:196`), `DeletedItemCapsInSpace` (`:206`) | Ya, per event dan per container | Ya: lokasi (`event.ItemID`, container) | B3-16 | `events_integration_test.go` |
| READ-015 | Email notifikasi | `mail/notification.go` (`:58–60`): `ItemCaps` dan `SpaceAccessFor` per penerima | Ya | Judul item dan nama Space di isi email | B3-16 | `mail/invitation_integration_test.go` |
| READ-016 | Ekspor Excel (item) | `POST /api/v1/items/{projectId}/excel-exports` (`project_excel.go:25`) → `projectexcel/export.go` (`ItemCapsInSpace` :73, `ItemsCaps` :140) | Ya | Nama dan field semua item project | B3-17 | `projectexcel/*` |
| READ-017 | Ekspor Excel (Space) | `POST /api/v1/spaces/{spaceId}/excel-exports` (`project_excel.go:26`) | `NEEDS_TRACE` sampai dibuka | — | B3-17 | — |
| READ-018 | Snapshot Gantt (baca) | `GET /api/v1/items/{projectId}/gantt-snapshots` → `listGanttSnapshots` (`:58`), `policy.View` | Ya | — | B3-17 | `httpserver/gantt_snapshots_integration_test.go` |
| READ-019 | Snapshot Gantt publik | `GET /api/v1/public/gantt/{token}` (`gantt_snapshots.go:211`) | Token; payload beku | Tidak ada izin hidup; membaca payload saat dibuat | B3-17 | — |
| READ-020 | Templat Space (daftar) | `httpserver/space_templates.go:20` `listSpaceTemplates` | `NEEDS_TRACE` (tidak ada `policy.` di badan) | — | B3-09 | `space_templates_integration_test.go` |
| READ-021 | Pencarian pengguna (mention, picker) | `items/search.go:273` (kandidat aktor), `MentionPrincipals` | Ya | Daftar user di luar klien | B3-15 | — |
| READ-022 | Relasi dan dependensi (baca) | `POST /api/v1/dependencies/query` (`dependencies.go:12`) → `ListDependencies` (`dependencies.go:43`) → `ItemCaps` (`:55`), `ItemsCaps` (`:79`) | Ya; filter per item | — | B3-10 | — |

### 5.1 Permukaan file dan publik

| Surface ID | Operasi | Jalur | Kebijakan | Catatan | Task | Tes |
|---|---|---|---|---|---|---|
| FILE-001 | Unggah lampiran | `POST /api/v1/items/{itemId}/attachments` (`attachments.go:38`) → `uploadAttachment` (`:111`) → `attachmentAccess` (`policy.ItemCaps`, `policy.Attach`) | Ya (`attachmentAccess`) | Kebijakan `policy.Attach` | B3-17 | `attachment_*_integration_test.go` (5 berkas) |
| FILE-002 | Tautan lampiran | `POST /api/v1/items/{itemId}/attachment-links` (`attachments.go:39`) → `createAttachmentLink` (`:227`) | Ya | Tautan eksternal (`link_url`) | B3-17 | `attachment_*` |
| FILE-003 | Unduh lampiran | `GET /api/v1/attachments/{id}/download` (`attachments.go:40`) → `attachmentResource` | Ya (`attachmentAccess`) | Dikonfirmasi lewat rute; badan fungsi tidak diperiksa | B3-17 | `attachment_storage_integration_test.go` |
| FILE-004 | Versi dan proofing | `/api/v1/attachments/{id}/versions/…` (`attachment_proofing.go:26`) → `versionItem` (`:38`) → `versionItemOf` (`:45`) → `attachmentAccess` | Ya | — | B3-17 | `attachment_versions_integration_test.go` |
| FILE-005 | Perbandingan file | `attachment_compare.go` | `NEEDS_TRACE` | — | B3-17 | `attachment_compare_integration_test.go` |
| FILE-006 | Pratinjau | `previews/previews.go` (job) | `SYSTEM_ACTOR` | Membaca file tanpa aktor pengguna; izin harus dipastikan dari entri antrean | B3-17 | `attachment_pdf_skipped_integration_test.go` |
| FILE-007 | Berkas project (daftar) | `httpserver/project_files.go:19` `listProjectFiles` → `policy.ItemsCaps`, `policy.View` | Ya | — | B3-17 | — |
| FILE-008 | Folder Pydio (daftar) | `GET /api/v1/items/{itemId}/pydio-files?field=` → `listPydioFiles` (`httpserver/pydio_files.go:20`) → `items.Get` (**kebijakan item, termasuk grant field**) → `pydio.Client.Folder` (validasi host dan root) → `pydio.Client.List` (`server/internal/pydio/pydio.go`, S3 gateway dengan SigV4, token layanan dari `PMS_PYDIO_LINK_TOKEN`) | Ya, lewat `items.Get`. **Token layanan** dipakai untuk semua pengguna yang lolos gerbang item | B4-02, B4-05, B4-06 | `pydio_files_integration_test.go`, `pydio/pydio_test.go` |
| FILE-009 | Field Pydio (tulis) | `PATCH /api/v1/items/{id}` dengan nilai `Pydio Folder` (lewat `patch`) | Ya (AUTH-018). **Validasi host/root belum ada saat tulis** (hanya saat listing) | Nilai tidak tervalidasi bisa tersimpan (lihat §8, F-06) | B4-02 | — |
| FILE-010 | Ekspor Gantt, snapshot | `gantt_snapshots.go` | Lihat READ-018, READ-019 | — | B3-17 | — |

## 6. Pekerjaan latar belakang (worker)

| Surface ID | Paket / file | Titik masuk | Aktor | Operasi | Reotorisasi | Task | Tes |
|---|---|---|---|---|---|---|---|
| JOB-001 | `automation/engine.go` | `Run`/`tick`; aktor di `:281` | Pemilik rule (`ownerKind`) | Tulis: tindakan rule lewat paket items | Ya: `CanOnSpace(Edit)` di `automation/rules.go:171` pada eksekusi | B3-13 | `automation_*` (17 berkas) |
| JOB-002 | `duereminders/duereminders.go` | Job pengingat; aktor `:106` | Penerima pengingat | Baca item, kirim email | `ItemCaps` (`:112`) | B3-16 | `duereminders_*` (2 berkas) |
| JOB-003 | `reportdigest/reportdigest.go` | Job digest; aktor `:100`, `SpaceAccessFor` (`:128`) | Pemilik langganan | Baca, kirim | Ya per slot; tes yang gagal di CI ada di `httpserver/report_digest_integration_test.go:15` | B3-16 | `TestReportSubscriptionsAreDeliveredOncePerSlotWithTheSubscribersRights` (lihat §7) |
| JOB-004 | `connectors/worker.go`, `connectors/pilot.go` | Job sinkronisasi; `ItemsCaps` (`pilot.go:211`) | Pembuat instalasi | Baca/tulis item | Ya (`ItemsCaps`) | B3-13, B4 | `connectors_integration_test.go`, `pilot_integration_test.go` |
| JOB-005 | `webhooks/webhooks.go` | Pengiriman; `ItemCaps` (`:373`) | Pemilik webhook (`t.owner`) | Kirim payload | Ya per penerima | B3-16 | `webhooks_integration_test.go` |
| JOB-006 | `mail/mail.go`, `mail/notification.go`, `mail/invitation.go` | Antrean email | Penerima | Kirim | Ya (`ItemCaps`, `SpaceAccessFor` di `notification.go:58–60`) | B3-16 | `invitation_integration_test.go` |
| JOB-007 | `housekeeping/housekeeping.go` | Pembersihan berkala | Sistem | Hapus `sessions`, `user_tokens`, `shares` (kedaluwarsa, `:94`), `project_excel_exports` | Tidak (sistem) | B3-08 (retensi share) | — |
| JOB-008 | `httpserver/approval_deadlines.go` | Tenggat approval | Sistem | Tulis `items` (`auto_approval.go:66`) | `NEEDS_TRACE` | B3-13 | `approval_*` |
| JOB-009 | `realtime/`, `httpserver/events.go` | Dispatch event SSE | Pengguna SSE (`streamActor`) | Baca | Ya (lihat READ-014) | B3-16 | `events_integration_test.go` |
| JOB-010 | `seed/seed.go` | Saat `PMS_SEED` | `Admin` (sistem) | Tulis field dan grup | Tidak (sekali per database) | — (MVP-03) | `seed/seed_test.go` |

Pemanggilan `policy.Actor{}` kosong (tanpa ID) di `automation/lifecycle.go:61` dan `automation/log.go:332` untuk audit: `NEEDS_TRACE` apakah ada aksi tulis dengan aktor kosong.

## 7. Tes yang relevan

| Area | Berkas tes yang ditemukan | Catatan |
|---|---|---|
| Kebijakan inti | `policy/policy_test.go` (5 fungsi), `policy/contract_test.go` (1), `policy/custom_roles_test.go` (4) | Unit, tanpa basis data |
| Otorisasi integrasi | `httpserver/authorization_*` (`authorization_final_regression_integration_test.go`, `authorization_locked_space_integration_test.go`, `authorization_space_admin_downgrade_integration_test.go`), `approval_authorization_integration_test.go`, `m2_authorization_integration_test.go`, `m3_authorization_integration_test.go` | Termasuk `TestAuthorizationFinalRegressionExplicitAccessOnly` |
| Endpoint sweep | `httpserver/endpoint_sweep_integration_test.go:58` `TestEveryProtectedEndpointChecksTheSessionAndSurvivesUnknownIds` | Menjangkau semua operasi yang dideklarasi di OpenAPI |
| Matriks akses | CI job `authorization-matrix` (`.github/workflows/ci.yml:82–107`): `-run 'Authorization\|AccessRules\|Matrix\|Capabilit\|SpaceAdmin\|ItemShare\|Move\|ConcurrentMoves\|ReplaceItemLocations'` | Job ini hijau pada `c3782a7` dan `72f9902` (run kedua). Ambang: `passed >= 23`, `skipped = 0`, `failed = 0` |
| Share | `item_shares_integration_test.go`, `item_share_revoke_audit_integration_test.go`, `custom_field_item_share_integration_test.go`, `share_delegation_integration_test.go` | — |
| Move dan lokasi | `move_race_integration_test.go`, `move_parent_child_regression_integration_test.go`, `move_audit_integration_test.go` | Tidak ada tes lintas klien (klien belum ada) |
| Space admin dan governance | `space_admin_eligibility_integration_test.go`, `space_governance_integration_test.go`, `space_members_integration_test.go`, `spaces_integration_test.go` | — |
| Custom field dan akses field | `custom_field_access_integration_test.go`, `custom_field_assignments_integration_test.go`, `custom_fields_integration_test.go` | CF-D03 |
| Form dan publik | `form_submission_scope_integration_test.go`, `form_space_metadata_integration_test.go`, `forms_*` | — |
| Analitik | `analytics_integration_test.go`, `analytics_varied_integration_test.go`, `analytics_cost_integration_test.go`, `operational_report_integration_test.go` | `analytics_integration_test.go` memverifikasi admin tanpa grant = 0 |
| Pydio | `pydio_files_integration_test.go`, `pydio/pydio_test.go`, `seed/seed_test.go` | Lihat MVP-04/13 |
| Lampiran | `attachment_*_integration_test.go` (5) | — |
| Worker | `automation_*` (17), `duereminders_*` (2), `report_digest_integration_test.go` (1), `connectors` (4) | `TestReportSubscriptions…` ada di `httpserver/report_digest_integration_test.go:15` |
| Lintas klien | **Tidak ada** (klien belum ada) | Fixture bersama B3-22 harus dibuat |

Jumlah tes ini adalah hitungan **berkas**, bukan kesimpulan cakupan.

## 8. Celah B3/B4 dan ringkasan risiko

Risiko diukur sebagai risiko implementasi (bukan kerentanan yang terbukti). HIGH = isolasi klien di masa depan, mutasi tanpa otorisasi, atau metadata rahasia. MEDIUM = regresi, cakupan tidak lengkap, atau inkonsistensi operasional. LOW = dokumentasi atau pemeliharaan.

| Finding ID | Permukaan | Bukti | Potensi dampak | Task | Tindak lanjut yang diperlukan |
|---|---|---|---|---|---|
| F-01 | READ-008 analitik operasional | `analytics.go:364–371`: `runGrouped(operational=true)` melewati gerbang per item; hanya `MinGroupItems` | **HIGH.** Hitungan grup bisa membocorkan jumlah item lintas Space/klien | B3-15 | Pasang gerbang klien sebelum agregasi; uji hitungan untuk user tanpa membership |
| F-02 | AUTH-006 `itemsCaps` (sentral) | `item.go:51–100`: union dari semua lokasi tanpa penyaringan owner | **HIGH.** Ini titik yang harus disaring sebelum `caps[id] |=` | B3-05 | Tambah filter lokasi per Space eligible dan owner sama di CTE `ancestry`; uji assign tetap memakai `SharedOnEveryPath` |
| F-03 | Jalur tulis tanpa guard di badan (WRITE-002 `applyFields`/`setAssignees`, WRITE-014/015/016, WRITE-018 `restoreStatusAndFields`, WRITE-013 `UpdateDependency`/`DeleteDependency`, WRITE-012 `SetArchived`) | `patch.go:222,404`; `custom_field_types.go:341`; `scheduling.go:163`; `effort.go:18`; `blueprints.go:596`; `dependencies.go:157,232`; `governance.go:75` | **HIGH** bila pemanggil berubah atau ada pemanggil lain. Jalur ini bergantung pada pemanggil | B3-06, B3-09, B3-10, B3-03 | Trace setiap pemanggil; tambahkan guard owner di lapisan yang sama dengan pemanggil |
| F-04 | DA-004 SQL langsung di `httpserver/` (admin.go, auto_approval.go, workflows.go, scim.go, create_menu.go, server.go `acceptInvitation`) | §3.3 | **HIGH.** Penulisan langsung ke `shares` dan `items` melewati paket domain | B3-08, B3-11, B3-13 | Trace tiap berkas; pindahkan atau tambah guard owner |
| F-05 | JOB-003 digest dan JOB-001 automation | `reportdigest.go:100,128`; `automation/rules.go:171` | MEDIUM. Digest memakai aktor langganan; automation memeriksa ulang pada eksekusi | B3-13, B3-16 | Pastikan reauthorization menyertakan gerbang klien dan replay |
| F-06 | FILE-009 nilai `Pydio Folder` | Validasi host/root hanya saat listing (`pydio_files.go`), tidak saat tulis | MEDIUM. Nilai tak tervalidasi bisa tersimpan (listing akan menolak, tetapi tautan tetap tampil di panel) | B4-02 | Validasi saat tulis (PATCH, import, blueprint) |
| F-07 | FILE-008 token layanan Pydio | `pydio.Client` memakai satu token untuk semua pengguna yang lolos gerbang item | **HIGH** untuk kerahasiaan nama file. Risiko yang sudah dicatat di MVP-04/13 | B4-05 | Keputusan K-21 sebelum listing diaktifkan di production |
| F-08 | READ-010 dashboard publik dengan hak pembuat | `dashboards_public.go:182–200` | MEDIUM. Dashboard publik menampilkan data dengan hak pembuat yang mungkin melintasi klien | B3-17 | Batasi dashboard publik ke Space tunggal; flag `PMS_PUBLIC_LINKS` |
| F-09 | DA-009/022 form publik (`External`) | `forms_public.go:173`; `submitPublicForm` | MEDIUM. Jalur tanpa sesi; bergantung pada token dan form | B3-12, B3-17 | Pastikan form publik tidak bisa menargetkan Space klien |
| F-10 | `policyCache` salah sasaran | `access_policy.go:33` (§1.1) | LOW (dokumentasi). Rencana B3-07 tidak punya target cache | B3-07 | Perbarui B3-07 menjadi "pastikan tidak ada cache keputusan; uji pencabutan membership langsung" |
| F-11 | WRITE-010 `adminInvite`/`adminGroup` | `admin.go:237,428`; guard admin di handler, bukan `policy` | MEDIUM. Undangan Full (DEF-045, sudah diperbaiki di 0.12.5-rc3) | B3-11 | Uji ulang DEF-045 di jalur ini setelah B3 |
| F-12 | Tes lintas klien tidak ada | §8 | MEDIUM. Tidak ada fixture klien yang bisa dijalankan | B3-22 | Buat fixture bersama |
| F-13 | DA-006 housekeeping hapus `shares` dengan SQL | `housekeeping.go:94` | LOW. Aturan kedaluwarsa, bukan akses | B3-08 | Tinjau retensi bersama B3-19 |

## 9. Pemetaan ke task implementasi

| Task | Permukaan terpetakan | Catatan |
|---|---|---|
| B3-03 (create Space dan item) | WRITE-001, WRITE-012 | — |
| B3-04 (gerbang Space) | AUTH-001..005, WRITE-011, READ-005 | F-02 ada di B3-05 |
| B3-05 (gerbang item per lokasi) | AUTH-006..008, AUTH-018, READ-002, READ-004 | F-02 |
| B3-06 (aturan assign) | AUTH-009, AUTH-013, AUTH-014, WRITE-002, WRITE-016 | — |
| B3-07 (cache dan pencabutan) | AUTH-019, F-10 | Target cache tidak ada |
| B3-08 (lokasi, move, restore) | WRITE-003..007, JOB-007 | — |
| B3-09 (copy, blueprint, template) | WRITE-017..020, READ-020 | — |
| B3-10 (dependensi dan relasi) | WRITE-013..015, READ-022 | F-03 |
| B3-11 (share item, Space, grup) | WRITE-008..010, AUTH-010, AUTH-015 | F-11 |
| B3-12 (form) | WRITE-021, WRITE-022, AUTH-007, F-09 | — |
| B3-13 (automation dan worker) | WRITE-024, JOB-001, JOB-004, JOB-008 | F-05 |
| B3-14 (import) | WRITE-023 | — |
| B3-15 (list, search, agregasi) | READ-001..009, READ-011, READ-021, F-01 | Prioritas F-01 |
| B3-16 (notifikasi, SSE, email, webhook) | READ-012..015, JOB-002, JOB-003, JOB-005, JOB-006, JOB-009 | F-05 |
| B3-17 (lampiran, publik, ekspor) | FILE-001..007, READ-010, READ-016..019, F-08 | — |
| B4 (Pydio) | FILE-008, FILE-009, F-06, F-07 | Tidak ada permintaan live |

## 10. Batasan dan jalur yang belum ditelusuri

### 10.1 Jalur NEEDS_TRACE

- `restoreStatusAndFields` (`items/blueprints.go:596`) dan pemanggilnya (`:548`), yang memanggil `applyFields` tanpa guard.
- `SetArchived` (`spaces/governance.go:75`) dan pemanggilnya.
- `UpdateDependency` dan `DeleteDependency` (`items/dependencies.go:157`, `:232`): guard hanya terlihat untuk create.
- `httpserver/admin.go` `adminInvite`, `adminGroup` (`:237`, `:428`): guard admin di handler, belum diverifikasi sampai akhir.
- Rute otomasi, webhook, template, bookmark, dan star (WRITE-024..026): belum dibuka.
- Ekspor Excel untuk Space (READ-017), laporan (READ-011), dan perbandingan file (FILE-005): belum dibuka.
- `httpserver/workflows.go` `updateStatus` dan `deleteStatus` (`items` :418, :487): belum dibuka.
- Penulisan aktor kosong di `automation/lifecycle.go:61` dan `log.go:332`.
- Badan fungsi handler yang hanya meneruskan ke paket domain: tidak semuanya dibaca penuh; guard diverifikasi di paket, bukan di handler.

### 10.2 Keterbatasan metode

- Tidak ada tes yang dijalankan (aturan Prompt #002). Klaim "terlindungi oleh tes" tidak dibuat di luar daftar berkas di §8.
- Pemeriksaan badan fungsi dibatasi 90–120 baris dari awal fungsi. Guard yang berada lebih jauh bisa terlewat; fungsi yang ditandai `NO-POLICY-IN-BODY` belum tentu tanpa guard.
- Hitungan §3.2 adalah berkas dengan pola SQL, bukan semua akses. Akses lewat nama kolom tanpa nama tabel, atau lewat struktur `dbgen`, tidak terhitung.
- Tidak ada pemeriksaan database nyata. Skema dibaca dari migrasi dan kode, bukan dari instance.

### 10.3 Working tree

Diperiksa sebelum dan sesudah. Perubahan: satu berkas baru, `docs/roadmap/PMS_PLATFORM_IMPACT_ASSESSMENT.md`. Berkas tracked tidak berubah.

### 10.4 Bagian yang belum diisi

Sesuai aturan Prompt #002, bagian pemetaan klien dan inventaris data (B0-04, B0-05) belum diisi.

---

## B0-04 — Existing Data Inventory

| | |
|---|---|
| **Tanggal** | 27 November 2026 |
| **Mode** | Read-only. Tidak ada data dibaca, ditulis, diekspor, atau dipetakan. Tidak ada migrasi atau seed dijalankan |
| **Rencana acuan** | [Rencana MVP v0.2](LENS_MVP_IMPLEMENTATION_PLAN.md) §5 B0-04, B3-01, B3-18, B3-19 |
| **Status B0-04** | **PARTIAL.** Skema dan kode terverifikasi. Jumlah data operasional **NOT VERIFIED** (lihat §1) |
| **Pembaruan** | Bagian "Bagian yang belum diisi" (§10.4 di atas) sekarang dilengkapi untuk data inventory. Pemetaan klien tetap belum diisi (B0-05) |

### 1. Baseline dan sumber data

| Sumber | Tersedia | Jenis data | Status |
|---|---|---|---|
| Skema repo (`server/migrations/`, 00001–00362) | Ya | Definisi skema | **Schema verified** |
| Definisi seed (`server/internal/seed/seed.go`, `project-master`) | Ya | Definisi konfigurasi | **Schema verified** (definisi, bukan data) |
| Prosedur MVP-02 dan MVP-03 | Ya | Dokumen | Dokumen, bukan data |
| Server PostgreSQL lokal (`localhost:5432`) | Server ada | Tidak diketahui | **NOT VERIFIED.** Daftar nama database diambil dari katalog server (`pg_database`), tanpa membaca data: `maloka_dev`, `pms_qa`, `pms_test`, `pms_test_<angka>`, `pms_tpl_<hash>`, `postgres`. Tidak ada identitas yang bisa membuktikan database mana yang merupakan instance PMS berwenang. **Tidak dipakai.** |
| Staging `dev-lens` | Tidak diakses dari audit ini | Tidak diketahui | **NOT VERIFIED.** Hanya kesehatan publik yang diperiksa di Prompt #001 |
| Production | Tidak diakses | — | Dilarang |
| Laporan RC3 (`outputs/qa-final/pr28-20261126/`) | Ya | Data QA sintetis (Space dan akun uji) | Bukan data operasional. Tidak dihitung |
| `docs/qa/RELEASE_READINESS_REPORT.md` | Ya | Catatan lingkungan | Menyebut pengujian dilakukan langsung di DB `dev-lens`, dengan data QA terbatas. **Data QA dan data operasional di DB itu tidak bisa dipisahkan dari dokumen ini** |

**Keputusan akses:** tidak ada database yang identitasnya bisa dibuktikan sebagai instance berwenang, dan belum ada akses baca yang diotorisasi untuk B0-04. Karena itu tidak ada query data. Jumlah operasional semuanya NOT VERIFIED. Tidak ada angka fixture yang dicampur dengan angka operasional.

### 2. Inventaris entitas

Semua kolom dan constraint dibaca dari migrasi. "Migrasi" menunjuk nomor berkas yang menambahkan kolom atau constraint.

| Entitas / grup | Tabel atau penyimpanan | Relasi kunci | Jumlah tersedia | Sumber bukti | Pertimbangan migrasi |
|---|---|---|---|---|---|
| Pengguna | `users` (`user_type` enum, `status`) | Grup, membership | NOT VERIFIED | `00002` | Pengguna eksternal belum ada; `user_type` menentukan batas ceiling |
| Grup | `groups` (`name` unik case-insensitive), `group_members` | Share ke grup | NOT VERIFIED | `00004`; `scim_external_id` `00303` | Grup lintas klien mungkin ada; belum ada cara menandainya |
| Space | `spaces` (`kind` shared/personal, `owner_id`, `locked` `00304`, `archived_at`, `deleted_at`) | Owner, admin, item, share | NOT VERIFIED | `00003`, `00304`, `00320` | Tidak ada `client_id`. `spaces_one_personal_per_owner` (unik untuk personal) |
| Admin Space | `space_admins` (`space_id`, `user_id`, `granted_by`) | Space | NOT VERIFIED | `00346` | Admin dibackfill; aturan `00355` |
| Item (project, folder, task, milestone) | `items` (`type`, `type_id`→`item_type_defs`, `owner_id`, `fields` jsonb, `deleted_at`, `deleted_root_id`) | `item_locations`, shares, assignees | NOT VERIFIED | `00005`, `00046` | **`items.owner_id` sudah ada dan berarti pengguna** (`read.go:175` join ke `users`). Nama `owner` sudah terpakai (lihat §5 MIG-13) |
| Lokasi item | `item_locations` (`item_id`, `space_id`, `parent_item_id`, `rank`; `UNIQUE NULLS NOT DISTINCT (item_id, space_id, parent_item_id)`) | Item, Space, parent | NOT VERIFIED | `00005`, `00007` | Satu item bisa di banyak Space. Constraint tidak melarang parent dari Space lain |
| Penerima tugas | `item_assignees` | Item, user | NOT VERIFIED | `00005` | Assignee lintas klien mungkin ada |
| Pengikut | `item_followers` | Item, user | NOT VERIFIED | `00005` | — |
| Dependensi | `item_dependencies` (`predecessor_id`, `successor_id`, `type`, `lag`; tanpa kolom Space) | Item ke item | NOT VERIFIED | `00016` | **Tidak ada constraint Space.** Dependensi lintas Space diizinkan skema |
| Relasi field | `item_relations` (`field_id`, `from_item_id`, `to_item_id`) | Field relation, item | NOT VERIFIED | `00047` | Tidak ada constraint Space; field dan item terikat FK |
| Definisi field | `custom_fields` (`space_id` nullable, `name`, `type`, `options` jsonb, `position`, `archived_at`; `UNIQUE (space_id, position) DEFERRABLE`) | Assignment, grants, nilai di `items.fields` | NOT VERIFIED | `00017`, `00328` | `space_id` NULL = Workspace. Lihat §3 |
| Penugasan field | `custom_field_assignments` (`field_id`, `space_id`, `scope_kind`, `scope_id`) | Field, Space, project/folder | NOT VERIFIED | `00332` | **`scope_id` tanpa FK** (polimorfik) |
| Grant field | `custom_field_grants` (`field_id`, `subject_kind`, `subject_id`, `level`) | Field, user/grup | NOT VERIFIED | `00331` | `subject_id` tanpa FK (polimorfik) |
| Nilai field | `items.fields` jsonb (kunci = UUID field) | Item | NOT VERIFIED | `00005` | Tidak ada tabel nilai terpisah |
| Share | `shares` (`target_type` space/item, `target_id`, `principal_type`, `principal_id`, `role`, `custom_role_id`) | Space, item, user, grup | NOT VERIFIED | `00004`, `00301` | **`target_id` tanpa FK** (polimorfik): share yatim mungkin ada |
| Peran kustom | `custom_roles` (`caps_mask`) | Share | NOT VERIFIED | `00301`, `00333` | — |
| Tipe item | `item_type_defs` (`key` unik, `base`, `allowed_children`), `item_type_fields` (`default_value`) | Item, field | NOT VERIFIED | `00046`, `00312` | Tipe kustom lintas klien mungkin dipakai bersama |
| Blueprint | `blueprints` (`scope` space/account, `space_id` nullable, `owner_id`, `tree` jsonb) | Space, user | NOT VERIFIED | `00032`, `00333`, `00335` | **Blueprint `account` tidak terikat satu Space** |
| Template Space tersimpan | `saved_space_templates` (`body` jsonb, `source_space_id`) | Space sumber | NOT VERIFIED | `00357` | Isi `body` belum diinspeksi |
| Formulir permintaan | `request_forms` (`space_id`, `access` private/internal, `public_token`, `response` jsonb) | Space, token publik | NOT VERIFIED | `00026`, `00027`, `00033` | **`response.location.space_id` boleh Space lain** (`forms/forms.go:275`; `httpserver/forms.go:380–400`) |
| Otomasi | `automation_rules` (`space_id` nullable, `owner_id`, `container_id`) | Space, container | NOT VERIFIED | `00054`, `00339` | Rule `account` tidak terikat satu Space |
| Lampiran | `attachments` (`item_id`, `link_url`, `current_version`), `attachment_versions` (`storage_key`, `sha256`) | Item | NOT VERIFIED | `00010`, `00090`, `00092` | Lampiran dan tautan eksternal (`link_url`) |
| Event domain | `domain_events` (`origin_space_id` `ON DELETE SET NULL`) | Item, Space | NOT VERIFIED | `00053`, `00341` | `origin_space_id` bisa kosong |
| Audit | `audit_events` | Aktor, target | NOT VERIFIED | `00300`, `00358` | Retensi belum diinspeksi |
| Kebijakan akses | `access_policy` (id=1), `account` (`owner_id`) | Instance | 1 baris (singleton, struktur) | `00002`, `00304`, `00305` | Satu instance = satu akun |
| Seed | `seed_runs` | Nama seed | NOT VERIFIED | `00362` | Mencatat `project-master` setelah seed berjalan |
| Project Pydio Folder | Nilai field teks di `items.fields` (tanpa tabel) | Item | NOT VERIFIED | `seed.go` (MVP-04) | Tidak ada validasi host/root saat tulis (lihat B4-02) |
| Dashboard, laporan, workflow, status | Tidak diinspeksi | — | NOT INSPECTED | — | Di luar daftar wajib B0-04 |

Kesimpulan per kelompok: **Organisasi dan hierarki** lengkap skemanya; **akses** ada tiga jalur (grant Space, share item, grup) dan semuanya polimorfik di sisi `target_id`/`subject_id`; **konfigurasi bisnis** semuanya di `items.fields` dan `custom_fields`; **relasi lintas** (dependensi, relasi, lokasi, formulir, blueprint) tidak punya constraint Space.

### 3. Inventaris referensi Customer

| Aspek | Temuan | Bukti |
|---|---|---|
| Identitas field | **Tidak ada kunci tetap.** Field Customer ditemukan dengan nama (prosedur MVP-02 meminta nama `Customer`). Tidak ada migrasi atau seed yang membuatnya | `rg -i customer server/migrations` tidak menemukan apa pun; `seed.go` tidak membuatnya |
| Cakupan | Workspace (`space_id` NULL) atau Space (`space_id` terisi). Penugasan ke Space lewat `custom_field_assignments` | `00328`, `00332` |
| Identitas opsi | **Stabil.** Setiap opsi punya `id` UUID (wajib), `label` bebas, `color` | `custom_fields.go:72–93` `validateOptions` |
| Penyimpanan nilai | `items.fields[<field-uuid>] = "<option-uuid>"` | `custom_field_types.go` (validasi pilihan, sekitar `:229`) |
| Validasi nilai | Nilai harus salah satu opsi. Opsi yang sudah dipakai tidak bisa dihapus | `custom_field_types.go:229`; `custom_fields.go:378` `preserveUsedOptions` |
| Opsi yatim | Label kosong di analitik bila opsi tidak ada (`LEFT JOIN`) | `analytics/analytics.go:150–152` |
| Duplikat definisi | **Mungkin.** Tidak ada `UNIQUE` pada `name`. `UNIQUE (space_id, position)` tidak mencegah duplikat untuk Workspace karena `NULL` tidak dianggap sama | `00017`; `00328` |
| Arsip | Field diarsipkan (`archived_at`), nilai tetap tersimpan | `custom_fields.go`; `item_type_defs` |
| Konvensi label `KODE — Nama` | **Tidak ditegakkan** di sisi server. Hanya prosedur | `MVP-02_CUSTOMER_REFERENCE.md` |
| Seed | Seed `project-master` memakai ulang field Workspace yang namanya sama (huruf kecil diabaikan), hanya saat `space_id` NULL. Field Space bernama sama **tidak** dipakai ulang → duplikat mungkin | `seed.go` (query `lower(f.name)`), `seed_test.go` |
| Pydio Folder | Mekanisme yang sama (field teks di `items.fields`). Tidak ada tabel referensi terpisah | `seed.go` (MVP-04) |

**Yang tidak bisa dibuktikan:** berapa definisi Customer yang ada, apakah ada duplikat, dan apakah label opsi mengikuti konvensi. Semuanya membutuhkan data operasional.

### 4. Ringkasan jumlah yang diminta

| Entitas | Jumlah | Sumber |
|---|---:|---|
| Space | NOT VERIFIED | — |
| Space personal | NOT VERIFIED | — |
| Project | NOT VERIFIED | — |
| Task atau item lain | NOT VERIFIED | — |
| Item multi-lokasi | NOT VERIFIED | — |
| Definisi field Customer | NOT VERIFIED | — |
| Opsi Customer | NOT VERIFIED | — |
| Project tanpa Customer | NOT VERIFIED | — |
| Space campuran kandidat | NOT VERIFIED | — |
| Referensi Pydio Folder | NOT VERIFIED | — |

### 5. Matriks konflik migrasi kepemilikan

Klasifikasi: **POTENTIAL** = skema atau kode mengizinkan kondisi ini; jumlahnya belum diketahui. **NOT_VERIFIED** = tidak bisa dihitung dari data. Tidak ada **CONFIRMED** karena tidak ada data yang dibaca. **NOT_FOUND** tidak digunakan karena tidak ada pemeriksaan data yang berhasil. Multi-lokasi dan dependensi lintas Space bukan defek; keduanya diizinkan desain.

| ID konflik | Jumlah | Bukti | Klasifikasi | Task masa depan |
|---|---:|---|---|---|
| MIG-01 Space berisi project dengan nilai Customer berbeda | NOT VERIFIED | Field Customer dapat diisi per project; tidak ada constraint Space | POTENTIAL | B0-05, B3-18 |
| MIG-02 Project tanpa Customer | NOT VERIFIED | Nilai field opsional | POTENTIAL | B0-05 |
| MIG-03 Item di banyak Space | NOT VERIFIED | `item_locations` mengizinkan banyak `space_id` per item | POTENTIAL | B3-18 |
| MIG-04 Multi-lokasi lintas konteks kandidat | NOT VERIFIED | Sama dengan MIG-03; konteks kandidat dari MIG-01 | POTENTIAL | B3-18 |
| MIG-05 Item di Personal Space dan Space lain | NOT VERIFIED | `spaces_one_personal_per_owner`; `itemsCaps` memberi akses personal hanya ke owner (AUTH-006) | POTENTIAL | B0-05 (keputusan Personal Space) |
| MIG-06 Parent dan child lintas konteks | NOT VERIFIED | `parent_item_id` tanpa constraint Space | POTENTIAL | B3-08 |
| MIG-07 Dependensi dan relasi lintas konteks | NOT VERIFIED | `item_dependencies` dan `item_relations` tanpa kolom Space | POTENTIAL | B3-10 |
| MIG-08 Share langsung atau grup yang berubah saat membership berubah | NOT VERIFIED | `shares.target_id` polimorfik; grup tidak terikat klien | POTENTIAL | B3-11, B3-07 |
| MIG-09 Item arsip, hapus, atau recycle | NOT VERIFIED | `deleted_at`, `deleted_root_id`, `spaces.archived_at`, `spaces.deleted_at`; semua dibuat sebagai fitur | POTENTIAL | B3-08, B0-05 |
| MIG-10 Formulir, blueprint, template, atau otomasi lintas Space | NOT VERIFIED | `response.location.space_id` boleh Space lain (kode `checkDestination`); blueprint dan otomasi `account` tidak terikat satu Space | POTENTIAL (jalur kode terkonfirmasi) | B3-09, B3-12, B3-13 |
| MIG-11 Referensi Pydio Folder hilang, tidak valid, atau bertentangan | NOT VERIFIED | Tidak ada validasi saat tulis; tautan lampiran (`link_url`) juga ada | POTENTIAL | B4-02 |
| MIG-12 Referensi yatim | NOT VERIFIED | `shares.target_id`, `custom_field_assignments.scope_id`, `custom_field_grants.subject_id` tanpa FK | POTENTIAL | B3-19, B3-22 |
| MIG-13 Nama kepemilikan ganda | — | `items.owner_id` sudah berarti pengguna (`00005`, `read.go:175`); `spaces.owner_id` berarti pemilik Space; `account.owner_id` berarti admin | CONFIRMED (skema) | B0-05 (penamaan) |

### 6. Batas dan ketersediaan data

- Jumlah operasional **seluruhnya NOT VERIFIED.** Tidak ada database yang identitasnya terbukti sebagai instance berwenang.
- Schema verified untuk semua tabel di §2. Dashboard, laporan, workflow, dan status tidak diinspeksi.
- Isi `saved_space_templates.body`, `blueprints.tree`, `request_forms.definition/response`, dan `automation_rules` belum dibaca; hanya strukturnya.
- Konvensi label Customer dan duplikat definisi hanya bisa diuji dengan data.
- Tidak ada klaim bahwa suatu konflik tidak ada. Semua status konflik adalah POTENTIAL atau NOT_VERIFIED.

### 7. Input yang dibutuhkan untuk B0-05

1. **Akses baca yang diotorisasi** ke satu instance yang identitasnya terbukti (nama host dan nama database dicatat dan disetujui PO). Tanpa itu, B0-05 hanya bisa berdasarkan skema.
2. **Daftar definisi field Customer per instance** dalam bentuk agregat: jumlah definisi, cakupan (Workspace/Space), jumlah opsi, dan apakah ada nama duplikat. Tanpa nama pelanggan.
3. **Hitungan agregat** untuk MIG-01..12 sesuai query yang diusulkan di §5, dijalankan read-only dengan `statement_timeout` dan `ROLLBACK`.
4. **Keputusan PO** tentang Personal Space: apakah item di Personal Space boleh ikut Space klien (MIG-05).
5. **Keputusan PO** tentang blueprint `account`, otomasi `account`, dan formulir dengan tujuan Space lain (MIG-10).
6. **Keputusan penamaan** kepemilikan: `items.owner_id` sudah ada untuk pengguna, sehingga kolom klien perlu nama lain (MIG-13).
7. **Keputusan** apakah `shares` dan grant polimorfik akan diberi FK atau dicek di aplikasi (MIG-12).

### 8. Bukti verifikasi read-only

| Pemeriksaan | Hasil |
|---|---|
| Git | Branch `docs/restructure-mvp-roadmap`, HEAD `a8477f4`. Tidak ada perubahan tracked |
| Migrasi | Dibaca dari `server/migrations/`. Terakhir `00362` |
| Pencarian `Customer` di migrasi dan seed | Tidak ditemukan |
| Katalog server PostgreSQL | Hanya nama database (`pg_database`). Tidak ada tabel atau baris yang dibaca |
| Database yang dibuka | **Tidak ada** |
| Perubahan database | Tidak ada |
| `deploy/.env` | Hanya dicek keberadaannya, tidak dibaca isinya |
| Tes | Tidak ada |
| Berkas yang diubah | `docs/roadmap/PMS_PLATFORM_IMPACT_ASSESSMENT.md` (bagian B0-04 ditambahkan) |

---

## B0-05 — Client Mapping Specification

| | |
|---|---|
| **Tanggal** | 27 November 2026 |
| **Mode** | Spesifikasi desain, read-only. Tidak ada data dibaca, dipetakan, atau diubah. Tidak ada kode, migrasi, atau skrip yang dibuat |
| **Rencana acuan** | [Rencana MVP v0.2](../roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md) §5 B0-05, B2, B3-18..20 |
| **Status spesifikasi** | **COMPLETE** untuk desain. Semua aturan di bawah bersifat usulan sampai ADR disetujui |
| **Status pemetaan operasional** | **NOT VERIFIED / BLOCKED.** Tidak ada akses baca yang diotorisasi dan identitas instance yang terbukti (lihat B0-04 §1) |
| **Kesiapan migrasi** | **NOT READY.** Pemetaan operasional, keputusan PO, dan bukti dry-run belum ada |

Contoh dalam bagian ini bersifat **sintetis**. Nilai seperti `CUST-EXAMPLE-A` dan UUID `00000000-…` bukan identitas pelanggan nyata.

### 1. Model kepemilikan yang diusulkan

Aturan berikut adalah rekomendasi desain. Belum ada ADR yang menyetujuinya.

| Aspek | Aturan |
|---|---|
| Identitas klien | Entitas native `clients` adalah Customer Master kanonik. Setiap klien punya UUID yang tidak berubah dan kode stabil yang unik |
| Kolom kepemilikan | `spaces.client_id` dan `items.client_id` (atau asosiasi kepemilikan eksplisit yang setara). Nama fisik ditetapkan di task implementasi |
| `items.owner_id` | **Tidak dipakai untuk klien.** Kolom itu sudah berarti pengguna (B0-04 MIG-13) |
| Tenant | Tidak ada entitas Tenant. Instance tetap batas deployment (D-01) |
| Status kepemilikan Space | Tiga status yang bisa dibedakan: **CLIENT** (terhubung ke klien kanonik), **INTERNAL** (ditandai internal secara eksplisit), **UNRESOLVED** (belum ditetapkan) |
| NULL | **NULL tidak pernah berarti INTERNAL.** Hanya status INTERNAL eksplisit yang berarti internal |
| Satu Space | Satu Space = satu ruang lingkup kepemilikan |
| Item | Kepemilikan item stabil, tidak tergantung lokasi yang sedang dipilih |
| Lokasi | Setiap lokasi item harus cocok dengan ruang lingkup kepemilikan item |
| Multi-lokasi | Didukung di dalam satu klien, sesuai izin yang ada |
| Lintas klien | Ditolak: klien ke klien, dan klien ke internal |
| UNRESOLVED setelah enforcement | Dikarantina dari akses biasa. Tidak terlihat oleh siapa pun kecuali jalur rekonsiliasi |

**Ruang lingkup kepemilikan** = salah satu dari CLIENT(id), INTERNAL, atau UNRESOLVED. Tidak ada nilai lain.

### 2. Personal Space

| Aturan | Keterangan |
|---|---|
| Personal Space tetap privat dan berlingkup internal | Sesuai `itemsCaps` (B0-03 AUTH-006): personal hanya untuk owner |
| Item klien tidak boleh punya lokasi di Personal Space | Lokasi tersebut harus dipindahkan atau dilepas melalui jalur terkontrol |
| My Work dan saved view yang diizinkan tetap bisa memantau tugas klien | Tanpa memindahkan item |
| Visibilitas Personal Space **bukan** izin membaca konten klien | Penegasan dari rencana §8.2 |

### 3. Pemetaan Customer legacy

#### 3.1 Dua tahap yang harus dipisahkan

| Tahap | Pertanyaan | Hasil |
|---|---|---|
| **Penemuan definisi** | Field mana yang berfungsi sebagai Customer? | Daftar kandidat definisi field (ID UUID, cakupan, nama) |
| **Pemetaan data** | Opsi mana yang dipetakan ke klien kanonik mana? | Tabel pemetaan dengan status, bukti, dan persetujuan |

Penemuan definisi **tidak** menentukan pemetaan. Sebuah field bernama `Customer` belum tentu sumber yang sah.

#### 3.2 Identitas sumber komposit

Setiap opsi legacy diidentifikasi dengan tiga nilai:

1. **UUID field sumber**. Kunci yang dipakai di `items.fields`.
2. **UUID opsi sumber**. Dari `custom_fields.options`.
3. **Label opsi**. Hanya untuk tampilan dan audit, **tidak** untuk pencocokan.

UUID opsi adalah **referensi migrasi**, bukan UUID klien kanonik.

#### 3.3 Status pemetaan

| Status | Arti | Boleh menjadi aktif? |
|---|---|---|
| MAPPED | Satu opsi sumber menunjuk ke satu klien kanonik, dengan persetujuan | Ya |
| UNMAPPED | Belum ada keputusan | Tidak |
| AMBIGUOUS | Lebih dari satu kandidat klien, belum diputuskan | Tidak |
| CONFLICT | Dua keputusan berbeda untuk sumber yang sama, atau satu klien dari dua sumber yang bertentangan | Tidak |
| APPROVED_INTERNAL | Disetujui sebagai internal secara eksplisit | Ya, sebagai INTERNAL |
| QUARANTINED | Ditahan. Tidak masuk akses biasa | Tidak |

#### 3.4 Tabel spesifikasi (contoh sintetis)

| Source field ID | Source option ID | Source label | Target customer ID | Mapping status | Evidence | Approved by |
|---|---|---|---|---|---|---|
| `00000000-0000-4000-8000-0000000000f1` (contoh) | `00000000-0000-4000-8000-0000000000a1` (contoh) | `CUST-EXAMPLE-A — Contoh A` | `00000000-0000-4000-8000-0000000000c1` (contoh) | MAPPED | Keputusan tercatat, referensi tiket (contoh) | Pemilik bisnis (contoh) |
| `00000000-0000-4000-8000-0000000000f1` (contoh) | `00000000-0000-4000-8000-0000000000a2` (contoh) | `CUST-EXAMPLE-A — Contoh A (ejaan lain)` | (kosong) | AMBIGUOUS | Label mirip dengan opsi lain. **Tidak dicocokkan dengan label** | — |
| `00000000-0000-4000-8000-0000000000f1` (contoh) | `00000000-0000-4000-8000-0000000000a3` (contoh) | `Internal` | — | APPROVED_INTERNAL | Keputusan internal tercatat | Pemilik bisnis (contoh) |
| `00000000-0000-4000-8000-0000000000f2` (contoh, definisi kedua) | `00000000-0000-4000-8000-0000000000b1` (contoh) | `CUST-EXAMPLE-B` | (kosong) | QUARANTINED | Definisi duplikat, belum dipilih | — |

Baris contoh ini hanya menunjukkan format. Tidak ada baris nyata yang dibuat.

#### 3.5 Aturan pemetaan

1. **Tidak mencocokkan atau menggabungkan berdasarkan label.** Dua label yang sama dengan UUID berbeda tetap dua sumber.
2. **Tidak menyimpulkan kepemilikan dari nama Space atau nama project.**
3. **Nilai Customer yang kosong tidak berarti INTERNAL.** Item tanpa nilai masuk status UNRESOLVED sampai ada klasifikasi pemilik bisnis.
4. **Definisi ganda dan opsi yang bertentangan harus terdeteksi** dan tidak boleh diabaikan.
5. **Banyak opsi sumber boleh menunjuk ke satu klien** hanya setelah rekonsiliasi yang disetujui, dan keputusan itu dicatat sebagai bukti.
6. **Provenance dipertahankan**: field sumber, opsi sumber, dan label disimpan bersama target.
7. **Idempoten dan dapat diulang**: menjalankan pemetaan dua kali dengan masukan sama menghasilkan hasil sama, tanpa duplikat.
8. **Tidak menimpa pemetaan yang sudah disetujui** secara diam-diam. Perubahan harus melalui keputusan baru yang mencatat keputusan sebelumnya.

Penting: kolom `Target customer ID` tidak diisi di sini. Tidak ada klien kanonik yang dibuat pada tahap ini.

### 4. Matriks resolusi konflik

Setiap skenario memakai MIG-01..13 dari B0-04. Tidak ada skenario yang dikonfirmasi terjadi di data operasional.

| Skenario | Perlakuan yang diusulkan | Bukti yang dibutuhkan | Hasil pemetaan | Persetujuan | Migrasi berhenti atau karantina | MIG / task B3 |
|---|---|---|---|---|---|---|
| Space berisi project milik satu klien terverifikasi | Kandidat Space terikat klien | Semua nilai Customer di Space sama dan termap | MAPPED | Pemilik bisnis | Tidak | MIG-01 / B3-03, B3-18 |
| Space berisi project milik klien berbeda | Butuh rencana pemisahan atau pemindahan yang disetujui | Daftar project per klien | CONFLICT | Pemilik bisnis dan Security | Ya, Space itu dikarantina sampai ada rencana | MIG-01 / B3-18, B3-19 |
| Project tanpa Customer | Butuh klasifikasi eksplisit pemilik bisnis | Keputusan tertulis per project | UNRESOLVED (QUARANTINED) | Pemilik bisnis | Ya, untuk project itu | MIG-02 / B0-05 |
| Item punya banyak lokasi dalam satu klien | Pertahankan perilaku yang ada | Semua lokasi termap ke klien yang sama | MAPPED | Tidak perlu tambahan | Tidak | MIG-03 / B3-05 |
| Item punya lokasi lintas klien | Karantina dan rekonsiliasi | Daftar lokasi dan Space pemilik | CONFLICT | Pemilik bisnis dan Security | Ya, item itu | MIG-04 / B3-08, B3-18 |
| Item klien punya lokasi di Personal Space | Butuh pelepasan atau pemindahan terkontrol | Daftar lokasi personal | CONFLICT | Owner item dan pemilik bisnis | Ya, lokasi itu sampai dilepas | MIG-05 / B3-08 |
| Parent dan child punya pemilik berbeda | Tolak sampai rekonsiliasi | Pohon hierarki dengan pemilik | CONFLICT | Pemilik bisnis | Ya, cabang itu | MIG-06 / B3-08 |
| Dependensi atau relasi lintas pemilik | Butuh tinjauan. Jangan dihapus diam-diam | Daftar tautan dengan pemilik kedua ujung | CONFLICT | Pemilik bisnis | Tidak dihapus. Ditandai dan ditahan dari perubahan | MIG-07 / B3-10 |
| Penerima share belum punya membership klien | Pertahankan bukti audit. Akses ditolak sampai memenuhi syarat | Daftar share dan membership | MAPPED untuk share, akses ditolak | Admin klien | Tidak, share dibiarkan dan akses ditolak | MIG-08 / B3-11 |
| Item diarsipkan atau dihapus | Pertahankan kepemilikan historis dan referensi | Status arsip dan pemilik | MAPPED (historis) | Pemilik bisnis | Tidak | MIG-09 / B3-08 |
| Definisi Customer ganda | Pilih dan rekonsiliasi definisi sumber secara eksplisit | Daftar definisi dan jumlah nilai per definisi | AMBIGUOUS sampai dipilih | Pemilik bisnis | Ya, sampai dipilih | §3 / B0-05 |
| Referensi opsi Customer tidak valid | Tandai UNRESOLVED | Opsi yang hilang | UNRESOLVED | Pemilik bisnis | Ya, item itu | §3 / B0-05 |
| Referensi Pydio Folder hilang | Catat kesenjangan onboarding. Jangan menyimpulkan klien dari nama folder | Daftar project tanpa tautan | UNRESOLVED | Pemilik bisnis | Tidak, item tetap ada tanpa tautan | MIG-11 / B4-02 |

### 5. Aturan Blueprint, Automation, dan Request Form

#### 5.1 Blueprint dan Space Template

- Template generik boleh dipakai ulang lintas klien.
- Record khusus klien, file, field terbatas, dan referensi rahasia **tidak boleh disalin** ke klien lain.
- Pemilik Space tujuan menentukan kepemilikan pekerjaan baru.
- Pemeriksaan izin yang ada tetap wajib.
- Referensi sumber yang tidak aman atau ambigu ditolak.
- Isi `blueprints.tree` dan `saved_space_templates.body` belum diinspeksi (B0-04). Aturan ini berlaku sampai inspeksi itu dilakukan.

#### 5.2 Automation tingkat akun

- Definisi tingkat akun tetap boleh ada.
- Cakupan rule **tidak** memberi akses ke setiap klien.
- Target kepemilikan dan identitas eksekusi ditentukan **saat eksekusi**.
- Pemeriksaan kapabilitas dan kelayakan klien diterapkan ulang saat eksekusi.
- Mutasi lintas pemilik ditolak.
- Perilaku audit yang ada tetap berlaku.

#### 5.3 Request Form

- Space tujuan harus punya ruang lingkup kepemilikan yang diketahui.
- Otorisasi tujuan yang ada tetap berlaku (`checkDestination`, B0-03 WRITE-021).
- Pembuatan lintas Space hanya diizinkan dalam ruang lingkup kepemilikan yang kompatibel.
- Pengiriman publik dan unggahan eksternal yang belum disetujui tetap nonaktif atau dibatasi untuk MVP internal (B3-17).

Tidak ada mesin automation atau form baru yang dirancang di sini.

### 6. Urutan migrasi dan verifikasi

| Langkah | Isi | Keluaran | Status |
|---|---|---|---|
| 1 | Verifikasi identitas database yang berwenang | Nama host dan database disetujui PO | BLOCKED |
| 2 | Kumpulkan inventaris agregat | Jumlah per entitas (B0-04 §4) | NOT VERIFIED |
| 3 | Identifikasi dan setujui pemetaan Customer | Tabel §3.4 terisi dengan keputusan nyata | BLOCKED |
| 4 | Klasifikasi setiap Space yang relevan | CLIENT, INTERNAL, atau UNRESOLVED | BLOCKED |
| 5 | Deteksi lokasi dan relasi yang tidak konsisten | Daftar konflik per MIG | BLOCKED |
| 6 | Laporan dry-run konflik | Read-only, tanpa perubahan | BLOCKED |
| 7 | Setujui rencana remediasi atau karantina | Keputusan tertulis | BLOCKED |
| 8 | Backfill kepemilikan stabil | Idempoten, dengan checkpoint | Belum dimulai |
| 9 | Rekonsiliasi ID, hierarki, lokasi, grant, dan file | Laporan selisih nol | Belum dimulai |
| 10 | Aktifkan enforcement kepemilikan | Hanya setelah gerbang keamanan lulus | Belum dimulai |

#### 6.1 Syarat penerimaan minimum

- Tidak ada status UNRESOLVED yang diam-diam dianggap INTERNAL.
- Tidak ada lokasi item aktif yang tidak konsisten.
- Tidak ada relasi atau share lintas klien yang lolos sebagai bypass.
- Tidak ada ID item atau referensi historis yang hilang.
- Tidak ada akses konten implisit untuk Workspace Admin (D-17.1).
- Membership saja tidak memberi akses ke sumber daya.
- Semantik assignment Full-on-every-path tetap berlaku.
- Migrasi dan rollback tercatat dalam audit.

### 7. Keputusan yang menunggu ADR formal

| ID | Keputusan | Usulan | Pemilik |
|---|---|---|---|
| K-20 | Customer diturunkan dari owner Space, bukan field Customer | Setujui (sudah di rencana v0.2) | Pemilik produk |
| K-23 (baru) | Nama fisik kolom kepemilikan (`client_id`) dan hubungannya dengan `items.owner_id` | Setujui penamaan yang tidak bentrok | Arsitektur |
| K-24 (baru) | Status UNRESOLVED dan larangan NULL = INTERNAL | Setujui | Security, Arsitektur |
| K-25 (baru) | Personal Space tidak boleh punya lokasi item klien | Setujui | Pemilik produk, Security |
| K-26 (baru) | Akun tingkat automation dan blueprint: cakupan tidak memberi akses lintas klien | Setujui | Pemilik produk |
| K-27 (baru) | Formulir boleh membuat item di Space lain dalam ruang lingkup kompatibel | Setujui dengan batas ini | Pemilik produk |
| K-28 (baru) | Pemilik bisnis yang berwenang menyetujui pemetaan dan klasifikasi INTERNAL | Tentukan nama pemberi persetujuan | Pemilik produk, Legal |
| K-29 (baru) | Share dan grant polimorfik: FK atau pemeriksaan aplikasi | Tentukan | Arsitektur |

### 8. Batas data aktual

- **Pemetaan operasional: NOT VERIFIED / BLOCKED.** Tidak ada opsi sumber yang dipetakan.
- Tidak ada jumlah klien, Space, project, atau item yang diketahui.
- Tidak ada keputusan PO tentang Personal Space, blueprint, automation, atau formulir.
- Tidak ada klaim bahwa suatu konflik tidak ada.
- Kesiapan migrasi: **NOT READY.**

### 9. Status penyelesaian

| Bagian | Status |
|---|---|
| Model kepemilikan | COMPLETE (desain, perlu ADR) |
| Spesifikasi pemetaan legacy | COMPLETE (desain, perlu ADR) |
| Matriks konflik | COMPLETE (desain) |
| Aturan Personal Space, blueprint, automation, formulir | COMPLETE (desain) |
| Verifikasi migrasi | COMPLETE (urutan dan syarat) |
| Pemetaan operasional | NOT VERIFIED / BLOCKED |
| Kesiapan migrasi | NOT READY |
| **B0-05 secara keseluruhan** | **PARTIAL.** Spesifikasi selesai, pemetaan operasional wajib dan belum tersedia |
