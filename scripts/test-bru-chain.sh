#!/usr/bin/env bash
#
# Regression test for the seed-first collection rewire (Fix 2).
#
# The bruno CLI runner (v4) flattens the collection tree in folder order
# (`folder.yml` `seq`), then request order (`info.seq`), starts at the first
# request in that order, and follows `bru.runner.setNextRequest("<name>")`
# jumps. Before this change auth/magiclink.yml (auth folder seq 1) was the
# suite entry and referenced Organization/<org> that a fresh CI Blaze never
# had, so the very first request 409'd and the whole suite bailed.
#
# This test pins the rewired contract so regressions fail statically without
# needing a live environment:
#
#   - the fhir folder sorts before auth (seq 1 < 2), making
#     fhir/seed/seed-organization.yml the suite entry;
#   - seed-organization chains into the auth chain ("Send Magic Link");
#   - the auth chain resumes at the rest of the seed chain ("Seed Location");
#   - magiclink uses the seeded organization id var, never the raw
#     process.env.ORGANIZATION mailbox value;
#   - nothing anywhere still targets "Seed Organization" as a next request.
#
# Bash-only body (uses [[ ]]); run with: bash scripts/test-bru-chain.sh

ROOT="$(dirname "$0")"
[[ "${ROOT}" = "." ]] && ROOT="$(pwd)"
ROOT="$(cd "${ROOT}/.." && pwd)"

FAILED=0

# assert_grep <label> <file> <pattern> — pattern must match the file.
assert_grep() {
  label="$1"
  file="$2"
  pattern="$3"
  if grep -qE "${pattern}" "${ROOT}/${file}"; then
    echo "PASS: ${label}"
  else
    echo "FAIL: ${label} — '${pattern}' not found in ${file}"
    FAILED=1
  fi
  return 0
}

# assert_absent <label> <file> <pattern> — pattern must NOT match the file.
assert_absent() {
  label="$1"
  file="$2"
  pattern="$3"
  if grep -qE "${pattern}" "${ROOT}/${file}"; then
    echo "FAIL: ${label} — unexpected '${pattern}' in ${file}"
    FAILED=1
  else
    echo "PASS: ${label}"
  fi
  return 0
}

echo "== folder order: seed lives under fhir BEFORE auth =="
assert_grep "fhir folder seq is 1 (suite entry lands under it)" docs/api/fhir/folder.yml '^[[:space:]]*seq: 1$'
assert_grep "auth folder seq is 2 (no longer the tree entry)" docs/api/auth/folder.yml '^[[:space:]]*seq: 2$'

echo "== chain edges =="
assert_grep "seed-organization chains into the auth chain" docs/api/fhir/seed/seed-organization.yml 'setNextRequest\("Send Magic Link"\)'
assert_grep "seed-organization still captures the org id" docs/api/fhir/seed/seed-organization.yml 'bru.setVar\("organizationId", body.id\)'
assert_grep "active-role chains into the rest of the seed chain" docs/api/auth/active-role.yml 'setNextRequest\("Seed Location"\)'
assert_grep "active-role docs reflect the new target" docs/api/auth/active-role.yml 'chains to "Seed Location"'

echo "== magiclink uses the seeded org id =="
assert_grep "magiclink organizationId comes from the seeded org var" docs/api/auth/magiclink.yml '"organizationId": "{{organizationId}}"'
assert_absent "magiclink no longer uses the RAW mailbox org" docs/api/auth/magiclink.yml '"organizationId": "{{process.env.ORGANIZATION}}"'

echo "== no dangling 'Seed Organization' next-request targets =="
for f in docs/api/auth/*.yml docs/api/fhir/seed/*.yml; do
  if grep -q 'setNextRequest("Seed Organization")' "${ROOT}/${f}" 2>/dev/null; then
    echo "FAIL: ${f} still chains to Seed Organization"
    FAILED=1
  fi
done
if [[ "${FAILED}" = "1" ]]; then echo "FAILED: one or more chain assertions did not hold"; exit 1; fi
echo "PASS: no resource targets 'Seed Organization' as next"
echo "ALL PASS"
