# Retest audit visual PMS

## Batch A · UI-01

**PASS pada cakupan lokal.** Audit resmi tetap menggunakan baseline 64/100. Retest UI-01 membandingkan working tree sebelum dan sesudah perbaikan dengan fixture yang sama, Chromium melalui Playwright, locale Inggris dan waktu tetap. Baseline audit tidak diperbarui.

Di bawah 1024 px, navigasi berpindah ke drawer dari Topbar. Drawer menggunakan Dialog, Button dan konten Sidebar yang sama. Desktop mempertahankan preferensi collapse, sidebar sticky dan navigasi yang ada. Header Space dapat membungkus tombol saat ruang sempit. Lebar sidebar 248 px dibagi melalui token CSS `--sidebar-width`. Tidak ada perubahan kontrak API.

| Halaman | 1440×900 | 1280×800 | 768×1024 | 390×844 |
|---|---|---|---|---|
| Home | 1440 → 1440 | 1280 → 1280 | 768 → 768 | 489 → 390 |
| Space Table | 1440 → 1440 | 1280 → 1280 | 768 → 768 | 583 → 390 |
| Space Board | 1440 → 1440 | 1280 → 1280 | 768 → 768 | 583 → 390 |
| Task berjudul panjang | 1440 → 1440 | 1280 → 1280 | 768 → 768 | 557 → 390 |

Angka menunjukkan lebar dokumen dalam piksel sebelum → sesudah. Scroll Table dan Board tetap berada di region data. Perbandingan pixel Playwright terhadap bukti before lokal menghasilkan tujuh dari delapan pasangan desktop identik dan satu pasangan Home 1440 berbeda satu piksel. Pada tablet dan mobile, perubahan posisi dan lebar konten sesuai layout drawer.

Bukti [sebelum](ui-audit-retest/before/) dan [sesudah](ui-audit-retest/after/) untuk empat halaman pada empat ukuran. Contoh [Task 390 sebelum](ui-audit-retest/before/task-390.png) dan [sesudah](ui-audit-retest/after/task-390.png). Drawer [390](ui-audit-retest/after/mobile-drawer-390.png) dan [768](ui-audit-retest/after/mobile-drawer-768.png).

| Pemeriksaan | Hasil |
|---|---|
| `npm run typecheck` | PASS |
| Playwright Navigation, Spaces, Screens axe, Search, Board lanes, Views | PASS 50/50 pada Chrome. Mencakup kemampuan Space, preferensi collapse, ekspansi tree, aksi More desktop, dan aksi Table/Board. |
| Keyboard dan fokus drawer | PASS pada 390 dan 768. Tab mencapai pemicu, Enter membuka drawer, Escape mengembalikan fokus, dan pemilihan Space menutup drawer serta memulihkan fokus. Search dari drawer memindahkan fokus ke input. Resize ke desktop menutup drawer. |
| Axe | PASS untuk drawer pada 390 dan 768, serta sampel Home, Table dan Board pada 390 dan 1440. Suite layar turut mencakup tema terang dan gelap. |
| Komparasi visual terhadap golden snapshot terdaftar | NOT TESTED. Repo tidak memiliki golden untuk skenario ini. Bukti before/after dibandingkan secara lokal. |
| Safari, Firefox, perangkat sentuh nyata dan screen reader | NOT TESTED. |

Percobaan tes awal gagal saat Tab dikirim sebelum shell siap dan saat axe serta geometri diperiksa di tengah animasi dialog. Tes akhir menunggu pemicu serta posisi drawer stabil, dan drawer memakai konfigurasi animasi tersendiri.

## Batch B · Settings dan Gantt

