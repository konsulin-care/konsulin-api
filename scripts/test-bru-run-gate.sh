#!/usr/bin/env bash
#
# Regression test for the Bruno report gate (Fix 1): scripts/bru-report-gate.mjs
# and its wiring inside scripts/bru-run.sh.
#
# The bru CLI's --reporter-json output is a JSON *array* of per-iteration
# results: [ { iterationIndex, results, summary } ]. The original inline node
# snippet read `r.summary` off the array root, saw `undefined`, computed 0 and
# always exited 0 — so every PR check passed even when the suite bailed with
# failures. This test pins the fixed behaviour:
#
#   - array-root and object-root reports are both understood;
#   - any failedAssertions / failedTests / failedRequests / errorRequests count
#     makes the gate exit 1;
#   - skippedByBail > 0 is a hard failure (a bailed run is an incomplete run);
#   - missing or unparseable report files exit 1 (fail closed);
#   - bru-run.sh delegates to the gate module so pre-push and CI share it.
#
# Requires node and bash. Exits non-zero when any assertion fails.
# Run with: bash scripts/test-bru-run-gate.sh

SCRIPT_DIR="$(dirname "$0")"
[[ "${SCRIPT_DIR}" = "." ]] && SCRIPT_DIR="$(pwd)"
GATE="${SCRIPT_DIR}/bru-report-gate.mjs"
TMP="${TMPDIR:-/tmp}/bru-gate-test-$$"
mkdir -p "${TMP}"
trap 'rm -rf "${TMP}"' EXIT

FAILED=0

write_fixture() {
  local name="$1"
  local content="$2"
  printf '%s' "${content}" > "${TMP}/${name}"
  return 0
}

expect_rc() {
  expected="$1"
  label="$2"
  path="$3"
  node "${GATE}" < "${path}" >/dev/null 2>&1
  rc=$?
  if [[ "${rc}" -eq "${expected}" ]]; then
    echo "PASS: ${label} (rc=${rc})"
  else
    echo "FAIL: ${label} — expected rc=${expected}, got ${rc}"
    FAILED=1
  fi
  return 0
}

echo "== fixture reports =="

# The exact CI failure shape: array root, one iteration, 83 requests, bail at #1.
write_fixture "fail-array.json" '[{"iterationIndex":0,"results":[{"name":"Send Magic Link","status":"pass"}],"summary":{"totalRequests":83,"passedRequests":0,"failedRequests":1,"errorRequests":0,"skippedRequests":82,"skippedByBail":82,"totalAssertions":4,"passedAssertions":1,"failedAssertions":3,"totalTests":0,"passedTests":0,"failedTests":0,"totalPreRequestTests":0,"passedPreRequestTests":0,"failedPreRequestTests":0,"failedPostResponseTests":0,"totalPostResponseTests":0,"passedPostResponseTests":0}}]'

# Clean local-run shape: array root, all green, no bail.
write_fixture "pass-array.json" '[{"iterationIndex":0,"results":[],"summary":{"totalRequests":69,"passedRequests":69,"failedRequests":0,"errorRequests":0,"skippedRequests":0,"skippedByBail":0,"totalAssertions":90,"passedAssertions":90,"failedAssertions":0,"totalTests":61,"passedTests":61,"failedTests":0,"totalPreRequestTests":0,"passedPreRequestTests":0,"failedPreRequestTests":0,"failedPostResponseTests":0,"totalPostResponseTests":0,"passedPostResponseTests":0}}]'

# Older/object-root schema robustness.
write_fixture "fail-object.json" '{"summary":{"totalRequests":10,"passedRequests":8,"failedRequests":2,"errorRequests":0,"skippedRequests":0,"skippedByBail":0,"totalAssertions":20,"passedAssertions":18,"failedAssertions":2,"totalTests":0,"passedTests":0,"failedTests":0,"totalPreRequestTests":0,"passedPreRequestTests":0,"failedPreRequestTests":0,"failedPostResponseTests":0,"totalPostResponseTests":0,"passedPostResponseTests":0}}'
write_fixture "pass-object.json" '{"summary":{"totalRequests":10,"passedRequests":10,"failedRequests":0,"errorRequests":0,"skippedRequests":0,"skippedByBail":0,"totalAssertions":20,"passedAssertions":20,"failedAssertions":0,"totalTests":10,"passedTests":10,"failedTests":0,"totalPreRequestTests":0,"passedPreRequestTests":0,"failedPreRequestTests":0,"failedPostResponseTests":0,"totalPostResponseTests":0,"passedPostResponseTests":0}}'

# Bail-only: every count zero except skippedByBail — must still fail (incomplete run).
write_fixture "bail-only.json" '[{"iterationIndex":0,"results":[],"summary":{"totalRequests":83,"passedRequests":0,"failedRequests":0,"errorRequests":0,"skippedRequests":82,"skippedByBail":82,"totalAssertions":0,"passedAssertions":0,"failedAssertions":0,"totalTests":0,"passedTests":0,"failedTests":0,"totalPreRequestTests":0,"passedPreRequestTests":0,"failedPreRequestTests":0,"failedPostResponseTests":0,"totalPostResponseTests":0,"passedPostResponseTests":0}}]'

# Bare entries without a summary must not crash the gate.
write_fixture "bare-entries.json" '[{"iterationIndex":0,"results":[]}]'

write_fixture "not-json.txt" 'this is not json'

echo "== failure detection (array root) =="
expect_rc 1 "CI failure shape (failedRequests=1, failedAssertions=3, skippedByBail=82)" "${TMP}/fail-array.json"
echo "== clean runs pass =="
expect_rc 0 "clean array report" "${TMP}/pass-array.json"
expect_rc 0 "clean object report" "${TMP}/pass-object.json"
expect_rc 0 "bare entries without summary" "${TMP}/bare-entries.json"
echo "== object-root robustness =="
expect_rc 1 "object report with failures" "${TMP}/fail-object.json"
echo "== bail is a hard failure =="
expect_rc 1 "skippedByBail only" "${TMP}/bail-only.json"
echo "== fail closed =="
expect_rc 1 "missing report file (empty stdin)" "/dev/null"
expect_rc 1 "unparseable report file" "${TMP}/not-json.txt"

echo "== wiring: bru-run.sh delegates to the gate module =="
if grep -q "bru-report-gate.mjs" "${SCRIPT_DIR}/bru-run.sh"; then
  echo "PASS: bru-run.sh references scripts/bru-report-gate.mjs"
else
  echo "FAIL: bru-run.sh does not delegate to the gate module"
  FAILED=1
fi

if [[ "${FAILED}" = "1" ]]; then
  echo "FAILED: one or more gate assertions did not hold"
  exit 1
fi
echo "ALL PASS"
