# Staging rehearsal — PROMPT #029 (Option A, kandidat 0.14.0-candidate.2628f41)

**Status: PARTIAL. Kandidat berjalan di staging (Opsi A). Smoke dengan akun dan matriks keamanan live belum dijalankan.**

## 1. Identitas

| Item | Sebelum | Sesudah |
|---|---|---|
| Host | `staging-host` (198.51.100.10), user `deploy-user` (dikonfirmasi `PUBLIC_URL=https://staging.example.invalid`) | sama |
| Image | `pms:0.13.0` | `pms:candidate-2628f41` (ID `04471259e823`); `pms version` → `0.14.0-candidate.2628f41` |
| Skema | 361 | 364 |
| Database | `pms` (asli) | `pms` (hasil migrasi dan mapping); asli disimpan sebagai `pms_pre_candidate` |
| Commit | — | `2628f41` (HEAD sumber) |
| Health | healthy | healthy, 0 restart |
| Readiness | `/readyz` 200 | `/readyz` 200 (lokal dan publik) |

## 2. Backup

- Backup beku terakhir: `20261127T053614Z` (dibuat setelah app dihentikan). Backup sebelumnya: `20261127T052719Z`, `20261127T053540Z`. Checksum diverifikasi.
- **Hanya lokal di server.** Tidak ada `BACKUP_REMOTE`. Off-host **BLOCKED**.
- Salinan lama `pms` tetap ada sebagai database `pms_pre_candidate` (tidak dihapus).

## 3. Mapping dan validasi

Mapping yang disetujui pengguna (nama Customer simulasi):

| Space (nama) | Scope | Customer |
|---|---|---|
| ITA Internal | internal | — (Tipe Internal) |
| WK EEES | client | SIM-CUST-002 (PT Sagara Energi Operasi) |
| PPN JBB | client | SIM-CUST-003 (PT Nusa Rantai Energi) |
| Bentu | client | SIM-CUST-004 (PT Rimba Energi Gas) |

Keputusan tambahan pengguna: tiga admin dengan share `full` pada Space di atas ditambahkan sebagai anggota Customer yang sesuai (membership eksplisit, dicatat di tabel `client_memberships` dengan `granted_by`). Tanpa ini dry-run ditolak (`share_recipient_not_member`, 3 kejadian).

Hasil pada clone dari backup `20261127T053540Z` (`pms_opta`):
- Dry-run: tanpa konflik. Apply: berhasil.
- Spaces: client 3, internal 25, unresolved 18.
- Items: client 138, internal 170, unresolved 1683.
- Audit: `client_scope.classify` per perubahan. Tidak ada penulisan ulang.
- Jumlah baris dipertahankan: users 34, spaces 46, items 1991, dependencies 50, attachments 21, comments 22.

Space di luar mapping (18 Space: QA dan uji) tetap `unresolved`, sehingga itemnya tidak terlihat oleh pengguna biasa (fail-closed). Ini perilaku yang diharapkan, bukan keputusan untuk mengklasifikasikannya.

## 4. Swap (dilakukan)

1. `docker compose stop app` (freeze) → backup beku → jumlah baris sama dengan snapshot klon.
2. `ALTER DATABASE pms RENAME TO pms_pre_candidate`; `ALTER DATABASE pms_opta RENAME TO pms`.
3. `.env` (cadangan `.env.bak-pre-029-20261127`): `PMS_IMAGE=pms:candidate-2628f41`; `SMTP_HOST` dikosongkan (mode log, tidak ada email keluar selama rehearsal).
4. `docker compose up -d app` → healthy, 0 restart.

## 5. Verifikasi

| Pemeriksaan | Hasil |
|---|---|
| `/readyz` lokal dan publik | 200 / 200 |
| Login page | 200 |
| `/api/v1/version` tanpa sesi | 401 (sesuai desain) |
| Error log 10 menit pertama | 0 baris ERROR |
| Skema database `pms` | 364 |
| Smoke fungsional dengan akun | **Belum** (perlu akun uji dari Anda; pembuatan fixture adalah penulisan data) |
| Matriks keamanan STG-01..16 | **Belum** (membutuhkan sesi per peran) |
| Pydio live (STG Pydio, listing OFF) | **Belum** (tidak ada persetujuan ACL) |
| Email | Dinonaktifkan sementara (`SMTP_HOST` kosong). Pengiriman live belum diuji |

