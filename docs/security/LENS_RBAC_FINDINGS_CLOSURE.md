# LENS RBAC — Penutupan Temuan (S-1, S-2, S-4, S-5)

Baseline: `docs/security/` (tujuh dokumen audit). Branch `feat/m12c-ui-clean`. Tidak ada commit, push, merge, atau deploy dalam batch ini kecuali yang disebut di bagian "Perubahan" (S-4 sudah di-commit sebelumnya sebagai `240da43`). Tidak ada perubahan pada database production.

## Ringkasan status

| ID | Status | Ringkasan |
|---|---|---|
| S-1 | **FIXED** (backend, UI) + **DECISION** untuk audit data | Space Admin hanya untuk user type `admin`/`regular`. Data dev-lens sudah bersih (0 tidak memenuhi syarat). |
| S-2 | **DECISION** (konflik D-17) | ARCHITECTURE disesuaikan dengan kode. D-SA1 menggantikan D-05 untuk Space Admin. Keputusan produk D-17 belum ada. |
| S-4 | **FIXED** (commit `240da43`) | Revoke, downgrade, dan upgrade share item dibatasi hak pemberi. |
| S-5 | **FIXED** | Move menulis satu event riwayat per perubahan nyata, dalam transaksi yang sama. Gagal = rollback termasuk event. |

## S-1 — Space Admin

**Root cause.** `GrantAdmin` dan `TransferOwnership` menulis `space_admins` tanpa memeriksa user type. Kapabilitas Space Admin hanya dipotong plafon, sehingga External dapat menjadi admin tanpa hak yang dijanjikan.

**Bukti sebelum fix.** `TestSpaceAdminRequiresAdminOrRegularUserType` gagal: `a external user cannot become Space Admin: status 204, want 422`.

**Perubahan.**
- `server/internal/spaces/governance.go`: fungsi `requireAdminEligible` (baru), dipanggil di `GrantAdmin` dan `TransferOwnership` setelah pemeriksaan keanggotaan. Menolak dengan 422 untuk tipe selain `admin` dan `regular`.
- `web/src/components/space-settings.tsx`: tipe `SpaceMember` menerima `user_type`. Kandidat Space Admin difilter ke `admin` dan `regular`. API members sudah mengirim `user_type`, jadi tidak ada perubahan API.

**Yang tidak berubah.** Share editor, limited, read, dan full untuk semua tipe. Tes `TestSpaceGovernance` tetap lulus.

**Audit data (read-only, dev-lens, `losaka-app`).**
```
SELECT count(*) FROM space_admins sa JOIN users u ON u.id=sa.user_id
WHERE u.user_type NOT IN ('admin','regular');   -- hasil: 0
```
Tidak ada data yang perlu dimigrasi saat ini.

**Risiko tersisa.** Jika tipe user sebuah admin diturunkan setelah ditetapkan, baris `space_admins` tetap ada. Tidak ada mekanisme otomatis yang mencabutnya. Usulan: mencabut Space Admin saat tipe user berubah (perubahan terpisah, memengaruhi alur `ChangeType`). Tidak diimplementasikan di batch ini.

## S-2 — Dokumentasi

**Temuan baru yang mengubah S-2.** Ada dua sumber yang bertentangan, dan keduanya perlu keputusan:
- `docs/DECISIONS.md` D-17 dan `ARCHITECTURE.md` §14 menyatakan admin otomatis `full` di semua Space shared.
- Kode sejak commit `f8b772b` menerapkan opsi A: admin tanpa hak implisit (`policy/engine.go`, `spaceCaps`).
- `docs/SPACE_MANAGEMENT_ANALISA.md` §12.1 mencatat opsi A pernah dicoba lalu dibatalkan, dan butuh keputusan produk (A1 atau A2).

Jadi kode dan dokumen keputusan memang berbeda, dan keputusan ulang diperlukan. Batch ini tidak mengubah perilaku kode.

**Perubahan.**
- `docs/ARCHITECTURE.md` §6.2 poin 5 dan §14 poin 1: menyatakan kondisi kode sekarang, dan menandai keputusan produk sebagai terbuka.
- `docs/DECISIONS.md`: baris D-17 diberi status konflik. Baris baru **D-SA1** untuk Space Admin eksplisit.
- Referensi D-05 untuk Space Admin diganti D-SA1 di: `policy/locked.go`, `policy/engine.go`, `httpserver/spaces_governance.go`, `httpserver/space_governance_integration_test.go`, `web/src/components/space-settings.tsx`, `api/openapi.yaml`, dan kode yang digenerasi ulang (`server/internal/apigen/openapi.gen.go`, `web/src/api/schema.d.ts`).
- D-05 tetap berarti keputusan stack (`DECISIONS.md` baris 17) dan D-05 progres unggah (baris 233, `web/src/lib/upload.ts`).
- Tidak diubah: `server/migrations/00346_space_governance.sql`, `docs/SPACE_MANAGEMENT_ANALISA.md`, dan log `docs/qa/`. Itu catatan historis.

