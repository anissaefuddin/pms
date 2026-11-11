# Analisa Fitur Screen — Wrike Web

Sumber: 99 file zip (`Wrike Web *.zip`) → 540 screenshot, diekstrak ke [`screens/`](screens/) (satu folder per flow).
Analisa dibuat dari pembacaan visual tiap screenshot; hanya yang terlihat di layar yang dicatat.

## Ringkasan Modul

| Modul | Jumlah flow | Screen |
|---|---|---|
| 1. Autentikasi & Onboarding | 7 | 60 |
| 2. Navigasi, Inbox & Pencarian | 10 | 38 |
| 3. Space, Project & Folder | 19 | 95 |
| 4. Views (Table/Board/Gantt/Calendar/Timelog) | 10 | 35 |
| 5. Task & Item | 17 | 109 |
| 6. Workflow & Status | 3 | 26 |
| 7. Request Form | 4 | 31 |
| 8. Dashboard, Widget & Analytics | 11 | 56 |
| 9. Report | 3 | 17 |
| 10. Menu & Kustomisasi | 1 | 6 |
| 11. Akun, Profil & Settings | 7 | 33 |
| 12. Pengguna & Tim | 4 | 25 |
| 13. Recycle Bin | 3 | 9 |

## Daftar Isi

- **1. Autentikasi & Onboarding**: Logging in, Logging out, Resetting password, Onboarding, Accepting a team invitation, Setting up two-factor authentication, Changing password
- **2. Navigasi, Inbox & Pencarian**: Inbox, Marking all as read, Stream, My to-do, Filtering & sorting to-do, Searching Wrike, Filtering search results, Adding a bookmark, Customizing sidebar menu, Leaving a feedback
- **3. Space, Project & Folder**: Creating a space, Space overview, Filtering space presets, Projects and folders, Creating a project, Creating a folder, View project detail, Editing a project name, Changing project status, Setting project dates, Adding a project location, Sharing a project, Approving a project, Adding an approval, Creating a blueprint, Customizing blueprints menu, Exporting project to excel, Creating a public snapshot, Copying a link
- **4. Views (Table/Board/Gantt/Calendar/Timelog)**: Board view, Gantt chart view, Timelog view, Adding a calendar view, Setting a default view, Duplicating a view, Renaming a view, Sorting by group, Editing card preview, Filtering custom fields
- **5. Task & Item**: Adding a task, Adding a new item (board), Adding a new item (gantt), Adding a subitem, Editing an item, Changing a status, Setting a date, Adding an assignee, Adding a description, Adding a comment, Adding a file, Setting progress, Adding impact, Editing impact settings, Editing a budget, Editing budget settings, Editing a timesheet
- **6. Workflow & Status**: Creating a workflow, Editing workflow, Creating a status
- **7. Request Form**: Creating a request form, Customizing a form, Manage forms, Sending a request form
- **8. Dashboard, Widget & Analytics**: Dashboard, Adding a custom widget, Adding widget description, Editing a widget, Switching widget view, Changing layout, Duplicating a dashboard, Renaming a dashboard, Deleting a dashboard, Creating a public link, Creating custom analytics
- **9. Report**: Creating a report, Editing a report, Subscribing to a report
- **10. Menu & Kustomisasi**: Customizing a menu
- **11. Akun, Profil & Settings**: Settings, Account info, Updating profile details, Updating email preferences, Switching to dark mode, Work schedule, Apps & Integrations
- **12. Pengguna & Tim**: Inviting a teammate, User types, Deleting a user, Deleting an account
- **13. Recycle Bin**: Recycle bin, Restoring an item, Emptying recycle bin

---

## 1. Autentikasi & Onboarding

### Logging in
*6 screen — [`screens/Logging in/`](screens/Logging%20in/)*
- **Lokasi/entry point:** Situs pemasaran Wrike (tombol/tautan "Log in") > halaman login.
- **Langkah:**
  1. Di halaman beranda klik "Log in".
  2. Isi "Email or corporate ID" lalu klik "Next" (alternatif: login dengan Google, Apple, atau Office).
  3. Layar kedua "Enter your password": email ditampilkan dengan tautan "Wrong email?"; isi Password lalu klik "Log in".
  4. Jika salah: field berwarna merah dengan tooltip "Please double check your email address and/or password".
  5. Isi ulang kata sandi yang benar lalu "Log in".
- **Komponen UI:** Kartu login dua langkah, input, tombol Next/Log in, tautan Forgot password?, tombol SSO (Google, Sign in with Apple, Office), tooltip error, tautan Back.
- **Field/data:** Email atau corporate ID; Password; tautan "Don't have a Wrike account yet? Get started now"; footer: Have multiple accounts? Log in here, Log in with One-Time Password, Get in touch, Help Center.
- **Catatan:**
  - Pesan error tidak membedakan email salah atau kata sandi salah.
  - Kata sandi ditampilkan sebagai titik; ada tombol Back untuk kembali mengubah email.

### Logging out
*2 screen — [`screens/Logging out/`](screens/Logging%20out/)*
- **Lokasi/entry point:** Avatar pengguna (inisial) di bagian atas sidebar.
- **Langkah:**
  1. Klik avatar; menu profil terbuka (nama Jane Smith, jabatan Designer, organisasi Mobbin).
  2. Pilih "Log out" di bagian bawah menu.
  3. Pengguna diarahkan kembali ke halaman beranda/login Wrike.
- **Komponen UI:** Popover menu profil, halaman pemasaran dengan tombol "Log in".
- **Field/data:** Menu: Settings, Workspace appearance, Recycle bin, Apps & Integrations, Mobile apps, Help, Privacy policy, Log out.
- **Catatan:**
  - Tidak ada dialog konfirmasi sebelum logout.

### Resetting password
*6 screen — [`screens/Resetting password/`](screens/Resetting%20password/)*
- **Lokasi/entry point:** Halaman login langkah "Enter your password" > tautan "Forgot password?".
- **Langkah:**
  1. Pada layar "Enter your password" klik "Forgot password?".
  2. Layar "Password restore": email sudah terisi, klik "Submit" (tombol menjadi nonaktif setelah dikirim).
  3. Setelah tautan reset dibuka (tidak terlihat), halaman "Enter your new password" tampil.
  4. Ketik password baru: bila lemah, field merah dengan pesan dan panel kekuatan password.
  5. Ketik password kuat + konfirmasi (ikon centang hijau), klik "Submit".
- **Komponen UI:** Kartu login terpusat, input password, tombol Submit, tautan "Back", popover aturan password ("Good password rules"), indikator error/sukses di field, ilustrasi latar.
- **Field/data:** Login: email (dengan "Wrong email?"), Password. Restore: email. Reset: New password, Confirm new password. Aturan: gunakan kata/frasa tidak umum, huruf besar non-standar, angka/simbol tidak jelas; "Password must NOT: Have less than 6 characters"; "Current password strength: Too weak".
- **Catatan:**
  - Validasi kekuatan password real-time dengan pesan "Your password isn't strong enough".
  - Tombol Submit nonaktif setelah klik sebagai penanda proses/anti dobel kirim.

### Onboarding
*27 screen — [`screens/Onboarding/`](screens/Onboarding/)*
- **Lokasi/entry point:** Halaman landing wrike.com: kolom email di header (placeholder "name@company.com") + tombol "Try Wrike for free"; alternatif tombol "Log in" / "Contact us".
- **Langkah:**
  1. Klik kolom email di header, ketik email, klik "Try Wrike for free".
  2. Modal wizard "Create your account" (langkah 1 dari 7): isi nama, kode negara + telepon, nama perusahaan, negara, centang Terms of Service/Privacy Policy, klik Next.
  3. Langkah 2 "Just a few quick questions": pilih ukuran perusahaan dan jumlah orang yang akan berkolaborasi, Next.
  4. Langkah 3 "Tell us a bit about what you do": pilih tim/departemen dan job role dari autosuggest, Next.
  5. Langkah 4 "What's your team working on?": isi nama project (pratinjau board langsung berubah).
  6. Langkah 5 "What needs to be done?": isi 3 tugas awal (muncul sebagai kartu di pratinjau board kolom New/In Progress).
  7. Langkah 6 "How do you want to see your work?": pilih tampilan Table / Gantt / Board.
  8. Langkah 7 "Start building your team": isi hingga 4 email undangan, Next.
  9. Layar loading "Preparing your workspace...".
  10. Modal sambutan "Welcome, Jane! Your free 14-day trial starts today": pilih fitur yang ingin dicoba (Proofing & approvals, Requests, Digital asset management) atau "explore on your own".
  11. Carousel fitur (tab Reviews, Requests, DAM, Views, Reporting, Adobe Sync) dengan "Check it out" / "Next feature" / "Book a call"; tutup (X) untuk masuk ke workspace (board project dengan checklist Quick start).
- **Komponen UI:** Modal wizard dengan stepper 7 titik dan tombol Next; input teks; dropdown kode negara & negara; checkbox persetujuan; radio-card (pilihan ukuran); autosuggest dropdown; pratinjau workspace live di sisi kanan; layar loading animasi; modal welcome dengan kartu pilihan; carousel fitur; sidebar checklist "Quick start" dengan progress bar.
- **Field/data:** Full name, kode negara (+1), phone number, company name (ikon bantuan), country of residence, checkbox ToS; ukuran perusahaan (2-25, 26-100, 101-250, 251-5,000, 5,001+, It's just me!); jumlah kolaborator (2-5, 6-10, 11-20, 21-50, 51+, It's just me!); team/department (Administrative, C-Level, Creative Design, Engineering, Finance, HR, IT, ...); job role; project name (placeholder "e.g. New Logo"); 3 task (placeholder "e.g. Sketch ideas"); pilihan view Table/Gantt/Board; 4 email undangan; sisa masa trial 14 hari.
- **Catatan:**
  - Validasi inline: field telepon kosong menjadi merah dengan pesan "Enter your phone number. Why?".
  - Teks bantuan "You can always invite more people later" dan "you can change views later" menurunkan hambatan pengisian.
  - Pratinjau workspace di sisi kanan berubah mengikuti input pengguna (nama project, tugas, jenis view).

### Accepting a team invitation
*7 screen — [`screens/Accepting a team invitation/`](screens/Accepting%20a%20team%20invitation/)*
- **Lokasi/entry point:** Layar "Create an account to join the team" (modal di atas workspace yang diburamkan) setelah pengguna mengikuti undangan tim.
- **Langkah:**
  1. Modal onboarding tampil dengan kolom nama aktif (fokus).
  2. Isi nama lengkap ("Sam Lee").
  3. Isi password; password lemah memunculkan error merah "Your password isn't strong enough".
  4. Password kuat menampilkan centang hijau dan panel "Current password strength: Strong" berisi aturan password.
  5. Pilih department ("Product") dan job title ("Product Designer").
  6. Centang "I agree to the Terms of Service and the Privacy Policy".
  7. Klik "Get started" lalu masuk ke workspace (tampilan Gantt "Feature 2.0" di space Creative Team).
- **Komponen UI:** Modal penuh dengan ilustrasi, input teks, input password dengan ikon info/validasi, tooltip kekuatan password, dropdown department & job title, checkbox persetujuan, tombol primer "Get started".
- **Field/data:** Full name, Password, Department, Job title, checkbox Terms of Service/Privacy Policy.
- **Catatan:**
  - Validasi password real-time (error merah vs centang hijau); aturan: gunakan kata tak umum, huruf besar non-standar, angka/simbol; tidak boleh kurang dari 6 karakter.
  - Checkbox persetujuan belum dicentang pada layar awal, dicentang sebelum submit.

### Setting up two-factor authentication
*8 screen — [`screens/Setting up two-factor authentication/`](screens/Setting%20up%20two-factor%20authentication/)*
- **Lokasi/entry point:** Settings > Personal settings > Profile > halaman "Sign-in & Security" > bagian "Two Step verification" > tombol "Enable".
- **Langkah:**
  1. Klik "Enable".
  2. Modal "Enter password" meminta password; ketik lalu "Confirm".
  3. Halaman "Setting up Two Step verification": pilih aplikasi autentikator (Google Authenticator Android/iOS atau Microsoft Authenticator Windows), klik "Continue".
  4. Setup Google Authenticator: pindai QR code (Opsi 1) atau masukkan secret key (Opsi 2).
  5. Masukkan kode 6 digit, klik "Verify".
  6. Halaman "Congratulations!" > "Go to profile settings".
  7. Sign-in & Security kini menunjukkan 2-step aktif dengan tombol "Reconfigure" dan "Turn Off".
- **Komponen UI:** Halaman pengaturan dengan sidebar, modal konfirmasi password, radio list aplikasi, instruksi bernomor, QR code, kotak secret key, input kode 6 digit, halaman sukses.
- **Field/data:** Old/New/Repeat password; Two Step verification (Enable); Authorized Apps ("Open Apps & Integrations"); Enter your password; pilihan aplikasi; QR code, secret key (contoh "R2IH XEWO WNDV TONT"); kode verifikasi 6 digit (placeholder "E.g. 123456"). Status aktif: "enabled since 27 May 2024 via Google Authenticator for iOS".
- **Catatan:**
  - Pengubahan setelan keamanan memerlukan konfirmasi ulang password.
  - Dua jalur setup (QR atau kunci manual) disediakan; setelah aktif tersedia Reconfigure/Turn Off.

### Changing password
*4 screen — [`screens/Changing password/`](screens/Changing%20password/)*
- **Lokasi/entry point:** Personal settings > Profile > bagian "Sign-in & Security" > "Password & 2-Step Verification" > "Change password".
- **Langkah:**
  1. Buka Sign-in & Security; form berisi Old password, New password, Repeat password (tombol "Change password" nonaktif).
  2. Isi Old password dan New password (ikon centang hijau jika kuat).
  3. Jika Repeat password tidak cocok, field merah dan pesan "Password does not match the confirm password".
  4. Perbaiki hingga cocok; tombol "Change password" menjadi hijau/aktif.
  5. Klik tombol; banner biru di atas "Your password has been changed successfully" dan form dikosongkan.
- **Komponen UI:** Form tiga input password, ikon validasi, pesan error inline, tombol primer, banner sukses yang dapat ditutup, panel Two Step verification ("Enable"), seksi Authorized Apps ("Open Apps & Integrations").
- **Field/data:** Old password, New password, Repeat password; email utama jdoe@content-mobbin.com (Primary) dengan "Add email address"; Two Step verification via aplikasi autentikator.
- **Catatan:**
  - Validasi real-time pada kekuatan dan kecocokan password.
  - Tombol aksi hanya aktif jika seluruh field valid.


---

## 2. Navigasi, Inbox & Pencarian

### Inbox
*3 screen — [`screens/Inbox/`](screens/Inbox/)*
- **Lokasi/entry point:** Item "Inbox" di sidebar (badge merah jumlah notifikasi belum dibaca).
- **Langkah:**
  1. Buka Inbox; tab "Incoming" menampilkan notifikasi dari Wrike Bot dengan waktu relatif (58 m, 4 h) dan titik biru untuk belum dibaca.
  2. Panel kanan kosong dengan ilustrasi lonceng hingga satu notifikasi dipilih.
  3. Pindah ke tab "Sent": pesan yang dikirim pengguna (dengan @mention dan tombol "Go to full details"), dikelompokkan per tanggal.
  4. Pindah ke tab "Archive": empty state "Nothing archived yet".
- **Komponen UI:** Tab (Incoming/Sent/Archive), daftar notifikasi master-detail, kartu onboarding yang dapat ditutup (Inbox help, mobile app, integrations), ikon filter dan menu "...", empty state.
- **Field/data:** Notifikasi: waktu, nama tugas, pengirim, cuplikan pesan, indikator belum dibaca; kartu: "Read more about inbox", "Try mobile", "Explore integrations".
- **Catatan:**
  - Kartu onboarding menjelaskan kapan notifikasi muncul (ditugaskan, di-mention, diberi akses, perlu dilibatkan).
  - Archive menjelaskan fungsinya: memindahkan notifikasi yang sudah selesai untuk merapikan Inbox.

### Marking all as read
*3 screen — [`screens/Marking all as read/`](screens/Marking%20all%20as%20read/)*
- **Lokasi/entry point:** Inbox > tab "Incoming"; menu "..." di bar tab.
- **Langkah:**
  1. Buka Inbox: ada notifikasi belum dibaca (titik biru, badge merah di sidebar).
  2. Klik menu "..." di kanan atas daftar.
  3. Menu menampilkan "Mark all as read" dan "Archive all"; pilih "Mark all as read".
  4. Titik biru dan badge merah pada Inbox di sidebar hilang; item tetap berada di daftar.
- **Komponen UI:** Menu dropdown, indikator belum dibaca (titik biru), badge jumlah di sidebar, kartu onboarding.
- **Field/data:** Opsi menu: Mark all as read, Archive all; ikon filter di sebelahnya.
- **Catatan:**
  - Tidak ada dialog konfirmasi; teks notifikasi tidak lagi dicetak tebal setelah dibaca.

### Stream
*3 screen — [`screens/Stream/`](screens/Stream/)*
- **Lokasi/entry point:** Sidebar > "More" > "Stream".
- **Langkah:**
  1. Klik "More" lalu pilih "Stream" dari menu.
  2. Halaman feed aktivitas terbuka (judul pada screenshot: "Shared with me"), berisi kartu task beserta riwayat perubahan.
  3. Gulir untuk melihat kartu lain; filter di kanan.
