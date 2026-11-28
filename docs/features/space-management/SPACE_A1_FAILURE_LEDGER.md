# Ledger kegagalan Go (A1)

Sumber: `go test ./...` pada database uji terisolasi, sesi ini. Jumlah FAIL: 79.

Kategori: ENV = gerbang lingkungan (variabel `PMS_*` tidak diset; BLOCKED, bukan defek). PERF = ambang waktu lokal. REAL = kegagalan nyata yang belum ditelusuri.

| Tes | Kategori | Pesan pertama |
|---|---|---|
| TestAnalyticsEngineMatchesRawDataAndPolicy | ENV |     analytics_perf_integration_test.go:19: set PMS_PERF=1 to measure widgets and reports on 10,000 items |
| TestAnalyticsMatchesRawDataOnVariedFixtures | REAL |     attachment_compare_integration_test.go:48: upload a.png: 404 map[code:not_found detail:not found request_id:pv96KygDoCPJNRh- status:404 title:Not Found type:about:blank] |
| TestApprovalOutcomeTriggersOnlyTheMatchingRule | REAL |     automation_a4_integration_test.go:42: item Discuss: status 404, want 201 (map[code:not_found detail:not found request_id:xuSaLfO8jNcP3w7H status:404 title:Not Found type:about:blank]) |
| TestArchiveRestoreAndTheTrail | REAL |     automation_mention_group_integration_test.go:21: item Needs review: status 404, want 201 (map[code:not_found detail:not found request_id:ZQIVhRo6c7aD7Qt3 status:404 title:Not Found type:about:blan |
| TestAttachmentVersionsAreImmutableAndPolicyChecked | REAL |     automation_a4_integration_test.go:24: item Sign me off: status 404, want 201 (map[code:not_found detail:not found request_id:2gXZWuiZ3LUgB3Xq status:404 title:Not Found type:about:blank]) |
| TestAttachmentsOnExternalStorageAndLinks | REAL |     attachment_versions_integration_test.go:53: v1: 404 map[code:not_found detail:not found request_id:ssTmcmpYv8iCODhB status:404 title:Not Found type:about:blank] |
| TestAutomationChangesAreAttributedToTheBot | REAL |     automation_b_integration_test.go:16: item Quiet: status 404, want 201 (map[code:not_found detail:not found request_id:hIaj_Ns_AibECBj- status:404 title:Not Found type:about:blank]) |
| TestAutomationKeepsApprovalsWorkflowsAndRoles | REAL |     automation_integration_test.go:280: an admin may switch it off: status 404, want 200 (map[code:not_found detail:not found request_id:blZvR4PsSzjfEypd status:404 title:Not Found type:about:blank]) |
| TestBlueprintSaveAndUse | ENV |     collaboration_integration_test.go:22: set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture |
| TestCancelledCreateReleasesParentLock | REAL |     custom_field_access_regression_integration_test.go:16: field A: status 404, want 201 (map[code:not_found detail:not found request_id:5RpS3NCqfLeyAfjj status:404 title:Not Found type:about:blank]) |
| TestCollaborationAccessAndAttachmentLifecycle | REAL |     create_lock_integration_test.go:29: folder P: status 404, want 201 (map[code:not_found detail:not found request_id:wtuvV-B-DDtphr5w status:404 title:Not Found type:about:blank]) |
| TestCommentTriggersARuleOnceWithoutLooping | REAL |     automation_a4_integration_test.go:57: item Late: status 404, want 201 (map[code:not_found detail:not found request_id:jVyEGufoUn-SuFuC status:404 title:Not Found type:about:blank]) |
| TestCompareTwoVersionsOfOneFile | REAL |     attachment_pdf_skipped_integration_test.go:40: upload: 404 map[code:not_found detail:not found request_id:DoAXIZ98g2H0Ujbe status:404 title:Not Found type:about:blank] |
| TestConcurrentCreateReorderAndMoveShareOneParent | REAL |     create_lock_integration_test.go:29: folder A: status 404, want 201 (map[code:not_found detail:not found request_id:raEmk_q5Efll2RVJ status:404 title:Not Found type:about:blank]) |
| TestConcurrentPublicFormSubmissionsAllSucceedOnce | REAL |     forms_public_integration_test.go:267: build form: status 404, want 201 (map[code:not_found detail:not found request_id:C6KHKSTSeSXP_2jr status:404 title:Not Found type:about:blank]) |
| TestCreateItemUnderTheTriggeringItem | REAL |     automation_attribution_integration_test.go:13: item Attributed: status 404, want 201 (map[code:not_found detail:not found request_id:U35kTPcEoMD-bV_P status:404 title:Not Found type:about:blank]) |
| TestCustomFieldAccessRegression | REAL |     custom_field_item_share_integration_test.go:40: relation field: status 404, want 201 (map[code:not_found detail:not found request_id:mJu-cpeFlFtDoWsL status:404 title:Not Found type:about:blank]) |
| TestCustomFieldPeopleDirectoryAndLocation | ENV |     custom_fields_integration_test.go:13: set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture |
| TestDashboardLifecycle | REAL |     endpoint_sweep_integration_test.go:268: 139 operations exercised with real ids; 54 have no real id in the fixture: |
| TestDuplicateCopiesTheDefinitionAsADisabledRule | REAL |     automation_settings_integration_test.go:41: item Busy: status 404, want 201 (map[code:not_found detail:not found request_id:sd59D_kF4a9x325L status:404 title:Not Found type:about:blank]) |
| TestEmptyRecycleBinPurgesOnlyWhatActorMayDelete | REAL |     relation_fields_integration_test.go:22: relation field: status 404, want 201 (map[code:not_found detail:not found request_id:vdBVkrWmTC3c6DMU status:404 title:Not Found type:about:blank]) |
| TestEventReplayRespectsCurrentAccess | ENV |     events_integration_test.go:106: set PMS_TEST_SSE_HEARTBEAT=1 to check the 25-second interval |
| TestFileApprovalPinnedToVersionWithExternalReviewer | REAL |     form_blueprint_routing_integration_test.go:19: root Bug triage: status 404, want 201 (map[code:not_found detail:not found request_id:aMO-pqUsyHPNiJ-y status:404 title:Not Found type:about:blank]) |
| TestFormAccessAndManageList | REAL |     forms_public_integration_test.go:74: create link: status 404, want 200 (map[code:not_found detail:not found request_id:pRKl2lmAhvc44VRJ status:404 title:Not Found type:about:blank]) |
| TestFormBranchesDecideQuestionsItemTypeAndFields | REAL |     form_submission_scope_integration_test.go:76: create public link: status 404, want 200 (map[code:not_found detail:not found request_id:WsRuBDE6Hji-UirZ status:404 title:Not Found type:about:blank] |
| TestFormBranchingBlueprintItem | REAL |     form_routing_integration_test.go:27: severity: status 404, want 201 (map[code:not_found detail:not found request_id:ImvIQQJUQNYUg56e status:404 title:Not Found type:about:blank]) |
| TestFormsDefinitionAndSettings | REAL |     forms_integration_test.go:131: form in another Space: status 404, want 201 (map[code:not_found detail:not found request_id:fxouiFWLTgWDoX7u status:404 title:Not Found type:about:blank]) |
| TestFormulaFieldsAreValidatedRecalculatedAndProtected | PERF |     formula_fields_integration_test.go:128: formula over 500 items: 34.878333ms |
| TestInactiveItemFiresOncePerQuietSpell | REAL |     automation_b_integration_test.go:47: item Review: status 404, want 201 (map[code:not_found detail:not found request_id:PuiugGXiT0CeokNu status:404 title:Not Found type:about:blank]) |
| TestInvalidRuleWhenADependencyIsGone | REAL |     automation_lifecycle_integration_test.go:91: type: status 404, want 201 (map[code:not_found detail:not found request_id:qQafv5E8o-Tvb52m status:404 title:Not Found type:about:blank]) |
| TestItemLifecycle | ENV |     m10_browser_integration_test.go:11: set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture |
| TestItemTypeDefaultsApplyOnEveryCreatePath | REAL |     item_type_defaults_integration_test.go:41: assign Workspace field to the Space: status 404, want 201 (map[code:not_found detail:not found request_id:unTdeXSgWvbTsoii status:404 title:Not Found typ |
| TestItemTypeFieldDefaultsAdminAPIAndOrphans | REAL |     item_type_defaults_integration_test.go:41: assign Workspace field to the Space: status 404, want 201 (map[code:not_found detail:not found request_id:QXqaH_FcSOWYuPxV status:404 title:Not Found typ |
| TestItemTypeInExportSearchAndReports | REAL |     item_type_scope_integration_test.go:56: an item in another Space cannot use the Space type: status 404, want 422 (map[code:not_found detail:not found request_id:a_JCouyCbr7qBHnb status:404 title:N |
| TestItemTypeScope | REAL |     items_integration_test.go:87: create: 404 map[code:not_found detail:not found request_id:7CdzNpPO3kirBtHV status:404 title:Not Found type:about:blank] |
| TestListFilterAndSortByCustomField | PERF |     item_field_list_integration_test.go:104: 500 items, 10 number fields, sort+filter by field: p50=45.809375ms p95=132.56625ms max=139.8735ms (one page of 200) |
| TestMentionIsRefusedWhenItWouldReachTooManyPeople | REAL |     automation_tryit_integration_test.go:23: item Try me: status 404, want 201 (map[code:not_found detail:not found request_id:a9-YU1Y5WAkwolaI status:404 title:Not Found type:about:blank]) |
| TestMentionNotifiesOnlyPeopleWhoCanSeeTheItem | REAL |     automation_b_integration_test.go:75: item Mover: status 404, want 201 (map[code:not_found detail:not found request_id:ugYT68lzScZHeJCd status:404 title:Not Found type:about:blank]) |
| TestMentionReachesFollowersAndGroupsAtRunTime | REAL |     automation_registry_integration_test.go:69: duplicate: status 404, want 201 (map[code:not_found detail:not found request_id:mJuFImWMbPlKdC4W status:404 title:Not Found type:about:blank]) |
| TestMoveItemIntoAProject | REAL |     automation_guards_integration_test.go:32: item Needs sign-off: status 404, want 201 (map[code:not_found detail:not found request_id:2vsJUoaoYM5H6rIv status:404 title:Not Found type:about:blank]) |
| TestMultiLocationCreatesDoNotDeadlock | REAL |     create_lock_integration_test.go:29: folder P: status 404, want 201 (map[code:not_found detail:not found request_id:mwA720U_ZzB1QkKP status:404 title:Not Found type:about:blank]) |
| TestMyTodoOnlyReturnsVisibleAssignedTasks | ENV |     twofactor_browser_integration_test.go:12: set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture |
| TestNotificationReplayChecksCurrentAccess | ENV |     pilot_performance_integration_test.go:10: set PMS_TEST_PILOT_PERFORMANCE=1 to measure the real browser against a 10,000-Item database |
| TestNotificationWorkerHonoursEmailPreferences | REAL | (tanpa pesan) |
| TestOverdueRuleFiresOncePerItem | REAL |     automation_a4_integration_test.go:76: item Parent: status 404, want 201 (map[code:not_found detail:not found request_id:-KQt0-oQCl8YEwWO status:404 title:Not Found type:about:blank]) |
| TestOwnerTransferIsAdminOnlyAndRevalidates | REAL |     automation_lifecycle_integration_test.go:146: item Before archive: status 404, want 201 (map[code:not_found detail:not found request_id:__UKNIuCbqPyXFFJ status:404 title:Not Found type:about:blank |
| TestPDFPreviewWithoutRendererIsSkippedAndPageIs404 | REAL |     attachment_proofing_integration_test.go:54: upload logo.png: 404 map[code:not_found detail:not found request_id:wanBkuf4fpwzRGwi status:404 title:Not Found type:about:blank] |
| TestPausedRuleResumesOnlyAfterRevalidation | REAL |     automation_lifecycle_integration_test.go:53: item Archived field: status 404, want 201 (map[code:not_found detail:not found request_id:qt5j01TLC-5pk4MR status:404 title:Not Found type:about:blank] |
| TestProjectFiles | ENV |     project_progress_integration_test.go:12: set PMS_TEST_BROWSER=1 to run the browser against the PostgreSQL fixture |
| TestProjectProgressFollowsAuthorizedLiveLocations | PERF |     qa_b3a_search_test.go:114: 500 tasks, scoped to project, limit 50 with ancestor paths: p50=11.516625ms p95=13.042958ms max=13.119292ms |
| TestProjectTimelog | ENV |     time_entries_integration_test.go:127: set PMS_TEST_BROWSER=1 for Chrome with PostgreSQL |
| TestProofingAnnotationsStayOnTheirVersion | REAL |     attachment_storage_integration_test.go:82: upload: 404 map[code:not_found detail:not found request_id:N6fX4fDD5JY5opss status:404 title:Not Found type:about:blank] objects=0 |
| TestPublicConfirmationUsesTheFormsWording | REAL |     formula_fields_integration_test.go:22: field Price: status 404, want 201 (map[code:not_found detail:not found request_id:PHv08JfH0M6IzvRZ status:404 title:Not Found type:about:blank]) |
| TestPublicFormLink | REAL |     forms_public_integration_test.go:222: build form: status 404, want 201 (map[code:not_found detail:not found request_id:Z8RLX6YKStpGnqn9 status:404 title:Not Found type:about:blank]) |
| TestPublicSubmissionCannotChooseAnotherDestination | REAL |     forms_integration_test.go:84: create: 422 map[code:validation_failed detail:approvers must be able to see the destination errors:[map[field:response.approval.approver_ids message:approvers must be |
| TestRecycleRestoresCascadeAndMissingParent | REAL |     recycle_purge_integration_test.go:42: delete 2a6d8d8b-b1d9-45da-95ea-ebd4e7219d2b: {status:404 body:map[code:not_found detail:not found request_id:nCVGkI5esUMwsEY4 status:404 title:Not Found type: |
| TestRefusedActionStopsTheRule | REAL |     automation_lifecycle_integration_test.go:127: the new owner must be able to run rules here: status 404, want 422 (map[code:not_found detail:not found request_id:libF1xR6NDvMjvAf status:404 title:N |
| TestRelationFieldRejectsHiddenTarget | REAL |     custom_field_types_integration_test.go:123: create People field: status 404, want 201 (map[code:not_found detail:not found request_id:aoVH_VgLAsxM6OrW status:404 title:Not Found type:about:blank]) |
| TestRelationFieldsValidateTargetsAndDoNotLeakNames | REAL |     report_digest_integration_test.go:99: the admin sees every Space: |
| TestReplaceItemLocations | ENV |     item_rank_scale_integration_test.go:15: set PMS_TEST_SCALE=1 to run the 500/1000/2000-children scale test |
| TestReportResultAndExport | ENV |     scheduling_integration_test.go:433: set PMS_TEST_SCHEDULING_PERFORMANCE=1 to measure p95 scheduling latency against PostgreSQL |
| TestReportSubscriptionsAreDeliveredOncePerSlotWithTheSubscribersRights | REAL |     reports_integration_test.go:204: admin result = map[layout:column name:Counts result:map[rows:[map[break:<nil> group:map[key:active label:active] value:2]] total:2]] |
| TestRuleIsInvalidWhileItsItemTypeIsInactive | REAL |     automation_lifecycle_integration_test.go:16: item Lost access: status 404, want 201 (map[code:not_found detail:not found request_id:_f8821oVr5hJL9Qt status:404 title:Not Found type:about:blank]) |
| TestRuleLifecycleVersionsAndValidation | REAL |     automation_integration_test.go:441: item Before: status 404, want 201 (map[code:not_found detail:not found request_id:12RoBub2Ke5rOoIx status:404 title:Not Found type:about:blank]) |
| TestRulesRunWithTheirOwnersRightsAndCurrentStatus | REAL |     automation_integration_test.go:569: item Parked: status 404, want 201 (map[code:not_found detail:not found request_id:xxQLt7JmWlAc0nex status:404 title:Not Found type:about:blank]) |
| TestRunLogDeadEventsAndReplay | REAL |     automation_itemtype_integration_test.go:14: Space item type: status 404, want 201 (map[code:not_found detail:not found request_id:-nsxPKI9JanGoCIw status:404 title:Not Found type:about:blank]) |
| TestSSELatencyUnderClientLoad | REAL |     sse_multiuser_integration_test.go:144: patch: {status:404 body:map[code:not_found detail:not found request_id:1RmeJl9nN3mb5q3P status:404 title:Not Found type:about:blank] header:map[Content-Lengt |
| TestSSETwoUsersLatencyOrderReconnectAndResync | REAL |     time_entries_integration_test.go:70: create project view: status 404, want 201 (map[code:not_found detail:not found request_id:Lb8DkLep10eKWayw status:404 title:Not Found type:about:blank]) |
| TestSearchPrefixAndScopesRespectVisibility | REAL |     search_integration_test.go:112: 10,000 items and 500 comments, 24 HTTP queries after 4 warmups: p95=25.366541ms max=25.591416ms |
| TestSpaceAccessRules | REAL |     sse_load_integration_test.go:79: patch: status 404, want 200 (map[code:not_found detail:not found request_id:LoXwqQHz9-KGF61e status:404 title:Not Found type:about:blank]) |
| TestStarsAndStarredWidget | REAL |     dashboards_integration_test.go:94: move dashboard between Spaces: status 404, want 200 (map[code:not_found detail:dashboard, widget, or Space not found request_id:tgcU-OMfVo2m3MCI status:404 title |
| TestSubmissionAuditNamesTheActor | REAL |     form_submission_scope_integration_test.go:90: create public link: status 404, want 200 (map[code:not_found detail:not found request_id:pysjNHp1d_muoahA status:404 title:Not Found type:about:blank] |
| TestTryItOnAnExistingRuleRunsAsItsOwner | REAL |     automation_workspace_integration_test.go:107: admin creates the item in the first Space: status 404, want 201 (map[code:not_found detail:not found request_id:WGW5oAA7DCPbKF8_ status:404 title:Not  |
| TestTryItRefusesWhatARunWouldRefuseAndWritesNothing | REAL |     automation_tryit_integration_test.go:99: item Owner check: status 404, want 201 (map[code:not_found detail:not found request_id:wQYhLejAaTmpypqs status:404 title:Not Found type:about:blank]) |
| TestWebhookRetryThenAutoDisable | REAL |     webhooks_integration_test.go:216: item Encrypted secret plan: status 404, want 201 (map[code:not_found detail:not found request_id:g9dbT6tBVQYMWCkN status:404 title:Not Found type:about:blank]) |
| TestWebhookSecretIsEncryptedAtRest | ENV |     workload_integration_test.go:96: set PMS_PERF=1 for the workload benchmark |
| TestWebhookSignedDeliveryAndVisibility | REAL |     webhooks_integration_test.go:141: item One: status 404, want 201 (map[code:not_found detail:not found request_id:nWcip43A-LV98wzJ status:404 title:Not Found type:about:blank]) |
| TestWidgetDataPresetsAndCustomQueries | REAL |     dashboards_data_integration_test.go:182: admin stars the secret task: status 404, want 204 (map[code:not_found detail:not found request_id:EMjdnef5IqYDWSYF status:404 title:Not Found type:about:bl |
| TestWorkspaceRuleRefusesOnlyWhereTheOwnerCannotAct | REAL |     blueprints_integration_test.go:16: create root: status 404, want 201 (map[code:not_found detail:not found request_id:VYamot9CjimF61pD status:404 title:Not Found type:about:blank]) |

## Tujuh tes investigasi (A1.4)

Sudah diverifikasi lulus satu per satu pada database terisolasi:
- TestConcurrentDependenciesCannotCreateCycle: akar masalah fixture (admin perencana tanpa grant ke Space Dependencies). Grant eksplisit ditambahkan. Bukan defek.
- TestGitHubErrorsAreIsolatedAndBackedOff, TestGoogleOneWayLifecycle, TestGoogleErrorsRefreshAndRateLimit: akar masalah fixture (installer connector bukan anggota Space Repo). Grant Full untuk installer. Bukan defek.
- TestProjectFilesReviewStatusIsPerVersion: akar masalah fixture (aktor tanpa akses ke Space). Grant eksplisit.
- TestRT29UnassignedBucketLabel: akar masalah fixture (admin membaca tugas tanpa grant). Grant eksplisit.
- TestResourceWorkload: akar masalah fixture (perencana membaca workload dengan tugas). Grant Full pada dua Space.

Belum ada defek nyata yang terbukti dari perubahan A1 pada tujuh tes ini.


## Pembaruan putaran terbaru
- Hitungan FAIL Go penuh: 77 (11 gerbang lingkungan, sisanya nyata). PASS: 382.
- Tes kontrak yang sudah PASS: TestAnalyticsEngineMatchesRawDataAndPolicy, TestAnalyticsMatchesRawDataOnVariedFixtures, TestReportActualCostIsAdminOnly, TestReportTemplatesProduceTheRightNumbers, TestGoogleOnlyShowsWhatTheCreatorCanSee, TestUtilizationPlannedVersusActualMatchesRawData, TestReportsLifecycleAndAccess.
- OPEN: TestNotificationWorkerHonoursEmailPreferences (email tidak terkirim walau penerima berhak), TestSearchPrefixAndScopesRespectVisibility (hasil pencarian space_id berbeda).
- Daftar per tes yang belum diperbarui di tabel di atas; baris untuk tes yang sudah PASS perlu dianggap usang.


## Status akhir putaran (lihat SPACE_MANAGEMENT_ANALISA.md bagian 16)
FAIL 39 (10 env, 29 nyata); 9/9 kontrak PASS.
