# Gate G2a — Resource, jadwal, dan biaya (M10, S10.5.1)

**Tanggal:** 14 November 2026. **Hasil: BELUM LULUS.** [G0](G0_STABILIZATION.md) dan [G1a](G1A_CONFIG.md) belum lulus (D-60, D-72); gate ini mencatat bukti otomatis. UAT oleh orang (checklist di bawah) belum dilakukan, jadi tidak ada butir yang dinyatakan "UAT lulus".

## Requirement M10

| Req | Story | Bukti otomatis | Status |
|---|---|---|---|
| RSC-01 | S10.1.1 | `TestResourceWorkload` (kapasitas dengan libur dan cuti, hanya diri sendiri atau admin yang mengubah), E2E-PAR-07 live | **Terpenuhi** |
| RSC-02 | S10.1.2, S10.1.3 | Tabel kasus, `TestWorkloadPerformance`, `TestBrowserWorkload`, halaman Workload | **Terpenuhi** (hasil performa 50 × 2.000 ada di TESTING.md) |
| RSC-03 | S10.1.4 | Realokasi lewat PATCH item; audit satu event; beban berpindah | **Terpenuhi** |
| RSC-04 | S10.2.1 | CRUD role dan skill, hanya admin, filter workload | **Terpenuhi** |
| RSC-05 | S10.2.2 | Booking per project, orang, atau role masuk workload, konflik vs kapasitas | **Terpenuhi** |
| RSC-06 | S10.2.3 | `TestUtilizationPlannedVersusActualMatchesRawData` (kapasitas, terencana, aktual cocok data mentah) | **Sebagian**: panel di halaman Workload; widget dan template report belum |
| VIEW-05C | S10.3.1 | `TestBaselinesFreezeDatesAndCannotBeChanged`, E2E-PAR-08 live, axe | **Sebagian**: tanggal baseline sebagai kolom; bar baseline butuh SVAR PRO |
| SPC-16 | S10.3.2 | `TestGanttSnapshotsHidePrivateDataAndExpire` (beku, nama assignee opsional, kedaluwarsa, cabut, hak edit), E2E live snapshot tanpa akun dengan axe | **Terpenuhi** (snapshot tidak memuat link dependensi) |
| SCH-01 | S10.3.3 | `TestPortfolioTimelineCrossProjectDependencyAndPolicy` dan `TestCrossProjectDependenciesRespectPermissions` (dependensi lintas project dan Space, siklus ditolak, ujung tersembunyi 404, project tersembunyi tidak diungkap), `portfolio.spec.ts` | **Terpenuhi** untuk timeline dan dependensi lintas project; halaman memakai daftar ID project (bukan pemilih), bar sederhana (bukan SVAR), dan tanpa mengubah tanggal |
| TASK-14 | S10.4.1, S10.4.2 | `TestRatesSnapshotKeepHistoryAndCostIsAdminOnly` (rumus, snapshot, masking), `rates.spec.ts` | **Sebagian**: kolom cost di report belum; izin finance = admin (D-74) |

## Checklist UAT untuk orang (belum dijalankan)

1. Atur kapasitas dua orang dengan cuti, lihat Workload satu minggu, geser satu task ke orang lain dan periksa tanda kelebihan beban.
2. Buat booking untuk role, pastikan muncul sebagai permintaan role dan masuk beban.
3. Simpan baseline, geser tanggal task, periksa kolom Baseline dates dan Variance.
4. Buat snapshot publik, buka di jendela tanpa login, ubah rencana, pastikan snapshot tetap, lalu cabut.
5. Buat dependensi antara task dua project di dua Space, buka Portfolio.
6. Atur tarif, catat waktu billable, ubah tarif, periksa actual cost tidak berubah untuk entry lama.

Retest 14 Nov 2026 meluluskan PAR-07/08, snapshot publik, portfolio, utilization dan biaya. Workload p95 139,4 ms pada 50 orang dan 2.000 task. Tabel beban harian dan utilization kini memiliki nama aksesibel yang berbeda. [Bukti retest](../../quality/TESTING.md#retest-teknis-gate-m8m10-14-november-2026).

## Temuan saat M10

1. Gantt di dalam project tidak menampilkan baris karena induk tingkat atas menunjuk project yang bukan baris chart; diperbaiki.
2. Dua sesi mengerjakan working tree yang sama: nomor migrasi 00056 dan nomor keputusan D-72 bentrok; milik sesi ini dipindah ke 00060 dan D-73.
3. `GET /users/{userId}/capacity` menjawab 403 bagi orang luar, sengaja (kapasitas privat); sapuan endpoint kini menerima 403 untuk jalur itu.
4. Snapshot Gantt dari sesi lain belum terpasang di router, sehingga tesnya gagal; dipasang, ditambah pencabutan, batas laju publik, dan dokumentasi OpenAPI.

## Risiko yang diterima

- Bar baseline tidak ada di edisi gratis SVAR; ditampilkan sebagai kolom (D-73).
- Satu angka tarif per jam tanpa mata uang sendiri; biaya diformat dengan mata uang field budget.
- Snapshot publik berisi judul dan tanggal item yang terlihat oleh pembuat; judul bisa berisi data sensitif, pembuat bertanggung jawab memilihnya.

## Keputusan yang diminta

1. Jalankan checklist UAT di atas dengan orang, lalu putuskan kelulusan G2a.
2. Tentukan apakah kolom cost di report dan bar baseline (SVAR PRO) dikerjakan sebelum G2a atau ditunda.
