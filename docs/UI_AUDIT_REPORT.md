# Audit visual dan aksesibilitas PMS

**Skor 64/100.** Login dan formulir publik tetap dapat dibaca pada empat ukuran. Alur inti desktop dapat digunakan dalam fixture. Pada mobile, sidebar menyisakan 142 px untuk konten workspace. Masalah ini menjalar ke Table, Board, detail tugas, laporan, formulir admin, dan Settings. Calendar juga melampaui viewport tablet. Gantt menghasilkan struktur ARIA yang tidak valid pada dua viewport yang dipindai.

Baseline `main` pada `0872b69`. Chromium melalui Playwright dengan Chrome lokal, locale Inggris, zona Asia/Jakarta, jam tetap 13 November 2026 pukul 12.00 WIB, fixture admin dan data sintetis. Sebanyak 256 screenshot mencakup 64 skenario pada empat ukuran: 1440×900, 1280×800, 768×1024, 390×844. Skor adalah penilaian audit terhadap cakupan ini, dengan bobot setara. Ini bukan skor konformitas WCAG.

| Dimensi | Skor | Dasar penilaian |
|---|---:|---|
| Layout dan responsivitas | 7/20 | Alur desktop terbaca. Mobile menyempit secara sistemik dan Calendar melampaui tablet. |
| Tipografi, warna, dan hierarki | 15/20 | Inter dan tema konsisten pada sampel. Tingkat judul dan keterbacaan data padat bervariasi. |
| Konsistensi komponen | 14/20 | Primitif dialog, input dan tombol digunakan ulang. Tabel, navigasi dan pola pesan tersebar di berbagai fitur. |
| Feedback dan state | 15/20 | Login, validasi, sukses, loading dan error terlihat. Space kosong menampilkan dua pesan dan 404 global minim arah pemulihan. |
| Aksesibilitas dasar | 13/20 | Fokus Login dan pemulihan fokus Share diuji. Gantt memunculkan dua kegagalan ARIA dan reflow memengaruhi akses pada mobile. |

## Metode dan batas cakupan

- Stack yang terbuka pada port 4174 tidak merespons. Audit memakai aplikasi repo yang dijalankan pada port 4195. `TestBrowserCollaboration` menjalankan tiga skenario Chrome dengan Go dan PostgreSQL terisolasi. Login dengan kredensial fixture, membuat proyek/tugas, drag status dan notifikasi lintas pengguna lulus. Screenshot menggunakan intersepsi API deterministik dan tidak memuat data klien. Indikator Connecting pada capture fixture tidak menjadi defect koneksi.
- Screenshot diambil pada viewport asli, setelah state target muncul, font siap dan dua frame render. Loading sengaja ditahan; 401, 404, 410 dan 500 disimulasikan untuk state relevan. Pengecekan ukuran memakai `document.documentElement.scrollWidth`. Scroll di dalam region data dinilai terpisah dari overflow dokumen.
- Pada 34 capture, axe menjalankan aturan WCAG 2 A/AA dan 2.1 AA pada sampel desktop besar dan mobile. Keyboard diuji untuk urutan tab Login, fokus kontrol Show, dan Share dibuka dengan Enter lalu ditutup dengan Escape. Pembacaan screen reader, Safari/Firefox, perangkat sentuh nyata, zoom 200%, semua kombinasi peran, density comfortable, brand klien, serta Bahasa Indonesia **NOT TESTED**. Pemeriksaan visual dan axe tidak menjadi bukti bahwa suatu route lulus seluruh interaksi.
- Bukti PNG tersimpan di [ui-audit-evidence](ui-audit-evidence/). Gambar adalah bukti audit dan tidak dipakai sebagai baseline snapshot. Ukuran viewport dan state tercantum dalam nama file.

## Inventory dan acuan desain

