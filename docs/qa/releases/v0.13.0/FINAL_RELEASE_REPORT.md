# Laporan rilis final

Candidate `c3782a7` (`0.12.5-rc3`) · schema 361 · PR anissaefuddin/pms#28 · run pr28-20261126

## Keputusan
| Gate | Keputusan | Catatan |
|---|---|---|
| Merge PR #28 | **MERGED** 26 Nov 08.48 UTC | merge commit `72f9902`; tree identik dengan `c3782a7` |
| Staging | **belum dinilai** | butuh akses server dan izin deploy |
| Production | **belum CONDITIONAL_GO** | menunggu staging smoke test, backup, dan persetujuan |

## Candidate
| | |
|---|---|
| Commit | `c3782a79bd013a9b04ab1afc447c16f698a263ef` |
| CI | test, authorization-matrix, parity-e2e lulus (run 45779890214) |
| Artifact amd64 (CI) | `pms-image-amd64-01c2dcc…`, tar.gz SHA-256 `ea45947b405934b235bd1f9f91ea6971395d525e00d402eaa00fdaa8015f5c8a`, image `sha256:06e3b8c7…`, kedaluwarsa 10 Des 2026. Dibangun dari merge ref yang tree-nya identik dengan HEAD; start, migrasi 361 dan health lulus di runner amd64 asli |
| Image lokal | arm64 `sha256:8be5ce7e…`, amd64 emulasi `sha256:0074aac6…` |

## Hasil aktual RC3
| | SUCCESS | FAILED | BLOCKED | BACKLOG |
|---|---|---|---|---|
| UAT (118) | 105 | 0 | 6 | 7 |
| E2E (35) | 34 | 0 | 1 | 0 |

Harness: 150 hasil = 146 SUCCESS, 4 BLOCKED. Satu rerun: test tambahan DEF-047 gagal pada run penuh (145/4/1) karena harness: admin yang mengunci Space tidak punya hak edit di Space itu, jadi Space tidak pernah terkunci. Prakondisi ditambahkan dan hanya test itu diulang pada image dan environment yang sama (7/7). Hasil pertama disimpan (`rc/evidence/DEF-047-first-run`).

Gate lokal: Go 583 test dan 282 subtest, 0 gagal, 2 skip; unit web 118; Playwright 697 lulus, 46 skip (34 dijalankan lewat Go browser test, 10 screenshot saja, 2 Gantt performa dijalankan terpisah). Migrasi 354/358/360 → 361, rollback restore, dan skenario operasional lulus lokal.

## Defect
52 entri: 51 VERIFIED_FIXED, 1 DEFERRED_APPROVED (DEF-043). Detail: `UPDATED_DEFECT_REGISTER.csv`.

## Penting
**DEF-045 ada di versi yang terpasang** (direproduksi pada v0.12.5). Upgrade didahulukan; audit read-only di `52_…md`. Tidak ada bukti penyalahgunaan; produksi dan staging belum diaudit.

## Persetujuan yang diperlukan
1. ~~Merge PR #28~~ (selesai).
2. Tag rilis (usulan `v0.13.0`; masih 0.x sebelum MVP 1) agar CI mendorong image amd64 ke GHCR.
3. Akses server staging, backup, lalu deploy.
4. Deploy production (terpisah).
