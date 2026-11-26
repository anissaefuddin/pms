# Desain: Automation tingkat Workspace (AUT-02)

**Status: DESIGN APPROVED. IMPLEMENTED in the working tree (backend and API), NOT COMMITTED, NOT DEPLOYED.**
Implementasi dilakukan atas instruksi implementasi terpisah. Commit, push, dan deployment belum dilakukan.
Yang belum: UI Workspace Settings dan Space Settings (lihat bagian 10).

Tanggal: 18 Nov 2026. Dasar: keputusan user pada pesan persetujuan, dan kode yang ada di repo pada saat itu (lihat bagian 9).

---

## 1. Keputusan yang disetujui

| # | Keputusan | Konsekuensi desain |
|---|---|---|
| D1 | Workspace Automation memakai **status category generik** dan **Custom Field tingkat Workspace (ID workspace)**. Field tidak dicocokkan berdasarkan nama. Referensi tingkat Space hanya valid jika tersedia dan kompatibel di **seluruh** target. | Lihat 4.1 dan 4.2. |
| D2 | `target_space_ids` **wajib eksplisit** untuk aturan aktif, minimal satu Space. Draf boleh kosong. "Select All" menyimpan **snapshot daftar Space**, bukan wildcard. | Lihat 4.3. |
| D3 | Kuota awal **50 eksekusi/jam/aturan** dan **500 eksekusi/jam/workspace**, tetap menghormati kuota per Space. Semua limit **configurable** dan **tercatat di audit**. | Lihat 4.4. |
| D4 | Workspace Admin **tidak otomatis** mendapat akses ke konten Space. Setiap eksekusi tetap memerlukan izin **pemilik aturan** pada item dan Space terkait. Tidak ada akses ke Space confidential melalui registry, preview, log, atau automation. | Lihat 3 dan 4.5. |

---

## 2. Model data (target desain)

- `automation_rules.scope text NOT NULL DEFAULT 'space' CHECK (scope IN ('space','workspace'))`.
- Aturan `space`: tetap `space_id` wajib. Tidak berubah.
- Aturan `workspace`: `space_id` NULL, `target_space_ids uuid[]` (snapshot), `container_id` NULL (scope container hanya untuk aturan Space).
- Constraint: `(scope='space') = (space_id IS NOT NULL)`; untuk `scope='workspace' AND enabled`, `cardinality(target_space_ids) >= 1`.
- Draf workspace boleh `target_space_ids = '{}'`. Publish dan enable menolak jika kosong.
- Tabel `automation_limits` (singleton, `id=1`): `rule_per_hour int DEFAULT 50`, `workspace_per_hour int DEFAULT 500`, `space_per_hour int DEFAULT 500` (nilai per-Space yang sudah ada), `updated_by`, `updated_at`. Setiap perubahan menulis `audit_events` (`automation_limits.updated`) dengan nilai lama dan baru.
- Status workspace menyimpan **category** (`active`, `completed`, `deferred`, `cancelled`), bukan ID status Space.

---

## 3. Matriks permission

Legenda: ✓ diizinkan, ✗ ditolak, (O) hanya pemilik aturan, (S) hanya jika punya izin itu di Space terkait.

| Kemampuan | Workspace Admin | Pemilik aturan | Space Editor (non-pemilik) | Space Viewer | Pengguna lain |
|---|---|---|---|---|---|
| Membuat aturan **workspace** | ✓ | ✗ (kecuali admin) | ✗ | ✗ | ✗ |
| Membuat aturan **Space** | ✓ (S: Edit Space) | ✓ | ✓ | ✗ | ✗ |
| Melihat daftar aturan **workspace** | ✓ (metadata saja) | ✓ (aturan miliknya) | ✗ | ✗ | ✗ |
| Melihat nama Space target | ✓ (hanya Space yang juga bisa dilihat admin) | ✓ (Space yang bisa dilihat pemilik) | — | — | — |
| Mengedit / draf / publish | ✓ | ✓ (O) | ✗ | ✗ | ✗ |
| Mengaktifkan / menonaktifkan | ✓ | ✓ (O) | ✗ | ✗ | ✗ |
| Arsipkan / pulihkan | ✓ | ✓ (O) | ✗ | ✗ | ✗ |
| Pindah kepemilikan | ✓ | ✗ | ✗ | ✗ | ✗ |
| Melihat log eksekusi | ✓ (metadata: status, waktu, kode alasan; **tanpa judul item atau isi**) | ✓ (O) | ✗ | ✗ | ✗ |
| Try it (dry-run) | pemilik saja (O) | ✓ (O) | ✗ | ✗ | ✗ |
| Eksekusi pada item | **pemilik** di item dan Space (S) | **pemilik** (S) | — | — | — |
| Replay event gagal | ✓ | ✗ | ✗ | ✗ | ✗ |
| Duplikat / ekspor | pemilik atau admin (O) | ✓ (O) | ✗ | ✗ | ✗ |

