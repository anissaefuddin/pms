# LENS RBAC — Penutupan Temuan dan Verifikasi Akhir

Diperbarui 20 Nov 2026. Baseline: `docs/security/` (tujuh dokumen audit). Branch `feat/m12c-ui-clean`.

Commit yang sudah ada dari batch ini (tidak ada commit baru di sesi ini):
`240da43` (S-4 fix), `ac87a0a` (tes S-4), `3a72f25` (S-1), `4a512ee` (S-5), `fbb0952` (S-2), `2576f74` (dokumen).

Tidak ada push, merge, deploy, atau perubahan database production.

## 1. Keputusan D-17 dan keselarasan dokumen

**D-17.1 Explicit Access Only: APPROVED** (20 Nov 2026).

Workspace Admin tidak otomatis mendapat `full` atau akses ke konten Shared Space. Akses konten mengikuti share, grup, Space Admin eksplisit (D-SA1), User Type Ceiling, dan inheritance. Workspace Admin tetap mengelola fungsi administratif lewat capability (`manage_users`, `create_space`, dan endpoint admin lain).

**Kode tidak diubah.** `policy/engine.go` (`spaceCaps`) sudah sesuai keputusan ini sejak `f8b772b`.

**Dokumen yang dikoreksi (riwayat tetap ada):**

| Dokumen | Perubahan |
|---|---|
| `docs/DECISIONS.md` | D-17 ditandai SUPERSEDED untuk bagian admin otomatis `full`. Baris baru D-17.1 (APPROVED). D-SA1 tetap. D-05 tetap berarti stack dan progres unggah. |
| `docs/ARCHITECTURE.md` §6.2 poin 5 | Aturan admin workspace disesuaikan dengan D-17.1. |
| `docs/ARCHITECTURE.md` §14 poin 1 | Ditandai superseded oleh D-17.1. |
| `docs/SPACE_MANAGEMENT_ANALISA.md` §12.1 | Ditambahkan catatan keputusan 20 Nov. Isi historis tidak diubah. |
| `docs/BACKLOG.md` (catatan 12 Nov) | Ditandai superseded oleh D-17.1. |

**Pemisahan D-05 dan D-SA1:** selesai di commit `fbb0952`. Referensi Space Admin di kode, OpenAPI, dan UI memakai D-SA1. D-05 untuk stack (`DECISIONS.md`) dan D-05 untuk progres unggah (`web/src/lib/upload.ts`) tidak berubah.

## 2. Ringkasan status temuan

| ID | Status | Bukti |
|---|---|---|
| S-1 Space Admin hanya admin/regular | **FIXED** (`3a72f25`) | `TestSpaceAdminRequiresAdminOrRegularUserType`: gagal (204) tanpa fix, lulus dengan fix |
| S-2 Dokumentasi dan D-17 | **DECISION → APPROVED** (D-17.1), dokumen selaras | Lihat bagian 1 |
| S-4 Revoke, downgrade, upgrade share item | **FIXED** (`240da43`, tes `ac87a0a`) | `TestItemShareRevokeAboveOwnCapabilitiesIsRefused`: 200 tanpa fix, 403 dengan fix. `TestItemShareDowngradeAndUpgradeAgainstOwnRights` lulus |
| S-5 Audit event move | **FIXED** (`4a512ee`) | `TestMoveWritesHistoryOnlyWhenItSucceeds`: 0 event tanpa fix, 1 dengan fix. `TestConcurrentMovesRecordExactlyTheSuccessfulOnes` lulus |

## 3. Regresi authorization final (Task B)

Tes baru: `server/internal/httpserver/authorization_final_regression_integration_test.go`, `TestAuthorizationFinalRegressionExplicitAccessOnly`. Nama tes cocok dengan pola CI `authorization-matrix`.

| Persyaratan Task B | Subtest | Hasil |
|---|---|---|
| Workspace Admin tanpa share tidak bisa membaca Shared Space | `workspace admin without a share cannot read a shared Space or its items` | Lulus (404) |
| Workspace Admin dengan share sah mendapat hak efektif | `workspace admin with an explicit read share gets exactly the effective rights` | Lulus (hanya `view`; PATCH 403) |
| Admin tidak bisa membuka Personal Space orang lain karena user type | `workspace admin cannot open another user's personal space…` | Lulus (404 untuk space dan item) |
| Space Admin tunduk pada User Type Ceiling | Tidak ada subtest baru. Dicakup oleh `TestSpaceAdminRequiresAdminOrRegularUserType`: external, collaborator, viewer ditolak sebagai Space Admin | Lulus (lewat S-1) |
| Membership dan revoke langsung memengaruhi hak efektif | `membership revoke takes effect on the next request` | Lulus (200 lalu 404) |
| Move dan share aman lintas Space | `move and share stay bounded across spaces and parent and child` | Lulus untuk lintas Space (403). **Parent/child belum diuji langsung** |
| Direct API tidak bisa melewati permission check | `direct API calls cannot bypass the permission check` | Lulus (PATCH, DELETE, PUT shares 403; anonim 401; admin tanpa share 404) |

