# Penilaian kandidat v0.14.0 — B0-01 dan B0-02

| | |
|---|---|
| **Dokumen** | CANDIDATE.md (B0-01 identitas kandidat, B0-02 klasifikasi kasus QA) |
| **Tanggal audit** | 27 November 2026 |
| **Mode** | Read-only. Tidak ada kode aplikasi, migrasi, data, deploy, tes penuh, commit, atau push yang diubah |
| **Rencana acuan** | [Rencana MVP v0.2](../../../roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md) §5 B0-01, B0-02 dan §7 AC |
| **Hasil** | **PARTIAL.** Identitas Git terverifikasi. Versi, CI, dan staging belum bisa dibuktikan pada kandidat. Klasifikasi QA selesai |

Dokumen ini bukan bukti AC-01 atau AC-20 lulus. Tidak ada keputusan GO.

## 1. Identitas Git (B0-01 A)

| Item | Diharapkan (rencana) | Aktual | Status |
|---|---|---|---|
| Branch | `docs/restructure-mvp-roadmap` | `docs/restructure-mvp-roadmap`, 5 commit di depan `origin/main` (tidak tertinggal) | VERIFIED |
| HEAD | `c4f7afc` | `a8477f4` (`docs(roadmap): MVP plan v0.2…`). `c4f7afc` ada, empat commit sebelumnya ada | MISMATCH (HEAD sudah maju satu commit dokumen) |
| Commit di atas v0.13.0 | c096bb3, e779358, fdff421, c4f7afc | `c096bb3`, `e779358`, `fdff421`, `c4f7afc`, `a8477f4` | VERIFIED (plus `a8477f4`) |
| Release `72f9902` | ada, dasar v0.13.0 | ada; tag `v0.13.0` menunjuk ke `72f9902`; merupakan ancestor HEAD | VERIFIED |
| `main` lokal | — | `247ca12` (14 Nov 2026), ancestor HEAD, tertinggal 455 commit dari `origin/main` | MISMATCH (cabang lokal `main` usang) |
| `origin/main` | — | `72f9902` (hanya referensi cache lokal, bukan bukti remote terkini) | NOT VERIFIED (remote tidak dihubungi) |
| Tag release terbaru | `v0.13.0` | `v0.13.0` | VERIFIED |
| Tag `v0.14.0` | belum ada | belum ada | VERIFIED |
| Working tree | bersih untuk berkas tracked | tidak ada berkas tracked yang berubah. Berkas tak-ter-track tetap seperti sesi sebelumnya (`docs/qa/evidence/`, `docs/qa/results/`, PNG di `docs/qa/review-1/`, `outputs/`, `test-results/`) | VERIFIED |

## 2. Versi aplikasi dan skema (B0-01 B)

| Item | Diharapkan | Aktual | Status |
|---|---|---|---|
| Versi yang dideklarasikan repo | `0.13.0` (tag) | `web/package.json` menyatakan `0.12.5` pada `v0.13.0` dan pada HEAD. Versi Go di-set saat build lewat `-ldflags` (Dockerfile), default kode `dev` | MISMATCH (berkas versi tidak dinaikkan saat rilis v0.13.0) |
| Migrasi tertinggi di kode | `00361` (v0.13.0), rencana menyebut skema 361 | `v0.13.0`: `00361_workflow_revision_lock.sql`. HEAD: `00362_seed_runs.sql` (ditambahkan `fdff421`, belum ada di kandidat) | MISMATCH (HEAD menambah satu migrasi di atas kandidat; wajar, tetapi harus dicatat) |
| Skema yang benar-benar diterapkan di staging | 361 atau sesuai kandidat | Tidak terbaca. `GET /api/v1/version` di staging menjawab **401** (memerlukan sesi). Info skema hanya untuk admin | BLOCKED |
| Versi yang berjalan di staging | v0.13.0 (rencana MVP-01) atau lebih baru | Tidak terbaca (lihat §4). Laporan 26 Nov mencatat staging masih v0.12.5 | BLOCKED |
| Tidak ada migrasi dijalankan | — | Tidak ada migrasi, baik di repo maupun di database | VERIFIED |

## 3. Pembanding baseline historis

| Klaim historis | Temuan | Keterangan |
|---|---|---|
| Rilis v0.13.0 = `72f9902` | Sesuai | Tag dan merge PR #28 |
| Commit HEAD `c4f7afc` | Berbeda: HEAD `a8477f4` | Satu commit dokumen sesudahnya |
| Staging v0.12.5 | Belum dapat dikonfirmasi | Dicatat BLOCKED |
| Kandidat `c3782a7` (RC3) | Ada, `CI test/authorization-matrix/parity-e2e` hijau (run 45779890214) | Bukti historis, bukan kandidat sekarang |
| 118 skenario UAT: 105 PASS, 6 BLOCKED, 7 BACKLOG | Sesuai (dihitung ulang dari `UAT_Execution_Result_RC3.xlsx`, sheet UAT). Plus 1 BLOCKED pada E2E | Bukti tersedia |
| 52 defect, 51 `VERIFIED_FIXED` + 1 `DEFERRED_APPROVED` | Sesuai (kolom `latest_status` di `UPDATED_DEFECT_REGISTER.csv`) | Kolom `previous_status` berisi status historis |

