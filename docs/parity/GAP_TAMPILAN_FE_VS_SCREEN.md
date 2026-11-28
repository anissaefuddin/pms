# Gap Tampilan FE Aktual vs Layout Wrike di `docs/screens/`

| | |
|---|---|
| **Tanggal** | 14 November 2026 |
| **Input** | [`screens/`](../screens/) (99 folder, 540 PNG), [ANALISA_FITUR_SCREEN.md](ANALISA_FITUR_SCREEN.md), kode `web/src` (commit `eed323a`), screenshot UI aktual di [ui-audit-retest/after](../quality/ui-audit/ui-audit-retest/after/) (13 Nov, rute Home, Table, Board, Task) dan [ui-audit-evidence](../quality/ui-audit/ui-audit-evidence/) (rute lain) |
| **Kerangka** | [DESIGN_SYSTEM.md](../product/DESIGN_SYSTEM.md) baris 10: screenshot Wrike dipakai untuk memahami **fungsi**, bukan meniru warna, ikon, atau teks. Karena itu sidebar gelap, hijau Wrike, logo, kartu promo, dan trial counter **bukan gap**. Yang dinilai: struktur halaman, pola interaksi, dan kepadatan informasi |

## Batas analisis (baca dulu)

1. **Sembilan pasang gambar dibuka langsung** (Space overview, My to-do, Gantt, panel detail, Inbox, Dashboard, Settings dan dua gambar tambahan), dibandingkan dengan screenshot PMS untuk rute yang sama. Sisa flow dinilai dari catatan di ANALISA_FITUR_SCREEN dan dari kode, bukan dari gambar.
2. **Nama folder tidak selalu sama dengan isi gambar.** Contoh: `Dashboard 0` dan `Editing an item 0` memperlihatkan Gantt dan menu More. Flow dicocokkan menurut isi.
3. **Screenshot PMS berumur 13 Nov** dan belum memuat M8 sampai M11 kecuali yang tercantum di [UI_AUDIT_2_REPORT](../quality/ui-audit/UI_AUDIT_2_REPORT.md). Temuan yang saya cek ulang di kode ditandai **(kode)**.
4. Tidak ada aplikasi yang dijalankan ulang untuk laporan ini.

## Ringkasan

PMS sudah memenuhi fungsi sebagian besar layar, tetapi **polanya lebih mirip formulir administrasi daripada ruang kerja**: kontrol berlabel dan `<select>` bawaan di mana Wrike memakai chip, popover, dan edit langsung. Tiga gap paling besar:

1. **Panel detail item** berupa formulir panjang dengan tombol Save per blok (Wrike: chip edit langsung dan komentar di bawah).
2. **Toolbar view** memakai enam kontrol berlabel yang selalu terbuka (Wrike: satu baris chip dengan popover).
3. **Sidebar** tanpa struktur per Space (Space overview, Tools, Projects and folders), dan halaman project memuat formulir detail di atas view.

## Gap per area

Prioritas: **P1** merusak alur kerja harian, **P2** terasa kurang matang, **P3** kosmetik. Usaha: S (hari), M (beberapa hari), L (minggu).

### A. Shell dan navigasi

