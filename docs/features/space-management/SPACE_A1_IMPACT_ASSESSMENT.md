# Penilaian dampak A1 (D-17) terhadap 194 tes yang gagal

Sumber: percobaan opsi A pada salinan server di ruang kerja sementara (bukan repo utama). Hasil: 196 tes gagal (194 di paket httpserver, policy, connectors, mail, database, dan campuran; termasuk 2 yang dihitung ganda pada percobaan kedua). Daftar mentah ada di ruang kerja sementara; ringkasan ada di sini.

## Kategori

| Kategori | Jumlah | Arti | Tindakan |
|---|---|---|---|
| FIXTURE | 173 | Langkah setup (create/seed) memakai admin Workspace sebagai aktor tanpa share ke Space. Asersi tidak menyatakan admin melihat apa pun. | Tambahkan share eksplisit ke aktor di fixture. Asersi tidak diubah. |
| KONTRAK | 16 | Asersi memang menyatakan admin Workspace punya akses implisit (caps penuh, total analitik, daftar laporan, pencarian). Perilaku ini berubah secara sengaja oleh A1. | Perlu keputusan dan persetujuan eksplisit per tes sebelum asersi diubah. |
| PERLU TELAAH | 7 | Gagal tanpa pola setup yang jelas (konektor, workload, file review, RT29). | Ditelusuri satu per satu. |
| DEFEK NYATA | 0 | Tidak ditemukan. Semua kegagalan terkait model admin implisit. | - |

Daftar KONTRAK:
- TestAnalyticsEngineMatchesRawDataAndPolicy
- TestAnalyticsMatchesRawDataOnVariedFixtures
- TestAuthorizationMatrix
- TestFormSpaceMetadataFollowsSpaceAccess
- TestGoogleOnlyShowsWhatTheCreatorCanSee
- TestItemAuthorizationMatrix
- TestItemLocationRejectsCycle
- TestLockedSpaceIsAdminOnlyAndAudited
- TestLockedSpaceKeepsItemsAmongThePeopleWhoHaveTheSpace
- TestNotificationWorkerHonoursEmailPreferences
- TestReportActualCostIsAdminOnly
- TestReportTemplatesProduceTheRightNumbers
- TestSpaceCapsRules
- TestUtilizationPlannedVersusActualMatchesRawData
- TestReportsLifecycleAndAccess
- TestSearchPrefixAndScopesRespectVisibility

Daftar PERLU TELAAH:
- TestConcurrentDependenciesCannotCreateCycle
- TestGitHubErrorsAreIsolatedAndBackedOff
- TestGoogleErrorsRefreshAndRateLimit
- TestGoogleOneWayLifecycle
- TestProjectFilesReviewStatusIsPerVersion
- TestRT29UnassignedBucketLabel
- TestResourceWorkload

## Catatan
- Tidak ada asersi yang diubah dalam percobaan ini.
- A1 perlu dirancang sebelum fixture diperbaiki, karena fixture yang diberi share eksplisit hanya akan membuktikan bahwa anggota biasa bisa membaca Space miliknya. Kontrol admin operasional (laporan agregat, audit akses istimewa, approval dengan assignment) belum ada di kode.
- Sebelum A1 benar-benar diterapkan, celah ini harus ditutup: laporan, dashboard, pencarian, export, automation, dan report tidak boleh melewati policy item. Saat ini analitik dan report memakai jalur yang sama dengan admin implisit.


## Pembaruan: kontrak operasional
- Laporan operasional agregat tersedia di `POST /api/v1/analytics/operational` dengan permission `CanViewOperationalReport`, ambang privasi 5 item, tanpa break-down, tanpa biaya, dan audit.
- 7 dari 9 tes kontrak PASS; 2 terbuka (notifikasi, pencarian).