Aturan utama:
- **Workspace Admin tidak mendapat hak konten.** Admin dapat mengelola aturan (metadata), tetapi eksekusi tetap memakai izin pemilik. Admin tidak bisa melihat judul, deskripsi, atau field item di log dan preview.
- Untuk Space **confidential** (konsep yang belum ada di kode, lihat 9): aturan workspace tidak boleh menyebut Space itu dalam target kecuali pemilik punya akses penuh. Daftar dan log tidak menampilkan namanya kepada orang yang tidak punya akses.

---

## 4. Penjelasan keputusan

### 4.1 Status category generik (D1)
- Kondisi dan aksi status di aturan workspace memakai `grp` (`active`, `completed`, `deferred`, `cancelled`), bukan `status_id`.
- Saat eksekusi, status tujuan dipilih di Space item: status pertama di grup yang sama pada workflow item. Jika tidak ada, eksekusi ditolak dengan `status_missing`.
- Guard workflow dan approval tetap berlaku lewat `PatchTx` dengan pemilik (sama seperti Space, sudah diuji di A1).

### 4.2 Custom Field tingkat Workspace (D1)
- Aturan workspace hanya boleh merujuk field dengan **ID workspace** (`00328_workspace_custom_fields.sql`), bukan field Space.
- Field Space hanya valid jika **tersedia dan kompatibel di seluruh target**: ada di setiap Space target dengan tipe yang sama. Jika tidak, aturan tidak bisa dipublish (422 `field_not_compatible`).
- Pencocokan **tidak pernah** berdasarkan nama. Dua field dengan nama sama di dua Space dianggap berbeda.
- Saat run di Space yang tidak punya field tersebut, aksi ditolak (`field_missing`) dan dicatat, Space lain tetap berjalan.

### 4.3 Target Space (D2)
- Aktif: `target_space_ids` minimal satu, wajib eksplisit.
- Draf: boleh kosong.
- "Select All" di UI menulis daftar Space yang terlihat saat itu ke `target_space_ids`. Space baru di kemudian hari **tidak** otomatis ikut.
- Setiap Space target diperiksa saat run: pemilik masih punya Edit di Space itu, Space belum diarsipkan, dan item berada di Space itu.

### 4.4 Kuota (D3)
- Per aturan workspace: **50/jam** (default, configurable di `automation_limits.rule_per_hour`).
- Per workspace: **500/jam** (default, configurable di `automation_limits.workspace_per_hour`).
- Per Space: tetap **500/jam** (sekarang), tetap berlaku untuk setiap run yang menyentuh Space itu.
- Semua limit tercatat di audit saat berubah. Kuota yang terlampaui menghasilkan `quota_blocked` dengan alasan (`rule_quota`, `workspace_quota`, `space_quota`).

### 4.5 Akses konten (D4)
- Log, registry, preview, dan dry-run tidak menampilkan konten item atau field yang tidak bisa dibaca pemilik (sama dengan A0, perluas ke log workspace).
- Notifikasi tetap lewat `CanView` per item (sudah ada).
- Tidak ada jalur di mana Workspace Admin dapat melihat konten Space lewat aturan.

---

## 5. Kompatibilitas aturan Space yang sudah ada

- Semua baris yang ada menjadi `scope='space'`. Tidak ada perubahan data.
- `GET /spaces/{id}/automation-rules`, `POST`, `PUT draft`, `publish`, `export`, `import`, dan `duplicate` tetap sama.
- Kuota per aturan Space tetap 100/jam (sekarang) dan kuota per Space tetap 500/jam. Batas workspace hanya berlaku pada aturan workspace.
- Log aturan Space tetap sama; kolom baru (`reason`, `action_type`) sudah ada sejak A1/A2.
- Engine: `Handle` tetap mencari aturan dengan `space_id = ANY(...)`, ditambah cabang `OR scope='workspace' AND target_space_ids && $spaces`. Aturan Space tidak terpengaruh.

