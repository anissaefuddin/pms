# DESIGN SYSTEM — PMS

| | |
|---|---|
| **Versi** | 0.1 (draft) |
| **Cakupan** | Komponen dan pola untuk M0 + M1. Komponen M2+ (Gantt, chart, form builder) hanya dicatat di §6 |
| **Fondasi** | `apps/ui-kit` (Tailwind v4 + `@base-ui/react` + `class-variance-authority`), token di `ui-kit/design-tokens.json` |
| **Terkait** | [UI_SPEC.md](UI_SPEC.md), [ARCHITECTURE.md](ARCHITECTURE.md) §12 |

> Tampilan PMS dibuat dari `ui-kit` milik sendiri. Screenshot Wrike hanya dipakai untuk memahami **fungsi** layar, tidak untuk meniru tampilan, ikon, warna, atau teksnya.

---

## 1. Prinsip

1. **Padat dan tenang.** Aplikasi kerja berbasis data: permukaan netral (abu-abu), warna dipakai untuk makna (status, prioritas, orang), bukan dekorasi.
2. **Keyboard-first.** Semua aksi utama punya jalan pintas dan bisa dijangkau tanpa mouse (§5.5).
3. **Satu sumber token.** Komponen tidak boleh memakai hex atau nilai primitif langsung; hanya token semantik.
4. **Per klien bisa bertema.** Setiap instance klien dapat mengganti blok `brand` (warna utama, logo) lewat `data-brand`, tanpa mengubah kode komponen.
5. **Aksesibel secara default.** WCAG AA untuk kontras di mode terang dan gelap; fokus selalu terlihat; tidak ada informasi yang hanya disampaikan lewat warna.
6. **Semua teks lewat `t()`.** Tidak ada string UI di dalam komponen (§5.6).

## 2. Fondasi dari `ui-kit` (dipakai apa adanya)

| Area | Nilai di `ui-kit` | Catatan untuk PMS |
|---|---|---|
| Font | `Inter` (sans), `JetBrains Mono` (mono) | Diunduh sebagai file font lokal (tanpa CDN) agar instance bisa jalan tanpa internet keluar |
| Warna primitif | gray 50–900, blue, green, amber, red | Dasar semua token semantik |
| Warna aksen (kategori) | 6 hue: violet, teal, rose, orange, cyan, fuchsia (`--cat-1…6`), dipilih lewat hash nama (`categoricalClasses`) | Dipakai untuk avatar user dan label kategori. **Bukan untuk status** (aturan `ui-kit`, lihat §4.2) |
| Token semantik | `background`, `surface`, `border`, `text`, `brand` (satu-satunya blok yang diganti per klien), `status.{success,warning,danger,info}` + `-subtle` | |
| Spacing | skala 4 px (4, 8, 12, 16, 24, …) | |
| Radius | `control` (md), `card` (lg), `modal` (xl) | |
| Density | `compact` (font 13 px) dan `comfortable` (font 14 px) lewat `data-density` | **Default PMS: `compact`**, karena tabel dan Board padat. User bisa memilih di Settings |
| Tema | `data-brand` pada `<html>` | Default `default`; satu override per klien |
| Mode gelap | Variabel CSS di-redefinisi pada `.dark` | Wajib lulus kontras AA (§5.7) |

Token baru yang dibutuhkan PMS (ditambahkan ke `design-tokens.json`, bukan di PMS saja, agar proyek lain ikut manfaat):

| Token | Fungsi |
|---|---|
| `semantic.color.status-group.{active,completed,deferred,cancelled}` | Warna latar/teks lencana grup status (§4.1) |
| `semantic.color.priority.{low,medium,high}` (M2) | Disiapkan untuk custom field berwarna |
| `semantic.size.board-column-width` = 280 px | Lebar kolom Board |
| `semantic.size.detail-panel-width` = 560 px | Lebar panel detail item |
| `semantic.size.sidebar-width` = 248 px (ciut: 56 px) | |
| `semantic.z.{dropdown,sticky,panel,modal,toast}` | Skala z-index tunggal: 10 / 20 / 30 / 40 / 50 |
| `semantic.motion.{fast,base}` = 120 ms / 200 ms, easing `cubic-bezier(.2,0,0,1)`; nol pada `prefers-reduced-motion` | |

## 3. Tipografi

| Peran | Ukuran (compact / comfortable) | Bobot |
|---|---|---|
| Judul halaman | 20 / 22 px | 600 |
| Judul bagian / panel | 15 / 16 px | 600 |
| Isi teks, sel tabel | 13 / 14 px | 400 |
| Label, metadata, header kolom | 12 / 12 px | 500 |
| Kode, ID item (`#1234`) | 12 / 12 px, mono | 400 |

