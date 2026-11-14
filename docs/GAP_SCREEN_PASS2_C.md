# Gap layar pass 2, lane C (shell, Inbox, My to-do, Stream, akun, pengaturan, Recycle bin)

Gambar rujukan dari `docs/screens/` (hanya dibaca). **Gambar yang dibuka satu per satu: 18 dari ±140** di 26 flow: Inbox 0 dan 2, My to-do 1, Filtering & sorting to-do 2, Settings 0 dan 1, Recycle bin 1, Customizing sidebar menu 2, Searching Wrike 2, Updating profile details 2, Updating email preferences 2, Stream 1, Logging in 1, Setting up two-factor authentication 4, Work schedule 1, Accepting a team invitation 3. Flow lain (Marking all as read, Logging out, Resetting password, Changing password, Account info, Dark mode, Leaving a feedback, Customizing a menu, Restoring an item, Emptying recycle bin, Onboarding) dinilai dari gambar tetangga dan kode, **bukan** dari gambar sendiri. Teks promo, logo, trial counter, kartu "Quick start", dan tombol login sosial bukan gap (D-106).

| # | Flow | Gambar rujukan | UI aktual (sebelum) | Gap | Prio | Status |
|---|---|---|---|---|---|---|
| C1 | Logging out, Settings | Settings 0 | Menu akun: ikon generik, enam entri, tanpa identitas, tanpa pemisah | Header nama + jabatan, avatar inisial, Workspace appearance, pemisah sebelum Log out | P2 | Selesai |
| C2 | Customizing a menu / sidebar menu | Customizing sidebar menu 2 | "Customize menu" hanya tautan ke halaman profil | Dialog "Shown in menu / Hidden from menu" dengan tombol mata, simpan seketika | P1 | Selesai |
| C3 | Recycle bin, Emptying recycle bin | Recycle bin 1 | Judul, dua `<select>` besar, tombol Empty di bawah filter, baris tanpa ikon | Tombol Empty bin di samping judul, toolbar ringkas, ikon tipe per baris, Restore kecil | P2 | Selesai |
| C4 | Stream | Stream 1 | Satu kolom, tombol filter segmented di atas, daftar datar | Kartu per item, panel Filters di kanan dengan radio (Everything / Everything I followed / Assigned to me / My conversations) dan Hide auto updates | P2 | Selesai |
| C5 | Settings, Updating profile details, Updating email preferences, Work schedule, Account info | Settings 1, Updating email preferences 2, Work schedule 1 | Nav pribadi hanya Profile, Account, Security; Work schedule di grup Account management; tanpa kepala identitas | Grup Personal berisi Profile, Work schedule, Account, Email preferences, Workspace appearance (tautan jangkar), Security; kepala identitas (inisial, nama, jabatan di perusahaan) | P2 | Selesai |
| C6 | My to-do, Filtering & sorting to-do | My to-do 1, Filtering & sorting to-do 2 | Dua `<select>` berlabel besar, kotak tambah cepat berbingkai, avatar di ujung | Toolbar chip (ikon filter, ikon sort, opsi) tanpa bingkai, baris "+" tambah cepat, avatar di awal baris, judul grup kapital | P2 | Selesai |
| C7 | Inbox, Marking all as read | Inbox 0, Inbox 2 | Konten dibatasi `max-w-6xl`; keadaan kosong hanya satu baris teks | Lebar penuh; keadaan kosong dengan ikon bulat, judul, dan petunjuk | P3 | Selesai |
| C8 | Shell: bar atas | Inbox 0 | Tombol "+" hantu, ikon akun generik | "+" berwarna primer, avatar inisial | P3 | Selesai |
| C9 | Shell: sidebar | Inbox 0, Settings 0 | Tombol "New Space" di dasar daftar; Inbox tanpa lencana | Tombol "+" di header Spaces; lencana belum dibaca pada Inbox | P2 | Selesai |
| C10 | Logging in, Resetting password, Accepting a team invitation | Logging in 1 | Kartu putih di latar putih | Latar abu-abu lembut di belakang kartu (semua halaman AuthCard) | P3 | Selesai |
| C11 | Customizing a menu | Customizing sidebar menu 2 | Tidak ada urutan | Seret-urut entri: server hanya menyimpan kunci tersembunyi, urutan butuh API/migrasi | P3 | Ditunda (butuh API) |
| C12 | Work schedule | Work schedule 1 | Formulir jadwal tanpa kalender tahunan | Kalender 12 bulan dengan hitungan hari kerja/non-kerja dan klik tanggal untuk pengecualian | P3 | Ditunda (usaha L, halaman sudah fungsional) |
| C13 | Inbox | Inbox 0 | Kotak centang dan dua tombol terlihat di atas | Ikon filter dan menu "..." di ujung baris tab | P3 | Ditunda (kosmetik, aksi sudah ada) |
| C14 | Searching Wrike, Filtering search results | Searching Wrike 2 | Halaman hasil dengan preset dan filter | Master-detail dengan panel item di kanan | P3 | Ditunda (bergantung panel item lane A) |
| C15 | Accepting a team invitation | Accepting a team invitation 3 | Kartu satu kolom dengan indikator kekuatan sandi | Modal dua kolom dengan ilustrasi | P3 | Ditunda (fungsi setara, ilustrasi dikecualikan) |
| C16 | Setting up two-factor authentication | 2FA 4 | Komponen `TwoFactorSettings` (QR, kunci, kode) | Tidak ada gap fungsional yang terlihat dari gambar 4 | - | Setara |
| C17 | Onboarding | (tidak dibuka) | `OnboardingWizard` | Belum dibandingkan gambar per gambar | - | Ditunda (tidak diperiksa) |
| C18 | Sidebar | Settings 0 | Tanpa "Hidden spaces", tanpa "+" di Tools | Bagian Hidden spaces, tombol + per bagian | P3 | Ditunda |

**Hitungan:** Selesai 10 (C1 sampai C10), Setara 1, Ditunda 7. Tidak ada gap P1/P2 yang tersisa.
