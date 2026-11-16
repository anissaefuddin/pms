# UAT P0 dan P1

Gunakan instance pilot dengan versi yang dicatat. Untuk setiap requirement, catat PASS, FAIL atau SKIP beserta tanggal, pemeran, dan tautan bukti. Bukti otomatis tercatat di [TESTING.md](TESTING.md#verifikasi-kandidat-ga-13-november-2026).

## Pelaksanaan UAT teknis lokal

Revisi `main` pada `247ca12` dengan perubahan working tree. Playwright memakai Chrome, API Go dan PostgreSQL 16 terisolasi dengan data serta akun sintetis. Stack `pms-manual-app` berstatus berhenti dan image yang tercatat `pms:m1-views`. Hasil otomatis di bawah memberi bukti per alur. Kolom Hasil UAT pada matriks tetap untuk penerimaan per requirement oleh perwakilan pilot.

| Alur | Requirement yang disentuh | Skenario yang diperiksa |
|---|---|---|
| Collaboration / pilot | AUTH-01, SPC-05, VIEW-02, TASK-01/04/07/08A, NAV-03 | Login, proyek dan task, drag status, mention, lampiran, konflik ETag dan SSE reconnect |
| Penjadwalan dan view | SET-07, VIEW-03/05B/06/09, TASK-11A | Kalender hari kerja, perubahan jadwal penerus, custom field Number, swimlane dan Calendar |
| Workflow dan form | WF-02/03A, SPC-12, FORM-01/02/03/06/08 | Guard, approval, formulir internal dan publik, akses requester, rotasi link |
| Timesheet dan anggaran | VIEW-07, TASK-12/13 | Input durasi, Timelog proyek dan pembacaan budget |
| Dashboard dan laporan | DASH-01/02/03A, RPT-01/02/03 | Angka widget sesuai fixture, pembuatan dan ekspor report, konfigurasi langganan |

**Hasil teknis akhir 14 November 2026: lima suite lulus melalui run utama dan retest terarah, mencakup 14 skenario Playwright unik.** Run pertama mencatat satu HTTP 500 pada daftar item Space saat request dibatalkan. Handler diperbaiki dan kasusnya ditambahkan ke tes regresi PostgreSQL. Retest M2 lulus dua kali setelah pemilihan predecessor dibuat berdasarkan bar yang dituju. M5 lulus pada pengulangan setelah pemindaian axe sempat kehilangan execution context. Tidak ada 5xx di log setelah perbaikan.

**Penerimaan pengguna: NOT RUN, 93 requirement.** Kandidat lokal `g0-local-247ca12` tersedia di [localhost:18082](http://127.0.0.1:18082) dengan data sintetis dan akun `g0-pilot@example.test`. Kredensial admin disimpan dalam file lokal di direktori yang ditunjuk `/private/tmp/pms-g0-candidate-current`. Smoke test pada build terbundel lulus untuk health, readiness, aset frontend, login, sesi, dan daftar Space. Kandidat hanya dapat diakses dari mesin ini. Catat hasil dan bukti penerimaan per requirement di tabel berikut.

| Requirement | Prioritas | Skenario penerimaan (PRD) | Hasil UAT | Bukti |
|---|---|---|---|---|
| AUTH-01 | P0 | Login email + password dan tautan lupa password. **Tidak ada public signup**: user masuk hanya lewat undangan (AUTH-06, USR-01) | NOT RUN | — |
| AUTH-02 | P0 | Logout dari menu profil | NOT RUN | — |
| AUTH-03 | P0 | Reset password via email; indikator kekuatan password, minimal 8 karakter | NOT RUN | — |
| AUTH-04 | P0 | Ganti password (lama/baru/ulang) | NOT RUN | — |
| AUTH-05 | P1 | 2FA TOTP: QR + secret key, verifikasi kode 6 digit, status aktif dan tanggalnya | NOT RUN | — |
| AUTH-06 | P0 | Terima undangan tim: nama, password, departemen, jabatan, persetujuan ToS | NOT RUN | — |
| AUTH-07 | P1 | Wizard first-run untuk admin pertama: nama organisasi → proyek pertama + 3 task → pilih view → undang anggota. Pertanyaan profil perusahaan dari layar sumber (ukuran tim, departemen) tidak dipakai karena instance sudah milik satu klien | NOT RUN | — |
| AUTH-09 | P0 | Bootstrap instance lewat perintah CLI (`pms bootstrap`, dijalankan operator): membuat Account, admin pertama (email dari argumen, link set-password via email), workflow default, dan Space default. Idempoten; menolak jalan jika admin sudah ada | NOT RUN | — |
| NAV-01 | P0 | Sidebar: Search, Inbox, More, daftar Space (Personal + tim), Shared with me, Blueprints, Hidden spaces | NOT RUN | — |
| NAV-02 | P1 | Sidebar bisa dikustomisasi (show/hide: Created by me, Workload, Calendars, Dashboards, My to-do, Request forms, Timesheets, Reports, Stream, Starred). Search dan Inbox selalu tampil | NOT RUN | — |
| NAV-03 | P0 | Inbox notifikasi (mention, assign, perubahan) dengan indikator belum dibaca; Mark all as read dan Archive all | NOT RUN | — |
| NAV-04 | P1 | Stream aktivitas dengan filter (Everything, Followed, Assigned to me, My conversations) dan opsi "Hide auto updates" | NOT RUN | — |
| NAV-05 | P0 | My to-do: daftar task milik user dengan filter (All/Active/Overdue) dan sort (Priority, Date, Modified, Created, Status, Importance, Title) | NOT RUN | — |
| NAV-06 | P0 | Pencarian global (Ctrl/Cmd+K) dengan scope (Everywhere / Space / Project); hasil dikelompokkan (Tasks, Projects & Folders, Comments, Users, Request forms) | NOT RUN | — |
| NAV-07 | P1 | Halaman hasil pencarian dengan filter preset, keyword, filter tambahan, dan sort Relevance | NOT RUN | — |
| SPC-01 | P0 | CRUD Space dasar: nama, deskripsi, anggota, sharing. Space default dibuat saat bootstrap (AUTH-09) dan setiap user mendapat Personal space, sehingga Project selalu punya lokasi | NOT RUN | — |
| SPC-02 | P0 | Space overview: tab view (Table/Board) atas seluruh item space | NOT RUN | — |
| SPC-03 | P1 | Preset filter space (All tasks, All active, My active) dan sort (Priority, Due date, Status, Name, Predecessors, Manual order) | NOT RUN | — |
| SPC-04 | P0 | Pohon Projects and folders di sidebar dan view di dalamnya | NOT RUN | — |
| SPC-05 | P0 | Buat Project lewat tombol "+": name, location, default workflow, owner, start/finish date, default view, sharing. Field finansial (billing type, budget) ditambahkan ke form ini oleh TASK-12 di M4 | NOT RUN | — |
| SPC-06 | P0 | Buat Folder (tanpa atribut proyek): name, location, workflow, default view, sharing | NOT RUN | — |
| SPC-07 | P0 | Panel detail project: status, assignee/owner, tanggal, subitem, files, deskripsi, activity log; rename inline. Bagian approval baru muncul saat SPC-12 tersedia (M3) | NOT RUN | — |
| SPC-08 | P0 | Status project mengikuti workflow (Backlog, In Progress, Completed, On Hold, Cancelled, …) | NOT RUN | — |
| SPC-09 | P0 | Set start/due date project | NOT RUN | — |
| SPC-10 | P1 | Multi-lokasi: satu item bisa ditambahkan ke beberapa project/folder/space | NOT RUN | — |
| SPC-11 | P0 | Sharing dengan peran Full, Editor, Limited, Read Only ke user, grup, atau email eksternal | NOT RUN | — |
| SPC-12 | P1 | Approval pada item: description, approvers, due date, files; approver memilih Approve/Reject + komentar; tampil penghitung (0/2) | NOT RUN | — |
| SPC-13 | P1 | Blueprint: simpan struktur item (task/project/folder + status, assignee, tanggal) sebagai template dan buat item dari blueprint | NOT RUN | — |
| SPC-15 | P1 | Export project ke Excel dan import | NOT RUN | — |
| SPC-17 | P0 | Menu view: copy link to view, rename, duplicate, move up/down, delete | NOT RUN | — |
| SPC-18 | P1 | Buat Space dari template per kategori (Marketing, HR, IT, PM, dll.). Template membawa project, task, view, form, custom field, dan workflow | NOT RUN | — |
| VIEW-01 | P0 | **Table**: kolom bisa dipilih, inline edit, hierarki expand/collapse, tambah item di baris terakhir | NOT RUN | — |
| VIEW-02 | P0 | **Board/Kanban**: kolom per status, drag-drop kartu mengubah status | NOT RUN | — |
| VIEW-03 | P1 | Board: group by (None / Assignee) dalam bentuk swimlane | NOT RUN | — |
| VIEW-04 | P1 | Board: atur field yang tampil di kartu (cover image, days in status, location, author, ID, item type, tanggal, custom field) | NOT RUN | — |
| VIEW-05A | P0 | **Gantt**: bar per item, progres %, drag/resize untuk ubah tanggal, milestone | NOT RUN | — |
| VIEW-05B | P1 | Gantt: dependensi FS/SS/FF/SF dan penjadwalan (hari kerja, geser task penerus) yang dihitung di backend; library Gantt hanya menampilkan | NOT RUN | — |
| VIEW-06 | P1 | **Calendar**: tampilan bulanan, navigasi Today/prev/next, filter | NOT RUN | — |
| VIEW-07 | P1 | **Timelog view**: daftar time entry per proyek dengan kolom user, tanggal, durasi, kategori, komentar, billing type, lock. Kolom fees/cost menyusul bersama TASK-14 | NOT RUN | — |
| VIEW-08 | P0 | Kelola view: tambah view, set default, duplicate (Public/Personal), rename, urutkan, hide/show | NOT RUN | — |
| VIEW-09 | P1 | Pemilih kolom dengan grup Management, Schedule, Other, Custom fields, Baselines | NOT RUN | — |
| TASK-01 | P0 | Buat task dari "+" global, inline di Table, di kolom Board, dan di Gantt; field: name, type, status, assignee, date, location | NOT RUN | — |
| TASK-02 | P0 | Subitem bertingkat dengan jumlah subitem tampil di kartu/panel | NOT RUN | — |
| TASK-03 | P0 | Panel detail item: status, assignee, date, description (rich text), subitems, files, activity; menu aksi: convert to milestone, delete. Bagian approval baru muncul saat SPC-12 tersedia (M3) | NOT RUN | — |
| TASK-04 | P0 | Ubah status sesuai workflow item | NOT RUN | — |
| TASK-05A | P0 | Date picker: start, due, duration, opsi working days only (sebelum SET-07 tersedia, hari kerja default Senin–Jumat) | NOT RUN | — |
| TASK-05B | P1 | Effort dan allocation per item; durasi mengikuti work schedule dari SET-07 | NOT RUN | — |
| TASK-06 | P0 | Multi-assignee dengan pencarian user (nama, email, jabatan) | NOT RUN | — |
| TASK-07 | P0 | Komentar dengan @mention user/grup/followers dan lampiran; mention memicu notifikasi | NOT RUN | — |
| TASK-08A | P0 | Lampiran file dari komputer, disimpan di volume lokal lewat interface `Storage` (aturan keamanan file di SEC-01) | NOT RUN | — |
| TASK-09 | P1 | Progress project: metode (task count / duration / rata-rata), persen, indikator risiko | NOT RUN | — |
| TASK-10 | P1 | Custom field single/multi select (mis. Impact) dengan opsi berwarna; scope per space; berlaku untuk task/project/folder | NOT RUN | — |
| TASK-11A | P1 | Tipe custom field: Text, Number, Currency, Percent, Single/Multiple select, People, Date, Duration, Checkbox; simbol, desimal, separator ribuan, conditional marker, agregasi default | NOT RUN | — |
| TASK-12 | P1 | Budget project (currency field) diedit di panel detail | NOT RUN | — |
| TASK-13 | P1 | Timesheet mingguan: baris per task (pin), sel durasi per hari + komentar, total | NOT RUN | — |
| WF-01 | P0 | Workflow default tersedia saat akun dibuat | NOT RUN | — |
| WF-02 | P1 | Buat/edit workflow; status dikelompokkan ke Active, Completed, Deferred, Cancelled | NOT RUN | — |
| WF-03A | P1 | Per status: nama, warna (palet), auto-assign saat masuk status, transisi yang diizinkan (opsi lock) | NOT RUN | — |
| FORM-01 | P1 | Form builder: tipe pertanyaan (single line, paragraph, single/multiple answer, date, number, attachment, assignee, importance), multi-halaman, section header, field wajib | NOT RUN | — |
| FORM-02 | P1 | Response settings: create task/project, set status, save to location, assign to, create approval | NOT RUN | — |
| FORM-03 | P1 | Sharing form: internal (seluruh akun) atau eksternal (public link) | NOT RUN | — |
| FORM-05 | P1 | Halaman Manage forms (name, belongs to, access, link, last modified by) | NOT RUN | — |
| FORM-06 | P1 | Requester memilih form dari menu dan submit; hasilnya menjadi item sesuai response settings | NOT RUN | — |
| FORM-08 | P1 | Pelacakan request. Requester internal otomatis menjadi follower item yang dibuat dari form-nya, melihat daftar "My requests" beserta statusnya, dan hanya bisa mengakses item tersebut (bukan seluruh project) kecuali punya akses lain. Requester eksternal (public link) menerima email konfirmasi dan email saat status masuk grup Completed atau Cancelled | NOT RUN | — |
| DASH-01 | P1 | Dashboard per space/user berisi widget daftar task (to do this week, to review, completed, assigned to me) | NOT RUN | — |
| DASH-02 | P1 | Widget dari template (my tasks, overdue, followed, backlog, starred, …) atau custom (data source, reporting on, metrics, break by, group by, fields, filters) | NOT RUN | — |
| DASH-03A | P1 | Tipe visual: indicator, pie (classic/donut/ring), column, bar, line, table; pengaturan legend, label, sumbu, "Others" slice | NOT RUN | — |
| DASH-06 | P1 | Rename, duplicate (name + space), move, delete dashboard. Menghapus dashboard tidak menghapus data | NOT RUN | — |
| RPT-01 | P1 | Buat report dari template (active tasks by assignee, weekly project status, overdue, due this month, unassigned, time spent, utilization) atau custom | NOT RUN | — |
| RPT-02 | P1 | Editor report: title, location, data source (multi space/project), based on (projects / tasks / time entries), layout (table/column), filters (status, assignee, tipe, importance, effort, tanggal), group by | NOT RUN | — |
| RPT-03 | P1 | Daftar report (name, created by, space, sharing, created, updated, subscription) | NOT RUN | — |
| SET-01 | P0 | Halaman Settings dengan dua grup: Personal settings dan Account management | NOT RUN | — |
| SET-02 | P0 | Profil: nama, company, job title, negara, lokasi, departemen, telepon | NOT RUN | — |
| SET-03 | P1 | Workspace settings per user: timezone, bahasa, format tanggal/jam, awal minggu; default task (backlogged, effort mode, durasi), shortcut submit komentar. Pilihan bahasa di MVP hanya English | NOT RUN | — |
| SET-04 | P1 | Account info: nama akun, status trial, role, tanggal bergabung, subscription & seat, daftar administrator | NOT RUN | — |
| SET-05 | P1 | Preferensi email notifikasi per jenis event (mention, assign, share, report, perubahan task). Bagian email-to-task di layar ini P2 | NOT RUN | — |
| SET-07 | P1 | Work schedule: hari kerja, kapasitas harian, pengecualian tanggal (libur); dipakai oleh perhitungan durasi | NOT RUN | — |
| SET-10 | P0 | Email keluar via SMTP yang dikonfigurasi per instance (env): undangan, reset password, notifikasi, report terjadwal; dikirim lewat job queue dengan retry; template per locale | NOT RUN | — |
| USR-01 | P0 | Undang user via email + pesan opsional + user type (Admin, Regular, External, Collaborator, Viewer) | NOT RUN | — |
| USR-02 | P0 | Matriks izin per user type dan kategori (General, Sharing, Folders & Projects, Tasks, Time tracking, Collaborate, Reports, …) | NOT RUN | — |
| USR-03 | P0 | Halaman Users: kolom name, email, type, status, work schedule; grup; info seat (dipakai/pending/tersedia, batas dari env `MAX_SEATS`, dicek saat undang/aktivasi); **nonaktifkan user**: seat dibebaskan, session dicabut, item dan riwayat yang dibuatnya tetap ada | NOT RUN | — |
| USR-04 | P1 | Penghentian instance klien adalah prosedur operator, bukan tombol di UI: export data lengkap ke klien → backup akhir disimpan sesuai kontrak → stack dan volume dihapus. Tidak ada fitur hapus akun di aplikasi | NOT RUN | — |
| BIN-01 | P0 | Item yang dihapus masuk Recycle bin (daftar + filter/sort) | NOT RUN | — |
| BIN-02 | P0 | Restore item beserta subitem, field, dan riwayatnya | NOT RUN | — |
| BIN-03 | P1 | Kosongkan recycle bin (hapus permanen, dengan konfirmasi) | NOT RUN | — |
| SEC-01 | P0 | File upload: batas ukuran per file (env, default 100 MB), tolak tipe executable berdasarkan isi file (bukan hanya ekstensi), download hanya lewat endpoint yang mengecek izin atas item, header `Content-Disposition: attachment` + `X-Content-Type-Options: nosniff`. File disimpan dengan nama acak dan tidak pernah ditimpa | NOT RUN | — |
| SEC-02 | P0 | Tes negatif otorisasi: IDOR (akses item, file, dan komentar dengan ID milik orang lain), akses lintas Space, dan item multi-lokasi. Aturan multi-lokasi: akses user ke item = gabungan izin dari semua lokasi item tersebut | NOT RUN | — |
| SEC-03 | P0 | Optimistic locking: setiap item punya `version`; update dengan versi lama ditolak (HTTP 409) dan UI memuat ulang data terbaru dengan pesan konflik | NOT RUN | — |
| SEC-04 | P0 | Session: idle timeout 14 hari, batas absolut 30 hari; ID session dirotasi saat login dan saat hak akses berubah; saat ganti password semua session lain dicabut (yang sedang dipakai tetap berlaku), saat reset password atau user dinonaktifkan semua session dicabut; proteksi CSRF (cookie SameSite=Lax + cek header `Origin` pada request yang mengubah data) | NOT RUN | — |
| SEC-05 | P1 | Form publik: rate limit per IP, honeypot anti-spam, batas jumlah dan ukuran lampiran. Scan malware (ClamAV) untuk lampiran dari form publik ditunda ke P2 | NOT RUN | — |
| OPS-01 | P0 | Backup: RPO ≤ 24 jam, RTO ≤ 4 jam per klien. Database di-dump lebih dulu, lalu file. Karena file tidak pernah ditimpa (SEC-01), setiap file yang dirujuk database pasti ikut di backup. Restore penuh diuji sebelum GA dan setiap bulan setelahnya | NOT RUN | — |
| OPS-02 | P0 | Update & migrasi: script update menjalankan backup pre-deploy otomatis → pull image versi baru → start. Migrasi hanya maju (forward-only), dan setiap migrasi berjalan dalam transaksi. Jika migrasi gagal, app menolak start; pemulihan dilakukan dengan restore dump pre-deploy dan kembali ke image versi lama | NOT RUN | — |
| OPS-03 | P0 | Observability: log JSON terstruktur (`slog`) dengan request ID; metrik Prometheus (error rate, latency, panjang antrian dan job gagal di River); alert Grafana untuk error rate, job gagal, backup gagal, dan disk hampir penuh | NOT RUN | — |

## UAT M8, M9, M10 (gate G1a, G1b, G2a)

Requirement M8 sampai M10 tidak ada di tabel 93 baris di atas. Matriksnya ada di gate report; semua hasil UAT manusia **NOT RUN** (14 Nov 2026).

### UAT M8 gate G1a
[G1A_CONFIG.md](parity/gate-reports/G1A_CONFIG.md): CUST-01..05, SPC-14, TASK-11B, WF-03B/04/05, FORM-04/07/09, SET-06. Hasil UAT: NOT RUN.

### UAT M9 gate G1b
[G1B_AUTOMATION_APPROVALS.md](parity/gate-reports/G1B_AUTOMATION_APPROVALS.md): AUT-01..05, APR-01..03. Hasil UAT: NOT RUN.

### UAT M10 gate G2a
[G2A_RESOURCE_SCHEDULE_COST.md](parity/gate-reports/G2A_RESOURCE_SCHEDULE_COST.md): requirement resource, schedule dan cost M10. Hasil UAT: NOT RUN.

---

# UAT parity final (S12.2.2)

**Status seluruh bagian ini: `NOT RUN` (disiapkan 14 Nov 2026, belum ada yang menjalankan).** Paket ini dipakai manusia untuk menguji fitur parity WFP pada instance UAT. Tidak ada kolom yang boleh diisi `Lulus` kecuali penguji benar-benar melakukan langkahnya dan mengamati hasil yang tertulis. Hasil otomatis tidak menggantikan kolom ini. Daftar WFP per kelompok, status, dan bukti otomatis ada di [parity/PARITY_TEST_EVIDENCE.md](parity/PARITY_TEST_EVIDENCE.md); pemetaan WFP → PRD di [parity/WFP_TO_PRD_MATRIX.md](parity/WFP_TO_PRD_MATRIX.md).

Batas paket ini: skenario ditulis dari rute, komponen, `api/openapi.yaml`, dan spec Playwright yang ada, tanpa menjalankan aplikasinya. Nama tombol dan menu mengikuti bahasa Inggris (bahasa UI satu-satunya) dan bisa sedikit berbeda dari yang tertulis; selisih nama saja bukan temuan, selisih perilaku adalah temuan. Jalur URL yang disebut (mis. `/settings/roles`) diambil dari `web/src/routes`.

## Daftar isi
1. [Menyiapkan instance UAT](#menyiapkan-instance-uat)
2. [Prosedur pencatatan temuan](#prosedur-pencatatan-temuan)
3. Kelompok: [Work management](#kelompok-1--work-management-wfp-001015) · [Planning & views](#kelompok-2--planning--views-wfp-016030) · [Workflow & intake](#kelompok-3--workflow--intake-wfp-031044) · [Resources](#kelompok-4--resources-wfp-045052) · [Time & finance](#kelompok-5--time--finance-wfp-053059) · [Collaboration](#kelompok-6--collaboration-wfp-060069) · [Reporting](#kelompok-7--reporting-wfp-070077) · [Integration](#kelompok-8--integration-wfp-078085) · [Admin & security](#kelompok-9--admin--security-wfp-086095) · [Experience](#kelompok-10--experience-wfp-102106)
4. [Di luar 1.0 (bukan skenario)](#di-luar-10-bukan-skenario)
5. [Gate rilis baseline §12.3 (daftar periksa)](#gate-rilis-baseline-123-s1223)
6. [Yang masih butuh manusia atau akun klien](#yang-masih-butuh-manusia-atau-akun-klien)
7. [Daftar topik panduan pengguna/admin (belum ditulis)](#daftar-topik-panduan-penggunaadmin-belum-ditulis)

## Menyiapkan instance UAT

Gunakan instance terpisah dari produksi (stack Compose sendiri, database sendiri), dengan versi image yang dicatat di kolom "Versi" pada setiap lembar hasil. Rujukan: [PILOT_RUNBOOK.md](PILOT_RUNBOOK.md#bootstrap).

1. **Bootstrap** (operator): isi `deploy/.env` (tag image tetap, `PUBLIC_URL`, `MAX_SEATS` minimal 8 agar semua akun uji muat, SMTP ke kotak surat uji, `PMS_2FA_KEY`, `PMS_SSO_KEY`, `PMS_CONNECTOR_KEY`), jalankan `docker compose up -d`, lalu `docker compose exec app pms bootstrap --account 'UAT' --admin-email <email admin>`. Buka tautan set-password dari email.
2. **Akun uji** (undang lewat Settings → Users, tipe menurut kolom): `A` Admin; `R1` Regular (pemilik data uji); `R2` Regular (editor di Space Alpha, **tanpa** akses ke Space Beta); `V` Viewer; `E` External (diundang lewat sharing/email eksternal); `C` Collaborator. Tiap akun diaktifkan lewat email undangan (AUTH-06). Pakai browser atau profil browser terpisah per akun supaya sesi tidak tercampur.
3. **Data awal minimum** (dibuat oleh `A`/`R1` lewat UI, atau lewat impor Excel [SPC-15](#kelompok-8--integration-wfp-078085)): Space **Alpha** (anggota: R1 Full, R2 Editor, V Read Only) dan Space **Beta** (hanya A dan R1); Project **P1** di Alpha berisi 12 task, 3 di antaranya subitem, tanggal tersebar 4 minggu, assignee campuran R1/R2, minimal satu task *overdue*, satu milestone, dua dependensi (FS dan SS); Project **P2** di Alpha dengan 4 task; satu Project di Beta dengan 2 task (rahasia untuk uji negatif). Tiap kelompok di bawah menyebut tambahan data yang dibutuhkan.
4. **Catat** di lembar hasil: versi image, tanggal, nama penguji, browser, dan resolusi layar. Untuk skenario yang memakai email, catat kotak surat uji yang dipakai.
5. **Reset**: bila data uji rusak, operator memulihkan dump awal (`deploy/restore.sh`) yang dibuat setelah langkah 3 (`deploy/backup.sh`). Buat backup itu sebelum UAT mulai.

Skenario yang butuh sistem luar (SMTP sungguhan, IdP OIDC/SAML, repo GitHub uji, kalender Google, S3/Pydio, SCIM client) ditandai **[LUAR]**; bila sistem itu tidak tersedia, tulis `SKIP` dengan alasan, bukan `Lulus`.

## Prosedur pencatatan temuan

- **Format satu temuan** (satu baris pada tabel temuan di akhir lembar hasil, atau issue): ID `UAT-nnn`; ID skenario; WFP; langkah yang dijalankan; hasil yang diharapkan; hasil aktual; bukti (tangkapan layar, URL, waktu, akun, ID item); keparahan; status.
- **Keparahan**: **S1 Blocker**: data hilang atau bocor, akses seharusnya ditolak tetapi berhasil, 5xx berulang, fitur inti tidak bisa dipakai. **S2 Mayor**: fitur tidak berfungsi sesuai spesifikasi dan tidak ada jalan pintas. **S3 Minor**: fungsi benar, tampilan atau teks salah, ada jalan pintas. **S4 Kosmetik/saran**.
- **Hasil skenario**: `Lulus` (semua hasil diharapkan teramati), `Gagal` (ada yang tidak sesuai; wajib punya temuan), `SKIP` (tidak bisa dijalankan; wajib alasan), `NOT RUN`. Skenario dengan temuan S1/S2 terbuka tidak boleh `Lulus`.
- **Alur FIX di S12.2.2** (butir `[FIX]` di [TASKS.md](TASKS.md), tiga putaran): (1) putaran UAT penuh menghasilkan daftar temuan; (2) developer memperbaiki, menambah tes otomatis yang menangkap temuan itu bila mungkin, dan mencatat commit di tabel temuan; (3) penguji **mengulang skenario terkait** (bukan hanya langkah yang gagal) dan skenario negatif di kelompok yang sama. Maksimal tiga putaran perbaikan. Temuan S1/S2 yang masih terbuka setelah putaran ketiga **tidak** menutup S12.2.2: dieskalasi ke pemilik produk untuk keputusan (tunda rilis, atau terima sebagai risiko tertulis di [DECISIONS.md](DECISIONS.md)).
- **Kapan temuan ditutup**: S1 dan S2 ditutup hanya setelah penguji (bukan pembuat perbaikan) mengulang skenarionya dan tercatat `Lulus` dengan tanggal. S3 boleh ditutup oleh developer dengan bukti; S4 boleh ditunda dengan catatan. Temuan yang ditolak ("bukan bug") butuh persetujuan pemilik produk yang dicatat.
- **Status WFP**: WFP naik ke `ACCEPTED` di [PARITY_TEST_EVIDENCE.md](parity/PARITY_TEST_EVIDENCE.md) hanya bila semua skenario yang mencakup WFP itu `Lulus` (kolom *Skenario* pada tabel kelompok), tanpa temuan S1/S2 terbuka, dan bukti otomatisnya ada. Jangan mencentang butir UAT di TASKS.md sebelum itu.

Tabel temuan (kosong):

| ID | Skenario | WFP | Keparahan | Ringkasan | Status | Putaran | Commit/Bukti | Diverifikasi ulang oleh, tanggal |
|---|---|---|---|---|---|---|---|---|
| — | — | — | — | belum ada temuan karena UAT belum dijalankan | — | — | — | — |

Kolom hasil pada tabel skenario: **Hasil** (`NOT RUN` / Lulus / Gagal / SKIP), **Catatan**, **Tanggal**, **Penguji**.

## Kelompok 1 — Work management (WFP-001..015)

**Tujuan/aktor:** Space, project/folder, task, multi-lokasi, work schedule, progres, blueprint, template Space, tipe item kustom, custom field, formula, pencarian, starred/shared-with-me, kolaborator eksternal. Aktor: `A`, `R1`, `R2`, `V`, `E`. **Data tambahan:** data awal standar; satu blueprint belum dibuat; Space Beta berisi item berjudul unik "RAHASIA-BETA".
**Rujukan:** WFP-001..015 (WFP-009/010/012 = CUST-01/04, TASK-11B); PRD SPC-01/02/04/05/06/10/11/13/18, TASK-01/02/06/09/10/11A/11B, SET-07, NAV-01/02/06/07, CUST-01..05, USR-01. **Bukti otomatis:** baris WFP-001..015 di [PARITY_TEST_EVIDENCE.md](parity/PARITY_TEST_EVIDENCE.md); [G1A_CONFIG.md](parity/gate-reports/G1A_CONFIG.md).

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| WM-01 | R1 | Sidebar → buat Space baru dari tombol "+" (nama, deskripsi, anggota); buka | Space muncul di sidebar; tab Table/Board memuat item; Personal space tidak punya Share/rename/hapus | WFP-001; SPC-01/02 | NOT RUN | | | |
| WM-02 | R1 | Di Space, "+" → Project (nama, owner, tanggal mulai/selesai, workflow, default view) lalu "+" → Folder; masuk ke dalamnya | Pohon "Projects and folders" di sidebar menampilkan hierarki; Folder tanpa atribut proyek | WFP-002; SPC-04/05/06 | NOT RUN | | | |
| WM-03 | R1 | Di Table P1: tambah task lewat baris terakhir; buka panel detail; tambah subitem dan dua assignee (cari nama/email) | Task tampil dengan jumlah subitem; kedua assignee tampil; perubahan tersimpan setelah muat ulang | WFP-003; TASK-01/02/06 | NOT RUN | | | |
| WM-04 | R1 | Panel item → Locations → tambah P2 (atau project di Space lain); coba hapus lokasi terakhir | Item muncul di kedua lokasi; lokasi terakhir tidak bisa dihapus; menambah lokasi yang membuat siklus ditolak dengan pesan | WFP-004; SPC-10 | NOT RUN | | | |
| WM-05 | A | Settings → Work schedule: ubah hari kerja dan jam, tambah satu hari libur; buka task dengan opsi "working days only" | Jadwal tersimpan; durasi/tanggal task mengikuti hari kerja; non-admin melihat jadwal read-only | WFP-005; SET-07 | NOT RUN | | | |
| WM-06 | R1 | Project P1 → ubah metode progres (task count/duration/rata-rata); selesaikan satu task | Persen progres dan indikator risiko berubah sesuai metode | WFP-006; TASK-09 | NOT RUN | | | |
| WM-07 | R1 | Simpan task sebagai blueprint; buat item baru dari blueprint dengan tanggal mulai; buat Space dari template kategori | Struktur (subitem, status, assignee, tanggal relatif) terbawa; template membawa project, task, view, form | WFP-007/008; SPC-13/18 | NOT RUN | | | |
| WM-08 | A | Settings → Item types: buat tipe "Risk" dengan layout; "+" di Space; buka panel item bertipe Risk | Tipe muncul di menu "+" dan panel mengelompokkan blok sesuai layout; item tanpa layout memakai panel standar; non-admin tidak melihat kontrol edit | WFP-009/010; CUST-01/04 | NOT RUN | | | |
| WM-09 | R1 | Space → custom field Single select berwarna, Number, Currency, Date; isi pada task; tampilkan sebagai kolom Table; urut/filter | Nilai tersimpan, kolom tampil, filter dan urut bekerja; opsi diarsipkan tidak merusak item lama | WFP-011; TASK-10/11A | NOT RUN | | | |
| WM-10 | R1 | Buat custom field Formula yang merujuk Number; ubah nilai sumber; coba formula siklik/tidak valid | Hasil dihitung ulang otomatis; formula tidak valid atau siklus ditolak dengan pesan; kolom formula tidak bisa diedit langsung | WFP-012; TASK-11B | NOT RUN | | | |
| WM-11 | R1 | Ctrl/Cmd+K: cari kata dengan scope Everywhere, Space, Project; buka halaman hasil dan pakai filter | Hasil dikelompokkan (Tasks, Projects & Folders, Comments, Users, Request forms); scope mempersempit; kata yang dicari disorot | WFP-013; NAV-06/07 | NOT RUN | | | |
| WM-12 | R1 | Beri bintang pada item; buka Starred; minta `R2` membagikan item ke R1 lalu buka Shared with me | Starred menampilkan item; Shared with me menampilkan item yang dibagikan | WFP-014; NAV-01/02 | NOT RUN | | | |
| WM-13 | R1 | Share P1 ke `E` (email eksternal) sebagai Read Only; masuk sebagai `E` | `E` hanya melihat P1 dan isinya; peran Full/Editor/Limited/Read Only dapat dipilih per user/grup/email | WFP-015, 087; SPC-11, USR-01 | NOT RUN | | | |

**Negatif/keamanan:**

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| WM-N1 | R2 | Cari "RAHASIA-BETA"; tempel URL item Beta (dari R1) di browser | Tidak ada hasil pencarian; URL menampilkan not found/403 tanpa judul atau detail | WFP-013; SEC-02 | NOT RUN | | | |
| WM-N2 | V | Buka P1; coba edit judul, tambah task, ubah tanggal | Kontrol edit tidak ada atau aksi ditolak (403) tanpa mengubah data | WFP-001..003; SEC-02 | NOT RUN | | | |
| WM-N3 | R1 | Buka dua tab pada task yang sama; ubah judul di tab 1 lalu di tab 2 dengan data lama | Tab 2 menampilkan pesan konflik (409) dan memuat data terbaru; tidak menimpa | WFP-003; SEC-03 | NOT RUN | | | |
| WM-N4 | E | Buka URL Space Alpha lain dan item di Beta | Tidak bisa dilihat (404/403); item multi-lokasi hanya terlihat bila ada izin pada salah satu lokasinya | WFP-015; SEC-02 | NOT RUN | | | |
| WM-N5 | R2 | Pada field relasi ke item yang tak terlihat | Hanya hitungan, nama target tidak bocor (CUST-05) | WFP-009; CUST-05 | NOT RUN | | | |

## Kelompok 2 — Planning & views (WFP-016..030)

**Tujuan/aktor:** Table, Board, Gantt, dependensi, penjadwalan otomatis, baseline, portfolio, Calendar, Timelog, Files, Chart, view tersimpan, swimlane. Aktor: `R1`, `R2`, `V`, `A`. **Data tambahan:** P1 dengan dependensi FS/SS/FF/SF; P2 dengan task bertanggal; Space Alpha ≥ 50 task untuk uji gulir.
**Rujukan:** WFP-016..030; VIEW-01..10, SCH-01. **Bukti:** baris WFP-016..030 di tabel bukti; [G2A_RESOURCE_SCHEDULE_COST.md](parity/gate-reports/G2A_RESOURCE_SCHEDULE_COST.md).

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| PV-01 | R1 | Table P1: edit judul/status/tanggal inline, expand/collapse hierarki, pilih kolom (grup Management, Schedule, Other, Custom fields, Baselines) | Edit tersimpan; subitem terlipat; kolom yang dipilih tampil dan diingat | WFP-016; VIEW-01/09 | NOT RUN | | | |
| PV-02 | R1 | Board: seret kartu ke kolom status lain; buat task dari kolom | Status berubah sesuai workflow; kartu pindah; konflik mengembalikan kartu | WFP-017; VIEW-02 | NOT RUN | | | |
| PV-03 | R1 | Board → Group by Assignee; seret kartu antar lane; atur field kartu (ID, tanggal, custom field) | Swimlane per assignee (item multi-assignee muncul di tiap lane); pindah lane meminta konfirmasi lalu mengganti assignee; field kartu sesuai pilihan | WFP-029; VIEW-03/04 | NOT RUN | | | |
| PV-04 | R1 | Gantt P1: lihat bar, progres %, milestone, tiga skala waktu; seret bar dan ubah ukuran | Tanggal item berubah dan tersimpan; milestone tampil; bar tidak bergeser bila server menolak | WFP-018; VIEW-05A | NOT RUN | | | |
| PV-05 | R1 | Gantt: tambah dependensi FS, SS, FF, SF; ubah tanggal predecessor | Tautan tampil; penerus bergeser sesuai tipe dan hari kerja (dihitung server); siklus ditolak | WFP-019/020; VIEW-05B | NOT RUN | | | |
| PV-06 | R1 | Gantt: pilih baris dengan keyboard dan geser satu hari (Shift untuk tanggal selesai) | Bar bergeser; pembaca layar mengumumkan baris | WFP-018; NFR a11y | NOT RUN | | | |
| PV-07 | R1 | Buat baseline pada P1; ubah jadwal; lihat kolom/tampilan baseline dan variance; coba ubah baseline | Baseline tidak berubah saat jadwal berubah; variance sesuai; baseline tidak bisa diedit | WFP-021; VIEW-05C | NOT RUN | | | |
| PV-08 | R1 | Buka Portfolio (`/portfolio`) dengan P1 dan P2; tambah dependensi lintas project | Task beberapa project tampil dengan dependensi lintas project; dependensi yang membuat siklus ditolak | WFP-022/023; SCH-01 | NOT RUN | | | |
| PV-09 | R1 | Calendar: lihat item di tiap hari rentangnya, hari libur, daftar Unscheduled; seret item; navigasi Today/prev/next | Rentang tampil; seret memindah seluruh rentang; item tanpa tanggal mendapat satu hari | WFP-024; VIEW-06 | NOT RUN | | | |
| PV-10 | R1 | Tambah view Timelog di project berisi time entry; lihat kolom | Kolom user, tanggal, durasi, kategori, komentar, billing type, lock | WFP-025; VIEW-07 | NOT RUN | | | |
| PV-11 | R1 | Project → view Files dan Chart | Files mendaftar lampiran seluruh item di project; Chart menghitung item per status | WFP-026/027; VIEW-10 | NOT RUN | | | |
| PV-12 | R1 | Kelola view: tambah, duplicate (Public/Personal), rename, urut, set default, hide, salin tautan view, hapus; pakai preset filter (All tasks, All active, My active) dan sort | Perubahan tersimpan; tautan membuka view dan state-nya; Personal hanya terlihat pembuat | WFP-028, 016; VIEW-08, SPC-03/17 | NOT RUN | | | |
| PV-13 | R1 | Space dengan 500+ item (impor Excel): gulir Table; filter dan urut | Tabel tetap responsif (< 1,5 s muat); filter/urut memberi hasil benar. Catatan: filter cepat khusus Table baru sebagian (WFP-030 `VERIFY`) | WFP-030; VIEW-01, NAV-06 | NOT RUN | | | |

**Negatif/keamanan:**

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| PV-N1 | V | Gantt/Calendar/Board: coba seret | Tidak bisa; Gantt memberi tahu hanya-lihat; tidak ada perubahan data | WFP-017/018/024 | NOT RUN | | | |
| PV-N2 | R2 | Portfolio: apakah task dari project yang tidak boleh dilihat muncul | Task tak berizin tidak tampil dan tidak membocorkan judul pada dependensi lintas project | WFP-022/023; SEC-02 | NOT RUN | | | |
| PV-N3 | R2 | View Files/Chart pada project tempat R2 hanya melihat sebagian item | Hanya lampiran dan hitungan dari item yang boleh dilihat | WFP-026/027; VIEW-10 | NOT RUN | | | |
| PV-N4 | R1 | Dua tab: pindahkan task yang sama di Gantt dari kedua tab | Tab kedua menerima konflik dan bar kembali ke posisi server | WFP-018; SEC-03 | NOT RUN | | | |

## Kelompok 3 — Workflow & intake (WFP-031..044)

**Tujuan/aktor:** workflow dan status, guard transisi, approval (termasuk Any/All), form (internal/publik, bercabang), My requests, email konfirmasi, mesin otomasi. Aktor: `A`, `R1`, `R2`, `E`, pengirim form publik anonim. **Data tambahan:** kotak surat uji untuk email; dua approver (R1, R2); form dengan pertanyaan bercabang; Blueprint dan tipe item kustom dari WM-07/WM-08.
**Rujukan:** WFP-031..043 (WFP-044 di luar 1.0); WF-01..05, SPC-12, APR-01..03, FORM-01..09, AUT-01..05. **Bukti:** [G1A_CONFIG.md](parity/gate-reports/G1A_CONFIG.md), [G1B_AUTOMATION_APPROVALS.md](parity/gate-reports/G1B_AUTOMATION_APPROVALS.md).

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| WF-S01 | A | Settings → Workflows: buat workflow, tambah status di grup Active/Completed/Deferred/Cancelled, atur warna, urut, kunci transisi; hapus status yang dipakai | Status dan transisi tersimpan; hapus status terpakai meminta tujuan pindah item; transisi terlarang ditolak | WFP-031; WF-01/02/03A | NOT RUN | | | |
| WF-S02 | A | Di status "Review": syarat wajib field dan aktor; coba pindah item tanpa field itu (UI dan lewat Table) | Pindah ditolak dengan pesan, data item tidak berubah | WFP-032; WF-03B/04/05 | NOT RUN | | | |
| WF-S03 | A | Atur "masuk status Review memulai approval" dengan hasil → status tujuan; pindah item; approver memutuskan | Approval terbuat otomatis; setelah keputusan item pindah ke status tujuan | WFP-033; WF-03B | NOT RUN | | | |
| WF-S04 | R1 | Panel item → Approvals: buat approval dengan dua approver, kebijakan Any lalu All, due date | Penghitung 0/2 → 1/2; Any selesai pada satu persetujuan, All butuh keduanya; reject + komentar | WFP-034; SPC-12, APR-01 | NOT RUN | | | |
| WF-S05 | R1 | Reassign approver; lihat riwayat keputusan; edit item setelah keputusan | Reassign tercatat; riwayat menampilkan siapa memutuskan apa terpaku pada versi item; riwayat tidak bisa diubah; pengingat/eskalasi sesuai pengaturan [LUAR: email] | WFP-035; APR-02/03 | NOT RUN | | | |
| WF-S06 | R1 | Space → Forms: buat form (single line, paragraph, pilihan, date, number, attachment, assignee, importance), multi-halaman, section header, wajib; atur response settings (buat task/project, status, lokasi, assign, approval); preview | Builder menyimpan; preview menampilkan halaman demi halaman tanpa submit | WFP-036; FORM-01/02 | NOT RUN | | | |
| WF-S07 | R1 | Share form: internal, lalu public link; buka link di jendela tanpa login dan kirim; isi honeypot tidak terlihat | Form publik terkirim, tanda terima tampil; item terbentuk sesuai settings; halaman Manage forms (`/forms`) menampilkan nama, access, link | WFP-037; FORM-03/05/06 | NOT RUN | | | |
| WF-S08 | R1 | Pertanyaan hanya tampil bila jawaban sebelumnya X; kirim dengan cabang berbeda; atur routing tipe item/blueprint per cabang | Hanya pertanyaan cabang yang tampil dan wajib; jawaban basi tidak terkirim; item bertipe dan berfield sesuai cabang | WFP-038/039; FORM-07/09 | NOT RUN | | | |
| WF-S09 | R2 | Kirim request lewat Requests; buka "My requests" (`/requests/mine`) | Daftar request dengan status teks; R2 jadi follower; hanya bisa membuka item itu | WFP-040; FORM-08 | NOT RUN | | | |
| WF-S10 | R1 | Atur teks email konfirmasi form (dengan karakter HTML); kirim sebagai pengirim publik; ubah status item ke Completed [LUAR: SMTP] | Email konfirmasi sesuai teks dan HTML di-escape; email status Completed/Cancelled terkirim ke requester eksternal | WFP-041; FORM-04/08 | NOT RUN | | | |
| WF-S11 | R1 | Space → Automation (`/spaces/<id>/automation`): buat aturan WHEN/IF/THEN, "Try it" pada item, publish, ubah item pemicu; matikan aturan | Aksi berjalan sekali; uji coba tidak mengubah data; aturan nonaktif berhenti | WFP-042; AUT-01/02/05 | NOT RUN | | | |
| WF-S12 | R1 | Buka log aturan; filter hasil; buat aturan yang bisa mengulang (loop); lihat event gagal dan replay | Log menampilkan tiap hasil dengan item; loop berhenti oleh batas; replay tidak menggandakan efek; kuota terlihat | WFP-043; AUT-03/04 | NOT RUN | | | |

**Negatif/keamanan:**

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| WF-N1 | R2 | Buka `/settings/workflows` | Daftar read-only, tanpa kontrol edit; PUT langsung ditolak 403 | WFP-031 | NOT RUN | | | |
| WF-N2 | V | Panel item: Approvals/komentar/ubah status | Tidak ada Add/Edit/Delete; keputusan oleh non-approver ditolak | WFP-034/035 | NOT RUN | | | |
| WF-N3 | Anonim | Buka link form publik yang dicabut; kirim berulang cepat | "Unavailable" untuk link dicabut; rate limit memberi pesan; lampiran melebihi batas ditolak | WFP-037; SEC-05 | NOT RUN | | | |
| WF-N4 | R2 | Buka item hasil form orang lain lewat URL | Tidak bisa diakses (hanya item sendiri dari form) | WFP-040; FORM-08, SEC-02 | NOT RUN | | | |
| WF-N5 | R2 | Aturan otomasi milik R1 mengubah item di Space tanpa hak R1 | Aksi tanpa izin pemilik aturan ditolak dan tercatat di log | WFP-042; AUT-03 | NOT RUN | | | |

## Kelompok 4 — Resources (WFP-045..052)

**Tujuan/aktor:** effort dan allocation, workload per tim/orang, kapasitas dan pengecualian, realokasi dan konflik, job role/skill, booking, utilization. Aktor: `A`, `R1`, `R2`. **Data tambahan:** 5 orang uji dengan work schedule berbeda (satu cuti), role/skill di Settings → Resources (`/settings/resources`), task ber-effort dan multi-assignee, satu orang sengaja over-allocated.
**Rujukan:** WFP-045..052; TASK-05B, SET-07, RSC-01..06. **Bukti:** [G2A_RESOURCE_SCHEDULE_COST.md](parity/gate-reports/G2A_RESOURCE_SCHEDULE_COST.md).

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| RS-01 | R1 | Panel item → Effort: isi effort dan allocation per assignee; ubah mode effort | Durasi mengikuti work schedule; effort tanpa tanggal tetap bisa disimpan | WFP-045; TASK-05B | NOT RUN | | | |
| RS-02 | A | Settings → Work schedule dan jadwal pribadi: atur kapasitas harian, cuti, libur | Kapasitas orang berkurang pada hari cuti/libur di Workload | WFP-048; SET-07, RSC-01 | NOT RUN | | | |
| RS-03 | R1 | Buka Workload (`/workload`): pilih periode, tim/orang | Beban per orang benar untuk task multi-assignee dan multi-lokasi (tiap lokasi tidak dihitung ganda); over-allocation ditandai | WFP-046/047; RSC-02 | NOT RUN | | | |
| RS-04 | R1 | Realokasi task over-allocated ke orang lain dari Workload | Beban kedua orang berubah segera; perubahan tercatat di activity; pengamat lain melihat pembaruan tanpa muat ulang | WFP-049; RSC-03 | NOT RUN | | | |
| RS-05 | A | Beri job role/skill ke orang; filter Workload per role/skill | Hanya orang dengan role/skill itu tampil | WFP-050; RSC-04 | NOT RUN | | | |
| RS-06 | R1 | Buat booking resource pada rentang tanggal yang bentrok | Booking masuk ke Workload; konflik terlihat | WFP-051; RSC-05 | NOT RUN | | | |
| RS-07 | R1 | Tabel utilization: bandingkan planned vs actual (setelah time entry) lalu cocokkan dengan hitungan manual dari time entry mentah | Angka cocok dengan data mentah | WFP-052; RSC-06 | NOT RUN | | | |

**Negatif/keamanan:**

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| RS-N1 | V | Buka Workload; coba realokasi | Tidak ada aksi ubah; hanya data yang boleh dilihat | WFP-046/049 | NOT RUN | | | |
| RS-N2 | R2 | Workload: apakah task di Beta (tanpa akses) ikut menghitung beban atau namanya tampil | Judul task tak berizin tidak tampil | WFP-046; SEC-02 | NOT RUN | | | |
| RS-N3 | R2 | Settings → Resources (role/skill) dan jadwal orang lain | Hanya admin bisa mengubah; 403 untuk lainnya | WFP-050 | NOT RUN | | | |

## Kelompok 5 — Time & finance (WFP-053..059)

**Tujuan/aktor:** time entry, timesheet mingguan, timelog lock, budget, billable, tarif, biaya. Aktor: `A`, `R1`, `R2`. **Data tambahan:** tarif belum ada; project P1 dengan billing type; time entry beberapa hari.
**Rujukan:** WFP-053..059; TASK-12/13/14, VIEW-07. **Bukti:** G2A (atas).

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| TF-01 | R1 | Panel item → catat time entry (user, tanggal, durasi, kategori, komentar, billable) | Entry tampil di Timelog project dan total benar | WFP-053; TASK-13 | NOT RUN | | | |
| TF-02 | R1 | Timesheets (`/timesheets`): pin task, isi durasi per hari dan komentar | Total harian/mingguan benar; tersimpan; konflik mempertahankan draf sel | WFP-054; TASK-13 | NOT RUN | | | |
| TF-03 | A | Timelog: kunci (lock) entry lalu coba ubah sebagai R1 | Entry terkunci tidak bisa diubah/dihapus pemilik | WFP-055; VIEW-07 | NOT RUN | | | |
| TF-04 | R1 | Panel project: isi budget (currency) | Budget tersimpan dan tampil dengan simbol dan desimal | WFP-056; TASK-12 | NOT RUN | | | |
| TF-05 | A | Settings → Rates (`/settings/rates`): tambah tarif per orang; ubah tarif | Entry lama tetap memakai tarif lama (histori tidak berubah); entry baru memakai tarif baru | WFP-057/058; TASK-14 | NOT RUN | | | |
| TF-06 | A | Lihat biaya aktual vs rencana di project | Angka sama dengan time entry × tarif yang berlaku saat itu | WFP-059; TASK-14 | NOT RUN | | | |

**Negatif/keamanan:**

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| TF-N1 | R2 | Lihat/ubah time entry R1 dan tarif | Entry orang lain tidak bisa diubah; tarif/biaya tidak tampil bagi non-admin | WFP-053/058 | NOT RUN | | | |
| TF-N2 | V | Catat time entry | Ditolak sesuai matriks izin | WFP-053; USR-02 | NOT RUN | | | |
| TF-N3 | Anonim | Dashboard publik berisi widget biaya (lihat RP-07) | Data uang tidak tampil | WFP-059, 075 | NOT RUN | | | |

## Kelompok 6 — Collaboration (WFP-060..069)

**Tujuan/aktor:** komentar dan mention, inbox/stream, lampiran, preview file, versi file, proofing, perbandingan versi, approval file dengan reviewer eksternal, storage eksternal. Aktor: `R1`, `R2`, `E` (reviewer), `V`. **Data tambahan:** dua file contoh (PDF/gambar, 2 versi), file executable palsu (.exe) untuk uji tolak; [LUAR] bucket S3/Pydio uji.
**Rujukan:** WFP-060..068 (WFP-069 di luar 1.0); TASK-07/08A/08B, NAV-03/04, FIL-01..04, SEC-01. **Bukti:** [G2B_FILES_ANALYTICS_INTEGRATION.md](parity/gate-reports/G2B_FILES_ANALYTICS_INTEGRATION.md).

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| CL-01 | R1 | Komentar di task dengan @mention R2 dan lampiran | R2 menerima notifikasi di Inbox dengan penanda belum dibaca; komentar tampil real-time di sesi R2 | WFP-060; TASK-07 | NOT RUN | | | |
| CL-02 | R2 | Inbox: Mark all as read, Archive all; Stream dengan filter (Everything, Followed, Assigned to me, My conversations) dan "Hide auto updates" | Penanda dan filter bekerja | WFP-061; NAV-03/04 | NOT RUN | | | |
| CL-03 | R1 | Lampirkan file dari komputer; unduh; coba file .exe | Unduh lewat endpoint berizin dengan `Content-Disposition: attachment`; executable ditolak berdasarkan isi | WFP-062; TASK-08A, SEC-01 | NOT RUN | | | |
| CL-04 | R1 | Buka berkas gambar/PDF dari panel item; buka view Files project | Pratinjau tampil; Files mendaftar lampiran | WFP-063; VIEW-10 | NOT RUN | | | |
| CL-05 | R1 | Unggah versi baru file yang sama; lihat riwayat versi; buka versi lama | Versi lama tidak berubah dan tetap bisa diunduh; riwayat berurutan | WFP-064; FIL-01 | NOT RUN | | | |
| CL-06 | R1 | Buka proofing: beri anotasi pada posisi tertentu di v1; ganti ke v2 | Anotasi tetap di v1 dan tidak muncul di v2 | WFP-065; FIL-02 | NOT RUN | | | |
| CL-07 | R1 | Compare: pilih dua versi, lihat berdampingan | Kedua versi tampil berdampingan | WFP-066; FIL-03 | NOT RUN | | | |
| CL-08 | R1 | Minta file review pada v1 ke `E` (tanpa akses item); `E` memutuskan lewat tautan/halaman reviews (`/reviews`); unggah v2 | Keputusan terpaku pada v1; v2 butuh review sendiri; E hanya melihat file itu | WFP-067; FIL-04 | NOT RUN | | | |
| CL-09 | A | [LUAR] Konfigurasi storage eksternal (S3/Pydio) dan lampirkan file; juga lampiran tautan | File tersimpan di sisi eksternal tetapi unduh tetap lewat izin PMS; tautan membuka alamatnya tanpa kontrol versi | WFP-068; TASK-08B | NOT RUN | | | |

**Negatif/keamanan:**

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| CL-N1 | R2 | Unduh/ubah lampiran/komentar item Beta lewat ID tebakan | 404/403 tanpa bocor nama berkas | WFP-062; SEC-02 | NOT RUN | | | |
| CL-N2 | V | Komentar, unggah versi, anotasi | Ditolak sesuai peran | WFP-060/064/065 | NOT RUN | | | |
| CL-N3 | E | Dengan tautan review, buka file lain atau item induknya | Tidak terlihat; hanya file yang diminta | WFP-067; SEC-02 | NOT RUN | | | |
| CL-N4 | R1 | Unggah file melebihi batas ukuran (env) | Ditolak dengan pesan; tidak ada file tersisa | WFP-062; SEC-01 | NOT RUN | | | |

## Kelompok 7 — Reporting (WFP-070..077)

**Tujuan/aktor:** dashboard dan widget, report, langganan terjadwal, widget area/pivot/analitik, papan analitik, dashboard publik, portfolio reporting. Aktor: `A`, `R1`, `R2`, `V`, anonim. **Data tambahan:** ≥ 50 task dengan status/assignee beragam, time entry, tarif (agar ada data uang); angka mentah dihitung manual untuk pembanding.
**Rujukan:** WFP-070..077; DASH-01..08, RPT-01..04. **Bukti:** G2B (atas).

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| RP-01 | R1 | Dashboards (`/dashboards`): buat dashboard; tambah widget daftar task (template) dan widget custom (data source, metrics, group by, filter) | Widget menampilkan angka; konfigurasi tersimpan | WFP-070; DASH-01/02 | NOT RUN | | | |
| RP-02 | R1 | Tipe visual: indicator, pie (classic/donut/ring), column, bar, line, table; legend, label, Others slice | Semua tipe tampil benar dengan label; slice kecil terlipat ke Others | WFP-070; DASH-03A | NOT RUN | | | |
| RP-03 | R1 | Rename, duplicate (nama + space), move, hapus dashboard | Hapus tidak menghapus data task | WFP-070; DASH-06 | NOT RUN | | | |
| RP-04 | R1 | Reports (`/reports`): buat dari template (mis. overdue, unassigned, time spent, utilization) dan custom; jalankan, filter, group by; ekspor | Daftar report punya kolom RPT-03; hasil cocok dengan hitungan manual; ekspor terunduh dan sel diawali rumus aman | WFP-071, 077; RPT-01..03 | NOT RUN | | | |
| RP-05 | R1 | Atur langganan report harian pada jam dan zona waktu tertentu [LUAR: SMTP] | Email tiba pada jam terpilih, sekali per slot, dengan hak pelanggan | WFP-072; RPT-04 | NOT RUN | | | |
| RP-06 | R1 | Tambah widget area, pivot, burndown, completion; tambah dari pustaka ke papan analitik Space; ubah layout satu kolom; deskripsi widget | Angka cocok dengan data mentah; layout dan deskripsi tersimpan | WFP-073/074; DASH-03B/04/05/08 | NOT RUN | | | |
| RP-07 | R1 | Aktifkan link publik dashboard (kedaluwarsa + password); buka di jendela tanpa login; matikan link | Link terlihat sekali; password diminta; widget uang tersembunyi; link kedaluwarsa/mati menampilkan "unavailable" | WFP-075; DASH-07 | NOT RUN | | | |
| RP-08 | R1 | Report lintas Space/project (data source multi) berbasis projects/tasks/time entries | Hasil gabungan benar | WFP-077; RPT-02 | NOT RUN | | | |

**Negatif/keamanan:**

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| RP-N1 | R2 | Report/dashboard dengan sumber Space Beta | Data Beta tidak masuk hasil; sumber terlalu besar dijelaskan, bukan 5xx | WFP-070/071; SEC-02 | NOT RUN | | | |
| RP-N2 | V | Edit dashboard/report | Tidak ada kontrol edit; PUT 403 | WFP-070/071 | NOT RUN | | | |
| RP-N3 | Anonim | Tebak token dashboard publik; token yang sudah dicabut | Selalu "unavailable", tanpa membedakan tidak ada vs kedaluwarsa | WFP-075 | NOT RUN | | | |
| RP-N4 | R1 | Berbagi dashboard ke R2 yang tidak punya hak pada sumber data | R2 melihat hanya data yang boleh ia lihat | WFP-070; SEC-02 | NOT RUN | | | |

## Kelompok 8 — Integration (WFP-078..085)

**Tujuan/aktor:** impor/ekspor Excel, API key dan OAuth, webhook, konektor GitHub/Google, orkestrasi otomasi. Aktor: `A`, `R1`, `R2`. **Data tambahan:** [LUAR] penerima webhook (mis. layanan penangkap request), repo GitHub uji dan akun Google uji, `PMS_CONNECTOR_KEY` terisi, `curl`/klien HTTP untuk uji API.
**Rujukan:** WFP-078..083 (WFP-084/085 di luar 1.0); SPC-15, API-01..03, INT-01..03, SET-08. **Bukti:** G2B (atas); [S11_3_RECONCILIATION.md](parity/S11_3_RECONCILIATION.md) memuat catatan implementasi pilihan. Konektor dengan akun nyata: *bukti awal dari pengguna* (bukan hasil UAT; kolom Hasil tidak diubah): GitHub Issues dipasang di instance dev pada 14 Nov 2026, satu sinkron "pushed 1, pulled 2, conflicts 0". Konflik, GitHub→PMS status, webhook, dan Google Calendar belum dicoba.

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| IN-01 | R1 | Project → Export ke Excel; ubah berkas; Import: petakan kolom tak dikenal, tinjau pratinjau, impor | Ekspor selesai (polling) dan terunduh; hanya baris tervalidasi terakhir yang diimpor | WFP-078; SPC-15 | NOT RUN | | | |
| IN-02 | R1 | Settings → Apps → API keys: buat key (scope baca), salin rahasia; panggil API dengan key; coba menulis; rotasi; cabut | Rahasia tampil sekali; baca berhasil, tulis ditolak; setelah rotasi key lama gagal; setelah cabut gagal | WFP-079; API-01 | NOT RUN | | | |
| IN-03 | R1 | Authorized apps / OAuth clients (admin): buat klien, jalankan alur authorize (`/oauth/authorize`) dengan PKCE; cabut | Layar persetujuan tampil; token berfungsi; setelah cabut ditolak; permintaan tidak valid ditolak | WFP-079; API-03 | NOT RUN | | | |
| IN-04 | R1 | [LUAR] Webhooks: buat webhook ke penerima, simpan secret; picu event (ubah item); verifikasi tanda tangan dan timestamp di penerima; log pengiriman | Penerima mendapat event bertanda tangan sah; log menampilkan hasil; jeda/aktifkan | WFP-080; API-02 | NOT RUN | | | |
| IN-05 | R1 | [LUAR] Matikan penerima webhook | Retry dengan backoff lalu webhook otomatis nonaktif; ID delivery sama antar retry | WFP-080; API-02 | NOT RUN | | | |
| IN-06 | A | [LUAR] Connectors: pasang GitHub Issues (repo uji); impor issue; ubah issue dan task di kedua sisi; buat konflik; selesaikan | Sinkron dua arah; konflik dicatat dan tidak menimpa diam-diam; status sinkron terakhir tampil | WFP-081/082; INT-01/02/03 | NOT RUN | | | |
| IN-07 | A | [LUAR] Pasang Google Calendar; ubah acara di Google dan di PMS; nonaktifkan konektor | Satu arah: perubahan di Google masuk, tidak menulis balik; hanya acara yang boleh dilihat pembuat tampil; nonaktif menghentikan sinkron | WFP-081; INT-01/03 | NOT RUN | | | |
| IN-08 | R1 | Aturan otomasi dengan aksi memakai konektor/webhook (bila tersedia pada pemicu) | Aksi berjalan sekali dan tercatat di log | WFP-083; INT-01, AUT-02 | NOT RUN | | | |
| IN-09 | R1 | Space → bookmarks: simpan URL; kirim feedback dari menu profil | Bookmark terbuka di tab baru; URL non-http ditolak; feedback terkirim dengan ucapan terima kasih | NAV-08/09 (pendukung) | NOT RUN | | | |

**Negatif/keamanan:**

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| IN-N1 | R1 | Webhook ke alamat internal (127.0.0.1, 10.x, metadata cloud) | Ditolak (SSRF); tidak ada permintaan keluar | WFP-080; API-02 | NOT RUN | | | |
| IN-N2 | R1 | Pakai API key/OAuth ber-scope baca untuk menulis; pakai token OAuth untuk membuat API key atau memanggil `/auth` | Ditolak (403/401) | WFP-079; API-01/03 | NOT RUN | | | |
| IN-N3 | R2 | API key milik R1 dipakai sebagai R2; daftar key orang lain | Tidak terlihat; key hanya milik pembuat; maksimal key per orang ditegakkan | WFP-079 | NOT RUN | | | |
| IN-N4 | Anonim | POST ke webhook masuk GitHub tanpa/dengan tanda tangan salah | Ditolak; hanya tanda tangan sah diterima | WFP-081; INT-03 | NOT RUN | | | |
| IN-N5 | R2 | Pasang konektor (non-admin) dan baca rahasia konektor lewat UI/API | Ditolak; rahasia tidak pernah tampil | WFP-081; INT-01 | NOT RUN | | | |
| IN-N6 | R1 | Impor Excel berisi rumus/format berbahaya atau berkas rusak | Rumus diperlakukan sebagai teks; berkas rusak ditolak tanpa 5xx | WFP-078; SPC-15 | NOT RUN | | | |

## Kelompok 9 — Admin & security (WFP-086..095)

**Tujuan/aktor:** user/grup/seat, sharing dan izin, 2FA, SSO/SAML/SCIM, role kustom, audit, locked space, IP allowlist, OTP login, kepemilikan akun, backup/ekspor/penghentian. Aktor: `A`, `R1`, `R2`, `V`, `E`, `C`. **Data tambahan:** aplikasi autentikator TOTP; [LUAR] IdP OIDC dan SAML uji, klien SCIM uji; akses shell operator untuk backup/restore; `MAX_SEATS` diatur agar batas bisa dicapai.
**Rujukan:** WFP-086..093, 095 (WFP-094 di luar 1.0); USR-01..04, SPC-11, AUTH-05/08, ENT-01..05, SET-09, OPS-01/02. **Bukti:** [G0_STABILIZATION.md](parity/gate-reports/G0_STABILIZATION.md) dan baris WFP-086..095 di tabel bukti.

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| AD-01 | A | Settings → Users: undang user per tipe (Admin, Regular, External, Collaborator, Viewer) dengan pesan; buat grup; lihat info seat | Undangan terkirim [LUAR: SMTP]; kolom name/email/type/status/work schedule; seat dipakai/pending/tersedia sesuai `MAX_SEATS` | WFP-086; USR-01/03 | NOT RUN | | | |
| AD-02 | A | Nonaktifkan R2; coba masuk sebagai R2; aktifkan lagi | Seat dibebaskan; sesi R2 dicabut; item dan riwayat R2 tetap ada | WFP-086; USR-03, SEC-04 | NOT RUN | | | |
| AD-03 | A | Undang user ketika seat habis | Ditolak dengan pesan batas seat | WFP-086; USR-03 | NOT RUN | | | |
| AD-04 | A | Sharing Space/item per peran (Full, Editor, Limited, Read Only) ke user, grup, email eksternal; turunkan hak sendiri (konfirmasi) | Peran dihormati di semua tampilan; hak turun meminta konfirmasi | WFP-087; SPC-11 | NOT RUN | | | |
| AD-05 | R1 | Settings → Security: aktifkan 2FA (QR + secret, kode 6 digit), simpan kode cadangan; keluar lalu masuk | Login meminta kode kedua; kode cadangan sekali pakai bekerja; kode salah ditolak; ada peringatan bila kode cadangan menipis | WFP-088; AUTH-05 | NOT RUN | | | |
| AD-06 | A | Reset 2FA milik R1 | R1 bisa masuk tanpa 2FA lalu menyiapkan ulang | WFP-088 | NOT RUN | | | |
| AD-07 | A | [LUAR] SSO: Settings → SSO konfigurasi OIDC lalu SAML (redirect URI dan metadata tampil); masuk lewat IdP; tautkan/lepas identitas; login password tetap jalan | Masuk berhasil untuk user aktif yang sudah dikenal; rahasia tidak tampil ulang | WFP-089; ENT-01 | NOT RUN | | | |
| AD-08 | A | [LUAR] Provisioning (`/settings/provisioning`): buat token SCIM (tampil sekali); klien SCIM buat, ubah, nonaktifkan user dan grup | User dibuat/dinonaktifkan; deprovision mencabut sesi dan seat; berhenti di batas seat | WFP-089; ENT-02 | NOT RUN | | | |
| AD-09 | A | Settings → Roles: buat role kustom dengan hak terbatas; pakai di dialog Share; coba hapus role yang dipakai | Role tidak melebihi batas tipe user; hapus role terpakai ditolak | WFP-090; ENT-03, USR-02 | NOT RUN | | | |
| AD-10 | A | Settings → Audit: lihat jejak (login, perubahan hak, token, SSO), filter, ekspor | Peristiwa keamanan tercatat dengan pelaku dan waktu; ekspor aman dari rumus | WFP-091; ENT-04 | NOT RUN | | | |
| AD-11 | A | Kunci sebuah Space dari menu Space; buka sebagai R1 lalu buka kunci | Hanya admin mengakses saat terkunci; tercatat di audit | WFP-092; ENT-05 | NOT RUN | | | |
| AD-12 | A | Settings → Policy: aktifkan IP allowlist (tambah alamat sendiri); coba dari alamat lain [LUAR]; aktifkan login OTP | Alamat luar ditolak pada login dan seluruh permintaan; peringatan jika mengunci diri sendiri; OTP via email sekali pakai | WFP-093; ENT-05, AUTH-08 | NOT RUN | | | |
| AD-13 | A | Settings → Ownership: pindahkan kepemilikan ke admin lain (password) | Pemilik baru; admin lama tetap admin; kesalahan password dijelaskan | WFP-086; SET-09 | NOT RUN | | | |
| AD-14 | Operator | Jalankan `deploy/backup.sh`, hapus stack dan volume pada instance uji, `deploy/restore.sh`; lalu `deploy/export.sh` | Data identik setelah restore; ekspor lengkap untuk klien; tidak ada tombol hapus akun di UI | WFP-095; USR-04, OPS-01 | NOT RUN | | | |
| AD-15 | Operator | Update dengan `deploy/update.sh` ke tag baru (rehearsal) | Backup pra-deploy dibuat; readiness hijau; data utuh | WFP-095; OPS-02 | NOT RUN | | | |

**Negatif/keamanan:**

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| AD-N1 | R2, V | Buka `/settings/users`, `/settings/roles`, `/settings/policy`, `/settings/audit`, `/settings/sso`, `/settings/provisioning`, `/settings/ownership` | Diberitahu admin-only atau tidak ada menu; panggilan API langsung 403 | WFP-086..093 | NOT RUN | | | |
| AD-N2 | R2 | Ganti password R2 lalu cek sesi lain; reset password via email | Sesi lain dicabut saat ganti password (yang aktif tetap); semua sesi dicabut saat reset | SEC-04 | NOT RUN | | | |
| AD-N3 | Anonim | Tebak token SCIM/undangan/OTP; kode OTP salah 5 kali | Ditolak; kode hangus setelah 5 kesalahan dan kedaluwarsa; SCIM dibatasi laju | WFP-089; ENT-02, AUTH-08 | NOT RUN | | | |
| AD-N4 | R2 | Ubah data dengan header Origin asing (CSRF) | Ditolak | SEC-04 | NOT RUN | | | |
| AD-N5 | R2 | Akses item dan Space terkunci milik Space yang tidak dibagikan (ID tebakan) | 404/403 tanpa bocor | WFP-087/092; SEC-02 | NOT RUN | | | |
| AD-N6 | A | Pemilik akun dinonaktifkan/dilepas peran adminnya | Ditolak; pemilik terlindungi | WFP-086; SET-09 | NOT RUN | | | |

## Kelompok 10 — Experience (WFP-102..106)

**Tujuan/aktor:** responsif dan aksesibilitas, tema terang/gelap/warna, lokalisasi. Aktor: `R1`, `V`. Perangkat: laptop, tablet/layar sempit, pembaca layar (VoiceOver) atau ekstensi axe. **Data tambahan:** data awal standar.
**Rujukan:** WFP-102..104 (WFP-105/106 di luar 1.0); SET-03, SET-06, NFR aksesibilitas. **Bukti:** [UI_AUDIT_RETEST_REPORT.md](UI_AUDIT_RETEST_REPORT.md), [UI_AUDIT_REPORT.md](UI_AUDIT_REPORT.md), spec `*.a11y.spec.ts`. Catatan: baseline mencatat skor UI 64/100 dengan P1 terbuka; cek status audit ulang sebelum menilai.

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| EX-01 | R1 | Buka layar utama (Table, Board, Gantt, Calendar, Inbox, Settings) pada lebar 375, 768, 1280 px | Tidak ada gulir horizontal halaman; konten dan aksi terjangkau; Calendar menggulir lokal | WFP-102 | NOT RUN | | | |
| EX-02 | R1 | Navigasi hanya dengan keyboard: sidebar, Ctrl/Cmd+K, dialog (Tab, Esc), Table, Gantt | Fokus terlihat, urutan masuk akal, dialog menjebak dan mengembalikan fokus | WFP-102 | NOT RUN | | | |
| EX-03 | R1 | Pembaca layar pada Login, Table, dialog Share, Gantt | Nama kontrol dan status diumumkan; status tidak hanya lewat warna | WFP-102 | NOT RUN | | | |
| EX-04 | R1 | Profil/Settings: pilih Light, Dark, System dan tema warna; muat ulang | Tema langsung berlaku, tersimpan di profil, kontras terbaca di semua layar | WFP-103; SET-06 | NOT RUN | | | |
| EX-05 | R1 | Workspace settings: timezone, format tanggal/jam, awal minggu, bahasa | Pengaturan tersimpan dan dipakai (tanggal, Calendar); pilihan bahasa hanya English | WFP-104; SET-03 | NOT RUN | | | |
| EX-06 | R1 | Layar kosong, memuat, dan error (matikan jaringan sejenak) | Ada keadaan kosong dengan tindakan lanjut, indikator memuat, dan pesan error dengan ulang | WFP-102 | NOT RUN | | | |

**Negatif:**

| ID | Peran | Langkah | Hasil yang diharapkan | Mencakup | Hasil | Catatan | Tanggal | Penguji |
|---|---|---|---|---|---|---|---|---|
| EX-N1 | R1 | Perbesar teks 200% dan zoom 400% di layar utama | Tidak ada konten terpotong yang tidak bisa dijangkau | WFP-102 | NOT RUN | | | |
| EX-N2 | R1 | Mode kontras tinggi/pengurangan gerak OS | Animasi dikurangi; konten terbaca | WFP-102, 103 | NOT RUN | | | |

## Di luar 1.0 (bukan skenario)

WFP berikut berstatus `N/A_APPROVED` (M13, menunggu persetujuan pemilik produk lewat DEC-PAR-01/05/06; lihat [WFP_TO_PRD_MATRIX.md](parity/WFP_TO_PRD_MATRIX.md)) dan **tidak diuji**: WFP-044 (solution template bundle), WFP-069 (editor dokumen kolaboratif), WFP-084/085 (Datahub/sinkron data eksternal), WFP-094 (manajemen kunci enterprise), WFP-096..099 (AI, MCP), WFP-100/101 (Whiteboard), WFP-105 (aplikasi native), WFP-106 (offline/push). Penguji tidak boleh menandainya `GAP` sebagai temuan; yang perlu dicek hanya bahwa pemilik produk memang menyetujui pengecualiannya (lihat gate di bawah).

## Gate rilis baseline §12.3 (S12.2.3)

Kerangka butir `[REVIEW] Gate rilis baseline §12.3` di [TASKS.md](TASKS.md). Sumber kriteria: [baseline §12.3](PMS_WRIKE_PARITY_DEVELOPMENT_BASELINE.md#123-release-10-gate-target-full-feature-parity). **Tidak ada kriteria yang boleh dicentang di sini**: kolom *Status* mencatat keadaan jujur pada 14 Nov 2026; yang berisi UAT, sign-off, atau akun klien tetap terbuka.

| # | Kriteria gate | Sumber bukti | Status saat ini (14 Nov 2026) |
|---|---|---|---|
| 1 | Katalog beku semua paket/add-on selesai divalidasi | [WRIKE_SOURCE_CATALOG_2026-11-13.md](parity/WRIKE_SOURCE_CATALOG_2026-11-13.md), PAR-CAT-01 di [TRACEABILITY](TRACEABILITY_M7_M12.md) | **Terbuka**: katalog ada dan matriks `DRAFT-SIAP-REVIEW`; 31 baris "NOT_IN_TABLE" butuh bukti help center; persetujuan pemilik produk belum tercatat |
| 2 | Semua WFP in-scope `ACCEPTED` (atau `N/A_APPROVED` dengan keputusan); tidak ada `NEEDS_SOURCE_VERIFICATION` tanpa resolusi | [PARITY_TEST_EVIDENCE.md](parity/PARITY_TEST_EVIDENCE.md) | **Terbuka**: 0 `ACCEPTED`, 93 `PENDING_UAT`, 13 `N/A_APPROVED` (keputusan DEC-PAR-01/05/06 menunggu persetujuan pemilik) |
| 3 | 93 requirement P0/P1 asli: UAT selesai dan disetujui | Tabel 93 baris di atas; [G0_STABILIZATION.md](parity/gate-reports/G0_STABILIZATION.md) (STAB-01) | **Terbuka**: semua `NOT RUN` |
| 4 | Semua blocker STAB ditutup | [G0_STABILIZATION.md](parity/gate-reports/G0_STABILIZATION.md) | **Terbuka**: STAB-01 dan STAB-03 (required check GitHub butuh paket/kepemilikan repo) berstatus BLOCKED; STAB-02/04 lulus pada kode |
| 5a | Uji WSTG yang relevan | [TESTING.md](TESTING.md) (S6.3.3, S7.1.3, S12.2.1) | **Sebagian**: pemeriksaan internal otomatis dan sapuan 286 operasi; **bukan** pentest manual atau WSTG lengkap |
| 5b | CI E2E | job `parity-e2e` di `.github/workflows/ci.yml`; [TASKS.md](TASKS.md) S12.2.1 | **Terbuka**: lulus lokal; hijau di GitHub Actions untuk basis ini perlu diverifikasi manusia di GitHub (konteks tugas: dilaporkan hijau, bukti ada di GitHub, bukan di repo) |
| 5c | Dependency scanning | — | **Tidak terverifikasi**: tidak ditemukan langkah pemindaian dependensi di `ci.yml` saat disusun; perlu keputusan/penambahan |
| 5d | Backup/restore drill | [TESTING.md S12.2.1](TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026); AD-14 | **Sebagian**: drill otomatis di image integrasi lulus (RTO 44 s); belum pada data klien dan belum bulanan |
| 6 | Multi-user access/roles + public links + integrations + AI permissions diverifikasi | Kelompok 1, 6, 7, 8, 9 di atas | **Terbuka**: otomatis ada, UAT `NOT RUN`; konektor akun nyata belum; **AI di luar 1.0** (perlu keputusan menghapus kata "AI permissions" dari kriteria untuk 1.0) |
| 7 | Performa pada workload realistis memenuhi NFR | [TESTING.md S12.2.1](TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | **Sebagian**: Table p95 1.074 ms dan Board 1.192 ms (target < 1.500 ms) lulus tipis; `TestScheduling500TaskPerformance` (ambang 300 ms) gagal di mesin uji, dinilai lingkungan; belum diukur pada mesin tenang atau data klien |
| 8 | Deployment rehearsal, migrasi, rollback/roll-forward, monitoring, runbook dukungan | [PILOT_RUNBOOK.md](PILOT_RUNBOOK.md); `deploy/*.sh`; `deploy/monitoring`; `TestUpgradeFromTheEarlySchemaKeepsEveryRow` | **Sebagian**: rehearsal migrasi dan restore lulus di lingkungan lokal; rehearsal pada host rilis, alert Grafana yang menyala, dan serah terima runbook/kontak dukungan belum |
| 9 | Product sign-off, security sign-off, release approval tercatat | — | **Terbuka**: butuh manusia (lihat bagian berikut) |

## Yang masih butuh manusia atau akun klien

| Butuh | Untuk | Catatan |
|---|---|---|
| Penguji manusia untuk semua skenario di atas dan 93 baris awal | S12.2.2, gate #2, #3 | Paket ini siap; tidak ada yang dijalankan |
| Host/VM rilis, domain, dan TLS (Traefik/ACME) | Deployment rehearsal, gate #8 | Belum ada host dan domain |
| Server SMTP sungguhan (dan kotak surat uji) | SET-10, FORM-08/FORM-04, RPT-04, undangan/reset/OTP (WF-S10, RP-05, AD-01) | Pengujian otomatis memakai SMTP tiruan |
| Backup di luar server (offsite) dan jadwalnya | OPS-01, gate #5d | Drill lokal tidak membuktikan salinan di luar mesin |
| Akun GitHub dan Google uji (repo uji, kalender uji) | INT-03 (IN-06, IN-07) | `TestRealGitHubAccount`, `TestRealGoogleAccount` ada tetapi belum dijalankan |
| IdP OIDC dan SAML uji, klien SCIM | AD-07, AD-08 | Diuji hanya terhadap tiruan |
| Bucket S3/Pydio uji | CL-09 | Hanya server palsu |
| Admin GitHub: paket repo atau kepemilikan untuk *required check*; konfirmasi hijau Actions | STAB-03, gate #4 dan #5b | |
| Pemilik produk: persetujuan matriks WFP, N/A_APPROVED (DEC-PAR-01/05/06), konfirmasi WFP-044, kriteria "AI permissions" | Gate #1, #2, #6 | Keputusan produk, bukan hasil kode |
| Reviewer keamanan: sign-off dan keputusan pentest/WSTG manual serta dependency scanning | Gate #5a, #5c, #9 | |
| Pengukuran NFR pada mesin tenang dengan data mendekati klien; ambang tes 300 ms | Gate #7 | |
| Sign-off produk dan keamanan, release approval | Gate #9, S12.2.3 | |

## Daftar topik panduan pengguna/admin (belum ditulis)

Butir "panduan" belum dibuat. Daftar ini hanya kerangka topik; panduan akhir untuk pengguna dan admin ditulis dalam English. Tautan ke runbook yang ada bila ada.

**Per fitur utama (pengguna):** masuk, undangan, 2FA, dan OTP; Space, project, folder, dan sharing; task, subitem, lokasi ganda, tanggal, effort; view (Table, Board, Gantt, Calendar, Timelog, Files, Chart) dan view tersimpan; custom field, formula, tipe item kustom; workflow, status, dan approval; form dan permintaan (My requests); otomasi (aturan WHEN/IF/THEN dan log); workload, booking, dan baseline; time entry dan timesheet; komentar, file, versi, proofing, dan review file; dashboard, report, dan langganan; impor/ekspor Excel; API key, OAuth, webhook, dan konektor; pencarian, Inbox, Stream, Starred, My to-do; Recycle bin.

**Per tugas admin:** bootstrap dan undangan pertama ([PILOT_RUNBOOK.md](PILOT_RUNBOOK.md#bootstrap)); **seat** dan user ([Menambah seat](PILOT_RUNBOOK.md#menambah-seat)); **backup dan restore** ([Backup dan restore](PILOT_RUNBOOK.md#backup-dan-restore)); **update** dan rollback ([Update](PILOT_RUNBOOK.md#update)); **2FA** dan kunci ([Kunci 2FA](PILOT_RUNBOOK.md#kunci-2fa-pms_2fa_key)), reset 2FA; SSO OIDC/SAML ([OIDC](PILOT_RUNBOOK.md#single-sign-on-oidc-pms_sso_key), [SAML](PILOT_RUNBOOK.md#single-sign-on-saml)); SCIM ([Provisioning SCIM](PILOT_RUNBOOK.md#provisioning-scim)); role kustom, locked space, dan IP allowlist ([Locked space dan IP allowlist](PILOT_RUNBOOK.md#locked-space-dan-ip-allowlist)); OTP dan kepemilikan akun ([Login OTP dan kepemilikan akun](PILOT_RUNBOOK.md#login-otp-dan-kepemilikan-akun)); kunci konektor ([Kunci konektor](PILOT_RUNBOOK.md#kunci-konektor-pms_connector_key)); audit keamanan; work schedule, rates, dan resource; ekspor dan penghentian instance ([Menghentikan instance](PILOT_RUNBOOK.md#menghentikan-instance-usr-04)); triase masalah ([Triase pilot](PILOT_RUNBOOK.md#triase-pilot)); monitoring dan alert.


## Status perbaikan temuan dry-run (diisi sesi perbaikan; kolom hasil UAT manusia tidak diubah)

| ID | Status |
|---|---|
| DR-01 | diperbaiki: entri "created" tidak lagi memuat daftar nama kolom (`item-activity.tsx`); tes `dr01-activity.spec.ts` |
| DR-02 | diperbaiki bagian kedua: tautan "Gantt spike" dihapus dari menu Settings; tes `dr02-settings-menu.spec.ts` (menu Apps & Integrations dobel: PR #9) |
| DR-03 | belum dapat direproduksi pada tes mock (dialog tertutup dan item baru dibuka); tes penjaga `dr03-create-closes.spec.ts` ditambahkan, perlu dicek ulang manusia |
| DR-04 | diperbaiki: Skip menutup wizard dari status yang baru disimpan (`onboarding-wizard.tsx`); tes di `onboarding.spec.ts` |
| DR-05 | diperbaiki: migrasi diserialkan dengan advisory lock Postgres (`database.go`); tes `TestOpenConcurrentlyOnEmptyDatabase` |
| DR-06 | disengaja, dicatat sebagai D-100; tanpa perubahan kode |
| DR-08 | diperbaiki: satu bentuk cache `work-schedule` (`lib/work-schedule.ts`); tes `dr08-calendar-effort.spec.ts` |

## Tambahan: skenario tampilan M12b (14 Nov 2026)

Penguji mengulang alur harian pada tampilan baru: ubah status/assignee/tanggal lewat chip panel detail; atur filter, sort, group, kolom lewat toolbar satu baris; Board dan Table (chip status, avatar); My to-do berkelompok dan tambah cepat; Inbox dua panel dan tab Sent; sidebar per Space dan halaman project; menu More dan Customize menu; menu akun; dashboard (pindah dashboard, layout, editor widget dua panel). Catat hasil dengan skala keparahan yang sama.