- Route runtime diambil dari `web/src/routeTree.gen.ts` dan file route TanStack. `_app`, `__root`, layout Settings/Users dan index yang mengalihkan URL dicatat sebagai shell/alias, bukan halaman tambahan. Detail Item dapat muncul sebagai route penuh atau panel `?task=`. Table, Board, Gantt dan Calendar memakai `?view=`. Filter/preset/sort/group, kolom, tab dan menu adalah state fitur di bawah route induk.
- Primitif yang ditemukan di `web/src/components/ui`: Button, Input, Dialog, Popover, DropdownMenu, DatePicker, Toaster. Primitif auth: AuthCard, TextField, PasswordStrength, FormError. Shell: Sidebar/SpacesNav, Topbar, SearchPalette, CreateItemDialog dan panel detail. Workspace: ViewWorkspace, ViewTabs, ViewColumns, ItemTable/Board/Calendar/Gantt, ShareDialog, SpaceDialog, CustomFieldsDialog, SaveBlueprintDialog, SpaceExcelImport. Detail: komentar dan mention, lampiran, status, assignee, tanggal, effort, progress, approvals, field kustom, lokasi, Timelog, StarButton dan Excel export. Form: FormRenderer, FormQuestionsEditor, FormResponseSettings, FormAccessDialog, RequestTabs. Analitik: QueryEditor, ResultTable, WidgetCard, WidgetChart. Admin: OnboardingWizard, TwoFactorSettings dan ConfirmDialog. Banyak state tambahan berada langsung di file route.
- [DESIGN_SYSTEM.md](DESIGN_SYSTEM.md) dan [UI_SPEC.md](UI_SPEC.md) menjadi acuan. Runtime CSS di `web/src/index.css` memetakan warna semantik untuk permukaan, teks, border, brand/fokus, status dan enam warna kategori. Font Inter dan JetBrains Mono berasal dari Fontsource lokal. Skala dasar 4 px, radius kontrol/kartu/modal 6/8/12 px, baris compact/comfortable 32/40 px, teks 13/14 px, sidebar 248/56 px. Spesifikasi meminta fokus terlihat, urutan keyboard logis, alternatif keyboard untuk drag, AA di kedua tema, dan penggunaan token semantik. Gantt, chart dan sebagian fitur M2+ memiliki cakupan pedoman yang lebih terbatas.
- [docs/screens](screens/) berisi 540 PNG dalam 99 kelompok referensi Wrike. Design system menetapkannya sebagai acuan fungsi. Mockup PMS yang disetujui, baseline pixel Playwright dan desain resmi untuk breakpoint **NOT TESTED** karena tidak ditemukan di checkout. Perbandingan di bawah mengacu pada perilaku, hierarki, dan token yang didokumentasikan.

## Temuan

### UI-01 · P1 · Usability dan visual · Shell workspace di mobile

**Expected.** Navigasi dapat dibuka saat diperlukan dan konten utama menerima lebar yang memadai sejak pertama login. Judul tugas panjang membungkus secara wajar.

**Actual.** Pada 390 px sidebar mengambil 248 px (64%). Kolom utama hanya 142 px. Lebar dokumen mencapai 489 px pada Home, 583 px pada Table dan Board, serta 557 px pada detail Task. Judul detail tampak memanjang hampir satu karakter per baris. Login dan form publik tetap 390 px tanpa shell.

**Penyebab terverifikasi.** `sidebar.tsx` memakai sidebar `shrink-0` dengan lebar 248 px secara default dan state collapse dari penyimpanan lokal. `_app.tsx` menempatkannya dalam flex row tanpa mode sempit. Collapse menjadi 56 px menyembunyikan navigasi Space dan beberapa pintasan sampai sidebar dibuka kembali.

**Perbaikan dan kriteria penerimaan.** Tambahkan navigasi overlay pada viewport sempit dengan tombol yang bisa dioperasikan melalui keyboard. Pada 390 px pengguna dapat membuka Space, melihat Table dan membuka detail judul panjang. Lebar dokumen sama dengan viewport pada shell dan halaman umum. Region data yang lebar tetap bisa digulir sendiri. Verifikasi desktop 1280 dan 1440 serta pengembalian fokus ketika overlay ditutup.

Bukti [Home 390](ui-audit-evidence/home-390.png) · [Table 390](ui-audit-evidence/space-table-390.png) · [Task 390](ui-audit-evidence/task-390.png) · [Task 1440](ui-audit-evidence/task-1440.png).

### UI-02 · P1 · Usability dan visual · Settings tetap dua kolom

**Expected.** Pengguna dapat membaca dan mengubah Settings pada viewport sempit, termasuk setelah sidebar utama diciutkan.

**Actual.** Profile memiliki lebar dokumen 661 px pada viewport 390. Setelah sidebar diciutkan menjadi 56 px, dokumen masih 469 px. Account mencapai 827 px dalam konfigurasi awal. Pada viewport 768, isi tetap menyempit karena dua navigasi berdampingan.

**Penyebab terverifikasi.** `settings.tsx` menetapkan grid `15rem 1fr`, gap 32 px dan padding 32 px tanpa breakpoint. Account menambah grid label tetap 12 rem di dalamnya.

**Perbaikan dan kriteria penerimaan.** Pada ukuran sempit, pindahkan navigasi Settings ke pemilih atau daftar di atas konten dan susun label/nilai vertikal bila ruang kurang. Profile, Security, Account, Users, Work Schedule dan Workflows dapat dibaca dan dioperasikan pada 390 dan 768 tanpa scroll horizontal dokumen. Fokus serta navigasi kembali bekerja dengan keyboard.