Antrian job pada database sebelum swap: 434 `completed`, tidak ada pending.

## 6. Rollback

```bash
cd /home/deploy-user/pms
docker compose stop app
docker exec pms-postgres-1 sh -c 'psql -U "$POSTGRES_USER" -d postgres -c "ALTER DATABASE pms RENAME TO pms_failed_candidate" -c "ALTER DATABASE pms_pre_candidate RENAME TO pms"'
cp -p .env.bak-pre-029-20261127 .env          # mengembalikan PMS_IMAGE=pms:0.13.0 dan SMTP_HOST
docker compose up -d app
```

Migrasi maju tidak boleh dibatalkan dengan binary lama tanpa rollback database. Rollback di atas memakai database asli yang masih utuh.

## 7. Temuan

| Temuan | Prioritas |
|---|---|
| Tidak ada backup off-host | P0 (OPS-02) |
| Space di luar mapping (18) tidak terlihat oleh pengguna biasa sampai ditangani | P1 (keputusan bisnis) |
| Planner mengklasifikasikan 5 item pada Space pribadi tanpa mapping (K-25) | P1 (tinjau kebijakan) |
| Smoke dan matriks keamanan live belum dijalankan | P0 untuk penerimaan staging |
| Pydio live belum diverifikasi | P1 (AC-14) |

## 8. Status

- Deployment staging (Opsi A): **DONE** dengan batas di §5.
- Smoke fungsional dan matriks keamanan: **BELUM**.
- Off-host backup: **BLOCKED**.
- Onboarding operasional: **NO-GO** sampai smoke dan matriks dijalankan.
- Produksi: **NOT READY**.

## 9. Verifikasi akses di staging (sesi PROMPT #030, 2026-11-27)

Akun admin staging yang diberikan pemilik dipakai hanya untuk membuat akun dan fixture uji. Password tidak dicatat di dokumen ini.

**Akun uji (9, aktif, domain `.invalid`, dibuat lewat `POST /users/invitations` dan aktivasi):** `qa-admin` (admin), `qa-management`, `qa-pm-a`, `qa-pm-b`, `qa-editor-a`, `qa-member-only`, `qa-no-membership` (regular), `qa-reader-a` (viewer), `qa-limited-a` (external). Kredensial tersimpan di file 0600 di host staging, di luar repo. SMTP kosong, sehingga email undangan hanya dicatat di log (job `send_invitation_email` 9 `completed`, tidak ada pengiriman).

**Fixture:** Customer `QA-CUST-A` dan `QA-CUST-B`. Space `QA Asset Integrity` dan `QA Pipeline Inspection` (Customer A), `QA Process Safety` (Customer B), `QA Internal Development` (internal). Project 5, task 21, dependensi FS 16. Target awal 30–60 task belum tercapai.

**Batasan kelayakan:** `eligible()` hanya menerima `admin` dan `regular` (`server/internal/clients/membership.go:15`). Akun viewer dan external tidak bisa diberi kelayakan Customer, sehingga `qa-reader-a` dan `qa-limited-a` hanya diuji pada Space internal.

**Hasil pemeriksaan akses (25 dari 25 lulus, status HTTP dicatat):**

| Area | Pemeriksaan | Hasil |
|---|---|---|
| Flag publik mati | `/public/gantt`, `/public/dashboards`, `/public/forms` tanpa sesi | 404 ×3 |
| Listing Pydio mati | `/items/{id}/pydio-files` | 503 `pydio_listing_disabled` |
| Isolasi Customer A | Project A1 dan task A1 dibaca `qa-pm-a`, `qa-editor-a` (200); `qa-management`, `qa-member-only`, `qa-no-membership`, `qa-pm-b`, `qa-reader-a`, `qa-limited-a` (404) | sesuai |
| Isolasi Customer B | Project B1 dibaca `qa-pm-b` (200); `qa-pm-a`, `qa-editor-a` (404) | sesuai |
| Share internal | `qa-reader-a`, `qa-limited-a` membaca (200); `qa-member-only` (404) | sesuai |
| Share hanya-baca | `qa-reader-a` PATCH | 403 |
| Space unresolved | `/spaces/{id}` oleh `qa-admin` dan `qa-pm-a` | 404 |
| Privasi Space pribadi | item Space pribadi admin dibaca `qa-pm-a` | 404 |

