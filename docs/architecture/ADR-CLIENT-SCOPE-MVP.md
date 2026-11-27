# ADR: Client scope dan arsitektur Platform Ready MVP

| | |
|---|---|
| **ID** | ADR-CLIENT-SCOPE-MVP (usulan) |
| **Status** | **PROPOSED.** Belum disetujui Product Owner secara keseluruhan. Satu bagian disetujui untuk implementasi B1 (lihat §10) |
| **Tanggal** | 27 November 2026 |
| **Basis** | Rencana MVP v0.2 (`docs/roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md`), B0-01..B0-05 (`docs/roadmap/PMS_PLATFORM_IMPACT_ASSESSMENT.md`, `docs/qa/releases/v0.14.0/CANDIDATE.md`) |
| **Baseline kode** | Rilis `v0.13.0` = `72f9902`. Cabang kerja `docs/restructure-mvp-roadmap` @ HEAD saat dokumen ini ditulis (lihat MVP_SCOPE.md §1) |
| **Menggantikan** | Tidak ada. Mengamandemen sequencing D-55 **usulan** (lihat §2.1) |

Dokumen ini menggabungkan hasil audit B0-01..B0-05. Tidak ada kode yang diubah untuk menyusunnya.

---

## 1. Konteks

PMS adalah aplikasi manajemen proyek self-hosted, single-tenant (D-01). Satu instance dipakai oleh satu organisasi. Untuk operasi konsultansi, satu instance internal perlu mengelola beberapa klien sebagai **subjek data**, dengan batas akses yang bisa dibuktikan.

Audit menemukan:

1. Tidak ada entitas klien. Batas akses hanya ada lewat grant Space dan share item (B0-03 AUTH-001..006).
2. Otorisasi item menggabungkan izin dari semua lokasi (`policy/item.go:51`). Penyaringan per klien belum ada.
3. Jalur akses langsung ke tabel tersebar (B0-03 §3), termasuk jalur `NEEDS_TRACE` yang belum dibuktikan.
4. Field Customer ada sebagai field `single_select` dengan nama, bukan kunci tetap (B0-04 §3).
5. Ada relasi lintas Space yang diizinkan skema: dependensi, relasi field, lokasi, formulir (B0-04 §2, MIG-03..07, MIG-10).
6. `items.owner_id` sudah berarti pengguna (B0-04 MIG-13).
7. `policyCache` di `httpserver/access_policy.go` hanya untuk IP allowlist dan OTP. Bukan cache otorisasi (B0-03 §1.1).

---

## 2. Keputusan

Setiap keputusan diberi status. **Tidak ada** yang APPROVED.

### 2.1 Arsitektur platform

| ID | Keputusan | Status |
|---|---|---|
| ARCH-01 | Tetap Go, PostgreSQL, React/Vite. Satu modular monolith dan satu deployment (`deploy/`) | PROPOSED |
| ARCH-02 | **D-01 dipertahankan.** Instance adalah batas tenant. Tidak ada entitas Tenant | PROPOSED |
| ARCH-03 | **Amandemen D-55 (usulan).** Fondasi klien dan registry statis dimajukan ke MVP ini. Business Object Registry runtime tetap ditunda sampai kebutuhan vertical slice AIMS terbukti; bukan urutan wajib sesudah AIMS. Acuan: `DECISIONS.md` D-55 (cakupan rilis 1.0 = M0–M12, M13 backlog) | Bagian registry statis: **DISETUJUI untuk implementasi B1** (PO, 27 Nov 2026). Bagian lain: PROPOSED, perlu amandemen formal |
| ARCH-04 | PMS satu-satunya solution aktif. Tidak ada Solution Switcher | PROPOSED |
| ARCH-05 | Solution Registry statis (paket `server/internal/solutions`, B1). ID `pms`, nama "Projects", `Required` dan `Enabled` = true, capability metadata `work_management`. Didefinisikan di kode, salinan immutable, divalidasi saat start | **DISETUJUI untuk B1** (PO, 27 Nov 2026). Terimplementasi di `server/internal/solutions` dan divalidasi di `run()` sebelum `database.Open` |
| ARCH-06 | Tidak ada registry database, installer, atau runtime plugin dinamis | PROPOSED |
| ARCH-07 | URL, shell aplikasi, sidebar, dan route default (`/dashboards`) tidak berubah | PROPOSED |
| ARCH-08 | **Capability metadata registry tidak memberi izin RBAC.** Hanya dokumentasi dan validasi start | PROPOSED |

### 2.2 Kepemilikan klien