- **Komponen UI:** Menu "More", feed kartu task (judul, tag project, status, "Backlogged"), log aktivitas bertanggal, kotak komentar (lampiran, @mention, emoji, Send), panel Filters (radio) dan checkbox.
- **Field/data:** Filters: Everything, Everything I followed (terpilih), Assigned to me, My conversations; checkbox "Hide auto updates". Kartu: Brand pillars, High-fidelity and prototype, Wireframes, Concept (project Design assets; status Completed/In progress/In review).
- **Catatan:**
  - Komentar dapat langsung ditulis dari setiap kartu feed.
  - Judul halaman pada screenshot tampil "Shared with me" meski diakses dari menu Stream.

### My to-do
*4 screen — [`screens/My to-do/`](screens/My%20to-do/)*
- **Lokasi/entry point:** Menu "More" di sidebar > My to-do.
- **Langkah:**
  1. Klik "More"; popover menampilkan Dashboards, My to-do, Request forms, Timesheets, Reports, Calendars, Workload, Created by me, Stream, Starred tasks, Customize menu.
  2. Pilih "My to-do": daftar tugas milik pengguna dikelompokkan per waktu (Tomorrow, This week) dengan filter "Only active tasks" dan sort "Date".
  3. Ubah filter menjadi "All tasks" dan sort menjadi "Status": daftar dikelompokkan per status.
  4. Klik menu "..." untuk opsi tampilan: Task card display (One-line/Two-line), Task list display (Plain/Hierarchical), Show in list (Durations), Print, Export project to Excel.
  5. Pilih Two-line: baris menampilkan status dan nama proyek di baris kedua, serta durasi (mis. 28 May (3d)).
- **Komponen UI:** Popover menu navigasi, daftar tugas berkelompok, tombol "+ Item", filter/sort dropdown, menu opsi tampilan dengan centang, baris tugas (avatar, judul, tag proyek, tanggal, ikon tipe, status berwarna).
- **Field/data:** Judul tugas, tag proyek (Feature 2.0, My first event), tanggal, status (Not Started, In review, Planned, Completed), label workflow (Task workflow, Campaign Workflow), durasi.
- **Catatan:**
  - Baris induk dapat di-expand (panah) saat mode hierarkis aktif.
  - Terdapat opsi Print dan Export ke Excel langsung dari menu "...".

### Filtering & sorting to-do
*5 screen — [`screens/Filtering & sorting to-do/`](screens/Filtering%20%26%20sorting%20to-do/)*
- **Lokasi/entry point:** Halaman "My to-do" (More > My to-do); tombol filter dan sort di bar atas daftar.
- **Langkah:**
  1. Buka My to-do (default "Only active tasks", sort "Date", dikelompokkan Tomorrow/This week).
  2. Klik filter lalu pilih "All tasks", "Only active tasks", atau "Only overdue tasks".
  3. Pilih "All tasks": grup "Completed (1)" muncul.
  4. Klik sort lalu pilih kriteria (Priority, Date, Date modified, Date created, Status, Importance, Title).
  5. Pilih "Status": daftar dikelompokkan per status (Planned, In review, Not started, Completed) dengan label workflow di kanan.
- **Komponen UI:** Dropdown filter (centang), dropdown sort (centang), daftar tugas berkelompok dengan header, baris tugas (avatar, judul, tag proyek, tanggal, status), tombol "+ Item".
- **Field/data:** Filter: All tasks / Only active tasks / Only overdue tasks; Sort: Priority, Date, Date modified, Date created, Status, Importance, Title; label workflow "Task workflow" dan "Campaign Workflow".
- **Catatan:**
  - Label filter/sort di toolbar berubah sesuai pilihan (mis. "All tasks", "Status").
  - Judul grup menampilkan jumlah item, mis. "NOT STARTED (3)".

### Searching Wrike
*4 screen — [`screens/Searching Wrike/`](screens/Searching%20Wrike/)*
- **Lokasi/entry point:** Sidebar kiri > "Search" (pintasan ⌘K).
- **Langkah:**
  1. Klik Search; modal pencarian terbuka menampilkan recent projects/folders dan recent tasks.
  2. Ketik kata kunci ("design"); hasil dikelompokkan dan tab ruang lingkup tersedia.
  3. Klik "Search" untuk membuka halaman hasil lengkap.
  4. Klik salah satu hasil; panel detail task terbuka di kanan.
  5. Tutup panel; daftar hasil tampil penuh.
- **Komponen UI:** Modal pencarian dengan input dan tombol Search, tab ruang lingkup, daftar bergrup dengan "Show more", halaman "Search results" dengan filter dan sort, panel detail task.
- **Field/data:** Tab: Everywhere, In Creative Team, In Feature 2.0. Grup hasil: Tasks (3), Projects and Folders (2), Comments (2), Users (2), Request forms (1). Halaman hasil: tombol "1 filter", sort "Relevance", status (Planned), rentang tanggal. Panel: Status, Assignee, Date, subitems, dependency, aktivitas, komentar. Pintasan "More recent tasks Ctrl+Shift+E".
- **Catatan:**
  - Pencarian mencakup beberapa tipe entitas (task, project, komentar, pengguna, request form) sekaligus.
  - Saat belum mengetik, modal menampilkan item terbaru sebagai jalan pintas.

### Filtering search results
*3 screen — [`screens/Filtering search results/`](screens/Filtering%20search%20results/)*
- **Lokasi/entry point:** Halaman "Search results" (hasil pencarian global dengan kata kunci "Design").
- **Langkah:**
  1. Lihat hasil pencarian (3 item, status Planned) dengan sort "Relevance" dan tombol "1 filter".
  2. Klik "1 filter": dropdown menampilkan Filter Presets (All tasks, All active tasks, My active tasks) dan Applied task filters ("Keyword: Design").
  3. Pilih "Add more filters..." (atau "Clear filters") untuk menambah kriteria.
  4. Setelah filter bertambah (tombol "3 filters"), hasil menyempit menjadi 2 item (Initial Design, Design review).
- **Komponen UI:** Dropdown filter, chip filter terapan, daftar hasil dengan sorotan kata kunci tebal, ikon avatar, label status.
- **Field/data:** Preset filter; Applied filter (Keyword); Add more filters; Clear filters; sort Relevance; kolom: judul, durasi tanggal (mis. 5 Jun (3d)), status.
- **Catatan:**
  - Kata kunci yang cocok ditebalkan di judul hasil.
  - Jumlah filter aktif ditampilkan pada label tombol.

### Adding a bookmark
*3 screen — [`screens/Adding a bookmark/`](screens/Adding%20a%20bookmark/)*
- **Lokasi/entry point:** Header space "Creative Team", tautan "Add bookmark" di bawah judul (di samping "Public" dan "2 members").
- **Langkah:**
  1. Klik "Add bookmark" sehingga modal "Add bookmark" muncul.
  2. Isi URL, Title, dan Description (opsional), lalu Save.
  3. Bookmark dapat diedit lewat modal "Edit 'Team resource'" (URL Google Docs, Title "Team resources", Description "Admin, HR, reimbursements, etc.").
  4. Setelah disimpan, kartu bookmark "Team resources" muncul di bawah header dan penghitung berubah menjadi "1 bookmark".
- **Komponen UI:** Modal kecil dengan 3 input, tombol Save/Cancel, kartu bookmark dengan ikon dokumen.
- **Field/data:** URL, Title, Description (optional).
- **Catatan:**
  - Latar Gantt Chart tetap terlihat di belakang modal.
  - Placeholder: "Enter URL", "Enter title", "Enter optional description".

### Customizing sidebar menu
*5 screen — [`screens/Customizing sidebar menu/`](screens/Customizing%20sidebar%20menu/)*
- **Lokasi/entry point:** Sidebar kiri → "More" → "Customize menu" (bagian bawah popup).
- **Langkah:**
  1. Klik "More" di sidebar; popup menampilkan Dashboards, My to-do, Request forms, Timesheets, Reports, Calendars, Workload, Created by me, Stream, Starred tasks, Customize menu.
  2. Klik "Customize menu": modal dengan "Shown in menu" (Search, Inbox) dan "Hidden from menu".
  3. Arahkan kursor ke item tersembunyi (mis. Dashboards) dan klik ikon mata ("Show"), atau drag-and-drop untuk mengatur urutan.
  4. Dashboards dan Request forms pindah ke "Shown in menu"; klik Save.
  5. Sidebar kini menampilkan Dashboards dan Request forms langsung di bawah Inbox.
- **Komponen UI:** Popup menu, modal dengan dua daftar, drag handle, ikon mata dengan tooltip, tombol Save/Cancel.
- **Field/data:** Item yang bisa ditampilkan/disembunyikan: Created by me, Workload, Calendars, Dashboards, My to-do, Request forms, Timesheets, Reports, Stream, Starred tasks. Search dan Inbox tampil permanen (abu-abu).
- **Catatan:**
  - Instruksi: "Choose what you'd like to see in your menu, then drag and drop to reorder."

### Leaving a feedback
*5 screen — [`screens/Leaving a feedback/`](screens/Leaving%20a%20feedback/)*
- **Lokasi/entry point:** Dashboard; dari modal "Create a public link" terdapat tautan "Leave feedback about public links" yang membuka panel "Assistant / Leave feedback".
- **Langkah:**
  1. Klik "Public link" di Dashboard; modal "Create a public link" menampilkan tautan, masa berlaku, dan opsi kata sandi.
  2. Klik "Leave feedback about public links" di bagian bawah modal.
  3. Panel samping "Leave feedback" terbuka dengan subjek terisi "Public links".
  4. Pilih penilaian (3 ikon emoji) dan isi kotak teks komentar.
  5. Klik "Send" (aktif setelah teks diisi).
  6. Layar "Thank you for your feedback!" dengan tautan "Back".
- **Komponen UI:** Modal public link (toggle, tombol Copy, dropdown masa berlaku, field kata sandi dengan tampil/regenerasi/salin), panel samping (drawer), pemilih subjek, rating emoji, textarea, tombol Send, ilustrasi terima kasih.
- **Field/data:** Public link: toggle Dashboard public link, dibuat oleh, URL, Link expires (tanggal/jam), Require password; Feedback: Subject (pilih atau ketik), Rate your experience, komentar bebas, tautan "this form" untuk melaporkan masalah.
- **Catatan:**
  - Subjek terisi otomatis sesuai konteks fitur.
  - Tombol Send berwarna abu-abu sebelum teks diisi.


---

## 3. Space, Project & Folder

### Creating a space
*7 screen — [`screens/Creating a space/`](screens/Creating%20a%20space/)*
- **Lokasi/entry point:** Tombol "+" di header kanan atas → "Space"; alternatif lewat dropdown "Space" di sidebar.
- **Langkah:**
  1. Klik "+" → Space; halaman "Create space" / "Create a new space" tampil dengan galeri template.
  2. Telusuri template (kategori di kiri) atau cari; arahkan kursor ke kartu untuk melihat deskripsi dan tombol Preview.
  3. Klik template (Event Management) untuk membuka halaman detail dengan video tutorial dan daftar "Available features"; klik Select (atau "Create blank").
  4. Layar loading "Wait a few seconds till we prepare your workspace."
  5. Video "Use Case Template Tutorial" muncul sebagai pop-up; setelah ditutup, space baru terbuka (tampilan Planner berisi data contoh).
- **Komponen UI:** Halaman penuh Create space, sidebar kategori, kotak pencarian, kartu template dengan hover overlay (Preview), halaman detail dengan video YouTube tertanam, tombol Select/Create blank/Cancel, layar loading, modal video.
- **Field/data:** Kategori: All templates, Get started with the basics, Finance, HR, IT & engineering, Marketing team, Operations, Project management, Services & consulting. Template rekomendasi: Creative production, Event Management, Content Operations, Campaign Management, Social Media. Fitur template: Projects, Tasks, Table view, Gantt charts, Request forms, Custom fields, Custom workflows, Custom item types. Hasil: tab Partners and Vendors, Target KPIs, Event Analytics, Planner; kolom Item type, Name, Assignee, Stakeholder, Status, Start/Due date, Target Budget.
- **Catatan:**
  - Bagian "Recommended" disesuaikan ("Because you're in Marketing").
  - Space baru berisi proyek contoh "My first event (click to edit)".

### Space overview
*3 screen — [`screens/Space overview/`](screens/Space%20overview/)*
- **Lokasi/entry point:** Sidebar > Spaces > "Creative Team" > "Space overview".
- **Langkah:**
  1. Dari view project (Board "New Product"), klik "Space overview" di sidebar.
  2. Halaman space tampil dalam view Table (semua project/task lintas space).
  3. Pindah ke tab Board; kartu task seluruh space ditampilkan per status.
- **Komponen UI:** Header space (Public, 2 members, Add bookmark, ikon pengaturan/info), tab view (Table, Board, Gantt Chart, All), toolbar, tabel hierarki, board Kanban.
- **Field/data:** Kolom Table: Name, Assignee, Status, Due date, Budget, Impact (Low/Medium/High). Contoh baris: New Product, Research, Stakeholder Research, Requirement gathering, Initial Design, Marketing request. Kolom Board: New, Planned, In progress, In review.
- **Catatan:**
  - Space overview menggabungkan semua project/task dalam space dan menampilkan field kustom (Budget, Impact).

### Filtering space presets
*6 screen — [`screens/Filtering space presets/`](screens/Filtering%20space%20presets/)*
- **Lokasi/entry point:** Tampilan Gantt Chart space Creative Team; toolbar (filter "All tasks", sort "Due date", "Expand").
- **Langkah:**
  1. Klik "All tasks": dropdown Filter Presets (All tasks - dicentang, All active tasks, My active tasks, More filters...).
  2. Pilih "All active tasks"; baris tugas selesai tersaring dan titik biru muncul pada tab tampilan.
  3. Klik tombol sort lalu pilih urutan (Priority, Due date, Status, Name, Predecessors (Cascade), Gantt manual order).
  4. Pilih "Status"; baris tabel diurutkan berdasarkan status.
  5. Klik "Expand" > "Expand all"/"Collapse all"; "Collapse all" hanya menyisakan baris induk.
- **Komponen UI:** Dropdown preset filter, dropdown sort (centang), menu Expand, tabel hierarki + Gantt, indikator titik biru "tampilan diubah".
- **Field/data:** Preset: All tasks, All active tasks, My active tasks; sort: Priority, Due date, Status, Name, Predecessors (Cascade), Gantt manual order; kolom Name, Status, Start date, Due date, Impact.
- **Catatan:**
  - Tab Gantt Chart menampilkan titik biru saat filter/sort mengubah tampilan default.
  - Label tombol toolbar mengikuti pilihan aktif ("All active tasks", "Status").

### Projects and folders
*3 screen — [`screens/Projects and folders/`](screens/Projects%20and%20folders/)*
- **Lokasi/entry point:** Sidebar kiri > Spaces > Creative Team > bagian "Projects and folders" (ikon + untuk menambah); klik project "New Product".
- **Langkah:**
  1. Pilih project dari sidebar; terbuka dengan tampilan Board (kolom status).
  2. Pindah tab ke Table (daftar hierarki task/subitem dengan kolom).
  3. Pindah tab ke Gantt chart (tabel kiri + timeline kanan, dependency antar bar).
- **Komponen UI:** Tab view (Table, Board, Gantt chart, "All" dropdown), toolbar (All tasks, Priority, Columns, Group, Cards, Fields, Subitems, Public link, Due date, Expand, Public snapshots, undo/redo), kartu Kanban, tabel dengan baris expand/collapse, bagan Gantt, widget bantuan "Gantt Chart view", tombol Share/Automation, badge sisa hari trial, checklist Quick start di sidebar.
- **Field/data:** Kolom Board: New, Planned, In progress, In review. Kolom Table: Name, Assignee, Status (New, Planned, In review, Ready for review, Completed), Start date, Due date; Gantt: Name, Status, Start date, Due date, Impact (Low/Medium/High). Kartu menampilkan avatar assignee, rentang tanggal dan jumlah subitem.
- **Catatan:**
  - Project dan folder dibedakan dengan ikon berbeda di sidebar dan pada kartu.
  - Task dapat bersarang (project > task > subitem) dan tampil sebagai indentasi di Table/Gantt.

### Creating a project
*15 screen — [`screens/Creating a project/`](screens/Creating%20a%20project/)*
- **Lokasi/entry point:** Tombol "+" di header kanan atas → menu "Select an item to create" → "Project".
- **Langkah:**
  1. Klik "+" lalu pilih Project.
  2. Modal "Create project" terbuka dengan kolom Settings; isi Project name (Website Revamp).
  3. Pilih Location lewat dropdown pencarian (Feature 2.0, Creative Team, Personal, Event management, My first event); lokasi bisa dihapus/diganti (terpilih Creative Team).
  4. Pilih Default task workflow dari dropdown (Creative Team workflows: New workflow, Task workflow).
  5. Atur Project owner (avatar JS bisa dihapus, muncul "Add users", diganti SL/Sam Lee).
  6. Isi Start date, Finish date (04/06/2024 – 04/07/2024), Billing type, Budget (500).
  7. Pilih Default view (Board) dan Sharing (avatar peserta ditambah).
  8. Klik Create; proyek terbuka di tampilan Board dengan kolom Backlog dan Completed serta tombol "+ Status".
- **Komponen UI:** Dropdown "+", modal dua panel, input teks, dropdown pencarian, dropdown workflow, tooltip nama (Jane Smith), date picker, select Billing type, kartu view, radio Sharing, tombol Create/Cancel.
- **Field/data:** Project name, Location, Default task workflow, radio Project/Folder, Project owner, Start date, Finish date, Billing type (Non-billable), Budget ($), Default view (Table, Board, Gantt Chart, Calendar, Files, Chart), Sharing (Private / Shared with my team / Specific users and groups).
- **Catatan:**
  - Default view awalnya Gantt Chart; berubah saat pengguna memilih Board.
  - Panel Sharing berisi avatar dengan tombol "+" untuk menambah anggota.

