# M2 — Planning & Scheduling: story dan tugas

| | |
|---|---|
| **Status dokumen** | `DRAFT`. Keputusan M2-1 sampai M2-5 disetujui user 13 Nov 2026 (D-36 sampai D-40). Estimasi 41 hari vs 30 hari belum diputuskan |
| **Cakupan** | VIEW-05A, VIEW-05B, VIEW-03, VIEW-04, VIEW-06, VIEW-09; TASK-05B, TASK-09, TASK-10, TASK-11A; SET-07; SPC-03, SPC-10 (PRD §8) |
| **Exit criteria (PRD)** | Proyek bisa dijadwalkan penuh di Gantt dengan dependensi; durasi menghormati work schedule; custom field tampil di semua view |
| **Format** | Mengikuti [BACKLOG.md](BACKLOG.md) (story, estimasi, dependensi, acceptance criteria) dan [TASKS.md](TASKS.md) (tugas ≤ 0.5 hari). Setelah disetujui, pindahkan ke dua file itu atau jadikan GitHub Issues per story |
| **Definition of Done** | Sama dengan BACKLOG: `openapi.yaml` lebih dulu, tes otorisasi per endpoint baru, teks lewat `t()`, keyboard dan ARIA, migrasi diuji maju, `DONE` hanya dengan bukti di [TESTING.md](TESTING.md) |

## 0. Titik awal (sudah ada dari M0/M1)

- Tabel `items` sudah punya `start_date`, `due_date`, `duration_days`, `is_milestone`, dan `fields jsonb`; `item_locations` mendukung banyak lokasi dan `POST /items/{id}/locations` ada di kontrak.
- `views.type` baru `table`/`board`. Enum perlu diperluas.
- `server/internal/schedule` baru mencakup hari kerja Senin–Jumat dan geser satu relasi FS (hasil spike, [GANTT_SPIKE.md](GANTT_SPIKE.md)). Propagasi berantai, tipe SS/FF/SF, dan libur belum ada.
- Sort `priority` dan `importance` sudah ada di kontrak daftar tetapi disembunyikan sampai M2.
- Keputusan spike: **lanjut SVAR MIT** (`@svar-ui/react-gantt`). Performa dan penyimpanan perubahan belum diuji.

## 1. Keputusan (disetujui 13 Nov 2026, D-36 sampai D-40)

| # | Pertanyaan | Rekomendasi | Mempengaruhi |
|---|---|---|---|
| M2-1 | "Priority" (SPC-03 dan sort) disimpan di mana? | Kolom `importance` enum (`high`/`normal`/`low`) pada `items`, bukan custom field, karena dipakai sort dan filter preset di semua Space | S2.5.3 |
| M2-2 | Penjadwalan otomatis: kapan dijalankan? | Saat tanggal atau dependensi berubah (satu transaksi). Tidak ada tombol "jadwal ulang semua" di M2 | S2.1.3 |
| M2-3 | Task penerus yang dilanggar: digeser otomatis atau hanya ditandai? | Digeser otomatis ke tanggal paling awal yang sah, tidak pernah dimajukan lebih awal dari yang sudah diatur user (mode "tidak lebih awal dari") | S2.1.3 |
| M2-4 | Work schedule: satu per workspace atau per user? | Satu per workspace (SET-07 hanya menyebut satu). Per user menunggu M4 (timesheet) | S2.1.1 |
| M2-5 | Custom field berlaku per Space; bagaimana item yang punya banyak lokasi? | Nilai disimpan di item (`items.fields` berkunci id definisi). Kolom tampil bila definisi ada di salah satu Space lokasi. Tampilkan hanya yang tersedia di Space yang sedang dilihat | S2.3.x |

## 2. Urutan eksekusi

Satu developer, tidak paralel. Tiap baris memakai hasil baris di atasnya.

