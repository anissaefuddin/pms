# MVP Scope Freeze — PMS Platform Ready (B0-07)

| | |
|---|---|
| **Status scope** | **PROPOSED.** Belum disetujui Product Owner. Tidak boleh dianggap APPROVED sampai ada persetujuan tertulis |
| **Tanggal** | 27 November 2026 |
| **Acuan** | [Rencana MVP v0.2](LENS_MVP_IMPLEMENTATION_PLAN.md), [ADR-CLIENT-SCOPE-MVP](../architecture/ADR-CLIENT-SCOPE-MVP.md) (PROPOSED), [Impact assessment](PMS_PLATFORM_IMPACT_ASSESSMENT.md), [Penilaian kandidat](../qa/releases/v0.14.0/CANDIDATE.md) |
| **Status B0-07** | **PARTIAL.** Scope dan matriks AC lengkap sebagai usulan. Keputusan PO, data operasional, dan verifikasi kandidat belum ada |

Dokumen ini membekukan scope **usulan**. Tidak memperluas rencana v0.2.

---

## 1. Baseline git

| Item | Nilai | Catatan |
|---|---|---|
| Cabang kerja | `docs/restructure-mvp-roadmap` | Upstream lokal `origin/main` |
| HEAD | Lihat riwayat git (`git log -1`) | Tidak ditulis di dokumen agar tidak stale |
| Rilis dasar | `v0.13.0` = `72f9902` | Ancestor HEAD |
| Commit di atas v0.13.0 | 6 commit (`c096bb3`, `e779358`, `fdff421`, `c4f7afc`, `a8477f4`, `336b290`) | Lima pertama: restrukturisasi, MVP-02/03/04/13, rencana v0.2. `336b290`: checkpoint audit |
| `main` lokal | Usang | Tidak dipakai sebagai basis |
| Cabang remote kerja | **Tidak ada** di `origin` | `ls-remote` hanya menemukan `refs/heads/main` |
| Migrasi terakhir | `00362_seed_runs.sql` | Ditambahkan `fdff421` |
| Versi di `web/package.json` | `0.12.5` | **Tidak sinkron** dengan tag `v0.13.0` |
| Kandidat `v0.14.0` | Belum ada tag | Tidak boleh ditag sebelum B6 |

---

## 2. Scope wajib (usulan)

| Batch | Isi | Rujukan task rencana |
|---|---|---|
| **B1** | Solution Registry statis, hanya `pms`, tanpa UI | B1-01..06 |
| **B2** | Customer Master native (`clients`) dan membership | B2-01..08 |
| **B3** | Kepemilikan stabil, migrasi, dan enforcement otorisasi | B3-01..22 |
| **B4** | Referensi Pydio link-first dan validasi saat tulis | B4-01..08 |
| **B5** | Konfigurasi project, pilot onboarding, dan Portfolio | B5-01..10 |
| **OPS** | Backup off-host, restore, monitoring | OPS-01..05 |
| **B6** | Verifikasi terarah dan persiapan rilis | B6-01..06 |

Setiap batch menghasilkan PR kecil dengan tes. Perubahan migrasi dan `policy` diserialkan (lihat §6).

---

## 3. Scope bersyarat

| Item | Syarat | Keputusan |
|---|---|---|
| Listing Pydio read-only (B4-07) | Live gate lulus: otorisasi, kegagalan kredensial, timeout, folder hilang, tidak bocor di log | Product Owner setelah gate |
| Patch Excel status/assignee (R1-05, RT-04) | Hanya bila onboarding terbukti tidak bisa berjalan tanpa itu | Product Owner setelah pilot |
| SMTP dan reminder (R1-05) | Sesuai kebutuhan operasional. SMTP provider yang akan dipakai | Product Owner dan Infra |
| Konektor GitHub dan Google Calendar | **Default nonaktif** (usulan PROD-02) | Product Owner |

---

## 4. Ditunda secara eksplisit

- Runtime Business Object Registry, tabel MasterRecord, dan Custom Object Types.
- Visual builder dan schema builder.
- Solution Switcher.
- Solution Manifest dinamis dan engine instalasi.
- AIMS, HAZOP Web, RMS.
- Client Hub penuh dan Client 360.
- Portal klien dan pengguna eksternal.
- Sinkron dan tulis Pydio penuh.
- Evidence store engineering.
- SSO dan SCIM (kecuali terbukti perlu untuk akses nyata).
- Talent, Commercial, AI.
- Refaktor dan perbaikan fitur PMS non-kritis di luar daftar ini.

---

## 4a. Aturan implementasi (tidak mengubah scope)

- Tidak ada perubahan kode di luar task yang diizinkan batch.
- Setiap task mengulang tes terarah pada area yang disentuh. Tes lintas-permukaan dijalankan untuk B3.
- Tidak ada tes E2E penuh untuk setiap perubahan kecil.
- Tidak ada keputusan keamanan yang diambil di kode tanpa keputusan tertulis di ADR.

---

## 5. Matriks traceability AC

Status yang dipakai: **NOT STARTED** (belum dikerjakan), **PARTIAL** (ada bukti existing yang belum tentu berlaku untuk kandidat), **BLOCKED** (menunggu keputusan atau data). **Tidak ada AC yang PASS.** Desain dan inspeksi kode tidak cukup untuk PASS.

"Tes existing" = nama yang sudah diverifikasi ada di repo. "Tes usulan" = belum ada; nama final ditetapkan saat task dikerjakan.

