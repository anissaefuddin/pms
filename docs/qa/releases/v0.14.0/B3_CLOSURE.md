# B3 closure report — v0.14.0 (client-scope MVP)

Diperbarui oleh PROMPT #025 (penutupan keamanan lokal). Baseline: HEAD `b5531e3`.

Status keseluruhan:
- **B3 LOCAL IMPLEMENTATION: PARTIAL.** Implementasi lintas B3-01..SC ada, tetapi beberapa jalur belum dicakup tes (lihat §2).
- **B3 LOCAL SECURITY VERIFICATION: PARTIAL.** Tidak ada kebocoran lintas Customer yang diketahui pada jalur yang diuji. Ada jalur yang belum diuji (§2, §5).
- **B3 OPERATIONAL ACCEPTANCE: EXTERNAL_BLOCKED.**
- **B4 DEVELOPMENT: NO-GO (bersyarat).** Tiga blocker lokal tercantum di §8.
- **PRODUCTION READINESS: NOT READY.**

Skala status: IMPLEMENTED · VERIFIED · PARTIAL · EXTERNAL_BLOCKED · DEFERRED · NOT_APPLICABLE

## 1. Git

- HEAD awal PROMPT #025: `b5531e3`. HEAD akhir: lihat §10.
- Push: **SKIPPED** (wajib). Tidak ada PR, merge, tag, atau deploy.
- Working tree: 26 file untracked asli dipertahankan.

## 2. Status task B3

| Task | Status | Bukti | Batas |
|---|---|---|---|
| B3-01 skema ownership | VERIFIED | migrasi 00364; `internal/database` | — |
| B3-02 invariant lintas tabel | PARTIAL | CHECK baris (database tests); enforcement policy untuk dependensi, relasi, share, form, dan assignee | Pembacaan langsung di luar policy belum diaudit penuh (§5) |
| B3-03 ownership saat create | VERIFIED | CRT-01..19 | — |
| B3-04 eligibility Space | VERIFIED | SP-01..28 | — |
| B3-05 eligibility item | VERIFIED | IT, ML tests | — |
| B3-06 penugasan dan auto-assign | VERIFIED | AS-01..13; AU-02 (automation) | — |
| B3-07 revocasi membership | VERIFIED | RV-01..08 | — |
| B3-08 lokasi, move, restore | VERIFIED | LOC-01..17, CON-1..3 | — |
| B3-09 copy, blueprint, template | VERIFIED untuk blueprint dan template; NOT_APPLICABLE untuk duplikasi item | BP-01..04; SC-07; kontrol negatif | Tidak ada endpoint duplikasi item (sumber: tidak ada rute) |
| B3-10 dependensi dan relasi | VERIFIED | DR-01..06; `TestRelationTargetsShareOwner`; kontrol negatif | — |
| B3-11 sharing | VERIFIED | SH-01..08; SS-01..04; kontrol negatif | — |
| B3-12 Request Form | VERIFIED | FR-01..05; submission setelah arsip; kontrol negatif | — |
| B3-13 automation, webhook, job | PARTIAL | AU-01..04; webhook queued revocation; kontrol negatif; tinjauan konektor dan reminder | Automation terjadwal dan report subscription belum diuji end-to-end |
| B3-14 Excel import | VERIFIED | XL-01..05; kontrol negatif | — |
| B3-15 list, search, analytics | PARTIAL | matriks lintas Customer (search, overview, agenda, timeline, workload, todo, shared); tinjauan SQL stars, resource directory, baselines (sumber: setiap baris difilter `ItemsCaps`/`ItemCaps` sebelum dikembalikan) | Matriks belum mencakup portfolio dan laporan; kontrol negatif pada lapisan Go tidak gagal karena lapisan SQL menutup |
| B3-16 realtime dan notifikasi | VERIFIED untuk SSE dan inbox; PARTIAL untuk email | RT; inbox revocation; kontrol negatif | Email: visibilitas saat kirim dicakup tes yang ada; tidak ada tes revocasi email |
| B3-17 file, export, link publik | VERIFIED untuk AC-12 jalur yang didukung (lihat §4); PARTIAL untuk FILE-07 | FILE-01..06, FILE-08 (`file_access_matrix`), kontrol negatif; `PMS_PUBLIC_LINKS` dan `PMS_PYDIO_LISTING` default OFF | FILE-07 (isi export) bergantung pada tes export yang ada |
| B3-18 dry-run | PARTIAL | `internal/clientscope`: parser ketat, read-only, digest, konflik (MG-01..06), konflik form (`FORM-01`) | Konflik automation dan template belum dideteksi; quarantine per resource belum ada (§5) |
| B3-19 apply | PARTIAL | audit per perubahan dalam transaksi yang sama; refusal tercatat; idempoten; rollback saat audit gagal; ID, hierarki, share, dan placement dipertahankan | Apply operasional EXTERNAL_BLOCKED; mapping Customer adalah keputusan bisnis |
| B3-20 aktivasi enforcement | PARTIAL | tidak ada switch kompatibilitas; tidak ada UPDATE produksi yang mengubah `scope_state`/`client_id` (dicari); rute publik dan Pydio default OFF | Ownership belum dilindungi trigger immutability di database; UPDATE multi-baris belum diaudit satu per satu |
| B3-21 rollback dan recovery | PARTIAL | `docs/ops/CLIENT_SCOPE_RECOVERY_RUNBOOK.md`; rehearsal lokal backup→migrasi→restore→re-apply | RPO/RTO, off-host, `restore.sh`/`restore-test.sh` (Docker) belum dijalankan |
| B3-22 verifikasi final | PARTIAL | §3, §4, §5, dan matriks AC pada `docs/qa/releases/v0.14.0/AUTHZ_TRACEABILITY.md` | Playwright E2E tidak dijalankan (dibawa ke B6) |
| B3-SC Space creation | VERIFIED | SC-01..17 | — |