**Catatan keterbatasan:**
- Pemeriksaan "admin tanpa akses otomatis" (D-17.1) **tidak membuktikan apa-apa**: `qa-admin` yang membuat semua Space ini dan memegang grant eksplisit, sehingga 200 wajar. Pembuktian yang benar membutuhkan Space yang dibuat admin lain.
- Matriks SM-01..20, STG-01..16 dan DMS-01..10 dalam PROMPT #030 **belum dijalankan dalam bentuk lengkap**. Teks kasus tidak ada di repo, sehingga pemeriksaan di atas hanya mencakup area akses. Alur bisnis (edit, komentar, ekspor, webhook, laporan), STG-14 (token lama), dan Pydio live (DMS) belum diuji.
- Listing Pydio dan link publik tetap mati. Tidak ada perubahan pada Pydio, ACL, atau akun asli.

**Temuan baru:**
- Item `r3 quick task psi` berstatus `unresolved` di dalam Space pribadi internal. Ia fail-closed dan tersembunyi, tetapi scope-nya tidak mengikuti Space. Tidak diubah.
- URL aktivasi sembilan akun ada di log container. Sudah dipakai, tetapi ini perlu masuk tinjauan keamanan.
- `qa-management` tidak memiliki kapabilitas `manage_users` dan bukan peran Workspace Admin sungguhan.
- Basis data `pms_rehearsal2` masih ada dan belum dihapus.

## 10. Hasil PROMPT #030 — penerimaan staging (2026-11-27)

### 10.1 Identitas yang berjalan

| Item | Nilai terverifikasi |
|---|---|
| Container | `pms-app-1`, healthy, restart 0, `pms:candidate-2628f41` |
| Image ID lokal | `sha256:04471259e823…` (build lokal, tanpa repo digest karena tidak ada registry; label revisi OCI tidak ada) |
| Versi aplikasi | `0.14.0-candidate.2628f41` (dari `/api/v1/version` dengan sesi terautentikasi) |
| Sumber | `2628f41` adalah leluhur dari HEAD repo `e4a9ba0` |
| Skema | 364 |
| Database aktif | `pms` (clone migrasi). Asli: `pms_pre_candidate`, tidak diubah |
| Database lain | `pms_rehearsal2` masih ada, tidak disentuh |

Preflight: `SMTP_HOST` kosong di file env dan container. Flag `PMS_PUBLIC_LINKS`, `PMS_PYDIO_LISTING`, `PMS_PYDIO_LINK_URL` tidak diset (default OFF). Checkpoint baru `20261127T055250Z` (database dan file) lolos checksum; hanya lokal karena `BACKUP_REMOTE` kosong.

### 10.2 Bukti migrasi

- Persetujuan mapping: pesan "option A" dengan tabel mapping, dan jawaban untuk menambahkan tiga admin sebagai anggota Customer target. Bukti audit: 312 event `client_scope.classify` (308 item, 4 Space), run `7c4a6a7a…`, mapping digest `ec25f812…`.
- 18 Space tidak terpetakan tetap `unresolved`: 1.682 item, 31 lampiran, 1.595 tautan hierarki utuh. Admin dan `qa-pm-a` mendapat 404 pada Space tersebut. Pencarian admin tidak mengembalikannya.
- Lima item di Space pribadi: setiap item memiliki tepat satu lokasi (Space pribadinya), berscope `internal`, tanpa `client_id`, tanpa lokasi client atau shared. Audit `client_scope.classify` ada untuk kelimanya. `owner_id` bernilai NULL, dan itu tidak diubah. Konsisten dengan aturan K-25.
- Item keenam, `r3 quick task psi`, berstatus `unresolved` di dalam Space pribadi internal. Fail-closed dan tersembunyi, tetapi scope-nya tidak mengikuti Space. Tidak diubah. Tercatat sebagai temuan.

### 10.3 Akun dan membership (sesuai tabel PROMPT #030)