| # | Layar Wrike | PMS aktual | Gap | Prio | Usaha |
|---|---|---|---|---|---|
| A1 | Sidebar per Space: Space overview (+ roda gigi), Tools (Custom analytics, Dashboard, tanda +), Projects and folders (+, ikon project dan folder), lalu Shared with me, Blueprints, Hidden spaces; header Spaces punya +, cari, ... | Daftar Space datar; membuka Space hanya menampilkan project/folder tanpa ikon tipe; tidak ada Space overview, Tools, tombol + per bagian; Shared with me ada di menu More, Blueprints tidak ada di sidebar | Struktur navigasi Space tidak ada. Papan analitik Space (S11.2.2) tidak punya jalan masuk dari sidebar | P1 | M |
| A2 | Menu More: Dashboards, My to-do, Request forms, Timesheets, Reports, Calendars, Workload, Created by me, Stream, Starred, **Customize menu** | More memuat Portfolio, Workload, Timesheet, Starred, Dashboards, Reports, My to-do, Files to review, Shared, Recycle bin **(kode)**. Tidak ada Calendars, Created by me, Customize menu | Tiga entri hilang; kunci `sidebar_hidden` ada di server tetapi tidak ada layar untuk mengaturnya di menu | P2 | S |
| A3 | Bar atas hanya konteks halaman (Automation, Share, tombol + hijau) | Teks status **"Connecting…"** tampil di semua screenshot, di kiri bar atas **(kode: `topbar.tsx:26`)**; Search, lonceng, +, akun di kanan | Status realtime tampil seperti galat tetap | P2 | S |
| A4 | Menu akun (klik avatar): Settings, Workspace appearance, Recycle bin, Apps & Integrations, Help, Log out | Ikon akun di bar atas, serta roda gigi, lonceng, Help di dasar sidebar: dua tempat untuk hal serupa | Duplikasi pintu masuk | P3 | S |
| A5 | Pencarian dengan petunjuk pintasan `⌘K` | Tombol Search tanpa petunjuk pintasan | Petunjuk kecil | P3 | S |
| A6 | Settings: menu avatar dengan Workspace appearance | Menu Settings memuat tautan **"Gantt spike"** **(kode: `settings.tsx:38`)** dan pesan 2FA menyebut variabel lingkungan `PMS_2FA_KEY` kepada semua pengguna | Artefak pengembangan dan detail ops bocor ke UI | P1 | S |

### B. Header halaman dan toolbar view

| # | Layar Wrike | PMS aktual | Gap | Prio | Usaha |
|---|---|---|---|---|---|
| B1 | Header project/Space: breadcrumb, judul + chip **Info** + pin, baris meta (Public, N members, Add bookmark, pengaturan), tombol Automation, Share | Judul + deskripsi di kiri; New item, Forms, Share, ... di kanan. Halaman project (`/tasks/{id}`) menampilkan **seluruh formulir detail item di atas, view di bawah** **(kode: `tasks.$itemId.tsx`)** | Project tidak punya halaman bergaya "ruang kerja"; detail memakan layar sebelum view | P1 | M |
| B2 | View: kontrol bersegmen Table / Board / Gantt chart + dropdown "All" | Tab teks, **setiap tab punya tombol "..." sendiri** (empat tombol tambahan di urutan tab), "Add view" di samping | Terlalu bising; menambah tab stop | P2 | S |
| B3 | Satu baris toolbar chip: All tasks (filter), Priority/Due date (sort), Columns, Group, Cards, Expand, Public snapshots, undo/redo, ... | Panel enam kontrol berlabel setinggi sekitar 110 px: Filter items, Preset, Sort, Filter by field, Columns, Group (semuanya `<select>` bawaan) | Memakan ruang vertikal dan terasa formulir; tidak ada popover | P1 | M |
| B4 | Table: status berupa chip berwarna, assignee avatar, kolom jelas, baris "+ Item" menyatu | Status `<select>` bawaan di tiap baris, assignee berupa teks, tombol "Add item" terpisah di bawah | Kurang padat dan kurang terbaca | P2 | M |
| B5 | Board: header kolom berwarna per status, kartu dengan ikon tipe, avatar, rentang tanggal + durasi "(3d)", ikon jumlah subitem | Header kolom polos; kartu berisi teks "12 subitems 0 attachments 0 comments" dan **`<select>` status di dalam kartu**; tidak ada avatar atau ikon tipe | Kartu tidak dapat dipindai cepat | P1 | M |
| B6 | Gantt: tabel kiri berkolom (Status, Start, Due, Impact) + timeline, bar berwarna menurut status, label di ujung bar ("Research • Jane S."), garis hari ini, skala Weeks dengan tombol +/- | Satu kolom Name; semua bar biru tua dengan label putih di dalam bar; skala Days/Weeks/Months; teks bantuan keyboard permanen di atas chart | Kolom status dan tanggal tidak ada; bar tidak berkode warna | P2 | M |