### Creating a folder
*7 screen — [`screens/Creating a folder/`](screens/Creating%20a%20folder/)*
- **Lokasi/entry point:** Tombol hijau "+" di pojok kanan atas header (dari tampilan Gantt proyek Feature 2.0), menu "Select an item to create" → pilih "Folder".
- **Langkah:**
  1. Klik "+" lalu pilih Folder (menu berisi Task, Project, Folder, Space, Request, Invite, Customize this menu).
  2. Modal "Create folder" terbuka; isi "Folder name" (contoh: Design assets).
  3. Pilih Location lewat link "Select Project, Folder or Space" (terisi "Creative Team").
  4. Radio "Folder" dipilih (alternatif: Project); pilih default view (Table dipilih).
  5. Atur Sharing (Private / Shared with my team / Specific users and groups).
  6. Klik Create; folder muncul di sidebar "Projects and folders" dan terbuka di tampilan Table, awalnya kosong (hanya baris "+ Item"), kemudian terisi item (Concept, Wireframes, High-fidelity and prototype, Brand pillars).
- **Komponen UI:** Menu dropdown "+", modal dua kolom, input teks, link pemilih lokasi, dropdown Task workflow, radio Project/Folder, kartu pilihan view, radio Sharing, avatar peserta dengan tombol "+", tombol Create/Cancel.
- **Field/data:** Folder name, Location, Default task workflow, tipe (Project/Folder), Default view (Table, Board, Gantt Chart, Calendar, Files, Chart), Sharing. Tabel hasil: Name, Assignee, Status (Completed, In review, In progress), Start date, Due date. Tab view folder: Table, Timelog, Resources.
- **Catatan:**
  - Sharing default "Private"; saat Specific users and groups dipilih muncul avatar pengguna dan tombol tambah.
  - Penjelasan singkat di bawah radio: Folder tidak punya atribut seperti Status dan tanggal seperti Project.

### View project detail
*2 screen — [`screens/View project detail/`](screens/View%20project%20detail/)*
- **Lokasi/entry point:** Halaman project dalam view Gantt chart (Space > Projects and folders > New Product) > klik nama project di baris pertama.
- **Langkah:**
  1. Buka project "New Product" di view Gantt (tabel + timeline; widget bantuan "Gantt Chart view" di pojok).
  2. Klik nama project; panel detail terbuka di sisi kanan dan tabel menyempit.
  3. Panel menampilkan status, assignee, tanggal, subitem, deskripsi, aktivitas, dan kolom komentar.
- **Komponen UI:** Gantt chart dengan tabel kiri, panel detail geser dari kanan, field status dropdown, avatar assignee, link "Go to full details", activity log, kotak komentar (lampiran, @, emoji, Aa, Send), ikon salin tautan / menu / tutup.
- **Field/data:** Status (In Progress), Assignee (Jane Smith), Date ("Set date"), "3 subitems", Add files, Add approval, "Add a description", aktivitas ("Renamed project from First project to New Product").
- **Catatan:**
  - Panel detail dibuka tanpa meninggalkan konteks view (Gantt tetap terlihat di sebelah kiri).
  - Tersedia "Go to full details" untuk halaman penuh.

### Editing a project name
*3 screen — [`screens/Editing a project name/`](screens/Editing%20a%20project%20name/)*
- **Lokasi/entry point:** Tampilan Gantt chart proyek (mis. "New Product") di space Creative Team; panel detail proyek terbuka di sisi kanan.
- **Langkah:**
  1. Buka panel detail proyek di sisi kanan tampilan Gantt chart.
  2. Klik judul proyek sehingga menjadi field editable (placeholder "Add name to create").
  3. Ketik nama baru ("Feature 2.0" + emoji) lalu konfirmasi.
  4. Nama baru muncul di header tabel, panel detail, dan sidebar; tercatat di log aktivitas.
- **Komponen UI:** Panel detail (side panel), judul inline-edit, feed aktivitas/komentar, tabel + Gantt, sidebar proyek.
- **Field/data:** Status (In Progress), Assignee (Jane Smith), Date (Set date), 3 subitems, Add files, Add approval, Add a description, Go to full details.
- **Catatan:**
  - Perubahan nama dicatat otomatis di feed: "Renamed project from First project to New Product" lalu "from New Product to Feature 2.0".
  - Nama mendukung emoji.

### Changing project status
*3 screen — [`screens/Changing project status/`](screens/Changing%20project%20status/)*
- **Lokasi/entry point:** Panel detail project "Feature 2.0", dropdown Status (mis. "In Progress") di baris Status/Assignee/Date.
- **Langkah:**
  1. Klik dropdown Status; daftar "Default Workflow" tampil: Backlog, In Progress, Ready for Design, Completed, On Hold, Cancelled, dan "Edit workflow".
  2. Pilih "Backlog".
  3. Status berubah menjadi Backlog (warna kiri kolom berubah abu-abu).
- **Komponen UI:** Dropdown status berwarna dengan checkbox, panel detail, kartu approval, feed aktivitas.
- **Field/data:** Status workflow: Backlog, In Progress, Ready for Design, Completed, On Hold, Cancelled.
- **Catatan:**
  - Tersedia tautan "Edit workflow" untuk mengubah daftar status.
  - Checkbox di samping status dapat dipakai untuk menandai selesai.

### Setting project dates
*4 screen — [`screens/Setting project dates/`](screens/Setting%20project%20dates/)*
- **Lokasi/entry point:** Panel detail project (klik project di Gantt/Table) > field "Date" ("Set date").
- **Langkah:**
  1. Buka panel detail project "Feature 2.0"; klik "Set date" di field Date.
  2. Date picker muncul (Start date, Due date, kalender Mei-Juni 2024).
  3. Pilih 20 Mei lalu 20 Juni (rentang tersorot).
  4. Klik "Save"; Date menampilkan "20 May - 20 Jun" dan log aktivitas mencatat "Scheduled project for 20 May - 20 Jun".
- **Komponen UI:** Panel detail samping, popover date picker dua bulan, input tanggal, tombol Save/Cancel/Clear dates, activity log.
- **Field/data:** Start date, Due date (dd/mm/yyyy), field Date pada panel (Status, Assignee, Date), jumlah subitems, riwayat aktivitas.
- **Catatan:**
  - Perubahan tanggal otomatis tercatat di riwayat aktivitas project.

### Adding a project location
*3 screen — [`screens/Adding a project location/`](screens/Adding%20a%20project%20location/)*
- **Lokasi/entry point:** Panel detail project "Feature 2.0", chip lokasi (mis. "Creative Team") dengan tombol "+" di bawah judul.
- **Langkah:**
  1. Buka panel detail project; lokasi saat ini "Creative Team".
  2. Klik "+" di samping chip lokasi.
  3. Dropdown "Search project, folder, or space" muncul, menampilkan "Personal (Space)".
  4. Pilih "Personal"; lokasi bertambah menjadi dua chip: "Creative Team" dan "Personal".
- **Komponen UI:** Chip lokasi, popover pencarian lokasi, panel detail dengan approval, file, dan feed aktivitas.
- **Field/data:** Lokasi (project, folder, space); satu item dapat berada di banyak lokasi.
- **Catatan:**
  - Item dapat dikaitkan ke beberapa space sekaligus tanpa duplikasi.

### Sharing a project
*6 screen — [`screens/Sharing a project/`](screens/Sharing%20a%20project/)*
- **Lokasi/entry point:** Halaman project (Gantt) > tombol "Share" di pojok kanan atas.
- **Langkah:**
  1. Klik "Share"; modal "Share project Feature 2.0" terbuka dengan daftar "Shared with" (My Team, Jane Smith).
  2. Tambah orang lewat nama/email; muncul penerima baru (mis. email eksternal berlabel "external") dengan tombol Save/Cancel.
  3. Klik "Advanced"; panel samping menampilkan toggle "Selective sharing" (mati: mewarisi dari parent).
  4. Aktifkan "Selective sharing" (tidak mewarisi sharing parent; judul menjadi "Shared with (selective)").
  5. Klik dropdown peran pada My Team; pilih "Editor".
  6. Klik "Save".