| AC | Batch | Metode verifikasi | Tes existing | Tes usulan | Status |
|---|---|---|---|---|---|
| AC-01 | B0-01, B6-01 | Identitas kandidat, CI pada SHA kandidat, versi | CI jobs `test`, `authorization-matrix`, `parity-e2e` (`ci.yml`) | — | **BLOCKED.** CI terakhir pada `72f9902` gagal; versi tidak sinkron; staging tidak terverifikasi |
| AC-02 | B1 | Registry statis; registrasi tidak valid gagal; navigasi tidak berubah | `server/architecture_test.go` `TestPackageDependencies` (paket platform) | Unit `solutions` (SOL-01..02); Playwright navigasi yang ada | NOT STARTED |
| AC-03 | B2 | CRUD klien, kode immutable, duplikat ditolak, arsip, audit | — | Integrasi `clients` (B2-03, B2-05) | NOT STARTED |
| AC-04 | B2, B3-04 | Membership tanpa grant tidak memberi akses konten; audit membership | `httpserver/authorization_final_regression_integration_test.go` `TestAuthorizationFinalRegressionExplicitAccessOnly` (prinsip D-17.1, belum klien) | Fixture negatif B3-22 | NOT STARTED |
| AC-05 | B3-04 | Workspace Admin tanpa grant tidak membaca konten klien/privat | `httpserver/analytics_integration_test.go` (`TestAnalyticsEngineMatchesRawDataAndPolicy`: admin tanpa grant = 0) | Fixture negatif B3-22 | NOT STARTED |
| AC-06 | B3-06 | Full/Editor/Read/Limited dan assignment semua jalur | CI `authorization-matrix` (`-run 'Authorization\|…\|SpaceAdmin\|ItemShare\|Move…'`); `httpserver/assignment_authority_integration_test.go`; `items/assign_error_test.go` | Tes jalur restriktif multi-lokasi (AUTHZ-05) | NOT STARTED |
| AC-07 | B3-04, B3-05, B3-15 | Klien A tidak bisa URL, API, lookup, list, atau count klien B | `httpserver/endpoint_sweep_integration_test.go` `TestEveryProtectedEndpointChecksTheSessionAndSurvivesUnknownIds` (sesi dan ID tak dikenal, belum lintas klien) | Fixture negatif B3-22 | NOT STARTED |
| AC-08 | B3-15, B3-16 | Search, Portfolio, Workload, laporan, notifikasi tidak bocor | `httpserver/search_integration_test.go`, `portfolio_integration_test.go`, `workload_integration_test.go`, `overview_integration_test.go` (ada, belum lintas klien) | Tes lintas klien B3-15 | NOT STARTED |
| AC-09 | B3-08, B3-10 | Lokasi, move, restore, copy, dependensi, relasi lintas klien ditolak | `httpserver/move_race_integration_test.go`, `move_parent_child_regression_integration_test.go`, `move_audit_integration_test.go` (ada, belum lintas klien) | Tes lintas klien B3-08, B3-10 | NOT STARTED |
| AC-10 | B3-02, B3-07, B3-11 | Share grup, membership dicabut, mutasi paralel tidak menembus constraint | `httpserver/item_share_revoke_audit_integration_test.go`, `item_shares_integration_test.go` (ada) | Tes konkurensi dan pencabutan membership | NOT STARTED |
| AC-11 | B3-09, B3-12..14, B3-17 | Blueprint, form, import, automation, job, webhook memakai scope sama | `form_submission_scope_integration_test.go`, `automation_workspace_integration_test.go`, `webhooks_integration_test.go`, `blueprint_hardening_integration_test.go` (ada) | Tes lintas klien per jalur | NOT STARTED |
| AC-12 | B3-17 | Lampiran, preview, ekspor, tautan mengikuti scope dan revoke | `attachment_*_integration_test.go` (5 berkas), `attachment_versions_integration_test.go` | Tes scope dan revoke lampiran | NOT STARTED |
| AC-13 | B3-18, B3-19 | Rekonsiliasi pemetaan; UNRESOLVED dikarantina; tidak ada ID/file hilang | — | Laporan dry-run dan rekonsiliasi (B3-18/19) | **BLOCKED.** Data operasional NOT VERIFIED |
| AC-14 | B4-02..04 | Tautan Pydio hanya untuk pengguna berhak; ACL Pydio terkonfirmasi | `httpserver/pydio_files_integration_test.go` `TestPydioFilesFollowItemAndFieldAccess`; `server/internal/pydio/pydio_test.go` `TestFolderKeepsLinksInsideTheRoots` | Validasi tulis (B4-02); spike live (B4-04) | PARTIAL (tes palsu, belum Cells nyata) |
| AC-15 | B4-07 | Listing read-only lulus otorisasi dan gangguan, **atau** konektor nonaktif dengan fallback disetujui | `pydio_test.go` `TestListSignsTheGatewayRequestAndShapesEntries`; `storage/sigv4_test.go` `TestSignV4MatchesTheAWSExample` | Live gate B4-07 | **BLOCKED.** Belum ada live gate; keputusan K-21 |
| AC-16 | B6-02 | Smoke inti project/task/tanggal/status/assignee/progres/Gantt/Portfolio | Playwright ada (`web/tests/`); `httpserver/gantt_snapshots_integration_test.go`; Playwright di `web/tests/` | Regresi terarah kandidat | PARTIAL (historis pada RC3 `c3782a7`, bukan kandidat) |
| AC-17 | B5-07 | WBS/import pilot terekonsiliasi dan diterima PM | — | Tanda tangan PM | NOT STARTED |
| AC-18 | OPS-02, OPS-03 | Backup/restore off-host dengan checksum, DB + files, runbook | `deploy/backup.sh`, `deploy/restore-test.sh`; `docs/qa/BACKUP_RESTORE_DRILL_REPORT.md` (lokal, historis) | Drill off-host (OPS-03) | **BLOCKED.** Tidak ada tujuan off-host (RC3 T1-01-01) |
| AC-19 | OPS-04, B3-21 | Health, log, alert terverifikasi; rollback tidak membuka data ber-scope | `GET /api/v1/readyz`, `healthz` (ada); `deploy/monitoring/` (ada, tanpa tes) | Rehearsal rollback B6-04 | **BLOCKED.** Alert belum terverifikasi (RC3 T3-04-01) |
| AC-20 | B0-02, B6-03 | Isu BLOCKED/BACKLOG diklasifikasi; tidak ada defek kritis terbuka | Klasifikasi di `CANDIDATE.md` §6.2 (B0-02) | Register final B6-03 | PARTIAL. Klasifikasi selesai, status akhir belum |
| AC-21 | B6-05, B6-06 | Akses production internal saja; rencana onboarding; persetujuan GO tercatat | — | Sign-off PO | NOT STARTED |
| AC-25 *(regresi, dari rencana §11)* | Semua | Fitur PMS sebelumnya tetap berfungsi | CI `test`, `authorization-matrix`, `parity-e2e`; Playwright ada | Regresi terarah kandidat | PARTIAL (historis RC3; CI `72f9902` gagal) |

Catatan: AC-25 tidak ada di daftar AC-01..21 rencana v0.2, tetapi disebut di rencana §11. Ditampilkan di sini agar tidak hilang.

---

## 6. Kelompok tes kritis

| Kelompok | Status existing | Batch | Catatan |
|---|---|---|---|
| Registry PMS-only dan rute existing tidak berubah | Sebagian (arsitektur dan Playwright) | B1 | — |
| CRUD klien, kode immutable, arsip | Tidak ada | B2 | — |
| Membership tanpa grant konten | Tidak ada | B2, B3 | — |
| Klien A vs klien B (langsung, API, lookup, list, count) | Tidak ada | B3 | Fixture B3-22 |
| Workspace Admin tanpa grant | Ada (analitik) | B3 | — |
| Multi-lokasi sama klien vs beda klien | Ada (move, tanpa klien) | B3 | — |
| Assignment semua jalur (Full) | Ada (`authorization-matrix`) | B3 | — |
| SQL langsung, laporan, agregat | Sebagian | B3 | Analitik operasional (F-01) belum |
| Pencabutan membership | Tidak ada | B3 | — |
| Worker, automation, endpoint publik | Ada sebagian | B3 | `reportdigest` gagal di CI |
| Lampiran dan referensi Pydio | Ada | B3, B4 | — |
| Rekonsiliasi migrasi | Tidak ada | B3 | Butuh data |
| Inti project dan monitoring | Ada (Playwright, CI) | B5, B6 | — |
| Backup, restore, rollback | Sebagian (historis) | OPS, B6 | — |

Tes ada tetap dijalankan. Tidak ada tes yang dihapus atau dilemahkan.

---

## 7. Register temuan terbuka

Sumber: CANDIDATE.md, impact assessment (B0-03 §8), B0-04, B0-05, dan ADR §4. **Temuan tidak diperbaiki di dokumen ini.**

