# LENS — Audit Space Admin (Phase 2)

## B1. Model

Space Admin diimplementasikan sebagai **flag eksplisit**, bukan share role, dan bukan user type.

- Tabel `space_admins`, dicek lewat `policy.IsSpaceAdmin` (`policy/locked.go:58`).
- Hak efektif Space Admin: kapabilitas Full dipotong plafon user type (`policy/engine.go:123`).
- Share Full saja **tidak** membuat Space Admin (`engine.go:123`, `apigen/openapi.gen.go:5644`).
- Yang diberi hak tambahan dari flag: penempatan di Space terkunci (`items/location.go:152`, `items/create.go:511`).
- Pengelolaan anggota Space: hanya lewat kapabilitas `share` (`spaces/spaces.go:414`, `policy.CanOnSpace(…, policy.Share, …)`).

## B2. Hak Space Admin per fungsi

| Fungsi | Mekanisme | Hasil untuk External Space Admin |
|---|---|---|
| Rename / edit Space | `policy` edit | Tergantung plafon editor: diizinkan |
| Kelola anggota dan role | kapabilitas `share` | **Ditolak** (external tidak punya `share`) |
| Ubah role sendiri | `guardSpaceShareChange` + grant ceiling | Ditolak (tidak punya `share`) |
| Create/Folder/Project | `create` | Diizinkan jika ada |
| Workflows, custom fields, item types | perlu cek per fungsi; NOT TESTED | — |
| Archive dan delete Space | `spaces/governance.go` | NOT TESTED |
| Transfer ownership | `transferSpaceOwnership` | NOT TESTED |

## B3. Skenario SPA

| ID | Skenario | Expected | Actual (kode) | Status |
|---|---|---|---|---|
| SPA-01 | Workspace Admin tanpa share membuka Shared Space | 404 | 404, karena admin tidak punya hak implisit (`engine.go:117`, D-17 opsi A) | CONFIRMED (kode). Bertentangan dengan ARCHITECTURE §6.2 poin 5 |
| SPA-02 | Regular membuat Space | Diizinkan | Diizinkan (`spaces/spaces.go:159-160`) | CONFIRMED (kode). Collaborator, external, viewer ditolak |
| SPA-03 | External ditetapkan sebagai Space Admin | Dibatasi | Hak efektif tetap editor, tanpa `share`. Tidak bisa kelola anggota | **CONFIRMED (kode), DESIGN DECISION** (lihat temuan S-1) |
| SPA-04 | Collaborator mengubah membership | 403/404 | Ditolak: tidak ada `share` (`policy.go:100`) | CONFIRMED (kode) |
| SPA-05 | Space Admin mengubah role dirinya | Ditolak | Ditolak, karena `guardSpaceShareChange` dan ceiling | CONFIRMED (kode) |
| SPA-06 | Space Admin memberi izin di atas plafonnya | Ditolak | Ditolak: `you cannot grant a role above your own capabilities` (`spaces/spaces.go:518`) | CONFIRMED (kode) |
| SPA-07 | Admin membuka Personal Space orang lain | 404 | 404 (`spaces/spaces.go`, `spaceCaps` mengembalikan `none`) | CONFIRMED (kode) |
| SPA-08 | User dikeluarkan lalu memakai endpoint lama | 404 | Hak dihitung ulang tiap request dari tabel share. NOT TESTED runtime | CONFIRMED (kode), NOT TESTED |
| SPA-09 | Space Admin mengubah konfigurasi workspace | Ditolak | Kapabilitas workspace hanya dari tipe user (`server.go:272-273`) | CONFIRMED (kode) |
| SPA-10 | Akses Space settings lewat URL langsung | 404/403 | Bergantung pada `Caps` dan pemeriksaan endpoint. NOT TESTED | NOT TESTED |

## Temuan

**S-1 (P1, DESIGN DECISION): explicit Space Admin tidak bisa mengelola anggota jika bertipe External, Collaborator, atau Viewer.**
- Bukti: `spaces/spaces.go:414` memeriksa `policy.Share`. Plafon external (`policy/policy.go:109`) dan collaborator (`:110`) tidak memuat `share`, jadi Space Admin dari tipe itu tidak pernah lolos.
- Ketidaksesuaian: UI menyatakan "Space Admins manage members, visibility and archive" (`web` settings Space, teks members).
- Dampak: bukan eskalasi. Justru Space Admin tidak berfungsi sesuai janji UI. Pilihannya: menghapus janji dari UI, atau memberi Space Admin hak kelola anggota terpisah dari `share`. Keputusan ada di pemilik produk.

**S-2 (P2, DOKUMEN): dua sumber yang tidak sepakat soal Admin di Space shared.**
- ARCHITECTURE §6.2 poin 5: admin otomatis `full`. Kode dan D-17 opsi A: tidak ada hak implisit.
- Nomor keputusan: `apigen/openapi.gen.go:5644` dan `spaces/members.go` menulis "D-05", tetapi `docs/DECISIONS.md` memakai D-05 untuk hal lain. Referensi perlu dirapikan.

**S-3 (NOT TESTED): kelompok SPA-08, SPA-10, dan hak Space Admin atas archive, delete, dan transfer.** Belum dijalankan.
