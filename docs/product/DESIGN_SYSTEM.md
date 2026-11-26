# DESIGN SYSTEM — PMS

| | |
|---|---|
| **Versi** | 0.1 (draft) |
| **Cakupan** | Komponen dan pola untuk M0 + M1. Komponen M2+ (Gantt, chart, form builder) hanya dicatat di §6 |
| **Fondasi** | `apps/ui-kit` (Tailwind v4 + `@base-ui/react` + `class-variance-authority`), token di `ui-kit/design-tokens.json` |
| **Terkait** | [UI_SPEC.md](UI_SPEC.md), [ARCHITECTURE.md](../architecture/ARCHITECTURE.md) §12 |

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
| `semantic.size.sidebar-width` = 270 px (min 220, maks 400, ciut 56 px) | Sekarang token CSS `--sidebar-width*`, lihat §2.1 |
| `semantic.z.{dropdown,sticky,panel,modal,toast}` | Skala z-index tunggal: 10 / 20 / 30 / 40 / 50 |
| `semantic.motion.{fast,base}` = 120 ms / 200 ms, easing `cubic-bezier(.2,0,0,1)`; nol pada `prefers-reduced-motion` | |

### 2.1 Token CSS (`web/src/index.css`)

Sumber tunggal untuk warna, ukuran, dan motion di PMS. Light didefinisikan di `:root`, dark di `.dark`. Nilai di tabel adalah nilai light kecuali kolom "Dark".

**Navigasi dan permukaan (S-UX.0.1)**

| Token | Light / Dark | Fungsi |
|---|---|---|
| `--nav` | `#192238` / `#192238` | Latar sidebar navigasi (navy). Belum menjadi default `--sidebar`, menunggu persetujuan pemilik |
| `--nav-muted` | `#A0AEC5` | Teks sekunder di navigasi (grup, hint) |
| `--nav-text` | `#E2E8F3` | Teks item navigasi |
| `--nav-hover` | `#283650` | Latar item navigasi saat hover |
| `--nav-active` | `#354866` | Latar item navigasi yang aktif |
| `--selection` | `#EAF1FF` / `#1E3A5F` | Latar baris atau item terpilih |
| `--surface-2` | `#F0F4F8` / `#263245` | Permukaan kedua (panel samping, area kelompok) di atas `--card` |
| `--shadow-overlay` | `0 12px 40px rgba(23,32,51,0.2)` / `0 12px 40px rgba(0,0,0,0.5)` | Bayangan popover, menu, dan panel mengambang |
| `--sidebar` | `#FFFFFF` / `var(--card)` | Tidak diubah. Sidebar masih putih sampai keputusan navy disetujui |

Token navigasi juga tersedia sebagai kelas Tailwind: `bg-nav`, `text-nav-text`, `text-nav-muted`, `bg-nav-hover`, `bg-nav-active`, `bg-selection`, `bg-surface-2`.

**Skala tipografi (px)**: `--font-size-12`, `--font-size-13`, `--font-size-14`, `--font-size-16`, `--font-size-19`, `--font-size-24`. Berat: `--font-weight-regular` (400), `--font-weight-semibold` (600), `--font-weight-bold` (650).

**Spasi (kelipatan 4 px)**: `--space-1` (4), `--space-2` (8), `--space-3` (12), `--space-4` (16), `--space-5` (20), `--space-6` (24), `--space-8` (32), `--space-10` (40), `--space-12` (48).

**Tinggi**

| Token | Nilai | Fungsi |
|---|---|---|
| `--control-h` | 32 px | Tinggi tombol, input, dan select (compact) |
| `--control-h-lg` | 40 px | Tinggi kontrol di mode comfortable atau aksi utama |
| `--table-row-h` | 40 px | Tinggi baris tabel baru |
| `--row-h` | 32 px compact / 40 px comfortable | Sudah ada, mengikuti `data-density`. Tidak diubah di S-UX.0.1 |

**Lebar sidebar**: `--sidebar-width` 270 px (default), `--sidebar-width-min` 220 px, `--sidebar-width-max` 400 px, `--sidebar-width-collapsed` 56 px.

**Motion**: `--motion-fast` 120 ms, `--motion-base` 160 ms, `--motion-slow` 200 ms. Di `@media (prefers-reduced-motion: reduce)` ketiganya menjadi 0 ms.

**Radius** sudah ada di `@theme inline` (`--radius-sm` sampai `--radius-4xl`). Jangan mendefinisikan ulang.

**Contoh pemakaian**

```tsx
// Benar: token
<aside className="w-[var(--sidebar-width)] bg-nav text-nav-text">
<li className="hover:bg-nav-hover data-[active=true]:bg-nav-active transition-colors duration-[var(--motion-fast)]">
<div className="text-[length:var(--font-size-13)] p-[var(--space-3)] h-[var(--control-h)]">

// Salah: nilai langsung di komponen
<aside className="w-[248px] bg-[#192238] text-[#E2E8F3]">
<li className="transition-colors duration-150">
```

**Aturan**

1. Tidak ada kode hex (`#RRGGBB`, `#RGB`) dan `rgb()`/`hsl()` literal di `web/src/components` dan `web/src/routes`. Pakai token.
2. Tidak ada ukuran font, spasi, tinggi, lebar sidebar, atau durasi yang ditulis angka langsung bila sudah ada token. Pakai `var(--...)`.
3. Warna baru ditambahkan dulu sebagai token di `index.css` (light dan `.dark`), baru dipakai.
4. Teks utama dan teks status harus memenuhi kontras 4.5:1 di light dan dark. Lihat tabel di bawah.
5. Langkah CI yang memindai hex di dua folder tersebut ditambahkan di S-UX.0.2.

**Kontras yang sudah dihitung (WCAG, rasio)**

| Pasangan | Light | Dark | Status |
|---|---|---|---|
| `--foreground` di `--background` / `--card` | 16,98 / 17,74 | 16,98 / 14,05 | Lulus |
| `--foreground` di `--surface-2` | 16,05 | 12,37 | Lulus |
| `--muted-foreground` di `--background` / `--card` | 4,63 / 4,83 | 12,04 / 9,96 | Lulus |
| `--muted-foreground` di `--surface-2` | **4,37** | 8,77 | **Gagal (light)** |
| `--nav-text` di `--nav` | 12,86 | 12,86 | Lulus |
| `--nav-muted` di `--nav` | 7,05 | 7,05 | Lulus |
| `--nav-text` di `--nav-hover` / `--nav-active` | 9,85 / 7,52 | 9,85 / 7,52 | Lulus |
| `--info` di `--card` | 5,17 (di putih) | 8,14 | Lulus |
| `--destructive` di `--card` | 4,83 | 7,73 | Lulus |
| `--success` di `--card` | **3,30** | 10,45 | **Gagal (light)** |
| `--warning` di `--card` | **3,19** | 10,18 | **Gagal (light)** |

Teks status `--success` dan `--warning` di light belum memenuhi 4.5:1 sebagai teks. Nilai token tidak diubah karena dipakai di tempat lain. Usulan: tambah token teks terpisah (mis. `--success-text` dan `--warning-text`) dengan nilai yang lebih gelap, atau pakai warna ini hanya sebagai ikon/titik dengan teks netral. Keputusan menunggu. Untuk `--muted-foreground` di `--surface-2`, jangan dipakai sebagai teks di atas permukaan itu sampai nilainya disetel ulang.

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
