# Addendum scope: Create Task untuk Gantt, Calendar, dan Workload

> Status: **scope disetujui, belum diimplementasikan.** Dokumen ini mendefinisikan kebutuhan, acuan API yang sudah ada, kriteria penerimaan, dan keputusan yang masih perlu diambil sebelum build.
> Dasar: `docs/reviews/LENS_CREATE_TASK_ACTUAL_CODE_AUDIT.md` (G-05, G-06, G-07) dan `docs/reviews/LENS_CREATE_TASK_REMEDIATION_B1_B2.md`.
> Acuan perilaku: https://help.wrike.com/hc/en-us/articles/1500005120921-Creating-a-Task (behavioral reference, bukan salinan UI).

---

## 1. Gantt: Create Task

**Kebutuhan (scope):**
1. **Add subitem dari baris Gantt tabel.** Parent = baris terpilih, sama dengan Table (lihat R3/B2.3).
2. **Add above / Add below dari baris Task.** Task baru satu hari dengan start date sama dengan Task acuan, ditempatkan di atas atau di bawah dalam urutan manual (`/items/{itemId}/reorder`).
3. **Create dari timeline dengan drag** pada rentang tanggal kosong. Rentang yang di-drag menjadi start dan finish. Drag untuk memindahkan atau mengubah durasi bar tetap terpisah dari drag untuk membuat.

**Acuan API yang sudah ada:**
- `POST /items` menerima `start_date`, `due_date`, `locations` (dengan `parent_item_id`), dan `id` dari klien (Opsi B).
- `POST /items/{itemId}/reorder` untuk urutan above/below.
- Komponen: `web/src/components/item-gantt.tsx` (sudah ada interaksi drag untuk geser dan resize, `item-gantt.tsx:185, 201`).

**Kriteria penerimaan:**
- Add above/below membuat Task dengan start = start Task acuan, finish = start (satu hari), dan parent sama.
- Drag di area kosong timeline membuat Task dengan start dan finish dari rentang. Hari akhir pekan mengikuti aturan hari kerja yang sudah dipakai dialog.
- Drag pada bar yang sudah ada tetap memindahkan atau mengubah durasi, dan tidak membuat Task.
- Setelah create, bar muncul di Gantt pada posisi dan tanggal yang benar, dan data sama di Table (cross-view, lihat R5).
- Keyboard: tidak ada bentrok dengan seleksi, resize, atau pindah.

**Keputusan yang perlu:**
- G1. Hari akhir pekan di rentang drag: dilewati, atau rentang dipotong ke hari kerja?
- G2. Drag satu hari (klik tanpa geser) membuat Task satu hari atau dibatalkan?
- G3. Shortcut keyboard untuk create dari timeline (Wrike memakai shortcut platform-spesifik). Pilih tombol yang tidak bentrok dengan browser.
- G4. Batas zoom dan scroll saat drag di timeline yang panjang.

**Dependensi:** Gantt create dari timeline tidak bergantung pada Opsi B, tetapi memakai `POST /items` yang sama.

---

## 2. Calendar: Create Task

**Kebutuhan (scope):**
1. **Klik tanggal di Calendar View project** membuka dialog create dengan tanggal itu sebagai start dan finish default, dan parent = project.
2. **Klik tanggal di halaman Calendars** (lintas item) dengan lokasi dipilih di dialog.
3. **Durasi default** mengikuti skala tampilan, sama dengan Wrike: minggu dan bulan = satu hari; kuartal = lima hari; tahun = satu bulan. Berlaku jika tampilan punya skala tersebut.

**Acuan API yang sudah ada:**
- `POST /items` dengan `start_date`, `due_date`, dan `locations`.
- Komponen: `web/src/components/item-calendar.tsx`. Saat ini `readonly` dan tidak punya callback create (`item-calendar.tsx:24-25`). Halaman `web/src/routes/_app/calendars.tsx` juga read-only.

**Kriteria penerimaan:**
- Klik tanggal (bukan item) membuka dialog dengan tanggal terisi. Jika user punya hak create di project, dialog bisa dibuka.
- Durasi default sesuai tabel di atas, dan bisa diubah di dialog.
- Task langsung muncul di kalender pada tanggal yang benar, termasuk saat timezone user berbeda dari server (lihat tes timezone yang sudah ada untuk Calendar).
- Halaman Calendars (lintas item) hanya membuat Task jika user memilih lokasi di dialog. Tidak ada parent yang diasumsikan.