| ID | Keputusan | Status |
|---|---|---|
| OWN-01 | Entitas native `clients` adalah Customer Master kanonik. Setiap klien punya UUID tidak berubah dan kode stabil unik (dinormalisasi) | PROPOSED |
| OWN-02 | Keanggotaan `client_memberships` adalah **gerbang kelayakan**, bukan grant izin resource | PROPOSED |
| OWN-03 | Kepemilikan Space dan item memakai `spaces.client_id` dan `items.client_id`, atau asosiasi eksplisit yang setara. Nama fisik final di B3-01 | PROPOSED |
| OWN-04 | **`items.owner_id` tidak dipakai untuk klien.** Kolom itu tetap berarti pengguna | PROPOSED |
| OWN-05 | Tiga status kepemilikan: **CLIENT** (klien kanonik), **INTERNAL** (ditandai eksplisit), **UNRESOLVED** (belum ditetapkan) | PROPOSED |
| OWN-06 | **NULL tidak pernah berarti INTERNAL.** Hanya status INTERNAL eksplisit yang berarti internal | PROPOSED |
| OWN-07 | Satu Space = satu ruang lingkup kepemilikan | PROPOSED |
| OWN-08 | Kepemilikan item stabil, tidak mengikuti lokasi yang sedang dipilih | PROPOSED |
| OWN-09 | Setiap lokasi item harus cocok dengan ruang lingkup kepemilikan item | PROPOSED |
| OWN-10 | Multi-lokasi dalam **satu** klien tetap didukung sesuai izin yang ada | PROPOSED |
| OWN-11 | Lokasi, relasi, dependensi, move, dan share **lintas klien atau klien-ke-internal ditolak** | PROPOSED |
| OWN-12 | Kepemilikan UNRESOLVED dikarantina dari akses biasa setelah enforcement. Kegagalan menutup, bukan terbuka | PROPOSED |
| OWN-13 | **Kolom Customer pada field Workspace tidak boleh menjadi sumber kedua yang bisa diedit** setelah migrasi ke klien kanonik. Field itu dipensiunkan atau hanya-baca sebagai tampilan | PROPOSED |
| OWN-14 | Opsi Customer legacy adalah **referensi migrasi**, bukan UUID klien kanonik | PROPOSED |

### 2.3 Otorisasi

| ID | Keputusan | Status |
|---|---|---|
| AUTHZ-01 | **D-17.1 dipertahankan.** Explicit Access Only. Workspace Admin tidak mendapat akses konten | PROPOSED (D-17.1 sudah ada di DECISIONS.md) |
| AUTHZ-02 | Gerbang klien diterapkan **per lokasi sebelum union** izin (`itemsCaps`, `spaceRows`) | PROPOSED |
| AUTHZ-03 | Semantik Full, Editor, Read Only, dan Limited tidak berubah | PROPOSED |
| AUTHZ-04 | **Aturan assignment:** menugaskan orang lain butuh hak share di setiap jalur dari item ke Space. Diimplementasi oleh `policy.SharedOnEveryPath` (`policy/share_paths.go:13`). Sumber keputusan: **DEC-10** (APPROVED, 26 Nov) dan **D-W3.1** (PM = akses Full ke Space) | PROPOSED (rujukan sudah APPROVED; penerapan ke klien belum) |
| AUTHZ-05 | Penyaringan lokasi berdasarkan klien **tidak boleh** menghapus jalur restriktif sehingga assignment menjadi lebih luas | PROPOSED |
| AUTHZ-06 | Kepemilikan tidak konsisten **gagal tertutup** (deny) | PROPOSED |
| AUTHZ-07 | Membership klien **tidak** memberi akses ke konten. Membership dicek bersama grant, bukan menggantikannya | PROPOSED |
| AUTHZ-08 | Pencabutan membership berlaku segera pada pembacaan dan aksi berikutnya. Tidak ada cache keputusan yang perlu diinvalidasi saat ini (lihat §3) | PROPOSED |

### 2.4 Keputusan K-25 sampai K-28

Nomor ini dari rencana v0.2 dan B0-05. Tidak ada di `DECISIONS.md`. Lihat §4.

| ID | Topik | Keputusan usulan | Status |
|---|---|---|---|
| **K-25** | Personal Space | Tetap privat dan berlingkup internal. Item klien **tidak boleh** punya lokasi di Personal Space. Tugas klien yang di-assign tetap bisa dipantau lewat My Work dan saved view yang diizinkan, tanpa dipindahkan. Visibilitas Personal Space bukan izin membaca konten klien | PROPOSED, perlu Product Owner |
| **K-26** | Blueprint dan automation tingkat akun | Blueprint generik boleh dipakai ulang lintas klien. Data khusus klien, field terbatas, lampiran, dan referensi rahasia **tidak boleh** lintas batas klien. Definisi automation tingkat akun tetap didukung. Setiap eksekusi memverifikasi ulang aktor, kepemilikan target, dan kapabilitas. Cakupan rule **tidak** memberi akses | PROPOSED, perlu Product Owner |
| **K-27** | Request form | Routing lintas Space hanya dalam ruang lingkup kepemilikan yang sama dan izin yang ada. Pengiriman publik dan unggahan eksternal tetap nonaktif atau sangat dibatasi untuk MVP internal | PROPOSED, perlu Product Owner |
| **K-28** | Otoritas klasifikasi INTERNAL | Hanya setelah persetujuan tertulis Product Owner atau pemilik bisnis yang ditunjuk. **Nama pemberi persetujuan tidak ditentukan di dokumen ini.** Nilai Customer kosong **tidak** berarti INTERNAL | PROPOSED, perlu nama pemberi persetujuan |

### 2.5 Pydio

