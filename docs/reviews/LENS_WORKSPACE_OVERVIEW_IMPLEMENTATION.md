# LENS — Workspace Overview: Implementasi (status Batch A)

Status: **Batch A selesai** (kontrak API, agregasi aman, tes hak akses, bukti performa). **Batch B (UI Overview dan Spaces) dan Batch C (Timeline dan Agenda) belum dikerjakan.** Perubahan belum di-commit.

## 1. Audit kode (Phase 0, ringkas)

| Area | Temuan | Dipakai ulang |
|---|---|---|
| Hak akses Space | `policy.VisibleSpaces` mengembalikan Space yang bisa dilihat aktor dengan kapabilitas efektif | Ya |
| Registry Space | `spaces.Registry` sudah memfilter dengan `VisibleSpaces` | Pola yang sama |
| Status | `statuses.grp` enum: `active`, `completed`, `deferred`, `cancelled` | Ya |
| Zona waktu | `users.timezone` (default `UTC`) | Ya |
| Arsip Space | `spaces.archived_at` | Ya (lewat `SpaceAccess.Archived`) |
| Dashboard | scope Space, widget `indicator`, `task_list`, dll.; analytics hanya `scope: project | space` | Tidak untuk lintas Space |
| Portfolio, Workload, Calendar | per Space atau per container | Tidak dipakai; read model baru |

## 2. Kontrak API

`GET /api/v1/overview` (didefinisikan di `api/openapi.yaml`, kode generated diperbarui).

| Parameter | Default | Keterangan |
|---|---|---|
| `space_id` | — | Filter satu Space. Space yang tidak terlihat: 404 |
| `include_archived` | `false` | Arsip tetap tunduk akses |
| `sort` | `name` | `name`, `overdue`, `progress`, `last_activity` |
| `limit` | 25 (maks. 100) | |
| `cursor` | — | Cursor offset opak |

Respons: `generated_at`, `timezone`, `kpis`, `spaces` (daftar), `next_cursor`, `total`.

## 3. Definisi KPI

| KPI | Definisi | Batas tanggal |
|---|---|---|
| `accessible_spaces` | Jumlah Space shared yang bisa dilihat aktor (setelah filter arsip) | — |
| `active_projects` | Item `type='project'` dengan grup status `active`, distinct per ID | — |
| `overdue_tasks` | Item `type='task'` grup `active` dengan `due_date` < hari ini di zona pengguna | Hari ini = zona pengguna |
| `completed_this_week` | Item `type='task'` grup `completed` dengan `completed_at` sejak Senin 00:00 zona pengguna sampai sekarang | Senin di zona pengguna, dikonversi ke UTC di SQL |
| `at_risk_projects` | `NOT_AVAILABLE` | Definisi belum disetujui; tidak ada angka asumsi |

Per Space: `project_count`, `task_count`, `done_task_count`, `progress_percent` (null bila tanpa task), `overdue_count`, `health` (`empty` | `attention` | `on_track`), `last_activity`, `effective_access`.

Data personal Space dikecualikan dari KPI manajemen.

## 4. Model keamanan (RBAC)

1. Identitas dari session tervalidasi (`requireUser`). Tidak ada user ID dari parameter.
2. Daftar Space diambil dari `policy.VisibleSpaces`. Space yang tidak terlihat tidak dihitung dan tidak muncul, termasuk namanya.
3. Admin workspace tanpa grant tidak melihat Space apa pun (D-17.1). Tes: `TestWorkspaceOverviewAccessCountsAndTimezone`.
4. Item dihitung per ID dengan `COUNT(DISTINCT)`. Item multi-lokasi tidak dihitung dua kali pada total. Tes: proyek di dua Space, hitung sekali.
5. Hak akses dihitung ulang di setiap request. Tidak ada cache keputusan akses. Cabut share, dan Space langsung hilang. Tes: `TestWorkspaceOverviewRevokeArchiveFilterAndPagination`.
6. Space yang tidak terlihat via `space_id`: 404, bukan daftar kosong.
7. Viewer hanya melihat angka dari Space yang bisa dibaca. Kapabilitas tidak diperluas.
8. Angka beban per orang (workload) **belum disertakan**. Hanya untuk pemegang akses Full, akan dikerjakan di Batch B dengan pemeriksaan terpisah.
9. Drill-down ke Space atau Project memakai route biasa dengan authorization normal.

## 5. Implementasi (Batch A)

| File | Peran |
|---|---|
| `server/internal/overview/overview.go` | Read model: visibilitas, KPI, agregasi per Space, sort, paginasi |
| `server/internal/httpserver/overview.go` | Handler: parsing parameter dan validasi |
| `server/internal/httpserver/spaces.go` | Registrasi route |
| `api/openapi.yaml` | Kontrak endpoint |
| `server/internal/apigen/openapi.gen.go`, `web/src/api/schema.d.ts` | Generated (hanya penambahan) |
| `server/internal/httpserver/overview_integration_test.go` | Tes integrasi (2 fungsi) |