Bukti [Profile 390](ui-audit-evidence/profile-390.png) · [Profile dengan sidebar ciut 390](ui-audit-evidence/profile-collapsed-390.png) · [Account 390](ui-audit-evidence/account-390.png).

### UI-03 · P1 · Aksesibilitas · Semantik grid Gantt

**Expected.** Role grid dan atribut baris valid. Hierarki proyek dan tugas disampaikan dengan semantik yang sesuai dan indeks ARIA dimulai dari 1.

**Actual.** Axe pada Gantt 1440 dan 390 melaporkan `aria-valid-attr-value` (critical menurut axe) untuk `aria-rowindex="0"` dan `aria-conditional-attr` (serious) untuk `aria-expanded` pada row di dalam grid. Tingkat produk P1 karena temuan mengganggu representasi hierarki bagi teknologi bantu pada satu view.

**Penyebab terverifikasi.** Markup dihasilkan grid SVAR yang dipakai `item-gantt.tsx`. Paket terpasang `@svar-ui/react-grid` meneruskan indeks baris dan status ekspansi ke DOM. Pengalaman aktual screen reader **NOT TESTED**.

**Perbaikan dan kriteria penerimaan.** Periksa versi SVAR yang memperbaiki markup atau gunakan integrasi yang mendukung semantik hierarki. Kedua aturan axe bersih pada Gantt terang/gelap 1440 dan 390. Verifikasi manual nama, urutan dan ekspansi baris serta akses keyboard yang dideskripsikan dalam UI_SPEC.

Bukti [Gantt 1440](ui-audit-evidence/space-gantt-1440.png) · [Gantt 390](ui-audit-evidence/space-gantt-390.png). Atribut yang gagal dicatat di bagian Actual.

### UI-04 · P2 · Visual dan usability · Region Calendar meluapkan dokumen

**Expected.** Timeline atau grid yang membutuhkan dua dimensi dapat digulir di region sendiri, sementara header, aksi dan navigasi tetap dalam viewport.

**Actual.** Calendar menghasilkan dokumen 903 px pada viewport 768 dan 390. Pada 768, sisi kanan bulan berada di luar viewport awal. View Table pada fixture yang sama memiliki lebar dokumen 768 px.

**Penyebab.** `item-calendar.tsx` sudah menyediakan region dengan `overflow-x-auto`. Hipotesis berdasarkan layout dan pengukuran: ukuran minimum grid atau salah satu ancestor tetap memperlebar dokumen. Node pertama yang meluapkan layout perlu diisolasi saat perbaikan.

**Perbaikan dan kriteria penerimaan.** Batasi lebar region Calendar di dalam kolom workspace. Pada 768 dan 390 semua aksi bulan terlihat, dokumen tidak bergulir horizontal, dan pengguna bisa menjangkau ketujuh kolom dengan sentuh atau keyboard.

Bukti [Calendar 768](ui-audit-evidence/space-calendar-768.png) · [Calendar 390](ui-audit-evidence/space-calendar-390.png) · [Table 768](ui-audit-evidence/space-table-768.png).

### UI-05 · P2 · Usability · Kolom Link di Manage forms

**Expected.** Label dan aksi tabel tetap dapat dipindai. URL dapat disalin tanpa memaksa pembaca menelusuri token atau ID yang dipecah per karakter.

**Actual.** Pada 768, URL internal membungkus menjadi deretan baris sempit dan menggelembungkan tinggi baris. Dokumen mencapai 736 px pada viewport 390 dengan shell awal.

**Penyebab terverifikasi.** `forms.tsx` menampilkan URL utuh dalam `code.break-all` di sel yang juga memuat tombol salin, tanpa batas tampilan URL.

**Perbaikan dan kriteria penerimaan.** Gunakan label ringkas atau ellipsis dengan aksi salin berlabel, lalu sediakan cara membuka alamat lengkap. Pada 768, satu baris formulir contoh tetap mudah dipindai. Tombol salin mengambil URL asli dan dapat diaktifkan lewat keyboard.

Bukti [Manage forms 768](ui-audit-evidence/forms-768.png) · [Manage forms 390](ui-audit-evidence/forms-390.png).

### UI-06 · P2 · Usability · Route global 404 tanpa pemulihan

**Expected.** URL yang salah menghasilkan halaman yang menjelaskan masalah dan menyediakan cara kembali ke titik awal.

**Actual.** URL tak dikenal menampilkan teks “Not Found” di sudut halaman, tanpa heading atau aksi pemulihan. Error Space dan form publik memiliki hierarki judul yang lebih jelas.

