# Gap Tampilan Pass 2, Lane A (detail item dan aksi item)

Gambar yang dibuka: Editing an item 2, Setting a date 2, Adding a comment 3, Adding an approval 3, Sharing a project 3 (nama folder tidak selalu sama dengan isi gambar). Flow lain di lane ini memakai panel yang sama (Info panel, chip, komposer) dan dinilai dari gambar-gambar itu plus kode. Pass 1 (D-110) sudah menutup chip sebaris dan tombol Save per blok.

| Flow | Gambar rujukan | UI aktual | Gap | Prio | Status |
|---|---|---|---|---|---|
| Changing a status | Adding a comment 3 (chip In Progress berwarna) | `<select>` status polos | Status tidak berwarna | P2 | Selesai: tint warna workflow (D-150) |
| Adding an assignee | Adding a comment 3 (avatar + nama) | Chip hanya teks nama | Tanpa avatar | P2 | Selesai: `AvatarStack` di chip |
| Editing an item / View project detail | Adding an approval 3 (baris "3 subitems, 1 file") | Tidak ada baris penghitung | Ringkasan jumlah | P2 | Selesai: baris penghitung subitem/file/komentar (approval ditunda: item belum punya `approval_count`) |
| Adding a comment | Adding a comment 3 | Kartu komentar berbingkai tanpa avatar/waktu; komposer polos | Avatar, waktu, komposer berkartu | P2 | Selesai (placeholder "Try sharing..." dan ikon emoji/format: Ditunda P3, butuh ekstensi Placeholder tiptap) |
| Setting a date | Setting a date 2 (popover dua bulan + durasi + Working days only) | Popover dua pemilih tanggal, hari kerja dikunci | Kalender dua bulan, kolom Duration, Clear dates | P3 | Ditunda: kalender tunggal `DatePicker` bersama lane lain |
| Adding an approval | Adding an approval 3 | Blok approval sudah berkartu dengan pemilih approver | Pola sudah setara; "Advanced settings" tidak ada | P3 | Ditunda |
| Sharing a project | Sharing a project 3 | Dialog share sudah memuat daftar "Shared with", peran, Copy link (D-110 s.d. pass 1) | Toggle Selective sharing tidak ada | P3 | Ditunda: butuh API pewarisan (tanpa API baru tidak mungkin) |
| Adding a file / description / subitem | Adding an approval 3 ("Add files", "Go to full details") | Blok sudah ada dan fungsional | Tanpa gap struktur berarti | - | Selesai (tanpa perubahan) |

Ringkasan: 5 Selesai, 3 Ditunda (semua P3), 0 P1/P2 terbuka.
Bukti: `web/tests/items.spec.ts` (tes "UIP2-A"), 111 tes lulus (items, approvals, share, item-*, blueprints, ui-audit termasuk axe), tsc dan eslint bersih. Tidak dijalankan: tes live Go `PMS_TEST_BROWSER=1` dan tangkapan layar dari server nyata.
