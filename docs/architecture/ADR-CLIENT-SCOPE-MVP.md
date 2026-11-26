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

## 8. Referensi

- Rencana MVP v0.2: `docs/roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md`
- Audit akses: `docs/roadmap/PMS_PLATFORM_IMPACT_ASSESSMENT.md` (B0-03)
- Audit data: idem (B0-04), spesifikasi pemetaan (B0-05)
- Status kandidat: `docs/qa/releases/v0.14.0/CANDIDATE.md`
- Keputusan repo: `docs/architecture/DECISIONS.md`
- ADR yang ada: `docs/parity/adr/ADR-REG-registry-and-typed-relations.md`, `ADR-CIT-custom-item-types.md`, `ADR-AUT-domain-events-and-automation.md`, `ADR-IDP-identity-adapters.md`