**Catatan cakupan.** Subtest move hanya menguji lintas Space. Move item sebagai anak dari parent yang tidak boleh ditulis belum punya tes khusus. Itu tercatat sebagai risiko tersisa.

## 4. Hasil pengujian

Database: PostgreSQL 16.15 terisolasi (container lokal, dihapus setelah selesai). `PMS_TEST_DATABASE_URL` diset ke database itu.

**Go**

```
cd server
go test ./internal/policy/...                                                      # ok
go test ./internal/httpserver/ -run 'TestAuthorizationFinalRegressionExplicitAccessOnly'  # PASS, 6 subtest
go test ./internal/httpserver/ -run 'TestSpaceAdminRequiresAdminOrRegularUserType|TestSpaceGovernance' # ok
go test ./internal/httpserver/ -run 'TestItemShare|TestMove|TestConcurrentMoves|TestReplaceItemLocations' # ok
go test ./... -count=1 -timeout 15m                                                # semua paket ok, tanpa FAIL
```

**Perintah yang sama dengan job CI `authorization-matrix`:**
```
go test -count=1 ./server/internal/httpserver ./server/internal/policy -run 'Authorization|AccessRules|Matrix|Capabilit'
```
Hasil: exit 0. **Tetapi cakupannya sempit:** hanya 8 tes di `httpserver` yang cocok, dan `policy` tidak cocok satu tes pun. Tes S-1, S-4, dan S-5 tidak termasuk. Ini tidak sesuai dengan klaim ARCHITECTURE §6.3 (matriks lengkap).

**E2E Playwright (Chromium, `PLAYWRIGHT_CHANNEL=chrome`):**
```
npx playwright test tests/auth.spec.ts tests/roles.spec.ts tests/share.spec.ts tests/navigation.spec.ts tests/view-navigation.spec.ts
# 44 passed
npx playwright test tests/space-settings.spec.ts
# 10 passed
```

**Web unit dan typecheck:** `npm test` 109 dari 109 lulus. `tsc` dan `eslint` bersih untuk file yang diubah.

**Tidak dijalankan:**
- Job CI lain (`test`, `parity-e2e`) untuk commit ini. Hanya tes yang disebut di atas.
- E2E Playwright penuh di seluruh repo.
- Verifikasi UI di dev-lens setelah perubahan ini (belum di-deploy).

## 5. Perubahan dalam batch ini

Kode produksi tidak diubah di sesi ini. Perubahan sudah di-commit di batch S-1, S-2, S-4, dan S-5 (lihat daftar commit di atas). Sesi ini hanya menambahkan tes dan dokumen:

- `server/internal/httpserver/authorization_final_regression_integration_test.go` (baru)
- `docs/security/LENS_EFFECTIVE_PERMISSION_INSPECTOR_SPEC.md` (baru)
- `docs/security/LENS_RBAC_ENHANCEMENT_ROADMAP.md` (diperbarui)
- `docs/security/LENS_RBAC_FINDINGS_CLOSURE.md` (diperbarui)
- `docs/DECISIONS.md`, `docs/ARCHITECTURE.md`, `docs/SPACE_MANAGEMENT_ANALISA.md`, `docs/BACKLOG.md` (koreksi, D-17.1)

## 6. Regression risks

- **S-1:** siapa pun yang sudah menjadi Space Admin sebagai external, collaborator, atau viewer tetap admin. Dev-lens saat ini tidak punya kasus seperti itu, tetapi database lain bisa punya. Tidak ada migrasi otomatis.
- **S-4:** pemegang custom role `share` tanpa `delete` tidak lagi bisa mengubah share di atas haknya. Itu perubahan yang disengaja.
- **S-5:** setiap move sukses menambah satu baris `activity`. Tampilan Activity belum punya label untuk kunci `locations`.
- **D-17.1:** tidak ada perubahan kode. Dokumen sekarang menyatakan Explicit Access Only sebagai keputusan final.