| ID | Temuan | Status | Task | Blok coding? | Blok produksi? | Bukti yang dibutuhkan |
|---|---|---|---|---|---|---|
| F-CI-01 | Job `test` gagal pada run `45794044406` (`72f9902`): `TestReportSubscriptionsAreDeliveredOncePerSlotWithTheSubscribersRights` (`httpserver/report_digest_integration_test.go:15`) | OPEN | B6-01 | Tidak (B1/B2 bisa jalan) | **Ya** (AC-01) | Penyebab dan run hijau pada SHA kandidat |
| F-VER-01 | `web/package.json` = `0.12.5`, tag `v0.13.0` | OPEN | B6-01 | Tidak | Ya (penandaan) | Keputusan versi |
| F-STG-01 | Versi dan skema staging tidak terverifikasi | BLOCKED | B0-01 | Tidak | Ya (AC-01) | Akses baca staging yang diotorisasi |
| F-DEF045 | DEF-045 (High): undangan bisa memberi Full. Diperbaiki di `0.12.5-rc3`; instance terpasang belum diaudit | OPEN | B3-11, OPS | Tidak | **Ya** | Audit read-only instance |
| F-AUD-01 | Jalur akses `NEEDS_TRACE` (ADR §5.2, impact §10.1): `restoreStatusAndFields`, `SetArchived`, `UpdateDependency`/`DeleteDependency`, `adminInvite`/`adminGroup`, automation/webhook/template | OPEN | B3 | Tidak (dicek sebelum B3) | Ya | Trace per jalur |
| F-AUD-02 | Analitik operasional melewati gerbang per item (`analytics.go:364–371`) | OPEN | B3-15 | Ya untuk B3 | **Ya** | Perubahan dan tes |
| F-AUD-03 | Tidak ada cache keputusan otorisasi. Rencana B3-07 keliru | Dikoreksi di ADR §3 | B3-07 | Tidak | Tidak | Rencana v0.2 sudah diperbarui |
| F-DATA-01 | Inventaris data operasional tidak tersedia (B0-04) | BLOCKED | B0-04 | **Tidak** untuk B1/B2 | **Ya** (AC-13) | Akses baca yang diotorisasi, identitas terbukti |
| F-MAP-01 | Pemetaan klien operasional tidak tersedia (B0-05) | BLOCKED | B0-05 | **Tidak** untuk B1/B2; **Ya** untuk B3-18..19 | **Ya** | Data, keputusan PO |
| F-BKP-01 | Backup off-host dan restore belum ada (RC3 T1-01-01) | BLOCKED | OPS-02, OPS-03 | Tidak | **Ya** (AC-18) | Tujuan off-host dan drill |
| F-MON-01 | Monitoring dan alert belum terverifikasi (RC3 T3-04-01) | BLOCKED | OPS-04 | Tidak | **Ya** (AC-19) | Penerima alert dan bukti pengiriman |
| F-MAIL-01 | Email ke provider eksternal belum diuji (RC3 E2E-P1-YOPMAIL). Undangan dan reset bergantung padanya | BLOCKED | R1-05, OPS | Tidak | **Ya** (login pengguna baru) | Uji dengan SMTP yang akan dipakai |
| F-PY-01 | Identitas Pydio belum ditetapkan (K-21). Token layanan bersama dipakai untuk listing | DECISION_REQUIRED | B4-05 | Tidak untuk B4-02 | **Ya** untuk listing | Keputusan PO dan spike |
| F-PY-02 | Validasi tautan Pydio hanya saat listing, tidak saat tulis | OPEN | B4-02 | Tidak | Tidak (listing ditolak) | Tes penulisan |
| F-DEC-01 | `DEC-06 Rev.1` dikutip sebagai assignment, padahal biaya. Assignment = DEC-10 | OPEN (dokumen) | B3-06 | Tidak | Tidak | Perbaikan rujukan |
| F-DEC-02 | `D-08` di rencana menunjuk blueprint, bukan DECISIONS.md | OPEN (dokumen) | — | Tidak | Tidak | Label yang jelas |
| F-SCH-01 | `items.owner_id` sudah berarti pengguna. Nama kolom klien harus berbeda | DECISION_REQUIRED (K-23) | B3-01 | **Ya** untuk B3-01 | Tidak | Keputusan penamaan |
| F-SCH-02 | Referensi polimorfik tanpa FK (`shares.target_id`, `custom_field_assignments.scope_id`, `custom_field_grants.subject_id`) | DECISION_REQUIRED (K-29) | B3-19, B3-22 | Ya untuk B3-01 | Ya (integritas) | Keputusan FK atau cek aplikasi |
| F-SCH-03 | Duplikat definisi Customer dan posisi Workspace tidak dijamin unik di database | OPEN | B0-05, B3 | Tidak | Ya (pemetaan) | Data |
| F-PRD-01 | Keputusan PO K-20..K-28 belum disetujui | DECISION_REQUIRED | — | **Ya** (untuk B1 dimulai resmi) | **Ya** | Persetujuan tertulis |
| F-DEC-03 | Amandemen D-55 belum diputuskan | DECISION_REQUIRED | — | Tidak untuk B1 | Ya | Keputusan PO |
| F-GIT-01 | Cabang remote kerja tidak ada. Enam commit belum terbit | OPEN | — | Tidak | Tidak | Keputusan PO soal publikasi |
| F-GIT-02 | `main` lokal usang | OPEN | — | Tidak | Tidak | Sinkronisasi (tidak dilakukan di sini) |

---

## 8. Keputusan yang dibutuhkan dari Product Owner

Hanya yang blok B1 atau B2 secara langsung:

1. **Setujui scope §2–4** sebagai baseline implementasi (F-PRD-01).
2. **Setujui K-25..K-28 dan K-21..K-22** sesuai ADR §2.4..2.5.
3. **Tentukan pemberi persetujuan INTERNAL** (K-28).
4. **Putuskan K-23** penamaan kolom klien, dan **K-29** FK untuk referensi polimorfik. Keduanya blok B3-01.
5. **Setujui atau tolak amandemen D-55** (F-DEC-03).

Keputusan lain (K-18 host production, K-21 listing Pydio, PROD-01..08) menunggu sebelum B5 dan produksi, bukan sebelum B1.

### 8.1 Penambahan ke DECISIONS.md

Bagian PROPOSED ditambahkan ke `docs/architecture/DECISIONS.md` sebagai referensi (bukan keputusan APPROVED). Lihat commit terkait.

---

## 9. Kesiapan dan ukuran

### 9.1 Status kesiapan

| Area | Status | Alasan |
|---|---|---|
| Keputusan arsitektur (ADR) | **PROPOSED** | Belum disetujui PO |
| Scope freeze | **PROPOSED** | Belum disetujui PO |
| Implementasi B1 (registry statis) | **Siap dimulai setelah scope disetujui.** Tidak menunggu data | Tidak ada dependensi data |
| Implementasi B2 (klien) | **Siap dimulai setelah scope dan K-25..K-28 disetujui** | K-28 hanya dibutuhkan B5 |
| Implementasi B3 (kepemilikan) | **Tidak siap.** Menunggu B2, K-23, K-29, dan analitik F-AUD-02 ditangani | Dependensi |
| B4 (Pydio) | Spike dan validasi tulis dapat paralel. Listing menunggu K-21 | — |
| OPS | Dapat paralel. Menunggu host dan target off-host | — |
| Migrasi | **NOT READY** | Data operasional tidak tersedia (F-DATA-01, F-MAP-01) |
| Produksi | **NOT READY** | AC-01, AC-13, AC-15, AC-18, AC-19, AC-21 belum terpenuhi |

Kesiapan implementasi mandiri (B1, B2) **tidak** menunggu pemetaan operasional. Pemetaan operasional, backup, dan pemulihan wajib sebelum cutover produksi.

### 9.2 Ukuran relatif (bukan estimasi hari)

| Batch | Ukuran | Dependensi kritis |
|---|---|---|
| B0 | S–M (selesai sebagian) | — |
| B1 | S | B0-06, scope |
| B2 | M | B0-06, K-25..K-28 |
| B3 | **L** (risiko tertinggi) | B2, K-23, K-29, F-AUD-02 |
| B4 | M | B0-06 (K-21, K-22) |
| B5 | M | B3, B4 |
| OPS | M | Host, target off-host |
| B6 | M | B1–B5, OPS |

Ukuran ini relatif antar batch. Tidak ada estimasi hari.

### 9.3 Urutan

```text
B0 (selesai sebagian) ──► B1 ──────────────┐
                    └──► B2 ──► B3 ──► B5 ──► B6
B0 ──► spike Pydio (B4-04) ──► B4 ─────────┘
B0 ──► OPS ────────────────────────────────┘
```

- B1 dan B2 boleh paralel setelah scope disetujui.
- B3 menunggu B2.
- Spike Pydio dan OPS boleh paralel bila aman.
- Perubahan migrasi dan `policy` **diserialkan**: satu PR terbuka pada satu waktu.

---

## 10. Batas dokumen

- Tidak ada tes yang dijalankan.
- Tidak ada data operasional yang dibaca.
- Tidak ada AC yang ditandai PASS.
- Tidak ada keputusan yang ditandai APPROVED.
- Ukuran dan urutan adalah usulan.

---

## 11. Bukti eksekusi B1 (27 Nov 2026)

Otorisasi: Product Owner, hanya B1 (lihat ADR §7b). Tidak ada batch lain yang dikerjakan.

| Task | Status | Bukti |
|---|---|---|
| B1-01 Registry statis | DONE (lokal) | `server/internal/solutions/solutions.go`: `Builtin()` berisi tepat satu `pms`, salinan independen. Tes `TestBuiltinContainsOnlyTheActivePMSContract`, `TestBuiltinReturnsIndependentCopies`, `TestValidateRejectsInvalidRegistries` (9 kasus) |
| B1-02 Validasi startup | DONE (lokal) | `server/cmd/pms/main.go`: `checkSolutions` dipanggil di `run()` untuk command `serve`, **sebelum** `database.Open` (yang menjalankan migrasi). Tes `TestCheckSolutionsGatesTheStart` |
| B1-03 Aturan arsitektur | DONE (lokal) | `server/architecture_test.go`: `solutions` ditambahkan ke `isPlatform`. `TestPackageDependencies` lulus. Uji negatif (paket probe sementara yang mengimpor paket bisnis) gagal sesuai harapan, lalu paket probe dihapus |
| B1-04 Ekspos operator | DEFERRED | Tidak dibutuhkan (keputusan PO) |
| B1-05 Navigasi tidak berubah | PARTIAL | Tidak ada berkas frontend atau OpenAPI yang berubah. Smoke navigasi runtime **tidak dijalankan**: tidak ada lingkungan uji lokal yang disiapkan untuk ini. Tidak diklaim PASS |
| B1-06 Kontrak ekstensi | DONE (dokumen) | ADR §7a |

**Hasil verifikasi:**