---

## 6. Rencana migrasi (tidak dijalankan)

Dimulai setelah `00337`. Setiap langkah berdiri sendiri, bisa di-rollback, dan dijalankan terpisah.

| Langkah | Isi | Rollback | Catatan |
|---|---|---|---|
| M1 | `ALTER TABLE automation_rules ADD COLUMN scope text NOT NULL DEFAULT 'space' CHECK (...)`, `ADD COLUMN target_space_ids uuid[] NOT NULL DEFAULT '{}'` | drop kolom | Aman: default mengisi semua baris lama. |
| M2 | `ALTER COLUMN space_id DROP NOT NULL` + CHECK `(scope='space') = (space_id IS NOT NULL)` | kembalikan NOT NULL setelah semua baris `space` | Dijalankan dalam satu transaksi dengan M1. |
| M3 | Tabel `automation_limits` (singleton) dengan nilai default | drop tabel | Insert satu baris default. |
| M4 | Indeks `automation_rules_workspace_targets_idx` (GIN pada `target_space_ids`) | drop indeks | `CREATE INDEX CONCURRENTLY`. |
| M5 | Perluas CHECK `automation_runs.reason` jika perlu kode baru (`rule_quota`, `workspace_quota`, `field_not_compatible`) | kembalikan CHECK | Saat ini `reason` hanya `CHECK (char_length <= 60)`, tidak ada enum. |

Urutan deploy: M1–M3 dulu (tidak mengubah perilaku), lalu kode engine yang membaca `scope` (default `space`, perilaku sama), lalu UI workspace, lalu M4 bila data sudah cukup.

Data lama: tidak ada backfill. Seluruh aturan yang ada tetap `space`.

---

## 7. Matriks tes AUT-T02 dan terkait (direncanakan, belum dijalankan)

| ID | Skenario | Hasil yang diharapkan | Status |
|---|---|---|---|
| AUT-T02a | Workspace Admin membuat aturan workspace dengan 2 target | 201, scope workspace, target tersimpan | NOT RUN |
| AUT-T02b | Space Editor biasa membuat aturan workspace | 403 | NOT RUN |
| AUT-T02c | Publish atau enable aturan workspace dengan target kosong | 422 `target_required` | NOT RUN |
| AUT-T02d | Draf workspace dengan target kosong | 201, tidak aktif | NOT RUN |
| AUT-T02e | Select All menyimpan snapshot; Space baru setelahnya tidak ikut | target tetap, run tidak terjadi di Space baru | NOT RUN |
| AUT-T02f | Aturan workspace hanya berjalan di Space target | item di Space non-target tidak memicu | NOT RUN |
| AUT-T02g | Pemilik kehilangan Edit di satu Space target | run di Space itu `denied` `owner_no_access`; Space lain tetap berjalan | NOT RUN |
| AUT-T02h | Workspace Admin (bukan anggota Space) melihat log aturan workspace | metadata saja: status, waktu, alasan; tanpa judul atau isi item | NOT RUN |
| AUT-T02i | Workspace Admin mencoba eksekusi tanpa izin pemilik | tidak ada eksekusi; hak admin tidak dipakai | NOT RUN |
| AUT-T02j | Status category `completed` dipetakan ke status Space yang sesuai | item pindah ke status `completed` workflow-nya | NOT RUN |
| AUT-T02k | Status category tanpa padanan di workflow item | `status_missing`, Space lain tetap berjalan | NOT RUN |
| AUT-T02l | Field tingkat Space dengan tipe berbeda di satu target | publish ditolak 422 `field_not_compatible` | NOT RUN |
| AUT-T02m | Dua field nama sama di dua Space (ID berbeda) | tidak dicocokkan; field yang dipakai hanya ID workspace | NOT RUN |
| AUT-T02n | Kuota 50/jam per aturan terlampaui | `quota_blocked` `rule_quota`; kuota Space tidak berubah | NOT RUN |
| AUT-T02o | Kuota 500/jam per workspace terlampaui | `quota_blocked` `workspace_quota` | NOT RUN |
| AUT-T02p | Kuota per Space 500/jam tetap berlaku untuk run workspace | `quota_blocked` `space_quota` di Space itu | NOT RUN |
| AUT-T02q | Ubah `automation_limits` | audit `automation_limits.updated` dengan nilai lama dan baru | NOT RUN |
| AUT-T02r | Aturan Space lama setelah migrasi M1–M3 | perilaku, log, dan kuota sama (regresi) | NOT RUN |
| AUT-T02s | Space confidential: aturan workspace tidak menampilkan nama Space ke orang tanpa akses | nama tidak muncul di daftar atau log | NOT RUN (Space confidential belum ada, lihat 9) |
| AUT-T02t | Dry-run aturan workspace memakai izin pemilik, bukan admin | hasil sesuai pemilik; tidak ada kebocoran field | NOT RUN |

