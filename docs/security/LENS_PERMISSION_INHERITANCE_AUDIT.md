# LENS — Audit Pewarisan Permission (Phase 1)

Sumber kebenaran: `server/internal/policy/item.go` (CTE `ancestry`), `policy/engine.go`, `policy/policy.go`.

## A1. Resolusi permission

| # | Pertanyaan | Temuan | Status |
|---|---|---|---|
| 1 | Share parent berlaku ke descendant | Ya. CTE menaikkan rantai parent lalu mengambil `space` dan `item` sebagai target (`item.go:50-75`) | CONFIRMED (kode) |
| 2 | Share child tidak memperluas parent | Ya. Rantai hanya naik, tidak turun | CONFIRMED (kode) |
| 3 | Pewarisan berhenti di batas Space | Ya. Join `ancestry` memakai `l.space_id = a.space_id` (`item.go`, baris rekursif) | CONFIRMED (kode) |
| 4 | Banyak share digabung | Ya. Union semua share yang cocok (`policy.go:150-156`) | CONFIRMED (kode) |
| 5 | Plafon user type selalu diterapkan | Ya. `union & ceilings[t]` (`policy.go:156`) | CONFIRMED (kode, unit test) |
| 6 | Tanpa akses implisit lintas Space | Ya untuk Space shared dan personal (`engine.go`, `item.go:61`, `:77`) | CONFIRMED (kode) |
| 7 | Hapus direct share, inherited tetap berlaku | Ya, karena keputusan dihitung ulang tiap request dari tabel share | CONFIRMED (kode). NOT TESTED runtime |
| 8 | Perubahan parent menghasilkan perhitungan baru | Ya, tidak ada cache keputusan per item. Cache 10 detik di `httpserver/access_policy.go:17` khusus kebijakan akses workspace, bukan permission item | CONFIRMED (kode) |
| 9 | Cache permission usang | Tidak ditemukan cache permission item | CONFIRMED (kode) |
| 10 | Siklus parent tidak menyebabkan bypass | Siklus ditolak: `items/location.go:207` ("would put the item inside itself"). Walau begitu CTE memakai `UNION` (bukan `UNION ALL`), jadi rekursi tetap berhenti | CONFIRMED (kode + probe, lihat A2) |

## A2. Matriks uji

| ID | Status | Catatan |
|---|---|---|
| INH-01..INH-07 | Sebagian. Aturan kode CONFIRMED, runtime NOT TESTED | Dicakup oleh `TestReplaceItemLocations` dan tes matriks di CI |
| INH-08 | NOT TESTED | Perilaku sesuai rumus union ∩ plafon |
| INH-09 | CONFIRMED (kode) | `item.go` hanya memberi hak lewat share |
| INH-10 | CONFIRMED (kode) | Join `space_id` |
| INH-11 | NOT TESTED | Perilaku sesuai kode |
| INH-12 | NOT TESTED | Move diuji di `item_locations_integration_test.go` untuk kasus hak tulis, bukan hak descendant setelah move |
| INH-13 | **CONFIRMED lewat probe**: `TestMoveTaskUnderOwnDescendantIsRefused` lulus, siklus ditolak dengan 422 | Lihat `LENS_MOVE_SHARE_SECURITY_AUDIT.md` |

## Temuan pewarisan

Tidak ada temuan P0 atau P1 pada pewarisan. Hipotesis "siklus task→task lolos" **dibantah** oleh probe: status **NOT A BUG**.