**Penyebab terverifikasi.** Router tidak mengonfigurasi tampilan not-found global pada `main.tsx` atau `routes/__root.tsx`.

**Perbaikan dan kriteria penerimaan.** Tambahkan halaman 404 dengan heading, deskripsi dan aksi kembali yang relevan untuk pengguna login dan anonim. Aksi bisa dijangkau melalui Tab dan tampil pada seluruh viewport.

Bukti [404 global 390](ui-audit-evidence/not-found-390.png) · [Space tidak ditemukan 390](ui-audit-evidence/space-error-390.png).

### UI-07 · P3 · Visual dan usability · Feedback kosong ganda di Space

**Expected.** Workspace yang belum berisi data memberi satu pesan kontekstual. Pesan “No matching items” digunakan ketika filter menyaring semua data.

**Actual.** Space kosong menampilkan “No projects yet” di atas tabel dan “No matching items” di bawah header tabel meskipun preset default.

**Penyebab terverifikasi.** Route Space merender pesan kosong tingkat overview sementara `ViewWorkspace` tetap merender state kosong koleksi.

**Perbaikan dan kriteria penerimaan.** Untuk Space baru, tampilkan satu state kosong dan aksi tambah yang relevan bila diizinkan. Dengan data yang disaring habis, tampilkan hanya pesan filter dan aksi untuk meresetnya.

Bukti [Space kosong 1440](ui-audit-evidence/space-empty-1440.png) · [Space berisi data 1440](ui-audit-evidence/space-table-1440.png).

### UI-08 · P3 · Visual dan design system · Skala judul workspace

**Expected.** Level judul workspace memakai skala yang disepakati menurut peran dan density. Draft Design System §3 menyebut judul halaman 20 px compact dan 22 px comfortable.

**Actual.** Pada fixture default, ukuran computed H1 Settings 20 px, Manage forms dan Timesheets 24 px, dan Home 30 px. Status Home sebagai judul hero yang berbeda belum ditentukan secara eksplisit pada acuan.

**Penyebab terverifikasi.** Ukuran dipilih dengan kelas utilitas pada masing-masing route tanpa token atau komponen judul bersama untuk peran tersebut.

**Perbaikan dan kriteria penerimaan.** Sepakati pengecualian yang memang diperlukan dan samakan judul dengan peran setara. Validasi ukuran computed pada kedua density dan empat viewport.

Bukti [Home 1440](ui-audit-evidence/home-1440.png) · [Settings 1440](ui-audit-evidence/settings-1440.png) · [Manage forms 1440](ui-audit-evidence/forms-1440.png).

## Urutan perbaikan incremental

1. **P1 UI-01**. Benahi shell mobile dahulu karena dampaknya lintas route. Ulangi capture Home, Space, Table, Board dan detail task pada empat viewport.
2. **P1 UI-02**. Ubah tata letak Settings setelah shell stabil. **P1 UI-03** bisa dikerjakan independen. Ulangi axe dan periksa Gantt dengan keyboard serta screen reader.
3. **P2 UI-04 dan UI-05**. Sesuaikan region Calendar dan tabel form setelah mendapatkan ukuran konten yang stabil. Lanjutkan **P2 UI-06** untuk jalur pemulihan 404.
4. **P3 UI-07 dan UI-08**. Satukan state kosong dan rapikan skala judul. Review screenshot hasil perbaikan sebelum menetapkannya sebagai baseline.

P0 tidak ditemukan dalam skenario yang diperiksa. Tidak ada defect functional yang terverifikasi pada skenario live yang dijalankan. Temuan visual, usability dan aksesibilitas di atas tetap terbuka. Semua route/state di bawah diberi status **Ditinjau**, **Temuan**, atau **NOT TESTED** sesuai bukti.

## Matriks route dan state

Setiap sel merujuk ke capture pada ukuran yang tertera. **D** berarti state ditinjau. **T** berarti ada temuan terkait. **NT** berarti state target belum terverifikasi. State yang tidak disebut pada baris route tetap **NOT TESTED**.