Test regresi wajib sebelum implementasi: seluruh `automation_integration_test.go`, `automation_guards_integration_test.go`, `automation_lifecycle_integration_test.go`, `automation_registry_integration_test.go`, dan Playwright `automation*.spec.ts`.

---

## 8. Langkah berikutnya (menunggu instruksi terpisah)

1. Finalisasi desain (dokumen ini) — **selesai dengan status APPROVED**.
2. Instruksi implementasi terpisah dari user. Belum dimulai.
3. Setelah instruksi: M1–M3 dulu, lalu kode engine dan API, UI, lalu tes sesuai bagian 7.

---

## 9. Keterbatasan yang diketahui saat desain

- **Space confidential** tidak ada di kode saat ini (tidak ditemukan di migrasi atau kode Go). Bagian 3 dan AUT-T02s mengandaikan konsep ini. Perlu definisi produk sebelum implementasi.
- **Custom Field Workspace** ada (`00328_workspace_custom_fields.sql`), tetapi perilaku kompatibilitas per Space (D1) perlu diverifikasi terhadap implementasi sebelum M-langkah dijalankan.
- **Zona waktu** deadline belum diputuskan (AUT-T18). Desain ini tidak mengubah keputusan itu.
- **Kalender kerja** yang dipakai aturan workspace adalah kalender instance (`work_schedules`, satu baris). Desain ini tidak mengubahnya.
- Belum ada nilai numerik untuk `space_per_hour` workspace selain 500 yang sudah ada per Space.

---

## 10. Status implementasi (working tree)

| Bagian | Status | Bukti |
|---|---|---|
| M1–M3: migrasi `00339` (scope, target_space_ids, space_id opsional, automation_limits, space_id pada run dengan backfill) | Dibuat, teruji lewat template database | semua tes automation lulus |
| D1: status category (`status_group`) dan Custom Field Workspace (`space_id NULL`). Field Space hanya jika satu target. | Dibuat | `TestWorkspaceRuleMovesByStatusCategory`, `TestWorkspaceRuleFieldsAreWorkspaceLevelOrOneTarget` |
| D2: target wajib untuk aktif, minimal satu; snapshot | Dibuat (API). "Select All" di UI belum. | `TestWorkspaceRuleNeedsATargetToBeEnabled`, `TestWorkspaceRuleRunsOnlyInItsTargets` |
| D3: 50/jam per aturan, 500/jam per workspace, 500/jam per Space; configurable; audit | Dibuat (API GET/PUT). UI belum. | `TestWorkspaceLimitsCountAndAreAudited` |
| D4: admin tidak mendapat hak konten; run memakai izin pemilik; log admin tanpa judul dan detail | Dibuat | `TestWorkspaceRuleRefusesOnlyWhereTheOwnerCannotAct`, `TestWorkspaceLogShowsOutcomesNotContentToAdmins` |
| Matriks permission | Dibuat (lihat bagian 3). Pemilik boleh edit dan melihat log; hanya admin yang membuat dan mengganti kepemilikan. | tes di atas |

### 10.1 Matriks tes AUT-T02 (hasil)

