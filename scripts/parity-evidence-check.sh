#!/usr/bin/env bash
# Fails when a test named in docs/parity/PARITY_TEST_EVIDENCE.md (`go:Name`, `pw:file.spec.ts`) does not exist, or the summary disagrees with the table.
# It proves the names exist, not that the tests pass (see docs/quality/TESTING.md).
set -euo pipefail
cd "$(dirname "$0")/.."
doc=docs/parity/PARITY_TEST_EVIDENCE.md
bad=0; n=0
while read -r tok; do
  n=$((n+1))
  case "$tok" in
    go:*) grep -rqE --include='*_test.go' "^func ${tok#go:}\(" server || { echo "missing Go test: ${tok#go:}" >&2; bad=1; } ;;
    pw:*) [ -f "web/tests/${tok#pw:}" ] && grep -qE "^\s*test(\.\w+)?\(|^\s*test\.describe" "web/tests/${tok#pw:}" || { echo "missing Playwright spec: ${tok#pw:}" >&2; bad=1; } ;;
  esac
done < <(grep -oE '`(go|pw):[^`]+`' "$doc" | tr -d '`' | sort -u)
rows=$(grep -cE '^\| WFP-[0-9]+ ' "$doc")
for s in PENDING_UAT N/A_APPROVED GAP ACCEPTED; do
  in_table=$(grep -E '^\| WFP-' "$doc" | awk -F'|' -v s="$s" '{gsub(/ /,"",$9); if($9==s)c++} END{print c+0}')
  in_sum=$(grep -E "^\| \`$s\` \|" "$doc" | awk -F'|' '{gsub(/ /,"",$3); print $3}' || true)
  [ "${in_sum:-0}" = "$in_table" ] || { echo "summary $s=${in_sum:-0} but table has $in_table" >&2; bad=1; }
done
[ "$rows" = 106 ] || { echo "expected 106 WFP rows, found $rows" >&2; bad=1; }
[ $bad = 0 ] && echo "OK: $n distinct test references exist; $rows WFP rows; summary matches table"
exit $bad