Tinggi baris Table: 32 px (compact) / 40 px (comfortable). Angka dan tanggal memakai `font-variant-numeric: tabular-nums`.

## 4. Komponen

### 4.1 Inventaris

**Sumber:** `ui-kit` = sudah ada, dipakai ulang; `tambah` = ditambahkan ke `ui-kit` (generik, bukan khusus PMS); `PMS` = khusus aplikasi ini; `lib` = dibangun di atas library headless.

| Komponen | Sumber | Dipakai di (PRD ID / layar) | Catatan |
|---|---|---|---|
| Button, LinkButton | `ui-kit` | semua | `LinkButton` di `ui-kit` memakai `next/link`; **ganti dengan adapter TanStack Router** saat disalin |
| Input, Textarea, Label, Checkbox, RadioGroup, Switch | `ui-kit` | form, AUTH-*, SET-02 | |
| Select | `ui-kit` | filter, user type | Ingat aturan Base UI di §4.3 |
| DateInput | `ui-kit` | TASK-05A | Perlu dilengkapi kalender popover (di bawah) |
| Dialog, Sheet, Popover, DropdownMenu, Tooltip, Tabs | `ui-kit` | semua | `Sheet` = dasar panel detail item |
| Table | `ui-kit` | VIEW-01 | Hanya markup; perilaku dari TanStack Table |
| Avatar, Badge, StatusBadge, Skeleton, EmptyState, Breadcrumb, Pagination, ScrollArea, Separator, Sonner (toast), Stepper | `ui-kit` | semua | |
| Sidebar, AppSidebar, Topbar, FilterPanel, FilterChip | `ui-kit` | NAV-01, NAV-02 | `nav-config.ts` diganti data PMS |
| DatePicker + DateRangePicker | tambah | TASK-05A, SPC-09 | Kalender bulanan, ketik manual, pilihan "working days only" |
| Combobox (pilih satu/banyak, pencarian) | tambah | AssigneePicker, ShareDialog, filter | Dasarnya Base UI Combobox |
| CommandPalette | tambah | NAV-06 (Ctrl/Cmd+K) | Kelompok hasil + navigasi panah |
| **StatusPill / StatusPicker** | PMS | VIEW-01/02, TASK-04 | Titik warna + nama + grup; menu pilihan dari workflow item |
| **AssigneePicker / AvatarStack** | PMS | TASK-06 | Banyak assignee; tampil maksimal 3 avatar + "+N" |
| **ItemDetailPanel** | PMS (di atas `Sheet`) | SPC-07, TASK-03 | §4.4 |
| **DataTable** | lib (TanStack Table + Virtual) + `Table` | VIEW-01 | Baris bertingkat (expand/collapse), edit inline, kolom bisa disembunyikan |
| **BoardColumn / BoardCard** | lib (dnd-kit) | VIEW-02 | Kolom per status; kartu dapat diseret; keyboard drag |
| **ViewTabs** | PMS (di atas `Tabs`) | VIEW-08, SPC-17 | Tab view + menu per view (rename, duplicate, default, hapus, copy link) |
| **RichTextEditor** | lib (TipTap) | TASK-03, TASK-07 | Toolbar minimal: tebal, miring, daftar, tautan, kode, @mention |
| **CommentThread** | PMS | TASK-07 | |
| **FileDropzone + AttachmentList** | PMS | TASK-08A | Menampilkan batas ukuran dan penolakan tipe |
| **ShareDialog** | PMS | SPC-11 | Cari user/grup/email, pilih role, daftar yang sudah punya akses (termasuk "diwarisi") |
| **LocationPicker** | PMS | SPC-05, TASK-01 | Pohon Space → project/folder dengan pencarian |
| **ConfirmDialog** | PMS (di atas `Dialog`) | BIN-01, USR-03 | Varian destruktif (tombol merah, fokus awal di Batal) |
| **ConflictBanner** | PMS | SEC-03 | §5.2 |
| **LiveIndicator** | PMS | SSE | Titik kecil di topbar; abu-abu saat terputus |
| **PermissionGate** | PMS | semua | Menyembunyikan/menonaktifkan aksi berdasarkan `my_capabilities` dari API |

### 4.2 Warna status

Status item memakai **hanya lima warna semantik**; pewarnaan khusus per status baru dibuka di M3 (WF-02/03A).

| Status default | Grup | Token warna |
|---|---|---|
| New | active | `gray` |
| In Progress | active | `info` (biru) |
| Completed | completed | `success` (hijau) |
| On Hold | deferred | `warning` (kuning) |
| Cancelled | cancelled | `danger-subtle` + teks `text.secondary` + ikon silang (tidak merah penuh agar tidak terbaca sebagai error) |