| Perintah | Hasil |
|---|---|
| `go test ./server/internal/solutions` | ok (3 tes utama, semua subtes lulus) |
| `go test ./server/cmd/pms` | ok (`TestCheckSolutionsGatesTheStart`) |
| `go test ./server -run '^TestPackageDependencies$'` | ok |
| Uji negatif aturan arsitektur | Gagal sesuai harapan (probe dihapus) |
| `go build ./server/cmd/pms` (ke path sementara) | ok |
| `go vet` paket yang diubah | ok |
| `gofmt -l` berkas yang diubah | kosong |
| `git diff --check` | ok |

**Berkas yang berubah:** `server/internal/solutions/solutions.go` (baru), `server/internal/solutions/solutions_test.go` (baru), `server/cmd/pms/main.go` (+15 baris), `server/cmd/pms/main_test.go` (baru), `server/architecture_test.go` (1 baris), dokumen ADR dan scope.

**Penyimpangan dari rencana:** validasi dipasang di `run()` sebelum `database.Open`, bukan di awal `serve()`. Alasan: `database.Open` sudah menjalankan migrasi, dan registry yang tidak valid harus menghentikan start sebelum itu.

**AC-02:** PARTIAL. Registry dan validasi lulus tes. Bagian navigasi runtime belum diverifikasi.

**Keterbatasan:** `TestCheckSolutionsGatesTheStart` menguji fungsi yang dipanggil `run()`, bukan `run()` itu sendiri. Tidak ada tes integrasi yang menjalankan binary dengan registry tidak valid. Tes semacam itu belum dibuat.

**CI historis:** `TestReportSubscriptionsAreDeliveredOncePerSlotWithTheSubscribersRights` tetap terbuka dan tidak disentuh.

---

## 12. Bukti eksekusi B2-01/02 (skema, 27 Nov 2026)

Otorisasi: Product Owner, hanya skema B2-01 dan B2-02 (ADR §7c). B2-03 dan seterusnya belum dikerjakan.

| Task | Status | Bukti |
|---|---|---|
| B2-01 `clients` | DONE (skema) | `server/migrations/00363_clients.sql`. Lihat ADR §7c |
| B2-02 `client_memberships` | DONE (skema) | Migrasi yang sama. Model riwayat, satu aktif per pasangan |

**Hasil tes** (`go test ./server/internal/database`, `PMS_TEST_DATABASE_URL` menunjuk ke PostgreSQL lokal. Setiap tes membuat database `pms_test_<angka>` sendiri dan menghapusnya setelah selesai. Tidak ada staging atau production):

| Tes | Cakupan | Hasil |
|---|---|---|
| `TestClientsMigrationUpDownFromThePreviousSchema` | DB-01, DB-02, DB-15, DB-16 | PASS |
| `TestClientIdentityConstraints` | DB-03, DB-04, DB-05, DB-06, DB-07, DB-08, DB-11 (klien), perubahan kode ditolak | PASS |
| `TestClientMembershipConstraints` | DB-09, DB-10, DB-11 (membership), DB-12 | PASS |
| `TestMembershipRevokeAndRegrantKeepHistory` | DB-13 | PASS |
| `TestDeletesAreRestrictedWhenMembershipHistoryExists` | DB-14 | PASS |
| `TestSchemaRules` (sudah ada) | Aturan skema existing | PASS |
| `TestUpgradeFromTheEarlySchemaKeepsEveryRow` (sudah ada) | Upgrade dari skema awal | PASS |

Tiap penolakan dicek dengan SQLSTATE yang tepat (`23505`, `23514`, `23503`) dan dijalankan di transaksi sendiri, sehingga tidak ada yang lolos karena transaksi sebelumnya gagal.

**Temuan selama pengujian:** `btrim()` PostgreSQL hanya memotong spasi, sehingga nama atau kode yang hanya berisi tab lolos sebagai "tidak kosong". Diperbaiki di migrasi dengan regex kelas whitespace. Tes DB-05 dan DB-06 sekarang mencakup kasus tab dan newline.

**Yang belum diuji atau belum dilakukan:**
- Tidak ada tes API atau aplikasi. Skema belum ada yang memakainya.
- Kode immutable diuji di database. Pemeriksaan tambahan di aplikasi tetap wajib di B2-03 sebagai lapisan kedua.
- Upgrade dan rollback di lingkungan operasional belum dilakukan (B3/B6).
- Migrasi belum dijalankan di staging atau production.

**Status AC:** AC-03 dan AC-04 **tidak** PASS. Keduanya membutuhkan CRUD, aturan domain, pengujian otorisasi, dan audit yang belum ada.

**Sisa untuk B2-03:**
- Guard aplikasi untuk kode yang tidak berubah (lapisan kedua).
- Operasi CRUD klien, arsip, dan audit (`client.created`, `client.archived`, dan seterusnya).
- Grant dan revoke membership dengan audit, serta aturan arsip yang membatasi kerja baru.
- Pemeriksaan kelayakan di picker.

---

## 13. Bukti eksekusi B2-03 (domain service, 27 Nov 2026)

Otorisasi: Product Owner, hanya domain service B2-03 (ADR §7e). Tidak ada endpoint, UI, audit, atau enforcement.

| Item | Status | Bukti |
|---|---|---|
| B2-03 domain service | DONE (domain) | `server/internal/clients/clients.go`, `membership.go` |
| Tes domain | DONE | `clients_integration_test.go`: 26 PASS, 0 FAIL, 0 SKIP |
| Enforcement API dan resource | PENDING | B2-04 dan B3 |
| Audit | PENDING | B2-05 |
| AC-03 | **PARTIAL** | CRUD dan aturan domain lulus tes. API, audit, dan kontrol akses belum ada |
| AC-04 | **PARTIAL** | Membership tanpa izin konten dibuktikan di domain. Tes otorisasi resource belum ada |

**Perintah:** `PMS_TEST_DATABASE_URL=postgres://…@localhost:5432/postgres?sslmode=disable go test ./server/internal/clients -count=1 -v` (database uji dibuat dan dihapus oleh harness). `go test ./server -run '^TestPackageDependencies$'`: PASS.

**Pemeriksaan mutasi:** penguncian `FOR SHARE` pada grant dihapus sementara. MEM-11 gagal, dan dikembalikan. Tes konkurensi ini memang menangkap kesalahan penguncian.

**Batas:** tidak ada pemeriksaan otorisasi (B2-04). Tidak ada audit (B2-05). Tidak ada rute (B2-06). Tidak ada tes migrasi baru. Tidak ada tes lintas klien pada resource (B3).

---

## 14. Bukti eksekusi B2-04..06 (authorization, audit, API, 27 Nov 2026)

Otorisasi: Product Owner, untuk B2-04, B2-05, dan B2-06 (ADR §7f). B2-07 dan B2-08 belum dikerjakan, dan tidak ada enforcement klien di Space atau item (B3).

| Task | Status | Bukti |
|---|---|---|
| B2-04 Authorization | DONE | `requireUser(requireAdmin(...))` pada 8 operasi (`httpserver/clients_api.go`). API-01..06 |
| B2-05 Audit | DONE | Lima aksi di `audit.go`. Event ditulis dalam transaksi yang sama (`clients.Recorder`). AUD-01..07 |
| B2-06 REST API dan OpenAPI | DONE | 8 operasi di `api/openapi.yaml`. Go dan TypeScript digenerate ulang dengan alat yang ada. API-07..17 |
| AC-03 | **PASS** (tingkat CRUD dan audit) | Stabil dan kanonik (CL-01, CL-07), duplikat ditolak (CL-03, API-09), arsip menjaga riwayat (CL-08, API-12), CRUD diaudit (AUD-01) |
| AC-04 | **PARTIAL** | Membership diaudit (AUD-01). Membership tanpa grant tidak memberi akses konten **untuk admin** (AUD-07). Pengujian membership-pengguna-biasa terhadap resource menunggu B3 |

**Hasil tes** (`PMS_TEST_DATABASE_URL` menunjuk ke PostgreSQL lokal, database uji dibuat dan dihapus oleh harness. Tidak ada staging atau production):