| Akun | Membership | Akses terverifikasi |
|---|---|---|
| qa-admin (admin) | Tidak ada | Tidak ada akses ke Customer A/B; hanya Space internal lewat share eksplisit |
| qa-management | A dan B | Read pada Space A1, A2, B1 |
| qa-pm-a | A | Full pada A1, A2; space admin |
| qa-pm-b | B | Full pada B1; space admin |
| qa-editor-a | A | Editor pada A1 |
| qa-reader-a (viewer) | **Tidak bisa** (lihat 10.6) | Read pada Space internal saja |
| qa-limited-a (external) | **Tidak bisa** (lihat 10.6) | Limited pada Space internal saja |
| qa-member-only | A | Tidak ada grant |
| qa-no-membership | Tidak ada (dicabut) | Share read pada A1 tersisa, akses ditolak (404) |

Akun uji dibuat lewat `POST /users/invitations` dan aktivasi. Undangan tidak dikirim; tautan aktivasi hanya tercatat di log. Kredensial tersimpan di `/home/deploy-user/qa-staging-secrets/synthetic-accounts.tsv` (mode 0600), di luar repo.

Koreksi fixture yang dilakukan dalam sesi ini:
- `qa-admin` memiliki grant implisit sebagai creator (`space_admins`) pada empat Space QA. Grant itu dipindahkan ke persona anggota lewat `grantSpaceAdmin`, lalu dicabut dari `qa-admin`.
- Keanggotaan sementara `qa-admin` hanya untuk operasi administrasi, dicabut lagi setelah selesai.

### 10.4 Fixture

- Customer: `QA-CUST-A`, `QA-CUST-B`.
- Space: `QA Asset Integrity Space`, `QA Pipeline Inspection Space` (Customer A), `QA Process Safety Space` (Customer B), `QA Internal Development Space` (internal).
- Project: Asset Integrity Assessment, Pipeline Inspection, HAZOP Study, Risk Based Inspection, Internal Development.
- Task fixture: 37, dengan WBS bertingkat, milestone, status New/In Progress/Completed, task overdue, task tanpa assignee, effort_minutes, dan 16 dependensi FS.
- Artefak SM: project `QA SM Project`, folder, task bertingkat, dependensi, Space `QA SM Client Space` dan `QA SM Internal Space`, gantt snapshot yang sudah dicabut, dan dua item hasil impor Excel di Customer A.
- Lampiran sintetis: satu file uji yang diunggah dan diunduh pada STG-12.

### 10.5 Hasil

**Matriks keamanan STG-01..16 (PASS 15, FAIL 0, BLOCKED 1)**

Setiap skenario memakai sesi terautentikasi masing-masing aktor dan memeriksa isi respons, baris database, atau isi file ekspor. Hasil yang penting:
- Akses lintas Customer ditolak (404) dan badan respons tidak membocorkan judul.
- Pencarian dan Portfolio tidak mengembalikan record Customer lain. `POST /portfolio/timeline` dengan project tersembunyi mengembalikan 404 untuk seluruh permintaan.
- STG-05: admin dengan membership tetapi tanpa grant mendapat 404. Hasil awal 200 disebabkan space-admin yang dibuat setup (lihat 10.3), bukan kebocoran.
- STG-08: sesi yang sudah ada menjadi 404 setelah membership dicabut, dan kembali 200 setelah diberikan lagi.
- STG-09: pindah lokasi lintas Customer ditolak (404), lokasi tidak berubah.
- STG-10: dependensi lintas Customer ditolak, 0 baris.
- STG-11: penugasan non-anggota ditolak (422), 0 baris.
- STG-12: unduhan lampiran 200 dengan isi cocok untuk pemilik; 404 untuk `qa-pm-b` dan `qa-no-membership`.
- STG-13: workbook ekspor `qa-pm-a` berisi baris Customer A dan tidak memuat konten B. Ekspor Space A oleh `qa-pm-b` ditolak (404).
- STG-14: **BLOCKED**. Tidak ada token tautan publik yang pernah diterbitkan, dan tautan publik tidak boleh diaktifkan untuk membuatnya. Endpoint publik yang dinonaktifkan mengembalikan 404.
- STG-15: listing Pydio mengembalikan 503 `pydio_listing_disabled` untuk `qa-pm-a` dan `qa-management`.
- STG-16: Space unresolved mengembalikan 404 untuk admin, `qa-pm-a`, dan `qa-management`.