| ID | Keputusan | Status |
|---|---|---|
| **K-22** | Referensi folder memakai field Workspace `Pydio Folder` yang ada. Satu referensi kanonik per project. Tidak ada tabel baru kecuali bukti menunjukkan perlu | PROPOSED |
| **K-21** | Listing read-only: **utamakan identitas pengguna terdelegasi** dengan ACL Pydio. Token layanan bersama **tidak otomatis aman**. Listing tidak boleh dibuka hanya karena item PMS terlihat. Identitas layanan sempit butuh persetujuan eksplisit dan bukti otorisasi per pengguna. Bila gagal, listing dimatikan dan link-first menjadi fallback **usulan** yang butuh persetujuan Product Owner | PROPOSED |
| PY-01 | D-08 (batasan Pydio): Pydio tetap layanan dokumen kerja. Storage lampiran yang sudah terverifikasi tidak berubah. Tidak ada unggah, overwrite, sinkron, migrasi storage, atau jaminan versi historis | PROPOSED |

Catatan: **D-08** di `DECISIONS.md` berarti "Isi PRD v0.3". Keputusan D-08 tentang Pydio berasal dari dokumen blueprint LENS lain. Lihat §4.

### 2.6 Cakupan produksi (usulan, belum disetujui)

| ID | Usulan | Status |
|---|---|---|
| PROD-01 | Internal production saja | PROPOSED |
| PROD-02 | Konektor GitHub dan Google Calendar **nonaktif** | PROPOSED |
| PROD-03 | SSO ditunda. Autentikasi kata sandi dan 2FA yang sudah ada direkomendasikan | PROPOSED |
| PROD-04 | RT-32 dan RT-15 ditunda | PROPOSED |
| PROD-05 | Email diverifikasi dengan **SMTP provider yang akan dipakai**, bukan provider uji tertentu | PROPOSED |
| PROD-06 | Endpoint publik yang tidak disetujui dinonaktifkan secara efektif | PROPOSED |
| PROD-07 | Verifikasi DEF-045 wajib sebelum produksi (audit instance yang terpasang) | PROPOSED |
| PROD-08 | Backup off-host, restore, dan monitoring wajib | PROPOSED |

---

## 3. Koreksi dari audit

| Temuan | Koreksi |
|---|---|
| `policyCache` diasosiasikan dengan otorisasi (rencana B3-07) | **Salah.** `policyCache` (`httpserver/access_policy.go:33`) hanya untuk IP allowlist dan kebijakan OTP. Tidak ada cache keputusan Space atau item. B3-07 harus memastikan pencabutan membership langsung berlaku, tanpa mengubah cache IP/OTP |
| `DEC-06 Rev.1` dikutip sebagai aturan assignment | **Tidak sesuai dengan `DECISIONS.md`.** DEC-06 Rev.1 adalah keputusan **biaya** (actual cost dari time entry Approved). Aturan assignment tercatat di **DEC-10**. Lihat §4 |
| `D-08` dikutip sebagai keputusan Pydio | **Tidak sesuai dengan `DECISIONS.md`.** D-08 di sana adalah "Isi PRD v0.3". Keputusan Pydio berasal dari blueprint LENS. Lihat §4 |

---

## 4. Ketidaksesuaian ID dengan catatan repo

Sesuai aturan, ID yang ada **dipertahankan** dan ketidaksesuaian dicatat. Tidak ada ID yang diubah.

| ID yang dikutip | Yang tercatat di repo | Tindakan |
|---|---|---|
| DEC-06 Rev.1 (sebagai aturan assignment) | `DECISIONS.md`: DEC-06 Rev.1 = biaya. Assignment = **DEC-10** | Rujuk DEC-10 untuk assignment. Perbaiki rujukan di rencana v0.2 (B3-06) dan dokumen RC3 bila perlu |
| D-08 (sebagai keputusan Pydio) | `DECISIONS.md`: D-08 = isi PRD v0.3 | Gunakan label "D-08 (blueprint LENS)" untuk keputusan Pydio. Tidak mengubah `DECISIONS.md` |
| K-20..K-29 | Tidak ada di `DECISIONS.md`. Hanya di rencana v0.2 dan B0-05 | Ditambahkan sebagai bagian PROPOSED di `DECISIONS.md` (lihat MVP_SCOPE.md §8) |
| D-17.1 | Ada, APPROVED | Dipertahankan |
| D-55 | Ada, APPROVED (cakupan rilis 1.0 = M0–M12) | Amandemen **usulan**; butuh formal |
| D-01 | Ada, APPROVED | Dipertahankan |
| D-82, D-83, D-90 | Ada | Tidak diubah |

---

## 5. Konsekuensi

### 5.1 Positif

- Isolasi klien dibangun di lapisan policy, bukan di UI. Filter yang tersembunyi bukan keamanan.
- Kepemilikan stabil membuat migrasi dan audit lebih mudah dibuktikan.
- Status UNRESOLVED mencegah data yatim tampil sebagai internal.

### 5.2 Negatif dan risiko

- **Analitik operasional** (`analytics.go:364–371`) melewati gerbang per item. Harus diubah sebelum produksi (B3-15).
- **Jalur tulis tanpa guard di badan** (B0-03 F-03) bergantung pada pemanggil. Harus ditelusuri di B3.
- **Share dan grant polimorfik** (`shares.target_id`, `custom_field_grants.subject_id`) tidak punya FK. Keputusan K-29 belum dibuat.
- **Ada dua jalur assignment** (create dan PATCH), keduanya lewat `SharedOnEveryPath`. Tidak bercabang, tetapi harus diuji bersama.
- Pelaksanaan membutuhkan data operasional yang belum tersedia (B0-04, B0-05).