| Perintah | PASS | FAIL | SKIP |
|---|---:|---:|---:|
| `go test ./server/internal/clients -count=1 -v` (domain, termasuk konkurensi) | 26 | 0 | 0 |
| `go test ./server/internal/httpserver -run '^(TestClientAPIRefusesNonAdministrators\|TestClientAPIContract\|TestClientAuditTrail\|TestClientManagementGrantsNoContentAccess)$' -v` | 21 | 0 | 0 |
| Regresi: `TestEveryProtectedEndpointChecksTheSessionAndSurvivesUnknownIds`, `TestAbandonedRequestIsNotAServerError`, `TestAuthorizationFinalRegressionExplicitAccessOnly`, `TestAudit*`, `TestAdmin*` | 14 | 0 | 0 |
| `go test ./server -run '^TestPackageDependencies$' -v` | 1 | 0 | 0 |
| `go build ./server/...`, `go vet` paket yang diubah, `gofmt -l` | lulus | — | — |

Angka di atas adalah hasil dari perintah yang dijalankan sebelum commit. Perintah tidak memakai cache.

**Pemeriksaan mutasi.** Dua mutasi sementara dibuat dan dikembalikan:

- `FOR SHARE` pada grant dihapus: MEM-11 gagal (grant tidak menunggu arsip).
- Audit dalam transaksi gagal (trigger uji pada `audit_events`): AUD-05 memastikan klien tidak tercommit.

**Temuan selama pengerjaan (dan koreksi):**

- Generator `oapi-codegen` pertama kali tampak tidak memuat operasi baru. Ternyata itu pengamatan keliru atas file yang belum diperbarui. Setelah dijalankan ulang dengan konfigurasi resmi, file memuat 45 kecocokan untuk operasi klien. Setelah itu diverifikasi bahwa tidak ada baris lama yang hilang.
- Satu tes AUD-03 gagal karena saya mengirim nama yang berbeda dan menganggapnya "nama sama". Kodenya benar: perubahan nama menulis `client.updated`. Tes diperbaiki.
- Run tanpa `PMS_TEST_DATABASE_URL` (semua tes di-skip) tidak dipakai sebagai bukti. Hanya run dengan env yang diset dan output verbose yang dihitung.

**Keterbatasan:**

- Daftar klien belum punya cursor. Batasnya 200 per halaman.
- Tidak ada Customer Picker (B2-08) dan belum ada pengaturan UI (B2-07).
- Belum ada enforcement klien pada Space, item, atau resource. Itu B3.
- AC-04 tetap PARTIAL sampai B3 membuktikan pengguna dengan membership tapi tanpa grant tidak bisa membaca konten klien.

---

## 15. Bukti eksekusi B2-07 (Customer Settings UI, 27 Nov 2026)

Otorisasi: Product Owner, hanya B2-07 (Customer Management UI di Settings). B2-08 belum dikerjakan. Tidak ada perubahan backend, OpenAPI, atau migrasi.

| Item | Status | Bukti |
|---|---|---|
| Navigasi Settings | DONE | Item "Customers" di grup "Account management", hanya untuk admin (`components/settings-sidebar.tsx`) |
| Daftar | DONE | `routes/_app/settings/customers/index.tsx`: filter Active/Archived, pencarian pada hasil yang dimuat, tabel, state loading, kosong, dan error dengan retry |
| Buat, edit, arsip | DONE | Dialog buat dengan validasi; edit hanya `name` dan `short_name`; arsip dengan konfirmasi |
| Detail dan anggota | DONE | `routes/_app/settings/customers/$clientId.tsx`: tab Umum dan Anggota, picker hanya admin dan staf aktif, cabut dengan konfirmasi |
| Terjemahan EN dan ID | DONE | 93 kunci `settings.customers.*` di `i18n/en` dan `i18n/id` |
| AC-03 | PASS (tidak berubah) | Sudah dibuktikan di backend pada §14 |
| AC-04 | PARTIAL (tidak berubah) | Menunggu B3 |

**Perintah dan hasil** (Playwright dengan API ter-mock, `npm run dev` oleh konfigurasi):

| Perintah | Hasil |
|---|---|
| `npx playwright test tests/customers.spec.ts` | 6 PASS, 0 FAIL, 0 SKIP |
| `npx playwright test tests/settings-access.spec.ts tests/rates.spec.ts` (regresi Settings) | 4 PASS, 0 FAIL, 0 SKIP |
| `npm test` (unit) | lulus |
| `npm run typecheck` | lulus |
| `npx eslint` pada berkas yang diubah | 0 error (1 peringatan "file ignored" untuk `lib/*.ts` yang juga muncul pada berkas lain) |
| `npm run build` | lulus |
| `node scripts/check-no-hex.mjs` | lulus |

**Cakupan tes:** UI-01 (admin melihat menu), UI-02 dan UI-03 (non-admin tidak melihat dan tidak bisa membuka URL langsung tanpa memanggil API), UI-05, UI-07, UI-08, UI-09, UI-10, UI-11, UI-12, UI-13, UI-14, UI-15, UI-16, UI-17, UI-18, UI-19, UI-20, UI-22, UI-23 (ID), dan pemeriksaan axe pada daftar, formulir, detail, dan picker. Loading dan error (UI-06) diuji dengan kegagalan 500 yang diikuti retry berhasil.

**Keterbatasan:**
- Pencarian dan daftar hanya pada hasil satu halaman, maksimal 200 klien, karena API belum punya cursor. Halaman menampilkan pemberitahuan bila hasil mencapai batas itu.
- Picker anggota memakai pencarian server (`q`) pada endpoint pengguna, dengan maksimal 50 hasil.
- Tes berjalan dengan API ter-mock, bukan backend nyata. Tes otorisasi backend ada di §14.
- Pencegahan submit ganda saat request masih berjalan (UI-21) diterapkan dengan tombol disabled dan tidak diuji secara terpisah.
- Pengarsipan belum mengubah akses ke proyek atau item. Ini dinyatakan di dialog dan dikerjakan di B3.

## 16. Bukti eksekusi B2-08 dan penutupan B2 (Eligibility dan Customer Picker, 27 Nov 2026)

Otorisasi: Product Owner, hanya B2-08 (lookup kelayakan dan Customer Picker yang dapat dipakai ulang; ADR §7h). Tidak ada migrasi, tidak ada perubahan pada delapan operasi Customer Management, dan picker belum dipasang di form Space atau Project.

| Item | Status | Bukti |
|---|---|---|
| Domain lookup | DONE | `server/internal/clients/eligibility.go`: `ListEligible` (membership, SQL) dan `ListActiveForAdmin` (administrative) |
| Endpoint | DONE | `GET /api/v1/clients/eligible` (`httpserver/clients_api.go`), `requireUser`; administrative memakai `requireAdmin` |
| Batas administrative | DONE | Non-admin mendapat 403 dan tidak ada query (EL-09; PK-06) |
| Picker yang dapat dipakai ulang | DONE | `web/src/components/customer-picker.tsx`, aturan murni di `web/src/lib/customer-picker.ts` |
| OpenAPI dan generated | DONE | `api/openapi.yaml`: `listEligibleClients`, skema `ClientOption`. `server/internal/apigen/openapi.gen.go` dan `web/src/api/schema.d.ts` hanya bertambah (214 dan 57 baris, 0 dihapus) |
| Terjemahan EN dan ID | DONE | 10 kunci `settings.customers.picker.*` di `i18n/en` dan `i18n/id` |

**Keputusan keamanan**

1. Filter kelayakan ada di database: `EXISTS` pada membership aktif, `status='active'`, `user_type IN ('admin','regular')`, dan `status='active'` pada pengguna, semuanya sebelum pencarian dan `LIMIT`. Daftar tidak diambil penuh lalu disaring di klien.
2. Identitas pemanggil hanya dari sesi atau API key. Tidak ada parameter `user_id`.
3. Pencarian memakai parameter dan pola ILIKE yang di-escape (`%` dan `_` tetap literal). `q` dibatasi 100 karakter.
4. Kelayakan bukan izin. Tidak ada kontrol izin baru, tidak ada akses Space atau item, dan tidak ada audit untuk lookup (EL-14).
5. Tidak ada cache server. Pencabutan berlaku pada permintaan berikutnya. Cache klien (`staleTime` 30 detik, dihapus saat logout) dapat menampilkan opsi lama selama maksimal 30 detik. Pemilihan yang tidak lagi valid akan ditolak di B3.

**Perintah dan hasil**