Catatan: STG-09 dan SM-16 awalnya salah menggunakan `If-Match: "1"`. Keduanya diulang dengan ETag yang benar sebelum dicatat.

**Smoke SM-01..20 (PASS 19, PARTIAL 1)**

Pemeriksaan dilakukan lewat API, dengan membaca kembali baris database atau isi respons. **Antarmuka browser belum dijalankan**, sehingga hasil ini belum membuktikan UI.
- SM-01..11, SM-14..16, SM-20: PASS.
- SM-12 (gantt): snapshot dibuat dan dicabut. Render tidak diperiksa di browser.
- SM-13 (kalender): **PARTIAL**. Tidak ada route kalender khusus di API; `/me/todo` mengembalikan 200.
- SM-17 (impor Excel): pratinjau bersih, commit mengembalikan 201, dua item tersimpan di Customer A.
- SM-18 (ekspor) dan SM-19 (lampiran): PASS, bukti di STG-12 dan STG-13.

**Pydio DMS-01..10 (PASS 2, BLOCKED 8)**
- DMS-01: HTTPS 200, verifikasi TLS berhasil. Discovery Cells dapat diakses. Versi Cells tidak tampil di discovery, jadi versi dan edisi belum terkonfirmasi.
- DMS-09: listing OFF dari sisi PMS (503). Tidak ada permintaan listing ke Pydio.
- DMS-02..08 dan DMS-10: **BLOCKED**. Prasyarat yang hilang:
  1. Akun Pydio uji yang disetujui, satu per Customer, dengan token baca-saja.
  2. URL folder uji yang disetujui. Tidak ada yang diuji terhadap dokumen rahasia.
  3. Persetujuan pemilik untuk mengisi `PMS_PYDIO_LINK_URL` dan `PMS_PYDIO_LINK_ROOTS` di staging, dengan restart container.

Tidak ada perubahan pada Pydio, ACL, akun, atau dokumen.

**Migrasi dan temuan (ringkasan di 10.6)**

### 10.6 Temuan

| ID | Prioritas | Temuan | Bukti | Rekomendasi |
|---|---|---|---|---|
| LOCK-01 | P1 | `PUT /items/{id}/locations` mengabaikan `If-Match`. Tanpa header 200, versi usang 200. `PATCH` sudah benar (409/428). Ini bukan kebocoran akses, tetapi pembaruan lokasi bisa menimpa perubahan orang lain | Probe 10.5 | Terapkan pemeriksaan versi yang sama seperti PATCH, lalu regresi. Belum diperbaiki dalam sesi ini |
| GRANT-01 | P1 (keputusan) | Membuat Space memberi creator baris `space_admins` dengan akses `full` (untuk tipe `admin`/`regular`). Workspace Admin yang membuat Space tetap mendapat akses konten. Melanggar makna "tanpa grant otomatis" dalam PROMPT | `space_admins`, `policy/item.go` | Putuskan apakah pembuatan Space oleh Workspace Admin boleh memberi akses konten |
| POLICY-01 | P1 (keputusan) | Tipe `viewer` dan `external` tidak bisa menerima kelayakan Customer (`clients/membership.go:15`). Skenario reader dan limited pada Customer A tidak dapat dijalankan sesuai tabel PROMPT | Percobaan 422 | Putuskan apakah persona baca/terbatas harus berupa tipe lain untuk Customer |
| DATA-01 | P1 | Item `r3 quick task psi` `unresolved` di dalam Space pribadi internal (fail-closed) | 10.2 | Tinjau sebelum B5. Tidak diubah |
| OWN-01 | P1 (keputusan) | `owner_id` NULL pada 1.964 dari 1.991 item. Semantik owner Space pribadi belum jelas | Query | Putuskan semantik `owner_id` untuk item pribadi |
| LOG-01 | P1 (keamanan) | Log container memuat URL aktivasi dan URL notifikasi satu kali (43 baris "logged instead of sent"). Log harus dianggap sensitif | `mail.go:98-101` | Redaksi URL di log, atau tutup log pada rehearsal |
| CAL-01 | P2 | Tidak ada route kalender khusus. SM-13 hanya partial | SM-13 | Tinjau bersama pemilik fitur |
| JOB-01 | P2 | Jumlah job `completed` 430, sebelumnya 434 (dugaan: retensi). Belum diverifikasi | Query | Verifikasi kebijakan retensi river |
| UI-01 | P1 | Browser UI tidak dijalankan. Hasil SM bersifat API-level | — | Jalankan pemeriksaan UI sebelum acceptance akhir |