## 5b. Implementasi (Batch B)

| File | Peran |
|---|---|
| `web/src/lib/overview.ts` | Hook `useOverviewSummary` (satu halaman, sort overdue, limit 100) dan `useOverviewPages` (cursor, tombol Muat lainnya). Tipe respons dari `paths["/overview"]` |
| `web/src/components/workspace-overview.tsx` | Mode Ringkasan (KPI, kesehatan, Perlu perhatian, waktu pembaruan) dan mode Ruang (tabel, sort, muat lainnya) |
| `web/src/routes/_app/dashboards.index.tsx` | Segmented control "Dasbor saya" / "Ikhtisar ruang kerja". Filter "Tampilkan dasbor di" tetap hanya untuk mode dasbor |
| `web/src/i18n/en/dashboards.json`, `web/src/i18n/id/dashboards.json` | Kunci `views.*` dan `overview.*` |

Keputusan UI yang diambil:
- Ikhtisar bukan dasbor. Tidak ada tombol edit, hapus, atau bagikan di sana, dan filter lingkup tidak mengubah angkanya.
- At-risk ditampilkan sebagai "–" dengan keterangan "Belum tersedia", sesuai `NOT_AVAILABLE`.
- Kesehatan dan "Perlu perhatian" dihitung dari 100 Space pertama yang dikembalikan server. Jika lebih banyak, ada catatan "Menampilkan N dari M Ruang pertama".
- Tautan hanya ke halaman Space. Tautan ke Project belum ada.
- Tidak ada detail beban kerja per orang.

## 5c. Implementasi (Batch C)

Dua endpoint baru, keduanya read-only dan memakai akses yang sama dengan ikhtisar (Space bersama yang terlihat, D-17.1):

| Endpoint | Parameter | Isi |
|---|---|---|
| `GET /api/v1/overview/timeline` | `from` (YYYY-MM-DD, kosong = hari ini di zona pengguna), `days` (1–90, default 60), `space_id`, `include_archived` | Per Space: Project dengan `due_date` di rentang (sebagai tonggak) dan Task anaknya di Space yang sama; Task tanpa Project di rentang |
| `GET /api/v1/overview/agenda` | `from`, `days` (1–31, default 14), `space_id`, `type` (project/task), `limit` (1–500, default 200), `include_archived` | Item jatuh tempo diurutkan tanggal, penanda `overdue` untuk Task aktif yang lewat; `overdue_count` di luar rentang |

Aturan yang dipakai:
- Tonggak = Project dengan `due_date` di rentang. Item tidak punya tanggal mulai, jadi tidak ada bar durasi.
- Item selesai dan dibatalkan tidak tampil di agenda dan timeline.
- Hubungan induk-anak dibaca dari `item_locations.parent_item_id` di Space yang sama.
- Baca dibatasi 2.000 baris (timeline) dan `limit` (agenda); bila terpotong, respons memberi `truncated: true`.
- Tanggal dibandingkan sebagai tanggal biasa, tanpa konversi UTC. Hari ini dihitung di zona pengguna.
- Agenda menampilkan satu baris per lokasi. Item yang ada di dua Space yang bisa dilihat muncul dua kali, masing-masing di Space-nya.

UI: tab **Linimasa** (daftar per Space, Project bisa dibuka untuk melihat Task-nya, navigasi periode maju/mundur/hari ini) dan **Agenda** (dikelompokkan per tanggal, filter Space dan jenis, penanda Terlambat). Keduanya hanya-baca. Tidak ada grid kalender bulanan dan tidak ada drag.

Tes integrasi baru: `TestWorkspaceOverviewTimelineGroupsDueWork` dan `TestWorkspaceOverviewAgendaOrderOverdueAndKind`. Keduanya lulus bersama empat tes Batch A dan B (`go test ./internal/httpserver -run TestWorkspaceOverview`, 4/4 PASS, PostgreSQL 16 lokal).

## 6. Hasil pengujian

Database: PostgreSQL 16 terisolasi (container lokal, dihapus setelah selesai).

```
go test -count=1 ./internal/httpserver -run 'TestWorkspaceOverview|TestSpaceGovernance|TestSpaceAdminRequires|TestAuthorizationFinal' -v
--- PASS: TestAuthorizationFinalRegressionExplicitAccessOnly
--- PASS: TestWorkspaceOverviewAccessCountsAndTimezone
--- PASS: TestWorkspaceOverviewRevokeArchiveFilterAndPagination
--- PASS: TestSpaceAdminRequiresAdminOrRegularUserType
--- PASS: TestSpaceGovernance
```

`gofmt -l` bersih. `go vet` untuk `internal/overview` dan `internal/httpserver` bersih. Build server berhasil.

Batch B (web), dijalankan di worktree dengan `node_modules` dari repo utama:
- `tsc --noEmit -p .`: lulus.
- `eslint` pada tiga berkas yang diubah: bersih.
- `npm test`: 105 lulus, 0 gagal.