| # | Story | Judul | Hari | Dep |
|---|---|---|---|---|
| 1 | S2.1.1 | Work schedule (SET-07) | 2.5 | M1 |
| 2 | S2.1.2 | Dependensi item (data dan API) | 2.5 | M1 |
| 3 | S2.1.3 | Mesin penjadwalan di Go | 4 | S2.1.1, S2.1.2 |
| 4 | S2.2.1 | Effort dan allocation (TASK-05B) | 2.5 | S2.1.1 |
| 5 | S2.2.2 | Progress project (TASK-09) | 2 | S2.2.1 |
| 6 | S2.3.1 | Definisi custom field dan API (TASK-10) | 3 | M1 |
| 7 | S2.3.2 | Tipe field lengkap (TASK-11A) | 3 | S2.3.1 |
| 8 | S2.4.1 | Tipe view Gantt dan Calendar | 1 | M1 |
| 9 | S2.4.2 | Gantt: bar, progres, drag/resize, milestone (VIEW-05A) | 4 | S2.4.1, S2.2.2 |
| 10 | S2.4.3 | Gantt: dependensi dan penjadwalan (VIEW-05B) | 3 | S2.4.2, S2.1.3 |
| 11 | S2.3.3 | Custom field di panel, Table, Board + pemilih kolom (VIEW-09) | 3 | S2.3.2 |
| 12 | S2.5.1 | Board swimlane dan field kartu (VIEW-03, VIEW-04) | 3 | S2.3.3 |
| 13 | S2.5.2 | Calendar (VIEW-06) | 2.5 | S2.4.1 |
| 14 | S2.5.3 | Preset filter dan sort Space (SPC-03) | 1.5 | S2.3.3 |
| 15 | S2.5.4 | UI multi-lokasi (SPC-10) | 1.5 | M1 |
| 16 | S2.6.1 | E2E, aksesibilitas, dan performa M2 | 2 | semua |
| | | **Total** | **41** | |

**Selisih dengan rencana PRD:** PRD memberi 6 minggu = 30 hari. Estimasi bottom-up **41 hari** (≈ 8.2 minggu), selisih **+11 hari**. Penyebab terbesar: mesin penjadwalan (S2.1.3) dan Gantt (S2.4.2, S2.4.3). Kalibrasi ulang dengan kecepatan nyata M1 sebelum menjanjikan tanggal ke klien. Jalur kritis: S2.1.1 → S2.1.2 → S2.1.3 → S2.4.2 → S2.4.3 → S2.6.1.

Pilihan bila 30 hari harus dipertahankan: geser SPC-10 (S2.5.4, 1.5 hari) dan VIEW-04 (bagian field kartu di S2.5.1, ±1 hari) ke M3 (hemat ±2.5 hari, belum cukup menutup 11 hari; sisanya perlu pemangkasan lain atau jadwal baru). Itu perubahan scope dan perlu persetujuan user.

---

## E2.1 Jadwal dan dependensi

### S2.1.1 — Work schedule (SET-07)
- **Sebagai** admin, **saya ingin** menetapkan hari kerja, kapasitas harian, dan libur, **agar** durasi dan jadwal task mengikuti kalender kerja organisasi.
- **Est:** 2.5 · **Dep:** M1
- **Acceptance criteria:**
  - Admin mengatur hari kerja (default Senin–Jumat), jam kerja per hari (default 8), dan daftar tanggal libur lewat `/settings/work-schedule`; non-admin hanya bisa membaca.
  - Perubahan memakai `If-Match`; konflik memberi 409.
  - Fungsi Go `workdays(start, duration)` dan `duration(start, finish)` memakai jadwal ini; kasus libur di tengah rentang, libur di akhir pekan, dan jadwal tanpa hari kerja ditolak (422) teruji.
- **Tugas:**
  - [ ] `openapi.yaml`: `GET/PUT /settings/work-schedule`
  - [ ] Migrasi `work_schedules` (satu baris per account) dan `work_schedule_exceptions` (tanggal, nama)
  - [ ] Paket `schedule`: kalender dari data (ganti asumsi Senin–Jumat tetap)
  - [ ] Endpoint dan tes otorisasi (admin tulis, semua baca)
  - [ ] Layar pengaturan + tes Playwright

