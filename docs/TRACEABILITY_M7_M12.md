# Traceability M7–M12 — Requirement → Story → Penerimaan → Tes

| | |
|---|---|
| **Versi** | 0.1 (13 Nov 2026), menyertai PRD v0.4 |
| **Aturan** | Setiap requirement in-scope rilis 1.0 punya minimal satu story, satu kriteria penerimaan, dan satu tes. Kolom **Bukti** diisi saat story DONE (tautan TESTING/UAT). Requirement tanpa baris di sini tidak boleh dianggap selesai |
| **Detail** | Kriteria penerimaan lengkap ditulis di [BACKLOG.md](BACKLOG.md) saat milestone dimulai; tes E2E-PAR merujuk ke baseline §8.2 |

## Stabilisasi (M7)

| Req | Story | Kriteria penerimaan ringkas | Tes | Bukti |
|---|---|---|---|---|
| STAB-01 | S6.3.5 | 93 requirement P0/P1 punya hasil UAT dengan bukti | UAT.md | |
| STAB-02 | S7.1.1 | Nol 5xx pada skenario reproduksi | Tes regresi reproduksi | |
| STAB-03 | S1.1.4 | Required check aktif dan lulus di PR | Run CI | |
| STAB-04 | S7.1.3 | Setiap endpoint punya tes 401/403/404/IDOR | Skrip inventaris + suite Go | |
| STAB-05 | S7.1.2 | Status/komentar ≤ 3 detik, termasuk reconnect | Harness dua pengguna | |
| STAB-06 | S6.3.5 | Lima flow Playwright hijau di CI | Run CI | |
| STAB-07, 08 | S7.1.4 | Audit UI ulang tanpa P1 terbuka | Audit + axe | |
| PAR-CAT-01 | S7.2.1 | Katalog dibekukan dan disetujui | Review | |
| REG-00 | S7.3.2 | ADR registry dan relasi disetujui sebelum S8.1.6 dan S9.1.2 | Review | |

## M8 — Konfigurasi kerja

| Req | Story | Kriteria penerimaan ringkas | Tes | Bukti |
|---|---|---|---|---|
| CUST-01 | S8.1.1, S8.1.5 | Admin membuat tipe tanpa migrasi; tipe tampil di menu "+", view, search, ekspor; non-admin 403 | E2E-PAR-01, E2E-PAR-02 | TESTING M8 (sebagian); [G1a](parity/gate-reports/G1A_CONFIG.md) |
| CUST-02 | S8.1.2 | Default terisi dari keempat jalur create; inheritance sesuai aturan | Integrasi 4 jalur | TESTING M8 (sebagian) |
| CUST-03 | S8.1.3 | Parent/child terlarang ditolak di create, move, lokasi | Tabel kasus | TESTING M8 |
| CUST-04 | S8.1.4 | Panel item mengikuti layout tipe | Playwright + axe | TESTING M8 |
| CUST-05 | S8.1.6 | Relasi ke target tanpa akses ditolak dan namanya tidak bocor | Tes IDOR | TESTING M8 |
| SPC-14 | S8.1.5 | Menu blueprint dapat diatur tipe, nama, ikon | Playwright | |
| TASK-11B | S8.2.1 | Formula benar untuk ≥ 20 kasus; siklus ditolak | Tabel kasus + perf 500 item | TESTING M8 (sebagian) |
| WF-04 | S8.2.2 | Transisi tidak valid ditolak sama di UI/REST/impor tanpa perubahan data | E2E-PAR-03 | TESTING M8 |
| WF-05 | S8.2.3 | Aturan lintas record ditegakkan | Tabel kasus | TESTING M8 |
| WF-03B | S8.2.4 | Masuk status memicu approval; hasil memindah ke status tujuan | Integrasi + Playwright | TESTING M8 |
| FORM-07 | S8.3.1 | Hanya cabang yang sesuai ditampilkan dan disimpan | E2E-PAR-05 | TESTING M8 (sebagian) |
| FORM-09 | S8.3.2 | Item terbentuk dengan tipe dan field sesuai cabang | Integrasi | TESTING M8 (sebagian) |
| FORM-04 | S8.3.3 | Email konfirmasi sesuai konfigurasi, HTML di-escape | Integrasi SMTP tiruan | TESTING M8 |
| SET-06 | S8.3.4 | Light/Dark/System dan tema warna lulus kontras AA | Axe | TESTING M8 |

## M9 — Otomasi dan approval

