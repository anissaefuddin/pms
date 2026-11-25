#!/bin/sh
# DEF-020: every decommission attempt leaves an audit record, and the guards still refuse. Docker is stubbed; nothing real is touched.
set -eu
deploy=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/deploy" "$tmp/bin" "$tmp/pms-export-20260101T000000Z"
cp "$deploy/decommission.sh" "$deploy/lib.sh" "$tmp/deploy/"
printf '%s\n' PMS_HOST=pms.example.test POSTGRES_PASSWORD=not-for-the-audit >"$tmp/deploy/.env"
cat >"$tmp/bin/docker" <<'STUB'
#!/bin/sh
case "$*" in
  *" down "*) echo "$*" >>"$STUB_DOWN"; exit "${STUB_DOWN_RC:-0}" ;;
  *"pms version"*) echo v9.9.9 ;;
esac
STUB
chmod +x "$tmp/bin/docker"
export PATH="$tmp/bin:$PATH" STUB_DOWN="$tmp/down.log" DECOMMISSION_AUDIT_DIR="$tmp/audit"
for f in database.dump files.tar.gz SHA256SUMS; do echo x >"$tmp/pms-export-20260101T000000Z/$f"; done
good=$tmp/pms-export-20260101T000000Z.tar.gz
tar -czf "$good" -C "$tmp" pms-export-20260101T000000Z
echo garbage >"$tmp/corrupt.tar.gz"
tar -czf "$tmp/partial.tar.gz" -C "$tmp" pms-export-20260101T000000Z/database.dump
log=$tmp/audit/decommission.jsonl

failed=0
check() { if eval "$2" >/dev/null 2>&1; then echo "ok   $1"; else echo "FAIL $1"; failed=1; fi; }
# run LABEL WANT_EXIT BUNDLE TYPED_HOST
run() {
  : >"$STUB_DOWN"
  rc=0; "$tmp/deploy/decommission.sh" "$3" "$4" >"$tmp/out" 2>&1 || rc=$?
  check "$1: exit $2 (got $rc)" "[ $rc -eq $2 ]"
}
last() { tail -n 1 "$log" 2>/dev/null; }
refused() { # LABEL FIELD REASON_SUBSTRING
  check "$1: nothing removed" '[ ! -s "$STUB_DOWN" ]'
  check "$1: audited as $2=failed" "last | grep -q '\"$2\":\"failed\"' && last | grep -q '\"execution\":\"not_run\"' && last | grep -q '$3'"
}

run "wrong host" 2 "$good" other.example.test; refused "wrong host" confirmation PMS_HOST
run "empty confirmation" 2 "$good" ""; refused "empty confirmation" confirmation PMS_HOST
run "missing bundle" 1 "$tmp/nope.tar.gz" pms.example.test; refused "missing bundle" preflight "not found"
run "corrupt bundle" 1 "$tmp/corrupt.tar.gz" pms.example.test; refused "corrupt bundle" preflight unreadable
run "incomplete bundle" 1 "$tmp/partial.tar.gz" pms.example.test; refused "incomplete bundle" preflight lacks
cp "$log" "$tmp/before" 2>/dev/null || : >"$tmp/before"

export STUB_DOWN_RC=7
run "down fails" 7 "$good" pms.example.test
check "down fails: audited" "last | grep -q '\"execution\":\"failed\"' && last | grep -q 'exited 7'"
unset STUB_DOWN_RC

run "success" 0 "$good" pms.example.test
check "success: removed volumes" 'grep -q -- "--volumes" "$STUB_DOWN"'
check "success: started and succeeded records share one operation id" \
  '[ "$(tail -n 2 "$log" | sed "s/.*\"operation_id\":\"\([^\"]*\)\".*/\1/" | uniq | wc -l)" -eq 1 ] && tail -n 2 "$log" | head -n 1 | grep -q "\"execution\":\"started\"" && last | grep -q "\"execution\":\"succeeded\""'
sum=$({ sha256sum "$good" 2>/dev/null || shasum -a 256 "$good"; } | cut -d' ' -f1)
for want in "\"operator\":\"$(id -un)\"" '"instance":"deploy"' '"target_host":"pms.example.test"' '"app_version":"v9.9.9"' \
  '"bundle_id":"pms-export-20260101T000000Z"' "\"bundle_sha256\":\"$sum\"" '"preflight":"passed"' '"confirmation":"passed"' '"timestamp":"20' '"sudo_user":' '"failure_reason":""'; do
  check "success record has $want" "last | grep -qF '$want'"
done
check "append only: earlier records unchanged" '[ -s "$tmp/before" ] && head -n "$(wc -l <"$tmp/before")" "$log" | cmp -s - "$tmp/before"'
check "every line is valid JSON" 'python3 -c "import json,sys; [json.loads(l) for l in open(sys.argv[1])]" "$log"'
check "no secrets in the audit" '[ -s "$log" ] && ! grep -q not-for-the-audit "$log"'
check "modes 0700 / 0600" '[ "$(ls -ld "$tmp/audit" | cut -c1-10)" = drwx------ ] && [ "$(ls -l "$log" | cut -c1-10)" = -rw------- ]'

export DECOMMISSION_AUDIT_DIR="$tmp/corrupt.tar.gz/audit" # parent is a file: cannot be created, even by root
run "audit unwritable" 1 "$good" pms.example.test
check "audit unwritable: nothing removed" '[ ! -s "$STUB_DOWN" ]'

[ "$failed" -eq 0 ] && echo "PASS decommission-audit" || { echo "FAIL decommission-audit"; exit 1; }