### 5.3 Yang tidak dibuat

- Entitas Tenant.
- Runtime BOR, MasterRecord, atau tabel registry.
- Solution Switcher, installer, atau plugin runtime.
- Client Hub penuh, Client 360, atau portal klien.
- Pengguna eksternal.
- Sinkron atau tulis Pydio.

---

## 6. Alternatif yang dipertimbangkan

| Alternatif | Alasan ditolak |
|---|---|
| Membership klien sebagai grant izin | Memberi akses konten tanpa grant. Melanggar D-17.1 |
| Field Customer sebagai sumber kepemilikan | Bisa diedit per item. Tidak punya identitas stabil. Lihat OWN-13 |
| `items.owner_id` untuk klien | Sudah berarti pengguna. Lihat OWN-04 |
| NULL = INTERNAL | Data tanpa klien akan lolos sebagai internal. Lihat OWN-06 |
| Multi-tenant (tabel per tenant) | Melanggar D-01 dan tidak diperlukan |
| Pemetaan otomatis dari nama Space atau project | Tidak deterministik dan tidak bisa dibuktikan. Lihat spesifikasi B0-05 |

---

## 7. Syarat sebelum status menjadi ACCEPTED

1. Product Owner menyetujui K-25..K-28 dan K-21..K-22 secara tertulis.
2. Pemberi persetujuan INTERNAL ditetapkan (K-28).
3. Amandemen D-55 diputuskan secara formal.
4. Pemilik bisnis menyetujui pemetaan klien (B0-05) setelah data operasional tersedia.
5. Rujukan DEC-06/DEC-10 dan D-08 diperbaiki di dokumen yang masih memakainya.

---

## 7a. Kontrak ekstensi solution (B1-06)

- `pms` adalah solution pertama dan satu-satunya yang terdaftar. Tidak ada solution kedua di build ini.
- Solution kedua hanya boleh ditambahkan lewat **registrasi tepercaya di kode**: entri di `solutions.Builtin()` dan aturan yang sesuai di `solutions.Validate`, dengan tes. Tidak ada pemuatan dinamis.
- Metadata statis (ID, nama, capability) **tidak memberi izin**. Izin tetap di `policy`. ARCH-08 berlaku.
- Solution Switcher hanya diperkenalkan ketika ada lebih dari satu solution aktif (rencana §13).
- Business Object Registry runtime tetap ditunda sampai vertical slice AIMS direncanakan (lihat ARCH-03).

## 7b. Otorisasi Product Owner (27 Nov 2026)

Product Owner menyetujui **implementasi B1 saja**: B1-01, B1-02, B1-03, B1-05, B1-06, dan bagian registry statis dari amandemen D-55. B1-04 ditunda karena tidak ada kebutuhan API operator untuk MVP ini.

Persetujuan ini **tidak** mencakup: Client Scope ADR secara keseluruhan, K-20..K-29, kesiapan migrasi, kesiapan produksi, atau batch B2 dan seterusnya.

## 7c. Otorisasi PO untuk skema B2 (27 Nov 2026, Prompt #007)

Product Owner menyetujui **skema B2-01 dan B2-02 saja**: migrasi `clients` dan `client_memberships`, dengan desain berikut.

- `clients` adalah Customer Master kanonik. UUID dan kode ternormalisasi bersifat stabil.
- `client_memberships` adalah kelayakan, bukan izin resource.
- `items.owner_id` tetap referensi pengguna. Tabel dan kolom yang sudah ada tidak diubah.

Tidak disetujui: B2-03 dan seterusnya, migrasi kepemilikan, K-25..K-29, kesiapan produksi.

**Implementasi (migrasi `server/migrations/00363_clients.sql`, aditif):**

| Aspek | Keputusan |
|---|---|
| Identitas klien | `id uuid` dengan `gen_random_uuid()`. Kode unik dan kanonik: tidak kosong, tanpa spasi di awal atau akhir, huruf besar |
| Kode tidak berubah | Trigger `BEFORE UPDATE OF code` (`clients_code_no_change`), dengan pola yang sama seperti `approval_events`. Dicek di database, bukan hanya di aplikasi |
| Nama | Tidak kosong (ada karakter non-spasi). Tidak ada aturan panjang atau regex lain |
| Status | `active` (default) atau `archived`. `archived_at` terisi **tepat ketika** status `archived` |
| Model membership | **Satu baris per pemberian.** Cabut = ubah status menjadi `revoked` dan isi `revoked_at`. Beri lagi = baris baru. Baris tidak pernah dihapus, jadi riwayat tetap ada tanpa bergantung pada `audit_events` |
| Satu aktif per pasangan | Indeks unik parsial `client_memberships_one_active_idx (client_id, user_id) WHERE status = 'active'` |
| Indeks lain | `client_memberships_active_user_idx (user_id) WHERE status='active'` untuk daftar klien yang layak per pengguna. `client_memberships_client_idx (client_id)` untuk pemeriksaan FK restrict |
| Penghapusan klien | `ON DELETE RESTRICT` dari `client_memberships`. Klien dengan riwayat membership tidak bisa dihapus |
| Penghapusan pengguna | `ON DELETE RESTRICT` untuk semua referensi pengguna baru (`created_by`, `granted_by`, `revoked_by`, `archived_by`, `user_id`). Pengguna yang tercatat tidak bisa dihapus. **Ini menyimpang** dari konvensi `ON DELETE SET NULL` untuk kolom audit yang ada, karena membership dan kepemilikan klien tidak boleh kehilangan aktornya. Pengguna dinonaktifkan, bukan dihapus |
| Kolom aktor wajib | `created_by` dan `granted_by` NOT NULL. Keduanya selalu diisi oleh aksi admin |