Aturan: lencana status selalu **titik/ikon + teks**, tidak hanya warna (butir 5 prinsip).

> **Keputusan untuk M3:** `ui-kit` menetapkan hue aksen tidak boleh dipakai untuk status. Di M3 admin boleh membuat status dengan warna sendiri (WF-03A, palet ±12). Dua pilihan: (a) palet khusus status yang terpisah dari aksen, atau (b) mengubah aturan `ui-kit` untuk kasus status buatan user. Rekomendasi: (a), agar kategori dan status tetap terbaca berbeda.

### 4.3 Jebakan Base UI yang sudah ditemukan di `ui-kit` (wajib dibaca)

Dari `ui-kit/DECISIONS.md`:

1. `DropdownMenuLabel` harus berada di dalam `DropdownMenuGroup`, kalau tidak runtime error.
2. `SelectValue` menampilkan **nilai mentah**, bukan label item. Berikan fungsi anak `{(v) => labels[v]}` atau prop `items` pada `Select.Root`. Semua `Select` di PMS memakai pola ini.
3. Batasan Server Component → Client Component dalam catatan `ui-kit` adalah masalah Next.js; di Vite (SPA) tidak berlaku.

Aturan tambahan PMS: komponen dengan popup (menu, dialog, select) wajib diuji dengan klik nyata di browser (tes Playwright), bukan hanya type-check; pengalaman `ui-kit` menunjukkan bug muncul hanya saat popup benar-benar dibuka.

### 4.4 Anatomi ItemDetailPanel

```text
┌────────────────────────────────────────────────────────┐
│ #1234  Project › Folder › Task            ⤢  ⋯  ✕     │  header: breadcrumb lokasi, buka halaman penuh, menu, tutup
├────────────────────────────────────────────────────────┤
│ Judul item (edit inline)                               │
│ [StatusPicker]  [AssigneePicker]  [DateRangePicker]    │  properti utama (baris tombol)
├────────────────────────────────────────────────────────┤
│ Description  (RichTextEditor)                          │
│ Subitems (n)  [+ Add subitem]                          │
│ Files (n)     [FileDropzone]                           │
│ (M3) Approval                                          │  muncul hanya jika fitur tersedia
├────────────────────────────────────────────────────────┤
│ Tab: Comments | Activity                               │
│ CommentThread / daftar aktivitas                       │
│ [RichTextEditor komentar]            [Send]            │
└────────────────────────────────────────────────────────┘
```

## 5. Pola

### 5.1 Form dan validasi

- Label di atas input; wajib ditandai "(required)" dalam teks, bukan hanya tanda bintang.
- Validasi dua lapis: sisi klien (zod, bentuk) saat blur dan saat submit; sisi server (aturan bisnis) memetakan `errors[].field` ke field terkait. Pesan di bawah field, `aria-describedby` menunjuk ke pesan, `aria-invalid="true"`.
- Tombol submit tidak dinonaktifkan karena form belum valid; ditonaktifkan hanya saat request berjalan (dengan spinner), agar user selalu tahu apa yang salah.
- Setelah submit gagal, fokus pindah ke field pertama yang salah.
- Pesan error mengikuti nada netral dan memberi tindakan ("Password must be at least 8 characters").

### 5.2 Konflik edit (SEC-03)

Saat server menjawab 409 `version_conflict`:

1. Data terbaru diambil ulang; nilai yang sedang diketik user **tidak dibuang** — disimpan sebagai draf lokal.
2. `ConflictBanner` tampil di atas bagian yang berkonflik: "Someone else changed this item. Review the latest version." dengan dua tombol: **Reapply my changes** (menerapkan draf ke versi terbaru lalu mengirim ulang dengan `If-Match` baru) dan **Discard my changes**.
3. Untuk perubahan satu field kecil (status, drag Board) tidak ada banner: pembaruan optimistis dibatalkan dan toast "Item was updated by someone else — latest version loaded".

### 5.3 State kosong, memuat, error

| State | Pola |
|---|---|
| Memuat pertama kali | `Skeleton` seukuran konten (bukan spinner penuh halaman); daftar > 300 ms menampilkan skeleton |
| Memuat ulang di latar | Tanpa penghalang; `LiveIndicator` berdenyut singkat |
| Kosong | `EmptyState`: ikon, satu kalimat penjelas, satu tombol aksi utama bila user punya kemampuan `create`; tanpa tombol bila tidak |
| Tanpa hasil filter | "No items match these filters" + tombol "Clear filters" |
| Error muat | Panel galat dengan tombol "Try again" dan `request_id` yang bisa disalin |
| 404 | Halaman "Not found or you don't have access" (satu pesan untuk tidak ada dan tidak berizin) |
| 403 | Hanya untuk aksi pada item yang terlihat: aksi disembunyikan lewat `PermissionGate`, dan toast bila tetap terpanggil |
| Offline / SSE putus | `LiveIndicator` abu-abu + banner "Reconnecting…"; edit tetap dikirim lewat REST |