## 4. Bukti CI (B0-01 C)

| Item | Hasil |
|---|---|
| Run terakhir pada SHA `72f9902` (run `45794044406`, 26 Nov 09:39 UTC) | **failure**. Job `test` gagal: `TestReportSubscriptionsAreDeliveredOncePerSlotWithTheSubscribersRights` (paket `httpserver`, 295 s). `authorization-matrix` dan `parity-e2e` hijau. `image` dilewati |
| Run sebelumnya pada SHA yang sama (`45786256830`) | **success** untuk `test`, `authorization-matrix`, `parity-e2e` |
| Kandidat RC3 `c3782a7` (run `45779890214`) | success (bukti historis) |
| Run pada HEAD `a8477f4` dan `c4f7afc` | NOT VERIFIED (tidak dicari, tidak ada run di daftar 6 run terakhir) |
| Hubungan run gagal dan sukses | Diduga tes yang tidak stabil (run kedua pada SHA sama berhasil). Penyebab belum ditelusuri; tidak dijalankan ulang |

**Kesimpulan:** kandidat `72f9902` tidak mempunyai CI hijau yang terbaru. Ini blocker kandidat (lihat §7).

Catatan: `gh` digunakan hanya untuk membaca daftar run dan log. Tidak ada workflow yang dipicu.

## 5. Bukti staging (B0-01 D)

Staging: `https://staging.example.invalid/`

| Pemeriksaan | Hasil | Status |
|---|---|---|
| `GET /api/v1/healthz` | HTTP 200 | VERIFIED (hidup) |
| `GET /api/v1/readyz` | HTTP 200 | VERIFIED (siap) |
| Versi terpasang, commit, digest image | Tidak terbaca tanpa sesi admin | BLOCKED |
| Skema yang diterapkan | Tidak terbaca tanpa sesi admin | BLOCKED |
| Kredensial | Tidak diminta, tidak dipakai | — |

Pemeriksaan hanya memakai dua endpoint kesehatan publik yang terdokumentasi.

## 6. Ringkasan QA dan klasifikasi (B0-02)

### 6.1 Sumber

- Laporan utama: [FINAL_RELEASE_REPORT.md](../v0.13.0/FINAL_RELEASE_REPORT.md) (salinan di repo). Bukti lengkap: `outputs/qa-final/pr28-20261126/` (lokal, tidak di git).
- Kasus: `UAT_Execution_Result_RC3.xlsx` sheet **UAT** (118 baris) dan `E2E_Execution_Result_RC3.xlsx` sheet **E2E Journey** (35 baris). Hasil dibaca langsung dari berkas.
- Keterbatasan: `KNOWN_LIMITATIONS_AND_RISK_ACCEPTANCE.md` dan `POST_RELEASE_IMPROVEMENT_BACKLOG.md`.

Semua kasus di bawah berasal dari **RC3 `c3782a7`**, bukan dari `72f9902` atau HEAD. Bukti ini historis dan tidak membuktikan kandidat sekarang.

### 6.2 Kasus BLOCKED dan BACKLOG