**Yang tidak dilakukan migrasi ini:** tidak ada perubahan `spaces`, `items`, `users`, `groups`, `shares`, policy, atau RBAC. Membership tidak memberi izin, tidak mengaktifkan admin, dan tidak mendaftarkan pengguna secara otomatis.

## 7e. Domain service B2-03 (27 Nov 2026, Prompt #008)

Product Owner menyetujui **domain service B2-03 saja**: paket `server/internal/clients`. Tidak ada endpoint, UI, audit, atau enforcement resource. B2-04, B2-05, dan B2-06 belum disetujui.

**Batas kepercayaan.** Paket ini mempercayai pemanggilnya. Ia memeriksa input dan aturan siklus hidup, tetapi **tidak** memeriksa siapa yang boleh memanggil. Otorisasi di batas HTTP (B2-04) dan rute (B2-06) wajib ada sebelum fungsi ini diekspos. Membership di sini tetap kelayakan saja.

**Operasi:**

| Operasi | Perilaku |
|---|---|
| `Create` | Kode dinormalisasi (trim, huruf besar). Duplikat ditolak oleh unique constraint `clients_code_key`, bukan cek terlebih dulu. Hanya membuat klien aktif |
| `Get` | Berdasarkan UUID. Mengembalikan klien arsip juga. Tidak ditemukan: `ErrNotFound` |
| `List` | Filter `All`/`Active`/`Archived`. Urut `code, id`. Batas default 50, maksimum 200, di luar itu ditolak |
| `Update` | Hanya nama dan nama pendek. Tidak ada field kode atau UUID di kontrak. Nama pendek kosong menghapusnya. Klien arsip: `ErrArchived` (tidak diam-diam dipulihkan) |
| `Archive` | Status menjadi arsip, dengan `archived_at` dan `archived_by`. Arsip ulang **tidak** menimpa metadata pertama. Riwayat membership tidak disentuh |
| `ListMembers` | `history=false`: hanya aktif. `history=true`: termasuk yang dicabut. Batas 500. Tidak memilih kredensial atau field pengguna lain |
| `Grant` | Hanya untuk klien aktif. Pengguna harus aktif dan bertipe `admin` atau `regular`. Grant ulang yang masih aktif **mengembalikan baris yang sama** tanpa mengubah actor atau waktu |
| `Revoke` | Mengubah baris aktif menjadi dicabut dan mengisi `revoked_at`/`revoked_by`. Ulang: baris dicabut dikembalikan tanpa diubah. Tidak pernah ada: `ErrNotMember`. Boleh untuk klien arsip dan pengguna yang sudah tidak layak |

**Kelayakan pengguna.** Sama dengan aturan staf internal di `policy.CanBrowseDirectory`: `admin` dan `regular`, dengan status `active`. Tipe `external`, `collaborator`, dan `viewer` ditolak sampai K-19 diputuskan. Status `invited` dan `deactivated` ditolak untuk grant baru, tetapi tetap boleh dicabut.

**Strategi transaksi dan konkurensi:**

- Urutan kunci: **klien dulu, lalu pengguna atau membership.**
- Grant mengambil `FOR SHARE` pada baris klien. Arsip mengambil `FOR UPDATE`. Dengan begitu grant dan arsip saling menunggu: grant yang berjalan setelah arsip di-commit ditolak `ErrArchived`.
- Grant juga mengambil `FOR SHARE` pada baris pengguna, sehingga deaktivasi menunggu grant yang berjalan.
- Dua grant bersamaan untuk pasangan yang sama: yang kalah mendapat pelanggaran `client_memberships_one_active_idx` dan mencoba sekali lagi, lalu membaca baris pemenang. Hanya nama indeks itu yang dianggap "sudah ada". Kesalahan lain tidak ditelan.
- Pembuatan klien bersamaan: unique constraint adalah otoritas. Satu berhasil, sisanya `ErrDuplicateCode`.

**Pemetaan error.** Hanya nama constraint yang terverifikasi yang dipetakan ke error domain: `clients_code_key`, `clients_code_check`, `clients_name_check`, `clients_short_name_check`, `client_memberships_one_active_idx`, dan kesalahan FK ke pengguna (`ErrUserNotFound`). Error lain dibungkus sebagai error database generik. Tidak ada SQL atau nilai sensitif yang dikembalikan ke pengguna.

**Kompatibilitas siklus hidup pengguna (B2-03 §4).** Diperiksa di kode:

- Tidak ada jalur aplikasi yang menghapus pengguna dengan `DELETE FROM users`. Penghapusan hanya ada di tes (`schema_test.go`, `housekeeping_test.go`), dan pengguna tersebut tidak punya membership.
- Deaktivasi memakai status (`deactivated`), bukan penghapusan. FK `RESTRICT` dari migrasi 00363 **tidak memblokir** operasi yang didukung.
- Kalau nanti ada penghapusan pengguna yang tercatat di membership, operasi itu akan gagal dengan FK. Ini disengaja. Penanganannya adalah deaktivasi, bukan penghapusan. Ditulis sebagai catatan untuk siapa pun yang menambahkan fitur hapus pengguna.

**Bukti tes:** `server/internal/clients/clients_integration_test.go`. CL-01..CL-10, MEM-01..MEM-09, MEM-12, konkurensi pembuatan, MEM-10 (grant bersamaan), dan MEM-11 (grant menunggu arsip). Lihat MVP_SCOPE §13.

**Pemeriksaan mutasi:** tanpa `FOR SHARE` pada grant, MEM-11 gagal ("the grant never waited on the client lock"). Dengan penguncian dikembalikan, lulus.

## 7f. Authorization, audit, dan API B2-04..06 (27 Nov 2026, Prompt #009)

Product Owner menyetujui **B2-04, B2-05, dan B2-06**: manajemen klien hanya untuk Workspace Admin, membership hanya untuk admin dan staf aktif (partner ditunda), setiap mutasi wajib punya identitas actor.

**Authorization (B2-04).** Setiap operasi memakai `requireUser(requireAdmin(...))`, yaitu guard admin yang sudah ada (`httpserver/admin.go`). Urutan: sesi dulu (401), lalu kapabilitas admin (403), baru data diperiksa. Membership, Space Admin, dan grant resource **tidak** memberi akses manajemen. Actor selalu dari sesi, tidak pernah dari body.

**Audit (B2-05).** Lima aksi `client.created`, `client.updated`, `client.archived`, `client.member_granted`, `client.member_revoked`, terdaftar di `httpserver/audit.go` dan didefinisikan di paket `clients`.

- Domain menerima `Recorder` yang dipanggil **di dalam transaksi yang sama** dengan mutasi, sebelum commit. Jika audit gagal, mutasi di-rollback (AUD-05). Tidak ada mutasi yang tercommit tanpa audit-nya.
- Event hanya ditulis saat keadaan berubah. Grant ulang, revoke ulang, arsip ulang, dan PATCH yang nilainya sama tidak menulis event (AUD-03).
- Detail memuat hanya data non-rahasia: kode, nama field yang berubah, ID membership, ID pengguna target.

**API (B2-06).** Delapan operasi di `api/openapi.yaml`: `GET/POST /clients`, `GET/PATCH /clients/{clientId}`, `POST /clients/{clientId}/archive`, `GET/POST /clients/{clientId}/members`, `DELETE /clients/{clientId}/members/{userId}`. Tidak ada hapus atau unarchive.

| Kondisi | Respons |
|---|---|
| Tidak ada sesi | 401 |
| Bukan admin | 403 |
| UUID tidak valid, body tidak valid, field tidak dikenal (termasuk `code` atau `id` pada PATCH) | 400 |
| Klien atau pengguna tidak ditemukan, membership belum pernah ada | 404 |
| Kode duplikat, klien diarsipkan | 409 |
| Input domain tidak valid, pengguna tidak layak (`user_not_eligible`), limit di luar batas | 422 |
| Kesalahan database tak terduga | 500 `internal error`, dicatat di log, detail tidak ditampilkan |

Grant dan revoke mengembalikan 200. Grant ulang yang masih aktif mengembalikan baris yang sama. Daftar anggota memakai `include_revoked=true` untuk menyertakan riwayat.

**Artefak generated.** `server/internal/apigen/openapi.gen.go` dan `web/src/api/schema.d.ts` digenerate ulang dengan alat yang sudah ada (`go tool oapi-codegen`, `npm run generate`). Generator menghasilkan ulang tanpa perubahan lain: diverifikasi dengan membandingkan hasil dengan spec HEAD (cocok) dan generate dua kali (identik). Di `schema.d.ts` tidak ada baris yang dihapus. Di Go, git menampilkan banyak blok yang berpindah urutan. Semua baris lama masih ada dalam file baru.

**Batas.** Tidak ada Customer Picker (B2-08). Tidak ada enforcement klien pada Space, item, atau resource (B3). Daftar klien belum punya cursor: batas 200 per halaman, dan pengguna dengan lebih banyak klien perlu paginasi (B2-07 atau berikutnya).

**Tes:** `server/internal/httpserver/clients_api_integration_test.go`. Hasil ada di MVP_SCOPE §14.

## 7g. Antarmuka Customer Settings B2-07 (27 Nov 2026, Prompt #010)

Product Owner menyetujui **B2-07 saja**: halaman Pelanggan di Settings untuk delapan operasi REST yang sudah ada. Tidak ada endpoint, izin, atau migrasi baru.