**Keputusan yang perlu:**
- C1. Apakah Calendars (lintas item) juga menerima create, atau hanya Calendar View di project?
- C2. Klik tanggal di tampilan bulan: create langsung, atau hanya mengisi tanggal di dialog?
- C3. Jika user memakai Calendar View dengan filter aktif, create mengikuti filter atau tidak? Ini terkait feedback filter R4.

**Dependensi:** feedback filter (R4) sudah ada untuk dialog di Table dan Workspace. Calendar perlu mengikuti pola yang sama.

---

## 3. Workload: Create Task

**Kebutuhan (scope):**
1. **Create Task dari lane resource.** User memilih rentang tanggal pada baris orang (lane) di Workload, lalu membuat Task. Assignee otomatis orang di lane itu.
2. **Alokasi jam** pada Task baru mengikuti rentang dan jam yang diisi di dialog, dan tampil di Workload setelah create.
3. **Konflik dan kapasitas** ditampilkan sebelum simpan, sesuai aturan kapasitas yang sudah ada di backend.

**Acuan API yang sudah ada:**
- `POST /items` menerima `assignee_allocations` (`api/openapi.yaml`, `ItemCreate`), dengan dates dan `locations`.
- `GET /workload` menyediakan data lane, booking, dan kapasitas (`api/openapi.yaml:837`).
- `POST /resource-bookings` (admin, `api/openapi.yaml:959`) untuk booking. Booking ini berbeda dari alokasi Task. Keputusan perlu menentukan mana yang dipakai.
- Komponen: `web/src/routes/_app/workload.tsx` (hanya tampilan dan booking admin, `workload.tsx:83-104`).

**Kriteria penerimaan:**
- Create dari lane mengisi assignee dan alokasi sesuai rentang yang di-drag atau dipilih.
- Setelah create, workload dan kapasitas orang itu berubah, dan Task tampil di Table dengan assignee yang sama.
- Jika kapasitas terlampaui, dialog menampilkan peringatan dan user memilih: lanjut atau batal. Server tidak diam-diam menolak tanpa penjelasan.
- Jika create gagal, tidak ada alokasi "hantu" yang tertinggal (alokasi dan item dalam satu transaksi, sudah dijamin oleh `create` yang serializable).
- Permission: user hanya bisa membuat Task untuk orang yang bisa diberi tugas di Space itu.

**Keputusan yang perlu:**
- W1. Alokasi Task (`assignee_allocations`) atau booking (`resource-bookings`) yang menjadi sumber kebenaran untuk Workload create?
- W2. Konflik: lanjut dengan peringatan, atau blok keras?
- W3. Siapa yang boleh membuat Task untuk lane orang lain? Admin saja, atau siapa pun dengan hak create di Space?
- W4. Rentang lane: satu hari atau minggu, dan berapa jam per hari default?

**Dependensi:** pengambilan keputusan W1 penting sebelum build, karena menentukan model data.

---

## 3a. Keputusan Workload yang sudah disetujui

**W1: sumber kebenaran alokasi (APPROVED)**
- `assignee_allocations`: alokasi pekerjaan per Task, assignee, tanggal, dan effort.
- `resource-bookings`: reservasi kapasitas resource yang tidak harus berasal dari Task.
- Workload Chart menampilkan beban gabungan keduanya, tanpa menghitung dua kali alokasi yang mewakili pekerjaan yang sama.
- Create Task dari lane Workload membuat Task dan alokasi assignee-nya secara atomik. Tidak ada `resource-bookings` yang dibuat otomatis untuk Task itu.

**W2: konflik kapasitas sebagai soft constraint (APPROVED)**
- Contoh: kapasitas 8 jam/hari, sudah dialokasikan 7 jam, Task baru 3 jam. Ditampilkan "Capacity exceeded — Assigned workload: 10h / 8h (125%)". Task tetap dibuat.
- Aturan:
  1. Warning muncul sebelum konfirmasi, dan tampil di Workload Chart setelah penyimpanan.
  2. Overcapacity tidak menggagalkan create.
  3. Permission, data tidak valid, dan batasan keras yang memang dikonfigurasi tetap dapat memblokir operasi.
  4. Perhitungan kapasitas memperhitungkan working calendar, hari libur, durasi, dan effort per assignee.
  5. Hindari double counting antara Task Allocation dan Resource Booking.