| Req | Story | Kriteria penerimaan ringkas | Tes | Bukti |
|---|---|---|---|---|
| AUT-01 | S9.1.1 | Event tidak hilang saat crash; hasil efektif tepat sekali | Tes crash recovery | |
| AUT-02 | S9.1.2 | Setiap pemicu dan aksi bekerja; versi aturan tercatat | Unit tabel + integrasi | |
| AUT-03 | S9.1.3 | Loop berhenti; aksi tanpa izin ditolak; paralel aman | Tes loop, izin, konkurensi | |
| AUT-04 | S9.1.4 | Log lengkap; retry, dead job, replay idempoten | Integrasi | |
| AUT-05 | S9.1.5 | Pengguna membuat, menguji, menonaktifkan aturan dari UI | E2E-PAR-04 | |
| APR-01 | S9.2.1 | Any/All menghasilkan keputusan sesuai kebijakan | E2E-PAR-06 | [TESTING](TESTING.md#s921-dan-s922--approval-m9) |
| APR-02 | S9.2.2 | Reassign tercatat; pengingat dan eskalasi terkirim | Integrasi | [TESTING](TESTING.md#s921-dan-s922--approval-m9) |
| APR-03 | S9.2.3 | Riwayat tidak dapat diubah; keputusan menunjuk versi | Integrasi | |

## M10 — Resource, jadwal, biaya

| Req | Story | Kriteria penerimaan ringkas | Tes | Bukti |
|---|---|---|---|---|
| RSC-01 | S10.1.1 | Kapasitas mengikuti jadwal pribadi, cuti, libur | Integrasi | [TESTING](TESTING.md#m10--kapasitas-workload-dan-booking) |
| RSC-02 | S10.1.2, S10.1.3 | Beban benar untuk multi-assignee dan multi-lokasi; over-allocation tampil | E2E-PAR-07 + perf | [TESTING](TESTING.md#m10--kapasitas-workload-dan-booking) |
| RSC-03 | S10.1.4 | Realokasi memperbarui beban, audit, SSE | Integrasi | [TESTING](TESTING.md#m10--kapasitas-workload-dan-booking) |
| RSC-04 | S10.2.1 | Workload dapat difilter per role/skill | Integrasi | [TESTING](TESTING.md#m10--kapasitas-workload-dan-booking) |
| RSC-05 | S10.2.2 | Booking masuk workload; konflik terlihat | Integrasi | [TESTING](TESTING.md#m10--kapasitas-workload-dan-booking) |
| RSC-06 | S10.2.3 | Utilization cocok dengan data mentah | Tes kecocokan angka | |
| VIEW-05C | S10.3.1 | Baseline tidak berubah saat jadwal berubah; variance benar | E2E-PAR-08 | |
| SPC-16 | S10.3.2 | Snapshot publik read-only; nama assignee mengikuti opsi | Tes akses token | |
| SCH-01 | S10.3.3 | Dependensi lintas project menghormati izin dan siklus | Tes siklus + izin | |
| TASK-14 | S10.4.1, S10.4.2 | Perubahan tarif tidak mengubah histori; data finansial tidak muncul di publik | Tes rumus + masking | |

## M11 — File, analitik, integrasi

| Req | Story | Kriteria penerimaan ringkas | Tes | Bukti |
|---|---|---|---|---|
| VIEW-10 | S11.1.1 | View Files dan Chart hanya menampilkan data berizin | Izin + Playwright | `TestProjectFiles`, `project-files.spec.ts` |
| FIL-01 | S11.1.2 | Versi immutable; akses per versi dicek | Tes izin versi | `TestAttachmentVersionsAreImmutableAndPolicyChecked` |
| FIL-02 | S11.1.3 | Anotasi tetap di versinya | Integrasi | `TestProofingAnnotationsStayOnTheirVersion` |
| FIL-03 | S11.1.4 | Dua versi tampil berdampingan | Playwright | `web/tests/file-compare.spec.ts` |
| FIL-04 | S11.1.5 | Approval v1 tidak pindah ke v2 | E2E-PAR-09 | `TestFileApprovalPinnedToVersionWithExternalReviewer` |
| TASK-08B | S11.1.6 | Adapter eksternal tetap menegakkan izin dan pemindaian | Integrasi | `TestS3AdapterAgainstFakeServer`, `TestPydioAdapterAgainstFakeServer` |
| DASH-03B | S11.2.1 | Area dan pivot cocok dengan data mentah | Tes kecocokan angka | `TestAnalyticsWidgetsAreaPivotBurndownCompletion` |
| DASH-08 | S11.2.2 | Widget pustaka benar dan dapat ditambah ke papan | Tes angka + Playwright | `TestAnalyticsWidgetsAreaPivotBurndownCompletion` |
| DASH-04, 05 | S11.2.3 | Deskripsi dan layout tersimpan | Playwright | `TestDashboardLayoutAndWidgetDescription` |
| DASH-07 | S11.2.4 | Link kedaluwarsa dan password ditegakkan; finansial tersembunyi | Tes akses | `TestPublicDashboardLinkExpiryPasswordAndFinancialMasking` |
| RPT-04 | S11.2.5 | Report terkirim pada hari dan jam terpilih | Integrasi waktu | `TestReportScheduleDailyAndChosenTimeZone` |
| API-01, SET-08 | S11.3.1 | Key terbatas scope; rotasi dan cabut efektif | Integrasi | `TestAPIKeysScopeRotateRevokeAndOwnerLimits` |
| API-02 | S11.3.2 | Webhook bertanda tangan, idempoten, retry | E2E-PAR-11 | `TestWebhook*` (E2E-PAR-11) |
| API-03 | S11.3.3 | Token OAuth dicabut efektif | Integrasi | `TestOAuthCodePKCEAndRevocation` |
| INT-01 | S11.3.4 | Konektor dapat dipasang, diatur, dinonaktifkan | Integrasi | `TestLifecycleAndSecrets`, `TestConnectorRoutes` |
| INT-02 | S11.3.5 | Konflik tidak menimpa diam-diam | Tes konflik | `TestSyncBothWaysNoSilentOverwriteAndIdempotent` |
| INT-03 | S11.3.6 | GitHub dua arah dan Google Calendar satu arah berjalan | Uji akun uji | Server palsu lulus; akun sungguhan BELUM dijalankan |
| NAV-08, 09 | S11.3.7 | Bookmark dan feedback tersimpan | Playwright | `TestBookmarksAndFeedback`, `bookmarks-feedback.spec.ts` |

## M12 — Enterprise dan penerimaan

| Req | Story | Kriteria penerimaan ringkas | Tes | Bukti |
|---|---|---|---|---|
| ENT-01 | S12.1.1, S12.1.2 | Login OIDC dan SAML; assertion palsu ditolak; login password tetap bekerja | Tes keamanan SSO | |
| ENT-02 | S12.1.3 | Deprovision mencabut sesi dan seat | E2E-PAR-12 | |
| ENT-03 | S12.1.4 | Role kustom tidak melebihi hak yang diberikan | Matriks least privilege | |
| ENT-04 | S12.1.5 | Event keamanan tercatat, hanya admin | Integrasi | |
| ENT-05 | S12.1.6 | Locked space dan IP allowlist ditegakkan | Integrasi | |
| AUTH-08 | S12.1.7 | Login OTP berhasil dan sekali pakai | Integrasi | |
| SET-09 | S12.1.7 | Kepemilikan pindah tanpa kehilangan akses admin | Integrasi | |
| PAR-UAT | S12.2.1, S12.2.2 | Semua WFP in-scope `ACCEPTED`/`N/A_APPROVED` | E2E-PAR-01..12 + UAT | |

## Di luar rilis 1.0

E2E-PAR-13..16 (registry, AI, Whiteboard) tidak dijalankan untuk 1.0; kelompok M13 di [TASKS.md](TASKS.md) berstatus `N/A_APPROVED` dengan D-55.

## Katalog dan matriks WFP (S7.2.1, S7.2.2)

Katalog sumber per paket Wrike ada di [parity/WRIKE_SOURCE_CATALOG_2026-11-13.md](parity/WRIKE_SOURCE_CATALOG_2026-11-13.md). Setiap WFP-001..106 terpetakan ke ID PRD dan milestone di [parity/WFP_TO_PRD_MATRIX.md](parity/WFP_TO_PRD_MATRIX.md); tidak ada WFP tanpa ID PRD. Hasil katalog yang mengubah register: tidak ada paket Enterprise (WFP-089..093 berada di Pinnacle dan Apex), workflow/status khusus dan dashboard sejak Team, budgeting hanya Pinnacle dan Apex, satu baris ambigu (WFP-022), dan 31 baris yang tidak ada di tabel resmi sehingga perlu bukti dari help center sebelum masuk cakupan 1.0.