- **Rute:** `/settings/customers` (daftar) dan `/settings/customers/{clientId}` (detail, tab Umum dan Anggota). Item menu ada di grup "Account management", hanya untuk admin.
- **Guard:** sama dengan halaman admin lain (`settings.adminOnly` untuk non-admin). Guard ini **tampilan** saja. Otorisasi sebenarnya tetap di backend (B2-04).
- **Pola yang dipakai ulang:** `AdminPageHeader`, `AdminToolbar`, `AdminStatus`, `AdminEmptyState`, `ConfirmDialog`, `Dialog`, `ErrorState`, `LoadingState`, dan `api()`. Tidak ada komponen atau dependensi baru.
- **Keanggotaan** ditampilkan sebagai kelayakan, dengan teks yang menegaskan bahwa izin Space dan item dikelola terpisah. Tidak ada kontrol Full, Editor, Read Only, atau Limited.
- **Picker anggota** hanya menawarkan admin dan staf aktif, dan menyembunyikan yang sudah menjadi anggota aktif. Tampilan ini tidak menggantikan aturan backend.
- **Batas:** daftar hanya satu halaman (maksimal 200, tanpa cursor). Pencarian pada daftar berlaku pada hasil yang dimuat. Tidak ada Customer Picker untuk pembuatan Space atau project (B2-08).

**Bukti:** `web/tests/customers.spec.ts` (6 tes), lihat MVP_SCOPE §15.

## 7h. Lookup kelayakan dan Customer Picker B2-08 (27 Nov 2026, Prompt #011)

Product Owner menyetujui **B2-08 saja**: lookup kelayakan customer dan picker yang dapat dipakai ulang. Tidak ada migrasi, tidak ada perubahan izin pada delapan operasi yang ada, dan picker belum dipasang di form Space atau Project.

- **Endpoint:** `GET /api/v1/clients/eligible`. Segmen literal ini mengalahkan `/clients/{id}` (EL-15). Parameter: `scope` (`membership`, default, atau `administrative`), `q` (maksimal 100 karakter, dicocokkan pada nama, kode, dan nama pendek), dan `limit` (default 50, maksimal 200).
- **Mode membership (default):** untuk setiap pengguna yang masuk. Hanya customer aktif dengan membership aktif untuk pemanggil, dan pemanggil harus aktif dengan `user_type` admin atau regular. Filter ini ada di SQL (`eligibleOptionsSQL`), sebelum pencarian dan `LIMIT`. Membership yang dicabut dan baris riwayat tidak memenuhi syarat. Bentuk `EXISTS` menjaga satu baris per customer setelah cabut dan beri ulang.
- **Mode administrative:** memakai `requireAdmin`, guard yang sama dengan endpoint admin. Mengembalikan semua customer aktif. Lookup ini tidak membuat membership, akses Space, atau akses item (EL-08, EL-14).
- **Identitas:** diambil dari sesi atau API key (`actorOf`). Tidak ada user ID dari body atau query.
- **Respons:** `{items:[{id, code, name, short_name?}]}`. Tidak ada data membership, audit, jumlah resource, Space, atau Project (EL-12).
- **Error:** 401 tanpa sesi; 403 untuk administrative oleh non-admin; 400 untuk `scope` tidak dikenal; 422 untuk `limit` atau `q` di luar batas; 500 generik untuk kegagalan database.
- **Picker:** `web/src/components/customer-picker.tsx`. Hanya menampilkan opsi dari lookup. Cache key `["clients","eligible",userId,scope,q]` memisahkan pengguna dan mode. Logout sudah memanggil `queryClient.clear()`. Pilihan yang tidak lagi tersedia tampil sebagai "tidak tersedia", tidak diganti, dan tidak diklaim valid (`selectionStatus`).
- **Tidak ada izin baru.** Kelayakan bukan izin konten. Enforcement ownership tetap di B3, dan picker harus dipakai ulang di sana.

**Bukti:** lihat MVP_SCOPE §16.

## 7i. Skema kepemilikan B3-01 dan fondasi constraint B3-02 (27 Nov 2026, Prompt #014)

Product Owner menyetujui **B3-01 dan B3-02 saja**, dengan K-23, K-24, dan K-25, serta bagian fondasi D-55. Tidak ada enforcement kebijakan, tidak ada perubahan create, move, copy, atau restore, dan tidak ada backfill operasional.

**Migrasi:** `server/migrations/00364_client_scope.sql` (aditif).

- **Kolom:** `spaces.client_id`, `items.client_id` (uuid, nullable, FK ke `clients` dengan `ON DELETE RESTRICT`), dan `scope_state` (`text NOT NULL DEFAULT 'unresolved'`) pada kedua tabel.
- **Nilai state:** `client` (wajib `client_id`), `internal` (`client_id` harus NULL), dan `unresolved` (`client_id` harus NULL). Nilai lain ditolak. NULL ditolak oleh `NOT NULL`.
- **Constraint:** `spaces_scope_state_valid`, `spaces_scope_matches_client`, `spaces_personal_is_internal` (`kind <> 'personal' OR scope_state = 'internal'`), `items_scope_state_valid`, dan `items_scope_matches_client`. Semua CHECK dirancang tanpa jebakan NULL, karena kolom yang diuji selalu non-NULL atau diuji dengan `IS [NOT] NULL`.
- **Trigger Personal Space:** `spaces_personal_scope_internal` memaksa `scope_state = 'internal'` untuk `kind = 'personal'`. Kode yang ada membuat Personal Space dengan `INSERT ... kind='personal'` tanpa field kepemilikan (`httpserver/server.go`). Trigger ini memenuhi K-25 tanpa mengubah kode itu. Trigger tidak pernah menetapkan `client`.
- **Baris lama:** semua Space dan item lama berstatus `unresolved` dengan `client_id` NULL. Tidak ada tebakan dari nama Space, nama Project, label custom field, atau lokasi. Personal Space lama ditandai `internal` karena identitasnya sudah pasti dari `kind`. Item lama di Personal Space **tidak** diklasifikasikan otomatis, karena item itu bisa memiliki lokasi lain. Pembaruan ini memakai `DISABLE TRIGGER spaces_set_updated_at` sementara, sehingga `updated_at` tidak berubah.
- **Indeks:** `spaces_client_idx` dan `items_client_idx` (non-partial). Keduanya dipakai untuk pemeriksaan FK `RESTRICT` saat Customer dihapus. Pola ini mengikuti `client_memberships_client_idx`. Tidak ada indeks komposit spekulatif.
- **Arsip:** mengarsipkan Customer tidak mengubah `client_id`, dan penghapusan Customer yang masih dirujuk ditolak (OWN-DB-13, OWN-DB-14).
- **Down:** menghapus constraint, trigger, fungsi, indeks, dan kolom yang baru saja ditambahkan. Tidak memakai `CASCADE`. Hanya diuji pada basis data sementara. Rollback produksi tetap milik B3-21 dan B6.

