# B3 closure report — v0.14.0 (client-scope MVP)

Status keseluruhan: **PARTIAL.** Implementasi B3 selesai untuk tingkat policy dan mutasi pusat. Verifikasi lintas jalur, operasional,
dan pemulihan belum lengkap. Produksi: **NOT READY**. Dokumen ini tidak mengklaim status yang tidak diverifikasi.

Skala status: IMPLEMENTED · VERIFIED · PARTIAL · BLOCKED · DEFERRED · NOT_APPLICABLE

## 1. Git

- HEAD awal wave ini: `3a034b6`. HEAD akhir: lihat `git log -1` setelah commit laporan ini.
- Commit B3 (urutan): `73bff57` (B3-09/10), `99a0a1b` (regresi), `0a2228b` (B3-11), `bc6d2df` (B3-12), `8de4edc` (B3-13),
  `5f8d1e1` (B3-14), `06ba9f6`/`04c3d77` (B3-15, B3-17 gerbang publik), `ad5ee5b`/`e4c637d` (B3-16), `5264a57` (B3-17 Gantt),
  `b85d93b`/`0017a87` (B3-18/19), `a2afa63`/`a0874a5` (B3-21 runbook dan rehearsal), `39e889b` (fixture reminder).
- Push: **SKIPPED** (wajib). Tidak ada PR, merge, tag, atau deploy.
- Working tree: 26 file untracked asli dipertahankan. Tidak ada perubahan tracked yang belum di-commit.

## 2. Status task B3

| Task | Status | Bukti | Batas |
|---|---|---|---|
| B3-01 skema ownership | VERIFIED | migrasi 00364, `internal/database` (skema, CHECK) | — |
| B3-02 invariant lintas tabel | PARTIAL | CHECK baris (database tests); enforcement policy untuk dependensi, relasi, share | Pembacaan langsung di luar policy belum diaudit penuh |
| B3-03 ownership saat create | VERIFIED | CRT-01..19 | — |
| B3-04 eligibility Space | VERIFIED | SP-01..28 | — |
| B3-05 eligibility item | VERIFIED | IT, ML tests | — |
| B3-06 penugasan dan auto-assign | VERIFIED | AS-01..13, AU-01..02 (automation) | — |
| B3-07 revocasi membership | VERIFIED | RV-01..08 | — |
| B3-08 lokasi, move, restore | VERIFIED | LOC-01..17, CON-1..3 | — |
| B3-09 copy, blueprint, template | VERIFIED untuk blueprint dan template; NOT_APPLICABLE untuk duplikasi item | BP-01..04, SC-07, kontrol negatif | Provenance sumber lintas Customer hanya dicakup oleh cek Space sama; tidak ada endpoint duplikasi item |
| B3-10 dependensi dan relasi | VERIFIED | DR-01..06, `TestRelationTargetsShareOwner`, kontrol negatif | — |
| B3-11 sharing | VERIFIED | SH-01..08, SS-01..04, kontrol negatif | Jalur `governance` join publik tidak mengubah hasil; gerbang akses yang menutup |
| B3-12 Request Form | VERIFIED | FR-01..05, submission setelah arsip, kontrol negatif | Submission publik ditutup oleh B3-17 |
| B3-13 automation, webhook, job | PARTIAL | AU-01..04, webhook queued revocation, kontrol negatif | Automation terjadwal dan konektor tidak diuji end-to-end |
| B3-14 Excel import | VERIFIED | XL-01..05, kontrol negatif (tanpa perubahan produksi) | — |
| B3-15 list, search, analytics | PARTIAL | matriks lintas Customer untuk endpoint prioritas (search, overview, agenda, timeline, workload, todo, shared) | Stars, resource directory, baselines belum diaudit individual; kontrol negatif satu lapisan tidak gagal (lapisan lain sudah menutup) |
| B3-16 realtime dan notifikasi | VERIFIED untuk SSE dan inbox; PARTIAL untuk email | RT, inbox revocation, kontrol negatif; email dicakup tes visibilitas yang ada | Tidak ada tes email khusus revocasi |
| B3-17 file, export, link publik | PARTIAL | `PMS_PUBLIC_LINKS` default OFF (rute tidak terdaftar, tes default dan konfigurasi), reauth Gantt (kontrol negatif) | Lampiran, preview, dan file project ditinjau tetapi tidak diuji; Pydio listing belum dinonaktifkan (dependensi B4) |
| B3-18 dry-run | PARTIAL | `internal/clientscope`: parser ketat, read-only, konflik (MG-01..06) | Quarantine belum ada; konflik form, automation, dan template belum dideteksi |
| B3-19 apply | PARTIAL | idempoten, digest guard, rehearsal restore (lokal) | Audit persisten belum ada (hanya output reconciliation); apply operasional tidak dilakukan |
| B3-20 aktivasi enforcement | PARTIAL | tidak ada switch kompatibilitas (dicari); insert produksi eksplisit atau via trigger | UPDATE path tidak diaudit menyeluruh |
| B3-21 rollback dan recovery | PARTIAL | `docs/ops/CLIENT_SCOPE_RECOVERY_RUNBOOK.md`; rehearsal lokal backup→migrasi→restore→re-apply | RPO/RTO tidak terukur; off-host belum; `restore.sh` dan `restore-test.sh` (Docker) belum dijalankan |
| B3-22 verifikasi final | PARTIAL | laporan ini; regresi Go dan frontend (§3) | Matriks otorisasi tidak dikonsolidasi dalam satu fixture; E2E Playwright belum dijalankan |
| B3-SC Space creation | VERIFIED | SC-01..17 | — |

