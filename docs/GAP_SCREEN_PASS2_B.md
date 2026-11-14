# Gap Tampilan Pass 2, Lane B (view, Space/project, pembuatan)

| | |
|---|---|
| **Tanggal** | 15 November 2026 |
| **Cakupan** | Toolbar/tab view, Table, Board, Gantt, Calendar, Timelog, header Space dan project, dialog pembuatan project/folder/Space, snapshot publik, ekspor Excel |
| **Cara** | Gambar referensi dibuka satu per satu (lihat kolom "Gambar rujukan", dalam `docs/screens/<flow>/`) dan dibandingkan dengan UI aktual yang berjalan pada data seed ITA (Space "ITA Internal Timeline", 158 item, project > folder > task, Postgres `pms-pgB` port 55522, vite port 5174). Pass 1 (S-UIP.1-4) sudah menyamakan toolbar chip, tab bersegmen, header project, dan kartu Board dasar; gap di bawah adalah sisanya. |
| **Kerangka** | D-106: samakan pola dan gaya, bukan merek, teks promo, atau fitur komersial. Teks tetap English dan i18n. |

Prioritas: P1 merusak alur harian, P2 terasa kurang matang, P3 kosmetik.

| # | Flow | Gambar rujukan | UI aktual | Gap | Prio | Status |
|---|---|---|---|---|---|---|
| B2-1 | Board view, Adding a new item (board) | Board view 0/1, Adding a new item (board) 2 | Kolom berbingkai abu-abu dengan header kecil; kartu: judul tombol, baris teks tipe, tanggal, jumlah subitem, avatar | Kartu tanpa ikon tipe di judul; avatar dan tanggal beda baris; subitem tidak dipisah footer; header kolom tidak selebar kolom | P2 | Selesai (D-160) |
| B2-2 | Board view | Board view 0 | Dropdown status ada di dalam kartu | Wrike tidak punya kontrol status di kartu (status = kolom) | P3 | Ditunda: dropdown itu jalur pindah status lewat keyboard/pembaca layar (a11y, tes `uip-lane2`); dipertahankan, kini ditaruh di footer kartu |
| B2-3 | Sorting by group, Board swimlane | Sorting by group 2 | Setiap lane mengulang seluruh header status | Wrike: satu baris header status di atas, lane bisa dilipat dengan avatar | P3 | Ditunda: perlu restruktur layout lane (grid bersama), risiko regresi `board-lanes` |
| B2-4 | Editing card preview | Editing card preview 1 | Popover "Card fields" sudah ada (pass 1) dengan toggle | Rujukan juga memuat kolom pencarian field dan grup General/Schedule | P3 | Ditunda |
| B2-5 | Projects and folders, Gantt chart view | Projects and folders 2, Gantt 1 | Table: judul tanpa ikon tipe; tombol "Add item" ber-outline terpisah | Ikon tipe (task/folder/project) pada baris dan baris "+ Item" menyatu | P2 | Selesai (D-160) |
| B2-6 | Table | Gantt chart view 1 (panel kiri) | Status berupa `<select>` bergaya chip (tint) | Wrike memakai kotak warna + nama; select native tetap dibutuhkan (keyboard `s`, tes) | P3 | Ditunda: tint chip sudah membawa warna status |
| B2-7 | Gantt chart view, Adding a new item (gantt) | Gantt 1, Adding a new item (gantt) 3 | Kolom Status teks polos; tidak ada baris tambah; baris toolbar hanya tombol skala; tombol Columns lepas | Kolom status berwarna, "+ Item", toolbar chip (Filter, Columns, cari) seragam dengan view lain | P2 | Selesai (D-161) |
| B2-8 | Gantt chart view | Gantt 0/1 | Kolom tanggal 100 px membungkus tanggal ISO ke dua baris | Tanggal tidak boleh terpotong | P2 | Selesai (lebar 112) |
| B2-9 | Gantt chart view | Gantt 0 | Bar sudah berwarna menurut status, label di dalam bar | Rujukan: label di luar ujung bar, garis hari ini merah, baris ringkasan project | P3 | Ditunda: SVAR gratis tidak menyediakan label luar bar; garis hari ini sudah memakai `highlightTime` |
| B2-10 | Gantt chart view | Gantt 0 | Skala: tiga tombol Days/Weeks/Months + −/+ di atas chart | Wrike: dropdown "Weeks" + −/+ melayang di atas timeline | P3 | Ditunda: tes `gantt` mengikat tombol `aria-pressed` |
| B2-11 | Default view tab (Table, Board, Gantt) | Projects and folders 0 | Project baru hanya punya Table dan Board; Gantt/Calendar ditambah lewat "All > Add view" | Wrike memberi Gantt sejak awal | P3 | Ditunda: butuh perubahan seed view di server (migrasi/API) |
| B2-12 | Setting default view, Renaming a view, Duplicating a view | Duplicating a view 2, Renaming 0-3, Setting a default 0-2 | Menu "..." per tab aktif, dialog Rename/Duplicate sudah ada (pass 1) | Rujukan: daftar "Always visible/Hidden" di "All", dialog Duplicate berisi radio Public/Personal berpenjelasan | P3 | Ditunda |
| B2-13 | Adding a calendar view | Adding a calendar view 3 | Chip item berwarna abu `bg-secondary` | Bar berwarna menurut status dengan aksen kiri dan ikon tipe | P2 | Selesai (D-162) |
| B2-14 | Adding a calendar view | Adding a calendar view 3 | Bar tidak menyambung lintas hari | Wrike: bar multi-hari menyambung | P3 | Ditunda: perlu layout berbasis minggu (bukan sel), risiko untuk drag/keyboard |
| B2-15 | Timelog view | Timelog view 1 | Lima field berlabel terbuka di atas tabel | Satu baris toolbar (Filters) lalu tabel; total di bawah | P2 | Selesai (D-163) |
| B2-16 | Timelog view | Timelog view 1 | Tabel datar | Pengelompokan User > Date dengan SUM, Expand/Collapse, Export, Lock | P3 | Ditunda: butuh agregasi/ekspor di API |
| B2-17 | Creating a project, Creating a folder | Creating a project 3, Creating a folder 3 | Pilihan "Default view" berupa dua radio kecil | Kartu pilihan bergambar dengan penjelasan | P2 | Selesai untuk Table dan Board (D-164) |
| B2-18 | Creating a project | Creating a project 3 | Dialog satu panel | Dua panel (nama, lokasi, workflow | pengaturan, tampilan, sharing); kartu Gantt, Calendar, Files, Chart | P3 | Ditunda: `default_view_type` di API hanya table/board |
| B2-19 | Creating a space | Creating a space 3 | `space-dialog` formulir sederhana | Galeri template video | P3 | Tidak dikerjakan: fitur komersial/promo (D-106) |
| B2-20 | Filtering space presets, Filtering custom fields | Filtering space presets 2 | Chip bernama tetap "Filter"/"Sort"/"Group" | Label chip memuat nilai aktif ("All active tasks", "Due date") dan berwarna biru saat aktif | P3 | Ditunda: nama chip dipakai tes dan dibaca pembaca layar; chip aktif sudah berwarna `secondary` |
| B2-21 | Creating a public snapshot | Creating a public snapshot 1 | Dialog snapshot dari tombol di toolbar Gantt | Panel samping "Create a snapshot" dengan checkbox dan tautan | P3 | Ditunda: dialog sudah menyediakan fungsi yang sama |
| B2-22 | Exporting project to excel | Exporting project to excel 0-3 | Menu export di project | Fungsi setara | P3 | Tidak ada gap fungsional |
| B2-23 | Space overview, header Space | Space overview 0, Board view 0 | Header Space: judul, deskripsi, tombol | Baris meta "Public · N members · Add bookmark" | P3 | Ditunda: butuh data anggota (query tambahan); "Bookmarks" sudah ada |
| B2-24 | Tab view | Gantt 0, Timelog 1 | Tab teks saja | Ikon per tipe view | P3 | Selesai (D-162) |

Ringkasan: 24 gap, **Selesai 8** (B2-1, 5, 7, 8, 13, 15, 17, 24), **Ditunda 14**, **tidak dikerjakan 1** (B2-19, promo) dan **tanpa gap fungsional 1** (B2-22), 0 gap P1. Semua yang ditunda butuh API/migrasi, restruktur besar, atau bertabrakan dengan kontrak a11y/tes.