**Invarian lintas tabel (B3-02 lapisan B). Belum ditegakkan.** Setiap lokasi item aktif harus berada di Space dengan kepemilikan yang kompatibel:

| Kepemilikan item | Kepemilikan Space | Hasil |
|---|---|---|
| Client A | Client A | Kompatibel |
| Client A | Client B | Tidak kompatibel |
| Client A | Internal | Tidak kompatibel |
| Internal | Client A | Tidak kompatibel |
| Internal | Internal | Kompatibel secara kepemilikan; izin tetap diperiksa |
| Client | Personal Space | Tidak kompatibel |
| Unresolved | Apa pun | Tidak memenuhi syarat untuk penulisan normal setelah enforcement |

Invarian ini tidak bisa berupa CHECK sederhana, karena menyangkut dua tabel dan dapat berubah secara bersamaan. Saat ini belum ada item `client`, jadi pelanggaran belum mungkin terjadi pada data yang ada. Pelanggaran bisa terjadi begitu jalur create atau move mengisi kepemilikan.

**Protokol transaksi (didokumentasikan; belum diterapkan pada semua jalur mutasi):**

1. Tentukan item dan Space tujuan dari input. Belum mengunci.
2. Kunci Space tujuan dengan `SELECT ... FOR SHARE`. Perubahan kepemilikan Space kelak memakai `FOR UPDATE`.
3. Kunci baris item yang terlibat dengan `FOR UPDATE`, diurutkan menurut `id`. Ini konsisten dengan `items/patch.go` dan `items/reorder.go`.
4. Baca ulang kepemilikan dalam transaksi yang sama: `items.client_id/scope_state`, serta `spaces.kind/client_id/scope_state`.
5. Validasi dengan tabel kompatibilitas di atas. Jika gagal, tolak.
6. Tulis `item_locations`. Trigger `lock_item_location_edges` sudah mengambil advisory lock `5102005`, sehingga penulisan lokasi sudah diserialisasi.
7. Commit. Konflik serialization ditangani dengan transaksi serializable dan retry, seperti pola DEF-049 pada `items/lock_grants.go`.

Urutan kunci tetap: Space (`FOR SHARE`) sebelum item (`FOR UPDATE`, menurut `id`). Perubahan kepemilikan terkendali harus memakai urutan yang sama. **Belum diverifikasi terhadap semua jalur** (`custom_fields.go` juga mengunci Space dengan `FOR UPDATE`). Verifikasi konkurensi menjadi tugas B3-08, B3-09, dan B3-22.

**Yang belum selesai:** enforcement pada create (B3-03), move, copy, restore, dan location (B3-08, B3-09), migrasi kepemilikan terkendali, dan AC-09 atau AC-10. Operasional belum dipetakan. Pada titik ini client isolation **belum** diimplementasikan. Kolom `client_id` saja tidak menegakkan isolasi.

**Bukti:** `server/internal/database/ownership_schema_test.go` (OWN-DB-02 sampai OWN-DB-18, 18 kasus lewat 4 grup), lihat MVP_SCOPE §17.

## 7d. Referensi tes B2-01/02

Tes di `server/internal/database/clients_schema_test.go`: DB-01..DB-16 (lihat MVP_SCOPE §12). Tes domain: `server/internal/clients/clients_integration_test.go` (lihat §7e).

## 8. Referensi

- Rencana MVP v0.2: `docs/roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md`
- Audit akses: `docs/roadmap/PMS_PLATFORM_IMPACT_ASSESSMENT.md` (B0-03)
- Audit data: idem (B0-04), spesifikasi pemetaan (B0-05)
- Status kandidat: `docs/qa/releases/v0.14.0/CANDIDATE.md`
- Keputusan repo: `docs/architecture/DECISIONS.md`
- ADR yang ada: `docs/parity/adr/ADR-REG-registry-and-typed-relations.md`, `ADR-CIT-custom-item-types.md`, `ADR-AUT-domain-events-and-automation.md`, `ADR-IDP-identity-adapters.md`
