# Gap layar pass 2, Lane D (dashboard, report, form, blueprint, workflow, integrasi, user)

Gambar rujukan dibuka satu per satu dari `docs/screens` (Dashboard 1, Adding a custom widget 3, Editing a widget 2, Creating custom analytics 2, Creating a report 3, Creating a request form 5, Creating a workflow 4; flow lain dinilai dari nama folder, urutan gambar, dan kode). Lingkup D-106: pola dan gaya, bukan merek. Tanpa API/migrasi baru.

| # | Flow | Gambar rujukan | UI aktual | Gap | Prio | Status |
|---|---|---|---|---|---|---|
| D1 | Creating a workflow / Creating a status / Editing workflow | Creating a workflow 4 | Daftar baris formulir: tiap status berisi 8+ kontrol terbuka | Wrike: empat kolom grup (Active, Completed, Deferred, Cancelled), status berupa chip berwarna, "Add Status" per kolom, editor muncul saat chip diklik dengan palet warna | P1 | Selesai (D-180) |
| D2 | Creating a status | Creating a workflow 4 | Pemilih warna `<select>` berisi nama token | Palet swatch warna | P2 | Selesai (D-180) |
| D3 | Creating a report | Creating a report 3 | Formulir sebaris di atas daftar (tiga `<select>`) | Dialog "Create report" | P2 | Selesai (D-181) |
| D4 | Dashboard | Dashboard 1 | Pemindah dashboard, popover layout, Share, editor dua panel sudah ada (S-UIP.2.4/2.6) | Header kartu widget tanpa pemisah | P3 | Selesai (D-181) |
| D5 | Dashboard / Adding a custom widget | Dashboard 1, Adding a custom widget 3 | Daftar tugas widget sudah dikelompokkan dengan chip status (pass 1) | Baris chip jenis chart di dasar editor, ikon di galeri widget, "By {pemilik}" | P3 | Ditunda: butuh ikon per jenis dan data pemilik di payload; nilai kosmetik |
| D6 | Creating custom analytics | Creating custom analytics 2 | Papan analitik Space memakai editor widget yang sama | Panel Categories/Values dan Pie Type bergambar | P3 | Ditunda: editor query berbeda, perlu desain ulang |
| D7 | Creating a request form / Customizing a form | Creating a request form 5 | Menu "Add question" berupa daftar datar tanpa ikon | Menu bergrup Questions / Dividers dengan ikon per jenis | P2 | Selesai (D-182) |
| D8 | Manage forms, Sending a request form, Blueprints, Apps & Integrations, User types, Inviting, Deleting user/account | folder terkait (tidak dibuka satu per satu) | Halaman admin sudah berbentuk tab dan tabel | Tidak ada gap struktural yang terukur dari kode | - | Ditunda: diperlukan tinjauan visual lanjutan |

Ringkasan: 5 Selesai (D1-D4, D7), 3 Ditunda (D5, D6, D8). "New page" di menu Wrike tetap tombol "Add page" terpisah. D8 belum ditinjau dengan gambar.
