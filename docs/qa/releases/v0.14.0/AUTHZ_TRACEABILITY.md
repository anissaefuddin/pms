# Matriks ketertelusuran otorisasi AC-04..AC-13 (v0.14.0)

Matriks ini memetakan tiap AC ke tes yang ada, jalur yang dicakup, dan batasnya. Tidak ada satu fixture konsolidasi. Tes yang tidak dijalankan
di PROMPT #025 ditandai "sebelumnya", dan hasilnya dapat dilihat di `B3_CLOSURE.md` §3.

| AC | Tes (nama fungsi, berkas di `server/internal/httpserver` kecuali disebut) | Jalur | Batas |
|---|---|---|---|
| AC-04 membership tanpa grant tidak memberi konten | `TestDirectSharesRequireCustomerEligibility` (`share_eligibility_integration_test.go`, SH-01..08); `TestExcelImportRespectsCustomerOwnership` (XL-02); `TestFileAccessFollowsCurrentCustomerAccess` (FILE-02, FILE-04) | share Space/item, impor, file | Matriks gabungan belum ada |
| AC-05 Workspace Admin tanpa grant | `TestFileAccessFollowsCurrentCustomerAccess` (FILE-03: admin tanpa membership atau grant = 404); `TestAnalyticsEngineMatchesRawDataAndPolicy` (sebelumnya: admin tanpa grant = 0) | file, analytics | Skenario admin lintas Customer untuk overview dan portfolio belum ada |
| AC-06 assignment semua jalur | `TestAssignmentEnforcement`, `TestItemEligibility`, `TestItemMultiLocationEligibility` (`item_eligibility_integration_test.go`, AS-01..13); `TestAutomationAssignmentIsCheckedAtRunTime` (AU-01, AU-02) | create, patch, automation assign | — |
| AC-07 Customer A tidak mengakses Customer B | `TestCrossClientReadsDoNotDiscloseHiddenItems` (`cross_client_reads_integration_test.go`); `TestFileAccessFollowsCurrentCustomerAccess` (FILE-02) | search, overview, agenda, timeline, workload, todo, shared, file | `endpoint_sweep` (sebelumnya) tidak memakai skenario lintas Customer |
| AC-08 search, portfolio, workload, laporan | `TestCrossClientReadsDoNotDiscloseHiddenItems` (search, overview, workload) | search, overview, workload | Portfolio dan laporan belum ada tes lintas Customer; stars, resource directory, baselines ditinjau dari sumber (lihat `B3_CLOSURE.md` §5) |
| AC-09 lokasi, move, restore, copy, dependensi, relasi | `TestMembershipRevocationTakesEffectAtOnce`, `TestMutationsKeepStableOwnership`, `TestMutationsAreSerializedWithArchiveAndRevoke`, `TestAssigneeEligibilityIsSerializedWithRevoke` (`item_mutation_ownership_integration_test.go`, RV/LOC/CON); `TestDependencyRequiresOneOwner` (DR-01..06); `TestRelationTargetsShareOwner`; `TestBlueprintCopyTakesDestinationOwner` (BP-01..04) | move, restore, dependensi, relasi, blueprint | Move/restore lintas Customer dicakup RV/LOC; tidak ada tes khusus untuk setiap bentuk restore |
| AC-10 share grup, revocasi, konkurensi | `TestDirectSharesRequireCustomerEligibility` (SH-08, grup dalam Customer campuran); `TestInboxHidesItemsAfterAccessEnds`; `TestRealtimeStreamStopsAfterMembershipRevoked`; `TestMutationsAreSerializedWithArchiveAndRevoke` | share, revocasi, realtime | Konkurensi khusus share belum ada |
| AC-11 blueprint, form, import, automation, job, webhook | `TestBlueprintCopyTakesDestinationOwner`; `TestRequestFormDestinationRespectsOwner` (FR-01..05); `TestRequestFormSubmissionAfterArchiveWritesNothing`; `TestExcelImportRespectsCustomerOwnership`; `TestAutomationAssignmentIsCheckedAtRunTime`; `TestQueuedWebhookIsNotSentAfterAccessEnds` | blueprint, form, import, automation, webhook | Job terjadwal dan report subscription belum diuji (lihat `B3_CLOSURE.md` §5) |
| AC-12 lampiran, preview, ekspor, link | `TestFileAccessFollowsCurrentCustomerAccess` (FILE-01..06, FILE-08); `TestGanttSnapshotLinkEndsWithCreatorAccess`; `TestPublicLinksAreOffByDefault`; `TestPydioListingIsOffByDefault` | attachment download, versi, preview, export, Gantt publik, Pydio | FILE-07 (isi export) bergantung pada tes export sebelumnya; tidak ada endpoint thumbnail (sumber: rute tidak terdaftar) |
| AC-13 rekonsiliasi, karantina | `internal/clientscope` (`TestApplyWritesDurableAuditOnlyForRealChanges`, `TestRefusedPlanLeavesDurableRefusalRecord`, `TestFormDestinationOwnerIsAConflict`, `TestApplyPreservesIdentifiersHierarchyAndShares`, `TestRecoveryRehearsalRestoresThenReappliesTheSameMapping`) | migrasi sintetis | Karantina per resource belum ada; data operasional tidak diverifikasi (EXTERNAL_BLOCKED) |

Kontrol negatif yang sudah dijalankan untuk gerbang baru: attachment view check (FILE), PYDIO listing guard, Gantt creator reauth, audit apply (catatan: lihat `B3_CLOSURE.md` §3).

## Tambahan PROMPT #026

| Bagian | Tes | Catatan |
|---|---|---|
| Stars, baselines, resource directory | `TestStarsBaselinesAndDirectoryFollowCurrentAccess` (`stars_baselines_directory_integration_test.go`) | Kontrol negatif: view filter stars dimatikan → FAIL. Directory hanya berisi data direktori pengguna (admin), tanpa judul atau id resource. |
| Automation terjadwal | `TestScheduledAutomationUsesCurrentOwnerAccess` (`scheduled_automation_authority_integration_test.go`) | Pemeriksaan owner di engine tidak sensitif (item policy menutup); tercatat. |
| Report subscription | `TestReportSubscriptionFollowsCustomerAccessAtSendTime` (`report_customer_digest_integration_test.go`) | Subscriber tanpa akses tidak menerima digest; kontrol negatif pada `canRead` → FAIL. |
| Digest yang diantrikan | `TestQueuedReportDigestIsNotSentAfterAccessEnds` (`internal/mail`) | Pengiriman SMTP dihitung; kontrol negatif → 2 pesan. |
| FILE-07 | `TestSpaceExportContainsOnlyTheRequestedCustomersRecords` (`file_export_content_integration_test.go`) | Isi sel workbook dibaca; judul dan id Customer B tidak ada. |
| Konflik automation dan template | `TestAutomationAndTemplateConflictsRefuseAndRecordReasons`; `TestResolvingTheBlockingRecordsReleasesTheMapping` (`internal/clientscope`) | Alasan per resource tersimpan di audit; rilis melalui perbaikan record dan mapping baru. |