## S-4 — Pencabutan share

**Root cause.** `writeTargetShares` (`spaces/spaces.go`) membatasi grant role baru, tetapi menghapus atau menurunkan share item tanpa membandingkan dengan hak pemberi. Untuk Space, pemeriksaan ini sudah ada (`guardSpaceShareChange`).

**Bukti sebelum fix.** `TestItemShareRevokeAboveOwnCapabilitiesIsRefused`: `sharer revoking a Full holder: status 200`.

**Perubahan (commit `240da43`).** Blok `else` di `writeTargetShares`: untuk target selain Space, share yang dihapus atau diubah harus berada dalam kapabilitas pemberi. Jika tidak, 403.

**Matriks uji (`share_delegation_integration_test.go`, `item_share_revoke_audit_integration_test.go`).**

| Kasus | Expected | Hasil |
|---|---|---|
| Revoke share Full (pemberi: view+share, tanpa delete) | 403 | 403 (sebelum fix: 200) |
| Downgrade Full → Limited | 403, nilai tidak berubah | 403, tetap Full |
| Upgrade Read → Limited | 403, nilai tidak berubah | 403, tetap Read |
| Pemberi menjaga share Read yang tidak berubah | 200 | 200 |
| Pemberi menghapus share Read (dalam hak) | 200, baris hilang | 200 |
| Admin (Full dengan delete) menurunkan Full → Read | 200 | 200 |

**Yang tidak berubah.** Grant role baru di atas hak pemberi tetap 403 (`spaces.go`, blok pertama). Plafon user type tetap berlaku.

## S-5 — Audit move

**Root cause.** `replaceLocations` (`items/location.go`) menulis lokasi tanpa event apa pun. Di `activity` hanya ada event untuk `delete` (`detach`) dan perubahan lain, tidak untuk move. `audit_events` juga tidak mencatat move.

**Bukti sebelum fix.** `TestMoveWritesHistoryOnlyWhenItSucceeds`: `a successful move must write one history event, got 0`.

**Perubahan.**
- `items/location.go`: snapshot lokasi sebelum dan sesudah (`locationsSnapshot`), lalu satu baris `activity` (`action='update'`, `changes.locations.from/to`, `actor_id`) di transaksi yang sama. Tidak ditulis bila tidak ada perubahan (`IS DISTINCT FROM`).
- Timestamp berasal dari `activity.created_at`. Item, actor, source, destination, dan Space tercatat dalam `from`/`to` (`space_id`, `parent_item_id`).

**Matriks uji (`move_audit_integration_test.go`, `move_race_integration_test.go`).**

| Kasus | Expected | Hasil |
|---|---|---|
| Move sukses A → B | tepat 1 event | 1 (sebelum fix: 0) |
| Move gagal setelah lokasi pertama ditulis | 404, tanpa event, lokasi pertama di-rollback | 404, 1 event tetap, 0 lokasi |
| 8 move paralel ke 4 Space | tanpa event palsu; setiap event adalah perubahan nyata; rantai mulai dari Space awal | lulus 3 kali berturut-turut |

**Catatan.** Move ke lokasi yang sudah ada mengembalikan 200 tanpa event. Itu disengaja (`same list again changes nothing`). Event hanya menggambarkan perubahan nyata.

**Risiko.** Tabel `activity` bertambah satu baris per move sukses. Tampilan Activity dan Stream sudah memakai label kolom (`locations` belum punya label; perlu ditambahkan jika ingin ditampilkan dengan rapi). Itu belum dikerjakan.

## Hasil pengujian

Perintah yang dijalankan terhadap PostgreSQL 16.15 terisolasi (container lokal, dihapus setelah selesai):