### S2.1.2 — Dependensi item (data dan API)
- **Sebagai** user, **saya ingin** menghubungkan task dengan dependensi FS/SS/FF/SF, **agar** urutan kerja jelas.
- **Est:** 2.5 · **Dep:** M1
- **Acceptance criteria:**
  - Dependensi dibuat dan dihapus lewat API antara dua item yang pembacanya sama-sama punya akses; tipe FS, SS, FF, SF dengan lag (hari kerja, boleh negatif).
  - Siklus ditolak (422), juga siklus tidak langsung (A→B→C→A) dan dependensi ke diri sendiri.
  - Item yang masuk Recycle bin tidak ikut perhitungan, dan dependensinya kembali bila direstore.
  - Pengguna tanpa akses ke salah satu ujung tidak melihat ujung itu (404/disembunyikan), tes IDOR lulus.
- **Tugas:**
  - [ ] `openapi.yaml`: `GET/POST /items/{id}/dependencies`, `DELETE /dependencies/{id}`
  - [ ] Migrasi `item_dependencies` (predecessor, successor, type, lag) dengan unik dan CHECK bukan diri sendiri
  - [ ] Deteksi siklus (CTE rekursif) dalam transaksi Serializable
  - [ ] Policy: butuh hak edit pada penerus dan hak lihat pada pendahulu
  - [ ] Tes integrasi dengan PostgreSQL, termasuk soft delete dan restore

### S2.1.3 — Mesin penjadwalan di Go
- **Sebagai** PM, **saya ingin** task penerus bergeser otomatis ketika pendahulunya berubah, **agar** jadwal tetap konsisten tanpa hitung manual.
- **Est:** 4.0 · **Dep:** S2.1.1, S2.1.2
- **Acceptance criteria:**
  - Mengubah tanggal atau durasi sebuah task, atau menambah/mengubah dependensi, menghitung ulang seluruh rantai penerus dalam **satu transaksi** dan mengembalikan daftar item yang berubah.
  - Keempat tipe (FS, SS, FF, SF) dan lag benar menurut tabel kasus; hasil mengikuti hari kerja dan libur dari S2.1.1.
  - Keputusan M2-3 berlaku: penerus tidak pernah dimajukan, hanya digeser bila melanggar.
  - Milestone (durasi 0) dan task tanpa tanggal ditangani tanpa galat.
  - Rantai 500 task selesai dalam ≤ 300 ms (p95) pada PostgreSQL uji.
  - Setiap item yang digeser mencatat aktivitas dan menaikkan `version`; klien lain menerima event SSE.
- **Tugas:**
  - [ ] Perluas `schedule`: empat tipe, lag, kalender dari S2.1.1 (tes tabel ≥ 20 kasus)
  - [ ] Topological order dan propagasi ke seluruh keturunan (tahan siklus data rusak: berhenti dan lapor)
  - [ ] Integrasikan ke `PATCH /items/{id}` dan ke endpoint dependensi (satu transaksi, `If-Match` tetap)
  - [ ] Respons memuat `affected_items` agar UI memperbarui bar tanpa memuat ulang
  - [ ] Benchmark 500 task dan tes konflik dua edit bersamaan
  - [ ] Catatan keputusan di `DECISIONS.md` untuk M2-2 dan M2-3

## E2.2 Effort dan progress

### S2.2.1 — Effort dan allocation (TASK-05B)
- **Sebagai** PM, **saya ingin** mengisi effort dan alokasi per orang, **agar** beban kerja dan durasi terhitung.
- **Est:** 2.5 · **Dep:** S2.1.1
- **Acceptance criteria:**
  - Panel detail memuat effort (jam) dan alokasi per assignee (persen atau jam per hari); durasi dapat dihitung dari effort dan alokasi memakai kapasitas harian dari S2.1.1.
  - Menghapus assignee menghapus alokasinya; total alokasi di atas 100% memberi peringatan, bukan galat.
  - Mengubah effort tidak menggeser jadwal kecuali user memilih "hitung ulang durasi".