## 7. Remaining security risks

1. **Pola CI `authorization-matrix` tidak mencakup tes baru** (S-1, S-4, S-5). Regresi di area itu tidak akan terdeteksi di job tersebut. Usulan: perluas pola atau ubah nama tes (perubahan `ci.yml`, belum dilakukan).
2. **Move parent/child belum punya tes khusus.** Skenario: pengguna memindahkan task ke bawah parent yang tidak boleh ia tulis.
3. **Penurunan tipe user** tidak mencabut Space Admin yang sudah ada (lihat S-1).
4. **Audit move di `audit_events` (log admin) belum dibuat.** Hanya riwayat item di `activity`.
5. **Tidak ada inspector** untuk melihat hak efektif. Pengguna dan admin harus menebak sumber hak dari UI. Ini tertutup oleh Option C.

## 8. Kesiapan Option C dan Option B

**Option C (Effective Permission Inspector): READY FOR IMPLEMENTATION** dengan dua prasyarat terbuka:
1. Persetujuan UX oleh pemilik produk (spesifikasi ada di `LENS_EFFECTIVE_PERMISSION_INSPECTOR_SPEC.md`).
2. Pola CI `authorization-matrix` diperluas agar INS-10 dan INS-11 ikut berjalan.

Prasyarat teknis lain sudah terpenuhi: D-17.1 APPROVED, S-1, S-4, S-5 ditutup dengan tes, regresi authorization final lulus.

**Option B (Granular Capabilities): DEFERRED.** Mapping custom role masih berubah (S-4 baru saja mengubah aturan delegasi), dan model kapabilitas belum stabil. Pertimbangkan lagi setelah Option C memberi data tentang kebutuhan nyata.

## 9. Security Gate

**CONDITIONAL.**

Alasan:
- Tidak ada P0 terbuka. Empat temuan (S-1, S-2, S-4, S-5) sudah ditutup.
- Semua tes regresi yang diminta lulus, termasuk E2E authorization dan navigasi.
- Tetapi job CI `authorization-matrix` tidak menjalankan tes baru (lihat bagian 7, butir 1), dan move parent/child belum diuji (butir 2).

Gate akan menjadi **PASS** setelah dua hal itu ditutup. Tidak perlu perubahan kode produksi untuk itu.

## 10. Pembaruan: isolasi commit, dua prasyarat, dan residual risk (20 Nov 2026, lanjutan)

### 10.1 Isolasi commit

- Commit `30ee14f` (dokumen D-17.1 dan tes regresi) sudah ter-push dan tidak diubah. Memisahkannya berarti menulis ulang history publik, yang tidak dilakukan.
- Perubahan baru dipisah menjadi commit lokal berikut, masing-masing dengan scope sendiri. Tidak ada push.

| Commit | Scope |
|---|---|
| (1) | `fix(items)`: pemeriksaan Space terkunci dibaca sebelum lokasi lama dihapus (`items/location.go`), dan tes `authorization_locked_space_integration_test.go` |
| (2) | `fix(policy)`: hak Space Admin hanya berlaku untuk tipe `admin`/`regular` saat dipakai (`policy/engine.go`, `policy/locked.go`, `policy/item.go`), dan tes `authorization_space_admin_downgrade_integration_test.go` |
| (3) | `test(items)`: regresi move parent/child (`move_parent_child_regression_integration_test.go`) |
| (4) | `ci`: pola `authorization-matrix` dengan penjaga hasil (`.github/workflows/ci.yml`) |
| (5) | `docs(security)`: bagian ini dan roadmap |

Sesi lain: tidak ada file dari sesi lain yang di-stage. `docs/qa/usertest.md` dan berkas `avatar`/`server.go` tetap di working tree.

### 10.2 Temuan baru: bypass Space terkunci (P1, FIXED)

**Root cause.** `replaceLocations` (`items/location.go`) menghapus lokasi lama di dalam loop, lalu memanggil `policy.LockedSpacesOf` dan memeriksa Space Admin. Karena lokasi di Space terkunci sudah dihapus, `LockedSpacesOf` mengembalikan kosong, sehingga aturan D-95 tidak berlaku.

**Dampak.** Member Full yang bukan Space Admin bisa memindahkan item keluar dari Space terkunci. Ini terjadi sebelum batch ini, bukan karena penurunan tipe user.

**Bukti sebelum fix.** `TestAuthorizationLockedSpaceBlocksNonAdminMoveOut`: `a Full non-admin cannot move an item out of a locked Space: status 200, want 403`.