| ID | Hasil | Catatan |
|---|---|---|
| AUT-T02a | PASS | admin membuat aturan workspace (201) |
| AUT-T02b | PASS | editor ditolak (403) |
| AUT-T02c | PASS | enable tanpa target ditolak (422) |
| AUT-T02d | PASS | draf tanpa target boleh dibuat |
| AUT-T02e | PASS | item di target dikerjakan; item di luar target tidak |
| AUT-T02f | PASS | Space yang dibuat setelah snapshot bukan target |
| AUT-T02g | PASS | owner kehilangan akses di satu Space: Space itu ditolak (`owner_no_access`), Space lain jalan, aturan tidak dijeda |
| AUT-T02h | PASS | admin melihat hasil tanpa judul item |
| AUT-T02i | PASS (sebagian) | run hanya memakai izin pemilik; uji langsung admin-non-pemilik dicakup oleh T02g |
| AUT-T02j | PASS | kategori dipetakan ke status workflow item |
| AUT-T02k | PASS | kategori tanpa padanan: `status_missing`, aturan tidak dijeda |
| AUT-T02l | PASS | field Space dengan dua target ditolak; dengan satu target diterima |
| AUT-T02m | PASS | field dengan nama sama di Space lain tidak dicocokkan |
| AUT-T02n | PASS | batas 2/jam per aturan: `rule_quota` |
| AUT-T02o | PASS | batas workspace: `workspace_quota` |
| AUT-T02p | PASS | batas Space berlaku untuk aturan workspace: `space_quota` |
| AUT-T02q | PASS | perubahan batas tercatat di audit; editor ditolak; nilai di luar rentang ditolak 422 |
| AUT-T02r | PASS | seluruh suite automation Space lulus tanpa perubahan |
| AUT-T02s | NOT RUN | konsep Space confidential belum ada di kode |
| AUT-T02t | NOT RUN | dry-run per Space untuk aturan workspace belum diuji secara khusus |

### 10.2 Keterbatasan yang diketahui

- **Workspace Settings dan Space Settings (UI)** belum dibuat. Saat ini hanya API.
- **"Select All"** di UI belum ada. Target hanya bisa diisi lewat API.
- **Batas workspace** hanya bisa diubah lewat API, belum lewat UI.
- **Aturan workspace yang bekerja di banyak Space** dijalankan sekali per item, di Space target yang terkecil id-nya tempat item berada. Item yang berada di lebih dari satu target hanya dijalankan di satu Space.
- **Aturan workspace dan Try it:** dry-run per Space belum diuji khusus (AUT-T02t).
- **Commit:** migrasi `00338`, `00339` harus di-commit bersama migrasi `00333`–`00337`.

---

## 11. UI yang sudah dibuat (working tree)

| Bagian | Berkas | Keterangan |
|---|---|---|
| Workspace Settings > Automation (admin) | `web/src/routes/_app/settings/automation.tsx` | Daftar aturan workspace, status, target, pemilik, log terakhir, dan aktif/nonaktif. Hanya admin. |
| Batas eksekusi (D3) | `web/src/components/workspace-automation.tsx` (`WorkspaceLimitsCard`) | Tiga angka; setiap perubahan dicatat di audit oleh server. |
| Target dan "Select all" (D2) | `web/src/components/workspace-automation.tsx` (`TargetsDialog`) | "Select all" mengambil snapshot Space yang terlihat admin. Target yang tidak terlihat dipertahankan dan dihitung di layar. |
| Form aturan workspace | `WorkspaceRuleDialog` + `AutomationEditor scope="workspace"` | Kategori status, tanpa pemicu status Space, tanpa uji coba (Try it) karena butuh satu Space. |
| Daftar di Space (read-only) | `spaces_.$spaceId.automation.tsx` | Nama dan status aturan workspace yang bekerja di Space itu. Pemilik dan isi tidak ditampilkan (endpoint `GET /spaces/{id}/workspace-automation`). |
| Navigasi | `settings-sidebar.tsx`, `settings.tsx` | Entri "Workspace automation" untuk admin di grup Konfigurasi Kerja. |

Tes browser: `web/tests/automation-workspace.spec.ts` (3 tes: halaman dan batas; "Select all" mengirim snapshot; Space menampilkan nama dan status saja). Tes automation yang lama (`automation.spec.ts`, `automation-registry.spec.ts`) ikut lulus: 12/12.

### 11.1 Yang masih belum

