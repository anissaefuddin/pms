# Staging rehearsal — PROMPT #029 (kandidat 0.14.0-candidate.2628f41)

**Status: PARTIAL. Kandidat sudah dibangun dan migrasinya sudah dibuktikan pada clone. Layanan staging TIDAK diganti.**
Keputusan live swap menunggu persetujuan (lihat §5).

## 1. Identitas

| Item | Nilai |
|---|---|
| Host | `staging-host` (198.51.100.10), user `deploy-user`. Dikonfirmasi lewat `PUBLIC_URL=https://staging.example.invalid` di `.env` compose. |
| Staging publik | `https://staging.example.invalid/` (HTTP 200, `/readyz` 200) |
| Versi berjalan (sebelum) | `pms:0.13.0`, digest `sha256:d7aa2fdc…`, healthy, 0 restart |
| Skema berjalan | migrasi 361 (126 baris tercatat) |
| Postgres | `postgres:16.10-alpine` (`pms-postgres-1`) |
| Storage | lokal, volume `pms_pms_files` di `/data/files` |
| Kandidat | source HEAD `2628f41`, image `pms:candidate-2628f41` (ID `04471259e823`), `pms version` → `0.14.0-candidate.2628f41` |
| Mesin | x86_64, 4 CPU, RAM 7,8 GiB (3,7 GiB terpakai), disk 46 GiB bebas |

## 2. Backup (Fase B)

- Dibuat dengan `./backup.sh` (repositori): `20261127T052719Z` (database dump 1,1 MB, files 1,3 MB, manifest, SHA256SUMS).
- Checksum: OK untuk kedua artefak.
- **Hanya lokal di server.** `BACKUP_REMOTE`/`BACKUP_RSYNC_DEST` tidak diset. Off-host backup **BLOCKED** (OPS-02) dan dilaporkan terpisah.

## 3. Rehearsal migrasi (Fase D)

- Dump 05:27 di-restore ke database terisolasi `pms_rehearsal` (bukan database live).
- `pms migrate` kandidat dijalankan terhadap clone: 361 → 364 (`00362_seed_runs`, `00363_clients`, `00364_client_scope`). Tanpa error.
- Jumlah baris sama sebelum dan sesudah: users 34, spaces 46, items 1991, item_locations 1993, item_dependencies 50, attachments 21, comments 22.
- Dry-run `client-scope` read-only dengan mapping kosong (tidak ada mapping operasional yang disetujui): tanpa konflik; `items_to_classify` 5, `spaces_unresolved_remaining` 9, `items_unresolved_remaining` 1477.
- Clone sudah dihapus setelah rehearsal (data bisnis tidak disimpan di luar backup).

## 4. Temuan

| Temuan | Prioritas | Bukti | Rekomendasi |
|---|---|---|---|
| Setelah migrasi, seluruh 1991 item dan 22 Space non-pribadi berstatus `unresolved`. Pengguna biasa tidak melihat konten yang ada. | **P0 untuk live swap** | Query clone | Jangan ganti layanan live sebelum keputusan Opsi A/B (§5). |
| Off-host backup tidak ada | P0 | `BACKUP_REMOTE` tidak diset | OPS-02 |
| Dry-run mengklasifikasikan 5 item di Space pribadi (sudah internal, K-25) tanpa mapping | P1 (review kebijakan) | Output dry-run | Tinjau apakah item di Space internal boleh diklasifikasi otomatis; prinsip "tanpa klasifikasi otomatis" berlaku. |
| Versi dan skema staging tidak bisa dibaca lewat API tanpa sesi admin | P2 | — | Sudah terbaca langsung dari database |
| Live Pydio dan STG-01..16 belum dijalankan | — | — | Menunggu keputusan swap |

## 5. Keputusan yang diperlukan

Staging live masih `pms:0.13.0`. Mengganti ke kandidat tanpa mapping akan menyembunyikan seluruh konten yang ada dari pengguna (fail-closed). Ada dua opsi:

- **Opsi A:** clone yang tervalidasi, hanya bila mapping Customer disetujui secara tertulis.
- **Opsi B (direkomendasikan):** database sintetis terpisah untuk rehearsal (Customer, pengguna, dan project fiktif), dengan dokumentasi bahwa project historis staging tidak ikut.

Tanpa salah satu keputusan ini, live swap **NO-GO**.

## 6. Rollback

Jika swap dilakukan nanti: `docker compose stop app` (freeze), backup baru, ubah `PMS_IMAGE` ke kandidat, `docker compose up -d app`, cek `/readyz`. Jika gagal: restore backup dan image `pms:0.13.0` bersama-sama (`restore.sh <stamp> --force`). Jangan menjalankan binary lama di atas skema 364.

## 7. Yang belum dilakukan

Deploy live, smoke fungsional (Fase F), matriks keamanan STG-01..16 (Fase G), verifikasi Pydio live (Fase H; belum ada kredensial atau persetujuan ACL), dan pengukuran RPO/RTO.

## 8. Status

- Build kandidat: **VERIFIED**.
- Migrasi pada clone: **VERIFIED**.
- Deploy staging: **NOT DONE** (menunggu §5).
- Off-host backup: **BLOCKED**.
- Pydio live: **BLOCKED**.
- Onboarding: **NO-GO**. Produksi: **NOT READY**.
