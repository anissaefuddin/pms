# Gate G2b — File, analitik, dan integrasi (M11, S11.4.1)

**Tanggal:** 14 November 2026. **Hasil: BELUM LULUS.** Bukti otomatis untuk 18 requirement M11 tersedia; UAT oleh orang belum dilakukan, dan uji konektor dengan akun GitHub dan Google sungguhan belum dijalankan. [G0](G0_STABILIZATION.md), [G1a](G1A_CONFIG.md), [G1b](G1B_AUTOMATION_APPROVALS.md) dan [G2a](G2A_RESOURCE_SCHEDULE_COST.md) juga belum lulus, sehingga gate ini mencatat bukti dan tidak menyatakan lulus. Bukti rinci ada di [TESTING.md](../../TESTING.md) (bagian "M11").

## Requirement M11

| Req | Story | Bukti otomatis | Status |
|---|---|---|---|
| VIEW-10 | S11.1.1 | `TestProjectFiles` (hanya lampiran yang dapat dilihat, tanpa akses 404), `project-files.spec.ts` | **Terpenuhi** |
| FIL-01 | S11.1.2 | `TestAttachmentVersionsAreImmutableAndPolicyChecked` (versi tidak berubah, akses per versi) | **Terpenuhi** |
| FIL-02 | S11.1.3 | `TestProofingAnnotationsStayOnTheirVersion`, `file-proofing.spec.ts` | **Terpenuhi** |
| FIL-03 | S11.1.4 | `TestCompareTwoVersionsOfOneFile`, `file-compare.spec.ts` | **Terpenuhi** |
| FIL-04 | S11.1.5 | `TestFileApprovalPinnedToVersionWithExternalReviewer` (E2E-PAR-09 tingkat API), `file-review.spec.ts` | **Terpenuhi** |
| TASK-08B | S11.1.6 | `TestS3AdapterAgainstFakeServer`, `TestPydioAdapterAgainstFakeServer` | **Sebagian**: adapter diuji terhadap server palsu, bukan S3 atau Pydio sungguhan |
| DASH-03B, DASH-08 | S11.2.1, S11.2.2 | `TestAnalyticsWidgetsAreaPivotBurndownCompletion` (angka cocok dengan data mentah) | **Terpenuhi** |
| DASH-04, DASH-05 | S11.2.3 | `TestDashboardLayoutAndWidgetDescription`, `dashboards.spec.ts` | **Terpenuhi** |
| DASH-07 | S11.2.4 | `TestPublicDashboardLinkExpiryPasswordAndFinancialMasking` (kedaluwarsa, password, data uang disembunyikan) | **Terpenuhi** |
| RPT-04 | S11.2.5 | `TestReportScheduleDailyAndChosenTimeZone`, `TestReportSubscriptionsAreDeliveredOncePerSlotWithTheSubscribersRights` | **Sebagian**: terkirim sampai antrean `send_email`; SMTP sungguhan belum diuji |
| API-01, SET-08 | S11.3.1 | `TestAPIKeysScopeRotateRevokeAndOwnerLimits` (hash, scope, rotasi, cabut efektif), `api-keys.spec.ts` | **Terpenuhi** |
| API-02 | S11.3.2 | `TestWebhookSignedDeliveryAndVisibility`, `TestWebhookRetryThenAutoDisable`, `TestWebhookRejectsInternalTargets`, `TestWebhookSecretIsEncryptedAtRest`, `webhooks.spec.ts` | **Terpenuhi** (rotasi secret belum ada) |
| API-03 | S11.3.3 | `TestOAuthCodePKCEAndRevocation` (PKCE, kode sekali pakai, pencabutan efektif), `oauth.spec.ts` | **Terpenuhi** |
| INT-01 | S11.3.4 | `TestLifecycleAndSecrets` (pasang, atur, nonaktif, rahasia terenkripsi), `TestConnectorRoutes` | **Terpenuhi** |
| INT-02 | S11.3.5 | `TestSyncBothWaysNoSilentOverwriteAndIdempotent` (konflik tercatat, tidak ada penimpaan diam-diam) | **Terpenuhi** |
| INT-03 | S11.3.6 | Server palsu `httptest` untuk GitHub Issues dua arah dan Google Calendar satu arah (paginasi, 401/403/429, perubahan di kedua sisi) | **Sebagian**: `TestRealGitHubAccount` dan `TestRealGoogleAccount` ada tetapi **BELUM DIJALANKAN** (tanpa akun uji) |
| NAV-08, NAV-09 | S11.3.7 | `TestBookmarksAndFeedback`, `bookmarks-feedback.spec.ts` | **Terpenuhi** |

## Checklist UAT untuk orang (belum dijalankan)

1. Unggah versi kedua sebuah file, bandingkan berdampingan, beri anotasi pada v1, pastikan anotasi tidak pindah ke v2, lalu minta approval pada v1 dan v2 secara terpisah.
2. Buat widget area dan pivot, tambahkan widget pustaka ke papan, ubah layout, buat tautan publik dengan kedaluwarsa dan password, buka tanpa login, dan pastikan angka uang tidak tampil.
3. Atur jadwal report setiap hari pada jam tertentu di zona waktu Anda dan pastikan email tiba pada jam itu di kotak surat sungguhan.
4. Buat API key bertipe baca, panggil API dengan key itu, coba menulis (harus ditolak), rotasi key, dan cabut; ulangi untuk token OAuth dari Authorized apps.
5. Daftarkan webhook ke penerima sungguhan, verifikasi tanda tangan dan timestamp di sisi penerima, matikan penerima dan pastikan retry lalu nonaktif otomatis.
6. Pasang konektor GitHub Issues pada repo uji dan ubah issue dan task di kedua sisi; selesaikan satu konflik; pasang konektor Google Calendar dan pastikan perubahan di Google tidak menulis balik.
7. Simpan bookmark URL di sebuah Space dan kirim feedback dari menu profil.

## Temuan saat M11

1. Dua sesi paralel memakai nomor migrasi yang sama (`00071`, `00072`): migrasi file dipindah ke `00090`-`00092`. Sampai itu diperbaiki seluruh tes berbasis database gagal ("duplicate migration version").
2. `TestPublicDashboardLinkExpiryPasswordAndFinancialMasking` gagal acak karena widget tanpa posisi dikembalikan dalam urutan id acak; kini tiap widget diberi posisi.
3. Menggabungkan cabang API key dan OAuth membuka celah: token OAuth dengan scope tulis dapat membuat API key. Rute `/api/v1/api-keys` kini ditolak untuk token OAuth (sama seperti `/auth` dan `/oauth`).
4. Secret webhook tersimpan polos; kini terenkripsi dengan `PMS_CONNECTOR_KEY` bila kunci diisi.
5. Dua spec Playwright (feedback, M5) mengukur axe saat dialog masih memudar sehingga kontras terbaca salah; kini menunggu animasi selesai.
6. Halaman Apps & Integrations dibuat oleh tiga sesi secara terpisah; digabung menjadi satu halaman bertab (API, Webhooks, Authorized apps, OAuth clients, Connectors).

## Risiko yang diterima

- Perilaku GitHub dan Google nyata (format respons, batas laju, izin token) hanya diverifikasi terhadap tiruan yang ditulis dari dokumentasi API.
- Rahasia webhook dan konektor memakai satu kunci (`PMS_CONNECTOR_KEY`); kehilangan kunci membuat semuanya tidak terbaca. Rotasi secret webhook belum ada, dan langganan lama yang tersimpan polos tidak dienkripsi ulang.
- Bukti performa file besar dan banyak versi belum diukur.
