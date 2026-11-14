# UAT P0 dan P1

Gunakan instance pilot dengan versi yang dicatat. Untuk setiap requirement, catat PASS, FAIL atau SKIP beserta tanggal, pemeran, dan tautan bukti. Bukti otomatis tercatat di [TESTING.md](TESTING.md#verifikasi-kandidat-ga-13-november-2026).

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