### C. Panel detail item

| # | Layar Wrike | PMS aktual | Gap | Prio | Usaha |
|---|---|---|---|---|---|
| C1 | Panel samping ringkas: judul bertipe, chip lokasi, **status, assignee, tanggal sebagai chip sebaris yang bisa diedit langsung**, baris penghitung (subitem, file, approval), deskripsi, file, aktivitas, **komposer komentar menempel di dasar** (lampiran, @, emoji, format, Send), tautan salin, "Go to full details" | Formulir vertikal panjang: Name (+ **Save**), Status, Importance, Task progress (+ **Save progress**), Start/Finish (+ **Save**), Assignees (+ **Save**), Effort (+ **Save effort**) sebelum sampai ke komentar dan aktivitas | Tombol Save per blok, banyak gulir, dan konteks sunting terlalu berat. Ini gap paling terasa bagi pengguna harian | P1 | L |
| C2 | Judul muncul sekali | Judul tampil dua kali: sebagai heading dan sebagai kolom Name | Duplikasi | P3 | S |
| C3 | Lokasi item sebagai chip (Creative Team, Personal, +) | Lokasi berada di blok tersendiri di bawah | Kurang terlihat | P3 | S |

### D. Daftar pribadi

| # | Layar Wrike | PMS aktual | Gap | Prio | Usaha |
|---|---|---|---|---|---|
| D1 | **My to-do**: dikelompokkan per status (Planned (1), In review (1), ...) atau per waktu, label workflow di kanan; **baris tambah cepat** di atas dengan pemilih tipe; menu "..." (kartu satu atau dua baris, daftar datar atau hierarki, tampilkan durasi, cetak, ekspor); avatar dan chip project | Daftar datar; dua `<select>` (Filter tasks, Sort tasks); status `<select>` per baris; tidak ada pengelompokan, tambah cepat, atau opsi tampilan **(kode: `my-todo.tsx`)** | Fungsi daftar kerja pribadi paling sering dipakai tetapi paling sederhana | P1 | M |
| D2 | **Inbox**: tab Incoming / Sent / Archive, master-detail (daftar kiri, isi kanan), dikelompokkan Today, waktu relatif, titik belum dibaca, ilustrasi kosong, kartu bantuan yang bisa ditutup | Satu kolom: kotak centang "Unread only", Mark all as read, Archive all, pesan "You're all caught up" **(kode: `inbox.tsx`, 110 baris)** | Tanpa tab, tanpa panel detail, tanpa pengelompokan tanggal. Tab Sent butuh dukungan API (belum ada) | P1 | M |

### E. Dashboard dan analitik

| # | Layar Wrike | PMS aktual | Gap | Prio | Usaha |
|---|---|---|---|---|---|
| E1 | Toolbar dashboard: judul dengan **dropdown pindah dashboard** + pin, **+ Widget**, "By {pemilik}", **Change layout** (popover bergambar Vertical/Horizontal), Share, Public link, ... | Tautan "All dashboards", judul, keterangan "Personal dashboard", tombol Add widget, Public link, dan `<select>` Layout (ditambahkan di S11.2.3) | Tidak ada pemindah dashboard, popover layout bergambar, atau tindakan Share | P2 | M |
| E2 | Widget daftar tugas dikelompokkan (This week, Next week) dengan avatar dan chip status/project | Widget daftar berupa baris judul + tanggal | Kurang informatif | P2 | M |
| E3 | Editor widget: modal dua panel (pratinjau kiri, tab Data/Design/Filters kanan) dan galeri template | Satu dialog bertumpuk berisi QueryEditor dan pratinjau di bawah; modal "Choose widget" sudah ada (S11.2.2) | Pratinjau jauh dari kontrol yang diubah | P2 | M |
| E4 | Papan analitik dari Tools Space, filter sumber data di panel kanan | Dibuat dari tombol di halaman Dashboards (S11.2.2); tidak ada panel filter sumber data | Jalan masuk salah tempat (lihat A1) | P2 | S setelah A1 |