| ID | Status awal | Isu | Dampak ke MVP | Klasifikasi | Target task | Owner | Tindakan | Bukti |
|---|---|---|---|---|---|---|---|---|
| T1-01-01 | BLOCKED | Backup lokal lulus (0600, SHA256SUMS, retensi). Tidak ada tujuan off-host (`BACKUP_REMOTE`/`BACKUP_RSYNC_DEST`) | Pemulihan wajib (AC-18) | RELEASE_BLOCKER | OPS-02, OPS-03 | Infra | Siapkan backup off-host dan drill restore | UAT sheet T1-01-01; `evidence/ops/04-backup.log` |
| T3-04-01 | BLOCKED | Tidak ada instance Grafana atau penerima alert. Repo hanya berisi aturan alert, tanpa dashboard | Monitoring wajib (AC-19) | RELEASE_BLOCKER | OPS-04 | Infra | Pasang monitoring dan buktikan pengiriman alert | UAT sheet T3-04-01 |
| E2E-P1-YOPMAIL | BLOCKED | Instance QA terisolasi tidak punya relay SMTP eksternal/kredensial. Undangan dan reset kata sandi bergantung pada e-mail | Login pengguna baru dan reset kata sandi | RELEASE_BLOCKER | R1-05 (SMTP), OPS-01 | Infra, Produk | Uji e-mail ke provider nyata (SPF/DKIM) di staging | E2E sheet E2E-P1-YOPMAIL |
| B4-03-01 | BLOCKED | Booking berfungsi, tetapi **edit booking tidak didukung** (RT-32, ditunda DEC-08) | Tidak memblokir monitoring inti; ada workaround hapus dan buat ulang | DEFERRED_CANDIDATE | RT-32 (§9.1 rencana) | Produk | Keputusan PO untuk menunda | UAT sheet B4-03-01 |
| C1-05-01 | BLOCKED | Default tipe berfungsi; **pewarisan dari parent tidak ada** (RT-15, ditunda DEC-09) | Isi manual; tidak memblokir | DEFERRED_CANDIDATE | RT-15 (§9.1) | Produk | Keputusan PO untuk menunda | UAT sheet C1-05-01 |
| C2-03-01 | BLOCKED | Konektor GitHub dan Google Calendar belum diuji ke akun sandbox. Tes negatif lokal lulus (SEC-N14) | Di luar scope MVP (plan §5 tidak mencakup konektor ini). Risiko bila aktif di production tanpa bukti | NEEDS_EVIDENCE | — (keputusan: aktifkan atau matikan) | Produk, Security | PO memutuskan apakah konektor ini aktif di internal production. Bila aktif, perlu sandbox | UAT sheet C2-03-01 |
| D1-05-01 | BLOCKED | SSO OIDC/SAML belum diuji dengan IdP HTTPS nyata. v0.12.5 menolak issuer http/private/loopback | Autentikasi. Bila produksi memakai SSO, ini blocker; bila tidak, SSO tetap mati (plan §4) | NEEDS_EVIDENCE | — | Produk, Security | PO memutuskan metode login production. Bila SSO dipakai, perlu IdP HTTPS sandbox | UAT sheet D1-05-01 |
| D3-04-01 | BACKLOG | Pemindaian malware untuk form publik (SEC-05) tidak ada | Form publik menerima unggahan anonim (B-1 di analisa). Mitigasi: tutup jalur publik | COVERED_BY_PLANNED_TASK | B3-17 (`PMS_PUBLIC_LINKS` default mati) | Security | Pastikan flag mati di production; SEC-05 tetap backlog | UAT sheet D3-04-01; BACKLOG-PROBE |
| A1-06-01 | BACKLOG | Favorite, hide, duplicate Space tidak ada di build | Bukan syarat monitoring; tidak ada gate | DEFERRED_CANDIDATE | — | Produk | Tunda, catat | UAT sheet A1-06-01 |
| A3-06-01 | BACKLOG | List view tidak ada | Table dipakai sebagai alternatif | DEFERRED_CANDIDATE | — | Produk | Tunda, catat | UAT sheet A3-06-01 |
| F1-01-01 | BACKLOG | AI assistant tidak ada (plan §9: ditunda) | Di luar scope | DEFERRED_CANDIDATE | — | Produk | Tunda, catat | UAT sheet F1-01-01 |
| F1-02-01 | BACKLOG | Whiteboard tidak ada | Di luar scope | DEFERRED_CANDIDATE | — | Produk | Tunda, catat | UAT sheet F1-02-01 |
| F1-03-01 | BACKLOG | Email-to-task tidak ada | Di luar scope | DEFERRED_CANDIDATE | — | Produk | Tunda, catat | UAT sheet F1-03-01 |
| F1-04-01 | BACKLOG | Aplikasi native dan offline tidak ada | Di luar scope | DEFERRED_CANDIDATE | — | Produk | Tunda, catat | UAT sheet F1-04-01 |

Ringkasan: 3 RELEASE_BLOCKER, 1 COVERED_BY_PLANNED_TASK, 8 DEFERRED_CANDIDATE, 2 NEEDS_EVIDENCE, 0 NOT_APPLICABLE. Total 14 = 6 BLOCKED + 7 BACKLOG + 1 E2E.

### 6.3 Pemeriksaan khusus yang diminta