- **Komponen UI:** Modal berbagi, input "Add people by name or email", daftar penerima dengan dropdown peran, toggle Selective sharing, tombol Copy link / Advanced / Close advanced / Save / Cancel, dropdown "Access role".
- **Field/data:** Penerima: My Team (2 users), Jane Smith, Sam Lee, email eksternal (ikon info). Peran: Full (Can edit), Editor (Can edit, but can't share or delete), Limited (Can comment, change statuses, attach files, and start approvals), Read Only (Can view), tautan "See all permissions".
- **Catatan:**
  - Penerima eksternal diberi label dan ikon info; pengguna pemilik tidak punya dropdown peran.
  - Selective sharing memutus pewarisan izin dari space/folder induk (dijelaskan dengan teks "Learn more").

### Approving a project
*5 screen — [`screens/Approving a project/`](screens/Approving%20a%20project/)*
- **Lokasi/entry point:** Panel detail project "Feature 2.0", kartu "A project needs approval" dengan tombol Approve/Reject.
- **Langkah:**
  1. Lihat kartu approval (0/2) dengan tombol Approve dan Reject.
  2. Klik "Approve"; popover "You gave a decision: Approved" menampilkan kolom "Add comment to your decision".
  3. Ketik komentar ("Project approved, good to go for research. Design timeline extended.").
  4. Klik "Send comment" (atau "Don't send").
  5. Kartu berubah menjadi "1/2" dengan "Your decision: Approved" dan ikon undo; badge inbox bertambah.
  6. Kartu dapat diperluas: approvers (JS, SL), due date 3 Jun, Files, Advanced settings, "Cancel approval".
- **Komponen UI:** Kartu approval, tombol keputusan, popover komentar, tautan undo keputusan, avatar approver, indikator pecahan (1/2).
- **Field/data:** Keputusan (Approve/Reject), komentar opsional, due date, approvers, files.
- **Catatan:**
  - Komentar keputusan bersifat opsional.
  - Keputusan dapat dibatalkan lewat ikon undo.
  - Aktivitas mencatat "Gave a decision: Approved".

### Adding an approval
*6 screen — [`screens/Adding an approval/`](screens/Adding%20an%20approval/)*
- **Lokasi/entry point:** Panel detail project "Feature 2.0", tombol "Add approval" di baris aksi.
- **Langkah:**
  1. Klik "Add approval" sehingga form "Start a new approval" muncul.
  2. Isi deskripsi ("Research and design approval").
  3. Klik "Add approvers"; dropdown "Search people" menampilkan Jane Smith dan Sam Lee.
  4. Pilih approver (JS dan SL) dan opsional "Set due date", "Add files".
  5. Klik "Start".
  6. Kartu "A project needs approval 0/2" muncul dengan tombol Approve / Reject.
- **Komponen UI:** Form inline dengan border biru, dropdown pencarian orang, avatar approver, tombol Start/Cancel, tautan "Advanced settings", kartu status approval.
- **Field/data:** Description, Approvers, Due date, Files, Advanced settings; penghitung approval (0/2).
- **Catatan:**
  - Tombol "Start" aktif setelah form diisi; setelah mulai, aktivitas mencatat pembuatan approval.

### Creating a blueprint
*5 screen — [`screens/Creating a blueprint/`](screens/Creating%20a%20blueprint/)*
- **Lokasi/entry point:** Settings space "Creative Team" > tab "Blueprints" (tab lain: General, Custom item types, Custom fields, Workflows, Automation, Request forms).
- **Langkah:**
  1. Buka tab Blueprints; empty state "Create blueprints for routine work" dengan tombol "Create blueprint".
  2. Klik tombol; menu "Select an item to create": pencarian item, Task, Project, Folder, "Customize this menu".
  3. Pilih Task; modal "Add item" muncul (Save to: Creative Team Blueprints).
  4. Isi Name ("Feature 2.0 Research") dan tipe (Task); klik "Add and open".
  5. Panel detail terbuka dengan banner "This item is a Blueprint": status Backlog, assignee Empty, Date, "Create items to: Add to project, folder, or space", field Importance, Author, Created date, Item type, ID, Time spent.
  6. Blueprint muncul di tabel (Table/Board/Gantt Chart) sebagai baris "Feature 2.0 Research".
- **Komponen UI:** Empty state dengan ilustrasi, menu pilihan tipe item, modal "Add item", tombol "Add and open", panel detail slide-over dengan banner biru, tabel blueprint, sidebar daftar "Blueprints" ("There's nothing here").
- **Field/data:** Save to, Name, tipe item (Task/Project/Folder), status, assignee, date, importance, "Create items to".
- **Catatan:**
  - Blueprint dipakai untuk membuat item berulang dengan cepat.
  - Banner biru jelas menandai item sebagai blueprint, bukan item biasa.

### Customizing blueprints menu
*8 screen — [`screens/Customizing blueprints menu/`](screens/Customizing%20blueprints%20menu/)*
- **Lokasi/entry point:** Settings (Creative Team) → tab "Blueprints" → tombol "Create blueprint" → menu "Select an item to create" → "Customize this menu".
- **Langkah:**
  1. Pada tab Blueprints (kosong, ada tombol "Create blueprint" dan link "Learn more about space blueprints"), klik tombol untuk membuka menu item.
  2. Klik "Customize this menu" → daftar toggle (Task, Project, Folder) → "Create new type".
  3. Modal "Create a custom item type": pilih Project type, isi Name ("New Product Development").
  4. Opsional "Change icon": pemilih ikon dengan pencarian dan kategori.
  5. Klik Create; item baru tampil (toggle aktif) dan muncul di menu "Select an item to create".
- **Komponen UI:** Halaman Settings bertab, empty state dengan CTA, popover menu dengan toggle, modal, radio tipe, pemilih ikon (dropdown "Suggested", kotak pencarian, grid ikon), tombol Create/Cancel.
- **Field/data:** Tipe item (Task type / Project type), Name, Icon (kategori Suggested, Accessibility).
- **Catatan:**
  - Tombol Create nonaktif sampai nama diisi.
  - Item baru langsung tersedia sebagai opsi saat membuat blueprint.

### Exporting project to excel
*4 screen — [`screens/Exporting project to excel/`](screens/Exporting%20project%20to%20excel/)*
- **Lokasi/entry point:** Tampilan Board (juga di My to-do) space/proyek Creative Team; menu "..." di toolbar.
- **Langkah:**
  1. Buka menu "..." di toolbar Board.
  2. Pilih "Export project to Excel" (menu juga berisi "Tasks from subfolders" dan "Import").
  3. Dialog "Export to Excel" menjelaskan: semua tugas, proyek bersarang, folder, dan seluruh field akan diekspor.
  4. Klik "Export".
  5. Toast: "Exporting the file. Please do not close this browser window, it might take some time to complete."
  6. Toast selesai: "Exporting Excel completed. Click to download: Creative Team (export).xlsx".
- **Komponen UI:** Menu dropdown, modal konfirmasi, toast progres, toast sukses dengan tautan unduh.
- **Field/data:** Menu "Show on board": Tasks from subfolders (centang), Export project to Excel, Import (submenu); nama file hasil ekspor.
- **Catatan:**
  - Pengguna diminta tidak menutup jendela browser selama ekspor.
  - File diunduh dengan mengklik tautan pada toast.

### Creating a public snapshot
*3 screen — [`screens/Creating a public snapshot/`](screens/Creating%20a%20public%20snapshot/)*
- **Lokasi/entry point:** Tampilan Gantt Chart pada space (Creative Team), toolbar → "Public snapshots".
- **Langkah:**
  1. Klik "Public snapshots"; panel samping terbuka dengan penjelasan snapshot (link "Learn more", "Got it").
  2. Pilih "From current state" (alternatif "From project history") di bagian "Create a public snapshot".
  3. Panel "Create a snapshot": isi nama (Creative Team), centang "Share with public link", opsional "Show assignee names", URL tampil dengan ikon copy.
  4. Klik Create; snapshot muncul di daftar "Past snapshots" (22 May, Creative Team) dengan ikon salin.
- **Komponen UI:** Panel samping kanan, banner informasi, daftar aksi, form kecil dengan checkbox, input URL, tombol Create/Cancel, daftar riwayat.
- **Field/data:** Nama snapshot, Share with public link (dicentang), Show assignee names (tidak dicentang), URL publik, daftar Past snapshots (tanggal, nama). Data Gantt: Name, Status, Start date, Due date, Impact.
- **Catatan:**
  - Snapshot = jadwal proyek pada hari tertentu; dapat dibagikan atau dipakai sebagai baseline pembanding.

### Copying a link
*2 screen — [`screens/Copying a link/`](screens/Copying%20a%20link/)*
- **Lokasi/entry point:** Toolbar tampilan, dropdown "All" (daftar view) > menu "..." pada view (mis. "Gantt Chart").
- **Langkah:**
  1. Buka dropdown view: Always visible (Table, Board, Gantt Chart) dan Hidden (Table, Timelog).
  2. Klik "..." di baris "Gantt Chart"; menu: This is the default view (nonaktif), Copy link to this view, Rename, Duplicate, Move up, Move down, Delete.
  3. Pilih "Copy link to this view".
  4. Toast muncul: "Link to Gantt Chart copied to clipboard" dengan tombol tutup.
- **Komponen UI:** Dropdown daftar view, menu konteks, toast notifikasi di atas halaman, ikon pin dan drag handle.
- **Field/data:** Opsi: Copy link to this view, Rename, Duplicate, Move up, Move down, Delete; "Add view".
- **Catatan:**
  - Opsi "This is the default view" abu-abu karena view sudah default.
  - Konfirmasi berupa toast, tanpa dialog.


---

## 4. Views (Table/Board/Gantt/Calendar/Timelog)

### Board view
*2 screen — [`screens/Board view/`](screens/Board%20view/)*
- **Lokasi/entry point:** Space "Creative Team", tab "Board" pada toolbar tampilan (Table / Board / Gantt Chart).
- **Langkah:**
  1. Buka tab Board; kolom status ditampilkan berdampingan (New 2, Planned 1, In progress 0, In review 1, ...).
  2. Geser/scroll horizontal untuk melihat kolom lain (Ready for review 0, Completed 1) dan tombol "+ Status".
  3. Lihat kartu "Research" di kolom Completed dengan gambar sampul.
- **Komponen UI:** Kolom Kanban berwarna per status, kartu (ikon tipe, judul, avatar, rentang tanggal, jumlah subitem), toolbar (All tasks, Priority, Columns, Group, Cards), tombol "+ Status".
- **Field/data:** Status kolom: New, Planned, In progress, In review, Ready for review, Completed; kartu menampilkan nama, assignee, tanggal, durasi, jumlah subitem.
- **Catatan:**
  - Jumlah item ditampilkan di samping nama kolom.
  - Kartu dapat menampilkan sampul gambar (mis. dari lampiran).

### Gantt chart view
*2 screen — [`screens/Gantt chart view/`](screens/Gantt%20chart%20view/)*
- **Lokasi/entry point:** Space Creative Team (Space overview) > tab "Gantt Chart" di sebelah Table dan Board.
- **Langkah:**
  1. Buka tab "Gantt Chart": linimasa mingguan dengan bar tugas, dependensi, dan garis merah "hari ini".
  2. Gunakan zoom (dropdown "Weeks" dan tombol +/-) untuk skala waktu.
  3. Buka panel tabel di sisi kiri (handle pembuka panel) untuk melihat kolom Name, Status, Start date, Due date, Impact berdampingan dengan bar.
- **Komponen UI:** Bar Gantt berwarna per status, panah dependensi, header minggu/tanggal, pemilih zoom, toolbar (All tasks, Due date, Expand, Public snapshots, undo/redo), tombol Automation/Share, tabel hierarki yang dapat dibuka/ditutup.
- **Field/data:** Status (New, In Progress, Completed, In review, Ready for..., Planned), Start/Due date, Impact (Low, Medium, High); proyek/ folder dengan persentase progres ("New Product ... 45%").
- **Catatan:**
  - Sidebar dapat diciutkan; ikon notifikasi muncul di pojok saat sidebar tertutup.
  - Item tanpa tanggal tampil sebagai garis putus-putus di bawah induk.

### Timelog view
*3 screen — [`screens/Timelog view/`](screens/Timelog%20view/)*
- **Lokasi/entry point:** Space/project > dropdown "All" di samping tab view > bagian "Hidden" > "Timelog".
- **Langkah:**
  1. Klik "All"; pada daftar view yang tersembunyi pilih "Timelog".
  2. Tab "Timelog" muncul dan terbuka: tabel waktu tercatat dikelompokkan per User lalu Date.
  3. Gulir horizontal untuk melihat kolom tambahan (Category, Comment, Actual fees, Actual cost, Billing type, Lock).
- **Komponen UI:** Dropdown view, tab Timelog, toolbar (Filters, Group by User/Date +, Expand, Collapse, Lock or unlock dates, Export), tabel dengan baris grup, baris total.
- **Field/data:** Kolom: Task name, User, Date, Time spent (SUM 15h 15m), Project or folder, Category, Comment, Actual fees ($), Actual cost ($), Billing type (Non-billable), Lock. Baris contoh: Jane Smith > 21/05/2024 > "1369667897: Marketing request"; komentar "Team ideation"; "Total: 1 entry".
- **Catatan:**
  - View Timelog disembunyikan secara default dan perlu diaktifkan dari daftar view.
  - Tersedia Export dan Lock tanggal untuk mengunci entri waktu.

### Adding a calendar view
*5 screen — [`screens/Adding a calendar view/`](screens/Adding%20a%20calendar%20view/)*
- **Lokasi/entry point:** Toolbar tampilan di space (tab Table / Board / Gantt Chart), dropdown "All" lalu "Add view", atau tombol "+ View".
- **Langkah:**
  1. Buka dropdown "All" (Always visible: Table, Board, Gantt Chart; Hidden: Table, Timelog; "+ Add view").
  2. Klik Add view/"+ View" lalu menu pilihan tampilan muncul.
  3. Pilih "Calendar" dan isi Name ("Calendar").
  4. Pilih kebagikan: "Anyone with access (Public)" atau "Only me (Personal)", klik Create.
  5. Tampilan Calendar bulanan terbuka (Mei 2024) dengan batang tugas per hari.
  6. Navigasi ke bulan Juni; hover pada batang "Feature 2.0" menampilkan popover (status In Progress, lokasi, avatar, rentang tanggal 20 May - 20 Jun).
- **Komponen UI:** Dropdown daftar view, menu galeri view, popover pembuatan (radio button), kalender grid bulanan dengan batang berwarna per status, popover hover kartu, tombol "Next month".
- **Field/data:** Custom views: Table, Board, Gantt Chart, Files, Chart, Calendar, Stream; Classic views: Resources, "More classic views". Kontrol kalender: Today, panah, pemilih bulan, Month, Filters, Public links.
- **Catatan:**
  - Tiap view punya deskripsi singkat (mis. "See scheduled work on a calendar").
  - Item multi-hari ditampilkan sebagai batang yang melintasi hari, warna mengikuti status.

### Setting a default view
*3 screen — [`screens/Setting a default view/`](screens/Setting%20a%20default%20view/)*
- **Lokasi/entry point:** Halaman space/project > dropdown "All" di samping tab view > "..." pada view tertentu.
- **Langkah:**
  1. Klik "All" untuk membuka daftar view (Always visible / Hidden / + Add view); hover view "Gantt Chart" dan klik "...".
  2. Pilih "Set view as default" dari menu.
  3. Buka kembali menu: item teratas kini berbunyi "This is the default view" (nonaktif) untuk view tersebut.
- **Komponen UI:** Dropdown daftar view, context menu, ikon pin, indikator default (teks abu-abu dengan ikon centang).
- **Field/data:** Menu: Set view as default / This is the default view, Copy link to this view, Rename, Duplicate, Move up, Move down, Delete. View Always visible: Table, Board, Gantt Chart; Hidden: Table, Timelog.
- **Catatan:**
  - Opsi "Set view as default" berubah menjadi label status nonaktif setelah dipilih.

### Duplicating a view
*5 screen — [`screens/Duplicating a view/`](screens/Duplicating%20a%20view/)*
- **Lokasi/entry point:** Proyek (Feature 2.0) → tab view → dropdown "All" → ikon "..." di samping view (mis. Gantt chart).
- **Langkah:**
  1. Buka dropdown view; daftar "Always visible" (Table, Board, Gantt chart) dan "Hidden" (Files).
  2. Klik "..." pada Gantt chart; menu: This is the default view, Copy link to this view, Rename, Duplicate, Move up, Move down, Delete.
  3. Pilih Duplicate; modal `Duplicate "Gantt"` terbuka: isi View name ("Gantt Sample").
  4. Pilih visibilitas Public atau Personal; klik Create.
  5. View baru "Gantt Sample" muncul di tab dan di daftar "Hidden" (bersama Files, Analytics, Calendar, opsi "Add view").
- **Komponen UI:** Dropdown daftar view dengan drag handle dan ikon pin, menu konteks, modal, input teks, radio, tombol Create/Cancel.
- **Field/data:** View name; visibilitas: Public ("All users in this project can see this") / Personal ("Only you can see this and it can't be shared").
- **Catatan:**
  - Opsi "This is the default view" tampil abu-abu (nonaktif).
  - Default pilihan Public.

### Renaming a view
*4 screen — [`screens/Renaming a view/`](screens/Renaming%20a%20view/)*
- **Lokasi/entry point:** Halaman project > dropdown "All" di sebelah tab view > ikon "..." pada view yang dipilih.
- **Langkah:**
  1. Klik dropdown "All" untuk melihat daftar view; hover view "Gantt chart" lalu klik "...".
  2. Pilih "Rename".
  3. Modal "Rename view" muncul berisi nama saat ini; ubah menjadi "Gantt".
  4. Klik "Rename"; tab dan daftar view menampilkan nama baru.
- **Komponen UI:** Dropdown daftar view (Always visible / Hidden), context menu, modal "Rename view" dengan input + tombol Rename/Cancel/X, tab view.
- **Field/data:** Menu view: "This is the default view" (nonaktif), Copy link to this view, Rename, Duplicate, Move up, Move down, Delete. View Always visible: Table, Board, Gantt chart; Hidden: Files, Analytics, Calendar; opsi "+ Add view". Input: nama view.
- **Catatan:**
  - Latar halaman diredupkan saat modal terbuka.
  - View dapat diurutkan (Move up/down) dan disembunyikan; ada drag handle pada baris.

### Sorting by group
*5 screen — [`screens/Sorting by group/`](screens/Sorting%20by%20group/)*
- **Lokasi/entry point:** Space "Creative Team" > view Board > tombol "Group" di toolbar.
- **Langkah:**
  1. Klik "Group"; dropdown "Group by" tampil (None ✓, Assignee).
  2. Pilih "Assignee"; board terbagi menjadi swimlane per assignee (Unassigned 0, Jane Smith 5, Sam Lee 3).
  3. Gulir vertikal untuk melihat swimlane berikutnya; gulir horizontal untuk melihat kolom lain (Ready for review, Cancelled, Completed) dengan kartu bergambar thumbnail.
- **Komponen UI:** Dropdown "Group by", swimlane yang dapat di-collapse (chevron), header kolom status sticky, kartu Kanban dengan thumbnail/proofing, tombol "+ Status".
- **Field/data:** Opsi grup: None, Assignee. Kolom status: New, Planned, In progress, In review, Ready for review, Cancelled, Completed. Header swimlane: avatar, nama, jumlah item.
- **Catatan:**
  - Header kolom status tetap terlihat saat gulir; swimlane kosong ("Unassigned 0") tetap ditampilkan.

### Editing card preview
*3 screen — [`screens/Editing card preview/`](screens/Editing%20card%20preview/)*
- **Lokasi/entry point:** Tampilan Board space Creative Team; tombol "Cards" di toolbar.
- **Langkah:**
  1. Buka tampilan Board (kolom New, Planned, In progress, In review).
  2. Klik "Cards" di toolbar; popover "Choose which fields to show in card previews" terbuka.
  3. Aktifkan toggle field (mis. Author, Item type; Cover image dan Start date sudah aktif).
  4. Kartu di board langsung menampilkan baris "Item type: Task" dan "Author: Jane Smith".
- **Komponen UI:** Popover dengan pencarian, toggle switch, kelompok field (General, Schedule), kartu board dengan avatar dan sub-item, tautan "+ Custom field".
- **Field/data:** Search fields; General: Cover image, Days in status, Location, Author, ID, Item type; Schedule: Start date, Created date, dst. (daftar bisa di-scroll); "+ Custom field".
- **Catatan:**
  - Perubahan diterapkan seketika tanpa tombol simpan.
  - Pencarian field tersedia karena daftar panjang.

### Filtering custom fields
*3 screen — [`screens/Filtering custom fields/`](screens/Filtering%20custom%20fields/)*
- **Lokasi/entry point:** Tampilan Gantt chart proyek Feature 2.0; ikon gear di header kolom tabel (pemilih kolom).
- **Langkah:**
  1. Klik ikon gear di pojok kiri atas tabel Gantt; panel pemilih kolom terbuka.
  2. Centang/hapus centang kolom yang ingin ditampilkan, termasuk custom field (Budget, Impact).
  3. Centang "Assignee" sehingga kolom Assignee ditambahkan.
  4. Tutup panel; tabel menampilkan kolom terpilih (terlihat Name, Status, Assignee, Start date).
- **Komponen UI:** Popover berisi checkbox dikelompokkan, tabel Gantt dengan kolom dinamis, tautan "Manage custom fields".
- **Field/data:** Management (Assignee, Importance, Predecessors, Status, Status group, Progress); Schedule (Created date, Start date, Due date, Completed date, Duration, Effort, Time spent, Start constraint, Booked effort); Other (Author, ID, Item type); custom fields (Budget, Impact); baselines (Baseline start, Baseline due).
- **Catatan:**
  - Flow ini sebenarnya berupa pengaturan kolom yang terlihat, bukan filter nilai.
  - Perubahan kolom berlaku langsung saat checkbox diubah.


---

## 5. Task & Item

### Adding a task
*5 screen — [`screens/Adding a task/`](screens/Adding%20a%20task/)*
- **Lokasi/entry point:** Halaman "My to-do" (sidebar > Inbox/More area), baris "+ Item" di atas daftar.
- **Langkah:**
  1. Buka "My to-do" (daftar dikelompokkan per status: Planned, In review, Not started, Completed).
  2. Klik "+ Item"; input "Enter name or permalink" dengan pemilih tipe (Task) muncul.
  3. Ketik "Plan product weekly meeting" lalu Enter.
  4. Tugas muncul di grup baru "BACKLOG (1)".
  5. Klik tugas untuk membuka panel detail: status Backlog, assignee Jane Smith, Date "Set date", "+ Location".
- **Komponen UI:** Daftar tugas berkelompok, input inline, panel detail slide-over, aksi (Add subitem, Add files, Add approval, Add dependency, timer 0:00), kolom komentar.
- **Field/data:** Name, tipe, status (Backlog), assignee (Jane Smith), date, location, deskripsi; kelompok status berasal dari workflow (Task workflow, Campaign Workflow).
- **Catatan:**
  - Tugas baru otomatis ditugaskan ke pembuat dan berstatus Backlog.
  - Toolbar My to-do: All tasks (filter), Status (sort/group), tampilan, menu "...".

### Adding a new item (board)
*6 screen — [`screens/Adding a new item (board)/`](screens/Adding%20a%20new%20item%20%28board%29/)*
- **Lokasi/entry point:** Tampilan Board pada space "Creative Team", tombol "+" di header kolom status (mis. "In progress") atau "+ Item" saat hover.
- **Langkah:**
  1. Buka Board (kolom New, Planned, In progress, In review, dst).
  2. Hover pada kolom "In progress" lalu klik "+".
  3. Kartu input muncul dengan placeholder "Enter name" dan pemilih tipe item (Task).
  4. Ketik nama ("Resource allocation") dan tekan Enter.
  5. Klik avatar untuk memilih assignee: pencarian "Search assignees", daftar anggota (Sam Lee, Jane Smith, email undangan) dan "Invite by email".
  6. Kartu "Resource allocation" muncul di kolom In progress (hitungan menjadi 1) dengan avatar.
- **Komponen UI:** Kolom board berwarna per status, kartu inline-create, dropdown tipe item, dropdown assignee dengan pencarian, avatar.
- **Field/data:** Nama, tipe item (Task), assignee; kartu menampilkan avatar, rentang tanggal, jumlah subitem.
- **Catatan:**
  - Item baru otomatis mengikuti status kolom tempat dibuat.
  - Toolbar board: All tasks, Priority, Columns, Group, Cards.

### Adding a new item (gantt)
*11 screen — [`screens/Adding a new item (gantt)/`](screens/Adding%20a%20new%20item%20%28gantt%29/)*
- **Lokasi/entry point:** Tampilan Gantt Chart pada space "Creative Team", baris "+ Item" di bawah tabel.
- **Langkah:**
  1. Klik "+ Item" di akhir daftar.
  2. Baris input muncul dengan placeholder "Enter name or permalink" dan pemilih tipe (Task); ketik nama ("Development").
  3. Tekan Enter; item muncul di daftar (status New). Item kedua "Sprint Planning" ditambahkan.
  4. Klik kanan baris: menu "Add subitem", "Schedule for today", "Indent", "Delete"; gunakan "Indent" untuk menjadikan Sprint Planning subitem Development.
  5. Klik sel Start date: date picker bulan (Mei 2024) muncul; pilih tanggal (23/05 lalu 25/06/2024).
  6. Atur Due date (31/07/2024).
  7. Klik sel Impact: dropdown "Search items" dengan Low, Medium, High; pilih High.
- **Komponen UI:** Tabel Gantt dengan kolom (Name, Status, Start date, Due date, Impact), input inline, context menu, date picker bulanan, dropdown custom field, bar Gantt di panel kanan.
- **Field/data:** Name, tipe item, Status (New), Start date, Due date, Impact (Low/Medium/High).
- **Catatan:**
  - Item bisa dibuat dengan nama atau permalink.
  - Timeline Gantt menyesuaikan setelah tanggal diisi; tombol undo/redo tersedia di toolbar.

### Adding a subitem
*5 screen — [`screens/Adding a subitem/`](screens/Adding%20a%20subitem/)*
- **Lokasi/entry point:** Tampilan Board; kartu folder "Initial Design" (hover memunculkan "Add subitem" di dekat jumlah subitem).
- **Langkah:**
  1. Buka Board di kolom Planned.
  2. Hover kartu "Initial Design": muncul menu "..." dan tautan "+ Add subitem".
  3. Klik "Add subitem"; baris input "Enter name" dengan pemilih tipe (Task) tampil di dalam kartu.
  4. Ketik nama ("Product alignment") dan tekan Enter.
  5. Subitem baru muncul dalam kartu (status New), jumlah subitem bertambah dari 2 menjadi 3.
- **Komponen UI:** Kartu board yang dapat memuat subitem, input inline, dropdown tipe item, daftar subitem (avatar, nama, status, "Set date").
- **Field/data:** Nama subitem, tipe item, status (New), tanggal ("Set date"), assignee.
- **Catatan:**
  - Subitem baru tanpa assignee ditandai ikon avatar kosong (placeholder).

### Editing an item
*7 screen — [`screens/Editing an item/`](screens/Editing%20an%20item/)*
- **Lokasi/entry point:** Tampilan Gantt chart ruang/proyek (Creative Team); klik bar tugas (mis. "Research").
- **Langkah:**
  1. Klik bar tugas di Gantt sehingga kartu kecil muncul (status Completed, nama, avatar, rentang tanggal 16-22 May).
  2. Buka menu "..." di kartu dan pilih "Enable effort".
  3. Isi panel Effort mode: Basic (Duration 5d, Allocation, Effort 0h menjadi 10h).
  4. Menu "..." kini menawarkan "Disable effort", "Convert to milestone", "Delete task".
  5. Pilih "Convert to milestone": bar berubah menjadi penanda milestone, kartu menampilkan satu tanggal (22 May) dan progres proyek ikut berubah (25% menjadi 16%).
- **Komponen UI:** Kartu popover tugas pada bar Gantt, menu konteks, panel Effort mode (dropdown Basic), bar Gantt dengan handle, garis dependensi.
- **Field/data:** Duration, Allocation (placeholder 2h/d), Effort, status tugas, assignee, tanggal; opsi menu: Enable/Disable effort, Convert to milestone, Delete task.
- **Catatan:**
  - Allocation tampak abu-abu (otomatis) saat effort diatur dengan mode Basic.
  - Rentang tampil sebagai "(5d/10h)" setelah effort diisi.

### Changing a status
*7 screen — [`screens/Changing a status/`](screens/Changing%20a%20status/)*
- **Lokasi/entry point:** Tampilan Board; menu "..." pada kartu atau drag-and-drop antar kolom.
- **Langkah:**
  1. Hover kartu "1369667897: Marketing request"; ikon "..." (More) muncul.
  2. Klik "..." untuk menu: Open in new tab, Copy link, Add subitem, Delete, Change status >, Importance >.
  3. Pilih "Change status": submenu "Task workflow (Creative Team space)" berisi New, Planned, In progress, In review, Ready for review, Completed, "Edit workflow", serta "Other workflows" (Default Workflow, New workflow).
  4. Pilih "Planned"; kartu pindah ke kolom Planned (New 1, Planned 2).
  5. Cara alternatif: seret kartu "Design review" dari kolom New ke Planned (kartu asli memudar, garis biru penanda posisi drop).
  6. Hasil: New 0, Planned 3.
- **Komponen UI:** Menu konteks bertingkat, submenu status berwarna, drag-and-drop dengan drop indicator, penghitung kolom.
- **Field/data:** Status workflow: New, Planned, In progress, In review, Ready for review, Completed; workflow lain: Default Workflow, New workflow.
- **Catatan:**
  - Perubahan status dilakukan lewat menu atau drag-and-drop dan langsung memperbarui hitungan kolom.
  - Menu menyertakan jalan pintas "Edit workflow".

### Setting a date
*7 screen — [`screens/Setting a date/`](screens/Setting%20a%20date/)*
- **Lokasi/entry point:** Tampilan Board pada Space "Creative Team": kartu tugas tanpa tanggal menampilkan tautan "Set date" saat di-hover.
- **Langkah:**
  1. Hover kartu "Resource allocation"; klik "Set date".
  2. Date picker muncul (input Start date, Due date, Duration + kalender 2 bulan).
  3. Pilih tanggal 23 Mei lalu 24 Mei (rentang tersorot, Duration otomatis "2d").
  4. Opsional: aktifkan toggle "Effort" sehingga muncul panel Allocation/Effort (Specify effort to view allocation).
  5. Isi effort (2h); panel menampilkan alokasi "Jane Smith 2 days x 1h/d".
  6. Klik "Save"; kartu menampilkan "23 - 24 May (2d)".
- **Komponen UI:** Popover date picker dua bulan, input tanggal (dd/mm/yyyy), field durasi, checkbox "Working days only", toggle Effort, panel alokasi, tombol Save/Cancel/Clear dates.
- **Field/data:** Start date, Due date, Duration, Working days only (default tercentang), Effort (jam), Allocation, Effort mode: Basic; kartu menampilkan rentang tanggal dan durasi.
- **Catatan:**
  - Duration dihitung otomatis dari rentang tanggal; hari non-kerja tampak redup.
  - Tersedia "Clear dates" untuk menghapus tanggal.

### Adding an assignee
*5 screen — [`screens/Adding an assignee/`](screens/Adding%20an%20assignee/)*
- **Lokasi/entry point:** Panel detail project "Feature 2.0", kolom "Assignee" pada baris Status/Assignee/Date.
- **Langkah:**
  1. Klik kolom Assignee (saat ini Jane Smith).
  2. Field berubah menjadi input dengan chip "Jane Smith x" dan "Add more people"; dropdown mencantumkan Sam Lee, beberapa email undangan, dan "Invite by email".
  3. Pilih Sam Lee; chip kedua muncul dan daftar saran menyusut.
  4. Klik di luar; Assignee menampilkan dua avatar (Jane S., Sa...).
  5. Hover avatar Sam Lee menampilkan kartu profil (Designer, Product, email).
- **Komponen UI:** Multi-select chip dengan pencarian, dropdown daftar pengguna, "Invite by email", kartu profil hover, avatar berinisial.
- **Field/data:** Assignee (multi), nama, email, jabatan/departemen.
- **Catatan:**
  - Mendukung banyak assignee; email yang belum bergabung muncul sebagai pengguna undangan.

### Adding a description
*3 screen — [`screens/Adding a description/`](screens/Adding%20a%20description/)*
- **Lokasi/entry point:** Panel detail item "Feature 2.0" (Gantt chart), area "Add a description".
- **Langkah:**
  1. Buka panel detail item; bagian deskripsi menampilkan "Add a description" dan item di sidebar Quick start 40%.
  2. Klik "Add a description" untuk memunculkan toolbar editor rich text.
  3. Ketik deskripsi ("Research section is to be completed in order of list. Revisions can happen in parallel.").
  4. Deskripsi tersimpan dan terlihat di panel; aktivitas menampilkan log perubahan.
- **Komponen UI:** Panel slide-over, field kustom (Progress 28%, Author, Created date, Item type, ID, Booked effort, Time spent, More fields: Budget $8,000, Impact), editor rich text (heading, bold, italic, underline, strikethrough, warna, checklist, daftar, indent, link, gambar, tabel, undo/redo), feed aktivitas.
- **Field/data:** Status (In Progress), Assignee (Jane Smith), Date (20 May - 20 Jun), deskripsi, subitems (3), Add files, Add approval.
- **Catatan:**
  - Placeholder editor "Add a description" berupa teks abu-abu.

### Adding a comment
*6 screen — [`screens/Adding a comment/`](screens/Adding%20a%20comment/)*
- **Lokasi/entry point:** Panel detail item (sisi kanan) "Feature 2.0", kolom komentar di bagian bawah.
- **Langkah:**
  1. Buka panel detail item; lihat aktivitas (approval, perubahan status, lampiran) dan kolom "Try uploading files and adding comments...".
  2. Ketik komentar ("meeting with engineering needed before design approval"); tombol Send aktif.
  3. Ketik "@" untuk menyebut orang; muncul daftar: My Team, Sam Lee (email), followers.
  4. Pilih "Sam Lee" sehingga mention "@Sam Lee" tersisip.
  5. Klik Send; komentar tampil sebagai bubble di feed dengan nama, jam (21:26), dan mention.
- **Komponen UI:** Panel detail slide-over, text composer dengan toolbar (lampiran, @, emoji, Aa), dropdown mention, feed aktivitas, tombol Send, tautan "Jump to top"/"Show more updates", ikon filter feed.
- **Field/data:** Teks komentar, mention pengguna/grup (My Team, followers = "Everyone who follows this folder/project"), lampiran.
- **Catatan:**
  - Send nonaktif sampai ada teks.
  - Feed mencampur komentar dan log aktivitas (rename, assign, status, keputusan approval).

### Adding a file
*3 screen — [`screens/Adding a file/`](screens/Adding%20a%20file/)*
- **Lokasi/entry point:** Panel detail item "Feature 2.0", tombol "Add files" di baris aksi (3 subitems / Add files / Add approval).
- **Langkah:**
  1. Buka panel detail item dan klik "Add files".
  2. Menu sumber muncul: Your computer, Google Drive, Dropbox, Box, OneDrive, SharePoint, Youtube link, Website link.
  3. Setelah file diunggah, penghitung berubah menjadi "1 file" dan bagian file menampilkan "[Meeting Notes] - Feature 2.0.pdf" dengan nama pengunggah (Jane Smith).
- **Komponen UI:** Dropdown sumber file, daftar file dengan thumbnail PDF dan checkbox, penghitung lampiran.
- **Field/data:** Sumber: komputer lokal, Google Drive (submenu), Dropbox, Box, OneDrive, SharePoint, link YouTube, link website.
- **Catatan:**
  - Menu mendukung penyimpanan cloud pihak ketiga dan tautan eksternal, bukan hanya unggah lokal.

### Setting progress
*6 screen — [`screens/Setting progress/`](screens/Setting%20progress/)*
- **Lokasi/entry point:** Panel detail project (klik nama project di Gantt) > baris field "Progress" ("Set progress").
- **Langkah:**
  1. Buka panel detail project "Feature 2.0"; gulir ke daftar field dan klik "Set progress".
  2. Popover "Progress" menawarkan "Measure as": Completed tasks, Duration of completed tasks, Tasks and duration average, Don't measure progress (terpilih default).
  3. Pilih metode (mis. Tasks and duration average).
  4. Field Progress menampilkan bar dan persentase (mis. 28%); popover dapat menampilkan "55% - 12 of 22 days", metode terpilih, serta kartu "Medium risk - Preliminary risk assessment" (prediksi AI).
- **Komponen UI:** Panel detail samping, popover radio-group, progress bar, badge risiko, tautan "Learn more" / "Leave feedback", progress persentase di bar Gantt.
- **Field/data:** Progress (metode ukur + persen), Author, Created date, Item type (Project), ID, Booked effort, Time spent, Budget ($8,000/$6,500), Impact, subitems, files, approvals; risiko (Medium risk).
- **Catatan:**
  - Metode default "Don't measure progress"; progress hanya muncul setelah metode dipilih.
  - Setelah diaktifkan, persentase juga tampil di bar project pada Gantt (mis. "28%", "55%").

### Adding impact
*4 screen — [`screens/Adding impact/`](screens/Adding%20impact/)*
- **Lokasi/entry point:** Panel detail project "Feature 2.0" > bagian field kustom, "More fields" > field "Impact" (sebelumnya Empty).
- **Langkah:**
  1. Buka panel detail dan perluas ruas field (Progress 55%, Author, Created date, Item type, ID, Booked effort, Time spent, More fields).
  2. Klik field "Impact" (Empty); muncul "Search options".
  3. Dropdown opsi: Low, Medium, High, Design, Product, Engineering (berwarna).
  4. Pilih "High"; field berubah menjadi multi-select dengan chip dan teks "Add more options".
  5. Tambahkan "Design"; tersimpan dan ditampilkan sebagai dua chip (High, Design).
- **Komponen UI:** Field kustom multi-select dengan chip berwarna, dropdown opsi dengan pencarian, panel detail dengan "Search fields".
- **Field/data:** Impact (multi-pilihan: Low, Medium, High, Design, Product, Engineering), Budget ($6,500), Progress (55%).
- **Catatan:**
  - Opsi yang sudah dipilih hilang dari daftar saran.

### Editing impact settings
*9 screen — [`screens/Editing impact settings/`](screens/Editing%20impact%20settings/)*
- **Lokasi/entry point:** Panel detail proyek (Feature 2.0) > "More fields" > field custom "Impact"; ikon "..." > "Change field settings".
- **Langkah:**
  1. Buka panel detail proyek; field Impact masih "Empty".
  2. Arahkan kursor ke Impact, klik "..." > "Change field settings".
  3. Modal menampilkan Field name "Impact", Type "Single select".
  4. Ubah Type menjadi "Multiple select".
  5. Klik "+ Option" dan isi opsi satu per satu: Low, Medium, High.
  6. Aktifkan toggle "Colors" lalu pilih warna tiap opsi lewat dropdown warna.
  7. Klik "Save".
- **Komponen UI:** Menu konteks, modal pengaturan field, dropdown tipe, daftar opsi dengan drag handle dan ikon hapus, toggle Colors, color dropdown.
- **Field/data:** Field name; Type; Options (Low, Medium, High) dengan warna opsional; Default aggregation (None, tampil pada tipe Single select); Belongs to Creative Team; Applies to tasks, projects, folders.
- **Catatan:**
  - Opsi dapat diurutkan ulang (drag handle) dan dihapus (ikon sampah).
  - Input opsi baru langsung fokus saat "+ Option" diklik.

### Editing a budget
*3 screen — [`screens/Editing a budget/`](screens/Editing%20a%20budget/)*
- **Lokasi/entry point:** Panel detail proyek (mis. Feature 2.0) dari tampilan Gantt, bagian "More fields" → Budget.
- **Langkah:**
  1. Buka panel detail proyek di sisi kanan; perluas "More fields".
  2. Bidang Budget menampilkan nilai saat ini ($8,000).
  3. Klik bidang, edit nilai secara inline (angka 8000 tampil dalam mode edit).
  4. Setelah disimpan, Budget menampilkan $6,500.
- **Komponen UI:** Panel detail slide-in, daftar field dengan edit inline, bagian "More fields" yang dapat dilipat, pencarian field, kolom komentar dengan tombol Send.
- **Field/data:** Status (In Progress), Assignee (Jane Smith), Date (20 May – 20 Jun), Progress (28%), Author, Created date, Item type (Project), ID, Booked effort, Time spent, Budget, Impact. Aksi: subitems, Add files, Add approval, Add a description.
- **Catatan:**
  - Edit dilakukan langsung pada field tanpa dialog terpisah.

### Editing budget settings
*14 screen — [`screens/Editing budget settings/`](screens/Editing%20budget%20settings/)*
- **Lokasi/entry point:** Panel detail proyek (Feature 2.0) > "More fields" > field custom "Budget"; ikon "..." di samping field.
- **Langkah:**
  1. Buka panel detail proyek dan perluas "More fields" (Budget $6,500, Impact kosong).
  2. Arahkan kursor ke Budget lalu klik "..." > "Change field settings" (alternatif: "Show editing history").
  3. Modal "Field name: Budget" tampil dengan Type "Number".
  4. Buka dropdown Type lalu pilih "Currency".
  5. Atur Symbol ($ US Dollar), Default aggregation (None/Sum/Average), Decimal places (2), dan centang "Use thousands separator".
  6. Centang "Show conditional marker" sehingga panel "Conditional marker" muncul; atur aturan (operator >, nilai 5000, warna Red/Amber/Green); "Add another rule" tersedia.
  7. Klik "Save"; nilai $6,500 kini diberi titik warna oranye.
- **Komponen UI:** Menu konteks, modal pengaturan field dua panel, dropdown tipe, dropdown agregasi, checkbox, input numerik, color picker dropdown, preview "Sample".
- **Field/data:** Field name; Type (Text, Number, Currency, Percent, Single select, Multiple select, People, Date, Duration, Checkbox, Formula); Symbol; Default aggregation; Decimal places + Sample ($5,000.00); Use thousands separator; Show conditional marker (operator, nilai, warna); Belongs to (Creative Team); Applies to tasks, projects, folders.
- **Catatan:**
  - Ada pesan peringatan: jika field dipakai di filter, formula, request form, atau automation, ubah item tersebut sesuai perubahan tipe.
  - Field "Belongs to" Creative Team bisa diubah (ikon pensil).

### Editing a timesheet
*8 screen — [`screens/Editing a timesheet/`](screens/Editing%20a%20timesheet/)*
- **Lokasi/entry point:** Menu "More" di sidebar > Timesheets; halaman Timesheets dengan rentang minggu 20-26 May.
- **Langkah:**
  1. Buka Timesheets (minggu 20-26 May, filter "Me" dan "Active tasks").
  2. Klik "+" / "Add task" di baris baru lalu cari tugas lewat kotak "Search tasks".
  3. Pilih tugas dari dropdown ("Plan product weekly meeting"); baris baru muncul.
  4. Klik sel hari (mis. Tu 21) lalu popover "Add entry" muncul.
  5. Isi durasi (mis. "1h") dan komentar ("gather insights to present to team").
  6. Sel hari menampilkan "1h" dan total berubah dari 15h 15m menjadi 16h 15m.
- **Komponen UI:** Grid timesheet (baris tugas x kolom hari), dropdown pencarian tugas dengan status, popover entry waktu (durasi + komentar + ikon hapus), navigasi minggu (panah), filter pengguna dan tugas.
- **Field/data:** Kolom Pin, Task, Project or folder, Mo 20 - Sa 25; field entri: durasi (format "0h 0m") dan Comment; status di hasil pencarian (New, Backlog, Completed); baris total ("Total: 8 tasks", jumlah jam).
- **Catatan:**
  - Segitiga biru kecil di sel menandai entri yang sudah ada.
  - Entri bisa dihapus lewat ikon tempat sampah; popover juga menyediakan "+ Add entry" untuk entri tambahan.


---

## 6. Workflow & Status

### Creating a workflow
*10 screen — [`screens/Creating a workflow/`](screens/Creating%20a%20workflow/)*
- **Lokasi/entry point:** Settings (Creative Team) → tab "Workflows" → "Create or add workflow".
- **Langkah:**
  1. Buka tab Workflows: bagian "For tasks" (Task workflow, Default Workflow) dan "For projects" (Default Workflow), keduanya berlabel Default dengan ikon gembok.
  2. Klik "Create or add workflow"; muncul dropdown pencarian ("We didn't find anything") → "Create new workflow".
  3. Halaman "New workflow" menampilkan empat kolom: Active, Completed, Deferred, Cancelled, dengan status awal Active dan Completed.
  4. Klik status "Active" → ubah nama menjadi "Backlog", pilih warna.
  5. Atur "Assign on status change" (Jane Smith) dan "Allow manual transitions" (Completed).
  6. Klik "Add approval": pilih Approvers (Sam Lee, Jane Smith), centang "Add task creator as approver", opsi "Add files to approval", "Approval due date" (hari), "Finish approval automatically" untuk Approved dan Rejected, dan "Change status after finishing to" (opsional).
  7. Klik Save; ringkasan status menampilkan chip: Completed, Jane Smith, Approval.
- **Komponen UI:** Tab Settings, dropdown pencarian, halaman editor workflow berbentuk kolom, popover detail status, palet warna, multi-select user/status, section approval, checkbox, tombol Cancel/Save.
- **Field/data:** Nama status, Color, Assign on status change, Allowed transitions, Approval (Approvers, creator sebagai approver, files, due date dalam hari, aksi otomatis Approved/Rejected + status tujuan), checkbox "Lock transitions".
- **Catatan:**
  - Workflow default terkunci (ikon gembok).
  - Checkbox "Lock transitions" muncul setelah transisi manual ditentukan.

### Editing workflow
*9 screen — [`screens/Editing workflow/`](screens/Editing%20workflow/)*
- **Lokasi/entry point:** Panel detail proyek (Feature 2.0): dropdown Status > "Edit workflow" di bagian bawah daftar status.
- **Langkah:**
  1. Klik dropdown Status; daftar "Default Workflow" tampil (Backlog, In Progress, Ready for Design, Completed, On Hold, Cancelled).
  2. Klik "Edit workflow" lalu modal "Default Workflow" terbuka, dengan tooltip bahwa perubahan berlaku untuk seluruh akun dan hanya admin yang bisa mengedit.
  3. Klik sebuah status (mis. "New") lalu popover edit muncul: ubah nama (menjadi "Backlog"), pilih warna dari palet.
  4. Pada "Assign on status change (Tasks only)" klik "Select user", pilih pengguna dari daftar (Jane Smith); muncul sebagai chip.
  5. Status "Backlog" kini menampilkan "Jane Smith" di bawah namanya; klik "Save" (atau "Discard changes").
- **Komponen UI:** Modal, dropdown status, tooltip/popover informasi, popover edit status, palet warna, dropdown pencarian pengguna (nama + email), chip pengguna, banner "Don't forget" dengan "Got it".
- **Field/data:** Grup status: Active (New/Backlog, In Progress), Completed (Completed), Deferred (On Hold), Cancelled (Cancelled), masing-masing dengan "Add Status"; per status: nama, warna (13 pilihan), auto-assign user.
- **Catatan:**
  - Banner: "everyone currently using this workflow will see these changes".
  - Workflow bersifat tingkat akun dan hanya dapat diedit admin.

### Creating a status
*7 screen — [`screens/Creating a status/`](screens/Creating%20a%20status/)*
- **Lokasi/entry point:** Tampilan Board pada space (Creative Team) → tombol "+ Status" di kanan kolom; membuka modal "Task workflow".
- **Langkah:**
  1. Dari Board, klik "+ Status"; modal "Task workflow" terbuka (nama workflow dapat diedit).
  2. Pada grup Active (New, Planned, In progress, In review, Ready for review) klik "Add Status"; status baru "Active" muncul.
  3. Ubah nama menjadi "Cancelled" dan pilih warna di panel detail.
  4. Klik Save (atau Discard changes); kolom "Cancelled 0" muncul di Board di antara Ready for review dan Completed.
- **Komponen UI:** Modal workflow, daftar status per grup, panel detail status, palet warna, field "Select user", field "Select status", callout info biru, tombol Save/Discard changes/Done.
- **Field/data:** Grup status: Active, Completed, Deferred, Cancelled (masing-masing dengan "Add Status"). Pengaturan status: nama, Color, "Assign on status change (Tasks only)", "Allow manual transitions to the following statuses only", Add approval, Delete status.
- **Catatan:**
  - Banner: "Changes to this workflow will apply to only the space" dan "Don't forget, everyone currently using this workflow will see these changes."
  - Perubahan perlu disimpan lewat tombol Save.


---

## 7. Request Form

### Creating a request form
*18 screen — [`screens/Creating a request form/`](screens/Creating%20a%20request%20form/)*
- **Lokasi/entry point:** Proyek/space → dropdown di samping tombol "Automation" → "Request forms" (menu lain: Workflows, Apps & Integrations, Blueprints) → Settings tab "Request forms".
- **Langkah:**
  1. Buka Automation ▾ → Request forms; Settings terbuka pada tab Request forms (daftar: Name, Access, Last modified by) → klik "+ Form".
  2. Isi nama form ("Marketing request") dan deskripsi.
  3. Klik "+" pada "Add a question"; pilih tipe pertanyaan (Multiple answers); isi pertanyaan, deskripsi, dan opsi (Print, Website content, Social media, Other); tambah Option atau pakai custom fields sebagai opsi.
  4. Aktifkan "Answer required" dan atur "Add answer to" (Description) lewat dropdown pencarian field.
  5. Tambah pertanyaan Paragraph ("Requirements") dan aktifkan "Answer required".
  6. Atur Response settings dan Form sharing di panel kiri (opsional "Enable public link"), lalu klik Publish.
  7. Modal "Your form is published!" menampilkan link; klik "Go to form" atau salin link (toast "Link copied"). Form tampil bagi pengisi dengan tombol Submit.
- **Komponen UI:** Dropdown menu, halaman Settings bertab, daftar tabel, form builder dua kolom, menu tipe pertanyaan, kartu pertanyaan (ikon hapus, drag handle), toggle, dropdown pencarian field, checkbox, modal sukses, toast, halaman form publik/internal.
- **Field/data:** Panel kiri: About this form (Other spaces), Response settings (Create task, Set status to, Save to, Assign to, Create approval), Form sharing (Form is visible to: Everyone in account; Share public link). Tipe pertanyaan: Single line text, Paragraph, Single answer, Multiple answers, Date, Number, Attachment, Assignee, Importance; Divider: New page, Section header. Daftar form: Marketing request (Internal), My first request form (External).
- **Catatan:**
  - Status form "Draft" sebelum Publish; tersedia tombol Discard, Save, Publish.
  - Pertanyaan wajib ditandai tanda bintang (*) di form akhir.
  - Form dapat diakses juga lewat tombol "+" → Request di Wrike.

### Customizing a form
*4 screen — [`screens/Customizing a form/`](screens/Customizing%20a%20form/)*
- **Lokasi/entry point:** Pengaturan akun (Account management) → "Request forms" → tombol "Configure emails" di kanan atas daftar.
- **Langkah:**
  1. Buka Request forms (daftar: Name, Belongs to, Access, Last modified by) dan klik "Configure emails".
  2. Modal "Customize your thank-you email" terbuka.
  3. Isi Sender's name (Wrike), Greeting (Hello,), Message, dan Closing (Have a nice day).
  4. Ubah Message menjadi "We received your request! We'll get back to you shortly." sambil melihat Example email.
  5. Klik Save; kembali ke daftar form.
- **Komponen UI:** Menu navigasi samping pengaturan, tabel daftar form, modal dengan form + panel pratinjau email, textarea dengan ikon hapus, banner info, tombol Save/Cancel, link "Revert to default email".
- **Field/data:** Sender's name, Logo (diunggah di account settings), Greeting, Message, Closing. Daftar form: Marketing request, My first request form, New event request (Belongs to: Creative Team / Event management; Access: Internal/External).
- **Catatan:**
  - Pengaturan berlaku untuk semua email terima kasih kecuali form yang dikustomisasi secara khusus.
  - Hanya pengirim form di luar organisasi yang menerima email; anggota akun tidak.

### Manage forms
*2 screen — [`screens/Manage forms/`](screens/Manage%20forms/)*
- **Lokasi/entry point:** Menu "More" di sidebar > Request forms; tautan "Manage forms" di halaman Request forms.
- **Langkah:**
  1. Buka halaman "Request forms": daftar formulir (Marketing request, My first request form, New event request) dengan deskripsi dan kotak pencarian.
  2. Klik tautan "Manage forms" (ikon pensil).
  3. Halaman admin "Account management > Request forms" menampilkan tabel formulir dengan tombol "+ Form", "Search", dan "Configure emails".
- **Komponen UI:** Daftar formulir dengan pencarian, halaman admin dengan sidebar pengaturan, tabel (kolom dapat diurutkan), ikon buka/salin tautan, menu "..." per baris.
- **Field/data:** Kolom Name, Belongs to (Creative Team, Event management), Access (Internal/External), tautan, Last modified by; menu admin: Personal settings (Profile, Work schedule, Account info, Email preferences, Accessibility, Workspace appearance) dan Account management (Users, Job roles, Work schedules, Request forms, Workflows, Custom fields, Automation, Timelog, Subscription, Settings, Security, Activity reports, Access reports, Access roles, User types, Custom item types).
- **Catatan:**
  - Kolom Access membedakan formulir internal dan external (publik).
  - Halaman admin berada di area pengaturan akun yang terpisah dari ruang kerja.

### Sending a request form
*7 screen — [`screens/Sending a request form/`](screens/Sending%20a%20request%20form/)*
- **Lokasi/entry point:** Sidebar > "More" > "Request forms" (atau item "Request forms" langsung di sidebar).
- **Langkah:**
  1. Klik "More" di sidebar lalu pilih "Request forms".
  2. Halaman daftar form; pilih "Marketing request".
  3. Form terbuka; centang tujuan permintaan (checkbox) dan isi "Requirements".
  4. Klik "Submit"; tombol berubah menjadi "Submitting...".
  5. Layar konfirmasi "Your request was sent!" dengan tautan ke request, tombol "Open request" dan "Go to Home".
- **Komponen UI:** Menu "More" (popover), halaman daftar form dengan kolom pencarian, form dinamis (checkbox, textarea), tombol Submit dengan state loading, layar sukses dengan tautan salin.
- **Field/data:** Daftar form: Marketing request, My first request form, New event request (link "Manage forms"). Field form: "For what purpose is this request?" * (Print, Website content, Social media, Other), "Requirements" * (textarea). Menu More: Dashboards, My to-do, Request forms, Timesheets, Reports, Calendars, Workload, Created by me, Stream, Starred tasks, Customize menu.
- **Catatan:**
  - Field wajib ditandai tanda bintang.
  - Setelah terkirim, muncul tautan ke task request yang dibuat (dengan ikon salin) dan tombol "Back to list" tetap tersedia.


---

## 8. Dashboard, Widget & Analytics

### Dashboard
*3 screen — [`screens/Dashboard/`](screens/Dashboard/)*
- **Lokasi/entry point:** Sidebar "More" → "Dashboards" (juga tersedia di Tools pada space).
- **Langkah:**
  1. Klik More → Dashboards.
  2. Dashboard "My productivity dashboard" terbuka; widget awal menampilkan "Nothing to show... yet!" disertai petunjuk mengubah pengaturan widget.
  3. Setelah data dimuat, widget terisi: To do this week, Tasks assigned to me, To review, Tasks I completed this week.
- **Komponen UI:** Popup menu sidebar, header dashboard dengan dropdown nama dan ikon pin, toolbar (+ Widget, By Jane S., Change layout, Share, Public link, menu "..."), kartu widget dengan grup yang dapat dilipat, tabel ringkasan status, tag status berwarna dan label proyek.
- **Field/data:** Widget berisi daftar tugas dikelompokkan per periode (This week, Next week), nama tugas, avatar, status (Ready for review, In review, Planned, Completed), tanggal (May 23, May 28), jumlah item; tabel "To review" per status group (New 2, Planned 1, In review 1, Ready for review 1).
- **Catatan:**
  - Empty state widget: "Nothing to show... yet! To see data here, change this widget's settings."

### Adding a custom widget
*11 screen — [`screens/Adding a custom widget/`](screens/Adding%20a%20custom%20widget/)*
- **Lokasi/entry point:** Dashboard ("My productivity dashboard"), tombol "+ Widget" di toolbar.
- **Langkah:**
  1. Klik "+ Widget" lalu muncul layar "New widget" dengan Data source (Creative Team) dan kategori Recommended / Task / Project.
  2. Pilih template "Custom widget" (Add widget).
  3. Editor widget terbuka: beri nama ("Files to review"); pratinjau tabel tugas.
  4. Buka "Fields" dan aktifkan toggle (mis. Status, Location); atur urutan lewat "Priority" (Priority, Name, Status, Due date, Location).
  5. Pilih tipe visual di baris ikon bawah (tabel, angka, donut, bar, stacked bar, dll.).
  6. Atur metrik ("Active"), Break by (None/Status/Assignee/Status group/Other), dan "Then group by".
  7. Lihat pratinjau bar chart atau donut chart (New 38%, Planned 23%, In progress 15%, In review 15%, Ready for review 8%).
  8. Klik "Create widget"; widget "Files to review" muncul di dashboard.
- **Komponen UI:** Galeri template berupa kartu, modal editor dua panel (pratinjau + konfigurasi), tab Data/Design/Filters, dropdown Fields dengan toggle, dropdown sort, radio Break by, picker tipe chart, tombol Cancel/Create widget.
- **Field/data:** Data source (+ "Include items from subfolders"), Reporting on (Task), Metrics (+ Metric), Break by, Then group by, Fields (Assignee, Status, Status group, Location, Files, Importance, Predecessors, Author, ID, Item type), Filters. Template rekomendasi: My tasks, My tasks this week, My overdue tasks, Overdue tasks I created, Followed tasks, My backlog, Starred tasks, My pending file reviews, My overdue file reviews.
- **Catatan:**
  - Pratinjau diperbarui langsung saat konfigurasi diubah; tab "Design" muncul setelah memilih tipe chart.
  - Nama widget default "New custom widget" dapat langsung diedit (teks terblok).

### Adding widget description
*5 screen — [`screens/Adding widget description/`](screens/Adding%20widget%20description/)*
- **Lokasi/entry point:** Halaman "Custom analytics board" (Tools > Custom analytics board), ikon info (i) di header widget "Tasks by status".
- **Langkah:**
  1. Board dalam mode edit (Save/Cancel, "+ Widget", panel Filters).
  2. Klik ikon info pada widget; popover "Widget Details" menampilkan tautan "Add Description".
  3. Klik "Add Description"; muncul editor "Widget Description" dengan placeholder "Start typing to add a description...".
  4. Ketik "Progress analytics" lalu konfirmasi dengan centang (x untuk batal).
  5. Popover kini menampilkan deskripsi dengan ikon pensil untuk mengedit.
- **Komponen UI:** Popover Widget Details, editor teks kecil dengan ikon centang/silang, widget pie chart "Tasks by status", tabel "Task list", panel Filters ("Include Data From: Creative Team", toggle "Update on Every Change").
- **Field/data:** Judul board, widget (pie chart status 28% Completed dst; tabel Task ID, Task Name, Status, Due Date, Assignee, Duration (days), Get details), deskripsi widget.
- **Catatan:**
  - Papan analitik diedit lewat tombol Save/Cancel di atas.

### Editing a widget
*6 screen — [`screens/Editing a widget/`](screens/Editing%20a%20widget/)*
- **Lokasi/entry point:** Tools di sidebar > "Custom analytics board"; mode edit board dengan tombol Save/Cancel; widget "Tasks by status".
- **Langkah:**
  1. Buka board dalam mode edit (widget Pie Chart "Tasks by status").
  2. Di tab "Design", ubah Pie Type dari Classic ke Donut/Ring.
  3. Atur posisi Legend (Top/Left/Right/Bottom) dan Labels (Categories, Value, Percent, Decimals).
  4. Pada tab "Filters", atur Dashboard Filters, "Include Data From", serta mode Slice/Filter atau Highlight.
  5. Hasil tampil langsung pada grafik ("Update on Every Change" aktif); simpan dengan "Save".
- **Komponen UI:** Editor widget 3 panel (Categories/Values, pratinjau grafik, panel Design/Filters), pie/donut/ring chart, toggle, radio, checkbox, tab Filters/Design.
- **Field/data:** Chart type (Pie Chart); Categories: Status (Completed, In progress, In review, New, Planned, Ready for review, dengan warna); Values: Tasks; Pie Type (Classic, Donut, Ring); Legend; Labels; Show 'Others' Slice (include values less than 3% / show 7 slices and 'Others'); Dashboard Filters, Widget Filters (+).
- **Catatan:**
  - Toggle "Update on Every Change" memberi pratinjau real-time.
  - Menambah label Value/Decimals mengubah persentase menjadi mis. 27.8% beserta jumlah tugas.

### Switching widget view
*4 screen — [`screens/Switching widget view/`](screens/Switching%20widget%20view/)*
- **Lokasi/entry point:** Dashboard/board analitik "Custom analytics board" (Tools di sidebar) dalam mode edit > ikon pensil pada widget "Tasks by status".
- **Langkah:**
  1. Buka board analitik dalam mode edit (widget "Tasks by status" berupa pie chart dan "Task list" berupa tabel; panel Filters di kanan).
  2. Klik ikon edit pada widget; editor widget terbuka (tipe Pie Chart, Categories, Values, tab Design).
  3. Buka dropdown tipe visualisasi.
  4. Pilih "Column Chart"; widget berubah menjadi bagan kolom dengan opsi desain baru.
  5. Klik "Save" (atau Cancel).
- **Komponen UI:** Editor widget dengan panel kiri (kategori/nilai), kanvas pratinjau, panel kanan tab Filters/Design, dropdown tipe chart, toggle "Update on Every Change", tombol + Widget / Save / Cancel.
- **Field/data:** Tipe: Indicator, Pie Chart, Column Chart, Bar Chart, Line Chart, Area Chart, Pivot, Table. Categories: Status (Completed, In progress, In review, New, Planned, Ready for review); Values: Tasks; Break by. Desain Pie: Pie Type (Classic/Donut/Ring), Legend (Top/Left/Right/Bottom), Labels (Categories, Value, Percent, Decimals), Show 'Others' slice. Desain Column: Column Type (Classic/Stacked/Stack 100), Value Label, X-Axis, Y-Axis (grid, labels, logarithmic, min/max, interval), Auto Zoom. Filters: Include Data From (Creative Team).
- **Catatan:**
  - Data sumber tetap; hanya presentasi yang berubah, dengan pratinjau langsung.

### Changing layout
*3 screen — [`screens/Changing layout/`](screens/Changing%20layout/)*
- **Lokasi/entry point:** Dashboard "My productivity dashboard", tombol "Change layout" di toolbar.
- **Langkah:**
  1. Buka dashboard dengan widget tersusun vertikal (To do this week, Tasks assigned to me, To review, Tasks I completed this week).
  2. Klik "Change layout"; popover "Select dashboard layout" muncul.
  3. Pilih "Horizontal layout" (widget dalam kolom, scroll horizontal).
  4. Dashboard disusun ulang menjadi tiga kolom.
- **Komponen UI:** Popover pilihan layout dengan thumbnail dan deskripsi, widget daftar tugas dan tabel ringkas, toolbar dashboard (+ Widget, By Jane S., Change layout, Share, Public link, "...").
- **Field/data:** Layout: Vertical (baris, scroll vertikal, cocok untuk pelaporan/analitik) dan Horizontal (kolom, scroll horizontal, cocok untuk pengawasan/papan Kanban).
- **Catatan:**
  - Perubahan diterapkan langsung tanpa konfirmasi.
  - Dashboard dapat dibagikan lewat Share atau Public link.

### Duplicating a dashboard
*7 screen — [`screens/Duplicating a dashboard/`](screens/Duplicating%20a%20dashboard/)*
- **Lokasi/entry point:** Halaman Dashboard → ikon "..." di toolbar → "Duplicate".
- **Langkah:**
  1. Klik "..." dan pilih Duplicate (menu: Rename, Duplicate, Move, Delete).
  2. Modal `Duplicate "Dashboard"` terbuka; nama default "Dashboard copy" diganti menjadi "Events Dashboard".
  3. Pilih Space lewat dropdown (Personal, Creative Team, Event management); pilih Event management.
  4. Klik Save; dashboard baru muncul di Tools pada space Event management.
- **Komponen UI:** Menu konteks, modal kecil, input nama, dropdown pemilih space dengan ikon, tombol Save/Cancel.
- **Field/data:** Name, Space.
- **Catatan:**
  - Tombol Save nonaktif sampai Space dipilih.

### Renaming a dashboard
*4 screen — [`screens/Renaming a dashboard/`](screens/Renaming%20a%20dashboard/)*
- **Lokasi/entry point:** Halaman dashboard (mis. "My productivity dashboard", dari Tools di sidebar) > tombol "..." di toolbar.
- **Langkah:**
  1. Buka dashboard; klik "..." di toolbar.
  2. Pilih "Rename" dari menu.
  3. Judul menjadi input yang bisa diedit (garis bawah); ketik nama baru.
  4. Judul tersimpan sebagai "Dashboard".
- **Komponen UI:** Dropdown menu "...", judul inline-editable, toolbar dashboard (+ Widget, By Jane S., Change layout, Share, Public link, pin), widget kartu.
- **Field/data:** Menu: Rename, Duplicate, Move, Delete. Widget: To do this week, To review (Status group / Active: New, Planned, In review, Ready for review), Tasks I completed this week, Tasks assigned to me (dikelompokkan This week / Next week).
- **Catatan:**
  - Rename dilakukan inline pada judul, tanpa modal.

### Deleting a dashboard
*2 screen — [`screens/Deleting a dashboard/`](screens/Deleting%20a%20dashboard/)*
- **Lokasi/entry point:** Halaman dashboard → ikon "..." di toolbar atas.
- **Langkah:**
  1. Buka dashboard, klik "..."; menu berisi Rename, Duplicate, Move, Delete.
  2. Pilih Delete; dialog konfirmasi "Delete dashboard?" muncul.
  3. Konfirmasi dengan tombol merah Delete (atau Cancel).
- **Komponen UI:** Menu konteks, dialog konfirmasi kecil dengan tombol destruktif merah, tombol Cancel.
- **Field/data:** Pesan: "Deleting Dashboard won't delete any tasks, projects, or other data used in this dashboard."
- **Catatan:**
  - Konfirmasi menjelaskan bahwa hanya dashboard yang dihapus, bukan data sumbernya.

### Creating a public link
*7 screen — [`screens/Creating a public link/`](screens/Creating%20a%20public%20link/)*
- **Lokasi/entry point:** Halaman Dashboard (mis. "My productivity dashboard"), toolbar atas → tombol "Public link".
- **Langkah:**
  1. Klik "Public link" di toolbar dashboard.
  2. Modal "Create a public link" menampilkan toggle "Dashboard public link" (off).
  3. Aktifkan toggle; muncul dialog peringatan "Heads up!" → klik Continue (opsi "Don't show this again").
  4. Link dibuat: terlihat info pembuat, URL, tombol buka dan Copy, "Link expires" dan "Require password".
  5. Aktifkan "Require password": muncul kolom password tersamar, dengan ikon tampilkan/sembunyikan, regenerate, dan copy (password acak terlihat saat ditampilkan).
  6. Tutup modal; muncul banner hijau "Anyone with the public link can see this view. Open settings".
- **Komponen UI:** Modal, toggle switch, dialog konfirmasi peringatan, checkbox, input URL read-only, tombol Copy, dropdown tanggal kedaluwarsa, field password dengan ikon aksi, banner informasi, link "Leave feedback about public links".
- **Field/data:** Status link (toggle), "Created by Jane Smith on 23 May, 2024", URL (https://www.wrike.com/shared/dashboard?token...), Link expires (23 May, 2025 at 22:42), Require password, password.
- **Catatan:**
  - Peringatan: berbagi link ke pihak di luar organisasi dapat mengekspos informasi sensitif.
  - Penerima link hanya dapat melihat, tidak dapat mengedit atau membuka item.
  - Link kedaluwarsa default 1 tahun.

### Creating custom analytics
*4 screen — [`screens/Creating custom analytics/`](screens/Creating%20custom%20analytics/)*
- **Lokasi/entry point:** Sidebar space → Tools → "Custom analytics board" (papan baru).
- **Langkah:**
  1. Buka papan baru; tampil pesan "Welcome to your new dashboard!" dengan tombol "+ Select Data" di bagian "New Widget".
  2. Modal "Choose widget" terbuka; pilih kategori atau widget dari daftar.
  3. Pilih "Tasks by status": tampil deskripsi, field Name ("Tasks by status"), klik Add.
  4. Papan menampilkan pie chart "Tasks by status" dan widget "Task list" (tabel); simpan dengan Save atau batalkan dengan Cancel.
- **Komponen UI:** Halaman editor papan, panel Filters kanan ("Include Data From" dengan chip Creative Team), modal pemilih widget dengan sidebar kategori, panel detail widget, pie chart dengan legenda, tabel, toggle "Update on Every Change", tombol Widget/Save/Cancel.
- **Field/data:** Kategori widget: Pie chart, Lists, Trends, Indicators, Bar chart. Widget: Custom widget, Average of projects/tasks custom field, Burndown by task completion, Project list, Projects and folders by custom field, Projects by status, Task completion progress, Task list, Tasks by status, Tasks by custom field. Tabel Task list: Task ID, Task Name, Status, Due Date, Assignee, Duration (days), Get details.
- **Catatan:**
  - Papan memiliki judul yang dapat diedit dan filter sumber data di panel kanan.


---

## 9. Report

### Creating a report
*7 screen — [`screens/Creating a report/`](screens/Creating%20a%20report/)*
- **Lokasi/entry point:** Sidebar "More" → menu (Dashboards, My to-do, Request forms, Timesheets, Reports, Calendars, Workload, Created by me, Stream, Starred tasks, Customize menu) → "Reports".
- **Langkah:**
  1. Buka More → Reports; halaman Reports menampilkan template "Create an analytics board" dan "Create a report", serta daftar laporan yang ada.
  2. Pilih template (mis. "Active tasks by assignee"); modal "Create report" terbuka.
  3. Judul terisi, filter Type diubah dari "All reports" ke "Tasks"; daftar tipe dapat dicari.
  4. Location default Personal (kemudian Creative Team); pada Data source klik "Select project, folder or space" dan pilih dari dropdown pencarian (Design assets, Creative Team, Feature 2.0, Personal, Event management, My first event).
  5. Sumber data terpilih tampil sebagai chip (Creative Team, Feature 2.0, Personal) dengan tombol "+"; klik Create.
  6. Laporan terbuka dalam tabel dikelompokkan per Assignee.
- **Komponen UI:** Menu popover, galeri kartu template, modal dua panel, input judul, dropdown Type, kotak pencarian, daftar tipe, dropdown pencarian sumber data, chip, tabel laporan dengan grup yang dapat dilipat.
- **Field/data:** Template analytics: Custom analytics board, Project portfolio overview, Project status, Productivity analysis, Utilization and performance, Sprint overview, Agile Velocity, Capacity planning. Template report: Custom report, Active tasks by assignee, Weekly project status, Overdue tasks by assignee, Projects due this month, Unassigned tasks, Time spent this week, Team utilization. Daftar laporan: Name, Created by, Space, Sharing, Created date, Last updated, Subscription. Hasil: Title, Status, Due date, Project or folder; "Group by: Assignee"; total 13 tasks.
- **Catatan:**
  - Tombol Create baru aktif (hijau) setelah Data source dipilih.
  - Halaman laporan menyediakan Edit, Share, Subscribe, Expand all/Collapse all, dan indikator "Updated at".

### Editing a report
*4 screen — [`screens/Editing a report/`](screens/Editing%20a%20report/)*
- **Lokasi/entry point:** Halaman laporan "Active tasks by assignee" (Reports > nama laporan); tombol "Edit" di bar atas.
- **Langkah:**
  1. Buka laporan (tampilan tabel dikelompokkan per Assignee).
  2. Klik "Edit" lalu modal "Edit report" terbuka.
  3. Ubah pengaturan: Layout dari Table ke Column.
  4. Klik "Save".
  5. Laporan tampil sebagai grafik batang bertumpuk per Assignee, diwarnai per Status.
- **Komponen UI:** Modal "Edit report" dua kolom, radio button, checkbox filter (accordion), tabel dengan baris grup (Expand all/Collapse all), grafik batang bertumpuk dengan legend, tombol Edit/Share/Subscribe.
- **Field/data:** Title; Location (Creative Team); Data source (Creative Team, Feature 2.0, Personal, + tambah); Based on (Projects / Tasks and subtasks / Time entries); Layout (Table / Column); Filters (Custom item type, Status: All active/completed/deferred/canceled + Workflow, Assignee, Tasks to do, Approvals, Task type, Importance, Effort, Created date, Start date, Due date). Kolom tabel: Title, Status, Due date, Project or folder. "Group by: Assignee" (+ Status pada tampilan grafik).
- **Catatan:**
  - Footer tabel menampilkan "Total: 13 tasks" dan "Updated at" untuk waktu pembaruan data.
  - Tersedia tautan "Leave feedback" di laporan.

### Subscribing to a report
*6 screen — [`screens/Subscribing to a report/`](screens/Subscribing%20to%20a%20report/)*
- **Lokasi/entry point:** Sidebar > Tools > laporan "Active tasks by assignee" (breadcrumb Reports) > tombol "Subscribe".
- **Langkah:**
  1. Buka laporan (grup awal Assignee, lalu terlihat diubah ke Status group dengan grup "All active 13").
  2. Klik "Subscribe"; popover "Notification will be sent to your Inbox and email address" tampil.
  3. Klik jam (08:00) dan pilih waktu dari daftar dropdown (mis. 09:00).
  4. Pilih hari pengiriman (chip Su-Sa, mis. Fr).
  5. Klik "Subscribe"; tombol di toolbar berubah menjadi "Subscribed".
- **Komponen UI:** Halaman laporan tabel dengan toolbar (Edit, Share, Subscribe, "..."), group by, Expand/Collapse all, popover langganan, dropdown waktu per 30 menit, chip hari toggle, tombol Subscribe/Cancel.
- **Field/data:** Kolom: Title, Status, Due date, Project or folder; total "13 tasks", MAX due date; jam kirim; hari kirim (Su Mo Tu We Th Fr Sa); status tombol Subscribe/Subscribed.
- **Catatan:**
  - Notifikasi dikirim ke Inbox dan email; jadwal berupa hari dan jam yang dapat dipilih.
  - Tombol berubah menjadi "Subscribed" sebagai umpan balik.


---

## 10. Menu & Kustomisasi

### Customizing a menu
*6 screen — [`screens/Customizing a menu/`](screens/Customizing%20a%20menu/)*
- **Lokasi/entry point:** Tombol "+" di header kanan atas → "Customize this menu" (bagian bawah menu).
- **Langkah:**
  1. Klik "+" lalu "Customize this menu"; muncul daftar "Select items to show in this menu" dengan toggle per item.
  2. Klik "Create new type" → modal "Create a custom item type".
  3. Pilih Task type (atau Project type), isi Name ("Bug fixes"), opsional "Change icon".
  4. Klik Create (aktif setelah nama diisi); item baru muncul di daftar dengan toggle aktif.
- **Komponen UI:** Popover menu dengan toggle, kotak pencarian, modal, radio Task type/Project type, input teks, tombol Create/Cancel, link bantuan.
- **Field/data:** Item default: Task, Project, Folder; item kustom: New Product Development (off), Meeting notes (on), Bug fixes (on). Field modal: tipe item, Name, Change icon.
- **Catatan:**
  - Tombol Create dinonaktifkan sampai nama diisi.
  - Tersedia tombol "Back" untuk kembali ke menu pembuatan item.


---

## 11. Akun, Profil & Settings

### Settings
*2 screen — [`screens/Settings/`](screens/Settings/)*
- **Lokasi/entry point:** Klik avatar pengguna di sidebar > "Settings".
- **Langkah:**
  1. Klik avatar; popover menu profil muncul.
  2. Pilih "Settings"; halaman "Profile Details" terbuka dengan navigasi pengaturan di kiri.
- **Komponen UI:** Popover menu profil, halaman pengaturan dengan sidebar dua grup (Personal settings dan Account management), form profil, tombol "Save changes" (nonaktif sampai ada perubahan), dropdown.
- **Field/data:** Personal settings: Profile, Work schedule, Account info, Email preferences, Accessibility, Workspace appearance. Account management: Users, Job roles, Work schedules, Request forms, Workflows, Custom fields, Automation, Timelog, Subscription, Settings, Security, Activity reports, Access reports, Access roles, User types, Custom item types. Profil: First/Last name, Company, Job title, Country of residence, Location, Department, Phone. Workspace Settings: Time zone, Language, Date format, Time format, Week starts on.
- **Catatan:**
  - Pengaturan pribadi dan administrasi akun digabung dalam satu sidebar; menu Account management mengindikasikan peran admin.

### Account info
*2 screen — [`screens/Account info/`](screens/Account%20info/)*
- **Lokasi/entry point:** Menu profil (avatar) > halaman Personal settings; tab "Account info" di sidebar Personal settings.
- **Langkah:**
  1. Buka Personal settings > Profile (Profile Details + Workspace Settings).
  2. Klik "Account info" di sidebar.
  3. Lihat ringkasan akun dan daftar Account Administrators.
- **Komponen UI:** Sidebar dua grup (Personal settings & Account management), form profil, dropdown, tombol "Save changes" (nonaktif), badge "Trial"/"Owner", tabel administrator, tombol "Manage account" dan "Create a free linked account".
- **Field/data:** Profile: First name, Last name, Company, Job title, Country of residence, Location, Department, Phone (kode negara + nomor); Workspace Settings: Time zone, Language, Date format, Time format, Week starts on. Account info: nama akun + badge Trial, Your role in account (Owner), Joined account on, Subscription (50 users, Enterprise Pinnacle base plan), Account registration date; tabel Name, Email, Phone number, Location.
- **Catatan:**
  - Menu Account management (Users, Job roles, Work schedules, Request forms, Workflows, Custom fields, Automation, Timelog, Subscription, Settings, Security, Activity reports, Access reports, Access roles, User types, Custom item types) terlihat untuk peran Owner.
  - "Save changes" abu-abu (disabled) sebelum ada perubahan.

### Updating profile details
*7 screen — [`screens/Updating profile details/`](screens/Updating%20profile%20details/)*
- **Lokasi/entry point:** Settings > Personal settings > "Profile" (halaman Profile Details).
- **Langkah:**
  1. Buka Profile Details (avatar awal berupa inisial "JS").
  2. Hover avatar; muncul overlay "Upload"; unggah foto sehingga avatar berubah menjadi foto.
  3. Edit field (Job title menjadi "Product Designer", Location menjadi "San Francisco"); tombol "Save changes" aktif bersama "Cancel".
  4. Klik "Save changes"; perubahan tersimpan (tombol nonaktif lagi, subjudul "Product Designer at SLMobbin").
  5. Gulir ke "Workspace Settings" dan "Sign-in & Security" untuk setelan lain.
- **Komponen UI:** Avatar dengan overlay upload, form dua kolom, dropdown kode negara, tombol Save changes/Cancel, bagian Workspace Settings, halaman Sign-in & Security.
- **Field/data:** First name, Last name, Company, Job title, Country of residence, Location, Department, Phone (+1). Workspace Settings: Time zone (GMT-07:00 Pacific), Language, Date format (dd/mm/yyyy), Time format (24-Hour), Week starts on (Monday); Tasks & Subtasks: New tasks created by default as (Backlogged), Default Effort mode (No Effort), Default task duration (d/h/m), checkbox pop-up penyelarasan tanggal subtask; Keyboard shortcut to submit comments (CMD+Enter atau Enter). Sign-in & Security: email, ganti password, Two Step verification, Authorized Apps.
- **Catatan:**
  - Save changes hanya aktif jika ada perubahan; Cancel membatalkan.
  - Foto profil di header ikut berubah setelah diunggah.

### Updating email preferences
*5 screen — [`screens/Updating email preferences/`](screens/Updating%20email%20preferences/)*
- **Lokasi/entry point:** Settings > Personal settings > "Email preferences".
- **Langkah:**
  1. Dari Profile Details klik "Email preferences".
  2. Tinjau "Email Integration", "Notifications Settings" dan "Task, Project, and Folder Updates".
  3. Centang/hapus centang kondisi notifikasi; tombol "Save changes" aktif (dengan "Cancel").
  4. Ubah jadwal di "Email notification schedule" bila perlu.
  5. Klik "Save changes"; tombol kembali nonaktif.
- **Komponen UI:** Halaman setelan bergulir, tabel integrasi email, checkbox, dropdown jam, tombol Save changes/Cancel (aktif hanya saat ada perubahan), ikon bantuan.
- **Field/data:** Email Integration: Email sent from, Create tasks in folder (Root folder), Type of tasks (Backlogged). Notify me by email when: Someone @mentions me, Someone assigns me a task, Folder or task I follow is shared with someone new, Report is shared with me or delivered on schedule. Untuk task saya (dibuat/ditugaskan) dan task lain yang saya ikuti: Date/Assignee/Status changed, Description changes/files added, Comments without @mention, Task added/removed from Project or Folder. Jadwal: Risk report mingguan (Senin 6:00), AI suggestions untuk Automation rules bulanan (Senin terakhir 8:00).
- **Catatan:**
  - Tombol Save berstatus nonaktif sampai ada perubahan.
  - Setiap blok setelan punya tombol Save sendiri.

### Switching to dark mode
*6 screen — [`screens/Switching to dark mode/`](screens/Switching%20to%20dark%20mode/)*
- **Lokasi/entry point:** Settings > Personal settings > "Workspace appearance".
- **Langkah:**
  1. Dari Profile Details, klik "Workspace appearance".
  2. Halaman memuat "Appearance" (Light, Dark, My computer) dan "Themes" (Midnight, Winter White, Merlot, Sky Blue, Let's Go Indigo, Imperial Purple, Eggshell Creme, Wonderland, dst.).
  3. Pilih kartu "Dark"; bagian Themes hilang dan UI langsung berubah gelap.
  4. Kembali ke workspace: board, Gantt, dan dashboard tampil dalam mode gelap.
- **Komponen UI:** Kartu pilihan tema dengan pratinjau mini (terpilih berbingkai biru), grid tema warna, perubahan tema instan.
- **Field/data:** Mode: Light, Dark, My computer (ikuti sistem). Tema: Midnight, Winter White, Merlot, Sky Blue, Let's Go Indigo, Imperial Purple, Eggshell Creme, Wonderland dan lainnya.
- **Catatan:**
  - Pilihan tema warna hanya tampil pada mode Light.
  - Perubahan diterapkan langsung tanpa tombol simpan.

### Work schedule
*3 screen — [`screens/Work schedule/`](screens/Work%20schedule/)*
- **Lokasi/entry point:** Settings > Personal settings > "Work schedule".
- **Langkah:**
  1. Dari Profile Details klik "Work schedule".
  2. Lihat ringkasan jadwal (Default Schedule, workweek, kapasitas harian) dan kalender tahunan.
  3. Gulir untuk melihat 12 bulan; klik tanggal untuk menambah/mengubah pengecualian jadwal (tautan "Change work schedule" untuk mengganti jadwal).
- **Komponen UI:** Halaman informasi dengan deskripsi, ringkasan jadwal, navigasi tahun (panah), kalender 12 bulan dengan akhir pekan berwarna merah dan tanggal hari ini dilingkari, penghitung hari.
- **Field/data:** Work schedule (Default Schedule), Workweek (Mo-Su; Sa/Su redup), Daily capacity (8h), tahun 2024, Working days (262d), Non-working days (104d); petunjuk "Click a date to add or edit a schedule exception"; tautan "Learn more about setting up and managing work schedules".
- **Catatan:**
  - Teks peringatan: mengubah work schedule dapat memengaruhi tanggal project/task.
  - Dapat digunakan untuk mencatat libur/PTO sebagai pengecualian.

### Apps & Integrations
*8 screen — [`screens/Apps & Integrations/`](screens/Apps%20%26%20Integrations/)*
- **Lokasi/entry point:** Menu avatar profil (sidebar kiri atas) > "Apps & Integrations".
- **Langkah:**
  1. Klik avatar; menu: Settings, Workspace appearance, Recycle bin, Apps & Integrations, Mobile apps, Help, Privacy policy, Log out.
  2. Pilih "Apps & Integrations"; tab Gallery tampil (Recommended).
  3. Telusuri kategori (mis. Analytics: Clearbit, Mixpanel, Tableau) dengan "Send us a request" untuk integrasi yang hilang.
  4. Buka tab "Wrike Sync" (sinkronisasi 2 arah: Jira, Bitbucket, Github, Gitlab, Azure DevOps, Salesforce, HubSpot, Excel, Google Sheets, Google Calendar, Miro, Zendesk, Airtable).
  5. Buka tab "Wrike Integrate" (otomasi no-code, 400+ konektor).
  6. Buka "Authorized apps": "There are no apps authorized in your Wrike account."
  7. Buka tab "API": form "API Apps" (App name + "Create new") dan tautan dokumentasi.
- **Komponen UI:** Modal/halaman penuh dengan tab, sidebar kategori, kartu integrasi dengan badge paket (All plans, Business, Team, Enterprise), pencarian, banner promosi, form input app name.
- **Field/data:** Kategori (Artificial Intelligence, Analytics, Collaboration, Customer Relations, Digital Asset Management, Desktop & Mobile Apps, Developer, Finance, HR, IT, Marketing, Productivity, Project Management, Sales); integrasi (Android, iOS, Windows & Mac, Wrike ToDo, MS Teams, Slack, Zoom, Outlook, Gmail, Office Documents, Adobe CC, Document Editor, Github/Gitlab/Azure DevOps/Bitbucket via Wrike Sync, Typeform, Jotform, Google Forms, Survey Monkey); App name API.
- **Catatan:**
  - Kartu menampilkan batasan paket per integrasi.
  - Empty state "no apps authorized" ditampilkan pada Authorized apps.


---

## 12. Pengguna & Tim

### Inviting a teammate
*9 screen — [`screens/Inviting a teammate/`](screens/Inviting%20a%20teammate/)*
- **Lokasi/entry point:** Tombol "Share" di tampilan proyek (Feature 2.0, Gantt chart); modal "Share project".
- **Langkah:**
  1. Klik "Share"; modal "Share project Feature 2.0" tampil dengan daftar "Shared with" (My Team - 2 users, Jane Smith, akses Full).
  2. Ketik email di "Add people by name or email" lalu pilih "Invite by email".
  3. Modal "Invite people to join Mobbin" terbuka (46 seats available).
  4. Opsional: aktifkan "Add welcome item or space" dan pilih proyek/folder/space (mis. Creative Team); isi pesan opsional.
  5. Pilih "User type" dari dropdown (Admin, Regular User, External User, Collaborator, Viewer); di sini dipilih External User.
  6. Klik "Send invite".
  7. Modal konfirmasi "Item shared with 1 person" dengan tautan "See all pending invites" dan tombol "Got it".
- **Komponen UI:** Modal berlapis, input dengan saran, chip email, toggle (label "New"), panel samping "Add welcome item or space", dropdown tipe pengguna dengan deskripsi, tombol Copy link dan Advanced.
- **Field/data:** Email; toggle welcome item/space; pesan (opsional); User type: Admin (can manage account settings and edit), Regular User (can edit), External User (can edit, but can't share), Collaborator (can only comment, change statuses, attach files), Viewer (can only comment); tautan Create or manage, Learn more; hak akses Full pada daftar berbagi.
- **Catatan:**
  - Ada penghitung sisa seat (46 seats available) dan tautan "Get more seats".
  - Penerima mendapat email berisi tautan untuk bergabung dan akses item setelah membuat akun.

### User types
*2 screen — [`screens/User types/`](screens/User%20types/)*
- **Lokasi/entry point:** Settings > Account management > "User types".
- **Langkah:**
  1. Dari Profile Details klik "User types" di sidebar Account management.
  2. Pilih tab lisensi (Full license, Limited license, Free license).
  3. Pilih kategori izin di kiri (mis. General) dan tinjau/ubah toggle izin per tipe pengguna.
  4. Opsional: "+ Create User type".
- **Komponen UI:** Tab lisensi, daftar kategori, matriks izin dengan toggle (aktif hijau, nonaktif/terkunci abu-abu), tombol "+ Create User type".
- **Field/data:** Tab: Full license, Limited license, Free license. Kategori: General, Sharing, Folders and Projects, Tasks and Subtasks, Time tracking, Collaborate, Automatic calculations, Reports, Misc. Kolom: Admin, Regular User, External User. Izin (General): Access admin privileges ("Can manage account, update permissions, and invite users"), My Team member ("Has all permissions included with My Team"), Integrations with automation features ("Can create and edit automation rules that work with an integrated app").
- **Catatan:**
  - Sebagian toggle tampak terkunci/nonaktif (mis. admin privileges untuk non-admin).
  - Setiap izin disertai deskripsi singkat.

### Deleting a user
*5 screen — [`screens/Deleting a user/`](screens/Deleting%20a%20user/)*
- **Lokasi/entry point:** Account management → "Users" (dari Account info / menu profil) → klik pengguna pada daftar.
- **Langkah:**
  1. Dari Account info, buka Account management → Users; daftar "All users (4)" tampil.
  2. Klik pengguna dengan status "Invitation pending"; panel detail kanan terbuka (tab Settings/Groups).
  3. Pada bagian "Delete user" klik tombol "Delete user".
  4. Dialog "Delete ... forever?" muncul; klik "Delete forever".
  5. Toast "... was deleted" muncul; jumlah pengguna menjadi 3.
- **Komponen UI:** Menu samping pengaturan, panel grup + tabel pengguna, kotak pencarian, Filters/Fields, panel detail slide-in dengan tab, tombol Edit settings/Resend invitation/Delete user, dialog konfirmasi destruktif, toast.
- **Field/data:** Kolom: Name, Email, User type, Status, Work Schedule. Grup/tipe: All users, Group Admins, Ungrouped, Owner, Admin, Regular User, External User, Collaborator, Viewer. Info seat: "4 allocated seats: 2 in use, 2 pending. Available: 46. Get more seats". Account info: role (Owner), Joined account on, Subscription (50 users, Enterprise Pinnacle base plan), Account Administrators (Name, Email, Phone number, Location).
- **Catatan:**
  - Pengguna yang dihapus tidak dapat mengakses akun dan tidak dapat dipulihkan.
  - Untuk undangan pending, penghapusan membatalkan undangan tanpa memberi tahu penerima.

### Deleting an account
*9 screen — [`screens/Deleting an account/`](screens/Deleting%20an%20account/)*
- **Lokasi/entry point:** Personal settings/Account management → "Settings" → gulir ke bagian "Delete account" → link "Delete my account".
- **Langkah:**
  1. Buka Settings (Account name, Date and week settings, Fiscal Year Settings, Default currency, Full account backup, Account owner).
  2. Gulir ke bagian "Delete account" dan klik "Delete my account".
  3. Halaman peringatan "Deletion of your profile and 'SLMobbin' account": pilih "Keep my account" atau "Continue deletion".
  4. Form penghapusan: pilih alasan utama dari dropdown, isi Additional comments, masukkan password untuk konfirmasi.
  5. Klik "Delete my account"; pengguna diarahkan ke halaman utama marketing Wrike (halaman Log in).
- **Komponen UI:** Halaman Settings panjang, halaman peringatan, banner bantuan (nomor telepon, link "create a request"), dropdown alasan, textarea, input password tersamar, tombol Keep my account (utama) dan Delete my account (link).
- **Field/data:** Opsi alasan: I cannot adjust Wrike to my workflow; My team doesn't use Wrike often enough; I couldn't find the following features; I don't want to use a cloud solution; There is no need for Wrike at the moment; It's all about money; Other. Field: Additional comments, password. Bagian lain di Settings: Account owner (Transfer account ownership), backup (Account ID, "Include BI Export data in backup", Download backup tool).
- **Catatan:**
  - Peringatan: semua data (tugas, proyek, file, diskusi, pengguna) dihapus; bila tidak ada akun lain, profil juga dihapus dan tidak bisa login lagi.
  - Menu samping menyusut menjadi item terbatas selama alur penghapusan.
  - Hanya Owner yang memiliki akses; "Keep my account" ditonjolkan sebagai tombol utama.


---

## 13. Recycle Bin

### Recycle bin
*2 screen — [`screens/Recycle bin/`](screens/Recycle%20bin/)*
- **Lokasi/entry point:** Klik avatar pengguna (kiri atas sidebar) > menu "Recycle bin".
- **Langkah:**
  1. Buka menu avatar (nama, jabatan, workspace; item Settings, Workspace appearance, Recycle bin, Apps & Integrations, Mobile apps, Help, Privacy policy, Log out).
  2. Pilih "Recycle bin"; halaman daftar item terhapus terbuka.
- **Komponen UI:** Popover menu profil, halaman daftar, tombol "Empty bin", toolbar filter (All tasks, Priority, ikon tampilan/lainnya).
- **Field/data:** Daftar item terhapus (Resource allocation, New hire training docs, Prep sprint planning, Sprint Planning, Development, test) dengan ikon tipe item dan status "New".
- **Catatan:**
  - Tombol "Empty bin" tersedia di judul halaman untuk mengosongkan seluruh isi.

### Restoring an item
*4 screen — [`screens/Restoring an item/`](screens/Restoring%20an%20item/)*
- **Lokasi/entry point:** Halaman Recycle bin (menu avatar > Recycle bin).
- **Langkah:**
  1. Di daftar Recycle bin klik item (mis. "Resource allocation").
  2. Panel detail terbuka di kanan dengan tombol "Delete forever" dan "Restore"; log aktivitas menampilkan "Deleted task".
  3. Klik "Restore".
  4. Item hilang dari daftar Recycle bin (tersisa 5 item) dan task kembali ke project asalnya (Feature 2.0), panel detail menampilkan task normal.
  5. Tutup panel; daftar tanpa item tersebut.
- **Komponen UI:** Daftar item, panel detail samping, tombol aksi (Delete forever, Restore), tag project, activity log, kolom komentar.
- **Field/data:** Panel: Status, Assignee, Date, Search fields, Importance, Author, Created date, Item type, ID, Time spent, More fields (Budget, Impact, Notes), No subitems/files/approvals/dependencies, deskripsi, riwayat aktivitas.
- **Catatan:**
  - Panel detail menunjukkan project asal sehingga pemulihan mengembalikan item ke lokasi semula.
  - Aksi destruktif "Delete forever" ditempatkan berdampingan dengan Restore.

### Emptying recycle bin
*3 screen — [`screens/Emptying recycle bin/`](screens/Emptying%20recycle%20bin/)*
- **Lokasi/entry point:** Halaman "Recycle bin" (dapat dibuka dari menu avatar profil > Recycle bin); tombol "Empty bin" di samping judul.
- **Langkah:**
  1. Buka Recycle bin yang berisi daftar item terhapus (mis. New hire training docs, Prep sprint planning, Sprint Planning, Development, test).
  2. Klik "Empty bin".
  3. Dialog konfirmasi "Empty recycle bin? This will delete 6 items forever." muncul.
  4. Klik "Empty" (tombol merah), atau "Cancel".
  5. Halaman menampilkan empty state "Nothing here!" dan tombol "Empty bin" menjadi nonaktif.
- **Komponen UI:** Daftar item, dialog konfirmasi (tombol destruktif merah), ilustrasi empty state (mercusuar), toolbar filter/sort.
- **Field/data:** Baris item dengan ikon, nama, ikon tipe, status "New"; filter "All tasks", sort "Priority".
- **Catatan:**
  - Tindakan permanen ("delete ... forever") dengan konfirmasi eksplisit.
  - Di dialog tertulis 6 item, sedangkan daftar yang terlihat 5 (kemungkinan sebagian tidak terlihat).