**Perbaikan.** Pemeriksaan dan `restricted` dipindah ke sebelum loop penghapusan. Tes lulus setelah perbaikan.

### 10.3 Residual risk 1: penurunan tipe user (FIXED, data tidak diubah)

**Temuan.** Setelah tipe user Space Admin diturunkan ke external, flag `is_space_admin` tetap `true` dan pemeriksaan Space terkunci masih meloloskannya. Bukti: `TestAuthorizationSpaceAdminDowngradeLosesSpaceAuthority` gagal dengan `is_space_admin:true` sebelum fix.

**Perbaikan.** Hak Space Admin hanya dihitung jika tipe user saat ini `admin` atau `regular`. Diterapkan di empat tempat: `SpaceAccessFor` (flag), `IsSpaceAdmin`, query Space locked, dan query visibilitas item. Penegakan langsung berlaku tanpa mengubah data.

**Catatan.** Penurunan tipe juga mencabut sesi user (401), jadi jalur ini hanya relevan untuk sesi baru.

**Opsi remediation yang tidak diambil:**
- (a) Mencabut baris `space_admins` saat tipe user turun. Perubahan perilaku pada alur `ChangeType`. Tidak diambil karena belum ada permintaan.
- (b) Migrasi data untuk baris yang sudah tidak memenuhi syarat. Audit dev-lens: 0 baris. Tidak perlu migrasi saat ini.

**Yang tetap berlaku:** baris `space_admins` lama tetap ada di database. Itu tidak memberi hak, tetapi terlihat di daftar admin. Admin Space bisa mencabutnya lewat UI.

### 10.4 Residual risk 2: audit move (DIEVALUASI, tidak ditambah)

**Yang sudah tercatat.** Setiap move yang mengubah lokasi menulis satu baris `activity` dengan `actor_id`, `item_id`, `from`/`to` (Space dan parent), dan `created_at`, dalam transaksi yang sama. Gagal = rollback.

**Yang tercatat di `audit_events`.** Perubahan share (`auditShares`), 2FA, dan operasi admin lain. Move tidak ditulis ke sana.

**Penilaian.** `activity` memenuhi kebutuhan riwayat item: siapa, kapan, dari mana, ke mana. Move mengubah visibilitas hanya lewat lokasi, dan perubahan hak itu sendiri (share) sudah diaudit di `audit_events`. Jadi tidak ada celah audit yang terbukti.

**Rekomendasi.** Tambahkan `audit_events` hanya untuk move lintas Space jika produk membutuhkan log admin untuk itu. Tidak untuk setiap move, untuk menghindari duplikasi. Tidak diimplementasikan karena belum ada kebutuhan yang dinyatakan.

### 10.5 Dua prasyarat Security Gate

1. **Pola CI `authorization-matrix` (ditutup).** Pola sekarang: `Authorization|AccessRules|Matrix|Capabilit|SpaceAdmin|ItemShare|Move|ConcurrentMoves|ReplaceItemLocations`, ditambah `./server/internal/policy` penuh. Langkah gagal jika jumlah `--- PASS` kurang dari 23, ada `--- SKIP`, atau ada `--- FAIL`. Script langkah dijalankan lokal: `passed=33 skipped=0 failed=0`, exit 0.
2. **Regresi move parent/child (ditutup).** `TestMoveParentChildInheritanceValidationCycleAndRollback`, enam subtest: inheritance setelah move (dan berakhir saat share dicabut), validasi source, validasi destination, cycle, rollback, dan lintas Space. Semua lulus.

### 10.6 Hasil pengujian

- `go test ./... -count=1 -timeout 15m` pada PostgreSQL 16.15 terisolasi: exit 0, tanpa FAIL. `httpserver` 495,9 detik, `policy` lulus.
- Pola CI dijalankan dengan script langkah yang sama: passed=33, skipped=0, failed=0.
- E2E dan web tidak dijalankan ulang untuk perubahan Go di batch ini. E2E authorization dan navigasi (44/44) dan Space settings (10/10) lulus sebelumnya.

## 11. Security Gate (diperbarui)

**PASS untuk cakupan kode batch ini**, dengan catatan:
- Kedua prasyarat sudah terpenuhi, dan residual risk 1 sudah diperbaiki, residual risk 2 sudah dievaluasi.
- Belum di-deploy. Dev-lens masih menjalankan build sebelum batch ini.
- E2E tidak dijalankan ulang setelah perubahan Go.
- Commit belum di-push; pola CI baru belum berjalan di GitHub.