- **Tugas:**
  - [ ] `openapi.yaml`: kolom `effort_minutes` pada Item dan `allocation` pada assignee
  - [ ] Migrasi kolom dan CHECK nilai non-negatif
  - [ ] Rumus durasi dari effort (paket `schedule`) + tes
  - [ ] UI di panel detail dan tes Playwright

### S2.2.2 — Progress project (TASK-09)
- **Sebagai** PM, **saya ingin** melihat progres project dan risikonya, **agar** tahu mana yang terlambat.
- **Est:** 2.0 · **Dep:** S2.2.1
- **Acceptance criteria:**
  - Metode progres dipilih per project: jumlah task selesai, durasi berbobot, atau rata-rata persen task; persen dihitung di server.
  - Indikator risiko (hijau/kuning/merah) mengikuti aturan tertulis di `WORKFLOWS.md` (mis. terlambat bila `due_date` lewat dan belum selesai).
  - Task di Recycle bin dan status `cancelled` tidak dihitung.
- **Tugas:**
  - [ ] Aturan perhitungan dan risiko ditulis di `WORKFLOWS.md` dahulu
  - [ ] Kolom metode di project; persen dan risiko dikembalikan oleh API (tidak disimpan)
  - [ ] Tes tabel untuk tiga metode dan kasus kosong
  - [ ] Tampilkan di panel project dan baris Table

## E2.3 Custom field

### S2.3.1 — Definisi custom field dan API (TASK-10)
- **Sebagai** admin Space, **saya ingin** membuat field kustom dengan pilihan berwarna, **agar** tim memakai atribut yang sesuai pekerjaannya.
- **Est:** 3.0 · **Dep:** M1
- **Acceptance criteria:**
  - Admin atau pemilik Space membuat, mengubah, mengurutkan, dan mengarsipkan definisi field per Space; menghapus opsi yang sedang dipakai ditolak atau diarsipkan, tidak menghilangkan nilai item.
  - Nilai disimpan di `items.fields` berkunci id definisi dan divalidasi server (id tak dikenal 422).
  - Field berlaku untuk task, project, dan folder di Space itu; keputusan M2-5 berlaku untuk item multi-lokasi.
- **Tugas:**
  - [ ] `openapi.yaml`: `/spaces/{id}/custom-fields` dan `fields` pada PATCH item
  - [ ] Migrasi `custom_fields` (space, nama, tipe, opsi jsonb, posisi, arsip)
  - [ ] Validasi nilai pada PATCH item dalam transaksi yang sama dengan `If-Match`
  - [ ] Policy: kelola field butuh hak admin Space; lihat nilai ikut hak lihat item
  - [ ] Layar pengelolaan di pengaturan Space dan tes

### S2.3.2 — Tipe field lengkap (TASK-11A)
- **Sebagai** user, **saya ingin** tipe field yang sesuai datanya, **agar** angka, tanggal, dan orang tersimpan benar.
- **Est:** 3.0 · **Dep:** S2.3.1
- **Acceptance criteria:**
  - Tipe: Text, Number, Currency, Percent, Single select, Multiple select, People, Date, Duration, Checkbox. Setiap tipe punya validasi server dan pesan galat jelas.
  - Currency memiliki simbol, desimal, dan pemisah ribuan; Percent 0–100 atau lebih bila diizinkan; format mengikuti locale user.
  - Conditional marker (aturan warna berdasarkan nilai) dan agregasi default (jumlah, rata-rata, min, maks, hitung) tersedia untuk tipe numerik.
  - People hanya menerima user yang terlihat oleh pemilih; user nonaktif ditampilkan namun tidak bisa dipilih baru.
- **Tugas:**
  - [ ] Validator per tipe (satu tabel tes, termasuk batas dan nilai kosong)
  - [ ] Format dan parsing locale (`Intl`) di web, uji id-ID dan en-US
  - [ ] Pengeditor sel per tipe (pakai komponen `ui-kit`)
  - [ ] Conditional marker dan agregasi
  - [ ] Tes Playwright untuk tipe yang paling berisiko (Currency, People)