## 3. Verifikasi

Go (`go test -count=1 ./...` dengan `PMS_TEST_DATABASE_URL`, database uji lokal):
- 31 paket lulus, termasuk `internal/httpserver` (integrasi penuh) dan `internal/database`.
- `internal/duereminders`: satu kegagalan (`TestDeliverQueuesEachReminderOnce`) diperbaiki lewat fixture (commit `39e889b`); paket lulus setelah itu.
- `go vet ./...` dan `gofmt -l .` bersih.

Frontend (`web/`):
- `npm run typecheck`: lulus. `npm run lint`: lulus. `npm test`: 127 lulus, 0 gagal. `npm run lint:tokens`: lulus. `npm run build`: lulus.
- `npm run test:e2e` (Playwright): **tidak dijalankan**.

Kontrak: `openapi_drift_test.go` lulus setelah quoting deskripsi YAML diperbaiki; artefak `openapi.gen.go` dan `schema.d.ts` diregenerasi.

Tidak diverifikasi di sini: CI remote, staging, dan produksi.

## 4. Riwayat kegagalan

Kegagalan awal (56 `internal/httpserver`, ditambah tes `connectors`, `mail`, `database`, dan `duereminders`) berasal dari satu kelas: fixture
yang memasukkan item atau Space lewat SQL mentah, sehingga `scope_state` default `unresolved` dan baris tersembunyi (fail-closed).
Perbaikannya di fixture uji saja (default `internal` di database uji atau kolom eksplisit). Tidak ada default produksi yang diubah.
Kontrak OpenAPI drift (deskripsi tanpa kutip) diperbaiki. Tidak ada tes yang diubah ekspektasinya agar lulus.

## 5. AC-04..AC-13

| AC | Status | Catatan |
|---|---|---|
| AC-04 membership tanpa grant | PARTIAL | Bukti tersebar (SH, XL-02, AS); belum dikonsolidasi |
| AC-05 Workspace Admin tanpa grant | PARTIAL | Analytics admin = 0 (ada); tidak ada matriks lintas Customer untuk admin |
| AC-06 assignment semua jalur | VERIFIED | AS-01..13, AU-02 |
| AC-07 klien A tidak melihat klien B | PARTIAL | Matriks endpoint prioritas; endpoint lain disapu oleh `endpoint_sweep` tanpa skenario lintas Customer |
| AC-08 search, portfolio, workload, laporan | PARTIAL | Matriks prioritas; portfolio dan laporan belum dicakup skenario lintas Customer |
| AC-09 lokasi, move, restore, copy, dependensi, relasi | PARTIAL | Dependensi, relasi, dan lokasi tercakup; move/restore lintas Customer dicakup RV/LOC |
| AC-10 share grup, revocasi, konkurensi | PARTIAL | SH-08 (grup), RV, CON; konkurensi share belum diuji khusus |
| AC-11 blueprint, form, import, automation, job, webhook | PARTIAL | BP, FR, XL, AU, webhook tercakup; job terjadwal belum |
| AC-12 lampiran, preview, ekspor, link | **NOT VERIFIED** | Tidak diuji dalam wave ini |
| AC-13 rekonsiliasi pemetaan, karantina | BLOCKED | Rekonsiliasi sintetis terverifikasi; data operasional NOT VERIFIED; karantina belum ada |

## 6. Kapabilitas opsional yang dinonaktifkan

- Dashboard publik, form publik, dan snapshot Gantt publik: **nonaktif secara default** (`PMS_PUBLIC_LINKS` tidak diset). Rute publik dan rute pengelolaan tautan tidak terdaftar, sehingga token lama juga tidak berfungsi (`TestPublicLinksAreOffByDefault`, `TestPublicLinksDefaultOff`).
- Pydio listing: **tidak dinonaktifkan**. Ia memeriksa visibilitas item dan field sebelum membaca, tetapi verifikasi ACL Pydio adalah tanggung jawab B4.

## 7. Blocker dan prasyarat eksternal

- Migrasi operasional dan dry-run terhadap clone read-only: **EXTERNAL_BLOCKED** (tidak ada clone yang sah). Mapping Customer adalah keputusan bisnis.
- RPO/RTO terukur, backup off-host, drill `restore-test.sh` dengan Docker: **BLOCKED** (OPS-02, OPS-03).
- Alert monitoring: **BLOCKED** (OPS-04, AC-19).
- Remote CI: tidak dijalankan.

## 8. Prasyarat B4

- Verifikasi ACL Pydio nyata (live gate B4-07) sebelum listing Pydio dibuka.
- Keputusan K-21 untuk fallback konektor.
- Penyelesaian B3-02 (audit pembacaan langsung di luar policy) dan B3-13 (job terjadwal) sebaiknya sebelum B4 memakai data ber-scope.

## 9. Kesiapan

- Implementasi B3: PARTIAL (lihat §2).
- Verifikasi keamanan B3: PARTIAL. Kebocoran lintas Customer yang diketahui pada jalur yang diuji sudah ditutup. Jalur lain di §2 belum diuji.
- Kesiapan produksi: **NOT READY**.
