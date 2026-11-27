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