### S2.3.3 — Custom field di panel, Table, Board, dan pemilih kolom (VIEW-09)
- **Sebagai** user, **saya ingin** melihat dan mengedit custom field di semua tampilan, **agar** tidak berpindah layar.
- **Est:** 3.0 · **Dep:** S2.3.2
- **Acceptance criteria:**
  - Panel detail memuat semua field Space; sel Table dapat diedit inline; kartu Board menampilkan field yang dipilih.
  - Pemilih kolom mengelompokkan Management, Schedule, Other, dan Custom fields; pilihan disimpan per view. Grup Baselines tampil nonaktif ("segera hadir") karena baseline di luar M2.
  - Filter dan sort berdasarkan custom field bekerja di URL dan di server (bukan hanya di klien) untuk tipe skalar dan select.
  - Table dengan 500 item dan 10 kolom custom tetap p95 ≤ 1 detik.
- **Tugas:**
  - [ ] Pemilih kolom dan penyimpanan konfigurasi di `views.config`
  - [ ] Filter/sort server untuk `fields` (indeks GIN atau ekspresi, diukur)
  - [ ] Render dan edit sel per tipe di Table
  - [ ] Tampil di panel detail dan Board
  - [ ] Ukur 500 item dan catat di TESTING

## E2.4 Gantt

### S2.4.1 — Tipe view Gantt dan Calendar
- **Est:** 1.0 · **Dep:** M1
- **Sebagai** user, **saya ingin** menambah view Gantt dan Calendar pada project, **agar** bisa memilih tampilan sesuai kebutuhan.
- **Acceptance criteria:**
  - Enum `view_type` memuat `gantt` dan `calendar`; ViewTabs menawarkan keduanya pada "Add view"; view bawaan project baru tetap Table dan Board.
  - Duplikasi, rename, default, dan hapus bekerja seperti view lain.
- **Tugas:**
  - [ ] Migrasi menambah nilai enum (maju saja, kompatibel mundur satu versi)
  - [ ] `openapi.yaml` dan sqlc diregenerasi; tes `TestViewLifecycle` diperluas
  - [ ] ViewTabs dan placeholder layar

### S2.4.2 — Gantt: bar, progres, drag/resize, milestone (VIEW-05A)
- **Sebagai** PM, **saya ingin** melihat dan mengubah jadwal di bagan Gantt, **agar** perencanaan visual.
- **Est:** 4.0 · **Dep:** S2.4.1, S2.2.2
- **Acceptance criteria:**
  - Bar per item dengan hierarki project → task → subitem dapat dilipat; progres % tampil; milestone tampil sebagai penanda.
  - Drag memindahkan tanggal dan resize mengubah durasi; keduanya memanggil `PATCH` dengan `If-Match` dan memperbarui dari respons server. Gagal (409/403) mengembalikan bar dan menampilkan toast.
  - Pengguna dengan hak lihat saja tidak bisa drag (kontrol nonaktif, bukan hanya diabaikan).
  - 500 task dirender dan digulir tanpa jeda yang terlihat (p95 render ≤ 1.5 detik).
  - Bisa dipakai dengan keyboard untuk memilih bar dan menggeser per hari; label ARIA ada.
- **Tugas:**
  - [ ] Adaptor data Item → SVAR dan kembali (tanggal sebagai string `YYYY-MM-DD`, tanpa zona waktu)
  - [ ] Hierarki, lipat/buka, skala hari/minggu/bulan, garis hari ini
  - [ ] Drag/resize → PATCH optimistis + rollback + toast
  - [ ] Mode baca untuk pengguna tanpa hak edit
  - [ ] Ukur 500 task (Chrome) dan catat
  - [ ] Navigasi keyboard dan axe

