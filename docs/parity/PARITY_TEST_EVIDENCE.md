# Bukti uji per WFP (S12.2.2, butir DOC)

**Dibuat:** 14 Nov 2026 pada basis `integration/m12-regression`. **Aturan:** kolom *bukti UAT manusia* tetap `NOT RUN` sampai penguji mengisinya di [UAT.md](../UAT.md#uat-parity-final-s1222). Status akhir `ACCEPTED` hanya bila bukti otomatis **dan** UAT manusia lulus. Dokumen ini tidak menaikkan status apa pun.

- Daftar WFP dan kelompok: [baseline §4](../PMS_WRIKE_PARITY_DEVELOPMENT_BASELINE.md); ID PRD dari [WFP_TO_PRD_MATRIX.md](WFP_TO_PRD_MATRIX.md); story dari [TRACEABILITY_M7_M12.md](../TRACEABILITY_M7_M12.md).
- Status baseline (`GAP`, `PAR`, `VERIFY`) adalah foto sebelum M8–M12 dan tidak dipakai lagi. Status di sini dihitung dari keadaan sekarang: `PENDING_UAT` = fitur ada dan punya bukti otomatis, menunggu UAT manusia; `N/A_APPROVED` = di luar 1.0 (M13, menunggu persetujuan pemilik produk lewat DEC-PAR-01/05/06; WFP-044 ditandai M13 oleh matriks dan perlu dikonfirmasi); `GAP` = tidak ada fitur atau tidak ada bukti otomatis (saat ini **tidak ada baris**).
- Penulisan bukti: awalan `go:` + nama fungsi (`func Test...`) di `server/`; awalan `pw:` + nama berkas, spec di `web/tests/`. Keberadaannya diperiksa oleh [../../scripts/parity-evidence-check.sh](../../scripts/parity-evidence-check.sh). Skrip hanya membuktikan nama tes **ada**, bukan bahwa tes itu lulus; hasil lulus dicatat di TESTING.md.
- Batas: pemetaan WFP → tes adalah penilaian penyusun, bukan hasil pelacakan otomatis; sebagian tes membuktikan beberapa WFP sekaligus.

## Ringkasan (dihitung dari tabel)

| Status | Jumlah WFP |
|---|---|
| `PENDING_UAT` | 93 |
| `N/A_APPROVED` | 13 |
| Total | 106 |

## Tabel

| WFP | Kelompok | Requirement PRD | Story | Bukti otomatis | Hasil otomatis terakhir | Bukti UAT manusia | Status akhir yang diusulkan |
|---|---|---|---|---|---|---|---|
| WFP-001 | Work management | SPC-01, SPC-02 | M0–M6 | `go:TestSpaceAccessRules`<br>`go:TestAuthorizationMatrix`<br>`pw:spaces.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-002 | Work management | SPC-04, SPC-05, SPC-06 | M0–M6 | `go:TestItemLifecycle`<br>`pw:items.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-003 | Work management | TASK-01, TASK-02, TASK-06 | M0–M6 | `go:TestItemLifecycle`<br>`go:TestItemAuthorizationMatrix`<br>`pw:items.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-004 | Work management | SPC-10 | M2 S2.5.x | `go:TestReplaceItemLocations`<br>`pw:item-locations.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-005 | Work management | SET-07 | S2.1.1 | `go:TestWorkSchedule`<br>`pw:work-schedule.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-006 | Work management | TASK-09 | S2.2.2 | `go:TestProjectProgressMethodsAndRisk`<br>`go:TestCalculateProjectProgress`<br>`pw:project-progress.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-007 | Work management | SPC-13 | M3 | `go:TestBlueprintSaveAndUse`<br>`pw:blueprints.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-008 | Work management | SPC-18 | M3 | `go:TestSpaceTemplatesBuildWholeSpaceOnceAndOnlyForCreators`<br>`go:TestCatalogIsConsistent` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-009 | Work management | CUST-01 | S8.1.1, S8.1.5 | `go:TestItemTypesCRUDDefaultsAndChildRules`<br>`pw:m8.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-010 | Work management | CUST-04 | S8.1.4 | `go:TestBrowserM8Flows`<br>`pw:m8.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-011 | Work management | TASK-10, TASK-11A | S2.3.1, S2.3.2 | `go:TestCustomFieldLifecycleAndItemValues`<br>`go:TestCustomFieldTypesAndConfig`<br>`pw:custom-fields.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-012 | Work management | TASK-11B | S8.2.1 | `go:TestFormulaFieldsAreValidatedRecalculatedAndProtected`<br>`go:TestFormulaEvaluation` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-013 | Work management | NAV-06, NAV-07 | M1 | `go:TestSearchPrefixAndScopesRespectVisibility`<br>`go:TestSearchExtraFilters`<br>`pw:search.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-014 | Work management | NAV-01, NAV-02 | M1, M3 | `go:TestStarsAndStarredWidget`<br>`go:TestSidebarPreferencePersistsAndValidates`<br>`pw:stars.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-015 | Work management | SPC-11, USR-01 | M1 | `go:TestSharesAreUnionedAndCappedAndValidated`<br>`go:TestItemSharesVersionedReplace`<br>`pw:share.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-016 | Planning & views | VIEW-01 | M1 | `go:TestItemLists`<br>`pw:views.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-017 | Planning & views | VIEW-02 | M1 | `go:TestBrowserM2Flows`<br>`pw:views.spec.ts`<br>`pw:board-lanes.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-018 | Planning & views | VIEW-05A | S2.4.2 | `go:TestBrowserM2Flows`<br>`pw:gantt.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-019 | Planning & views | VIEW-05B | S2.1.2, S2.4.3 | `go:TestDependencyLifecycle`<br>`go:TestShiftFinishToStart`<br>`pw:gantt.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-020 | Planning & views | VIEW-05B | S2.1.3 | `go:TestSchedulingChain`<br>`go:TestPropagateDiamondAndCycle` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-021 | Planning & views | VIEW-05C | S10.3.1 | `go:TestBaselinesFreezeDatesAndCannotBeChanged` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-022 | Planning & views | SCH-01 | S10.3.3 | `go:TestPortfolioTimelineCrossProjectDependencyAndPolicy`<br>`pw:portfolio.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-023 | Planning & views | SCH-01 | S10.3.3 | `go:TestCrossProjectDependenciesRespectPermissions` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-024 | Planning & views | VIEW-06 | S2.4.3 | `go:TestBrowserM2Flows`<br>`pw:calendar.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-025 | Planning & views | VIEW-07 | M4 | `go:TestProjectTimelog` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-026 | Planning & views | VIEW-10 | S11.1.1 | `go:TestProjectFiles`<br>`pw:project-files.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-027 | Planning & views | VIEW-10 | S11.1.1 | `go:TestProjectFiles`<br>`pw:project-files.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-028 | Planning & views | VIEW-08 | M1, S2.5.x | `go:TestViewLifecycle`<br>`pw:view-presets.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-029 | Planning & views | VIEW-03, VIEW-04 | S2.5.x | `go:TestItemCardFields`<br>`pw:board-lanes.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-030 | Planning & views | VIEW-01, NAV-06 | M1 | `go:TestListFilterAndSortByCustomField`<br>`pw:view-fields.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026); pencarian/filter khusus Table hanya sebagian (baseline: VERIFY) | NOT RUN | PENDING_UAT |
| WFP-031 | Workflow & intake | WF-01, WF-02, WF-03A | M1 | `go:TestWorkflowEditor`<br>`pw:workflows.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-032 | Workflow & intake | WF-03B, WF-04, WF-05 | S8.2.2, S8.2.3 | `go:TestStatusGuardsRequireFieldsAndActorAndWriteNothingOnRefusal` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-033 | Workflow & intake | WF-03B, SPC-12 | S8.2.4 | `go:TestEnteringAStatusStartsAnApprovalWhoseResultMovesTheItem`<br>`pw:approvals.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-034 | Workflow & intake | APR-01 | S9.2.1 | `go:TestApprovalPoliciesReassignAndDeadlines`<br>`pw:approvals.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-035 | Workflow & intake | APR-02, APR-03 | S9.2.2, S9.2.3 | `go:TestApprovalHistoryIsAppendOnlyAndPinnedToItemVersion`<br>`go:TestApprovals` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-036 | Workflow & intake | FORM-01, FORM-02 | M3 | `go:TestFormsDefinitionAndSettings`<br>`pw:forms.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-037 | Workflow & intake | FORM-03, FORM-05, FORM-06 | M3 | `go:TestPublicFormLink`<br>`go:TestFormAccessAndManageList`<br>`go:TestFormSubmission`<br>`pw:public-form.spec.ts`<br>`pw:manage-forms.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-038 | Workflow & intake | FORM-07 | S8.3.1 | `go:TestFormBranchesDecideQuestionsItemTypeAndFields`<br>`go:TestConditionalAnswers` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-039 | Workflow & intake | FORM-09, SPC-13 | S8.3.2 | `go:TestFormBranchesDecideQuestionsItemTypeAndFields`<br>`go:TestBlueprintSaveAndUse` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-040 | Workflow & intake | FORM-08 | M3 | `go:TestMyRequests`<br>`pw:my-requests.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-041 | Workflow & intake | FORM-04 | S8.3.3 | `go:TestConfirmationEmailRules`<br>`go:TestPublicConfirmationUsesTheFormsWording`<br>`go:TestBrandedConfirmationEscapesEverythingAndKeepsTheInstanceAddress` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-042 | Workflow & intake | AUT-01, AUT-02, AUT-05 | S9.1.1, S9.1.2, S9.1.5 | `go:TestTriggersConditionsAndActions`<br>`go:TestRuleEffectIsExactlyOnceOnRedelivery`<br>`pw:automation.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-043 | Workflow & intake | AUT-03, AUT-04 | S9.1.3, S9.1.4 | `go:TestRunLogDeadEventsAndReplay`<br>`go:TestQuotasAndParallelRules`<br>`go:TestRulesCannotLoopForever` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-044 | Workflow & intake | SPC-18, SPC-13 (bundle: M13) | — | — | — | — | N/A_APPROVED |
| WFP-045 | Resources | TASK-05B | S2.2.1 | `go:TestItemEffortAndAllocation`<br>`pw:item-effort.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-046 | Resources | RSC-02 | S10.1.2 | `go:TestResourceWorkload`<br>`go:TestWorkloadPerformance` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-047 | Resources | RSC-02 | S10.1.3 | `go:TestResourceWorkload`<br>`go:TestBrowserWorkload` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-048 | Resources | SET-07, RSC-01 | S2.1.1, S10.1.1 | `go:TestWorkSchedule`<br>`go:TestCalendar` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-049 | Resources | RSC-03 | S10.1.4 | `go:TestBrowserWorkload` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-050 | Resources | RSC-04 | S10.2.1 | `go:TestResourceWorkload` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-051 | Resources | RSC-05 | S10.2.2 | `go:TestResourceWorkload`<br>`go:TestBrowserM10Flows` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-052 | Resources | RSC-06 | S10.2.3 | `go:TestUtilizationPlannedVersusActualMatchesRawData` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-053 | Time & finance | TASK-13 | S4.1.1 | `go:TestTimeEntryAuthorization`<br>`go:TestBrowserM4Flows` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-054 | Time & finance | TASK-13 | S4.1.1 | `go:TestBrowserM4Flows`<br>`pw:timesheets.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-055 | Time & finance | VIEW-07, TASK-13 | S4.1.1 | `go:TestProjectTimelog` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-056 | Time & finance | TASK-12 | M4 | `go:TestProjectBudget` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-057 | Time & finance | TASK-14 | S10.4.1 | `go:TestRatesSnapshotKeepHistoryAndCostIsAdminOnly`<br>`pw:rates.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-058 | Time & finance | TASK-14 | S10.4.1 | `go:TestRatesSnapshotKeepHistoryAndCostIsAdminOnly`<br>`pw:rates.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-059 | Time & finance | TASK-14 | S10.4.2 | `go:TestRatesSnapshotKeepHistoryAndCostIsAdminOnly` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-060 | Collaboration | TASK-07 | M1 | `go:TestCommentLifecycle`<br>`go:TestBrowserCollaboration`<br>`pw:collaboration-live.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-061 | Collaboration | NAV-03, NAV-04 | M1, M3 | `go:TestNotificationWorkflow`<br>`go:TestStreamFiltersAndVisibility` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-062 | Collaboration | TASK-08A, SEC-01 | M1 | `go:TestCollaborationAccessAndAttachmentLifecycle` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-063 | Collaboration | VIEW-10, TASK-08A | S11.1.1, S11.1.3 | `go:TestProjectFiles`<br>`pw:project-files.spec.ts`<br>`pw:file-proofing.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-064 | Collaboration | FIL-01 | S11.1.2 | `go:TestAttachmentVersionsAreImmutableAndPolicyChecked`<br>`pw:file-versions.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-065 | Collaboration | FIL-02 | S11.1.3 | `go:TestProofingAnnotationsStayOnTheirVersion`<br>`pw:file-proofing.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-066 | Collaboration | FIL-03 | S11.1.4 | `go:TestCompareTwoVersionsOfOneFile`<br>`pw:file-compare.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-067 | Collaboration | FIL-04 | S11.1.5 | `go:TestFileApprovalPinnedToVersionWithExternalReviewer`<br>`pw:file-review.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-068 | Collaboration | TASK-08B | S11.1.6 | `go:TestAttachmentsOnExternalStorageAndLinks`<br>`go:TestS3AdapterAgainstFakeServer`<br>`go:TestPydioAdapterAgainstFakeServer` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-069 | Collaboration | N/A (editor dokumen, M13) | — | — | — | — | N/A_APPROVED |
| WFP-070 | Reporting | DASH-01..03A | M5 | `go:TestDashboardLifecycle`<br>`go:TestWidgetDataPresetsAndCustomQueries`<br>`pw:dashboards.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-071 | Reporting | RPT-01..03 | M5 | `go:TestReportsLifecycleAndAccess`<br>`go:TestReportResultAndExport`<br>`go:TestReportTemplatesProduceTheRightNumbers`<br>`pw:reports.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-072 | Reporting | RPT-04 | S11.2.5 | `go:TestReportScheduleDailyAndChosenTimeZone`<br>`go:TestReportSubscriptionsAreDeliveredOncePerSlotWithTheSubscribersRights` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-073 | Reporting | DASH-03B | S11.2.1 | `go:TestAnalyticsWidgetsAreaPivotBurndownCompletion`<br>`pw:dashboards.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-074 | Reporting | DASH-08 | S11.2.2 | `go:TestAnalyticsWidgetsAreaPivotBurndownCompletion`<br>`go:TestBrowserM11AnalyticsFlows`<br>`pw:m11-analytics-live.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-075 | Reporting | DASH-07 | S11.2.4 | `go:TestPublicDashboardLinkExpiryPasswordAndFinancialMasking`<br>`pw:dashboard-public.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-076 | Reporting | DASH-08, API-01 | S11.2.2, S11.3.1 | `go:TestAnalyticsWidgetsAreaPivotBurndownCompletion`<br>`go:TestAPIKeysScopeRotateRevokeAndOwnerLimits` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-077 | Reporting | RPT-02 | S5.2.2 | `go:TestReportResultAndExport`<br>`go:TestAnalyticsMatchesRawDataOnVariedFixtures` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-078 | Integration | SPC-15 | M4 | `go:TestProjectExcelExport`<br>`go:TestProjectExcelImport`<br>`go:TestBrowserExcelRoundTrip`<br>`pw:project-excel.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-079 | Integration | API-01, API-03 | S11.3.1, S11.3.3 | `go:TestAPIKeysScopeRotateRevokeAndOwnerLimits`<br>`go:TestOAuthCodePKCEAndRevocation`<br>`pw:api-keys.spec.ts`<br>`pw:oauth.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-080 | Integration | API-02 | S11.3.2 | `go:TestWebhookSignedDeliveryAndVisibility`<br>`go:TestWebhookRetryThenAutoDisable`<br>`go:TestWebhookRejectsInternalTargets`<br>`pw:webhooks.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-081 | Integration | INT-01, INT-03 | S11.3.4, S11.3.6 | `go:TestConnectorRoutes`<br>`go:TestLifecycleAndSecrets`<br>`go:TestGitHubTwoWayStatusAndBody`<br>`go:TestGoogleOneWayLifecycle`<br>`pw:connectors.spec.ts`<br>`pw:connectors-pilot.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026); akun GitHub/Google sungguhan BELUM dijalankan | NOT RUN | PENDING_UAT |
| WFP-082 | Integration | INT-02, INT-03 | S11.3.5, S11.3.6 | `go:TestSyncBothWaysNoSilentOverwriteAndIdempotent`<br>`go:TestGitHubConflictNeverOverwritesSilently` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026); akun GitHub/Google sungguhan BELUM dijalankan | NOT RUN | PENDING_UAT |
| WFP-083 | Integration | INT-01, AUT-02 | S11.3.4, S9.1.2 | `go:TestConnectorRoutes`<br>`go:TestTriggersConditionsAndActions` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-084 | Integration | REG-00 (M13) | — | — | — | — | N/A_APPROVED |
| WFP-085 | Integration | REG-00 (M13) | — | — | — | — | N/A_APPROVED |
| WFP-086 | Admin & security | USR-01..03 | M1 | `go:TestAdminSeatAndLifecycle`<br>`pw:roles.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-087 | Admin & security | SPC-11 | M1 | `go:TestSpaceAccessRules`<br>`go:TestItemSharesVersionedReplace`<br>`pw:policy.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-088 | Admin & security | AUTH-05 | S3.5.1 | `go:TestTwoFactorLifecycle`<br>`go:TestTwoFactorBackupCodesAndLockout`<br>`pw:two-factor.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-089 | Admin & security | ENT-01, ENT-02 | S12.1.1–S12.1.3 | `go:TestSSOSignInLinksBySubjectAndKeepsPasswordLogin`<br>`go:TestSAMLSignInLinksAndRefusesForgeries`<br>`go:TestSCIMProvisionSignInDeprovisionAndRestore`<br>`pw:sso.spec.ts`<br>`pw:provisioning.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-090 | Admin & security | ENT-03 | S12.1.4 | `go:TestCustomRolesLeastPrivilegeAndLifecycle`<br>`pw:roles.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-091 | Admin & security | ENT-04 | S12.1.5 | `go:TestAuditTrailRecordsSecurityChangesForAdminsOnly`<br>`go:TestAuditExportIsFormulaSafeAndAudited`<br>`pw:audit.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-092 | Admin & security | ENT-05 | S12.1.6 | `go:TestLockedSpaceIsAdminOnlyAndAudited`<br>`pw:policy.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-093 | Admin & security | ENT-05 | S12.1.6 | `go:TestIPAllowlistGuardsSignInAndEveryRequest`<br>`pw:policy.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-094 | Admin & security | N/A (kunci enterprise, M13) | — | — | — | — | N/A_APPROVED |
| WFP-095 | Admin & security | USR-04, OPS-01, OPS-02 | S6.3.1, S6.3.2 | `go:TestNoEndpointDeletesTheAccount`<br>`go:TestUpgradeFromTheEarlySchemaKeepsEveryRow` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-096 | AI & visualization | N/A (AI, M13) | — | — | — | — | N/A_APPROVED |
| WFP-097 | AI & visualization | N/A (AI, M13) | — | — | — | — | N/A_APPROVED |
| WFP-098 | AI & visualization | N/A (AI, M13) | — | — | — | — | N/A_APPROVED |
| WFP-099 | AI & visualization | N/A (AI, M13) | — | — | — | — | N/A_APPROVED |
| WFP-100 | AI & visualization | N/A (Whiteboard, M13) | — | — | — | — | N/A_APPROVED |
| WFP-101 | AI & visualization | N/A (Whiteboard, M13) | — | — | — | — | N/A_APPROVED |
| WFP-102 | Experience | UI audit (STAB-07), NFR aksesibilitas | S7.1.4 | `pw:screens.a11y.spec.ts`<br>`pw:login.a11y.spec.ts`<br>`pw:pilot.a11y.spec.ts`<br>`pw:ui-audit.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026); audit UI: lihat [UI_AUDIT_RETEST_REPORT](../UI_AUDIT_RETEST_REPORT.md) | NOT RUN | PENDING_UAT |
| WFP-103 | Experience | SET-06 | S8.3.4 | `pw:theme.spec.ts` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-104 | Experience | SET-03 | M1 | `go:TestProfilePreferencesPersist` | [TESTING §S12.2.1](../TESTING.md#m12--s1221-regresi-parity-penuh-14-nov-2026) | NOT RUN | PENDING_UAT |
| WFP-105 | Experience | N/A (aplikasi native, M13) | — | — | — | — | N/A_APPROVED |
| WFP-106 | Experience | N/A (offline/push, M13) | — | — | — | — | N/A_APPROVED |