- UI-02. Enam halaman Settings pada 390 px sebelumnya memiliki lebar dokumen 413–579 px setelah Batch A. Semuanya kini 390 px. Navigasi berada di atas konten pada layar sempit. Dua kolom desktop tetap tersedia. [Sebelum](ui-audit-retest/batch-b/before/) · [Sesudah](ui-audit-retest/batch-b/after/).
- UI-03. Gantt mengoreksi role dan level hierarki pada DOM SVAR, menghapus indeks virtual yang salah, dan membatasi atribut ekspansi pada induk. Pemeriksaan axe terang dan gelap lulus dengan dua aturan ARIA diaktifkan kembali. Drag, resize dan pintasan tanggal lulus. Screen reader **NOT TESTED**. Adapter bergantung pada markup internal SVAR dan perlu diperiksa saat upgrade.
- Uji Settings menemukan route Users non-admin kosong ketika melempar not-found saat render. Tampilan penolakan sekarang dirender langsung dan guard tetap berada sebelum Outlet.

## Batch C · Calendar, Forms dan 404

- UI-04. Calendar pada 390 px turun dari lebar dokumen 639 ke 390 px setelah scroll region menjadi containing block untuk label screen reader. Tujuh kolom tetap tersedia dan ArrowRight menggulir grid saat fokus. Viewport 768, 1280 dan 1440 juga pas. Tes drag, keyboard tanggal dan axe lulus.
- UI-05. Label Open form menggantikan URL mentah. Tautan dan salin menggunakan URL lengkap, termasuk link publik. Lebar dokumen mobile 488 → 390 px. Salin dengan Enter dan axe terang/gelap lulus.
- UI-06. 404 memberi judul, penjelasan dan aksi kembali. Tab/Enter dan axe pada sesi fixture diuji. [Sebelum](ui-audit-retest/batch-c/before/) · [Sesudah](ui-audit-retest/batch-c/after/).


## Batch D · Space kosong dan skala judul

- UI-07. Space baru menampilkan satu penjelasan kosong. Aksi New item tetap tersedia bagi pengguna dengan izin create. View dapat dibuka dan diganti pada Space kosong. Filter yang menghilangkan seluruh hasil di Table, Board dan Calendar menampilkan Clear filters untuk menghapus filter aktif sambil mempertahankan view dan sort.
- UI-08. Judul halaman workspace memakai token 20 px compact dan 22 px comfortable. Home mempertahankan judul sambutan 30 px sebagai pengecualian hierarki.
- [Sebelum](ui-audit-retest/batch-d/before/) · [Sesudah](ui-audit-retest/batch-d/after/).

## Cakupan retest B–D

Screenshot fixture deterministik pada 1440×900, 1280×800, 768×1024 dan 390×844. Bukti ditulis ke folder batch terpisah. Baseline audit 64/100 dan golden snapshot tidak diperbarui. Pengujian perangkat nyata, Safari/Firefox dan screen reader **NOT TESTED**. Skor keseluruhan belum dihitung ulang.

| Defect | Hasil lokal | Pemeriksaan utama |
|---|---|---|
| UI-02 | PASS | Lebar enam halaman Settings, profil, jadwal, workflow dan guard Users |
| UI-03 | PASS otomatis | Axe tanpa pengecualian ARIA, keyboard tanggal, drag dan resize. Screen reader NOT TESTED. |
| UI-04 | PASS | Tujuh kolom, scroll lokal dan keyboard 390 px |
| UI-05 | PASS | Tautan ringkas, clipboard dengan Enter dan axe. Tes hapus form lulus pada retry. |
| UI-06 | PASS | Tab/Enter, axe, tujuan untuk pengguna aktif dan anonim |
| UI-07 | PASS | Empty Space dan filter tanpa hasil, Clear filters serta buka ulang view |
| UI-08 | PASS | Computed size 20/22 px dan capture dengan density compact |

Typecheck dan `git diff --check` lulus. Suite gabungan 91 kasus mencatat 89 lulus langsung, satu tes hapus form lulus pada retry, dan satu tes drawer tablet gagal sebelum fixture dan penantian fokus diperbaiki. Retest Navigation serial pada fixture dan assertion akhir lulus 16/16. Semua 18 tes axe layar lulus pada run serial terpisah. Ada 136 PNG sebelum/sesudah untuk B–D.