| Perintah | Hasil |
|---|---|
| `PMS_TEST_DATABASE_URL=… go test ./internal/httpserver -run TestCustomerEligibilityLookup -count=1 -v` | PASS: 15 kasus EL. EL-04 dan EL-05 berada di satu subtest; EL-08 juga memeriksa bahwa lookup tidak membuat membership (EL-14), dan EL-14 punya subtest sendiri |
| `go test ./internal/httpserver -run 'TestClientAPIRefusesNonAdministrators\|TestClientAPIContract\|TestClientAuditTrail\|TestClientManagementGrantsNoContentAccess'` | PASS (regresi API Customer dan sweep endpoint) |
| `go test . -run Architecture` (paket server) | PASS |
| Mutasi: hapus `m.status='active'` dari SQL membership | EL-04 GAGAL, sebagaimana diharapkan. Berkas dikembalikan dan diverifikasi identik |
| `npm test` (unit, termasuk `customer-picker.test.ts`) | 127 PASS, 0 FAIL |
| `npx playwright test tests/customer-picker.spec.ts` | 10 PASS (PK-01 sampai PK-10) |
| Mutasi: hapus ID pengguna dari cache key | PK-08 GAGAL, sebagaimana diharapkan. Berkas dikembalikan |
| `npx playwright test tests/customers.spec.ts` (regresi UI Customer) | 6 PASS |
| `npm run typecheck`, `npx eslint` pada berkas yang diubah, `node scripts/check-no-hex.mjs`, `npm run build` | lulus (0 error lint; peringatan "file ignored" untuk `lib/*.ts` seperti biasa) |
| `go tool oapi-codegen --config api/oapi-codegen.yaml api/openapi.yaml` (dari akar repo), `npm run generate` | generated diperbarui; diff hanya penambahan |

**Cakupan tes PK-01..PK-10.** Harness uji di `web/tests/fixtures/customer-picker/` adalah fixture khusus tes. Harness ini tidak ada di router, tidak ikut build produksi (Vite hanya membangun `index.html`), dan memanggil API yang di-mock. Harness dipakai agar picker bisa diuji tanpa rute demo atau halaman Settings baru. PK-01 sampai PK-05 dan PK-10 berjalan di browser. PK-06 dan PK-08 berjalan di browser dan aturan murninya juga diuji di unit. PK-07 berjalan di browser dan unit. PK-09 diuji di browser dan kesetaraan kunci EN/ID diuji di unit.

**Cakupan EL-01..EL-15.** Pengujian memakai database nyata dan sesi nyata. Tes ini belum menjalankan browser terhadap backend nyata.

**Batas**

- Picker menampilkan paling banyak 50 opsi per permintaan. Pencarian berjalan di SQL sebelum `LIMIT`, jadi setiap customer yang eligible dapat dicari. Hasil yang lebih dari 50 harus dipersempit dengan pencarian. Tidak ada cursor.
- Query membership memakai `client_memberships_active_user_idx`. Rencana eksekusinya belum diverifikasi dengan `EXPLAIN`.
- Picker belum dipasang di form produksi. Integrasi dilakukan di B3.
- Tidak ada enforcement klien pada Space, item, atau resource. Itu B3.

**Status B2 setelah B2-08**

| ID | Status |
|---|---|
| B2-01 dan B2-02 | DONE: skema (§12) |
| B2-03 | DONE: domain service (§13) |
| B2-04, B2-05, B2-06 | DONE: otorisasi, audit, API (§14) |
| B2-07 | DONE: UI Settings (§15) |
| B2-08 | DONE: lookup kelayakan dan picker (§16) |

**Penutupan B2:** implementasi B2 selesai. AC-03 tetap PASS. AC-04 tetap PARTIAL sampai B3 karena membership belum menjadi gerbang akses resource. Verifikasi integrasi frontend dan backend yang hidup belum dilakukan dan tertunda. Client ownership belum diimplementasi. Status produksi: NOT READY.

## 17. Bukti eksekusi B3-01 dan B3-02 fondasi (Skema kepemilikan, 27 Nov 2026)