| Route | State yang ditinjau | 1440×900 | 1280×800 | 768×1024 | 390×844 |
|---|---|---|---|---|---|
| `/login` | Default, kredensial ditolak, fokus dan gelap | [D login](ui-audit-evidence/login-1440.png) · [D login-error](ui-audit-evidence/login-error-1440.png) · [D login-focus](ui-audit-evidence/login-focus-1440.png) · [D login-dark](ui-audit-evidence/login-dark-1440.png) | [D login](ui-audit-evidence/login-1280.png) · [D login-error](ui-audit-evidence/login-error-1280.png) · [D login-focus](ui-audit-evidence/login-focus-1280.png) · [D login-dark](ui-audit-evidence/login-dark-1280.png) | [D login](ui-audit-evidence/login-768.png) · [D login-error](ui-audit-evidence/login-error-768.png) · [D login-focus](ui-audit-evidence/login-focus-768.png) · [D login-dark](ui-audit-evidence/login-dark-768.png) | [D login](ui-audit-evidence/login-390.png) · [D login-error](ui-audit-evidence/login-error-390.png) · [D login-focus](ui-audit-evidence/login-focus-390.png) · [D login-dark](ui-audit-evidence/login-dark-390.png) |
| `/forgot-password` | Default | [D](ui-audit-evidence/forgot-1440.png) | [D](ui-audit-evidence/forgot-1280.png) | [D](ui-audit-evidence/forgot-768.png) | [D](ui-audit-evidence/forgot-390.png) |
| `/reset-password` | Default dan validasi mismatch | [D reset](ui-audit-evidence/reset-1440.png) · [D reset-validation](ui-audit-evidence/reset-validation-1440.png) | [D reset](ui-audit-evidence/reset-1280.png) · [D reset-validation](ui-audit-evidence/reset-validation-1280.png) | [D reset](ui-audit-evidence/reset-768.png) · [D reset-validation](ui-audit-evidence/reset-validation-768.png) | [D reset](ui-audit-evidence/reset-390.png) · [D reset-validation](ui-audit-evidence/reset-validation-390.png) |
| `/invitations/$token` | Aktif dan kedaluwarsa | [D invitation](ui-audit-evidence/invitation-1440.png) · [D invitation-expired](ui-audit-evidence/invitation-expired-1440.png) | [D invitation](ui-audit-evidence/invitation-1280.png) · [D invitation-expired](ui-audit-evidence/invitation-expired-1280.png) | [D invitation](ui-audit-evidence/invitation-768.png) · [D invitation-expired](ui-audit-evidence/invitation-expired-768.png) | [D invitation](ui-audit-evidence/invitation-390.png) · [D invitation-expired](ui-audit-evidence/invitation-expired-390.png) |
| `/f/$token` | Default, validasi, sukses dan link tidak tersedia | [D public-form](ui-audit-evidence/public-form-1440.png) · [D public-form-validation](ui-audit-evidence/public-form-validation-1440.png) · [D public-form-success](ui-audit-evidence/public-form-success-1440.png) · [D public-form-unavailable](ui-audit-evidence/public-form-unavailable-1440.png) | [D public-form](ui-audit-evidence/public-form-1280.png) · [D public-form-validation](ui-audit-evidence/public-form-validation-1280.png) · [D public-form-success](ui-audit-evidence/public-form-success-1280.png) · [D public-form-unavailable](ui-audit-evidence/public-form-unavailable-1280.png) | [D public-form](ui-audit-evidence/public-form-768.png) · [D public-form-validation](ui-audit-evidence/public-form-validation-768.png) · [D public-form-success](ui-audit-evidence/public-form-success-768.png) · [D public-form-unavailable](ui-audit-evidence/public-form-unavailable-768.png) | [D public-form](ui-audit-evidence/public-form-390.png) · [D public-form-validation](ui-audit-evidence/public-form-validation-390.png) · [D public-form-success](ui-audit-evidence/public-form-success-390.png) · [D public-form-unavailable](ui-audit-evidence/public-form-unavailable-390.png) |
| `/` | Home, loading shell dan modal onboarding tahap pertama | [D home](ui-audit-evidence/home-1440.png) · [D workspace-loading](ui-audit-evidence/workspace-loading-1440.png) · [D onboarding](ui-audit-evidence/onboarding-1440.png) | [D home](ui-audit-evidence/home-1280.png) · [D workspace-loading](ui-audit-evidence/workspace-loading-1280.png) · [D onboarding](ui-audit-evidence/onboarding-1280.png) | [D home](ui-audit-evidence/home-768.png) · [D workspace-loading](ui-audit-evidence/workspace-loading-768.png) · [D onboarding](ui-audit-evidence/onboarding-768.png) | [T home](ui-audit-evidence/home-390.png) · [D workspace-loading](ui-audit-evidence/workspace-loading-390.png) · [D onboarding](ui-audit-evidence/onboarding-390.png) |
| `/spaces/$spaceId` | Table berisi proyek, kosong, 404 dan gelap | [D space-table](ui-audit-evidence/space-table-1440.png) · [T space-empty](ui-audit-evidence/space-empty-1440.png) · [D space-error](ui-audit-evidence/space-error-1440.png) · [D space-dark](ui-audit-evidence/space-dark-1440.png) | [D space-table](ui-audit-evidence/space-table-1280.png) · [T space-empty](ui-audit-evidence/space-empty-1280.png) · [D space-error](ui-audit-evidence/space-error-1280.png) · [D space-dark](ui-audit-evidence/space-dark-1280.png) | [D space-table](ui-audit-evidence/space-table-768.png) · [T space-empty](ui-audit-evidence/space-empty-768.png) · [D space-error](ui-audit-evidence/space-error-768.png) · [D space-dark](ui-audit-evidence/space-dark-768.png) | [T space-table](ui-audit-evidence/space-table-390.png) · [T space-empty](ui-audit-evidence/space-empty-390.png) · [T space-error](ui-audit-evidence/space-error-390.png) · [T space-dark](ui-audit-evidence/space-dark-390.png) |
| `/spaces/$spaceId?view=…` | Board, Gantt terang/gelap, Calendar | [D space-board](ui-audit-evidence/space-board-1440.png) · [T space-gantt](ui-audit-evidence/space-gantt-1440.png) · [T gantt-dark](ui-audit-evidence/gantt-dark-1440.png) · [D space-calendar](ui-audit-evidence/space-calendar-1440.png) | [D space-board](ui-audit-evidence/space-board-1280.png) · [D space-gantt](ui-audit-evidence/space-gantt-1280.png) · [D gantt-dark](ui-audit-evidence/gantt-dark-1280.png) · [D space-calendar](ui-audit-evidence/space-calendar-1280.png) | [D space-board](ui-audit-evidence/space-board-768.png) · [D space-gantt](ui-audit-evidence/space-gantt-768.png) · [D gantt-dark](ui-audit-evidence/gantt-dark-768.png) · [T space-calendar](ui-audit-evidence/space-calendar-768.png) | [T space-board](ui-audit-evidence/space-board-390.png) · [T space-gantt](ui-audit-evidence/space-gantt-390.png) · [T gantt-dark](ui-audit-evidence/gantt-dark-390.png) · [T space-calendar](ui-audit-evidence/space-calendar-390.png) |
| `/spaces/$spaceId` | Share terang/gelap, Escape dan pemulihan fokus, Create Item validasi | [D space-share](ui-audit-evidence/space-share-1440.png) · [D share-dark](ui-audit-evidence/share-dark-1440.png) · [D share-keyboard](ui-audit-evidence/share-keyboard-1440.png) · [D create-validation](ui-audit-evidence/create-validation-1440.png) | [D space-share](ui-audit-evidence/space-share-1280.png) · [D share-dark](ui-audit-evidence/share-dark-1280.png) · [D share-keyboard](ui-audit-evidence/share-keyboard-1280.png) · [D create-validation](ui-audit-evidence/create-validation-1280.png) | [D space-share](ui-audit-evidence/space-share-768.png) · [D share-dark](ui-audit-evidence/share-dark-768.png) · [D share-keyboard](ui-audit-evidence/share-keyboard-768.png) · [D create-validation](ui-audit-evidence/create-validation-768.png) | [D space-share](ui-audit-evidence/space-share-390.png) · [D share-dark](ui-audit-evidence/share-dark-390.png) · [D share-keyboard](ui-audit-evidence/share-keyboard-390.png) · [D create-validation](ui-audit-evidence/create-validation-390.png) |
| `/spaces/$spaceId?task=…` | Panel detail, judul panjang | [D](ui-audit-evidence/item-panel-1440.png) | [D](ui-audit-evidence/item-panel-1280.png) | [D](ui-audit-evidence/item-panel-768.png) | [D](ui-audit-evidence/item-panel-390.png) |
| `/items/$itemId` | Detail proyek | [D](ui-audit-evidence/item-1440.png) | [D](ui-audit-evidence/item-1280.png) | [D](ui-audit-evidence/item-768.png) | [T](ui-audit-evidence/item-390.png) |
| `/tasks/$itemId` | Detail tugas, judul panjang | [D](ui-audit-evidence/task-1440.png) | [D](ui-audit-evidence/task-1280.png) | [D](ui-audit-evidence/task-768.png) | [T](ui-audit-evidence/task-390.png) |
| `/inbox` | Kosong | [D](ui-audit-evidence/inbox-1440.png) | [D](ui-audit-evidence/inbox-1280.png) | [D](ui-audit-evidence/inbox-768.png) | [T](ui-audit-evidence/inbox-390.png) |
| `/search` | Berisi hasil | [D](ui-audit-evidence/search-1440.png) | [D](ui-audit-evidence/search-1280.png) | [D](ui-audit-evidence/search-768.png) | [T](ui-audit-evidence/search-390.png) |
| `/my-todo` | Berisi tugas dan judul panjang | [D](ui-audit-evidence/todo-1440.png) | [D](ui-audit-evidence/todo-1280.png) | [D](ui-audit-evidence/todo-768.png) | [T](ui-audit-evidence/todo-390.png) |
| `/stream` | Kosong | [D](ui-audit-evidence/stream-1440.png) | [D](ui-audit-evidence/stream-1280.png) | [D](ui-audit-evidence/stream-768.png) | [T](ui-audit-evidence/stream-390.png) |
| `/shared-with-me` | Kosong | [D](ui-audit-evidence/shared-1440.png) | [D](ui-audit-evidence/shared-1280.png) | [D](ui-audit-evidence/shared-768.png) | [D](ui-audit-evidence/shared-390.png) |
| `/starred` | Kosong | [D](ui-audit-evidence/starred-1440.png) | [D](ui-audit-evidence/starred-1280.png) | [D](ui-audit-evidence/starred-768.png) | [D](ui-audit-evidence/starred-390.png) |
| `/recycle-bin` | Berisi item | [D](ui-audit-evidence/recycle-1440.png) | [D](ui-audit-evidence/recycle-1280.png) | [D](ui-audit-evidence/recycle-768.png) | [T](ui-audit-evidence/recycle-390.png) |
| `/timesheets` | Mingguan dengan entri | [D](ui-audit-evidence/timesheets-1440.png) | [D](ui-audit-evidence/timesheets-1280.png) | [D](ui-audit-evidence/timesheets-768.png) | [T](ui-audit-evidence/timesheets-390.png) |
| `/forms` | Daftar, loading dan error | [D forms](ui-audit-evidence/forms-1440.png) · [D forms-loading](ui-audit-evidence/forms-loading-1440.png) · [D forms-error](ui-audit-evidence/forms-error-1440.png) | [D forms](ui-audit-evidence/forms-1280.png) · [D forms-loading](ui-audit-evidence/forms-loading-1280.png) · [D forms-error](ui-audit-evidence/forms-error-1280.png) | [T forms](ui-audit-evidence/forms-768.png) · [D forms-loading](ui-audit-evidence/forms-loading-768.png) · [D forms-error](ui-audit-evidence/forms-error-768.png) | [T forms](ui-audit-evidence/forms-390.png) · [T forms-loading](ui-audit-evidence/forms-loading-390.png) · [T forms-error](ui-audit-evidence/forms-error-390.png) |
| `/spaces/$spaceId/forms` | Daftar | [D](ui-audit-evidence/space-forms-1440.png) | [D](ui-audit-evidence/space-forms-1280.png) | [D](ui-audit-evidence/space-forms-768.png) | [T](ui-audit-evidence/space-forms-390.png) |
| `/spaces/$spaceId/forms/$formId` | Editor berisi data dan route new | [D form-builder](ui-audit-evidence/form-builder-1440.png) · [D form-builder-new](ui-audit-evidence/form-builder-new-1440.png) | [D form-builder](ui-audit-evidence/form-builder-1280.png) · [D form-builder-new](ui-audit-evidence/form-builder-new-1280.png) | [D form-builder](ui-audit-evidence/form-builder-768.png) · [D form-builder-new](ui-audit-evidence/form-builder-new-768.png) | [T form-builder](ui-audit-evidence/form-builder-390.png) · [T form-builder-new](ui-audit-evidence/form-builder-new-390.png) |
| `/requests` | Daftar formulir | [D](ui-audit-evidence/requests-1440.png) | [D](ui-audit-evidence/requests-1280.png) | [D](ui-audit-evidence/requests-768.png) | [T](ui-audit-evidence/requests-390.png) |
| `/requests/$formId` | Form internal default | [D](ui-audit-evidence/request-form-1440.png) | [D](ui-audit-evidence/request-form-1280.png) | [D](ui-audit-evidence/request-form-768.png) | [D](ui-audit-evidence/request-form-390.png) |
| `/requests/mine` | Kosong | [D](ui-audit-evidence/my-requests-1440.png) | [D](ui-audit-evidence/my-requests-1280.png) | [D](ui-audit-evidence/my-requests-768.png) | [T](ui-audit-evidence/my-requests-390.png) |
| `/dashboards` | Daftar | [D](ui-audit-evidence/dashboards-1440.png) | [D](ui-audit-evidence/dashboards-1280.png) | [D](ui-audit-evidence/dashboards-768.png) | [T](ui-audit-evidence/dashboards-390.png) |
| `/dashboards/$dashboardId` | Detail dengan chart | [D](ui-audit-evidence/dashboard-1440.png) | [D](ui-audit-evidence/dashboard-1280.png) | [D](ui-audit-evidence/dashboard-768.png) | [T](ui-audit-evidence/dashboard-390.png) |
| `/reports` | Daftar | [D](ui-audit-evidence/reports-1440.png) | [D](ui-audit-evidence/reports-1280.png) | [D](ui-audit-evidence/reports-768.png) | [T](ui-audit-evidence/reports-390.png) |
| `/reports/$reportId` | Editor report | [D](ui-audit-evidence/report-1440.png) | [D](ui-audit-evidence/report-1280.png) | [D](ui-audit-evidence/report-768.png) | [T](ui-audit-evidence/report-390.png) |
| `/settings` | Redirect ke Security | [D](ui-audit-evidence/settings-1440.png) | [D](ui-audit-evidence/settings-1280.png) | [T](ui-audit-evidence/settings-768.png) | [T](ui-audit-evidence/settings-390.png) |
| `/settings/profile` | Default dan sidebar ciut | [D profile](ui-audit-evidence/profile-1440.png) · [D profile-collapsed](ui-audit-evidence/profile-collapsed-1440.png) | [D profile](ui-audit-evidence/profile-1280.png) · [D profile-collapsed](ui-audit-evidence/profile-collapsed-1280.png) | [T profile](ui-audit-evidence/profile-768.png) · [T profile-collapsed](ui-audit-evidence/profile-collapsed-768.png) | [T profile](ui-audit-evidence/profile-390.png) · [T profile-collapsed](ui-audit-evidence/profile-collapsed-390.png) |
| `/settings/account` | Data akun dan administrator | [D](ui-audit-evidence/account-1440.png) | [D](ui-audit-evidence/account-1280.png) | [T](ui-audit-evidence/account-768.png) | [T](ui-audit-evidence/account-390.png) |
| `/settings/security` | Password dan 2FA nonaktif | [D](ui-audit-evidence/security-1440.png) | [D](ui-audit-evidence/security-1280.png) | [T](ui-audit-evidence/security-768.png) | [T](ui-audit-evidence/security-390.png) |
| `/settings/users` | Daftar dan seat | [D](ui-audit-evidence/users-1440.png) | [D](ui-audit-evidence/users-1280.png) | [T](ui-audit-evidence/users-768.png) | [T](ui-audit-evidence/users-390.png) |
| `/settings/users/types` | Matriks izin | [D](ui-audit-evidence/user-types-1440.png) | [D](ui-audit-evidence/user-types-1280.png) | [T](ui-audit-evidence/user-types-768.png) | [T](ui-audit-evidence/user-types-390.png) |
| `/settings/work-schedule` | Minggu kerja default | [D](ui-audit-evidence/work-schedule-1440.png) | [D](ui-audit-evidence/work-schedule-1280.png) | [T](ui-audit-evidence/work-schedule-768.png) | [T](ui-audit-evidence/work-schedule-390.png) |
| `/settings/workflows` | Workflow dan status default | [D](ui-audit-evidence/workflows-1440.png) | [D](ui-audit-evidence/workflows-1280.png) | [T](ui-audit-evidence/workflows-768.png) | [T](ui-audit-evidence/workflows-390.png) |
| `/hello` | Diagnostik server tersedia | [D](ui-audit-evidence/hello-1440.png) | [D](ui-audit-evidence/hello-1280.png) | [D](ui-audit-evidence/hello-768.png) | [D](ui-audit-evidence/hello-390.png) |
| `/spikes/gantt` | Spike SVAR | [D](ui-audit-evidence/spike-gantt-1440.png) | [D](ui-audit-evidence/spike-gantt-1280.png) | [D](ui-audit-evidence/spike-gantt-768.png) | [D](ui-audit-evidence/spike-gantt-390.png) |
| `(route tak dikenal)` | 404 global | [T](ui-audit-evidence/not-found-1440.png) | [T](ui-audit-evidence/not-found-1280.png) | [T](ui-audit-evidence/not-found-768.png) | [T](ui-audit-evidence/not-found-390.png) |

**State tambahan NOT TESTED** meliputi onboarding tahap 2–4, login 2FA dan backup code, konfirmasi email reset, Settings Groups, pengguna non-admin, sharing eksternal, hapus/pulihkan, impor/ekspor Excel, date picker, perubahan status keyboard, konflik versi, chart editor, Timelog, approval, upload dan komentar sebagai audit visual individual. Alur live memberi bukti fungsional terbatas untuk komentar/mention dan unggah.