Tidak ditemukan P0 security (kebocoran, bypass otorisasi, atau pelanggaran kepemilikan) setelah koreksi fixture. STG-05 awal gagal karena fixture, bukan karena produk.

### 10.7 Keputusan

- Delapan belas Space unresolved: tetap unresolved dan tersembunyi, tidak ada klasifikasi otomatis.
- Lima item Space pribadi: konsisten, tidak diubah. Satu item unresolved lain tercatat sebagai temuan DATA-01.
- SMTP: **OFF**. Empat puluh tiga job email tercatat di log dan tidak dikirim, tidak ada job pending, tidak ada error. Tidak ada sink SMTP terisolasi yang tersedia. Sebelum SMTP diaktifkan kembali: sink terisolasi, kebijakan untuk antrean yang belum terkirim, dan redaksi log (LOG-01).
- Public link: OFF. Pydio listing: OFF. Pydio storage: tidak didukung (ditolak saat start). Connector: tidak diuji dalam sesi ini.
- Data bisnis yang sudah ada: tidak diubah. Hanya fixture `QA` dan artefak SM yang ditulis.

### 10.8 Git dan deployment

- HEAD repo: `e4a9ba0` (dokumen). Kandidat yang berjalan: `2628f41`, leluhur dari HEAD.
- Commit lokal sesi ini: `e4a9ba0` dan commit dokumen ini. Tidak ada perubahan kode dan tidak ada redeploy.
- Push: **SKIPPED**. Tidak ada PR, merge, tag, perubahan visibilitas repo, atau deployment produksi.

### 10.9 Acceptance

| Area | Status |
|---|---|
| Deployment teknis staging | GO |
| Rehearsal fungsional staging | CONDITIONAL GO (SM API-level PASS; UI belum; SM-13 partial) |
| Keamanan staging | CONDITIONAL GO (STG 15 PASS, 1 BLOCKED; tidak ada P0; LOCK-01, GRANT-01, POLICY-01 perlu keputusan) |
| Pydio live | PARTIAL (DMS-01, DMS-09 PASS; lainnya BLOCKED) |
| Kesiapan pengembangan B5 | NO-GO sampai keputusan GRANT-01, POLICY-01, OWN-01 |
| Onboarding operasional | NO-GO (tidak ada backup off-host; LOG-01) |
| Produksi | NOT READY |

## 11. Koreksi konsolidasi PROMPT #031 (2026-11-27)

Kandidat yang berjalan: `pms:candidate-888e4c57e51c7de656634b5bebab2805a0f9aa05` (image ID `sha256:93a30a85c1f4…`), revisi terverifikasi `0.14.0-candidate.888e4c57…` dari `/api/v1/version`. Schema tetap 364 (tidak ada migrasi baru). Health `healthy`, 0 restart, `/api/v1/readyz` 200. Kandidat sebelumnya `pms:candidate-2628f41` (image ID `sha256:04471259e823…`). `.env` sebelum perubahan disimpan sebagai `.env.bak-pre-031-*`. Checkpoint sebelum deploy: `20261127T060906Z` (database dan file, checksum OK).

Commit lokal: `888e4c5` (koreksi kode dan tes). Commit dokumen ini menyusul. Push: **SKIPPED**.

### 11.1 Hasil per temuan