- **Try it untuk aturan workspace** di UI belum tersedia. Dry-run API per Space belum diuji khusus (AUT-T02t).
- **Pemilih Space di editor aturan Space** tidak berubah.
- **Konsep Space confidential** belum ada (AUT-T02s).
- **Pembuatan aturan workspace** di UI menggunakan pembuat sebagai pemilik, sesuai desain.

---

## 12. Keputusan penutupan (disetujui user)

### 12.1 Confidential Space: DEFERRED
- Mode visibility `standard/confidential` didefinisikan sebagai rancangan untuk Space Settings, belum dibangun.
- AUT-T02s ditandai **DEFERRED** sampai fitur Confidential Space tersedia.
- Sementara, akses dibatasi dengan permission Space yang sudah ada. Tes `TestUnauthorizedSpaceIsHiddenWithTheExistingPermissions` (PASS): non-anggota mendapat 404 untuk daftar aturan Space, pemakaian Space, log, dan duplikasi aturan.
- Tidak ada model confidentiality baru dalam scope AUT-02.

### 12.2 Pemakaian kuota (AUT-16): DONE
- Endpoint `GET /api/v1/automation/usage` (`?rule_id`, `?space_id`, atau tanpa parameter untuk admin). Penghitung yang sama dengan enforcement (`usageOf` di `usage.go`).
- Rolling window 1 jam. State: `ok`, `warning` dari 80%, `exhausted` dari 100%. `throttled_by` memberi alasan: `rule_quota`, `workspace_quota`, `space_quota`.
- Hak akses: pemilik atau admin untuk aturan, admin untuk total workspace, editor Space untuk Space itu. Lainnya 404 atau 403.
- Tampilan: halaman Workspace Settings (kartu batas) dan halaman Automation Space.
- Batas tetap configurable (`automation_limits`) dan tercatat di audit.
- Tes: `TestUsageStatesAndThrottlingOrder` (unit), `TestUsageWarnsAndThrottles`, `TestUsageIsShownOnlyToThoseWhoMayManageIt` (integrasi dan negatif).

### 12.3 Eksekusi multi-target
- Migrasi `00340`: `automation_runs.matched_space_ids`. Setiap run mencatat `matched_space_ids` (semua target yang cocok) dan `space_id` (tempat eksekusi).
- **Penentuan Space eksekusi** (`resolveExecutionSpace`): lokasi primer item di antara target yang cocok. Urutan: lokasi root sebelum subitem, lalu `rank` (urutan yang dilihat orang), lalu nama Space, lalu id sebagai urutan total terakhir.
- **Tidak memakai UUID terkecil sebagai aturan bisnis.** Id hanya penutup urutan total dan tidak pernah menentukan hasil bisnis.
- **Tidak ada fallback** ke Space lain: jika owner tidak berhak di Space itu, run ditolak di Space itu (`owner_no_access`).
- **Idempotensi:** unik `(rule_version, event, action)` memastikan satu eksekusi per event. Resolver deterministik, sehingga event yang dikirim ulang tetap di Space yang sama.
- Status guard, RBAC, dan kuota tetap berlaku di Space eksekusi.
- Tes: `TestMultiTargetRunsOnceInThePrimarySpace` (PASS: satu eksekusi, Space primer, dua Space tercatat, event ulang tidak menjalankan lagi), `TestMultiTargetNeverFallsBackToAnotherSpace` (PASS: negatif, tidak ada fallback, item tidak berubah di Space tanpa hak).

### 12.4 Risiko yang tersisa
1. **Space asal event tidak tersedia.** Event domain tidak membawa Space tempat perubahan terjadi, sehingga "originating Space" belum bisa diprioritaskan. Resolver memakai lokasi primer. Untuk memakainya, event perlu membawa Space asal; ini perubahan di semua titik emit.
2. **Tie-break rank lalu nama Space** bisa berbeda dari yang diharapkan bila dua lokasi punya rank yang sama. Perlu konfirmasi aturan bisnisnya.
3. **Aturan workspace di banyak Space** memakai satu pemilik. Jika Space primer item tidak berhak bagi pemilik, run ditolak di Space itu dan tidak dipindahkan ke Space lain. Ini sesuai desain (tidak ada fallback), tetapi berarti item yang primernya di Space tanpa hak tidak akan diproses meski Space lainnya berhak.
4. **Confidential Space** belum ada; AUT-T02s deferred.
5. **Commit:** migrasi `00333` sampai `00340` harus di-commit bersama. Belum ada commit, push, merge, atau deployment.

