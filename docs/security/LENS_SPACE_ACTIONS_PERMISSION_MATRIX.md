# LENS — Matriks Hak Akses Aksi Space (dikoreksi ke kode aktual)

Dasar: D-17.1 (Explicit Access Only) dan D-SA1 (Space Admin eksplisit, hanya tipe `admin`/`regular`).
Setiap baris diverifikasi terhadap kode `main` (`b36f843`). Tidak ada perubahan permission engine.

## Prinsip
- Hak aksi = effective capability (union share ∩ User Type Ceiling), bukan nama role.
- Full share **tidak** memberi Space Admin. Aksi yang memakai `is_space_admin` hanya untuk Space Admin eksplisit.
- Menu hanya menampilkan aksi yang boleh dilakukan. Backend tetap menjadi sumber keputusan.

## Matriks

| Aksi | Hak yang dipakai di kode | Sumber kode | Status | Catatan |
|---|---|---|---|---|
| Open Space | `view` | `spaces.go:383` (View) | Ada | — |
| Copy link | `view` | — (tautan `/spaces/{id}`, client) | Ada (UI) | Tidak perlu endpoint |
| Add/Remove Favorite (Space) | — | `/stars` hanya untuk item (project, folder, task) | **Tidak tersedia** | Butuh dukungan backend; tidak dibuat di batch ini |
| Hide from my sidebar | — | — | **Tidak tersedia** | Preferensi pribadi belum ada untuk Space |
| Rename Space | `edit` | `spaces.go:221` (Update, Edit) | Ada | Bukan Space Admin. UI menyembunyikan untuk Personal dan arsip |
| Ubah icon | `edit` | `spaces.go:221` | Ada | Warna Space tidak ada di model data |
| Members & access | `share` | `spaces.go:414` (ReplaceShares, Share) | Ada | **Bukan** Space Admin (koreksi dari usulan) |
| Kelola Space Admin | Space Admin eksplisit | `governance.go` (`administeredSpace`) | Ada | Hanya tipe `admin`/`regular` (D-SA1) |
| Manage views & tools | `edit` | `views.go:121` | Ada | Per view |
| Space settings (buka halaman) | `view` | — | Ada | Kontrol edit mengikuti capability masing-masing |
| Archive Space | Space Admin eksplisit | `governance.go:87` (`administeredSpace`) | Ada | Koreksi: bukan `full`. Tampilkan hanya jika `is_space_admin` dan bukan Personal |
| Delete Space | `delete` | `spaces.go:293` (Delete) | Ada | **Keputusan produk**: `full` share saat ini punya `delete`. Tidak diubah |
| Duplicate Space | — | Tidak ada endpoint | **Tidak tersedia** | Butuh spesifikasi kloning |

## Koreksi terhadap usulan awal
1. Rename dan Members & access **tetap** memakai `edit` dan `share`, bukan Space Admin (sesuai keputusan Batch 1).
2. Favorite dan Hide from sidebar **tidak tersedia** untuk Space. Itu pekerjaan backend, bukan UI.
3. Archive memakai Space Admin eksplisit. Ini sudah sesuai usulan.
4. Duplicate Space belum punya endpoint.

## Keputusan yang masih terbuka
- **Delete Space oleh `full` share**: apakah harus tetap `delete` (kondisi sekarang) atau dibatasi Space Admin eksplisit.
- **Favorite dan Hide untuk Space**: perlu endpoint dan preferensi pribadi sebelum masuk menu.