| Temuan | Status | Bukti |
|---|---|---|
| LOCK-01 | **FIXED dan terverifikasi live** | `PUT /items/{id}/locations` sekarang mewajibkan `If-Match` dan membandingkan versi di bawah kunci baris. Keberhasilan menaikkan versi. Live: versi usang 409, tanpa header 428, versi terkini 200, versi naik 4→5. Tes: `TestLocationEdit*` (4 tes, termasuk dua penulis bersamaan: tepat satu 200 dan satu 409) |
| LOG-01 | **FIXED** | Jalur SMTP-mati hanya mencatat template dan subjek, tanpa tautan atau alamat penerima (`logUnsentEmail`). Tautan bootstrap dicetak ke keluaran perintah, bukan log aplikasi. Tes: `TestUnsentEmailLogHasNoLinkOrAddress` |
| GRANT-01 | **TERVERIFIKASI** | Creator memegang grant eksplisit (`space_admins`). Mencabut membership menolak Space dan item (404). Admin tanpa membership tidak bisa membuat Space Customer dan tidak punya akses ke Space orang lain (`TestCreatorGrantNeedsMembershipAndEndsWithIt`, `TestWorkspaceAdminWithoutMembership…`) |
| POLICY-01 | **TERVERIFIKASI** | Viewer dan external ditolak sebagai anggota Customer (`TestViewerAndExternalCannotBecomeCustomerMembers`). Read Only dan Limited tampil tetapi tidak bisa mengubah (`TestReadOnlyAndLimitedGrants…`) |
| OWN-01 | **Koreksi label** | Label ini saya buat sendiri di laporan sebelumnya dan tidak ada di ledger mana pun. Disposisi berdasarkan data: `owner_id` hanya dipakai untuk project (kontrak API: "Hanya project"). Data: task 1.880 tanpa owner, folder 102 tanpa owner (sesuai desain), project 57 dengan 33 owner dan 24 tanpa owner. **Tidak ada perubahan data.** Yang tersisa: 24 project tanpa owner, data-quality P2, untuk ditinjau pemilik |
| DATA-01 | Tetap, fail-closed, tidak diubah | Item `r3 quick task psi` |
| 18 Space unresolved | Tetap, tidak diubah | Tidak ada klasifikasi |
| 5 item Space pribadi | Diterima (keputusan PO) | Satu lokasi, scope internal, audit ada |

### 11.2 Koreksi fixture

- `qa-reader-a` dan `qa-limited-a` diubah dari `viewer`/`external` menjadi `regular` (hanya akun sintetis). Keduanya diberi membership Customer A dan grant eksplisit: Read Only pada A1 dan Limited pada A2. Live: GET 200, PATCH 403.
- Tidak ada perubahan pada akun bisnis.

### 11.3 Tes yang dijalankan

- Regresi paket terdampak (`items`, `automation`, `mail`, `httpserver`, `cmd/pms`): semua **ok** (exit 0) setelah koreksi.
- `go vet`, `gofmt`: bersih untuk paket yang diubah. `npm run typecheck`: lulus.
- Tes sapuan endpoint (`TestEveryProtectedEndpoint…`) menemukan helper uji yang salah. Helper sekarang mengambil versi terkini untuk `PUT …/locations` tanpa `If-Match` (klien web juga mengirim versi ini).
- STG-01..16 ulang di build baru: **15 PASS, 0 FAIL, 1 BLOCKED** (STG-14).

### 11.4 Yang belum dijalankan dan alasannya

- **UAT browser:** built-in browser sudah berisi sesi admin nyata milik pemilik (bukan akun sintetis). Saya tidak keluar dari sesi itu dan tidak login sebagai persona, karena itu berarti mengubah sesi pemilik. Perlu keputusan: pemilik login sendiri ke persona di browser, atau memberi izin untuk login terpisah. Sampai itu, SM-13 dan UI lain tetap belum terverifikasi.
- **Pydio:** DMS-02..08, DMS-10 tetap BLOCKED (akun uji dan folder yang disetujui belum ada; `PMS_PYDIO_LINK_*` belum diisi).
- **STG-14:** tetap BLOCKED (tidak ada token lama; tautan publik tidak boleh diaktifkan).

### 11.5 Rotasi dan invalidasi

- Password admin yang dikirim di chat: **perlu diganti** lewat mekanisme aman, belum saya lakukan.
- Tautan aktivasi sembilan akun sintetis di log container lama: sudah dipakai, tetapi log lama harus diperlakukan sensitif. Redaksi berlaku mulai build ini.
- Tautan bootstrap yang mungkin pernah tercetak di log container sebelum build ini: periksa dan invalidasi undangan yang belum terpakai.

### 11.6 Backlog B5 (prioritas)

1. Pertahankan kontrak versi untuk endpoint mutasi lain yang belum diperiksa.
2. Verifikasi UI browser dengan sesi persona terpisah (lihat 11.4).
3. Pydio live setelah akun dan folder disetujui.
4. Tinjau 24 project tanpa owner.
5. Backup off-host (OPS-02) sebelum onboarding.