### S2.4.3 — Gantt: dependensi dan penjadwalan (VIEW-05B)
- **Sebagai** PM, **saya ingin** menarik garis dependensi dan melihat penerus bergeser, **agar** jadwal terhubung.
- **Est:** 3.0 · **Dep:** S2.4.2, S2.1.3
- **Acceptance criteria:**
  - Garis dependensi (FS/SS/FF/SF) digambar dari data server; membuat dan menghapus garis lewat drag dan menu memanggil API S2.1.2.
  - Setelah drag atau perubahan dependensi, bar penerus bergeser sesuai `affected_items` dari server; library tidak menghitung jadwal sendiri.
  - Dependensi yang akan membuat siklus ditolak dengan pesan jelas dan tidak ada perubahan yang tersimpan.
  - Dua pengguna mengedit bersamaan: yang kalah mendapat pesan konflik dan data segar (S1.4.3).
- **Tugas:**
  - [ ] Render link dan pemilih tipe/lag
  - [ ] Buat/hapus dependensi dengan pembaruan dari `affected_items`
  - [ ] Matikan penjadwalan otomatis bawaan library (jadwal hanya dari backend)
  - [ ] Tes Playwright: geser pendahulu → penerus bergeser; siklus ditolak; konflik 409
  - [ ] Realtime: penerima SSE memperbarui bar tanpa memuat ulang

## E2.5 View lain dan Space

### S2.5.1 — Board swimlane dan field kartu (VIEW-03, VIEW-04)
- **Est:** 3.0 · **Dep:** S2.3.3
- **Sebagai** user, **saya ingin** mengelompokkan Board per assignee dan memilih isi kartu, **agar** papan sesuai cara tim bekerja.
- **Acceptance criteria:**
  - Group by None atau Assignee membuat swimlane; item tanpa assignee masuk "Unassigned"; item dengan banyak assignee muncul di tiap lajur dan drag di antara lajur mengubah assignee dengan konfirmasi.
  - Kartu menampilkan field pilihan: cover image (dari lampiran gambar pertama), days in status, lokasi, author, ID, tipe item, tanggal, custom field; pilihan disimpan per view.
  - Drag status tetap optimistis dengan rollback.
- **Tugas:**
  - [ ] Pengelompokan di server atau klien (diputuskan setelah ukur 500 item)
  - [ ] Konfigurasi kartu di `views.config` dan layar pengaturan
  - [ ] `days in status` dari riwayat aktivitas (tambah indeks bila lambat)
  - [ ] Tes Playwright untuk swimlane dan drag lintas lajur

### S2.5.2 — Calendar (VIEW-06)
- **Est:** 2.5 · **Dep:** S2.4.1
- **Sebagai** user, **saya ingin** melihat task pada kalender bulanan, **agar** tenggat terlihat sekilas.
- **Acceptance criteria:**
  - Tampilan bulanan dengan Today, prev, next; filter sama dengan view lain; item tampil dari `start_date` sampai `due_date`; item tanpa tanggal ada di daftar "Unscheduled".
  - Drag item ke hari lain mengubah tanggal (mengikuti S2.1.3); awal pekan mengikuti pengaturan user; libur dari S2.1.1 diberi tanda.
  - Dapat dipakai dengan keyboard; hari dan item punya label ARIA.
- **Tugas:**
  - [ ] Komponen kalender bulanan (tanpa dependensi baru bila cukup dengan CSS grid, sesuai ui-kit)
  - [ ] Pemuatan per rentang bulan dengan paginasi cursor
  - [ ] Drag → PATCH dan pembaruan penerus
  - [ ] Tes Playwright dan axe

### S2.5.3 — Preset filter dan sort Space (SPC-03)
- **Est:** 1.5 · **Dep:** S2.3.3
- **Sebagai** user, **saya ingin** preset All tasks, All active, dan My active, **agar** tidak menyusun filter berulang.
- **Acceptance criteria:**
  - Tiga preset tersedia di Table/Board/Calendar; sort Priority, Due date, Status, Name, Predecessors, Manual order bekerja di server dan tercermin di URL.
  - Sort Priority memakai keputusan M2-1 (kolom `importance`); Predecessors mengurutkan berdasarkan jumlah dan posisi dependensi.
  - Opsi Priority/Importance yang disembunyikan di M1 sekarang tampil, termasuk di My to-do.
