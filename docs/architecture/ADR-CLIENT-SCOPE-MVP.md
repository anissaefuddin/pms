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

## 7j. Kepemilikan saat pembuatan item B3-03 (27 Nov 2026, Prompt #015–#017) — COMPLETE pada tingkat pembuatan pusat

Product Owner menyetujui **B3-03 saja**. Keputusan di bawah diterapkan di jalur create pusat.

- **Sumber kepemilikan:** server, dari Space tujuan (item akar) atau dari induk (anak). Tubuh permintaan tidak bisa mengirim kepemilikan: kunci `client_id` atau `scope_state` ditolak oleh `applyFields` (422, CRT-15).
- **Kompatibilitas:** setiap lokasi awal harus memberi kepemilikan yang sama. Induk harus cocok dengan Space tujuan.
- **Ditolak:** `unresolved` (`ownership_unresolved`, 422), ketidakcocokan (`ownership_mismatch`, 422), dan Customer diarsipkan (`customer_archived`, 409). Pesan tidak menyebut nama Customer atau Space.
- **Penguncian:** Customer dibaca `FOR SHARE` saat status dicek. Ini menutup celah antara pengecekan status dan INSERT. Hanya kunci foreign key (`FOR KEY SHARE`) tidak cukup, karena arsip tidak akan menunggu pengecekan status yang sudah lewat. Urutan: Customer, lalu Space, lalu induk, masing-masing `FOR SHARE`. Arsip memakai `FOR UPDATE` pada baris Customer. Tidak ada urutan kunci yang berlawanan dengan pola yang ada.
- **Penulisan:** `scope_state` dan `client_id` ditulis eksplisit di INSERT item, dalam transaksi yang sama dengan lokasi.

**Jalur yang melewati fungsi pusat:** tidak ada. Satu-satunya INSERT ke `items` ada di `items/create.go`. Pemanggil: HTTP create, `CreateTx` (form submit, space templates, connector, automation), blueprint (`create`), dan impor Excel (`createWithCustomFields`). Duplikasi atau salin belum ditemukan sebagai jalur terpisah.

**Bukti tes:** `server/internal/httpserver/item_ownership_creation_integration_test.go`. CRT-01..19 lulus (5 grup, 23 entri PASS). Mutasi: menghapus `FOR SHARE` pada Customer membuat CRT-19 gagal, lalu dikembalikan.