**W3: permission membuat Task untuk orang lain (APPROVED)**
- Space Admin dan Project Manager: boleh membuat Task pada lane anggota lain, selama punya akses yang sesuai.
- Member: boleh membuat Task untuk dirinya sendiri jika punya Create. Untuk anggota lain, juga harus punya Assign Task.
- Viewer dan Read-only: tidak boleh membuat Task.
- Cross-Space: ditolak jika tidak punya hak yang diperlukan di Space tujuan.
- Backend wajib memvalidasi Create dan Assign. Menyembunyikan tombol di frontend tidak cukup.
- **Perlu dicek:** daftar kapabilitas LENS saat ini (`view, comment, change_status, attach, edit, create, delete, share, use_blueprint, create_blueprint, manage_blueprint`) belum punya "assign". Sesuai keputusan, jika Assign belum ada, assignment ke orang lain dibatasi ke Space Admin dan Project Manager sampai permission itu didefinisikan.

**W4: rentang tanggal dan alokasi default (APPROVED)**

| Kondisi | Perilaku |
|---|---|
| Klik atau drag satu hari | Task berdurasi 1 hari |
| Drag beberapa hari | Start sampai Finish mengikuti rentang drag, inklusif |
| Default effort | 1 jam per hari kerja yang dipilih |
| Akhir pekan dan libur | Tidak mendapat alokasi otomatis, mengikuti working calendar |
| Hanya hari nonkerja dipilih | Minta pengguna memilih hari kerja atau mengatur jadwal secara eksplisit |
| Effort | Bisa diedit sebelum menyimpan |
| Capacity exceeded | Warning, bukan blocking (W2) |
| Source of truth | `assignee_allocations` (W1) |

Contoh: drag Senin sampai Jumat menghasilkan Task rentang 5 hari dengan total alokasi default 5 jam, jika kelimanya hari kerja.

Perhitungan memakai kalender kerja resource. Jika konfigurasi itu belum ada, pakai default Workspace yang terdokumentasi, bukan asumsi 8 jam sehari untuk semua pengguna.

**Yang masih terbuka dari Workload:** tidak ada.

**Catatan Calendar (Batch 3A):** keputusan C1 sampai C3 belum disetujui. Implementasi 3A memakai default konservatif: create hanya di Calendar View project, klik "+" pada hari kerja membuka dialog dengan tanggal terisi, dan Calendars lintas item tetap read-only.

## 4. Urutan build yang disarankan

1. **Keputusan dulu:** G1–G4, C1–C3, W1–W4. Tanpa ini, implementasi akan menebak aturan produk.
2. **Calendar create** (paling kecil): klik tanggal → dialog dengan tanggal. Memakai `POST /items` yang sudah ada.
3. **Gantt add above/below dan subitem** (menengah): memakai reorder yang sudah ada.
4. **Gantt drag create** (menengah-besar): interaksi baru, perlu tes E2E khusus untuk bentrok dengan drag yang sudah ada.
5. **Workload create** (besar): model alokasi dan kapasitas, perlu desain sebelum kode.

Setiap langkah mengikuti pola remediasi: tes integrasi untuk aturan server, unit untuk logika klien, E2E mock untuk UI, dan tes live bila perlu.

## 5. Ringkasan keputusan yang dibutuhkan

| ID | Keputusan | Dampak |
|---|---|---|
| G1 | Hari akhir pekan di rentang drag | Gantt |
| G2 | Drag satu hari | Gantt |
| G3 | Shortcut keyboard | Gantt |
| G4 | Zoom dan scroll saat drag | Gantt |
| C1 | **APPROVED:** create hanya di Calendar View project. Calendars lintas item tetap read-only | Calendar |
| C2 | **APPROVED:** klik "+" pada hari kerja membuka dialog dengan tanggal terisi, bukan membuat langsung | Calendar |
| C3 | **APPROVED:** filter tidak diubah. Jika Task baru tersembunyi filter, berlaku feedback R4 | Calendar |
| W1 | **APPROVED:** `assignee_allocations` sebagai sumber kebenaran alokasi Task | Workload |
| W2 | **APPROVED:** konflik kapasitas menjadi warning (soft constraint), bukan blok keras | Workload |
| W3 | **APPROVED:** membuat Task untuk lane orang lain: Space Admin, Project Manager, atau pengguna dengan Create dan Assign | Workload |
| W4 | **APPROVED:** rentang mengikuti drag, alokasi default 1 jam per hari kerja terpilih | Workload |

## 6. Keputusan Calendar (disetujui)

- **C1 (APPROVED):** create hanya di Calendar View project. Calendars lintas item tetap read-only.
- **C2 (APPROVED):** klik "+" pada hari kerja membuka dialog dengan tanggal terisi (start dan finish), bukan membuat Task langsung.
- **C3 (APPROVED):** filter tidak diubah. Jika Task baru tidak tampil karena filter aktif, berlaku feedback R4 (notice dengan tombol "Buka").

Implementasi di Batch 3A sudah sesuai dengan ketiga keputusan ini.