- **Tugas:**
  - [ ] Migrasi kolom `importance` (bila M2-1 disetujui) dan field di panel detail
  - [ ] Preset dan sort di endpoint daftar dan `openapi.yaml`
  - [ ] UI preset dan tes

### S2.5.4 — UI multi-lokasi (SPC-10)
- **Est:** 1.5 · **Dep:** M1
- **Sebagai** user, **saya ingin** menambahkan item ke project atau Space lain, **agar** satu pekerjaan muncul di beberapa tempat.
- **Acceptance criteria:**
  - Panel detail menampilkan daftar lokasi; "Add location" memilih Space atau project tujuan yang bisa diakses user; menghapus lokasi terakhir ditolak.
  - Item muncul di Table dan Board setiap lokasi; perubahan di satu tempat terlihat di tempat lain (satu item, bukan salinan).
  - Hapus lokasi tidak menghapus item; siklus hierarki yang dibuat lewat lokasi baru ditolak.
- **Tugas:**
  - [ ] UI di panel detail memakai `POST/DELETE /items/{id}/locations` yang sudah ada
  - [ ] Pemilih tujuan hanya menampilkan Space dan project yang boleh ditulis
  - [ ] Tes Playwright dan tes otorisasi lokasi

## E2.6 Hardening

### S2.6.1 — E2E, aksesibilitas, dan performa M2
- **Est:** 2.0 · **Dep:** semua story M2
- **Sebagai** tim, **saya ingin** bukti end-to-end, **agar** M2 bisa dinyatakan selesai.
- **Acceptance criteria:**
  - Tiga alur Playwright dengan PostgreSQL: (1) atur work schedule → buat task berdependensi → geser pendahulu → penerus bergeser mengikuti libur; (2) buat custom field → isi nilai → tampil di Table, Board, dan Gantt; (3) Calendar dan swimlane dengan drag.
  - `axe` pada Gantt, Calendar, dan pengaturan custom field (terang dan gelap).
  - Matriks otorisasi diperluas ke dependensi dan custom field; tes IDOR lulus.
  - Gantt dan Table 500 task serta rantai dependensi 500 task diukur dan dicatat di TESTING.
  - Runbook pilot diperbarui untuk migrasi M2 dan rollback.
- **Tugas:**
  - [ ] Tiga alur Playwright dan fixture data
  - [ ] Perluasan matriks otorisasi
  - [ ] Benchmark dan pencatatan p95
  - [ ] Perbarui `PILOT_RUNBOOK.md`, `TESTING.md`, `HANDOVER.md`

## 3. Risiko M2

| Risiko | Sinyal | Mitigasi |
|---|---|---|
| Penjadwalan berantai lebih rumit dari spike (SS/FF/SF, lag negatif, libur) | Tabel kasus S2.1.3 terus bertambah | Tulis tes tabel lebih dulu; batasi ke aturan M2-3; jangan tambah mode jadwal lain |
| SVAR MIT kurang untuk kebutuhan (mis. fitur dependensi atau virtualisasi tertutup di PRO) | Drag/link berhenti bekerja pada 500 task | Ukur di awal S2.4.2; fallback: tampilan read-only + edit lewat panel, catat sebagai keputusan baru |
| Filter/sort custom field lambat di `jsonb` | p95 Table > 1 detik di 500 item | Indeks ekspresi per tipe populer; ukur sebelum memilih |
| Perubahan jadwal berantai memicu banyak event SSE dan konflik versi | Toast konflik sering muncul saat dua orang mengedit | Kirim satu event ringkas per transaksi (`affected_items`) |
| Estimasi 41 hari melebihi 30 hari | Story pertama melewati estimasi > 20% | Hitung ulang dari kecepatan nyata; tawarkan penundaan SPC-10 dan sebagian VIEW-04 |

## 4. Yang sengaja tidak masuk M2

Baseline (grup kolom hanya placeholder), actual cost, timesheet (M4), form dan approval (M3), dashboard dan report (M5), template Space (M6), work schedule per user, tombol "jadwal ulang semua".