**Remediasi fixture (Prompt #016):** 30 fungsi tes yang gagal di Prompt #015 kini memakai ownership eksplisit. Penggantian yang dilakukan:
- `f.space` menjadi `f.internalSpace` (helper uji, Space internal dengan share admin yang sama seperti `f.space`) di 17 fungsi di 7 berkas.
- `lockFixture` (`create_lock_integration_test.go`) memakai `f.internalSpace`. Ketiga pemanggilnya termasuk dalam 30 fungsi.
- `newInternalAssignCast` dan `newInternalScopeCast` (varian explicit dari `newAssignCast` dan `newScopeCast`, tanpa share admin tambahan). Dipakai oleh 9 pemanggil yang gagal. Pemanggil lain (`pr_review`, `review_fixes`, dan `TestAutomationCannotBorrowAssignmentRights`) tetap memakai helper lama.
- `TestItemAuthorizationMatrix`: Space `scopedSpace` internal, dan item fixture diberi `scope_state='internal'` secara eksplisit.
- `TestRankingAtScale` dan `TestBrowserCreateNested` (keduanya berjalan hanya dengan `PMS_TEST_SCALE=1` dan `PMS_TEST_BROWSER=1`) memakai `f.internalSpace`. Keduanya sebelumnya tidak berjalan, dan diklasifikasikan PASS secara keliru.

Hasil: 39 dari 39 tes lama PASS dengan kedua variabel uji diaktifkan. 23 entri CRT PASS. Tes arsip Customer di paket `clients` PASS (`TestCustomerUpdateAndArchive`, `TestGrantWaitsForArchiveAndIsRefused`).

**Remediasi lanjutan (Prompt #017):** 8 fungsi regresi penugasan (`pr_review_integration_test.go`, `review_fixes_integration_test.go`, dan `assign_scope` melalui helper) memakai ownership eksplisit. Empat fungsi memakai `f.scopedSpace(..., "internal", "")` sebagai pengganti `f.spaceWithoutAdmins`, sehingga tidak ada share admin tambahan. Tiga fungsi memakai `newInternalScopeCast`, dan satu memakai `newInternalAssignCast`. `TestMovingWorkDoesNotCarryAssignmentRights` juga memerlukan `UPDATE spaces SET scope_state='internal'` pada Space yang dibuat lewat `POST /spaces`, karena Space API bersama tetap `unresolved`. Asersi 201 pada `POST /spaces` tetap ada.

**Status akhir B3-03 (verifikasi):**
- 39 dari 39 tes pembuatan dan hierarki lama PASS (dengan `PMS_TEST_SCALE=1` dan `PMS_TEST_BROWSER=1`).
- 23 entri CRT PASS (5 fungsi, 18 subtest), 0 FAIL, 0 SKIP.
- 22 tes penugasan dan Personal Space PASS (0 FAIL, 0 SKIP, 0 PANIC).
- 2 tes arsip Customer di paket `clients` PASS.
- Asersi tidak diubah. Tidak ada tes yang dilemahkan atau di-skip.

**Yang belum selesai:**
- Pembuatan Space bersama lewat API masih menghasilkan `unresolved`, sehingga item tidak bisa dibuat di Space itu sampai alur ownership-aware Space tersedia.
- Pemetaan HTTP per jalur pembuatan (form, template, connector, automation, blueprint, impor) belum diverifikasi.
- B3-02 tetap PARTIAL: invarian lintas tabel pada jalur mutasi (move, copy, restore, tambah lokasi) belum ditegakkan.
- OWN-DB-16 (`shares` dan `custom_field_grants`) tetap menjadi celah ringan, dialihkan ke B3-22.

## 7k. Eligibility Space pusat B3-04 (27 Nov 2026, Prompt #018) — STATUS: PARTIAL, lihat §7l

Product Owner menyetujui **B3-04 saja**.

- **Fungsi yang berubah:** satu predikat SQL di `spaceRows` (`server/internal/policy/engine.go`). Ketiga titik masuk memakai query ini: `VisibleSpaces` (daftar), `SpaceAccessFor`, dan `CanOnSpace`. Tidak ada evaluator kedua, dan tidak ada filter di frontend.
- **Urutan otorisasi:** predikat kelayakan dievaluasi di WHERE, sebelum share, grup, Space Admin, dan batas tipe pengguna. Setelah itu aturan resource yang ada tidak berubah.
- **Aturan:** Space internal dan Personal Space lolos tanpa membership (aturan lama tetap). Space client-owned memerlukan `client_memberships` aktif untuk Customer-nya dan pengguna aktif bertipe admin atau regular. Status Customer tidak diperiksa, sehingga Customer yang diarsipkan tetap memberi akses historis bagi anggota aktif. Ownership `unresolved` atau tidak konsisten tidak cocok dengan cabang mana pun dan disembunyikan (fail closed).
- **Membership tidak memberi izin:** tidak ada share, Space Admin, atau peran yang dibuat. Admin tanpa membership tidak mendapat akses konten.
- **Revocation:** membaca membership saat ini di setiap permintaan. Tidak ada cache server.

**Bypass yang tersisa (dicatat, tidak diperbaiki di batch ini):**
- `policy/item.go` (`itemCaps`) bergabung langsung ke `spaces` dan tidak memanggil predikat. Akses item melalui share item di Space client tetap terbuka. Ini dependensi B3-05, dan `item.go` dilarang diubah di batch ini.
- Pembacaan `FROM spaces` langsung di luar policy: `spaces/spaces.go`, `spaces/registry.go`, `spaces/governance.go`, `httpserver/create_menu.go`, `httpserver/server.go`, `automation/workspace.go`, `automation/engine.go`, `analytics/analytics.go`, `housekeeping/housekeeping.go`, dan `items/*` yang memakai `spaces` langsung. Ini perlu diverifikasi di B3-05, B3-06, dan B3-22.

**Bukti tes:** `server/internal/httpserver/space_client_eligibility_integration_test.go`, `TestClientSpaceEligibility`, dengan SP-01..SP-28 sebagai subtest (28 PASS). Kontrol negatif: jika membership revoked diterima, SP-06, SP-20, dan SP-26 gagal. File dikembalikan dan dicocokkan byte per byte.

**Regresi yang belum selesai (blocker):** predikat ini membuat Space bersama yang masih `unresolved`, serta tes yang membuat anggota tanpa membership, menjadi tidak terlihat. Pengukuran pada binary akhir:
- Tes Space terkait (78 kandidat): 53 FAIL, 1 PANIC, 24 PASS (pengukuran sebelum fixture CRT diperbarui).
- Tes pembuatan dan hierarki lama (39): 31 PASS, 8 FAIL.
- Tes penugasan dan privasi (22): 15 PASS, 7 FAIL.
- CRT-01..19: 5 fungsi dan 18 subtest PASS setelah fixture membership diperbaiki.

Penyebab kegagalan: Space dengan ownership `unresolved` dibuat lewat helper uji yang ada (`f.space`, `spaceWithoutAdmins`, API `POST /spaces`), atau aktor tidak punya membership. Perbaikannya membutuhkan keputusan membership per tes dan perubahan helper bersama. Ini di luar batas fixture yang diotorisasi, sehingga harus menjadi tugas regresi terpisah.

## 7l. Konvensi fixture Space uji dan ledger regresi B3-04 (27 Nov 2026, Prompt #019) — STATUS: SUPERSEDED oleh §7m

Product Owner mengotorisasi perubahan default helper uji. Helper berikut ada di `server/internal/httpserver/spaces_integration_test.go` dan `item_ownership_creation_integration_test.go`. Keduanya hanya berlaku untuk tes, dan tidak ada kode produksi yang berubah.

| Helper | Perilaku akhir |
|---|---|
| `f.space(name)` | Shared Space dengan ownership `internal` eksplisit dan share admin otomatis (sama seperti sebelumnya) |
| `f.spaceWithoutAdmins(name)` | Shared Space `internal` eksplisit, tanpa share admin otomatis |
| `f.internalSpace(name)` | Sama dengan `f.space` |
| `f.scopedSpace(name, scope, client)` | Space dengan ownership yang ditentukan: `internal`, `client` + Customer, atau `unresolved` |
| `f.unresolvedSpace(name)` | Space `unresolved` eksplisit untuk tes negatif (fail closed) |
| `f.addMember(admin, client, user)` / `f.revokeMember(...)` | Membership eksplisit per aktor dan Customer, lewat API admin |

**Mengapa aman:** `f.space` dan `f.spaceWithoutAdmins` hanya dipakai oleh tes. Default kepemilikan internal tidak mengubah izin: grant, share, Space Admin, dan tipe pengguna tetap sama. Tes yang secara eksplisit ingin menguji kepemilikan memakai `f.unresolvedSpace` atau `f.scopedSpace` (SP-16, CRT-07, CRT-08). Produksi tidak berubah: default `spaces.scope_state` tetap `unresolved`.

**Konsekuensi produksi yang dicatat (bukan diperbaiki di batch ini):** `spaces.Create` memanggil pemeriksaan keterlihatan setelah INSERT. Space bersama baru di produksi bersifat `unresolved`, sehingga pembuat tidak melihat Space yang baru dibuatnya. Akibatnya `POST /api/v1/spaces` mengembalikan 500 (`created space is not visible to its creator`). Space `General` dari bootstrap juga dibuat `unresolved`, sehingga tidak terlihat oleh siapa pun. Ini perlu diselesaikan oleh **B3-SC** (ownership-aware shared Space creation), yang wajib sebelum penutupan B3.

**Ledger regresi (pengukuran pada binary akhir):**

| Grup | PASS | FAIL | PANIC | Penyebab |
|---|---:|---:|---:|---|
| Tes Space terkait (78) | 70 | 7 | 1 | Semua: pembuatan Space lewat API ditolak (500, tidak terlihat oleh pembuat) |
| Pembuatan dan hierarki (39) | 37 | 2 | 0 | Sama, lewat API |
| Penugasan dan privasi (22) | 18 | 4 | 0 | Sama, lewat API (`own space`) |
| SP-01..28 | 28 | 0 | 0 | — |
| CRT-01..19 | 23 | 0 | 0 | — |
| Policy (`internal/policy`) | ok | 0 | 0 | — |

Dua belas tes unik gagal (tujuh FAIL, satu PANIC, lalu bayangan di grup lain). Semuanya memanggil `POST /spaces`, template, atau pembuatan Space lain lewat API, lalu menerima 500. Tes tersebut tidak dapat lulus tanpa perubahan produksi (B3-SC) atau penggantian pembuatan API dengan fixture, dan penggantian itu menghapus asersi 201 yang diminta. Keduanya tidak dilakukan.

**Harness:** `f.want` dan `f.call` melaporkan ke `t` induk. Subtest pada SP dan CRT sekarang memakai `base.on(t)` atau `f.on(t)`. Tes lain di luar grup ini masih memakai pola lama dan perlu diperiksa di tugas regresi terpisah.

**B3-SC** (wajib sebelum penutupan B3): pembuatan shared Space dengan ownership yang diverifikasi, termasuk Space `General` bootstrap dan jalur template. Tanpa itu, pembuatan Space lewat API tidak berfungsi di bawah B3-04.

## 7m. B3-SC: pembuatan shared Space dengan ownership eksplisit (27 Nov 2026, Prompt #020) — COMPLETE

Product Owner mengotorisasi B3-SC dan penyelesaian B3-04. Ini menutup F1: `spaces.Create` menyisipkan Space `unresolved`, lalu pemeriksaan visibilitas gagal dan menghasilkan HTTP 500.

**Kontrak API:** `POST /api/v1/spaces` dan `POST /api/v1/space-templates/{id}/spaces` mewajibkan `scope_state` (`internal` atau `client`). `client_id` wajib untuk `client` dan dilarang untuk `internal`. Tidak ada default dan tidak ada fallback internal untuk pemanggil lama. `unresolved` dan nilai lain ditolak (422). OpenAPI `SpaceCreate` dan `createSpaceFromTemplate` diperbarui, lalu artefak Go dan TypeScript diregenerasi dari root repositori.

**Validasi dan transaksi** (`spaces.ownershipForNewSpace`, dalam transaksi yang sama dengan INSERT):
- Customer dibaca `FOR SHARE` dan harus ada serta `active`. Customer diarsipkan ditolak (422).
- Membership pembuat dibaca `FOR SHARE`, dengan `status='active'`, pengguna aktif, dan tipe admin atau regular. Tidak ada membership: 403. Admin tanpa membership juga 403, sehingga Workspace Admin tidak melewati eligibility.
- Urutan kunci: Customer, lalu membership. Revoke memakai urutan yang sama (`FOR UPDATE` pada membership), sehingga keduanya saling menunggu.
- Semua validasi dilakukan sebelum INSERT. Kegagalan apa pun me-rollback seluruh transaksi, termasuk share dan baris `space_admins`. Tidak ada commit terpisah.
- Visibilitas pembuat diperiksa dalam transaksi yang sama. Pembuat mendapat share `full` dan baris `space_admins` seperti sebelumnya, dan kini hanya setelah ownership valid.

**Caller dan jalur:**
- `POST /spaces` (`httpserver/spaces.go`): meneruskan `scope_state` dan `client_id`.
- Instansiasi template (`httpserver/space_templates.go`): sama. Replay dengan `request_id` yang sama tidak diubah.
- Onboarding (`web/src/components/onboarding-wizard.tsx`): mengirim `internal` secara eksplisit untuk Space workspace pertama.
- Dialog Space (`web/src/components/space-dialog.tsx`): pilihan kepemilikan wajib (radio, tanpa default). Pilihan Customer memakai `CustomerPicker` (B2-08, lookup membership, juga untuk admin). Pindah ke Internal menghapus `client_id`. Backend tetap memvalidasi ulang.
- Bootstrap General (`bootstrap/bootstrap.go`): `scope_state='internal'` secara eksplisit. Bootstrap tetap idempoten. Space General lama tidak diubah.
- Personal Space: tetap `internal` melalui trigger 00364 (tidak berubah).
- PATCH: `scope_state` dan `client_id` ditolak sebagai field yang tidak dikenal (422). Ownership hanya berubah lewat migrasi terkendali.

**Yang belum selesai (dicatat):**
- Space yang sudah ada di produksi dengan `unresolved` (termasuk General lama) tetap tidak terlihat sampai dipetakan (B3-18/19).
- Specialized creation lain: tidak ada jalur lain yang membuat `spaces` di luar `CreateTx`, `bootstrap`, dan `server.go` (Personal). Pembacaan `spaces` langsung di luar policy tetap menjadi bypass terbuka (lihat §7k).
- Tampilan metadata kepemilikan di halaman pengaturan Space belum dibuat. Tidak wajib untuk penutupan.

**Tes:** `server/internal/httpserver/space_creation_ownership_integration_test.go` (SC-01..SC-12, SC-15..SC-17: 14 subtest PASS). SC-13 dan SC-14 ada di `bootstrap_integration_test.go` (assert General `internal`, idempotensi). Kontrol negatif: menghapus pemeriksaan membership menggagalkan SC-08, SC-09, SC-11, dan SC-17. File dikembalikan identik.

**Frontend:** `web/tests/space-create-ownership.spec.ts` (5 PASS, mocked API). Tes Space yang ada (`spaces.spec.ts`) diperbarui untuk memilih kepemilikan. Tes `onboarding.spec.ts` dan `screens.a11y.spec.ts` (Space) juga PASS.

**Regresi tes Space:** tes yang gagal karena F1 sekarang mengirim kepemilikan eksplisit dalam payload API (sesuai instruksi, tidak diganti dengan fixture). Penggantian `UPDATE spaces` sementara di `TestMovingWorkDoesNotCarryAssignmentRights` dihapus.

## 7n. Eligibility item B3-05 dan penegakan penugasan B3-06 (27 Nov 2026, Prompt #021) — COMPLETE pada tingkat policy pusat

Product Owner mengotorisasi B3-05 dan B3-06. Prinsip: kepemilikan item stabil adalah otoritas, eligibility klien dievaluasi sebelum union permission, membership bukan grant resource, dan Workspace Admin tidak melewati membership.

**B3-05 (`policy/item.go`).** Konstanta `itemGate` dipasang pada anchor `ancestry` di `itemsCaps` (dan pada jalur form). Sebuah lokasi hanya ikut menyumbang jika:
- item `client` dan lokasinya berada di Space `client` dengan `client_id` yang sama, dan aktor memiliki membership `active` pada Customer itu, dengan pengguna `active` bertipe admin atau regular; atau
- item `internal` dan lokasinya berada di Space `internal` (`client_id` NULL).
Item atau Space `unresolved`, dan kombinasi yang tidak konsisten, tidak cocok dengan cabang mana pun. Rekursi ke induk mensyaratkan kepemilikan yang sama dengan akar. Personal Space tetap milik pemiliknya. Grant share (langsung, grup, dan warisan) hanya berlaku melalui lokasi yang lolos gate, sehingga share item tidak dapat memotong eligibility. Arsip Customer tidak diperiksa, sehingga akses historis anggota aktif tetap ada.

**Jalur "Full on every path" (`policy/share_paths.go`).** `SharedOnEveryPath` memanggil `eligibleItems` untuk setiap item di jalur (termasuk induk), dan jalur yang tidak lolos gate dianggap tidak ada. Dengan demikian item atau folder yang tidak memenuhi syarat tidak dapat membuat aksi serah terima tampak aman. Pemeriksaan `CanAssignOthers`, `CanReassignMany`, dan `SharedOnEveryPath` yang ada tidak berubah selain ini. DEC-10 dan D-W3.1 tetap, dan `policy/engine.go` tidak berubah pada batch ini.

**B3-06 (`items/assignee_eligibility.go`, `items/patch.go`).** Dalam `applyFields` (dijalankan untuk create dan PATCH), assignee baru pada item `client` harus memiliki membership aktif di Customer item. Membership dibaca `FOR SHARE`, sehingga revoke (yang mengunci `FOR UPDATE`) menunggu. Assignee yang sudah ada tidak diperiksa ulang, sehingga assignee yang sudah dicabut tetap dapat dihapus (AS-11). Item internal tidak berubah. Penolakan memakai 422 `assignee_ineligible` tanpa menyebut orang atau Customer. Tidak ada membership, share, atau Space Admin yang dibuat otomatis.

**Tes:** `server/internal/httpserver/item_eligibility_integration_test.go`: IT-01..15 (15 PASS), ML-01..08 (8 PASS), AS-01..12 (12 PASS), dan AS-13 (1 PASS, pengikatan dengan revoke melalui `items.CreateTx`).

**Kontrol negatif (setiap kali dipulihkan dan diverifikasi identik):**
- Gate item dibuat `TRUE`: IT-03, IT-04, IT-05, IT-07, IT-08, IT-09, IT-13, IT-14, ML-02..ML-07 gagal.
- Kompatibilitas lokasi client dihapus: ML-02 dan ML-03 gagal.
- Syarat jalur penuh (ancestor harus punya Share) dilonggarkan: `TestAFolderGivesNoAssignmentSayOutsideItself` gagal.

**Koreksi fixture (hanya fixture uji, tidak ada asersi yang dilemahkan):** `INSERT INTO items` mentah tanpa ownership di fungsi-fungsi yang gagal kini memakai `scope_state='internal'` secara eksplisit. Helper bersama `customFieldItem`, `scheduledItem`, dan `dependencyItem` dibuat internal setelah dipastikan tidak ada pemanggil dengan Space client. Benchmark scheduling 500 tugas juga dibuat eksplisit. Dua asersi diubah secara sadar:
- CRT-08: induk `unresolved` tidak terlihat, sehingga anak mendapat 404 (sebelumnya 422). Penolakan tetap terjadi.
- CRT-09 dan CRT-10: induk client tidak terlihat di Space yang lokasinya tidak kompatibel, sehingga 404 (sebelumnya 422). Penolakan tetap terjadi.

**Penolakan yang tidak dilakukan:** tidak ada perubahan pada `items.owner_id`, migrasi, OpenAPI, frontend, atau kebijakan share.

**Sisa dan risiko (dicatat):**
- Banyak pembacaan `items` langsung di luar `policy` (sekitar 40 berkas, termasuk `spaces/item_shares.go`, `analytics`, `overview`, `views`, `automation`, `connectors`, `housekeeping`). `list`, `search`, `stream`, dan `overview` memanggil policy pusat, sehingga tercakup. Sisanya belum diverifikasi satu per satu.
- Assignee yang ditambahkan otomatis oleh aksi status (`autoAssign`) dan jalur `assignee` di luar `applyFields` belum diperiksa eligibility-nya.
- Mutasi lintas tabel lokasi (move, copy, restore, tambah lokasi) belum ditegakkan (B3-08/09). Ketidakkonsistenan lokasi dapat terjadi lewat jalur itu.
- Suite penuh tidak dijalankan. Gate item menyentuh semua pembacaan item, sehingga regresi di luar grup ini tetap mungkin.
## 7d. Referensi tes B2-01/02

Tes di `server/internal/database/clients_schema_test.go`: DB-01..DB-16 (lihat MVP_SCOPE §12). Tes domain: `server/internal/clients/clients_integration_test.go` (lihat §7e).

## 8. Referensi

- Rencana MVP v0.2: `docs/roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md`
- Audit akses: `docs/roadmap/PMS_PLATFORM_IMPACT_ASSESSMENT.md` (B0-03)
- Audit data: idem (B0-04), spesifikasi pemetaan (B0-05)
- Status kandidat: `docs/qa/releases/v0.14.0/CANDIDATE.md`
- Keputusan repo: `docs/architecture/DECISIONS.md`
- ADR yang ada: `docs/parity/adr/ADR-REG-registry-and-typed-relations.md`, `ADR-CIT-custom-item-types.md`, `ADR-AUT-domain-events-and-automation.md`, `ADR-IDP-identity-adapters.md`
