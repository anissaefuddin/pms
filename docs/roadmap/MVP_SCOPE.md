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