---

## 13. Penutupan risiko AUT-02 (kontrak)

### 13.1 Perubahan kontrak
| Kontrak | Perubahan | Kompatibilitas |
|---|---|---|
| Domain event `origin_space_id` | Kolom nullable (migrasi `00341`). `events.Event.OriginSpaceID`. `events.WithOrigin(ctx, space)`: event yang dipancarkan di bawah ctx itu membawa origin jika belum diisi. | Event lama tetap valid (NULL). Dispatcher membaca kolom ini. |
| Emitter yang dimigrasi | Event yang dipancarkan oleh aksi aturan (dalam Space eksekusi) dan `form.submitted` (Space form). Emitter lain belum: origin kosong. | Event lama dan emitter belum migrasi diproses dengan owning Space. |
| Resolusi konteks | Originating → owning (tepat satu target) → SKIPPED (`ambiguous_context`, `origin_outside_targets`, `not_in_target`). Tanpa rank, nama, atau UUID terkecil. | Aturan Space tidak berubah. |
| Run | `space_id` (Space eksekusi, NULL bila di-skip) dan `matched_space_ids`. Kedua-duanya ada di respons log (`execution_space_id`, `matched_space_ids`). | Field baru, aditif. |

### 13.2 Hasil tes
| Tes | Hasil |
|---|---|
| `TestResolveExecutionSpace` (unit, tanpa DB) | PASS: origin yang valid; origin di luar target tidak diganti; origin tidak bisa di-parse ditolak; owning tunggal; ambigu; tidak ada target; urutan target tidak mengubah hasil. |
| `TestMissingOriginRunsInTheOwningSpaceWhenUnambiguous` | PASS: event lama, satu target cocok, berjalan di Space itu. |
| `TestMissingOriginAndTwoSpacesIsSkippedAsAmbiguous` | PASS: tanpa origin, dua target: di-skip dengan `ambiguous_context`, matched dicatat, Space eksekusi NULL. |
| `TestInvalidOriginIsSkippedWithoutFallback` | PASS (negatif): origin di luar target, tidak ada run di Space mana pun, alasan `origin_outside_targets`. |
| `TestValidOriginRunsOnceAndStampsItsChanges` | PASS: satu run di origin; perubahan yang dibuat aturan membawa origin itu; event yang dikirim ulang tidak menjalankan lagi. |
| `TestOriginWithoutAccessIsRefusedWithoutFallback` | PASS (negatif): origin tanpa akses owner ditolak `owner_no_access` di Space itu; target lain yang berhak tidak dipakai. |
| `TestEventOriginIsStoredWhenKnown` | PASS: origin tersimpan jika diketahui, NULL jika tidak. |
| `TestUnauthorizedSpaceIsHiddenWithTheExistingPermissions` | PASS (negatif): non-anggota tidak melihat Space. Confidential Space tetap DEFERRED. |
| Regresi automation (unit dan integrasi) | PASS. Dua tes lama berbasis rank dihapus karena digantikan oleh tes origin di atas. |

### 13.3 Keputusan yang dicatat
- AUT-T02s: **DEFERRED** (Confidential Space belum dibangun; tidak diimplementasikan dalam AUT-02).
- AUT-16: **DONE** (lihat bagian 12.2).
- Multi-target: **DONE** dengan kontrak di atas.

### 13.4 Sisa risiko sebelum AUT-02 dinyatakan DONE
1. **Emitter belum dimigrasi.** Perubahan langsung oleh orang (item, komentar, approval) belum membawa origin. Untuk item di banyak Space, event seperti itu di-skip (`ambiguous_context`) sampai emitter-nya dimigrasi. Ini aman tetapi bisa melewatkan aturan.
2. **Event yang dikirim ulang oleh replay** dengan origin lama tetap memakai origin yang tersimpan; tidak ada re-resolusi.
3. **Tidak ada UI untuk melihat alasan skip konteks** selain log (kode alasan sudah tampil di log).
4. **Confidential Space** DEFERRED.
5. **Commit, push, merge, dan deployment** belum dilakukan. Migrasi `00333` sampai `00341` harus di-commit bersama.