Batch C: `tsc --noEmit` lulus, `eslint` pada berkas baru bersih, `npm test` 105 lulus, `go build ./...` lulus, `go test -run TestWorkspaceOverview` 4/4 PASS.

**Belum dijalankan:** E2E live, CI pada PR, dan deploy.

## 7. Bukti performa

Fixture lokal: 30.000 item (3.000 project, 27.000 task) di 10 Space, satu aktor dengan read di semua Space. Waktu server per request (tiga kali berturut-turut): **86–102 ms**. Hasil KPI konsisten antar-run.

Catatan: fixture ini tidak mewakili beban produksi penuh. Tes performa dengan skala lebih besar dan indeks perlu dilakukan sebelum rilis.

Batch D (pengukuran ulang, fixture 30.000 item, 10 Space, satu aktor dengan read di semua Space, PostgreSQL 16 lokal, waktu terbaik dari 3 kali):

| Permintaan | Waktu | Catatan |
|---|---|---|
| `GET /overview?sort=overdue&limit=100` | 73 ms | KPI dan per Space |
| `GET /overview?sort=name&limit=25` | 70 ms | halaman pertama tab Ruang |
| `GET /overview/timeline` (60 hari) | 27 ms | dibatasi 2.000 baris, jadi cepat karena terpotong |
| `GET /overview/timeline?days=90` | 26 ms | sama, dibatasi 2.000 baris |
| `GET /overview/agenda` (14 hari) | 59 ms | termasuk hitung item terlambat di seluruh Space |
| `GET /overview/agenda?days=31&limit=500` | 60 ms | |

Pengukuran dilakukan dengan tes sementara yang dihapus setelah dijalankan. Angka ini belum mewakili beban produksi. Hitung item terlambat memindai semua Task aktif di Space yang bisa dilihat; bila jumlahnya jutaan, perlu indeks `items(due_date)` atau tabel ringkasan.

## 7b. Verifikasi Batch D

- `go test ./internal/...` (dengan `PMS_TEST_DATABASE_URL`): 21 paket lulus, termasuk `TestWorkspaceOverview*`.
- Matriks otorisasi (pola CI `authorization-matrix`): 33 PASS, 0 SKIP, 0 FAIL. Syarat CI minimal 23.
- Playwright mock, `tests/workspace-overview.spec.ts`: 8 lulus. Mencakup KPI, Ruang dengan pemuatan halaman berikutnya, Linimasa (buka Project, navigasi periode), Agenda (kelompok tanggal, penanda terlambat, filter jenis dan Space), axe pada ringkasan dan agenda, dan tidak ada gulir horizontal pada 390 px di keempat tab.
- Spec dashboard yang sudah ada (`dashboards`, `dashboard-public`, `b2b-a11y-responsive`): 26 lulus.
- `tsc --noEmit` lulus. ESLint bersih untuk berkas yang diubah.

**Belum diverifikasi:** E2E live (`PMS_TEST_BROWSER=1`), deploy ke dev-lens, dan pemeriksaan visual manual pada mode gelap.

## 8. Keputusan terbuka

1. **Pemilihan tempat di `/dashboards`:** disetujui sebagai entry sistem read-only, bukan filter analitik. Sudah diimplementasikan di Batch B sebagai segmented control.
2. **Definisi at-risk:** perlu disetujui. Sampai itu, tampil `NOT_AVAILABLE`.
3. **Sort dan paginasi:** saat ini paginasi di memori setelah daftar Space diambil. Cukup selama jumlah Space yang bisa dilihat satu aktor kecil. Jika ratusan Space, ganti dengan keyset di SQL.
4. **Zona waktu:** diambil dari profil pengguna. Belum ada parameter `tz` dari klien.

## 9. Risiko dan langkah berikutnya

- **Batch B (UI):** selesai secara kode (lihat 5b). Belum ada E2E untuk halaman ini.
- **Cache klien (2 menit):** angka di layar bisa tertinggal sampai dua menit setelah akses dicabut, hanya untuk sesi pengguna yang sama. Server tetap memutuskan akses di setiap permintaan, jadi data yang tidak boleh terlihat tidak bocor. Jika pencabutan harus langsung terlihat, turunkan `OVERVIEW_STALE_MS` atau invalidasi saat mutasi share.
- **Batch C (Timeline dan Agenda):** selesai secara kode dan tes integrasi (lihat 5c). Belum ada E2E atau pemeriksaan visual.
- **Workload per orang:** belum ada. Perlu aturan akses Full terpisah.
- **Performa:** uji dengan skala lebih besar dan pertimbangkan indeks pada `item_locations(space_id)` dan `items(due_date)` jika perlu.
- **Cache:** belum ada. Rencana: cache hanya untuk angka agregat per pengguna selama beberapa menit, dengan kunci yang mencakup versi akses agar pencabutan tetap langsung terlihat.