## 3. Verifikasi (PROMPT #025)

| Kelompok | Tes | Hasil |
|---|---|---|
| File dan export (AC-12) | `TestFileAccessFollowsCurrentCustomerAccess` (FILE-01..06, 08) | PASS; kontrol negatif (view check attachment dimatikan) menghasilkan FAIL |
| Pydio listing | `TestPydioListingIsOffByDefault`; `TestPydioFilesFollowItemAndFieldAccess` (fixture aktif) | PASS; kontrol negatif FAIL |
| Gantt, publik | `TestGanttSnapshotLinkEndsWithCreatorAccess`; `TestPublicLinksAreOffByDefault` | PASS; kontrol negatif FAIL |
| Migrasi | `internal/clientscope` (10 tes termasuk audit, refusal, form, preservasi, rehearsal) | PASS |
| Audit atomik | `TestAuditFailureRollsBackOwnershipChange` | PASS. Catatan: kontrol "abaikan error audit" tidak gagal karena PostgreSQL membatalkan transaksi pada statement yang gagal; commit berikutnya juga gagal. Jaminan rollback ada di database, dan tes memverifikasi itu. |
| Regresi paket terdampak | `cmd`, `config`, `clientscope`, `database`, `httpserver` | Lihat §10 |

## 4. AC-04..AC-13 (matriks rinci: `AUTHZ_TRACEABILITY.md`)

| AC | Status | Catatan |
|---|---|---|
| AC-04 membership tanpa grant | PARTIAL | Bukti tersebar (SH-02, XL-02, AS, FILE-02/04) |
| AC-05 Workspace Admin tanpa grant | PARTIAL | Analytics admin = 0; FILE-03 (admin tanpa akses = 404) |
| AC-06 assignment semua jalur | VERIFIED | AS-01..13, AU-02 |
| AC-07 Customer A tidak melihat Customer B | PARTIAL | Matriks endpoint prioritas; file dan export (FILE-02); endpoint lain disapu tanpa skenario lintas Customer |
| AC-08 search, portfolio, workload, laporan | PARTIAL | Search, overview, agenda, timeline, workload, todo, shared tercakup; portfolio dan laporan belum |
| AC-09 lokasi, move, restore, copy, dependensi, relasi | PARTIAL | Dependensi, relasi, lokasi (RV/LOC), copy (BP) tercakup |
| AC-10 share grup, revocasi, konkurensi | PARTIAL | SH-08 (grup); RV; CON; konkurensi share belum khusus |
| AC-11 blueprint, form, import, automation, job, webhook | PARTIAL | BP, FR, XL, AU, webhook tercakup; job terjadwal belum |
| AC-12 lampiran, preview, ekspor, link | VERIFIED untuk jalur yang didukung | FILE-01..06, FILE-08 (lampiran, versi, preview, export); FILE-09/10 via tes link publik default OFF dan reauth Gantt. Tidak ada endpoint thumbnail (NOT_APPLICABLE, sumber: rute tidak terdaftar). Pengunduhan yang sudah terjadi di luar sistem tidak dapat ditarik kembali. |
| AC-13 rekonsiliasi, karantina | EXTERNAL_BLOCKED | Rekonsiliasi sintetis dan audit terverifikasi; karantina per resource belum; data operasional tidak diverifikasi |