Otorisasi: Product Owner, hanya B3-01 dan B3-02 (ADR §7i). Tidak ada enforcement kebijakan, create, move, copy, restore, atau backfill operasional. Remote checkpoint masih BLOCKED (Prompt #013).

| Item | Status | Bukti |
|---|---|---|
| B3-01 kolom `client_id` dan `scope_state` pada `spaces` dan `items` | DONE | `server/migrations/00364_client_scope.sql` |
| Constraint baris (state, konsistensi client, Personal Space internal, FK) | DONE | OWN-DB-05 sampai OWN-DB-12, OWN-DB-17 |
| Baris lama tetap `unresolved`; Personal Space lama `internal` | DONE | OWN-DB-03, OWN-DB-04, OWN-DB-17 (lama) |
| Customer tidak bisa dihapus saat dirujuk; arsip mempertahankan referensi | DONE | OWN-DB-13, OWN-DB-14 |
| Owner, hierarki, lokasi, dan `space_admins` tidak berubah | DONE | OWN-DB-15, OWN-DB-16 (`items.owner_id`, `item_locations`, `space_admins`, dan nama serta jenis Space) |
| Down dan up pada basis data sementara | DONE | OWN-DB-18 |
| Protokol transaksi lintas tabel | DOCUMENTED (ADR §7i) | Belum diterapkan pada jalur mutasi |
| Enforcement lintas tabel pada mutasi | PARTIAL | Menunggu B3-03, B3-08, dan B3-09 |

**Perintah dan hasil** (basis data uji lokal yang terisolasi):

| Perintah | Hasil |
|---|---|
| `go test ./internal/database/ -run 'TestOwnership\|TestCustomerLifecycle' -count=1 -v` | PASS: 18 kasus OWN-DB, 0 FAIL, 0 SKIP |
| `go test ./internal/database/ -run 'TestClientsMigrationUpDownFromThePreviousSchema\|TestClientIdentityConstraints\|TestClientMembershipConstraints\|TestMembershipRevokeAndRegrantKeepHistory\|TestDeletesAreRestrictedWhenMembershipHistoryExists'` | PASS. Satu perubahan pada test: `provider.Down` menjadi `provider.DownTo(ctx, clientsVersion-1)`, karena sekarang ada 00364 yang harus ikut diturunkan |
| `go test ./internal/database/ -run 'TestUpgradeFromTheEarlySchemaKeepsEveryRow\|TestSchemaRules\|TestOpenRefusesOldBaselineDatabase\|TestSpaceAdminBackfill\|TestOpenConcurrentlyOnEmptyDatabase\|TestItemLocationRejectsCycle\|TestItemEffortMigration'` | PASS (regresi migrasi yang ada) |

**Keterbatasan:**
- Tes OWN-DB-15 dan OWN-DB-16 memeriksa `items.owner_id`, `item_locations`, `space_admins`, dan nama serta jenis Space. Tabel `shares` dan `custom_field_grants` tidak dibandingkan secara terpisah.
- Pengujian konkurensi belum ada. Protokol di ADR §7i belum dibuktikan.
- Belum ada item `client`, jadi invarian lintas tabel belum bisa dilanggar. Itu tidak membuktikan enforcement.
- Klasifikasi operasional (pemetaan data nyata ke Customer) **belum diverifikasi**.
- Isolasi klien belum diimplementasikan. AC-04, AC-07, AC-09, dan AC-10 belum PASS.
- Remote checkpoint dan CI masih BLOCKED.

## 18. Bukti eksekusi B3-03 (kepemilikan saat pembuatan item, 27 Nov 2026)

Otorisasi: Product Owner, B3-03 dan enforcement pembuatan dari B3-02 (ADR §7j), termasuk remediasi fixture Prompt #016 dan #017. Tidak ada migrasi baru, tidak ada perubahan kebijakan, dan tidak ada B3-04.

| Item | Status | Bukti |
|---|---|---|
| Kepemilikan root dari Space tujuan | DONE | `items/ownership.go` `creationOwnership`, dipanggil dari `items/create.go` |
| Anak mewarisi induk, induk harus cocok dengan Space | DONE | CRT-04, CRT-09, CRT-10, CRT-16 |
| Personal Space internal | DONE | CRT-06, CRT-10 |
| Customer diarsipkan ditolak (409, `customer_archived`) | DONE | CRT-11, CRT-17 |
| Unresolved ditolak (422, `ownership_unresolved`) | DONE | CRT-07, CRT-08 |
| Penulisan atomik dan tidak ada sisa parsial | DONE | CRT-13, CRT-14 |
| Input ownership dari klien ditolak | DONE | CRT-15 |
| Owner_id tetap referensi pengguna | DONE | CRT-12 |
| Serialisasi dengan arsip Customer | DONE | CRT-17, CRT-19 (`FOR SHARE` Customer, Space, induk) |
| Pemetaan HTTP per jalur pembuatan | PENDING | Belum diverifikasi di luar HTTP create |
| Pembuatan Space bersama dengan ownership | PENDING | Space API bersama tetap `unresolved` |
| Enforcement lintas tabel pada mutasi (B3-02) | PARTIAL | Menunggu B3-08 dan B3-09 |

**Perintah dan hasil** (basis data uji lokal, binary hasil `go test -c`, setiap tes dijalankan sendiri dengan `-test.v`):

| Grup | PASS | FAIL | SKIP | PANIC |
|---|---:|---:|---:|---:|
| 39 tes pembuatan dan hierarki lama | 39 | 0 | 0 | 0 |
| CRT-01..19 (5 fungsi, 18 subtest) | 23 entri | 0 | 0 | 0 |
| Penugasan dan Personal Space (22 tes `httpserver`) | 22 | 0 | 0 | 0 |
| Arsip Customer di `clients` (2 tes) | 2 | 0 | 0 | 0 |

Rekonsiliasi hitung: laporan #016 menjumlahkan 16 PASS dan 8 FAIL dari 24 baris. Itu menggabungkan 22 tes `httpserver` (14 PASS dan 8 gagal) dengan 2 tes `clients` (2 PASS). Setelah remediasi, 22 tes `httpserver` PASS dan 2 tes `clients` PASS, sehingga total 24 PASS.

Build (`go build ./...`), `go vet` (`items`, `httpserver`, `clients`), `gofmt`, dan `git diff --check` lulus.

**Remediasi fixture (Prompt #015, #016, #017):** 30 fungsi di 10 berkas (#016) dan 8 fungsi regresi penugasan di 3 berkas (#017). Dua tes opt-in (`TestRankingAtScale`, `TestBrowserCreateNested`) juga diperbaiki, karena sebelumnya tercatat PASS padahal dilewati. Semua perubahan hanya pada setup fixture. Tidak ada asersi yang diubah. Tidak ada perubahan kode produksi sejak Prompt #015, dan tidak ditemukan product defect.

**Keterbatasan:**
- Pembuatan Space bersama lewat API masih menghasilkan `unresolved`. Production tidak boleh dideploy sebelum alur ownership-aware Space selesai.
- Jalur pembuatan selain HTTP create (form, template, connector, automation, blueprint, impor) memakai error yang sama, tetapi pemetaan HTTP per jalur belum diverifikasi.
- Tidak ada pengujian konkurensi lintas mutasi. Itu tugas B3-08 dan B3-09.
- OWN-DB-16 (`shares`, `custom_field_grants`) belum lengkap dan dialihkan ke B3-22.
- Isolasi klien belum selesai. AC-04, AC-07, AC-09, AC-10, dan AC-13 tetap terbuka. Kesiapan produksi NOT READY.

## 19. Bukti eksekusi B3-04 (eligibility Space pusat, 27 Nov 2026) — PARTIAL

Otorisasi: Product Owner, hanya B3-04 (ADR §7k). Tidak ada perubahan `item.go`, SharedOnEveryPath, migrasi, atau B3-05.

| Item | Status | Bukti |
|---|---|---|
| Predikat eligibility di query Space pusat | DONE | `policy/engine.go` `spaceRows`, dipakai `VisibleSpaces`, `SpaceAccessFor`, `CanOnSpace` |
| Membership tidak memberi izin resource | DONE | SP-01..03, SP-07, SP-14, SP-27 |
| Workspace Admin dan Space Admin tidak melewati eligibility | DONE | SP-08..SP-12 |
| Internal dan Personal Space tidak berubah | DONE | SP-17, SP-18 |
| Unresolved fail-closed | DONE | SP-16 |
| Customer diarsipkan: akses historis anggota aktif | DONE | SP-19, SP-20 |
| Revocation langsung | DONE | SP-06, SP-26 (kontrol negatif) |
| Daftar, lookup, dan kueri kapabilitas konsisten | DONE | SP-21..SP-25 |
| Bypass item (`item.go`) | PENDING B3-05 | |
| Regresi tes Space lama | FAIL (blocker) | Lihat di bawah |

**Hasil tes:**
- `TestClientSpaceEligibility` (SP-01..SP-28): 28 subtest PASS.
- Kontrol negatif (membership revoked diterima): SP-06, SP-20, SP-26 gagal. Engine dikembalikan identik.
- Paket `policy`: `go test ./internal/policy/` ok.
- Pengukuran regresi pada binary akhir: 78 tes Space terkait 24 PASS dan 54 gagal/panic. 39 tes pembuatan dan hierarki 31 PASS, 8 FAIL. 22 tes penugasan dan privasi 15 PASS, 7 FAIL. CRT-01..19: 23 entri PASS.

**Perubahan fixture (hanya tes CRT, tidak ada asersi yang dilemahkan):**
- Membership ditambahkan untuk kreator yang dimaksud pada Customer tempat item dibuat (CRT-01..19, dan viewer di `TestOwnershipDoesNotChangeCreatePermissions`).
- CRT-07: ekspektasi berubah dari 422 `ownership_unresolved` menjadi 404. Space unresolved sekarang tidak terlihat, sehingga penolakan terjadi lebih awal dan gagal tertutup. Pemeriksaan B3-03 tetap berlaku untuk induk unresolved (CRT-08).
- CRT-17: `defer tx.Rollback` ditambahkan. Tanpa itu, kegagalan meninggalkan koneksi terkunci dan `pool.Close` menggantung.

**Temuan harness:** `f.want` dan `f.call` melaporkan ke `t` induk. Dari dalam subtest, kegagalan menghentikan subtest berikutnya. `fixture.on(t)` dipakai di tes SP. Pola yang sama ada di tes CRT lain dan perlu diperiksa di tugas regresi.

**Belum selesai:**
- Regresi 54 tes Space lama, 8 tes pembuatan, dan 7 tes penugasan. Penyebabnya adalah fixture `unresolved` dan membership yang belum ada. Perlu tugas regresi terpisah dengan keputusan membership per tes.
- Pembuatan Space bersama dengan ownership masih pending.
- Bypass item (B3-05) dan pembacaan `spaces` langsung di luar policy.
- OWN-DB-16 tetap di B3-22.
- B3-02 tetap PARTIAL. AC-04, AC-05, AC-07, AC-09, AC-10, dan AC-13 tetap terbuka. Isolasi klien belum selesai. Operasional belum diverifikasi. Kesiapan produksi NOT READY.

## 20. Pembaruan B3-04: konvensi fixture dan ledger regresi (27 Nov 2026, Prompt #019) — PARTIAL

Otorisasi: Product Owner, penyelesaian B3-04 dengan remediasi regresi dan konsolidasi fixture (ADR §7l). Tidak ada perubahan produksi.

| Item | Status | Bukti |
|---|---|---|
| Default `f.space` dan `f.spaceWithoutAdmins` = ownership internal eksplisit | DONE | `spaces_integration_test.go` |
| `f.unresolvedSpace` untuk negatif | DONE | SP-16 |
| Helper membership eksplisit | DONE | `f.addMember`, `f.revokeMember` |
| Harness: subtest memakai `t` sendiri | DONE untuk SP dan CRT | `fixture.on(t)` |
| SP-01..28 | 28/28 PASS | `TestClientSpaceEligibility` |
| CRT-01..19 | 23/23 entri PASS | 5 fungsi, 18 subtest |
| Policy | ok | `go test ./internal/policy/` |
| Tes Space terkait | 70/78 PASS (7 FAIL, 1 PANIC) | Semua pembuatan Space lewat API |
| Pembuatan dan hierarki | 37/39 PASS (2 FAIL) | Sama |
| Penugasan dan privasi | 18/22 PASS (4 FAIL) | Sama |
| Ownership-aware shared Space creation (B3-SC) | PENDING, wajib sebelum penutupan B3 | |

**Kesimpulan:** grup wajib belum 100% hijau, sehingga B3-04 belum di-commit. Penyebab sisa 12 tes adalah satu hal: pembuatan shared Space lewat API ditolak karena pembuat tidak melihat Space `unresolved` yang baru. Ini bukan kegagalan fixture. Perbaikannya ada di B3-SC.

**Keterbatasan:**
- Hanya grup yang ditentukan yang dijalankan. Perubahan default `f.space` (dipakai di 134 berkas) belum diuji di seluruh suite. Ini risiko yang belum diverifikasi.
- Subtest tes lain di luar SP dan CRT masih memakai pola harness lama.
- B3-02 PARTIAL. AC-04 PARTIAL. Bypass item (B3-05) terbuka. Bypass SQL langsung di luar policy terbuka. OWN-DB-16 ada di B3-22. Operasional belum diverifikasi. Kesiapan produksi NOT READY.

## 21. Bukti penutupan B3-SC dan B3-04 (27 Nov 2026, Prompt #020) — COMPLETE untuk tingkat policy dan creation pusat

Otorisasi: Product Owner (B3-SC, koreksi bootstrap General, integrasi Customer Picker di Create Space, penyelesaian regresi B3-04). Keputusan dan kontrak: ADR §7m. Tidak ada migrasi baru. Tidak ada B3-05 atau B3-06.

| Item | Status | Bukti |
|---|---|---|
| Kontrak `scope_state` dan `client_id` pada create dan template | DONE | OpenAPI `SpaceCreate`, `createSpaceFromTemplate`; artefak Go dan TS diregenerasi |
| Validasi kombinasi, state, Customer, membership | DONE | SC-01..SC-11 |
| Admin tidak melewati eligibility | DONE | SC-09 (kontrol negatif terbukti) |
| Transaksi atomik, tanpa Space yatim | DONE | SC-11; validasi sebelum INSERT, rollback penuh |
| Serialisasi dengan arsip dan revoke | DONE | SC-16, SC-17 (tunggu-kunci deterministik) |
| Ownership tidak berubah lewat PATCH | DONE | SC-12 |
| General bootstrap `internal` eksplisit, idempoten | DONE | `bootstrap_integration_test.go` (TestBootstrapCreatesFoundationOnce) |
| Personal tetap privat | DONE | SC-15, SP-18 |
| Create Space UI dengan Customer Picker membership | DONE | `space-create-ownership.spec.ts` (5 PASS) |
| F1 (HTTP 500 pada pembuatan) | RESOLVED | Pembuatan valid sekarang 201; yang tidak valid mendapat 4xx yang jelas |

**Hasil verifikasi (binary akhir, setiap tes dijalankan sendiri dengan `-test.v`, `PMS_TEST_SCALE=1`, `PMS_TEST_BROWSER=1`):**

| Grup | PASS | FAIL | SKIP | PANIC |
|---|---:|---:|---:|---:|
| SP-01..28 (`TestClientSpaceEligibility`) | 28 subtest | 0 | 0 | 0 |
| Tes Space terkait (78) | 78 | 0 | 0 | 0 |
| Pembuatan dan hierarki (39) | 39 | 0 | 0 | 0 |
| Penugasan dan privasi (22) | 22 | 0 | 0 | 0 |
| CRT-01..19 | 23 entri | 0 | 0 | 0 |
| SC-01..17 (`TestSpaceCreationOwnership`) | 14 subtest | 0 | 0 | 0 |
| Bootstrap | 2 | 0 | 0 | 0 |
| Policy, architecture | ok | 0 | 0 | 0 |
| Frontend Space create (`spaces.spec.ts` 19, `space-create-ownership.spec.ts` 5) | 24 | 0 | 0 | 0 |
| Frontend onboarding (3) dan a11y Space (6) | 9 | 0 | 0 | 0 |

**Rekonsiliasi 12 dan 14:** 14 entri gagal di Prompt #019 (8 di grup Space, 2 di pembuatan, 4 di penugasan) berasal dari 12 tes unik. Semuanya memanggil `POST /spaces` atau template. Setelah kontrak mewajibkan ownership dan payload tes mengirimnya secara eksplisit, ke-12 tes lulus. Asersi 201 tetap ada, dan `UPDATE` sementara sudah dihapus.

**Build dan lint:** `go build ./...`, `go vet` (`spaces`, `httpserver`, `bootstrap`, `policy`), gofmt, `npm run typecheck`, `npx eslint` pada berkas yang diubah, `npm run build`, `check-no-hex`, dan `git diff --check` lulus. Generator dijalankan dari root repositori; tidak ada file salah tempat.

**Keterbatasan:**
- Frontend diuji dengan API ter-mock. Ini verifikasi antarmuka, bukan integrasi hidup.
- Tidak ada suite penuh yang dijalankan. Perubahan default `f.space` (dipakai di 134 berkas) belum diverifikasi di seluruh suite. Ini risiko yang masih terbuka.
- Bypass item (B3-05): `policy/item.go` tidak memakai predikat Space. Akses item lewat share di Space client tetap terbuka.
- Pembacaan `spaces` langsung di luar policy (`spaces/`, `automation/`, `analytics/`, `housekeeping/`) tetap bypass terbuka.
- Space produksi `unresolved` yang sudah ada tetap tidak terlihat sampai pemetaan operasional (B3-18/19). Pemetaan belum diverifikasi.
- B3-02 PARTIAL (invarian lintas tabel pada mutasi). AC-04, AC-07, AC-09, AC-10, dan AC-13 belum PASS. Isolasi klien belum lengkap. Kesiapan produksi NOT READY.
- OWN-DB-16 (`shares`, `custom_field_grants`) tetap di B3-22.

## 22. Bukti eksekusi B3-05 dan B3-06 (eligibility item dan penugasan, 27 Nov 2026, Prompt #021) — COMPLETE pada tingkat policy pusat

Otorisasi: Product Owner (ADR §7n). Tidak ada migrasi, perubahan OpenAPI, atau frontend. Tidak ada B3-07 atau tugas lanjutan.

| Item | Status | Bukti |
|---|---|---|
| Gate kepemilikan item sebelum union permission | DONE | `policy/item.go` `itemGate` pada anchor `itemsCaps`; IT-01..15 |
| Eligibility membership klien per item | DONE | IT-02..IT-05, IT-07..IT-10, IT-14 |
| Lokasi tidak kompatibel tidak menyumbang | DONE | ML-02..ML-07 |
| Jalur form dan Personal | DONE | IT-12, ML-06 |
| Full on every path lewat gate | DONE | `policy/share_paths.go` `eligibleItems`; AS-01..AS-06 |
| Target assignee harus anggota Customer | DONE | `items/assignee_eligibility.go`; AS-08, AS-09, AS-12 |
| Revoke dan stale assignee | DONE | AS-11, AS-13 |
| Tidak ada membership atau grant otomatis | DONE | AS-10 |
| Metadata lokasi ditolak tidak bocor | DONE | ML-08 |

**Hasil verifikasi** (binary akhir, setiap tes dijalankan sendiri, `PMS_TEST_SCALE=1`, `PMS_TEST_BROWSER=1`, `PMS_TEST_SCHEDULING_PERFORMANCE=1`):

| Grup | PASS | FAIL | SKIP | PANIC |
|---|---:|---:|---:|---:|
| IT-01..15 | 15 subtest | 0 | 0 | 0 |
| ML-01..08 | 8 subtest | 0 | 0 | 0 |
| AS-01..12 dan AS-13 | 12 + 1 | 0 | 0 | 0 |
| SP-01..28 | 28 subtest | 0 | 0 | 0 |
| SC-01..17 | 14 subtest | 0 | 0 | 0 |
| Tes Space terkait (78) | 78 | 0 | 0 | 0 |
| Pembuatan dan hierarki (39) | 39 | 0 | 0 | 0 |
| Penugasan dan privasi (22) | 22 | 0 | 0 | 0 |
| CRT-01..19 | 23 entri | 0 | 0 | 0 |
| Item, sharing, lokasi, otorisasi, penugasan, dan scheduling (90 tes) | 90 | 0 | 0 | 0 |
| Policy, bootstrap, items, architecture | ok | 0 | 0 | 0 |

Build, vet, gofmt, dan `git diff --check` lulus. Kontrol negatif dijelaskan di ADR §7n.

**Keterbatasan:**
- Suite penuh tidak dijalankan. Gate item menyentuh semua pembacaan item, dan fixture yang sudah disesuaikan hanya di grup yang disebut di atas.
- Pembacaan `items` langsung di luar policy belum diverifikasi satu per satu (ADR §7n).
- Mutasi lokasi lintas tabel (B3-08/09) belum ditegakkan, sehingga B3-02 tetap PARTIAL.
- AC-04, AC-05, AC-07, AC-09, dan AC-10 hanya mendapat bukti parsial dari tes di atas. Isolasi klien belum lengkap. Kesiapan produksi NOT READY.