| Topik | Temuan |
|---|---|
| SEC-PUB-2 → B3-17 | Form, dashboard, dan Gantt publik masih aktif di v0.12.5 (verifikasi kode, `dashboards_public.go`, `forms_public.go`, `gantt_snapshots.go`). Blokir di proxy belum dibuktikan. COVERED_BY_PLANNED_TASK |
| STO-1 → B4-08 | `PMS_STORAGE=pydio` memakai `Bearer` dan belum diuji ke Cells nyata. Tidak tercakup QA RC3. Tidak dimasukkan ke klasifikasi karena bukan kasus QA |
| RT-04 → B5-08 | Di `UPDATED_DEFECT_REGISTER.csv` dan `PRODUCT_DECISION_REGISTER`, RT-04 tercatat "terbuka"; di kode, template advanced sudah membawa status dan assignee (catatan 26 Nov). Perlu verifikasi di staging. Bukan kasus BLOCKED |
| Backup dan restore | Wajib gate produksi. Lihat T1-01-01 (RELEASE_BLOCKER) |
| Pydio listing | Bersyarat (B4-07). Lihat MVP-04/13 prosedur. Tidak ada kasus QA RC3 |
| Aturan assign Full di semua jalur | Tidak ada kasus BLOCKED/BACKLOG. Sumber RC3: DEC-06 Rev.1 dan `authorization-matrix` (hijau). Verifikasi ulang di B3-06 |
| D-17.1 (tanpa akses konten otomatis untuk Workspace Admin) | Tidak ada kasus BLOCKED/BACKLOG. Tes `authorization-matrix` mencakupnya. Verifikasi ulang di B3-04 |

### 6.4 Bukti terbaru (lebih baru dari RC3)

Tidak ada laporan QA yang lebih baru dari RC3 di repo. Run CI `45794044406` pada `72f9902` (lihat §4) adalah bukti paling baru, dan gagal.

## 7. Blocker kandidat

| # | Blocker | Klasifikasi | Dasar |
|---|---|---|---|
| 1 | CI pada SHA kandidat gagal di job `test` (run terbaru) | RELEASE_BLOCKER (AC-01) | §4 |
| 2 | Versi di `web/package.json` dan tag tidak sinkron | RELEASE_BLOCKER untuk tagging kandidat | §2 |
| 3 | Staging tidak terverifikasi (versi, skema, digest) | RELEASE_BLOCKER (AC-01) | §5 |
| 4 | Backup off-host belum ada | RELEASE_BLOCKER (AC-18) | T1-01-01 |
| 5 | Monitoring/alert belum ada | RELEASE_BLOCKER (AC-19) | T3-04-01 |
| 6 | E-mail provider eksternal belum diuji | RELEASE_BLOCKER (undangan, reset) | E2E-P1-YOPMAIL |
| 7 | DEF-045 (High, keamanan): undangan bisa memberi Full tanpa cek pengundang. Sudah diperbaiki di 0.12.5-rc3, tetapi audit instance yang sudah terpasang belum dilakukan | RELEASE_BLOCKER sampai audit selesai | `FINAL_RELEASE_REPORT.md` §"Penting"; `52_DEF045…md` |

## 8. Bukti yang hilang dan ketergantungan eksternal

| Bukti | Pemilik | Diperlukan untuk |
|---|---|---|
| Penyebab kegagalan run `45794044406` dan run hijau pada SHA yang sama | Engineering | AC-01 |
| Versi resmi kandidat dan penyesuaian `web/package.json` | Engineering | AC-01 |
| Akses baca staging (sesi admin) untuk versi, commit, digest, skema | Infra | AC-01 |
| Relay SMTP dan akun uji | Infra | E2E-P1-YOPMAIL, AC-21 |
| Tujuan backup off-host dan drill restore | Infra | T1-01-01, AC-18 |
| Grafana/Prometheus dan penerima alert | Infra | T3-04-01, AC-19 |
| Sandbox GitHub/Calendar | Produk | C2-03-01 (keputusan dulu) |
| IdP HTTPS sandbox | Produk, Security | D1-05-01 (keputusan dulu) |
| Audit DEF-045 pada instance terpasang | Security | Blocker 7 |
| Status cabang `main` lokal dan `origin/main` terkini | Engineering | Verifikasi remote |

## 9. Tindakan yang direkomendasikan

Urutan (tanpa memulai B0-03):

1. Engineering: tentukan penyebab run gagal `45794044406` pada `72f9902`. Bila tes tidak stabil, perbaiki atau tandai terpisah. Jangan tag kandidat sebelum CI hijau.
2. Produk: putuskan lima hal (C2-03-01, D1-05-01, metode login production, RT-32, RT-15). Keputusan ini mengubah klasifikasi dan tidak dilakukan oleh audit ini.
3. Infra: siapkan akses baca staging (sesi admin) untuk mengisi §2 dan §5.
4. Lanjut ke **B0-03** setelah PO menyetujui audit ini.

## 10. Batasan

- Hanya membaca Git, berkas repo, dan dua endpoint kesehatan publik staging. Tidak ada tes yang dijalankan.
- Tidak ada akses remote Git. `origin/main` adalah referensi cache lokal.
- Staging tidak bisa dibaca di luar kesehatan publik. Versi, skema, dan digest tidak terverifikasi.
- Klasifikasi adalah usulan. Keputusan akhir ada di Product Owner.
- Kasus diambil dari RC3 `c3782a7`. Hasil ulang pada kandidat baru belum ada.
- Tidak dicek: isi `docs/qa/` yang tidak ter-track, dan daftar lengkap run CI selain 6 run terakhir.