```
export PMS_TEST_DATABASE_URL="postgres://pms:audit@localhost:<port>/postgres?sslmode=disable"
cd server
go test ./internal/policy/...                                                  # ok
go test ./internal/httpserver/ -run 'Inherit|Ancestor|Locat|Access|Custom|Recycle'  # ok
go test ./internal/httpserver/ -run 'Governance|SpaceAdmin|Admin|Ownership|Transfer' # ok
go test ./internal/httpserver/ -run 'Move|Location|Share|Revoke|Downgrade|Concurrent' # ok
go test ./internal/spaces/... ./internal/items/...                             # ok
go test ./... -count=1 -timeout 15m                                            # lihat di bawah
```

Hasil akhir suite penuh (`go test ./... -count=1`): semua 24 paket lulus, tanpa FAIL. Paket `httpserver` (415 detik) memuat seluruh tes integrasi termasuk tes baru di atas. Web: `npm test` 109 dari 109 lulus.

Web:
```
cd web && npx tsc --noEmit -p . && npx eslint src/components/space-settings.tsx   # lulus
```

Tidak dijalankan: E2E Playwright dan matriks `authorization-matrix` di CI.

## Perubahan (file)

| File | Status |
|---|---|
| `server/internal/spaces/governance.go` | diubah (S-1) |
| `server/internal/items/location.go` | diubah (S-5) |
| `web/src/components/space-settings.tsx` | diubah (S-1, komentar D-SA1) |
| `server/internal/policy/locked.go`, `policy/engine.go`, `httpserver/spaces_governance.go` | komentar D-SA1 |
| `api/openapi.yaml` | komentar D-SA1 |
| `server/internal/apigen/openapi.gen.go`, `web/src/api/schema.d.ts` | digenerasi ulang |
| `docs/ARCHITECTURE.md`, `docs/DECISIONS.md` | diubah (S-2) |
| `server/internal/httpserver/space_admin_eligibility_integration_test.go` | baru (S-1) |
| `server/internal/httpserver/share_delegation_integration_test.go` | baru (S-4) |
| `server/internal/httpserver/move_audit_integration_test.go`, `move_race_integration_test.go` | baru (S-5) |
| `server/internal/httpserver/space_governance_integration_test.go` | komentar D-SA1 |
| `docs/security/LENS_RBAC_FINDINGS_CLOSURE.md` | baru (dokumen ini) |

**Perhatian sebelum commit.** Dua file generated (`openapi.gen.go`, `schema.d.ts`) juga memuat tiga baris `AvatarVersion` yang sudah ada sebelumnya di working tree dan berasal dari sesi lain. Jika di-commit, perubahan itu ikut masuk. Pisahkan atau minta sesi pemilik perubahan itu menyelesaikannya dulu.

## Risiko regresi

- S-1: mengubah siapa yang bisa menjadi Space Admin. Admin/regular tidak terpengaruh.
- S-4: pencabutan share item oleh pemegang custom role yang tidak punya seluruh hak pemberi sekarang ditolak. Admin dan Full tidak terpengaruh.
- S-5: satu baris `activity` per move sukses. Tidak ada perubahan skema.

## Risiko keamanan tersisa

1. **Keputusan D-17 belum ada** (P1 untuk produk, bukan eksploitasi): admin workspace tanpa akses ke Space shared. Laporan dan approval workspace terpengaruh.
2. **Space Admin yang tipe user-nya diturunkan** tetap menjadi admin (lihat S-1).
3. **S-1 UI/E2E** belum diuji di browser.
4. **Audit move di `audit_events`** (log admin) tidak dibuat. Hanya riwayat item.
5. **Kebijakan dan gate** untuk perubahan role custom yang memuat `share` tanpa `delete` sekarang lebih ketat. Dokumentasi custom role perlu diperbarui.

## Rekomendasi Option C dan Option B

**Option C (Effective Permission Inspector): layak dilanjutkan**, setelah D-17 diputuskan. Inspector perlu menampilkan asal hak admin workspace. Tanpa keputusan itu, tampilannya akan menyajikan aturan yang belum final.

**Option B (Granular Capabilities): belum layak sekarang.** S-1 dan D-17 masih terbuka, dan Option B mengubah pemetaan custom role dan migrasi. Tunggu sampai model hak untuk admin workspace dan Space Admin sudah final.

## Security Gate

**CONDITIONAL.** Tidak ada P0 terbuka. Empat temuan sudah ditutup atau diputuskan (S-1 dan S-4 diperbaiki, S-5 diperbaiki, S-2 dicatat sebagai keputusan). Gate tidak PASS karena: D-17 belum diputuskan, UI S-1 dan E2E belum dijalankan di browser, dan matriks `authorization-matrix` belum dijalankan lokal.