## Rekomendasi perbaikan

**Prinsip:** pakai ulang data dan API yang sudah ada; yang diubah adalah komponen tampilan. Tidak ada gap di atas yang butuh migrasi, kecuali tab Sent di D2.

### Batch 1: alur harian (P1, sekitar 3 minggu)

1. **C1 Panel detail**: ubah blok status, assignee, tanggal, importance menjadi chip yang menyimpan otomatis saat nilai dipilih (PATCH dengan `If-Match`, toast saat gagal, tombol Save hanya untuk Name dan deskripsi). Pindahkan effort, progress, custom field ke bagian "More fields" yang dilipat. Tempel komposer komentar di dasar panel. Tambahkan "Go to full details" dan salin tautan. Pertahankan urutan `BLOCK_ORDER` agar tata letak per tipe (CUST-04) tetap berlaku.
2. **B3 Toolbar view**: satu baris chip (Filter, Sort, Group, Columns, Cards) dengan `Popover` Base UI yang sudah ada di `components/ui`; keadaan tetap di parameter URL seperti sekarang. Pencarian teks menjadi kotak kecil di ujung baris.
3. **B5 Kartu Board** dan **B4 Table**: komponen `StatusChip` (warna dari workflow, WF-03A sudah menyimpan warna) dan `AvatarStack` (inisial); hapus `<select>` dari kartu, ubah status lewat seret atau menu kartu.
4. **D1 My to-do**: pengelompokan sesuai urutan yang dipilih (status atau tanggal) dengan jumlah di judul grup, baris tambah cepat memakai `POST /items` ke Space pribadi, dan menu opsi tampilan (cetak dan ekspor boleh menyusul).
5. **D2 Inbox**: tab Incoming dan Archive dengan master-detail di lebar ≥ 1024 px dan pengelompokan tanggal; tab Sent ditunda sampai ada endpoint.
6. **A6** hapus tautan "Gantt spike" dari navigasi produksi dan ganti pesan 2FA dengan teks netral (detail variabel hanya untuk admin).
7. **A1 dan B1**: sidebar per Space (Space overview, Tools, Projects and folders dengan +, ikon tipe) dan halaman project berbentuk header + view, dengan detail di panel "Info".

### Batch 2: kematangan (P2, sekitar 2 minggu)

A2 (Calendars, Created by me, Customize menu), A3 (tampilkan status realtime hanya saat terputus), B2 (tab bersegmen, "..." hanya di tab aktif), B6 (kolom Status dan tanggal di Gantt, warna bar menurut status, label assignee), E1 sampai E4.

### Batch 3: kosmetik (P3)

A4, A5, C2, C3.

### Cara memverifikasi

- Tambahkan **baseline visual Playwright** per rute pada 1440 dan 390 px (infrastruktur screenshot sudah ada di `ui-audit-*`), lalu bandingkan dengan tabel di atas setiap selesai batch.
- Jalankan ulang axe: pola chip dan popover baru adalah area risiko aksesibilitas (urutan fokus, nama aksesibel).
- Perbarui [UAT.md](../quality/UAT.md) dan [UI_AUDIT_REPORT.md](../quality/ui-audit/UI_AUDIT_REPORT.md) setelah Batch 1, dan story-nya ada di [TASKS.md](../roadmap/history/TASKS.md) bagian "M12b — Kesetaraan tampilan FE".

## Yang sengaja tidak dianggap gap

Tema gelap dan warna sidebar, logo dan merek, kartu promo "Discover Wrike" dan penghitung trial, quick-start checklist berhadiah, obrolan bantuan, kartu iklan aplikasi seluler dan integrasi di Inbox, serta ikon dan ilustrasi. Semuanya komersial atau identitas Wrike, dan dikecualikan oleh DESIGN_SYSTEM.
