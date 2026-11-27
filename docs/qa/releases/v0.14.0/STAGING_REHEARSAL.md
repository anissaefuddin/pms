# Staging rehearsal — PROMPT #029 (v0.14.0 kandidat)

**Status: BLOCKED.** Deployment ke staging belum dilakukan. Tidak ada perubahan pada staging.

## 1. Baseline yang dapat diverifikasi (read-only)

| Item | Hasil |
|---|---|
| HEAD sumber | `2628f41` (branch `release/pms-platform-ready-mvp`) |
| Staging publik | `https://staging.example.invalid/` menjawab HTTP 200; `/readyz` dan `/api/v1/readyz` menjawab HTTP 200 (dilihat dari luar, lewat Cloudflare) |
| Versi, skema, digest yang berjalan | **Tidak terbaca.** `GET /api/v1/version` memerlukan sesi admin (sudah tercatat di `CANDIDATE.md` §2) |

## 2. Identitas host staging — tidak terverifikasi

- Alias SSH yang ada di `~/.ssh/config` tidak memuat nama staging. Alias tidak diberi label lingkungan.
- `app-server`: terjangkau. Hostname `app`. 36 kontainer; **tidak ada kontainer PMS atau LENS**. Satu kontainer Pydio (`app-cells-1`, image `pydio/cells:v5-dev-local`) berstatus *Exited* sejak 5 minggu lalu. Host ini tidak dapat dibuktikan sebagai host staging.
- `lan-host` (198.51.100.20): tidak terjangkau (no route to host).
- `db-server`, `monitoring-server`, `utility-server`: pemeriksaan read-only dihentikan oleh pembatasan izin tool (ditolak oleh classifier otomatis). Tidak ada pemeriksaan lanjutan yang dilakukan, dan pembatasan itu tidak diakali.

Karena host staging tidak teridentifikasi, tidak ada operasi yang bersifat mengubah state (backup, build transfer, migrasi, deploy) yang dijalankan. Aturan PROMPT #029 §3 mewajibkan itu.

## 3. Yang belum dikerjakan

- Phase B (backup): tidak dijalankan. Tidak ada host yang bisa dipastikan.
- Phase C (build kandidat): tidak dijalankan. Build lokal `go build ./...`, `go vet`, dan build frontend sudah lulus di pass sebelumnya, tetapi image kontainer belum dibuat.
- Phase D (migrasi di clone): tidak dijalankan.
- Phase E–I (deploy, smoke, matriks keamanan STG-01..16, Pydio live, operasional): tidak dijalankan.

## 4. Yang dibutuhkan untuk melanjutkan

1. Konfirmasi alias SSH atau nama host yang menjalankan `staging.example.invalid`, dan nama kontainer PMS-nya.
2. Akses baca ke versi dan skema staging (sesi admin atau akses `docker exec` ke kontainer PMS), agar baseline dapat diisi.
3. Lokasi backup staging yang ada dan tujuan restore terisolasi.
4. Keputusan Opsi A atau B (lihat PROMPT #029 §7). Opsi B (database sintetis terpisah) direkomendasikan.
5. Otorisasi tertulis untuk deploy ke staging, dan untuk akses Pydio staging (`dms.example.invalid`) yang read-only.

## 5. Rollback

Belum berlaku: tidak ada perubahan pada staging. Jika deploy dilakukan nanti, prosedur mengikuti `docs/ops/CLIENT_SCOPE_RECOVERY_RUNBOOK.md`: freeze tulis, backup database dan file lebih dulu, lalu restore bersama image lama. Migrasi tidak boleh dibatalkan dengan menjalankan binary lama.

## 6. Status

- Deployment staging: **BLOCKED** (identitas host).
- Rehearsal fungsional staging: **BLOCKED**.
- Pydio live: **BLOCKED** (tidak ada akses ke instance terkonfirmasi).
- Onboarding operasional: **NO-GO**.
- Produksi: **NOT READY**.
