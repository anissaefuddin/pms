#!/usr/bin/env bash
# Runs the tests that carry parity scenarios E2E-PAR-01..12 and fails when a listed test did not run.
# usage: scripts/parity-e2e.sh 01-04 | 05-08 | 09-12   (needs PMS_TEST_DATABASE_URL)
set -euo pipefail
cd "$(dirname "$0")/.."
case "${1:?group 01-04, 05-08 or 09-12}" in
  01-04) tests=(TestItemTypesCRUDDefaultsAndChildRules TestCustomFieldTypesAndConfig TestItemAuthorizationMatrix TestStatusGuardsRequireFieldsAndActorAndWriteNothingOnRefusal TestTriggersConditionsAndActions TestRuleEffectIsExactlyOnceOnRedelivery TestOutboxEmitsWithTheChangeAndDispatchesReliably) ;;
  05-08) tests=(TestFormBranchesDecideQuestionsItemTypeAndFields TestFormBranchingBlueprintItem TestBlueprintSaveAndUse TestApprovals TestApprovalHistoryIsAppendOnlyAndPinnedToItemVersion TestApprovalPoliciesReassignAndDeadlines TestResourceWorkload TestBaselinesFreezeDatesAndCannotBeChanged) ;;
  09-12) tests=(TestFileApprovalPinnedToVersionWithExternalReviewer TestFormulaFieldsAreValidatedRecalculatedAndProtected TestAnalyticsEngineMatchesRawDataAndPolicy TestAPIKeysScopeRotateRevokeAndOwnerLimits TestWebhookSignedDeliveryAndVisibility TestWebhookRetryThenAutoDisable TestSCIMProvisionSignInDeprovisionAndRestore) ;;
  *) echo "unknown group $1" >&2; exit 2 ;;
esac
pattern="^($(IFS='|'; echo "${tests[*]}"))\$"
out=$(go test ./server/internal/httpserver -run "$pattern" -count=1 -v 2>&1) || { grep -E "^(--- FAIL|FAIL|panic)" <<<"$out"; exit 1; }
for t in "${tests[@]}"; do
  grep -q -- "^--- PASS: $t " <<<"$out" || { echo "not run or not passing: $t" >&2; grep -E "^--- (FAIL|SKIP): $t " <<<"$out" >&2 || true; exit 1; }
done
grep -E "^--- PASS" <<<"$out"
