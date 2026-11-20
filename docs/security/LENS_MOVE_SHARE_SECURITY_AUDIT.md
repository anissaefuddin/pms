# LENS — Audit Move dan Share (Phase 3)

## C1. Operasi move

Move dilakukan oleh `items.ReplaceLocations` → `replaceLocations` (`server/internal/items/location.go`) dalam transaksi `inSerializable` dengan kunci per lokasi (`items/location.go:67-83`, `:116`).

| Persyaratan | Implementasi | Bukti | Status |
|---|---|---|---|
| 1. Akses ke source | `View` wajib, else 404 | `location.go:88` | CONFIRMED (kode) |
| 2. Hak ubah source parent | `Edit` pada item | `location.go:91` | CONFIRMED (kode) |
| 3. Hak di destination | `View` (404), lalu `Create` (403) | `location.go:171-175` | CONFIRMED (kode) |
| 4. allowed-children | `checkAllowedUnder` dan aturan "task hanya boleh berisi task" | `location.go:180-187`, `items/item_types.go:354` | CONFIRMED (kode) |
| 5. Batas Space | placement per Space, locked Space | `location.go:146-152` | CONFIRMED (kode) |
| 6. Tanpa siklus | `would put the item inside itself` | `location.go:207`; **probe lulus, siklus ditolak 422** | CONFIRMED (kode + runtime) |
| 7. Hitung ulang permission | keputusan tiap request | `policy` tanpa cache | CONFIRMED (kode) |
| 8. Descendant ikut aturan baru | pewarisan dari parent baru | `policy/item.go` | CONFIRMED (kode), NOT TESTED runtime |
| 9. Tidak ada kebocoran | `View` di destination dicek sebelum menulis | `location.go:171` | CONFIRMED (kode) |
| 10. Atomik | `inSerializable`, retry sampai 15 kali | `items/create.go:52-56` | CONFIRMED (kode) |
| 11. Audit log | NOT TESTED. Tidak diperiksa di audit ini | — | NOT TESTED |
| 12. Race antara auth dan perubahan parent | Serializable dan kunci lokasi | `location.go:116`, `items/create.go:52` | CONFIRMED (kode), NOT TESTED runtime |

**Probe C1 (dijalankan):** `server/internal/httpserver/item_location_cycle_audit_integration_test.go`. Parent dipindah ke bawah child, lalu child ke bawah parent. Hasil: 422 `this location would put the item inside itself`, dan tidak ada lokasi tertulis. Status **NOT A BUG**.

## C2. Operasi share

Entry point: `httpserver/items.go:16` (`itemShares`), `spaces/item_shares.go:14` (`ItemShares`), `httpserver/spaces.go:250` (`replaceSpaceShares`), `spaces/spaces.go:401` (`ReplaceShares`).

| Persyaratan | Implementasi | Bukti | Status |
|---|---|---|---|
| Hak share wajib | `Share` pada item, `Share` pada Space | `item_shares.go:42`, `spaces.go:414` | CONFIRMED (kode) |
| Grant tidak boleh di atas hak pemberi | `grantable.Contains(caps)` untuk role baru | `spaces/spaces.go:516-518` | CONFIRMED (kode) |
| Tidak mencabut akses di atas hak pemberi (Space) | `guardSpaceShareChange` | `spaces/members.go:58-73` | CONFIRMED (kode) |
| Tidak mencabut akses di atas hak pemberi (item) | Diperiksa seperti Space setelah perbaikan S-4 | `spaces/spaces.go:521-534` | CONFIRMED (kode) + probe lulus |
| Concurrency | `If-Match` dan `UPDATE ... WHERE version=$2` | `item_shares.go:50-58` | CONFIRMED (kode), NOT TESTED runtime |
| Space terkunci | hanya admin, untuk Space | `spaces/spaces.go:428` | CONFIRMED (kode) |
| Personal space | tidak bisa dibagikan | `item_shares.go:42`, `spaces.go:421` | CONFIRMED (kode) |
| Audit share | `auditShares` dipanggil saat versi dikirim | `httpserver/items.go:31` | CONFIRMED (kode), NOT TESTED runtime |

## Skenario SHR

| ID | Status |
|---|---|
| SHR-01 editor memberi full | Ditolak oleh `grantable` (`spaces.go:518`). CONFIRMED (kode) |
| SHR-02 full membagikan ke external | Dibatasi plafon external (grant di atas plafon ditolak). CONFIRMED (kode) |
| SHR-03 full menghapus share orang lain | Pemegang custom role tanpa delete ditolak 403 (probe S-4). Full tanpa batas tetap berlaku, sesuai hak pemberi | CONFIRMED (runtime) |
| SHR-04 hapus direct, inherited tetap | CONFIRMED (kode), NOT TESTED runtime |
| SHR-05 API langsung tanpa hak | Ditolak 403 oleh `item_shares.go:42`. CONFIRMED (kode) |
| SHR-06 share di Space yang tidak bisa diakses | 404 (`item_shares.go:27`). CONFIRMED (kode) |
| SHR-07 membership dicabut, akses lama | Keputusan dihitung ulang per request. CONFIRMED (kode), NOT TESTED runtime |
| SHR-08 dua perubahan bersamaan | `If-Match` menghasilkan 409 pada perubahan kedua. CONFIRMED (kode), NOT TESTED runtime |
| SHR-09 forged actor ID | Actor berasal dari sesi, bukan body. NOT TESTED |
| SHR-10 external memberi full | Ditolak oleh `grantable` dan plafon. CONFIRMED (kode) |

## Temuan share

**S-4 (P2, CONFIRMED, DIPERBAIKI): share item yang lebih tinggi bisa dicabut oleh pemegang `share` yang tidak punya `delete`.**
- Bukti kode: pada target item, `writeTargetShares` menghapus semua share yang tidak ada di `desired` (`spaces/spaces.go:526-531`) tanpa membandingkan dengan hak pemberi. Pemeriksaan "di atas hak sendiri" hanya ada untuk Space (`members.go:73`).
- Kasus: custom role dengan mask `share` tanpa `delete` (dimungkinkan karena mask bebas, `spaces/spaces.go:471-482`). Pemegang role itu bisa menghapus share `full` orang lain pada item yang sama.
- Dampak: pembatalan hak orang lain oleh pemegang yang lebih lemah. Bukan eskalasi.
- Status: SUSPECTED. Belum ada tes yang memverifikasi. Perlu tes sebelum fix.

**S-5 (NOT TESTED): audit log untuk move.** Belum diperiksa.