## 5. Temuan yang masih terbuka

Lokal, dapat diselesaikan (blocker B4):
1. **Quarantine per resource belum ada.** Konflik menolak apply seluruhnya; resource ambigu tetap `unresolved` (tertutup), tetapi alasan per resource tidak disimpan.
2. **Automation terjadwal dan report subscription belum diuji end-to-end** (B3-13).
3. **Jalur pembacaan langsung di luar policy** belum diaudit penuh (B3-02, B3-15): stars, resource directory, dan baselines sudah ditinjau dari sumber, tetapi tidak ada tes lintas Customer khusus.

Lokal, tidak blocker B4 (dicatat):
- Ownership belum dilindungi trigger immutability di database; migrasi tool adalah satu-satunya penulis (`grep` tidak menemukan penulis lain).
- Lampiran dan export yang sudah diunduh di luar sistem tidak dapat ditarik kembali.
- `PMS_PYDIO_LISTING` default OFF. Listing dan verifikasi ACL Pydio tetap B4.

## 6. Kapabilitas opsional yang dinonaktifkan

| Kapabilitas | Kontrol | Default | Tes |
|---|---|---|---|
| Dashboard, form, snapshot Gantt publik | `PMS_PUBLIC_LINKS` | OFF (rute tidak terdaftar) | `TestPublicLinksAreOffByDefault`, `TestPublicLinksDefaultOff` |
| Listing folder Pydio | `PMS_PYDIO_LISTING` | OFF (503 sebelum item dibaca) | `TestPydioListingIsOffByDefault` |

Untuk mengaktifkan listing Pydio di B4: set `PMS_PYDIO_LISTING=true` **hanya** setelah (1) identitas Pydio terverifikasi, (2) ACL folder terkonfirmasi pada tes live (B4-07), dan (3) keputusan K-21 untuk fallback konektor.

## 7. Blocker eksternal (EXTERNAL_BLOCKED / OPS / B6)

Dipindahkan ke tugas OPS dan B6, bukan ditandai PASS:
- Backup off-host dengan checksum (OPS-02, AC-18).
- Rehearsal restore Docker dan `restore-test.sh` (OPS-03).
- RPO dan RTO terukur (UAT: RPO ≤ 24 jam, RTO ≤ 4 jam).
- Alert monitoring terverifikasi (OPS-04, AC-19).
- Rehearsal migrasi di staging (B6-04).
- Dry-run dan apply mapping operasional, serta backfill (B3-18/19 operasional, AC-13).
- CI remote pada kandidat (AC-01).
- Playwright E2E terintegrasi (B6-02).

## 8. Keputusan B4

**B4 development: NO-GO (bersyarat).** Syarat GO:
1. Quarantine per resource dengan alasan tersimpan, atau keputusan tertulis bahwa refusal-seluruhnya cukup untuk MVP.
2. Tes lintas Customer untuk automation terjadwal dan report subscription.
3. Audit jalur baca langsung di luar policy pada `stars`, `resource_directory`, dan `baselines` dengan tes, bukan hanya tinjauan sumber.

Item 1–3 bersifat lokal. Setelah itu status B4 dapat ditinjau ulang. Blocker eksternal di §7 tidak menghalangi pengembangan B4 selama Pydio listing tetap OFF.

## 9. Kesiapan

- Implementasi B3 lokal: PARTIAL.
- Verifikasi keamanan lokal: PARTIAL.
- Operasional: EXTERNAL_BLOCKED.
- Produksi: **NOT READY**.

## 10. Git dan verifikasi akhir

Diisi setelah commit PROMPT #025: lihat `git log b5531e3..HEAD`.