### 5.4 Aksi destruktif dan Recycle bin

- Menghapus item **tidak** memakai dialog konfirmasi (dapat dipulihkan); gunakan toast dengan tombol **Undo** selama 8 detik (memanggil restore). Dialog konfirmasi hanya untuk aksi yang tidak bisa dibatalkan: menonaktifkan user, membatalkan undangan, mengosongkan Recycle bin (M6), menghapus view.
- Tombol destruktif memakai varian `destructive`; fokus awal pada Batal.

### 5.5 Pintasan keyboard (1.0)

| Pintasan | Aksi |
|---|---|
| `Ctrl/Cmd + K` | Buka CommandPalette (pencarian global) |
| `C` | Buat task di konteks saat ini |
| `Esc` | Tutup panel/dialog/menu |
| `J` / `K` | Baris berikutnya / sebelumnya di Table dan daftar |
| `Enter` | Buka panel detail baris terpilih |
| `E` | Edit judul item terpilih |
| `S` | Buka pemilih status pada item terpilih |
| `A` | Buka pemilih assignee |
| `Ctrl/Cmd + Enter` atau `Enter` | Kirim komentar (dapat dipilih di SET-02) |
| `?` | Daftar pintasan |

Pintasan satu huruf dinonaktifkan saat fokus berada di input atau editor. Setiap pintasan juga tersedia lewat menu (tidak ada fungsi yang hanya bisa lewat pintasan).

### 5.6 i18n

- Kunci terjemahan berbentuk `modul.layar.elemen`, contoh `item.detail.status.label`. Satu file JSON per modul di `i18n/en/`.
- Dilarang: menyambung string (`"Hello " + name`); pakai interpolasi `t('x', { name })`. Pluralisasi memakai fitur ICU/i18next (`_one`, `_other`).
- Tanggal, angka, dan zona waktu diformat lewat `Intl.*` memakai locale, timezone, `date_format`, dan `week_start` dari profil user (SET-02), bukan nilai tetap.
- Teks pada gambar, ikon dengan huruf, dan nama file ekspor tidak boleh mengandung string yang tidak diterjemahkan.
- Tata letak tidak boleh bergantung pada panjang teks bahasa Inggris (tombol dan kolom boleh bertambah ±40%).
- Tes CI: memindai kode untuk literal string pada JSX (aturan ESLint `no-literal-string` dengan pengecualian terdaftar).

### 5.7 Aksesibilitas (checklist per komponen baru)

- [ ] Dapat dijangkau dan dioperasikan penuh dengan keyboard; urutan fokus logis; tidak ada jebakan fokus kecuali di dialog (yang mengembalikan fokus ke pemicu saat ditutup).
- [ ] Peran dan nama ARIA benar (`role`, `aria-label`, `aria-expanded`, `aria-selected`); ikon-saja punya label teks.
- [ ] Kontras teks ≥ 4.5:1 dan komponen UI ≥ 3:1, di mode terang **dan** gelap.
- [ ] Fokus terlihat (cincin `border.focus`, ≥ 2 px).
- [ ] Informasi tidak hanya lewat warna (status = titik + teks).
- [ ] Target klik ≥ 24×24 px (≥ 32 px di compact untuk aksi utama).
- [ ] Drag-and-drop punya alternatif keyboard (dnd-kit `KeyboardSensor`) dan menu "Move to…".
- [ ] Perubahan dinamis (toast, hasil pencarian, update realtime) diumumkan lewat `aria-live="polite"`.
- [ ] Hormati `prefers-reduced-motion` dan `prefers-color-scheme` (opsi Light/Dark/System di Settings).
- [ ] Diuji dengan `axe` di Playwright pada layar utama.

### 5.8 Responsif

Target utama ≥ 1280 px. Di bawah 1024 px: sidebar menciut menjadi ikon; panel detail menjadi layar penuh. Di bawah 768 px (ponsel) hanya mode baca dan ubah status/komentar yang dijamin (PRD §6).

## 6. Komponen yang ditunda (M2+)

GanttView (SVAR), CalendarView, WidgetCard (ECharts), FormBuilder (dnd-kit), ApprovalCard, TimesheetGrid, ReportTable, WorkflowEditor, CustomFieldEditor. Desainnya ditulis saat milestone-nya dimulai.
